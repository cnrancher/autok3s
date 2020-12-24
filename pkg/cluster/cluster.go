package cluster

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/types/native"
	"github.com/cnrancher/autok3s/pkg/types/tencent"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/rancher/k3s/pkg/agent/templates"
	"github.com/sirupsen/logrus"
	yamlv3 "gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/config"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	logger                 *logrus.Logger
	initCommand            = "curl -sLS %s | %s K3S_TOKEN='%s' INSTALL_K3S_EXEC='server %s --tls-san %s --node-external-ip %s %s' %s sh -"
	joinCommand            = "curl -sLS %s | %s K3S_URL='https://%s:6443' K3S_TOKEN='%s' INSTALL_K3S_EXEC='%s' %s sh -"
	getTokenCommand        = "sudo cat /var/lib/rancher/k3s/server/node-token"
	catCfgCommand          = "sudo cat /etc/rancher/k3s/k3s.yaml"
	dockerCommand          = "curl http://rancher-mirror.cnrancher.com/autok3s/docker-install.sh | sh -s - %s"
	deployUICommand        = "echo \"%s\" | base64 -d | sudo tee \"%s/ui.yaml\""
	masterUninstallCommand = "sh /usr/local/bin/k3s-uninstall.sh"
	workerUninstallCommand = "sh /usr/local/bin/k3s-agent-uninstall.sh"
)

const (
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
)

func InitK3sCluster(cluster *types.Cluster) error {
	logger = common.NewLogger(common.Debug)
	logger.Infof("[%s] executing init k3s cluster logic...\n", cluster.Provider)

	p, err := providers.GetProvider(cluster.Provider)
	if err != nil {
		return err
	}

	k3sScript := cluster.InstallScript
	k3sMirror := cluster.Mirror
	dockerMirror := cluster.DockerMirror

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

	masterExtraArgs := cluster.MasterExtraArgs
	workerExtraArgs := cluster.WorkerExtraArgs

	if cluster.DataStore != "" {
		masterExtraArgs += " --datastore-endpoint " + cluster.DataStore
	}

	if cluster.Network != "" {
		masterExtraArgs += fmt.Sprintf(" --flannel-backend=%s", cluster.Network)
	}

	if cluster.ClusterCIDR != "" {
		masterExtraArgs += " --cluster-cidr " + cluster.ClusterCIDR
	}

	logger.Infof("[%s] creating k3s master-%d...\n", cluster.Provider, 1)
	master0ExtraArgs := masterExtraArgs
	providerExtraArgs := p.GenerateMasterExtraArgs(cluster, cluster.MasterNodes[0])
	if providerExtraArgs != "" {
		master0ExtraArgs += providerExtraArgs
	}
	if err := initMaster(k3sScript, k3sMirror, dockerMirror, publicIP, master0ExtraArgs, cluster, cluster.MasterNodes[0]); err != nil {
		return err
	}
	logger.Infof("[%s] successfully created k3s master-%d\n", cluster.Provider, 1)

	for i, master := range cluster.MasterNodes {
		// skip first master nodes
		if i == 0 {
			continue
		}
		logger.Infof("[%s] creating k3s master-%d...\n", cluster.Provider, i+1)
		masterNExtraArgs := masterExtraArgs
		providerExtraArgs := p.GenerateMasterExtraArgs(cluster, master)
		if providerExtraArgs != "" {
			masterNExtraArgs += providerExtraArgs
		}
		if err := initAdditionalMaster(k3sScript, k3sMirror, dockerMirror, publicIP, masterNExtraArgs, cluster, master); err != nil {
			return err
		}
		logger.Infof("[%s] successfully created k3s master-%d\n", cluster.Provider, i+1)
	}

	workerErrChan := make(chan error)
	workerWaitGroupDone := make(chan bool)
	workerWaitGroup := &sync.WaitGroup{}
	workerWaitGroup.Add(len(cluster.WorkerNodes))

	for i, worker := range cluster.WorkerNodes {
		go func(i int, worker types.Node) {
			logger.Infof("[%s] creating k3s worker-%d...\n", cluster.Provider, i+1)
			extraArgs := workerExtraArgs
			providerExtraArgs := p.GenerateWorkerExtraArgs(cluster, worker)
			if providerExtraArgs != "" {
				extraArgs += providerExtraArgs
			}
			initWorker(workerWaitGroup, workerErrChan, k3sScript, k3sMirror, dockerMirror, extraArgs, cluster, worker)
			logger.Infof("[%s] successfully created k3s worker-%d\n", cluster.Provider, i+1)
		}(i, worker)
	}

	go func() {
		workerWaitGroup.Wait()
		close(workerWaitGroupDone)
	}()

	select {
	case <-workerWaitGroupDone:
		break
	case err := <-workerErrChan:
		return err
	}

	// get k3s cluster config.
	cfg, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, []string{catCfgCommand})
	if err != nil {
		return err
	}

	logger.Infof("[%s] deploying additional manifests\n", cluster.Provider)

	// deploy additional UI manifests.
	if cluster.UI {
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, []string{fmt.Sprintf(deployUICommand,
			base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(dashboardTmpl, cluster.Repo))), common.K3sManifestsDir)}); err != nil {
			return err
		}
	}

	logger.Infof("[%s] successfully deployed additional manifests\n", cluster.Provider)

	// merge current cluster to kube config.
	if err := SaveCfg(cfg, publicIP, cluster.Name); err != nil {
		return err
	}

	cluster.Status.Status = common.StatusRunning

	// write current cluster to state file.
	// native provider no need to operate .state file.
	if p.GetProviderName() != "native" {
		if err := SaveState(cluster); err != nil {
			return err
		}
	}

	logger.Infof("[%s] successfully executed init k3s cluster logic\n", cluster.Provider)
	return nil
}

func JoinK3sNode(merged, added *types.Cluster) error {
	logger = common.NewLogger(common.Debug)
	logger.Infof("[%s] executing join k3s node logic\n", merged.Provider)

	p, err := providers.GetProvider(merged.Provider)
	if err != nil {
		return err
	}
	k3sScript := merged.InstallScript
	k3sMirror := merged.Mirror
	dockerMirror := merged.DockerMirror

	if merged.IP == "" {
		if len(merged.MasterNodes) <= 0 || len(merged.MasterNodes[0].InternalIPAddress) <= 0 {
			return errors.New("[cluster] master node internal ip address can not be empty")
		}
		merged.IP = merged.MasterNodes[0].InternalIPAddress[0]
	}

	// get cluster token from `--ip` address.
	if merged.Token == "" {
		serverNode := types.Node{}
		if len(added.MasterNodes) > 0 {
			serverNode = added.MasterNodes[0]
		} else {
			serverNode = added.WorkerNodes[0]
		}
		serverNode.PublicIPAddress = []string{merged.IP}
		token, err := execute(&hosts.Host{Node: serverNode}, []string{getTokenCommand})
		if err != nil {
			return err
		}
		merged.Token = strings.TrimSpace(token)
	}

	if merged.Token == "" {
		return errors.New("[cluster] k3s token can not be empty")
	}

	errChan := make(chan error)
	waitGroupDone := make(chan bool)
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(len(added.WorkerNodes))

	for i := 0; i < len(added.Status.MasterNodes); i++ {
		for _, full := range merged.MasterNodes {
			extraArgs := merged.MasterExtraArgs
			if added.Status.MasterNodes[i].InstanceID == full.InstanceID {
				logger.Infof("[%s] joining k3s master-%d...\n", merged.Provider, i+1)
				additionalExtraArgs := p.GenerateMasterExtraArgs(added, full)
				if additionalExtraArgs != "" {
					extraArgs += additionalExtraArgs
				}
				if err := joinMaster(k3sScript, k3sMirror, dockerMirror, extraArgs, merged, full); err != nil {
					return err
				}
				logger.Infof("[%s] successfully joined k3s master-%d\n", merged.Provider, i+1)
				break
			}
		}
	}

	for i := 0; i < len(added.Status.WorkerNodes); i++ {
		for _, full := range merged.WorkerNodes {
			extraArgs := merged.WorkerExtraArgs
			if added.Status.WorkerNodes[i].InstanceID == full.InstanceID {
				go func(i int, full types.Node) {
					logger.Infof("[%s] joining k3s worker-%d...\n", merged.Provider, i+1)
					additionalExtraArgs := p.GenerateWorkerExtraArgs(added, full)
					if additionalExtraArgs != "" {
						extraArgs += additionalExtraArgs
					}
					joinWorker(waitGroup, errChan, k3sScript, k3sMirror, dockerMirror, extraArgs, merged, full)
					logger.Infof("[%s] successfully joined k3s worker-%d\n", merged.Provider, i+1)
				}(i, full)
				break
			}
		}
	}

	go func() {
		waitGroup.Wait()
		close(waitGroupDone)
	}()

	select {
	case <-waitGroupDone:
		break
	case err := <-errChan:
		return err
	}

	// sync master & worker numbers.
	merged.Master = strconv.Itoa(len(merged.MasterNodes))
	merged.Worker = strconv.Itoa(len(merged.WorkerNodes))

	// write current cluster to state file.
	// native provider no need to operate .state file.
	if p.GetProviderName() != "native" {
		if err := SaveState(merged); err != nil {
			return nil
		}
	}

	logger.Infof("[%s] successfully executed join k3s node logic\n", merged.Provider)
	return nil
}

func SSHK3sNode(ip string, cluster *types.Cluster, ssh *types.SSH) error {
	var node types.Node

	for _, n := range cluster.Status.MasterNodes {
		if n.PublicIPAddress[0] == ip {
			node = n
			break
		}
	}

	for _, n := range cluster.Status.WorkerNodes {
		if n.PublicIPAddress[0] == ip {
			node = n
			break
		}
	}

	if ssh.User != "" {
		node.SSH.User = ssh.User
	}
	if ssh.Port != "" {
		node.SSH.Port = ssh.Port
	}
	if ssh.Password != "" {
		node.SSH.Password = ssh.Password
	}
	if ssh.SSHKey != "" {
		node.SSH.SSHKey = ssh.SSHKey
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

	if node.SSH.Port == "" {
		node.SSH.Port = "22"
	}

	// preCheck ssh config
	if node.SSH.User == "" || (node.SSH.Password == "" && node.SSH.SSHKeyPath == "") {
		return fmt.Errorf("couldn't ssh to chosen node with current ssh config: --ssh-user %s --ssh-port %s --ssh-password %s --ssh-key-path %s", node.SSH.User, node.SSH.Port, node.SSH.Password, node.SSH.SSHKeyPath)
	}

	return terminal(&hosts.Host{Node: node})
}

func ReadFromState(cluster *types.Cluster) ([]types.Cluster, error) {
	r := make([]types.Cluster, 0)
	v := common.CfgPath
	if v == "" {
		return r, errors.New("[cluster] cfg path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return r, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return r, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s", err)
	}

	for _, c := range converts {
		name := cluster.Name

		switch cluster.Provider {
		case "alibaba":
			if option, ok := cluster.Options.(alibaba.Options); ok {
				name = fmt.Sprintf("%s.%s", cluster.Name, option.Region)
			}
		case "native":
			if _, ok := cluster.Options.(native.Options); ok {
				name = cluster.Name
			}
		case "tencent":
			if option, ok := cluster.Options.(tencent.Options); ok {
				name = fmt.Sprintf("%s.%s", cluster.Name, option.Region)
			}
		}

		if c.Provider == cluster.Provider && c.Name == name {
			r = append(r, c)
		}
	}

	return r, nil
}

func AppendToState(cluster *types.Cluster) ([]types.Cluster, error) {
	v := common.CfgPath
	if v == "" {
		return nil, errors.New("[cluster] cfg path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return nil, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return nil, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s", err)
	}

	index := -1

	for i, c := range converts {
		if c.Provider == cluster.Provider && c.Name == cluster.Name {
			index = i
		}
	}

	if index > -1 {
		converts[index] = *cluster
	} else {
		converts = append(converts, *cluster)
	}

	return converts, nil
}

func DeleteState(name string, provider string) error {
	r, err := deleteClusterFromState(name, provider)
	if err != nil {
		return err
	}

	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty")
	}

	return utils.WriteYaml(r, v, common.StateFile)
}

func UninstallK3sNodes(nodes []types.Node) (warnMsg []string) {
	for _, node := range nodes {
		if node.Master {
			_, e := execute(&hosts.Host{Node: node}, []string{masterUninstallCommand})
			if e != nil {
				warnMsg = append(warnMsg, fmt.Sprintf("failed to uninstall k3s on master node %s: %s", node.InstanceID, e.Error()))
			}
		} else {
			_, e := execute(&hosts.Host{Node: node}, []string{workerUninstallCommand})
			if e != nil {
				warnMsg = append(warnMsg, fmt.Sprintf("failed to uninstall k3s on worker node %s: %s", node.InstanceID, e.Error()))
			}
		}
	}

	return
}

func ConvertToClusters(origin []interface{}) ([]types.Cluster, error) {
	result := make([]types.Cluster, 0)

	b, err := yaml.Marshal(origin)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func SaveState(cluster *types.Cluster) error {
	r, err := AppendToState(cluster)
	if err != nil {
		return err
	}

	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty")
	}

	return utils.WriteYaml(r, v, common.StateFile)
}

func FilterState(r []*types.Cluster) error {
	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty")
	}

	return utils.WriteYaml(r, v, common.StateFile)
}

func SaveCfg(cfg, ip, context string) error {
	replacer := strings.NewReplacer(
		"127.0.0.1", ip,
		"localhost", ip,
		"default", context,
	)

	result := replacer.Replace(cfg)

	tempPath := fmt.Sprintf("%s/.kube", common.CfgPath)
	if err := utils.EnsureFolderExist(tempPath); err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp folder error, msg: %s", err)
	}

	temp, err := ioutil.TempFile(tempPath, common.KubeCfgTempName)
	if err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp file error, msg: %s", err)
	}
	defer func() {
		_ = temp.Close()
	}()

	err = utils.WriteBytesToYaml([]byte(result), tempPath, temp.Name()[strings.Index(temp.Name(), common.KubeCfgTempName):])
	if err != nil {
		return fmt.Errorf("[cluster] write content to kubecfg temp file error, msg: %s", err)
	}

	return mergeCfg(context, temp.Name())
}

func OverwriteCfg(context string) error {
	c, err := clientcmd.LoadFromFile(fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
	if err != nil {
		return err
	}

	delete(c.Clusters, context)
	delete(c.Contexts, context)
	delete(c.AuthInfos, context)
	// clear current context
	if c.CurrentContext == context {
		c.CurrentContext = ""
	}

	return clientcmd.WriteToFile(*c, fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
}

func DeployExtraManifest(cluster *types.Cluster, cmds []string) error {
	if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, cmds); err != nil {
		return err
	}
	return nil
}

func initMaster(k3sScript, k3sMirror, dockerMirror, ip, extraArgs string, cluster *types.Cluster, master types.Node) error {
	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: master}, []string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			return err
		}
	}

	if cluster.Registry != "" {
		if err := handleRegistry(&hosts.Host{Node: master}, cluster.Registry); err != nil {
			return err
		}
	}

	logger.Debugf("[cluster] k3s master command: %s\n", fmt.Sprintf(initCommand, k3sScript, k3sMirror, cluster.Token,
		"--cluster-init", ip, ip, strings.TrimSpace(extraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel)))

	if _, err := execute(&hosts.Host{Node: master}, []string{fmt.Sprintf(initCommand, k3sScript, k3sMirror,
		cluster.Token, "--cluster-init", ip, ip, strings.TrimSpace(extraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel))}); err != nil {
		return err
	}

	return nil
}

func initAdditionalMaster(k3sScript, k3sMirror, dockerMirror, ip, extraArgs string, cluster *types.Cluster, master types.Node) error {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: master}, []string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			return err
		}
	}

	if cluster.Registry != "" {
		if err := handleRegistry(&hosts.Host{Node: master}, cluster.Registry); err != nil {
			return err
		}
	}

	if !strings.Contains(extraArgs, "server --server") {
		sortedExtraArgs += fmt.Sprintf(" server --server %s --tls-san %s --node-external-ip %s", fmt.Sprintf("https://%s:6443", ip), master.PublicIPAddress[0], master.PublicIPAddress[0])
	}

	sortedExtraArgs += " " + extraArgs

	logger.Debugf("[cluster] k3s additional master command: %s\n", fmt.Sprintf(joinCommand, k3sScript, k3sMirror,
		ip, cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel)))

	if _, err := execute(&hosts.Host{Node: master}, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, ip,
		cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel))}); err != nil {
		return err
	}

	return nil
}

func initWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	cluster *types.Cluster, worker types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: worker}, []string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	if cluster.Registry != "" {
		if err := handleRegistry(&hosts.Host{Node: worker}, cluster.Registry); err != nil {
			errChan <- err
		}
	}
	sortedExtraArgs += fmt.Sprintf(" --node-external-ip %s", worker.PublicIPAddress[0])
	sortedExtraArgs += " " + extraArgs

	logger.Debugf("[cluster] k3s worker command: %s\n", fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.IP,
		cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel)))

	if _, err := execute(&hosts.Host{Node: worker}, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.IP,
		cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel))}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func joinMaster(k3sScript, k3sMirror, dockerMirror,
	extraArgs string, merged *types.Cluster, full types.Node) error {
	sortedExtraArgs := ""

	if !strings.Contains(extraArgs, "server --server") {
		sortedExtraArgs += fmt.Sprintf(" server --server %s --tls-san %s --node-external-ip %s", fmt.Sprintf("https://%s:6443", merged.IP), full.PublicIPAddress[0], full.PublicIPAddress[0])
	}

	if merged.DataStore != "" {
		sortedExtraArgs += " --datastore-endpoint " + merged.DataStore
	}

	if merged.ClusterCIDR != "" {
		sortedExtraArgs += " --cluster-cidr " + merged.ClusterCIDR
	}

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: full}, []string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			return err
		}
	}

	if merged.Registry != "" {
		if err := handleRegistry(&hosts.Host{Node: full}, merged.Registry); err != nil {
			return err
		}
	}

	sortedExtraArgs += " " + extraArgs

	logger.Debugf("[cluster] k3s master command: %s\n", fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel)))

	// for now, use the workerCommand to join the additional master server node.
	if _, err := execute(&hosts.Host{Node: full}, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel))}); err != nil {
		return err
	}

	return nil
}

func joinWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	merged *types.Cluster, full types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: full}, []string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	if merged.Registry != "" {
		if err := handleRegistry(&hosts.Host{Node: full}, merged.Registry); err != nil {
			errChan <- err
		}
	}

	sortedExtraArgs += fmt.Sprintf(" --node-external-ip %s", full.PublicIPAddress[0])
	sortedExtraArgs += " " + extraArgs

	logger.Debugf("[cluster] k3s worker command: %s\n", fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel)))

	if _, err := execute(&hosts.Host{Node: full}, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel))}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func execute(host *hosts.Host, cmds []string) (string, error) {
	if len(cmds) <= 0 {
		return "", nil
	}

	dialer, err := hosts.SSHDialer(host)
	if err != nil {
		return "", err
	}

	tunnel, err := dialer.OpenTunnel(true)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	for _, cmd := range cmds {
		tunnel.Cmd(cmd)
	}

	if err := tunnel.SetStdio(&stdout, &stderr).Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func terminal(host *hosts.Host) error {
	dialer, err := hosts.SSHDialer(host)
	if err != nil {
		return err
	}

	tunnel, err := dialer.OpenTunnel(false)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	return tunnel.Terminal()
}

func mergeCfg(context, right string) error {
	defer func() {
		if err := os.Remove(right); err != nil {
			logrus.Errorf("[cluster] remove kubecfg temp file error, msg: %s", err)
		}
	}()

	if err := utils.EnsureFileExist(common.CfgPath, common.KubeCfgFile); err != nil {
		return fmt.Errorf("[cluster] ensure kubecfg exist error, msg: %s", err)
	}

	if err := OverwriteCfg(context); err != nil {
		return fmt.Errorf("[cluster] overwrite kubecfg error, msg: %s", err)
	}

	if err := os.Setenv(clientcmd.RecommendedConfigPathEnvVar, fmt.Sprintf("%s:%s",
		fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile), right)); err != nil {
		return fmt.Errorf("[cluster] set env error when merging kubecfg, msg: %s", err)
	}

	out := &bytes.Buffer{}
	opt := config.ViewOptions{
		Flatten:      true,
		PrintFlags:   genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme).WithDefaultOutput("yaml"),
		ConfigAccess: clientcmd.NewDefaultPathOptions(),
		IOStreams:    genericclioptions.IOStreams{Out: out},
	}
	_ = opt.Merge.Set("true")

	printer, err := opt.PrintFlags.ToPrinter()
	if err != nil {
		return fmt.Errorf("[cluster] generate view options error, msg: %s", err)
	}
	opt.PrintObject = printer.PrintObj

	if err := opt.Run(); err != nil {
		return fmt.Errorf("[cluster] merging kubecfg error, msg: %s", err)
	}

	return utils.WriteBytesToYaml(out.Bytes(), common.CfgPath, common.KubeCfgFile)
}

func deleteClusterFromState(name string, provider string) ([]types.Cluster, error) {
	v := common.CfgPath
	if v == "" {
		return nil, errors.New("[cluster] cfg path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return nil, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return nil, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s", err)
	}

	index := -1

	for i, c := range converts {
		if c.Provider == provider && c.Name == name {
			index = i
		}
	}

	if index > -1 {
		converts = append(converts[:index], converts[index+1:]...)
	} else {
		return nil, fmt.Errorf("[cluster] was not found in the .state file")
	}

	return converts, nil
}

func genK3sVersion(version, channel string) string {
	if version != "" {
		return fmt.Sprintf("INSTALL_K3S_VERSION='%s'", version)
	}
	return fmt.Sprintf("INSTALL_K3S_CHANNEL='%s'", channel)
}

func handleRegistry(host *hosts.Host, file string) error {
	registry, err := unmarshalRegistryFile(file)
	if err != nil {
		return err
	}

	tls, err := registryTLSMap(registry)
	if err != nil {
		return err
	}

	registry, cmd, err := saveRegistryTLS(registry, tls)
	if err != nil {
		return err
	}

	registryContent, err := registryToString(registry)
	if err != nil {
		return err
	}

	cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | sudo tee \"/etc/rancher/k3s/registries.yaml\"",
		base64.StdEncoding.EncodeToString([]byte(registryContent))))
	_, err = execute(host, cmd)
	return err
}

func unmarshalRegistryFile(file string) (*templates.Registry, error) {
	registry := &templates.Registry{}
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, err
	}

	if len(b) == 0 {
		return nil, fmt.Errorf("registry file %s is empty", file)
	}

	err = yamlv3.Unmarshal(b, registry)
	if err != nil {
		return nil, err
	}

	return registry, nil
}

func registryTLSMap(registry *templates.Registry) (m map[string]map[string][]byte, err error) {
	m = make(map[string]map[string][]byte)
	if registry == nil {
		err = fmt.Errorf("registry is nil")
		return
	}

	for r, c := range registry.Configs {
		if _, ok := m[r]; !ok {
			m[r] = map[string][]byte{}
		}
		if c.TLS.CertFile != "" {
			b, err := ioutil.ReadFile(c.TLS.CertFile)
			if err != nil {
				return m, err
			}
			m[r]["cert"] = b
		}
		if c.TLS.KeyFile != "" {
			b, err := ioutil.ReadFile(c.TLS.KeyFile)
			if err != nil {
				return m, err
			}
			m[r]["key"] = b
		}
		if c.TLS.CAFile != "" {
			b, err := ioutil.ReadFile(c.TLS.CAFile)
			if err != nil {
				return m, err
			}
			m[r]["ca"] = b
		}
	}

	return
}

func saveRegistryTLS(registry *templates.Registry, m map[string]map[string][]byte) (*templates.Registry, []string, error) {
	cmd := make([]string, 0)
	if m == nil || len(m) == 0 {
		return nil, cmd, fmt.Errorf("registry map is nil")
	}

	for r, c := range m {
		if r != "" {
			if _, ok := registry.Configs[r]; !ok {
				return nil, cmd, fmt.Errorf("registry map is not match the struct: %s", r)
			}

			// e.g /etc/rancher/k3s/mycustomreg:5000/
			path := fmt.Sprintf("/etc/rancher/k3s/%s", r)
			cmd = append(cmd, fmt.Sprintf("sudo mkdir -p %s", path))

			for f, b := range c {
				// e.g /etc/rancher/k3s/mycustomreg:5000/{ca,key,cert}
				file := fmt.Sprintf("%s/%s", path, f)
				cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | sudo tee \"%s\"", base64.StdEncoding.EncodeToString(b), file))
				cmd = append(cmd, fmt.Sprintf("sudo chmod 755 %s", file))

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

	return registry, cmd, nil
}

func registryToString(registry *templates.Registry) (string, error) {
	if registry == nil {
		return "", fmt.Errorf("can't save registry file: registry is nil")
	}
	b, err := yamlv3.Marshal(registry)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

func GetClusterConfig(name, kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := buildConfigFromFlags(name, kubeconfig)
	if err != nil {
		return nil, err
	}
	config.Timeout = 20 * time.Second
	c, err := kubernetes.NewForConfig(config)
	return c, err
}

func GetClusterStatus(c *kubernetes.Clientset) string {
	_, err := c.RESTClient().Get().RequestURI("/readyz").DoRaw(context.TODO())
	if err != nil {
		return types.ClusterStatusStopped
	}
	return types.ClusterStatusRunning
}

func GetClusterVersion(c *kubernetes.Clientset) string {
	v, err := c.DiscoveryClient.ServerVersion()
	if err != nil {
		return types.ClusterStatusUnknown
	}
	return v.GitVersion
}

func DescribeClusterNodes(client *kubernetes.Clientset) ([]types.ClusterNode, error) {
	// list cluster nodes
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil || nodeList == nil {
		return nil, err
	}
	nodes := []types.ClusterNode{}
	for _, n := range nodeList.Items {
		node := types.ClusterNode{
			ContainerRuntimeVersion: n.Status.NodeInfo.ContainerRuntimeVersion,
			Version:                 n.Status.NodeInfo.KubeletVersion,
		}
		// get address
		addressList := n.Status.Addresses
		for _, address := range addressList {
			switch address.Type {
			case v1.NodeHostName:
				node.HostName = address.Address
			case v1.NodeInternalIP:
				node.InternalIP = address.Address
			case v1.NodeExternalIP:
				node.ExternalIP = address.Address
			default:
				continue
			}
		}
		// get roles
		labels := n.Labels
		_, ok := labels[LabelNodeRoleMaster]
		node.Master = ok
		roles := []string{}
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
		node.Roles = strings.Join(roles, ",")
		// get status
		conditions := n.Status.Conditions
		for _, c := range conditions {
			if c.Type == v1.NodeReady {
				if c.Status == v1.ConditionTrue {
					node.Status = "Ready"
				} else {
					node.Status = "NotReady"
				}
				break
			}
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}
