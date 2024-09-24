package cluster

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts/dialer"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/pkg/errors"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	registryPath              = "/etc/rancher/k3s"
	datastoreCertificatesPath = "/etc/rancher/datastore"
)

// InitK3sCluster initial K3S cluster.
func (p *ProviderBase) InitK3sCluster(cluster *types.Cluster, deployCCM func() []string) error {
	p.Logger.Infof("[%s] executing init k3s cluster logic...", p.Provider)

	provider, err := providers.GetProvider(p.Provider)
	if err != nil {
		return err
	}

	pkg, err := airgap.PreparePackage(cluster)
	if err != nil {
		return err
	}
	// package's name is empty, it means that it is a temporary dir and it needs to be remove after.
	if pkg != nil && pkg.Name == "" {
		defer os.RemoveAll(pkg.FilePath)
	}

	if cluster.Token == "" {
		token, err := utils.RandomToken(16)
		if err != nil {
			return err
		}
		cluster.Token = token
	}

	if len(cluster.MasterNodes) <= 0 || len(cluster.MasterNodes[0].InternalIPAddress) <= 0 {
		return errors.New("[cluster] master node internal ip address can not be empty")
	}

	publicIP := cluster.IP
	if cluster.IP == "" {
		cluster.IP = cluster.MasterNodes[0].InternalIPAddress[0]
		publicIP = cluster.MasterNodes[0].PublicIPAddress[0]
	}

	// initialize the first master node and worker node to validate the K3s configuration.
	var firstControl, firstWorker types.Node
	controlNodes := []types.Node{}
	for i, control := range cluster.MasterNodes {
		if i == 0 {
			firstControl = control
			continue
		}
		controlNodes = append(controlNodes, control)
	}
	workerNodes := []types.Node{}
	if len(p.WorkerNodes) > 0 {
		for i, w := range cluster.WorkerNodes {
			if i == 0 {
				firstWorker = w
				continue
			}
			workerNodes = append(workerNodes, w)
		}
	}

	err = p.validateClusterConfig(cluster, provider, publicIP, pkg, firstControl, firstWorker, deployCCM)
	if err != nil {
		return err
	}

	for i, master := range controlNodes {
		p.Logger.Infof("[%s] join k3s control-%d...", p.Provider, i+1)
		if err := p.initControlNode(cluster, provider, publicIP, pkg, master, false); err != nil {
			return err
		}
		p.Logger.Infof("[%s] successfully created k3s master-%d", p.Provider, i+1)
	}

	// batch join worker nodes
	var wg sync.WaitGroup
	var l sync.RWMutex
	for i, worker := range workerNodes {
		wg.Add(1)
		go func(i int, worker types.Node) {
			defer wg.Done()
			p.Logger.Infof("[%s] creating k3s worker-%d...", p.Provider, i+1)
			if err := p.initWorkerNode(cluster, provider, publicIP, pkg, worker); err != nil {
				l.Lock()
				p.ErrM[worker.InstanceID] = err.Error()
				l.Unlock()
			}
			p.Logger.Infof("[%s] successfully created k3s worker-%d", p.Provider, i+1)
		}(i, worker)
	}
	wg.Wait()

	// get k3s cluster config.
	cfg, err := p.executeWithRetry(3, &cluster.MasterNodes[0], catCfgCommand)
	if err != nil {
		return err
	}

	// merge current cluster to kube config.
	if err := SaveCfg(cfg, publicIP, cluster.ContextName); err != nil {
		return err
	}
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, filepath.Join(common.CfgPath, common.KubeCfgFile))
	cluster.Status.Status = common.StatusRunning

	// write current cluster to state file.
	if err := common.DefaultDB.SaveCluster(cluster); err != nil {
		return err
	}

	p.Logger.Infof("[%s] deploying additional manifests", p.Provider)

	// deploy additional UI manifests.
	enabledPlugins := map[string]bool{}

	// deploy plugin
	if cluster.Enable != nil {
		for _, comp := range cluster.Enable {
			enabledPlugins[comp] = true
		}
	}

	for plugin := range enabledPlugins {
		if plugin == "explorer" {
			// start kube-explorer
			port, err := common.EnableExplorer(context.Background(), cluster.ContextName)
			if err != nil {
				p.Logger.Errorf("[%s] failed to start kube-explorer for cluster %s: %v", p.Provider, cluster.ContextName, err)
			}
			if port != 0 {
				p.Logger.Infof("[%s] kube-explorer for cluster %s will listen on 127.0.0.1:%d...", p.Provider, cluster.Name, port)
			}
		}
	}

	p.Logger.Infof("[%s] successfully deployed additional manifests", p.Provider)
	p.Logger.Infof("[%s] successfully executed init k3s cluster logic", p.Provider)
	return nil
}

// Join join K3S nodes to exist K3S cluster.
func (p *ProviderBase) Join(merged, added *types.Cluster) error {
	p.Logger.Infof("[%s] executing join k3s node logic", merged.Provider)

	provider, err := providers.GetProvider(merged.Provider)
	if err != nil {
		return err
	}

	pkg, err := airgap.PreparePackage(merged)
	if err != nil {
		return err
	}
	// package's name is empty, it means that it is a temporary dir and it needs to be remove after.
	if pkg != nil && pkg.Name == "" {
		defer os.RemoveAll(pkg.FilePath)
	}

	if merged.IP == "" {
		if len(merged.MasterNodes) <= 0 || len(merged.MasterNodes[0].InternalIPAddress) <= 0 {
			return errors.New("[cluster] master node internal ip address can not be empty")
		}
		merged.IP = merged.MasterNodes[0].InternalIPAddress[0]
	}
	publicIP := merged.IP

	// get cluster token from `--ip` address.
	if merged.Token == "" {
		serverNode := types.Node{}
		if len(added.MasterNodes) > 0 {
			serverNode = added.MasterNodes[0]
		} else {
			serverNode = added.WorkerNodes[0]
		}
		serverNode.PublicIPAddress = []string{merged.IP}
		token, err := p.execute(&serverNode, getTokenCommand)
		if err != nil {
			return err
		}
		merged.Token = strings.TrimSpace(token)
	}

	if merged.Token == "" {
		return errors.New("[cluster] k3s token can not be empty")
	}

	masterNodes := nodeByInstanceID(merged.MasterNodes)
	workerNodes := nodeByInstanceID(merged.WorkerNodes)

	for i := 0; i < len(added.Status.MasterNodes); i++ {
		currentNode := added.MasterNodes[i]
		full, ok := masterNodes[currentNode.InstanceID]
		if !ok {
			continue
		}
		extraArgs := merged.MasterExtraArgs
		p.Logger.Infof("[%s] joining k3s master-%d...", merged.Provider, i+1)
		additionalExtraArgs := provider.GenerateMasterExtraArgs(added, full)
		if additionalExtraArgs != "" {
			extraArgs += additionalExtraArgs
		}
		if err := p.initNode(false, publicIP, merged, full, extraArgs, pkg); err != nil {
			return err
		}
		p.Logger.Infof("[%s] successfully joined k3s master-%d", merged.Provider, i+1)
	}

	var wg sync.WaitGroup
	var l sync.RWMutex

	for i := 0; i < len(added.Status.WorkerNodes); i++ {
		currentNode := added.WorkerNodes[i]
		full, ok := workerNodes[currentNode.InstanceID]
		if !ok {
			continue
		}
		wg.Add(1)

		go func(i int, node types.Node) {
			defer wg.Done()
			p.Logger.Infof("[%s] joining k3s worker-%d...", merged.Provider, i+1)
			extraArgs := merged.WorkerExtraArgs
			additionalExtraArgs := provider.GenerateWorkerExtraArgs(added, full)
			if additionalExtraArgs != "" {
				extraArgs += additionalExtraArgs
			}
			if err := p.initNode(false, publicIP, merged, full, extraArgs, pkg); err != nil {
				l.Lock()
				p.ErrM[full.InstanceID] = err.Error()
				l.Unlock()
			}
			p.Logger.Infof("[%s] successfully joined k3s worker-%d", merged.Provider, i+1)
		}(i, full)
	}
	wg.Wait()

	// sync master & worker numbers.
	merged.Master = strconv.Itoa(len(merged.MasterNodes))
	merged.Worker = strconv.Itoa(len(merged.WorkerNodes))

	if p.Provider == "native" {
		// check cluster context exists
		kubeCfg := filepath.Join(common.CfgPath, common.KubeCfgFile)
		clientConfig, err := clientcmd.LoadFromFile(kubeCfg)
		if err != nil {
			return err
		}
		contexts := clientConfig.Contexts
		if _, ok := contexts[p.ContextName]; !ok {
			// get k3s cluster config.
			cfg, err := p.execute(&types.Node{
				PublicIPAddress: []string{merged.IP},
				SSH:             merged.SSH,
				Master:          true,
			}, catCfgCommand)
			if err == nil {
				// merge current cluster to kube config.
				if err := SaveCfg(cfg, merged.IP, p.ContextName); err != nil {
					p.Logger.Warnf("[%s] can't save kubeconfig file with error: %v", merged.Provider, err)
				}
				_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, filepath.Join(common.CfgPath, common.KubeCfgFile))
			} else {
				p.Logger.Warnf("[%s] can't get kubeconfig file from master %s", merged.Provider, merged.IP)
			}
		}
	}

	merged.Status.Status = common.StatusRunning
	// write current cluster to state file.
	if err = common.DefaultDB.SaveCluster(merged); err != nil {
		p.Logger.Errorf("failed to save cluster state: %v", err)
		return nil
	}

	p.Logger.Infof("[%s] successfully executed join k3s node logic", merged.Provider)
	return nil
}

// SSHK3sNode ssh to K3S node.
func SSHK3sNode(ip string, cluster *types.Cluster, ssh *types.SSH) error {
	var node types.Node

	for _, n := range cluster.Status.MasterNodes {
		if n.PublicIPAddress[0] == ip || n.InstanceID == ip {
			node = n
			break
		}
	}

	for _, n := range cluster.Status.WorkerNodes {
		if n.PublicIPAddress[0] == ip || n.InstanceID == ip {
			node = n
			break
		}
	}

	if ssh.SSHUser != "" {
		node.SSH.SSHUser = ssh.SSHUser
	}
	if ssh.SSHPort != "" {
		node.SSH.SSHPort = ssh.SSHPort
	}
	if ssh.SSHPassword != "" {
		node.SSH.SSHPassword = ssh.SSHPassword
	}
	if ssh.SSHKeyPath != "" {
		node.SSH.SSHKeyPath = ssh.SSHKeyPath
	}
	if ssh.SSHCert != "" {
		node.SSH.SSHCert = ssh.SSHCert
	}
	if ssh.SSHCertPath != "" {
		node.SSH.SSHCertPath = ssh.SSHCertPath
	}
	if ssh.SSHKeyPassphrase != "" {
		node.SSH.SSHKeyPassphrase = ssh.SSHKeyPassphrase
	}
	if ssh.SSHAgentAuth {
		node.SSH.SSHAgentAuth = ssh.SSHAgentAuth
	}
	if node.PublicIPAddress == nil {
		node.PublicIPAddress = []string{ip}
	}

	if node.SSH.SSHPort == "" {
		node.SSH.SSHPort = "22"
	}

	// preCheck ssh config
	if node.SSH.SSHUser == "" || (node.SSH.SSHPassword == "" && node.SSH.SSHKeyPath == "") {
		return fmt.Errorf("couldn't ssh to chosen node with current ssh config: --ssh-user %s --ssh-port %s --ssh-password %s --ssh-key-path %s", node.SSH.SSHUser, node.SSH.SSHPort, node.SSH.SSHPassword, node.SSH.SSHKeyPath)
	}

	return terminal(&node)
}

// UninstallK3sNodes uninstall K3S on the given nodes.
func (p *ProviderBase) UninstallK3sNodes(nodes []types.Node) (warnMsg []string) {
	for _, node := range nodes {
		if node.Master {
			_, e := p.execute(&node, masterUninstallCommand)
			if e != nil {
				warnMsg = append(warnMsg, fmt.Sprintf("failed to uninstall k3s on master node %s: %s", node.InstanceID, e.Error()))
			}
		} else {
			_, e := p.execute(&node, workerUninstallCommand)
			if e != nil {
				warnMsg = append(warnMsg, fmt.Sprintf("failed to uninstall k3s on worker node %s: %s", node.InstanceID, e.Error()))
			}
		}
	}

	return
}

// SaveCfg save kube config file.
func SaveCfg(cfg, ip, context string) error {
	replacer := strings.NewReplacer(
		"127.0.0.1", ip,
		"localhost", ip,
		"default", context,
	)

	result := replacer.Replace(cfg)

	tempPath := filepath.Join(common.CfgPath, ".kube")
	if err := utils.EnsureFolderExist(tempPath); err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp folder error, msg: %s", err)
	}

	temp, err := os.CreateTemp(tempPath, common.KubeCfgTempName)
	if err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp file error, msg: %s", err)
	}
	defer func() {
		_ = temp.Close()
		if err := os.Remove(temp.Name()); err != nil {
			logrus.Errorf("[cluster] remove kubecfg temp file error, msg: %s", err)
		}
	}()

	absPath, _ := filepath.Abs(temp.Name())
	if err = os.WriteFile(absPath, []byte(result), 0600); err != nil {
		return fmt.Errorf("[cluster] write content to kubecfg temp file error: %v", err)
	}

	return common.FileManager.SaveCfg(context, temp.Name())
}

// DeployExtraManifest deploy extra K3S manifest.
func (p *ProviderBase) DeployExtraManifest(cluster *types.Cluster, cmds []string) error {
	if _, err := p.execute(&cluster.MasterNodes[0], []string{fmt.Sprintf("mkdir -p %s", common.K3sManifestsDir)}...); err != nil {
		return err
	}
	if _, err := p.execute(&cluster.MasterNodes[0], cmds...); err != nil {
		return err
	}
	return nil
}

func (p *ProviderBase) initNode(isFirstMaster bool, fixedIP string, cluster *types.Cluster, node types.Node, extraArgs string, pkg *common.Package) error {
	if strings.Contains(extraArgs, "--docker") {
		dockerCmd := fmt.Sprintf(dockerCommand, cluster.DockerScript, cluster.DockerArg, cluster.DockerMirror)
		p.Logger.Infof("[cluster] install docker command %s", dockerCmd)
		if _, err := p.execute(&node, dockerCmd); err != nil {
			return err
		}
	}

	if cluster.Registry != "" || cluster.RegistryContent != "" {
		if err := p.handleRegistry(&node, cluster); err != nil {
			return err
		}
	}

	if cluster.DataStoreCAFile != "" || cluster.DataStoreCAFileContent != "" ||
		cluster.DataStoreCertFileContent != "" || cluster.DataStoreCertFile != "" ||
		cluster.DataStoreKeyFileContent != "" || cluster.DataStoreKeyFile != "" {
		if err := p.handleDataStoreCertificate(&node, cluster); err != nil {
			return err
		}
	}

	if pkg != nil {
		if err := p.scpFiles(cluster.Name, pkg, &node, extraArgs); err != nil {
			return err
		}
	}

	// handle configuration file
	if err := p.handleConfiguration(&node, cluster); err != nil {
		return err
	}

	cmd := getCommand(isFirstMaster, fixedIP, cluster, node, []string{extraArgs})
	nodeRole := "master"
	if !node.Master {
		nodeRole = "worker"
	}

	p.Logger.Infof("[cluster] k3s %s command: %s", nodeRole, cmd)

	if _, err := p.execute(&node, cmd); err != nil {
		return err
	}

	return nil
}

func (p *ProviderBase) execute(n *types.Node, cmds ...string) (string, error) {
	if len(cmds) <= 0 {
		return "", nil
	}

	dialer, err := dialer.NewSSHDialer(n, true, p.Logger)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = dialer.Close()
	}()
	output, err := dialer.ExecuteCommands(cmds...)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}

	return output, nil
}

func (p *ProviderBase) executeWithRetry(count int, n *types.Node, cmds ...string) (string, error) {
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   1,
		Steps:    count,
	}
	var rtn string
	var lastError error
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		rtn, lastError = p.execute(n, cmds...)
		if lastError != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", errors.Wrapf(lastError, "failed to execute cmd with max retry count %d", count)
	}
	return rtn, nil
}

func terminal(n *types.Node) error {
	dialer, err := dialer.NewSSHDialer(n, true, common.NewLogger(nil))
	if err != nil {
		return err
	}

	defer func() {
		_ = dialer.Close()
	}()
	shell, err := dialer.OpenShell()
	if err != nil {
		return err
	}

	shell.SetIO(os.Stdout, os.Stderr, os.Stdin)

	return shell.Terminal()
}

func (p *ProviderBase) handleRegistry(n *types.Node, c *types.Cluster) (err error) {
	if c.Registry == "" && c.RegistryContent == "" {
		return nil
	}
	var cmd []string

	registry, err := utils.VerifyRegistryFileContent(p.Registry, p.RegistryContent)
	if err != nil {
		return err
	}

	tls, err := registryTLSMap(registry)
	if err != nil {
		return err
	}

	if len(tls) > 0 {
		cmd, err = saveRegistryTLS(registry, tls)
		if err != nil {
			return err
		}
	} else {
		cmd = []string{fmt.Sprintf("mkdir -p %s", registryPath)}
	}

	registryContent, err := utils.RegistryToString(registry)
	if err != nil {
		return err
	}

	cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"/etc/rancher/k3s/registries.yaml\"",
		base64.StdEncoding.EncodeToString([]byte(registryContent))))
	_, err = p.execute(n, cmd...)
	return err
}

func (p *ProviderBase) handleConfiguration(n *types.Node, c *types.Cluster) (err error) {
	if c.ServerConfigFileContent == "" && c.ServerConfigFile == "" && c.AgentConfigFileContent == "" && c.AgentConfigFile == "" {
		return nil
	}
	var configFileContent []byte
	if n.Master {
		if c.ServerConfigFileContent != "" {
			configFileContent, err = base64.StdEncoding.DecodeString(c.ServerConfigFileContent)
			if err != nil {
				return err
			}
		}
		if c.ServerConfigFile != "" && c.ServerConfigFileContent == "" {
			configFileContent, err = os.ReadFile(c.ServerConfigFile)
			if err != nil {
				return err
			}
		}
	} else {
		if c.AgentConfigFileContent != "" {
			configFileContent, err = base64.StdEncoding.DecodeString(c.AgentConfigFileContent)
			if err != nil {
				return err
			}
		}
		if c.AgentConfigFile != "" && c.AgentConfigFileContent == "" {
			configFileContent, err = os.ReadFile(c.AgentConfigFile)
			if err != nil {
				return err
			}
		}
	}
	if len(configFileContent) == 0 {
		return errors.New("configuration file is empty")
	}
	cmd := []string{"mkdir -p /etc/rancher/k3s"}
	cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"/etc/rancher/k3s/config.yaml\"",
		base64.StdEncoding.EncodeToString(configFileContent)))
	_, err = p.execute(n, cmd...)
	return err
}

func registryTLSMap(registry *registries.Registry) (m map[string]map[string][]byte, err error) {
	m = make(map[string]map[string][]byte)
	if registry == nil {
		err = fmt.Errorf("registry is nil")
		return
	}

	for r, c := range registry.Configs {
		if _, ok := m[r]; !ok {
			m[r] = map[string][]byte{}
		}
		if c.TLS == nil {
			continue
		}
		if c.TLS.CertFile != "" {
			b, err := os.ReadFile(c.TLS.CertFile)
			if err != nil {
				return m, err
			}
			m[r]["cert"] = b
		}
		if c.TLS.KeyFile != "" {
			b, err := os.ReadFile(c.TLS.KeyFile)
			if err != nil {
				return m, err
			}
			m[r]["key"] = b
		}
		if c.TLS.CAFile != "" {
			b, err := os.ReadFile(c.TLS.CAFile)
			if err != nil {
				return m, err
			}
			m[r]["ca"] = b
		}
	}

	return
}

func saveRegistryTLS(registry *registries.Registry, m map[string]map[string][]byte) ([]string, error) {
	cmd := make([]string, 0)
	for r, c := range m {
		if r != "" {
			if _, ok := registry.Configs[r]; !ok {
				return cmd, fmt.Errorf("registry map is not match the struct: %s", r)
			}

			// i.e /etc/rancher/k3s/mycustomreg:5000/.
			path := fmt.Sprintf("/etc/rancher/k3s/%s", r)
			cmd = append(cmd, fmt.Sprintf("mkdir -p %s", path))

			for f, b := range c {
				// i.e /etc/rancher/k3s/mycustomreg:5000/{ca,key,cert}.
				file := fmt.Sprintf("%s/%s", path, f)
				cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s\"", base64.StdEncoding.EncodeToString(b), file))
				cmd = append(cmd, fmt.Sprintf("chmod 755 %s", file))

				switch f {
				case "cert":
					registry.Configs[r].TLS.CertFile = file
				case "key":
					registry.Configs[r].TLS.KeyFile = file
				case "ca":
					registry.Configs[r].TLS.CAFile = file
				}
			}
		}
	}

	return cmd, nil
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

// GetClusterConfig generate kube config.
func GetClusterConfig(name, kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := buildConfigFromFlags(name, kubeconfig)
	if err != nil {
		return nil, err
	}
	config.Timeout = 15 * time.Second
	c, err := kubernetes.NewForConfig(config)
	return c, err
}

// GetClusterStatus get cluster status using cluster's /readyz API.
func GetClusterStatus(c *kubernetes.Clientset) string {
	_, err := c.RESTClient().Get().Timeout(15 * time.Second).RequestURI("/readyz").DoRaw(context.TODO())
	if err != nil {
		return types.ClusterStatusStopped
	}
	return types.ClusterStatusRunning
}

// GetClusterVersion get kube cluster version.
func GetClusterVersion(c *kubernetes.Clientset) string {
	v, err := c.DiscoveryClient.ServerVersion()
	if err != nil {
		return types.ClusterStatusUnknown
	}
	return v.GitVersion
}

// DescribeClusterNodes describe cluster nodes.
func DescribeClusterNodes(client *kubernetes.Clientset, instanceNodes []types.ClusterNode) ([]types.ClusterNode, error) {
	// list cluster nodes.
	timeout := int64(5 * time.Second)
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{TimeoutSeconds: &timeout})
	if err != nil || nodeList == nil {
		return nil, err
	}
	for _, node := range nodeList.Items {
		var internalIP, externalIP, hostName string
		addressList := node.Status.Addresses
		for _, address := range addressList {
			switch address.Type {
			case v1.NodeInternalIP:
				internalIP = address.Address
			case v1.NodeExternalIP:
				externalIP = address.Address
			case v1.NodeHostName:
				hostName = address.Address
			default:
				continue
			}
		}
		for index, n := range instanceNodes {
			isCurrentInstance := false
			for _, address := range n.InternalIP {
				if address == internalIP {
					isCurrentInstance = true
					break
				}
			}
			if !isCurrentInstance {
				for _, address := range n.ExternalIP {
					if address == externalIP {
						isCurrentInstance = true
						break
					}
				}
			}
			if !isCurrentInstance {
				if n.InstanceID == node.Name {
					isCurrentInstance = true
				}
			}
			if isCurrentInstance {
				n.HostName = hostName
				n.Version = node.Status.NodeInfo.KubeletVersion
				n.ContainerRuntimeVersion = node.Status.NodeInfo.ContainerRuntimeVersion
				// get roles.
				labels := node.Labels
				roles := make([]string, 0)
				for role := range labels {
					if strings.HasPrefix(role, "node-role.kubernetes.io") {
						roleArray := strings.Split(role, "/")
						if len(roleArray) > 1 {
							roles = append(roles, roleArray[1])
						}
					}
				}
				if len(roles) == 0 {
					roles = append(roles, "<none>")
				}
				sort.Strings(roles)
				n.Roles = strings.Join(roles, ",")
				// get status.
				conditions := node.Status.Conditions
				for _, c := range conditions {
					if c.Type == v1.NodeReady {
						if c.Status == v1.ConditionTrue {
							n.Status = "Ready"
						} else {
							n.Status = "NotReady"
						}
						break
					}
				}
				instanceNodes[index] = n
				break
			}
		}
	}
	return instanceNodes, nil
}

func (p *ProviderBase) Upgrade(cluster *types.Cluster) error {
	p.Logger.Infof("[%s] executing upgrade k3s cluster logic...", p.Provider)
	if len(cluster.MasterNodes) <= 0 || len(cluster.MasterNodes[0].InternalIPAddress) <= 0 {
		return errors.New("[cluster] master node internal ip address can not be empty")
	}

	pkg, err := airgap.PreparePackage(cluster)
	if err != nil {
		return err
	}
	// package's name is empty, it means that it is a temporary dir and it needs to be remove after.
	if pkg != nil && pkg.Name == "" {
		defer os.RemoveAll(pkg.FilePath)
	}

	provider, err := providers.GetProvider(p.Provider)
	if err != nil {
		return err
	}
	masterExtraArgs := cluster.MasterExtraArgs
	workerExtraArgs := cluster.WorkerExtraArgs

	publicIP := cluster.IP
	if cluster.IP == "" {
		cluster.IP = cluster.MasterNodes[0].InternalIPAddress[0]
		publicIP = cluster.MasterNodes[0].PublicIPAddress[0]
	}

	// upgrade server nodes
	for i, node := range cluster.MasterNodes {
		extraArgs := masterExtraArgs
		providerExtraArgs := provider.GenerateMasterExtraArgs(cluster, node)
		if providerExtraArgs != "" {
			extraArgs += providerExtraArgs
		}

		var cmd string

		if pkg != nil {
			if err := p.scpFiles(cluster.Name, pkg, &node, extraArgs); err != nil {
				return err
			}
			cmd = k3sRestart
		} else {
			cmd = getCommand(i == 0, publicIP, cluster, node, []string{extraArgs})
		}

		p.Logger.Infof("[cluster] upgrading k3s master %d command: %s", i+1, cmd)

		if _, err := p.execute(&node, cmd); err != nil {
			return err
		}
	}

	// upgrade worker nodes
	for i, node := range cluster.WorkerNodes {
		extraArgs := workerExtraArgs
		providerExtraArgs := provider.GenerateWorkerExtraArgs(cluster, node)
		if providerExtraArgs != "" {
			extraArgs += providerExtraArgs
		}

		var cmd string
		if pkg != nil {
			if err := p.scpFiles(cluster.Name, pkg, &node, extraArgs); err != nil {
				return err
			}
			cmd = k3sAgentRestart
		} else {
			cmd = getCommand(false, publicIP, cluster, node, []string{extraArgs})
		}

		p.Logger.Infof("[cluster] upgrading k3s worker %d command: %s", i+1, cmd)

		if _, err := p.execute(&node, cmd); err != nil {
			return err
		}
	}

	return nil
}

func nodeByInstanceID(nodes []types.Node) map[string]types.Node {
	rtn := make(map[string]types.Node, len(nodes))
	for _, node := range nodes {
		rtn[node.InstanceID] = node
	}
	return rtn
}

func (p *ProviderBase) scpFiles(clusterName string, pkg *common.Package, node *types.Node, extraArgs string) error {
	dialer, err := dialer.NewSSHDialer(node, true, p.Logger)
	if err != nil {
		return err
	}
	defer dialer.Close()
	return airgap.ScpFiles(p.Logger, clusterName, pkg, dialer, extraArgs)
}

func (p *ProviderBase) handleDataStoreCertificate(n *types.Node, c *types.Cluster) error {
	cmd := make([]string, 0)
	cmd = append(cmd, fmt.Sprintf("mkdir -p %s", datastoreCertificatesPath))
	if c.DataStoreCAFile != "" {
		caFile, err := os.ReadFile(c.DataStoreCAFile)
		if err != nil {
			return err
		}
		cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s/ds-ca.pem\"",
			base64.StdEncoding.EncodeToString(caFile), datastoreCertificatesPath))
	}
	if c.DataStoreCertFile != "" {
		certFile, err := os.ReadFile(c.DataStoreCertFile)
		if err != nil {
			return err
		}
		cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s/ds-cert.pem\"",
			base64.StdEncoding.EncodeToString(certFile), datastoreCertificatesPath))
	}
	if c.DataStoreKeyFile != "" {
		keyFile, err := os.ReadFile(c.DataStoreKeyFile)
		if err != nil {
			return err
		}
		cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s/ds-key.pem\"",
			base64.StdEncoding.EncodeToString(keyFile), datastoreCertificatesPath))
	}
	if c.DataStoreCAFileContent != "" {
		cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s/ds-ca.pem\"",
			base64.StdEncoding.EncodeToString([]byte(p.DataStoreCAFileContent)), datastoreCertificatesPath))
	}
	if c.DataStoreKeyFileContent != "" {
		cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s/ds-key.pem\"",
			base64.StdEncoding.EncodeToString([]byte(p.DataStoreKeyFileContent)), datastoreCertificatesPath))
	}
	if c.DataStoreCertFileContent != "" {
		cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | tee \"%s/ds-cert.pem\"",
			base64.StdEncoding.EncodeToString([]byte(p.DataStoreCertFileContent)), datastoreCertificatesPath))
	}
	_, err := p.execute(n, cmd...)
	return err
}

func (p *ProviderBase) validateClusterConfig(cluster *types.Cluster, provider providers.Provider, publicIP string, pkg *common.Package, firstControl, firstWorker types.Node, deployCCM func() []string) error {
	p.Logger.Infof("[%s] initialize control node...", p.Provider)
	if err := p.initControlNode(cluster, provider, publicIP, pkg, firstControl, true); err != nil {
		return err
	}
	if deployCCM != nil {
		extraManifests := deployCCM()
		err := p.DeployExtraManifest(cluster, extraManifests)
		if err != nil {
			return err
		}
	}
	p.Logger.Infof("[%s] successfully initialize the first control node", p.Provider)

	if len(firstWorker.PublicIPAddress) <= 0 && firstWorker.InstanceID == "" {
		// skip with empty worker node
		return nil
	}
	p.Logger.Infof("[%s] initialize worker node...", p.Provider)
	if err := p.initWorkerNode(cluster, provider, publicIP, pkg, firstWorker); err != nil {
		return err
	}
	p.Logger.Infof("[%s] successfully initialize the first worker node", p.Provider)

	return nil
}

func (p *ProviderBase) initControlNode(cluster *types.Cluster, provider providers.Provider, publicIP string, pkg *common.Package, controlNode types.Node, isFirst bool) error {
	masterExtraArgs := cluster.MasterExtraArgs
	providerExtraArgs := provider.GenerateMasterExtraArgs(cluster, controlNode)
	if providerExtraArgs != "" {
		masterExtraArgs += providerExtraArgs
	}

	return p.initNode(isFirst, publicIP, cluster, controlNode, masterExtraArgs, pkg)
}

func (p *ProviderBase) initWorkerNode(cluster *types.Cluster, provider providers.Provider, publicIP string, pkg *common.Package, workerNode types.Node) error {
	workerExtraArgs := cluster.WorkerExtraArgs
	providerExtraArgs := provider.GenerateWorkerExtraArgs(cluster, workerNode)
	if providerExtraArgs != "" {
		workerExtraArgs += providerExtraArgs
	}
	return p.initNode(false, publicIP, cluster, workerNode, workerExtraArgs, pkg)
}
