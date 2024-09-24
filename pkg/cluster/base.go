package cluster

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts/dialer"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	pkgsshkey "github.com/cnrancher/autok3s/pkg/sshkey"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/rancher/wrangler/v2/pkg/schemas"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	k3sVersion          = ""
	k3sChannel          = "stable"
	k3sInstallScript    = "https://get.k3s.io"
	master              = "0"
	worker              = "0"
	ui                  = false
	embedEtcd           = false
	defaultCidr         = "10.42.0.0/16"
	uploadManifestCmd   = "echo \"%s\" | base64 -d | tee \"%s/%s\""
	dockerInstallScript = "https://get.docker.com"

	deployPluginCmd = "echo \"%s\" | base64 -d | tee \"%s/%s.yaml\""
)

// ProviderBase provider base struct.
type ProviderBase struct {
	types.Metadata `json:",inline"`
	types.Status   `json:"status"`
	types.SSH      `json:",inline"`
	M              *sync.Map
	ErrM           map[string]string
	Logger         *logrus.Logger
	Callbacks      map[string]*providerProcess
}

type providerProcess struct {
	ContextName string
	Event       string
	Fn          func(interface{})
}

// NewBaseProvider new base provider.
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
			DockerScript:  dockerInstallScript,
			Rollback:      true,
		},
		Status: types.Status{
			MasterNodes: make([]types.Node, 0),
			WorkerNodes: make([]types.Node, 0),
		},
		SSH: types.SSH{
			SSHPort: "22",
		},
		M:    new(syncmap.Map),
		ErrM: map[string]string{},
	}
}

// GetCreateOptions get create command flag options.
func (p *ProviderBase) GetCreateOptions() []types.Flag {
	return []types.Flag{
		{
			Name:  "cluster",
			P:     &p.Cluster,
			V:     p.Cluster,
			Usage: "Form k3s cluster using embedded etcd (requires K8s >= 1.19), see: https://docs.k3s.io/installation/ha-embedded",
		},
		{
			Name:  "manifests",
			P:     &p.Manifests,
			V:     p.Manifests,
			Usage: "A folder path for multiple manifest files(only support one directory) or a manifest file path. Auto-deploying manifests to K3s which is a manner similar to `kubectl apply`",
		},
		{
			Name:  "enable",
			P:     &p.Enable,
			V:     p.Enable,
			Usage: "Deploy add-ons (internal add-on: \"explorer\", \"rancher\"), e.g.(--enable explorer), explorer is simplify UI for K3s(cnrnacher/kube-explorer). Other add-ons can be found by `autok3s add-ons ls`",
		},
		{
			Name:  "package-name",
			P:     &p.PackageName,
			V:     p.PackageName,
			Usage: "The airgap package name from managed package list",
		},
		{
			Name:  "package-path",
			P:     &p.PackagePath,
			V:     p.PackagePath,
			Usage: "The airgap package path. The \"package-name\" flag will be ignored if this flag is also provided",
		},
		{
			Name:  "set",
			P:     &p.Values,
			V:     p.Values,
			Usage: "set values for add-on when enabled by --enable. e.g. --enable rancher --set rancher.Version=v2.7.5 --set rancher.Hostname=aa.bb.cc",
		},
	}
}

// GetClusterOptions get cluster flag options.
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
			Usage: "Change the default upstream k3s install script address, see: https://docs.k3s.io/installation/configuration#options-for-installation-with-script",
		},
		{
			Name:  "k3s-install-mirror",
			P:     &p.Mirror,
			V:     p.Mirror,
			Usage: "For Chinese users, set INSTALL_K3S_MIRROR=cn to use the mirror address to accelerate k3s binary file download",
		},
		{
			Name:  "docker-arg",
			P:     &p.DockerArg,
			V:     p.DockerArg,
			Usage: "Parameter for docker script, wrapped in quotes.  e.g.(--docker-arg 'P1=\"xxxx\" P2=\"xxxx\"') for multiple parameters, P1 P2 are the parameters in docker install script",
		},
		{
			Name:  "docker-script",
			P:     &p.DockerScript,
			V:     p.DockerScript,
			Usage: fmt.Sprintf("Change the default docker install script address, default is : %s", dockerInstallScript),
		},
		{
			Name:  "master-extra-args",
			P:     &p.MasterExtraArgs,
			V:     p.MasterExtraArgs,
			Usage: "Master extra arguments for k3s installer, wrapped in quotes. e.g.(--master-extra-args '--disable metrics-server'), for more information, please see: https://docs.k3s.io/reference/server-config",
		},
		{
			Name:  "worker-extra-args",
			P:     &p.WorkerExtraArgs,
			V:     p.WorkerExtraArgs,
			Usage: "Worker extra arguments for k3s installer, wrapped in quotes. e.g.(--worker-extra-args '--node-taint key=value:NoExecute'), for more information, please see: https://docs.k3s.io/reference/agent-config",
		},
		{
			Name:  "registry",
			P:     &p.Registry,
			V:     p.Registry,
			Usage: "K3s registry file, see: https://docs.k3s.io/installation/private-registry",
		},
		{
			Name:  "system-default-registry",
			P:     &p.SystemDefaultRegistry,
			V:     p.SystemDefaultRegistry,
			Usage: "K3s private registry to be used for all system images, see: https://docs.k3s.io/reference/server-config",
		},
		{
			Name:  "datastore",
			P:     &p.DataStore,
			V:     p.DataStore,
			Usage: "K3s datastore endpoint, Specify etcd, Mysql, Postgres, or Sqlite (default) data source name, see: https://docs.k3s.io/reference/server-config#database",
		},
		{
			Name:  "datastore-cafile",
			P:     &p.DataStoreCAFile,
			V:     p.DataStoreCAFile,
			Usage: "TLS Certificate Authority (CA) file used to help secure communication with the datastore, see: https://docs.k3s.io/installation/datastore#external-datastore-configuration-parameters",
		},
		{
			Name:  "datastore-certfile",
			P:     &p.DataStoreCertFile,
			V:     p.DataStoreCertFile,
			Usage: "TLS certificate file used for client certificate based authentication to your datastore, see: https://docs.k3s.io/installation/datastore#external-datastore-configuration-parameters",
		},
		{
			Name:  "datastore-keyfile",
			P:     &p.DataStoreKeyFile,
			V:     p.DataStoreKeyFile,
			Usage: "TLS key file used for client certificate based authentication to your datastore, see: https://docs.k3s.io/installation/datastore#external-datastore-configuration-parameters",
		},
		{
			Name:  "token",
			P:     &p.Token,
			V:     p.Token,
			Usage: "K3s token, if empty will automatically generated, see: https://docs.k3s.io/reference/server-config#cluster-options",
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
		{
			Name:  "rollback",
			P:     &p.Rollback,
			V:     p.Rollback,
			Usage: "Whether to rollback when the K3s cluster installation or join nodes failed.",
		},
		{
			Name:  "install-env",
			P:     &p.InstallEnv,
			V:     p.InstallEnv,
			Usage: "The install environment variables config for K3s with install script(only support env starts with INSTALL_), e.g. --install-env INSTALL_K3S_SKIP_SELINUX_RPM=true, see: https://docs.k3s.io/installation/configuration#configuration-with-install-script",
		},
		{
			Name:  "server-config-file",
			P:     &p.ServerConfigFile,
			V:     p.ServerConfigFile,
			Usage: "Config K3s server with configuration file which can do more complex configuration than environment variables and CLI arguments, see: https://docs.k3s.io/installation/configuration#configuration-file",
		},
		{
			Name:  "agent-config-file",
			P:     &p.AgentConfigFile,
			V:     p.AgentConfigFile,
			Usage: "Config K3s agent with configuration file which can do more complex configuration than environment variables and CLI arguments, see: https://docs.k3s.io/installation/configuration#configuration-file",
		},
	}

	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

// GetSSHOptions get ssh flag options.
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
		{
			Name:  "ssh-key-name",
			P:     &p.SSHKeyName,
			V:     p.SSHKeyName,
			Usage: "Use the stored ssh key with name",
		},
	}
}

// GetCommonConfig get common config.
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

// InitCluster init K3S cluster.
func (p *ProviderBase) InitCluster(options interface{}, deployCCM func() []string,
	cloudInstanceFunc func(ssh *types.SSH) (*types.Cluster, error), customInstallK3s func() (string, string, error), rollbackInstance func(ids []string) error) (er error) {
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
		if er != nil || len(p.ErrM) > 0 {
			// save failed status.
			state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
			if err == nil {
				var c types.Cluster
				if state == nil {
					c = types.Cluster{
						Metadata: p.Metadata,
						Options:  options,
						SSH:      p.SSH,
						Status:   types.Status{},
					}
				} else {
					c = common.ConvertToCluster(state, true)
				}
				if er != nil {
					p.Logger.Errorf("%v", er)
					c.Status.Status = common.StatusFailed
				}
				if len(p.ErrM) > 0 && len(c.WorkerNodes) > 0 {
					// remove failed node from cluster state
					workers := []types.Node{}
					for _, n := range c.WorkerNodes {
						if _, ok := p.ErrM[n.InstanceID]; !ok {
							workers = append(workers, n)
						}
					}
					c.WorkerNodes = workers
					c.Worker = strconv.Itoa(len(workers))
				}
				_ = common.DefaultDB.SaveCluster(&c)
			}
			_ = p.RollbackCluster(rollbackInstance)
		}
		if er == nil && len(p.Status.MasterNodes) > 0 {
			p.Logger.Info(common.UsageInfoTitle)
			p.Logger.Infof(common.UsageContext, p.ContextName)
			p.Logger.Info(common.UsagePods)
		}
		_ = logFile.Close()
		if p.Callbacks != nil {
			if process, ok := p.Callbacks[p.ContextName]; ok && process.Event == "create" {
				logEvent := &common.LogEvent{
					Name:        process.Event,
					ContextType: "cluster",
					ContextName: p.ContextName,
				}
				process.Fn(logEvent)
			}
		}
	}()
	p.Logger = common.NewLogger(logFile)
	p.Logger.Infof("[%s] begin to create cluster %s...", p.Provider, p.Name)
	c.Status.Status = common.StatusCreating
	// save cluster.
	if err = common.DefaultDB.SaveCluster(c); err != nil {
		return err
	}
	// store ssh key
	if newSSH, err := pkgsshkey.StoreClusterSSHKeys(p.ContextName, &c.SSH); err != nil {
		return err
	} else if newSSH != nil {
		p.Logger.Infof("[%s] cluster's ssh keys saved", p.Name)
		c.SSH = *newSSH
		// update cluster with stored key
		if err = common.DefaultDB.SaveCluster(c); err != nil {
			return err
		}
		p.SSH = *newSSH
	}

	c, err = cloudInstanceFunc(&c.SSH)
	if err != nil {
		return err
	}
	p.syncExistNodes()
	c.Status = p.Status

	if customInstallK3s == nil {
		// use install scripts to initialize K3s cluster.
		if err = p.InitK3sCluster(c, deployCCM); err != nil {
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
		_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, filepath.Join(common.CfgPath, common.KubeCfgFile))
		// change & save current cluster's status to database.
		c.Status.Status = common.StatusRunning
		if err = common.DefaultDB.SaveCluster(c); err != nil {
			return err
		}
	}

	cmds := []string{}
	//if deployCCM != nil {
	//	// install additional manifests to the current cluster.
	//	extraManifests := deployCCM()
	//	cmds = append(cmds, extraManifests...)
	//}

	if p.Manifests != "" {
		deployCmd, err := p.GetCustomManifests()
		if err != nil {
			p.Logger.Errorf("[%s] failed to get custom manifests by manifest %s: %v", p.Provider, p.Manifests, err)
		}
		cmds = append(cmds, deployCmd...)
	}

	if p.Enable != nil {
		for _, plugin := range p.Enable {
			if plugin != "explorer" {
				cmd, err := p.addonInstallation(plugin)
				if err != nil {
					continue
				}
				cmds = append(cmds, cmd)
			}
		}
	}

	// deploy custom manifests.
	if len(cmds) > 0 {
		if err = p.DeployExtraManifest(c, cmds); err != nil {
			return err
		}
		p.Logger.Infof("[%s] successfully deployed custom manifests", p.Provider)
	}

	return
}

// JoinNodes join K3S nodes.
// nolint: gocyclo
func (p *ProviderBase) JoinNodes(cloudInstanceFunc func(ssh *types.SSH) (*types.Cluster, error),
	syncExistInstance func() error, isAutoJoined bool, rollbackInstance func(ids []string) error) (er error) {
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
		if er != nil || len(p.ErrM) > 0 {
			// join failed.
			state, err = common.DefaultDB.GetCluster(p.Name, p.Provider)
			if err == nil {
				if len(p.ErrM) > 0 {
					// remove failed node from cluster state
					workerNodes := []types.Node{}
					_ = json.Unmarshal(state.WorkerNodes, &workerNodes)
					workers := []types.Node{}
					for _, n := range workerNodes {
						if _, ok := p.ErrM[n.InstanceID]; !ok {
							workers = append(workers, n)
						}
					}
					wb, err := json.Marshal(workers)
					if err == nil {
						state.WorkerNodes = wb
					}
					state.Worker = strconv.Itoa(len(workers))
				}
				state.Status = common.StatusRunning
				_ = common.DefaultDB.SaveClusterState(state)
				// rollback instance.
				_ = p.RollbackCluster(rollbackInstance)
			}
		}
		// remove join state file and save running state.
		_ = logFile.Close()
		if p.Callbacks != nil {
			if process, ok := p.Callbacks[p.ContextName]; ok && process.Event == "update" {
				logEvent := &common.LogEvent{
					Name:        process.Event,
					ContextType: "cluster",
					ContextName: p.ContextName,
				}
				process.Fn(logEvent)
			}
		}
	}()

	p.Logger = common.NewLogger(logFile)
	p.Logger.Infof("[%s] begin to join nodes for %v...", p.Provider, p.Name)
	state.Status = common.StatusUpgrading
	err = common.DefaultDB.SaveClusterState(state)
	if err != nil {
		return err
	}

	c, err := cloudInstanceFunc(&state.SSH)
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
	c.Status.MasterNodes = p.Status.MasterNodes
	c.Status.WorkerNodes = p.Status.WorkerNodes

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
		err = p.Join(c, added)
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
	return err
}

// MergeConfig merge cluster config.
func (p *ProviderBase) MergeConfig() ([]byte, error) {
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	p.overwriteMetadata(state)
	p.Status = types.Status{
		Status:     state.Status,
		Standalone: state.Standalone,
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
	p.Cluster = matched.Cluster
	p.Rollback = matched.Rollback
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
	if p.DockerArg == "" {
		p.DockerArg = matched.DockerArg
	}
	if p.DockerScript == "" {
		p.DockerScript = matched.DockerScript
	}
	if p.Registry == "" {
		p.Registry = matched.Registry
	}
	if p.SystemDefaultRegistry == "" {
		p.SystemDefaultRegistry = matched.SystemDefaultRegistry
	}
	if p.MasterExtraArgs == "" {
		p.MasterExtraArgs = matched.MasterExtraArgs
	}
	if p.WorkerExtraArgs == "" {
		p.WorkerExtraArgs = matched.WorkerExtraArgs
	}
}

func (p *ProviderBase) CheckCreateArgs(checkClusterExist func() (bool, []string, error)) error {
	if p.Provider != "native" {
		masterNum, err := strconv.Atoi(p.Master)
		if masterNum < 1 || err != nil {
			return fmt.Errorf("[%s] calling preflight error: `--master` number must >= 1",
				p.Provider)
		}
		if p.Provider != "k3d" && masterNum > 1 && !p.Cluster && p.DataStore == "" {
			return fmt.Errorf("[%s] calling preflight error: need to set `--cluster` or `--datastore` when `--master` number > 1",
				p.Provider)
		}
		if p.Provider != "k3d" && strings.Contains(p.MasterExtraArgs, "--datastore-endpoint") && p.DataStore != "" {
			return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
				p.Provider)
		}
		if _, err = strconv.Atoi(p.Worker); err != nil {
			return fmt.Errorf("[%s] calling preflight error: `--worker` must be number",
				p.Provider)
		}
	}

	// check name exist.
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return err
	}

	if state != nil && state.Status != common.StatusFailed {
		return fmt.Errorf("[%s] cluster %s is already exist", p.Provider, p.Name)
	}

	exist, _, err := checkClusterExist()
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("[%s] calling preflight error: cluster `%s` is already exist",
			p.Provider, p.Name)
	}

	// check file exists.
	if p.SSHKeyPath != "" && !utils.IsFileExists(p.SSHKeyPath) {
		return fmt.Errorf("[%s] failed to check --ssh-key-path %s", p.Provider, p.SSHKeyPath)
	}
	if p.SSHCertPath != "" && !utils.IsFileExists(p.SSHCertPath) {
		return fmt.Errorf("[%s] failed to check --ssh-cert-path %s", p.Provider, p.SSHCertPath)
	}

	if p.Registry != "" && !utils.IsFileExists(p.Registry) {
		return fmt.Errorf("[%s] failed to check --registry %s", p.Provider, p.Registry)
	}

	if p.DataStoreCAFile != "" && !utils.IsFileExists(p.DataStoreCAFile) {
		return fmt.Errorf("[%s] failed to check --datastore-cafile %s", p.Provider, p.DataStoreCAFile)
	}

	if p.DataStoreCertFile != "" && !utils.IsFileExists(p.DataStoreCertFile) {
		return fmt.Errorf("[%s] failed to check --datastore-certfile %s", p.Provider, p.DataStoreCertFile)
	}

	if p.DataStoreKeyFile != "" && !utils.IsFileExists(p.DataStoreKeyFile) {
		return fmt.Errorf("[%s] failed to check --datastore-keyfile %s", p.Provider, p.DataStoreKeyFile)
	}

	if p.InstallEnv != nil {
		unsupportEnv := []string{}
		for key, value := range p.InstallEnv {
			if !strings.HasPrefix(key, "INSTALL_") {
				unsupportEnv = append(unsupportEnv, fmt.Sprintf("%v=%v", key, value))
			}
		}
		if len(unsupportEnv) > 0 {
			return fmt.Errorf("[%s] Invalid install environment %v. Only support INSTALL_* environments. For K3S_* variables please use config file args", p.Provider, unsupportEnv)
		}
	}

	return nil
}

func (p *ProviderBase) CheckJoinArgs(checkClusterExist func() (bool, []string, error)) error {
	// check cluster exist.
	exist, _, err := checkClusterExist()

	if err != nil {
		return err
	}

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist",
			p.Provider, p.ContextName)
	}

	if strings.Contains(p.MasterExtraArgs, "--datastore-endpoint") && p.DataStore != "" {
		return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
			p.Provider)
	}

	if p.Provider != "native" {
		masterNum, err := strconv.Atoi(p.Master)
		if err != nil {
			return fmt.Errorf("[%s] calling preflight error: `--master` must be number",
				p.Provider)
		}

		if masterNum > 0 && p.DataStore == "" && !p.Cluster {
			return fmt.Errorf("[%s] calling preflight error: can't join master nodes to single node cluster", p.Provider)
		}

		workerNum, err := strconv.Atoi(p.Worker)
		if err != nil {
			return fmt.Errorf("[%s] calling preflight error: `--worker` must be number",
				p.Provider)
		}
		if masterNum < 1 && workerNum < 1 {
			return fmt.Errorf("[%s] calling preflight error: `--master` or `--worker` number must >= 1", p.Provider)
		}
	}

	if p.InstallEnv != nil {
		unsupportEnv := []string{}
		for key, value := range p.InstallEnv {
			if !strings.HasPrefix(key, "INSTALL_") {
				unsupportEnv = append(unsupportEnv, fmt.Sprintf("%v=%v", key, value))
			}
		}
		if len(unsupportEnv) > 0 {
			return fmt.Errorf("[%s] Invalid install environment %v. Only support INSTALL_* environments. For K3S_* variables please use config file args", p.Provider, unsupportEnv)
		}
	}

	return nil
}

// DeleteCluster delete cluster.
func (p *ProviderBase) DeleteCluster(force bool, delete func(f bool) (string, error)) error {
	isConfirmed := true

	if !force {
		isConfirmed = utils.AskForConfirmation(fmt.Sprintf("[%s] are you sure to delete cluster %s", p.Provider, p.Name), false)
	}
	if isConfirmed {
		logFile, err := common.GetLogFile(p.ContextName)
		if err != nil {
			return err
		}
		defer func() {
			_ = logFile.Close()
			// remove log file.
			_ = os.RemoveAll(common.GetClusterContextPath(p.ContextName))
		}()
		state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
		if err != nil && !force {
			return fmt.Errorf("[%s] failed to get cluster %s, got error %v", p.Provider, p.Name, err)
		}
		p.Logger = common.NewLogger(logFile)
		p.Logger.Infof("[%s] begin to delete cluster %v...", p.Provider, p.Name)
		if state != nil {
			state.Status = common.StatusRemoving
			err = common.DefaultDB.SaveClusterState(state)
			if err != nil && !force {
				return fmt.Errorf("[%s] failed to update cluster %s status to removing, got error %v", p.Provider, p.Name, err)
			}
		}

		contextName, err := delete(force)
		if err != nil {
			return err
		}
		err = common.FileManager.ClearCfgByContext(contextName)
		if err != nil && !force {
			return fmt.Errorf("[%s] merge kubeconfig error, msg: %v", p.Provider, err)
		}
		err = common.DefaultDB.DeleteCluster(p.Name, p.Provider)
		if err != nil && !force {
			return fmt.Errorf("[%s] failed to delete cluster state, msg: %v", p.Provider, err)
		}

		// release kube-explorer
		exp, err := common.DefaultDB.GetExplorer(p.ContextName)
		if err != nil && !force {
			return fmt.Errorf("[%s] failed to get kube-explorer config for cluster %s: %v", p.Provider, p.ContextName, err)
		}
		if exp != nil {
			if exp.Enabled {
				_ = common.DisableExplorer(p.ContextName)
			}
			err = common.DefaultDB.DeleteExplorer(p.ContextName)
			if err != nil && !force {
				return fmt.Errorf("[%s] failed to delete explorer setting for %s: %v", p.Provider, p.ContextName, err)
			}
		}

		p.Logger.Infof("[%s] successfully deleted cluster %s", p.Provider, p.Name)
	}
	return nil
}

// GetClusterStatus get cluster status.
func (p *ProviderBase) GetClusterStatus(kubeCfg string, c *types.ClusterInfo, describeFunc func() ([]types.Node, error)) *types.ClusterInfo {
	p.Logger = logrus.StandardLogger()

	c.Master = p.Master
	c.Worker = p.Worker
	if p.Cluster {
		c.IsHAMode = true
		c.DataStoreType = "Embedded DB(etcd)"
	} else if p.DataStore != "" {
		c.IsHAMode = true
		dataStoreArray := strings.Split(p.DataStore, "://")
		if dataStoreArray[0] == "http" {
			c.DataStoreType = "External DB(etcd)"
		} else {
			c.DataStoreType = fmt.Sprintf("External DB(%s)", dataStoreArray[0])
		}
	}

	if kubeCfg == "" {
		return c
	}

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

// SetMetadata set metadata.
func (p *ProviderBase) SetMetadata(config *types.Metadata) {
	sourceMeta := reflect.ValueOf(&p.Metadata).Elem()
	targetMeta := reflect.ValueOf(config).Elem()
	utils.MergeConfig(sourceMeta, targetMeta)
}

// SetClusterConfig set cluster config.
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

// SaveCredential save credential to database.
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

// ListClusters list clusters.
func ListClusters(providerName string) ([]*types.ClusterInfo, error) {
	stateList, err := common.DefaultDB.ListCluster(providerName)
	if err != nil {
		return nil, err
	}
	kubeCfg := filepath.Join(common.CfgPath, common.KubeCfgFile)
	clusterList := make([]*types.ClusterInfo, 0)
	for _, state := range stateList {
		// TODO skip harvester for historical data, will remove here after harvester provider added back
		if state.Provider == "harvester" {
			continue
		}
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
			info := provider.GetCluster("")
			info.Status = common.StatusUnknown
			info.Master = state.Master
			info.Worker = state.Worker
			clusterList = append(clusterList, info)
			logrus.Errorf("failed to check provider %s cluster %s exist, got error: %v ", state.Provider, state.Name, err)
			continue
		}
		if !isExist {
			logrus.Warnf("cluster %s (provider %s) is not exist, will remove from config", state.Name, state.Provider)
			// remove kube config if cluster not exist
			if err := common.FileManager.ClearCfgByContext(contextName); err != nil {
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

// Describe describe cluster info.
func (p *ProviderBase) Describe(kubeCfg string, c *types.ClusterInfo, describeInstance func() ([]types.Node, error)) *types.ClusterInfo {
	c.Master = p.Master
	c.Worker = p.Worker

	if p.Cluster {
		c.IsHAMode = true
		c.DataStoreType = "Embedded DB(etcd)"
	} else if p.DataStore != "" {
		c.IsHAMode = true
		dataStoreArray := strings.Split(p.DataStore, "://")
		if dataStoreArray[0] == "http" {
			c.DataStoreType = "External DB(etcd)"
		} else {
			c.DataStoreType = fmt.Sprintf("External DB(%s)", dataStoreArray[0])
		}
	}

	if kubeCfg == "" {
		c.Status = common.StatusMissing
		return c
	}
	p.Logger = logrus.StandardLogger()
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
				Standalone:              instance.Standalone,
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

// Connect ssh & connect to the K3S node.
func (p *ProviderBase) Connect(ip string, ssh *types.SSH, c *types.Cluster, getStatus func() ([]types.Node, error),
	isRunning func(status string) bool, customConnect func(id string, cluster *types.Cluster) error) error {
	p.Logger = logrus.StandardLogger()
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

// RollbackCluster rollback when error occur.
func (p *ProviderBase) RollbackCluster(rollbackInstance func(ids []string) error) error {
	if !p.Rollback {
		p.Logger.Warnf("[%s] skip executing rollback logic. This is only used for troubleshooting, the instances and cluster is out of control by AutoK3s. Please check K3s error log and uninstalled manually if needed.", p.Provider)
		return nil
	}
	p.Logger.Infof("[%s] executing rollback logic...", p.Provider)
	if rollbackInstance != nil {
		ids := make([]string, 0)
		// support for partial rollback
		if len(p.ErrM) > 0 {
			p.Logger.Warnf("[%s] The following instances need to rollback in some of reasons...", p.Provider)
			for key, value := range p.ErrM {
				p.Logger.Warnf("[%s] The instance %s is failed to join to the K3s cluster with error: %v", p.Provider, key, value)
				ids = append(ids, key)
			}
		} else {
			p.M.Range(func(key, value interface{}) bool {
				v := value.(types.Node)
				if v.RollBack {
					ids = append(ids, key.(string))
				}
				return true
			})
		}

		p.Logger.Infof("[%s] instances %s will be rollback", p.Provider, ids)

		// remove instance.
		if err := rollbackInstance(ids); err != nil {
			return err
		}

		state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
		if err != nil {
			return err
		}
		if state == nil || state.Status != common.StatusRunning {
			// remove context.
			if err := common.FileManager.ClearCfgByContext(p.ContextName); err != nil {
				logrus.Errorf("failed to remove cluster context %s from kube config", p.ContextName)
			}
		}

		p.Logger.Infof("[%s] successfully executed rollback logic", p.Provider)
	}

	return nil
}

// ReleaseManifests release manifests.
func (p *ProviderBase) ReleaseManifests() error {
	// remove ui manifest to release ELB.
	masterIP := p.IP
	for _, n := range p.Status.MasterNodes {
		if n.InternalIPAddress[0] == masterIP {
			dialer, err := dialer.NewSSHDialer(&n, true, p.Logger)
			if err != nil {
				return err
			}
			defer dialer.Close()
			_, _ = dialer.ExecuteCommands(
				fmt.Sprintf("kubectl delete -f %s/ui.yaml", common.K3sManifestsDir),
				fmt.Sprintf("rm %s/ui.yaml", common.K3sManifestsDir),
			)

			break
		}
	}
	return nil
}

// GetCustomManifests get custom manifests.
func (p *ProviderBase) GetCustomManifests() ([]string, error) {
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
	deployCmd := make([]string, 0)
	files, err := os.ReadDir(p.Manifests)
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

// RegisterCallbacks register callbacks.
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

func (p *ProviderBase) UpgradeK3sCluster(clusterName, installScript, channel, version, packageName, packagePath string) error {
	if p.Provider == "k3d" {
		return errors.New("the upgrade cluster for K3d provider is not supported yet")
	}
	state, err := common.DefaultDB.GetCluster(clusterName, p.Provider)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("cluster %s is not exist", clusterName)
	}
	p.Name = clusterName
	p.ContextName = state.ContextName
	logFile, err := common.GetLogFile(state.ContextName)
	if err != nil {
		return err
	}
	p.Logger = common.NewLogger(logFile)
	p.Logger.Infof("[%s] begin to upgrade cluster %s...", p.Provider, clusterName)
	state.Status = common.StatusUpgrading
	// save cluster.
	err = common.DefaultDB.SaveClusterState(state)
	if err != nil {
		return err
	}

	defer func() {
		// update cluster status
		state.Status = common.StatusRunning
		_ = common.DefaultDB.SaveClusterState(state)
		// remove upgrade state file and save running state.
		_ = logFile.Close()
		if p.Callbacks != nil {
			if process, ok := p.Callbacks[state.ContextName]; ok && process.Event == "update" {
				logEvent := &common.LogEvent{
					Name:        process.Event,
					ContextType: "cluster",
					ContextName: state.ContextName,
				}
				process.Fn(logEvent)
			}
		}
	}()

	c := common.ConvertToCluster(state, true)

	if installScript != "" {
		c.InstallScript = installScript
		state.InstallScript = installScript
	}
	if channel != "" {
		c.K3sChannel = channel
		state.K3sChannel = channel
	}
	if version != "" {
		c.K3sVersion = version
		state.K3sVersion = version
	}

	// if online install specified, clean up offline options and ignore package name/path input
	if installScript != "" || channel != "" || version != "" {
		c.PackageName = ""
		state.PackageName = ""
		c.PackagePath = ""
		state.PackagePath = ""
	} else {
		if packageName != "" {
			c.PackageName = packageName
			state.PackageName = packageName
		}
		if packagePath != "" {
			c.PackagePath = packagePath
			state.PackagePath = packagePath
		}
	}

	return p.Upgrade(&c)
}

func (p *ProviderBase) ValidateRequireSSHPrivateKey() error {
	errStr := "ssh key is require but none of --ssh-key-path or --ssh-key-name is provided"
	if !common.IsCLI {
		errStr = "ssh key is require but none of --ssh-key-path, --ssh-key-name or --ssh-key is provided"
	}
	if p.SSHKey == "" && p.SSHKeyPath == "" && p.SSHKeyName == "" {
		return errors.New(errStr)
	}
	return nil
}

func (p *ProviderBase) parseDefaultTemplates() template.FuncMap {
	templateValues := p.MasterNodes
	return template.FuncMap{
		"providerTemplate": func(metaName string) string {
			node := templateValues[0]
			nodeType := reflect.TypeOf(node)
			fieldValue := ""
			found := true
			for i := 0; i < nodeType.NumField(); i++ {
				field := nodeType.Field(i)
				if v, ok := field.Tag.Lookup("json"); ok {
					fieldName := strings.Split(v, ",")[0]
					if strings.EqualFold(fieldName, metaName) {
						found = true
						if field.Type.Kind() == reflect.Slice {
							value, ok := reflect.ValueOf(node).Field(i).Interface().([]string)
							if ok {
								fieldValue = value[0]
							}
						} else {
							fieldValue = reflect.ValueOf(node).Field(i).String()
						}
						break
					}

				}
			}
			if !found {
				p.Logger.Warnf("[%s] there's no metaName %s defined", p.Provider, metaName)
			}
			return fieldValue
		},
	}
}

func (p *ProviderBase) addonInstallation(plugin string) (string, error) {
	// check addon plugin
	addon, err := common.DefaultDB.GetAddon(plugin)
	if err != nil {
		p.Logger.Errorf("[%s] failed to get addon by name %s, got error: %v", p.Provider, plugin, err)
		return "", err
	}
	manifest := addon.Manifest
	defaultValues := addon.Values
	// check --set values
	setValues := map[string]string{}
	for key, value := range p.Values {
		if strings.HasPrefix(key, fmt.Sprintf("%s.", plugin)) {
			setValues[strings.TrimPrefix(key, fmt.Sprintf("%s.", plugin))] = value
		} else {
			setValues[key] = value
		}
	}
	values, err := common.GenerateValues(setValues, defaultValues)
	if err != nil {
		p.Logger.Errorf("[%s] failed to generate values for addon %s with values %v: %v", p.Provider, plugin, p.Values, err)
		return "", err
	}
	p.Logger.Debugf("assemble manifest with value %++v", values)
	assembleManifest, err := common.AssembleManifest(values, string(manifest), p.parseDefaultTemplates())
	if err != nil {
		p.Logger.Errorf("[%s] failed to assemble manifest for addon %s with values %v: %v", p.Provider, plugin, setValues, err)
		return "", err
	}
	return fmt.Sprintf(deployPluginCmd,
		base64.StdEncoding.EncodeToString(assembleManifest), common.K3sManifestsDir, plugin), nil
}
