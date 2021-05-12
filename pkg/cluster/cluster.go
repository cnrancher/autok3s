package cluster

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/rancher/k3s/pkg/agent/templates"
	"github.com/sirupsen/logrus"
	yamlv3 "gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	initCommand            = "curl -sLS %s | %s K3S_TOKEN='%s' INSTALL_K3S_EXEC='server %s --node-external-ip %s %s' %s sh -"
	joinCommand            = "curl -sLS %s | %s K3S_URL='https://%s:6443' K3S_TOKEN='%s' INSTALL_K3S_EXEC='%s' %s sh -"
	getTokenCommand        = "sudo cat /var/lib/rancher/k3s/server/node-token"
	catCfgCommand          = "sudo cat /etc/rancher/k3s/k3s.yaml"
	dockerCommand          = "if ! type docker; then curl -sSL %s | sh - %s; fi"
	deployUICommand        = "echo \"%s\" | base64 -d | sudo tee \"%s/ui.yaml\""
	masterUninstallCommand = "sh /usr/local/bin/k3s-uninstall.sh"
	workerUninstallCommand = "sh /usr/local/bin/k3s-agent-uninstall.sh"
	registryPath           = "/etc/rancher/k3s"
)

func (p *ProviderBase) InitK3sCluster(cluster *types.Cluster) error {
	p.Logger.Infof("[%s] executing init k3s cluster logic...", p.Provider)

	provider, err := providers.GetProvider(p.Provider)
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

	// append tls-sans to k3s install script:
	// 1. appends from --tls-sans flags.
	// 2. appends all master nodes' first public address.
	var tlsSans string
	p.TLSSans = append(p.TLSSans, publicIP)
	for _, master := range cluster.MasterNodes {
		if master.PublicIPAddress[0] != "" && master.PublicIPAddress[0] != publicIP {
			p.TLSSans = append(p.TLSSans, master.PublicIPAddress[0])
		}
	}
	for _, tlsSan := range p.TLSSans {
		tlsSans = tlsSans + fmt.Sprintf(" --tls-san %s", tlsSan)
	}
	// save p.TlsSans to db.
	cluster.TLSSans = p.TLSSans

	masterExtraArgs := cluster.MasterExtraArgs
	workerExtraArgs := cluster.WorkerExtraArgs

	if cluster.DataStore != "" {
		cluster.Cluster = false
		masterExtraArgs += " --datastore-endpoint " + cluster.DataStore
	}

	if cluster.Network != "" {
		masterExtraArgs += fmt.Sprintf(" --flannel-backend=%s", cluster.Network)
	}

	if cluster.ClusterCidr != "" {
		masterExtraArgs += " --cluster-cidr " + cluster.ClusterCidr
	}

	p.Logger.Infof("[%s] creating k3s master-%d...", p.Provider, 1)
	master0ExtraArgs := masterExtraArgs
	providerExtraArgs := provider.GenerateMasterExtraArgs(cluster, cluster.MasterNodes[0])
	if providerExtraArgs != "" {
		master0ExtraArgs += providerExtraArgs
	}
	if cluster.Cluster {
		master0ExtraArgs += " --cluster-init"
	}

	if err := p.initMaster(k3sScript, k3sMirror, dockerMirror, tlsSans, publicIP, master0ExtraArgs, cluster, cluster.MasterNodes[0]); err != nil {
		return err
	}
	p.Logger.Infof("[%s] successfully created k3s master-%d", p.Provider, 1)

	for i, master := range cluster.MasterNodes {
		// skip first master nodes.
		if i == 0 {
			continue
		}
		p.Logger.Infof("[%s] creating k3s master-%d...", p.Provider, i+1)
		masterNExtraArgs := masterExtraArgs
		providerExtraArgs := provider.GenerateMasterExtraArgs(cluster, master)
		if providerExtraArgs != "" {
			masterNExtraArgs += providerExtraArgs
		}
		if err := p.initAdditionalMaster(k3sScript, k3sMirror, dockerMirror, tlsSans, publicIP, masterNExtraArgs, cluster, master); err != nil {
			return err
		}
		p.Logger.Infof("[%s] successfully created k3s master-%d", p.Provider, i+1)
	}

	workerErrChan := make(chan error)
	workerWaitGroupDone := make(chan bool)
	workerWaitGroup := &sync.WaitGroup{}
	workerWaitGroup.Add(len(cluster.WorkerNodes))

	for i, worker := range cluster.WorkerNodes {
		go func(i int, worker types.Node) {
			p.Logger.Infof("[%s] creating k3s worker-%d...", p.Provider, i+1)
			extraArgs := workerExtraArgs
			providerExtraArgs := provider.GenerateWorkerExtraArgs(cluster, worker)
			if providerExtraArgs != "" {
				extraArgs += providerExtraArgs
			}
			p.initWorker(workerWaitGroup, workerErrChan, k3sScript, k3sMirror, dockerMirror, extraArgs, cluster, worker)
			p.Logger.Infof("[%s] successfully created k3s worker-%d", p.Provider, i+1)
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
	cfg, err := p.execute(&cluster.MasterNodes[0], []string{catCfgCommand})
	if err != nil {
		return err
	}

	p.Logger.Infof("[%s] deploying additional manifests", p.Provider)

	// deploy additional UI manifests.
	if cluster.UI {
		if _, err := p.execute(&cluster.MasterNodes[0], []string{fmt.Sprintf(deployUICommand,
			base64.StdEncoding.EncodeToString([]byte(dashboardTmpl)), common.K3sManifestsDir)}); err != nil {
			return err
		}
	}

	p.Logger.Infof("[%s] successfully deployed additional manifests", p.Provider)

	// merge current cluster to kube config.
	if err := SaveCfg(cfg, publicIP, cluster.ContextName); err != nil {
		return err
	}
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
	cluster.Status.Status = common.StatusRunning

	// write current cluster to state file.
	// native provider no need to operate .state file.
	if p.Provider != "native" {
		if err := common.DefaultDB.SaveCluster(cluster); err != nil {
			return err
		}
	}

	p.Logger.Infof("[%s] successfully executed init k3s cluster logic", p.Provider)
	return nil
}

func (p *ProviderBase) Join(merged, added *types.Cluster) error {
	p.Logger.Infof("[%s] executing join k3s node logic", merged.Provider)

	provider, err := providers.GetProvider(merged.Provider)
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
		token, err := p.execute(&serverNode, []string{getTokenCommand})
		if err != nil {
			return err
		}
		merged.Token = strings.TrimSpace(token)
	}

	if merged.Token == "" {
		return errors.New("[cluster] k3s token can not be empty")
	}

	// append tls-sans to k3s install script:
	// 1. appends from --tls-sans flags.
	// 2. appends all master nodes' first public address.
	var tlsSans string
	for _, master := range added.MasterNodes {
		if master.PublicIPAddress[0] != "" {
			merged.TLSSans = append(merged.TLSSans, master.PublicIPAddress[0])
		}
	}
	for _, tlsSan := range merged.TLSSans {
		tlsSans = tlsSans + fmt.Sprintf(" --tls-san %s", tlsSan)
	}

	errChan := make(chan error)
	waitGroupDone := make(chan bool)
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(len(added.WorkerNodes))

	for i := 0; i < len(added.Status.MasterNodes); i++ {
		for _, full := range merged.MasterNodes {
			extraArgs := merged.MasterExtraArgs
			if added.Status.MasterNodes[i].InstanceID == full.InstanceID {
				p.Logger.Infof("[%s] joining k3s master-%d...", merged.Provider, i+1)
				additionalExtraArgs := provider.GenerateMasterExtraArgs(added, full)
				if additionalExtraArgs != "" {
					extraArgs += additionalExtraArgs
				}
				if err := p.joinMaster(k3sScript, k3sMirror, dockerMirror, extraArgs, tlsSans, merged, full); err != nil {
					return err
				}
				p.Logger.Infof("[%s] successfully joined k3s master-%d", merged.Provider, i+1)
				break
			}
		}
	}

	for i := 0; i < len(added.Status.WorkerNodes); i++ {
		for _, full := range merged.WorkerNodes {
			extraArgs := merged.WorkerExtraArgs
			if added.Status.WorkerNodes[i].InstanceID == full.InstanceID {
				go func(i int, full types.Node) {
					p.Logger.Infof("[%s] joining k3s worker-%d...", merged.Provider, i+1)
					additionalExtraArgs := provider.GenerateWorkerExtraArgs(added, full)
					if additionalExtraArgs != "" {
						extraArgs += additionalExtraArgs
					}
					p.joinWorker(waitGroup, errChan, k3sScript, k3sMirror, dockerMirror, extraArgs, merged, full)
					p.Logger.Infof("[%s] successfully joined k3s worker-%d", merged.Provider, i+1)
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
	merged.Status.Status = common.StatusRunning
	// write current cluster to state file.
	// native provider no need to operate .state file.
	if p.Provider != "native" {
		if err = common.DefaultDB.SaveCluster(merged); err != nil {
			p.Logger.Errorf("failed to save cluster state: %v", err)
			return nil
		}
	}

	p.Logger.Infof("[%s] successfully executed join k3s node logic", merged.Provider)
	return nil
}

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

func (p *ProviderBase) UninstallK3sNodes(nodes []types.Node) (warnMsg []string) {
	for _, node := range nodes {
		if node.Master {
			_, e := p.execute(&node, []string{masterUninstallCommand})
			if e != nil {
				warnMsg = append(warnMsg, fmt.Sprintf("failed to uninstall k3s on master node %s: %s", node.InstanceID, e.Error()))
			}
		} else {
			_, e := p.execute(&node, []string{workerUninstallCommand})
			if e != nil {
				warnMsg = append(warnMsg, fmt.Sprintf("failed to uninstall k3s on worker node %s: %s", node.InstanceID, e.Error()))
			}
		}
	}

	return
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

	absPath, _ := filepath.Abs(temp.Name())
	if err = ioutil.WriteFile(absPath, []byte(result), 0600); err != nil {
		return fmt.Errorf("[cluster] write content to kubecfg temp file error: %v", err)
	}

	return mergeCfg(context, temp.Name())
}

func OverwriteCfg(context string) error {
	path := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, path)
	fMgr := &common.ConfigFileManager{}
	return fMgr.OverwriteCfg(path, context, fMgr.RemoveCfg)
}

func (p *ProviderBase) DeployExtraManifest(cluster *types.Cluster, cmds []string) error {
	if _, err := p.execute(&cluster.MasterNodes[0], cmds); err != nil {
		return err
	}
	return nil
}

func (p *ProviderBase) initMaster(k3sScript, k3sMirror, dockerMirror, tlsSans, ip, extraArgs string, cluster *types.Cluster, master types.Node) error {
	if strings.Contains(extraArgs, "--docker") {
		p.Logger.Infof("[cluster] install docker command %s", fmt.Sprintf(dockerCommand, cluster.DockerScript, dockerMirror))
		if _, err := p.execute(&master, []string{fmt.Sprintf(dockerCommand, cluster.DockerScript, dockerMirror)}); err != nil {
			return err
		}
	}

	if cluster.Registry != "" || cluster.RegistryContent != "" {
		if err := p.handleRegistry(&master, cluster); err != nil {
			return err
		}
	}

	p.Logger.Infof("[cluster] k3s master command: %s", fmt.Sprintf(initCommand, k3sScript, k3sMirror, cluster.Token,
		tlsSans, ip, strings.TrimSpace(extraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel)))

	if _, err := p.execute(&master, []string{fmt.Sprintf(initCommand, k3sScript, k3sMirror,
		cluster.Token, tlsSans, ip, strings.TrimSpace(extraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel))}); err != nil {
		return err
	}

	return nil
}

func (p *ProviderBase) initAdditionalMaster(k3sScript, k3sMirror, dockerMirror, tlsSans, ip, extraArgs string, cluster *types.Cluster, master types.Node) error {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := p.execute(&master, []string{fmt.Sprintf(dockerCommand, cluster.DockerScript, dockerMirror)}); err != nil {
			return err
		}
	}

	if cluster.Registry != "" || cluster.RegistryContent != "" {
		if err := p.handleRegistry(&master, cluster); err != nil {
			return err
		}
	}

	if !strings.Contains(extraArgs, "server --server") {
		sortedExtraArgs += fmt.Sprintf(" server --server %s %s --node-external-ip %s", fmt.Sprintf("https://%s:6443", ip), tlsSans, master.PublicIPAddress[0])
	}

	sortedExtraArgs += " " + extraArgs

	p.Logger.Infof("[cluster] k3s additional master command: %s", fmt.Sprintf(joinCommand, k3sScript, k3sMirror,
		ip, cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel)))

	if _, err := p.execute(&master, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, ip,
		cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel))}); err != nil {
		return err
	}

	return nil
}

func (p *ProviderBase) initWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	cluster *types.Cluster, worker types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := p.execute(&worker, []string{fmt.Sprintf(dockerCommand, cluster.DockerScript, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	if cluster.Registry != "" || cluster.RegistryContent != "" {
		if err := p.handleRegistry(&worker, cluster); err != nil {
			errChan <- err
		}
	}

	sortedExtraArgs += fmt.Sprintf(" --node-external-ip %s", worker.PublicIPAddress[0])
	sortedExtraArgs += " " + extraArgs

	p.Logger.Infof("[cluster] k3s worker command: %s", fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.IP,
		cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel)))

	if _, err := p.execute(&worker, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.IP,
		cluster.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(cluster.K3sVersion, cluster.K3sChannel))}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func (p *ProviderBase) joinMaster(k3sScript, k3sMirror, dockerMirror,
	extraArgs, tlsSans string, merged *types.Cluster, full types.Node) error {
	sortedExtraArgs := ""

	if !strings.Contains(extraArgs, "server --server") {
		sortedExtraArgs += fmt.Sprintf(" server --server %s %s --node-external-ip %s", fmt.Sprintf("https://%s:6443", merged.IP), tlsSans, full.PublicIPAddress[0])
	}

	if merged.DataStore != "" {
		sortedExtraArgs += " --datastore-endpoint " + merged.DataStore
	}

	if merged.ClusterCidr != "" {
		sortedExtraArgs += " --cluster-cidr " + merged.ClusterCidr
	}

	if strings.Contains(extraArgs, "--docker") {
		if _, err := p.execute(&full, []string{fmt.Sprintf(dockerCommand, merged.DockerScript, dockerMirror)}); err != nil {
			return err
		}
	}

	if merged.Registry != "" || merged.RegistryContent != "" {
		if err := p.handleRegistry(&full, merged); err != nil {
			return err
		}
	}

	sortedExtraArgs += " " + extraArgs

	p.Logger.Infof("[cluster] k3s master command: %s", fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel)))

	// for now, use the workerCommand to join the additional master server node.
	if _, err := p.execute(&full, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel))}); err != nil {
		return err
	}

	return nil
}

func (p *ProviderBase) joinWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	merged *types.Cluster, full types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := p.execute(&full, []string{fmt.Sprintf(dockerCommand, merged.DockerScript, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	if merged.Registry != "" || merged.RegistryContent != "" {
		if err := p.handleRegistry(&full, merged); err != nil {
			errChan <- err
		}
	}

	sortedExtraArgs += fmt.Sprintf(" --node-external-ip %s", full.PublicIPAddress[0])
	sortedExtraArgs += " " + extraArgs

	p.Logger.Infof("[cluster] k3s worker command: %s", fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel)))

	if _, err := p.execute(&full, []string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.IP,
		merged.Token, strings.TrimSpace(sortedExtraArgs), genK3sVersion(merged.K3sVersion, merged.K3sChannel))}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func (p *ProviderBase) execute(n *types.Node, cmds []string) (string, error) {
	if len(cmds) <= 0 {
		return "", nil
	}

	dialer, err := hosts.NewSSHDialer(n, true)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = dialer.Close()
	}()

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	dialer.SetStdio(&stdout, &stderr, nil).SetWriter(p.Logger.Out)

	for _, cmd := range cmds {
		dialer.Cmd(cmd)
	}

	if err := dialer.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func terminal(n *types.Node) error {
	dialer, err := hosts.NewSSHDialer(n, true)
	if err != nil {
		return err
	}

	defer func() {
		_ = dialer.Close()
	}()

	dialer.SetStdio(os.Stdout, os.Stderr, os.Stdin)

	return dialer.Terminal()
}

func mergeCfg(context, tempFile string) error {
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			logrus.Errorf("[cluster] remove kubecfg temp file error, msg: %s", err)
		}
		_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
	}()
	kubeConfigPath := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeConfigPath)
	fMgr := &common.ConfigFileManager{}
	_ = fMgr.OverwriteCfg(kubeConfigPath, context, fMgr.RemoveCfg)
	mergeKubeConfigENV := fmt.Sprintf("%s:%s", kubeConfigPath, tempFile)
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, mergeKubeConfigENV)
	return fMgr.OverwriteCfg(fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile), context, fMgr.MergeCfg)
}

func genK3sVersion(version, channel string) string {
	if version != "" {
		return fmt.Sprintf("INSTALL_K3S_VERSION='%s'", version)
	}
	return fmt.Sprintf("INSTALL_K3S_CHANNEL='%s'", channel)
}

func (p *ProviderBase) handleRegistry(n *types.Node, c *types.Cluster) (err error) {
	if c.Registry == "" && c.RegistryContent == "" {
		return nil
	}
	cmd := make([]string, 0)
	cmd = append(cmd, fmt.Sprintf("sudo mkdir -p %s", registryPath))
	var registry *templates.Registry
	if c.Registry != "" {
		registry, err = unmarshalRegistryFile(c.Registry)
		if err != nil {
			return err
		}
	} else if c.RegistryContent != "" {
		registry = &templates.Registry{}
		err = yamlv3.Unmarshal([]byte(c.RegistryContent), registry)
		if err != nil {
			return err
		}
	}

	tls, err := registryTLSMap(registry)
	if err != nil {
		return err
	}

	if tls != nil && len(tls) > 0 {
		registry, cmd, err = saveRegistryTLS(registry, tls)
		if err != nil {
			return err
		}
	}

	registryContent, err := registryToString(registry)
	if err != nil {
		return err
	}

	cmd = append(cmd, fmt.Sprintf("echo \"%s\" | base64 -d | sudo tee \"/etc/rancher/k3s/registries.yaml\"",
		base64.StdEncoding.EncodeToString([]byte(registryContent))))
	_, err = p.execute(n, cmd)
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
		if c.TLS == nil {
			continue
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
	for r, c := range m {
		if r != "" {
			if _, ok := registry.Configs[r]; !ok {
				return nil, cmd, fmt.Errorf("registry map is not match the struct: %s", r)
			}

			// i.e /etc/rancher/k3s/mycustomreg:5000/.
			path := fmt.Sprintf("/etc/rancher/k3s/%s", r)
			cmd = append(cmd, fmt.Sprintf("sudo mkdir -p %s", path))

			for f, b := range c {
				// i.e /etc/rancher/k3s/mycustomreg:5000/{ca,key,cert}.
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
	config.Timeout = 15 * time.Second
	c, err := kubernetes.NewForConfig(config)
	return c, err
}

func GetClusterStatus(c *kubernetes.Clientset) string {
	_, err := c.RESTClient().Get().Timeout(15 * time.Second).RequestURI("/readyz").DoRaw(context.TODO())
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

func DescribeClusterNodes(client *kubernetes.Clientset, instanceNodes []types.ClusterNode) ([]types.ClusterNode, error) {
	// list cluster nodes.
	timeout := int64(5 * time.Second)
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{TimeoutSeconds: &timeout})
	if err != nil || nodeList == nil {
		return nil, err
	}
	for _, node := range nodeList.Items {
		var internalIP, hostName string
		addressList := node.Status.Addresses
		for _, address := range addressList {
			switch address.Type {
			case v1.NodeInternalIP:
				internalIP = address.Address
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
