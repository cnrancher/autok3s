package cluster

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	k3sVersion        = ""
	k3sChannel        = "stable"
	k3sInstallScript  = "https://get.k3s.io"
	master            = "0"
	worker            = "0"
	ui                = false
	embedEtcd         = false
	defaultCidr       = "10.42.0.0/16"
	uploadManifestCmd = "echo \"%s\" | base64 -d | sudo tee \"%s/%s\""
)

type ProviderBase struct {
	types.Metadata `json:",inline"`
	types.Status   `json:"status"`
	types.SSH      `json:",inline"`
	M              *sync.Map
	Logger         *logrus.Logger
	Callbacks      map[string]*providerProcess
}

type providerProcess struct {
	ContextName string
	Event       string
	Fn          func(interface{})
}

func NewBaseProvider() *ProviderBase {
	return &ProviderBase{
		Metadata: types.Metadata{
			UI:            ui,
			K3sVersion:    k3sVersion,
			K3sChannel:    k3sChannel,
			InstallScript: k3sInstallScript,
			Cluster:       embedEtcd,
			Master:        master,
			Worker:        worker,
			ClusterCidr:   defaultCidr,
		},
		Status: types.Status{
			MasterNodes: make([]types.Node, 0),
			WorkerNodes: make([]types.Node, 0),
		},
		SSH: types.SSH{
			SSHPort: "22",
		},
		M: new(syncmap.Map),
	}
}

func (p *ProviderBase) GetCreateOptions() []types.Flag {
	return []types.Flag{
		{
			Name:  "ui",
			P:     &p.UI,
			V:     p.UI,
			Usage: "Enable K3s UI(kubernetes/dashboard). For how to login to UI, please see: https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md",
		},
		{
			Name:  "cluster",
			P:     &p.Cluster,
			V:     p.Cluster,
			Usage: "Form k3s cluster using embedded etcd (requires K8s >= 1.19), see: https://rancher.com/docs/k3s/latest/en/installation/ha-embedded/",
		},
		{
			Name:  "manifests",
			P:     &p.Manifests,
			V:     p.Manifests,
			Usage: "A folder path for multiple manifest files(only support one directory) or a manifest file path. Auto-deploying manifests to K3s which is a manner similar to `kubectl apply`",
		},
	}
}

func (p *ProviderBase) GetClusterOptions() []types.Flag {
	fs := []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
		{
			Name:  "k3s-version",
			P:     &p.K3sVersion,
			V:     p.K3sVersion,
			Usage: "Used to specify the version of k3s cluster, overrides k3s-channel",
		},
		{
			Name:  "k3s-channel",
			P:     &p.K3sChannel,
			V:     p.K3sChannel,
			Usage: "Channel to use for fetching K3s download URL. Defaults to “stable”. Options include: stable, latest, testing",
		},
		{
			Name:  "k3s-install-script",
			P:     &p.InstallScript,
			V:     p.InstallScript,
			Usage: "Change the default upstream k3s install script address, see: https://rancher.com/docs/k3s/latest/en/installation/install-options/#options-for-installation-with-script",
		},
		{
			Name:  "k3s-install-mirror",
			P:     &p.Mirror,
			V:     p.Mirror,
			Usage: "For Chinese users, set INSTALL_K3S_MIRROR=cn to use the mirror address to accelerate k3s binary file download",
		},
		{
			Name:  "master-extra-args",
			P:     &p.MasterExtraArgs,
			V:     p.MasterExtraArgs,
			Usage: "Master extra arguments for k3s installer, wrapped in quotes. e.g.(--master-extra-args '--no-deploy metrics-server'), for more information, please see: https://rancher.com/docs/k3s/latest/en/installation/install-options/server-config/",
		},
		{
			Name:  "worker-extra-args",
			P:     &p.WorkerExtraArgs,
			V:     p.WorkerExtraArgs,
			Usage: "Worker extra arguments for k3s installer, wrapped in quotes. e.g.(--worker-extra-args '--node-taint key=value:NoExecute'), for more information, please see: https://rancher.com/docs/k3s/latest/en/installation/install-options/agent-config/",
		},
		{
			Name:  "registry",
			P:     &p.Registry,
			V:     p.Registry,
			Usage: "K3s registry file, see: https://rancher.com/docs/k3s/latest/en/installation/private-registry",
		},
		{
			Name:  "datastore",
			P:     &p.DataStore,
			V:     p.DataStore,
			Usage: "K3s datastore endpoint, Specify etcd, Mysql, Postgres, or Sqlite (default) data source name, see: https://rancher.com/docs/k3s/latest/en/installation/install-options/server-config/#database",
		},
		{
			Name:  "token",
			P:     &p.Token,
			V:     p.Token,
			Usage: "K3s token, if empty will automatically generated, see: https://rancher.com/docs/k3s/latest/en/installation/install-options/server-config/#cluster-options",
		},
		{
			Name:  "tls-sans",
			P:     &p.TLSSans,
			V:     p.TLSSans,
			Usage: "Add additional hostnames or IPv4/IPv6 addresses as Subject Alternative Names on the server TLS cert, e.g.(--tls-sans 192.168.1.10 --tls-sans 192.168.2.10)",
		},
		{
			Name:  "master",
			P:     &p.Master,
			V:     p.Master,
			Usage: "Number of master node",
		},
		{
			Name:  "worker",
			P:     &p.Worker,
			V:     p.Worker,
			Usage: "Number of worker node",
		},
	}

	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

func (p *ProviderBase) GetSSHOptions() []types.Flag {
	return []types.Flag{
		{
			Name:  "ssh-user",
			P:     &p.SSHUser,
			V:     p.SSHUser,
			Usage: "SSH user for host",
		},
		{
			Name:  "ssh-port",
			P:     &p.SSHPort,
			V:     p.SSHPort,
			Usage: "SSH port for host",
		},
		{
			Name:  "ssh-key-path",
			P:     &p.SSHKeyPath,
			V:     p.SSHKeyPath,
			Usage: "SSH private key path",
		},
		{
			Name:  "ssh-key-passphrase",
			P:     &p.SSHKeyPassphrase,
			V:     p.SSHKeyPassphrase,
			Usage: "SSH passphrase of private key",
		},
		{
			Name:  "ssh-cert-path",
			P:     &p.SSHCertPath,
			V:     p.SSHCertPath,
			Usage: "SSH private key certificate path",
		},
		{
			Name:  "ssh-password",
			P:     &p.SSHPassword,
			V:     p.SSHPassword,
			Usage: "SSH login password",
		},
		{
			Name:  "ssh-agent-auth",
			P:     &p.SSHAgentAuth,
			V:     p.SSHAgentAuth,
			Usage: "Enable ssh agent",
		},
	}
}

func (p *ProviderBase) GetCommonConfig(sshFunc func() *types.SSH) (map[string]schemas.Field, error) {
	ssh := sshFunc()
	sshConfig, err := utils.ConvertToFields(*ssh)
	if err != nil {
		return nil, err
	}
	metaConfig, err := utils.ConvertToFields(p.Metadata)
	if err != nil {
		return nil, err
	}
	for k, v := range sshConfig {
		metaConfig[k] = v
	}
	return metaConfig, nil
}

func (p *ProviderBase) InitCluster(options interface{}, deployPlugins func() []string,
	cloudInstanceFunc func(ssh *types.SSH) (*types.Cluster, error), customInstallK3s func() (string, string, error), rollbackInstance func(ids []string) error) error {
	logFile, err := common.GetLogFile(p.ContextName)
	if err != nil {
		return err
	}

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  options,
		SSH:      p.SSH,
		Status:   p.Status,
	}
	defer func() {
		if err != nil {
			p.Logger.Errorf("%v", err)
			// save failed status.
			if c == nil {
				c = &types.Cluster{
					Metadata: p.Metadata,
					Options:  options,
					SSH:      p.SSH,
					Status:   types.Status{},
				}
			}
			c.Status.Status = common.StatusFailed
			_ = common.DefaultDB.SaveCluster(c)
			_ = p.RollbackCluster(rollbackInstance)
		}
		if err == nil && len(p.Status.MasterNodes) > 0 {
			p.Logger.Info(common.UsageInfoTitle)
			p.Logger.Infof(common.UsageContext, p.ContextName)
			p.Logger.Info(common.UsagePods)
		}
		_ = logFile.Close()
		if p.Callbacks != nil {
			if process, ok := p.Callbacks[p.ContextName]; ok && process.Event == "create" {
				logEvent := &common.LogEvent{
					Name:        process.Event,
					ContextName: p.ContextName,
				}
				process.Fn(logEvent)
			}
		}
	}()
	p.Logger = common.NewLogger(common.Debug, logFile)
	p.Logger.Infof("[%s] executing create logic...", p.Provider)
	c.Status.Status = common.StatusCreating
	// save cluster.
	err = common.DefaultDB.SaveCluster(c)
	if err != nil {
		return err
	}

	c, err = cloudInstanceFunc(&p.SSH)
	if err != nil {
		return err
	}
	p.syncExistNodes()
	c.Status = p.Status

	if customInstallK3s == nil {
		// use install scripts to initialize K3s cluster.
		if err = p.InitK3sCluster(c); err != nil {
			return err
		}
	} else {
		// some providers do not need to initialize the K3s cluster with scripts,
		// so we need to fill in the missing key information.
		cfg, ip, err := customInstallK3s()
		if err != nil {
			return err
		}
		// save current cluster's kubeConfig.
		if err := SaveCfg(cfg, ip, c.ContextName); err != nil {
			return err
		}
		_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
		// change & save current cluster's status to database.
		c.Status.Status = common.StatusRunning
		if err = common.DefaultDB.SaveCluster(c); err != nil {
			return err
		}
	}

	if deployPlugins != nil {
		// install additional manifests to the current cluster.
		extraManifests := deployPlugins()
		if extraManifests != nil && len(extraManifests) > 0 {
			if err = p.DeployExtraManifest(c, extraManifests); err != nil {
				return err
			}
			p.Logger.Infof("[%s] successfully deployed manifests", p.Provider)
		}
	}

	// deploy custom manifests
	if p.Manifests != "" {
		deployCmd, err := p.getCustomManifests()
		if err != nil {
			return err
		}
		if err = p.DeployExtraManifest(c, deployCmd); err != nil {
			return err
		}
		p.Logger.Infof("[%s] successfully deployed custom manifests", p.Provider)
	}

	return nil
}

func (p *ProviderBase) JoinNodes(cloudInstanceFunc func(ssh *types.SSH) (*types.Cluster, error),
	syncExistInstance func() error, isAutoJoined bool, rollbackInstance func(ids []string) error) error {
	if p.M == nil {
		p.M = new(syncmap.Map)
	}
	logFile, err := common.GetLogFile(p.ContextName)
	if err != nil {
		return err
	}
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("[%s] cluster %s is not exist", p.Provider, p.Name)
	}
	defer func() {
		if err != nil {
			// join failed.
			state.Status = common.StatusRunning
			_ = common.DefaultDB.SaveClusterState(state)
			// rollback instance
			_ = p.RollbackCluster(rollbackInstance)
		}
		// remove join state file and save running state
		_ = logFile.Close()
		if p.Callbacks != nil {
			if process, ok := p.Callbacks[p.ContextName]; ok && process.Event == "update" {
				logEvent := &common.LogEvent{
					Name:        process.Event,
					ContextName: p.ContextName,
				}
				process.Fn(logEvent)
			}
		}
	}()

	p.Logger = common.NewLogger(common.Debug, logFile)
	p.Logger.Infof("[%s] executing join logic...", p.Provider)
	state.Status = common.StatusUpgrading
	err = common.DefaultDB.SaveClusterState(state)
	if err != nil {
		return err
	}

	c, err := cloudInstanceFunc(&p.SSH)
	if err != nil {
		p.Logger.Errorf("[%s] failed to prepare instance, got error %v", p.Provider, err)
		return err
	}

	if syncExistInstance != nil {
		err = syncExistInstance()
		if err != nil {
			return err
		}
	}

	p.syncExistNodes()
	c.Status = p.Status

	added := &types.Cluster{
		Metadata: c.Metadata,
		Options:  c.Options,
		Status:   types.Status{},
	}

	p.M.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		// filter the number of nodes that are not generated by current command.
		if v.Current && v.Master {
			added.Status.MasterNodes = append(added.Status.MasterNodes, v)
		} else if v.Current && !v.Master {
			added.Status.WorkerNodes = append(added.Status.WorkerNodes, v)
		}
		return true
	})

	if !isAutoJoined {
		// execute k3s script to join nodes.
		if err := p.Join(c, added); err != nil {
			return err
		}
	} else {
		// some providers do not need to execute the K3s join logic,
		// so we need to fill in the missing key information.
		prevMasterNum, _ := strconv.Atoi(state.Master)
		prevWorkerNum, _ := strconv.Atoi(state.Worker)
		addedMasterNum, _ := strconv.Atoi(added.Master)
		addedWorkerNum, _ := strconv.Atoi(added.Worker)
		c.Master = strconv.Itoa(prevMasterNum + addedMasterNum)
		c.Worker = strconv.Itoa(prevWorkerNum + addedWorkerNum)
		c.Status.Status = common.StatusRunning
		err = common.DefaultDB.SaveCluster(c)
	}

	p.Logger.Infof("[%s] successfully executed join logic", p.Provider)
	return nil
}

func (p *ProviderBase) MergeConfig() ([]byte, error) {
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("[%s] cluster %s is not exist", p.Provider, p.Name)
	}
	p.overwriteMetadata(state)
	p.Status = types.Status{
		Status: state.Status,
	}
	masterNodes := make([]types.Node, 0)
	err = json.Unmarshal(state.MasterNodes, &masterNodes)
	if err != nil {
		return nil, err
	}
	workerNodes := make([]types.Node, 0)
	err = json.Unmarshal(state.WorkerNodes, &workerNodes)
	if err != nil {
		return nil, err
	}
	p.Status.MasterNodes = masterNodes
	p.Status.WorkerNodes = workerNodes

	source := reflect.ValueOf(&p.SSH).Elem()
	target := reflect.ValueOf(&state.SSH).Elem()
	utils.MergeConfig(source, target)

	return state.Options, nil
}

func (p *ProviderBase) overwriteMetadata(matched *common.ClusterState) {
	// doesn't need to be overwrite.
	p.Token = matched.Token
	p.IP = matched.IP
	p.UI = matched.UI
	p.ClusterCidr = matched.ClusterCidr
	p.DataStore = matched.DataStore
	p.Mirror = matched.Mirror
	p.DockerMirror = matched.DockerMirror
	p.InstallScript = matched.InstallScript
	p.Network = matched.Network
	// needed to be overwrite.
	if p.K3sChannel == "" {
		p.K3sChannel = matched.K3sChannel
	}
	if p.K3sVersion == "" {
		p.K3sVersion = matched.K3sVersion
	}
	if p.InstallScript == "" {
		p.InstallScript = matched.InstallScript
	}
	if p.Registry == "" {
		p.Registry = matched.Registry
	}
	if p.MasterExtraArgs == "" {
		p.MasterExtraArgs = matched.MasterExtraArgs
	}
	if p.WorkerExtraArgs == "" {
		p.WorkerExtraArgs = matched.WorkerExtraArgs
	}
}

func (p *ProviderBase) DeleteCluster(force bool, delete func(f bool) (string, error)) error {
	isConfirmed := true

	if !force {
		isConfirmed = utils.AskForConfirmation(fmt.Sprintf("[%s] are you sure to delete cluster %s", p.Provider, p.Name))
	}
	if isConfirmed {
		logFile, err := common.GetLogFile(p.ContextName)
		if err != nil {
			return err
		}
		defer func() {
			_ = logFile.Close()
			// remove log file.
			_ = os.Remove(filepath.Join(common.GetLogPath(), p.ContextName))
		}()
		p.Logger = common.NewLogger(common.Debug, logFile)
		p.Logger.Infof("[%s] executing delete cluster logic...", p.Provider)
		contextName, err := delete(force)
		if err != nil {
			return err
		}
		err = OverwriteCfg(contextName)
		if err != nil && !force {
			return fmt.Errorf("[%s] merge kubeconfig error, msg: %v", p.Provider, err)
		}
		err = common.DefaultDB.DeleteCluster(p.Name, p.Provider)
		if err != nil && !force {
			return fmt.Errorf("[%s] failed to delete cluster state, msg: %v", p.Provider, err)
		}

		p.Logger.Infof("[%s] successfully deleted cluster %s", p.Provider, p.Name)
	}
	return nil
}

func (p *ProviderBase) GetClusterStatus(kubeCfg string, c *types.ClusterInfo, describeFunc func() ([]types.Node, error)) *types.ClusterInfo {
	p.Logger = common.NewLogger(common.Debug, nil)

	client, err := GetClusterConfig(p.ContextName, kubeCfg)
	if err != nil {
		p.Logger.Errorf("[%s] failed to generate kube client for cluster %s: %v", p.Provider, p.ContextName, err)
		c.Status = types.ClusterStatusUnknown
		c.Version = types.ClusterStatusUnknown
		return c
	}

	c.Status = GetClusterStatus(client)
	if c.Status == types.ClusterStatusRunning {
		c.Version = GetClusterVersion(client)
	} else {
		c.Version = types.ClusterStatusUnknown
	}

	if describeFunc != nil {
		instanceList, err := describeFunc()
		if err != nil {
			p.Logger.Errorf("%v", err)
			c.Master = "0"
			c.Worker = "0"
			return c
		}
		masterCount := 0
		workerCount := 0
		for _, ins := range instanceList {
			if ins.Master {
				masterCount++
				continue
			}
			workerCount++
		}
		c.Master = strconv.Itoa(masterCount)
		c.Worker = strconv.Itoa(workerCount)
	}

	return c
}

func (p *ProviderBase) SetMetadata(config *types.Metadata) {
	sourceMeta := reflect.ValueOf(&p.Metadata).Elem()
	targetMeta := reflect.ValueOf(config).Elem()
	utils.MergeConfig(sourceMeta, targetMeta)
}

func (p *ProviderBase) SetClusterConfig(config []byte) (*types.Cluster, error) {
	c := types.Cluster{}
	err := json.Unmarshal(config, &c)
	if err != nil {
		return nil, err
	}
	sourceMeta := reflect.ValueOf(&p.Metadata).Elem()
	targetMeta := reflect.ValueOf(&c.Metadata).Elem()
	utils.MergeConfig(sourceMeta, targetMeta)

	sourceSSH := reflect.ValueOf(&p.SSH).Elem()
	targetSSH := reflect.ValueOf(&c.SSH).Elem()
	utils.MergeConfig(sourceSSH, targetSSH)
	return &c, nil
}

func (p *ProviderBase) SaveCredential(secrets map[string]string) error {
	cs, err := common.DefaultDB.GetCredentialByProvider(p.Provider)
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		s, err := json.Marshal(secrets)
		if err != nil {
			return err
		}
		err = common.DefaultDB.CreateCredential(&common.Credential{
			Provider: p.Provider,
			Secrets:  s,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func ListClusters() ([]*types.ClusterInfo, error) {
	stateList, err := common.DefaultDB.ListCluster()
	if err != nil {
		return nil, err
	}
	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	clusterList := make([]*types.ClusterInfo, 0)
	for _, state := range stateList {
		provider, err := providers.GetProvider(state.Provider)
		if err != nil {
			logrus.Errorf("failed to get provider %v: %v", state.Provider, err)
			continue
		}
		provider.SetMetadata(&state.Metadata)
		_ = provider.SetOptions(state.Options)
		contextName := provider.GenerateClusterName()
		if state.Status != common.StatusRunning {
			info := provider.GetCluster("")
			info.Status = state.Status
			info.Master = state.Master
			info.Worker = state.Worker
			clusterList = append(clusterList, info)
			continue
		}
		isExist, _, err := provider.IsClusterExist()
		if err != nil {
			logrus.Errorf("failed to check provider %s cluster %s exist, got error: %v ", state.Provider, state.Name, err)
			continue
		}
		if !isExist {
			logrus.Warnf("cluster %s (provider %s) is not exist, will remove from config", state.Name, state.Provider)
			// remove kube config if cluster not exist
			if err := OverwriteCfg(contextName); err != nil {
				logrus.Errorf("failed to remove unexist cluster %s from kube config", state.Name)
			}
			// update status to missing
			state.Status = common.StatusMissing
			if err := common.DefaultDB.SaveClusterState(state); err != nil {
				logrus.Errorf("failed to update cluster %s state to missing", state.Name)
			}
			info := provider.GetCluster("")
			info.Status = state.Status
			info.Master = state.Master
			info.Worker = state.Worker
			clusterList = append(clusterList, info)
			continue
		}
		clusterList = append(clusterList, provider.GetCluster(kubeCfg))
	}
	return clusterList, nil
}

func (p *ProviderBase) syncExistNodes() {
	p.M.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		nodes := p.Status.WorkerNodes
		if v.Master {
			nodes = p.Status.MasterNodes
		}
		index, b := putil.IsExistedNodes(nodes, v.InstanceID)
		if !b {
			nodes = append(nodes, v)
		} else {
			node := nodes[index]
			node.InstanceStatus = v.InstanceStatus
			nodes[index] = node
		}
		if v.Master {
			p.Status.MasterNodes = nodes
		} else {
			p.Status.WorkerNodes = nodes
		}
		return true
	})
}

func (p *ProviderBase) Describe(kubeCfg string, c *types.ClusterInfo, describeInstance func() ([]types.Node, error)) *types.ClusterInfo {
	if kubeCfg == "" {
		c.Status = common.StatusMissing
		c.Master = p.Master
		c.Worker = p.Worker
		return c
	}
	p.Logger = common.NewLogger(common.Debug, nil)
	client, err := GetClusterConfig(p.ContextName, kubeCfg)
	if err != nil {
		p.Logger.Errorf("[%s] failed to generate kube client for cluster %s: %v", p.Provider, p.Name, err)
		c.Status = types.ClusterStatusUnknown
		c.Version = types.ClusterStatusUnknown
		return c
	}
	c.Status = GetClusterStatus(client)
	if describeInstance != nil {
		instanceList, err := describeInstance()
		if err != nil {
			p.Logger.Errorf("[%s] failed to get instance for cluster %s: %v", p.Provider, p.Name, err)
			c.Master = "0"
			c.Worker = "0"
			return c
		}
		instanceNodes := make([]types.ClusterNode, 0)
		masterCount := 0
		workerCount := 0
		for _, instance := range instanceList {
			instanceNodes = append(instanceNodes, types.ClusterNode{
				InstanceID:              instance.InstanceID,
				InstanceStatus:          instance.InstanceStatus,
				InternalIP:              instance.InternalIPAddress,
				ExternalIP:              instance.PublicIPAddress,
				Status:                  types.ClusterStatusUnknown,
				ContainerRuntimeVersion: types.ClusterStatusUnknown,
				Version:                 types.ClusterStatusUnknown,
			})
			if instance.Master {
				masterCount++
				continue
			}
			workerCount++
		}
		c.Master = strconv.Itoa(masterCount)
		c.Worker = strconv.Itoa(workerCount)
		c.Nodes = instanceNodes
		if c.Status == types.ClusterStatusRunning {
			c.Version = GetClusterVersion(client)
			nodes, err := DescribeClusterNodes(client, instanceNodes)
			if err != nil {
				p.Logger.Errorf("[%s] failed to list nodes of cluster %s: %v", p.Provider, p.Name, err)
				return c
			}
			c.Nodes = nodes
		} else {
			c.Version = types.ClusterStatusUnknown
		}
	}

	return c
}

func (p *ProviderBase) Connect(ip string, ssh *types.SSH, c *types.Cluster, getStatus func() ([]types.Node, error),
	isRunning func(status string) bool, customConnect func(id string, cluster *types.Cluster) error) error {
	p.Logger = common.NewLogger(common.Debug, nil)
	p.Logger.Infof("[%s] executing ssh logic...", p.Provider)

	if getStatus == nil {
		return fmt.Errorf("failed to get status for provider %s", p.Provider)
	}

	status, err := getStatus()
	if err != nil {
		return err
	}

	ids := make(map[string]string, len(status))

	if ip == "" {
		// generate the node name and determine the current state of the node.
		for _, s := range status {
			var info string
			if len(s.PublicIPAddress) > 0 {
				info = s.PublicIPAddress[0]
			} else {
				info = s.InstanceID
			}
			if s.Master {
				info = fmt.Sprintf("%s (master)", info)
			} else {
				info = fmt.Sprintf("%s (worker)", info)
			}
			if !isRunning(s.InstanceStatus) {
				info = fmt.Sprintf("%s - Unhealthy(%s)", info, s.InstanceStatus)
			}
			ids[s.InstanceID] = info
		}
	}

	if ip == "" {
		ip = strings.Split(utils.AskForSelectItem(fmt.Sprintf("[%s] choose ssh node to connect", p.Provider), ids), " (")[0]
	}

	if ip == "" {
		return fmt.Errorf("[%s] choose incorrect ssh node", p.Provider)
	}

	if customConnect == nil {
		// ssh to the typically node.
		if err := SSHK3sNode(ip, c, ssh); err != nil {
			return err
		}
	} else {
		// some providers do not typically use IP connections,
		// so we need to use a custom connect function.
		if err := customConnect(ip, c); err != nil {
			return err
		}
	}

	p.Logger.Infof("[%s] successfully executed ssh logic", p.Provider)
	return nil
}

func (p *ProviderBase) RollbackCluster(rollbackInstance func(ids []string) error) error {
	p.Logger.Infof("[%s] executing rollback logic...", p.Provider)
	if rollbackInstance != nil {
		ids := make([]string, 0)
		p.M.Range(func(key, value interface{}) bool {
			v := value.(types.Node)
			if v.RollBack {
				ids = append(ids, key.(string))
			}
			return true
		})

		p.Logger.Infof("[%s] instances %s will be rollback", p.Provider, ids)

		// remove instance
		if err := rollbackInstance(ids); err != nil {
			return err
		}
		// remove context
		if err := OverwriteCfg(p.ContextName); err != nil {
			logrus.Errorf("failed to remove cluster context %s from kube config", p.ContextName)
		}
		p.Logger.Infof("[%s] successfully executed rollback logic", p.Provider)
	}

	return nil
}

func (p *ProviderBase) ReleaseManifests() error {
	// remove ui manifest to release ELB.
	masterIP := p.IP
	for _, n := range p.Status.MasterNodes {
		if n.InternalIPAddress[0] == masterIP {
			dialer, err := hosts.NewSSHDialer(&n, true)
			if err != nil {
				return err
			}

			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)

			_ = dialer.SetStdio(&stdout, &stderr, nil).SetWriter(p.Logger.Out).
				Cmd(fmt.Sprintf("sudo kubectl delete -f %s/ui.yaml", common.K3sManifestsDir)).
				Cmd(fmt.Sprintf("sudo rm %s/ui.yaml", common.K3sManifestsDir)).Run()
			_ = dialer.Close()
			break
		}
	}
	return nil
}

func (p *ProviderBase) getCustomManifests() ([]string, error) {
	// check is folder or file.
	info, err := os.Stat(p.Manifests)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		cmd, err := prepareManifestFile(p.Manifests, info.Name())
		return []string{cmd}, err
	}
	// upload all files under directory, not include recursive folders.
	deployCmd := []string{}
	files, err := ioutil.ReadDir(p.Manifests)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		cmd, err := prepareManifestFile(filepath.Join(p.Manifests, f.Name()), f.Name())
		if err != nil {
			return nil, err
		}
		deployCmd = append(deployCmd, cmd)
	}
	return deployCmd, nil
}

func prepareManifestFile(path, name string) (string, error) {
	manifestContent, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(uploadManifestCmd, base64.StdEncoding.EncodeToString(manifestContent), common.K3sManifestsDir, name), nil
}

func (p *ProviderBase) RegisterCallbacks(name, event string, fn func(interface{})) {
	if p.Callbacks == nil {
		p.Callbacks = map[string]*providerProcess{}
	}
	p.Callbacks[name] = &providerProcess{
		ContextName: name,
		Event:       event,
		Fn:          fn,
	}
}
