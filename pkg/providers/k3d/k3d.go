package k3d

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/hosts/dialer"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	typesk3d "github.com/cnrancher/autok3s/pkg/types/k3d"
	"github.com/cnrancher/autok3s/pkg/utils"

	dockerunits "github.com/docker/go-units"
	cliutil "github.com/k3d-io/k3d/v5/cmd/util"
	k3dutil "github.com/k3d-io/k3d/v5/cmd/util"
	"github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/config"
	k3dtypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	k3dconf "github.com/k3d-io/k3d/v5/pkg/config/v1alpha5"
	k3dlogger "github.com/k3d-io/k3d/v5/pkg/logger"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	k3dversion "github.com/k3d-io/k3d/v5/version"
	"github.com/moby/term"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	providerName = "k3d"

	k3dAPIPort = "0.0.0.0:0"
)

var (
	k3dImage = fmt.Sprintf("%s:%s", k3d.DefaultK3sImageRepo, k3dversion.K3sVersion)
)

// K3d provider k3d struct.
type K3d struct {
	*cluster.ProviderBase `json:",inline"`
	typesk3d.Options      `json:",inline"`
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *K3d {
	base := cluster.NewBaseProvider()
	base.Provider = providerName
	return &K3d{
		ProviderBase: base,
		Options: typesk3d.Options{
			APIPort: k3dAPIPort,
			Image:   k3dImage,
		},
	}
}

// GetProviderName returns provider name.
func (p *K3d) GetProviderName() string {
	return p.Provider
}

// GenerateClusterName generates and returns cluster name.
func (p *K3d) GenerateClusterName() string {
	// must comply with the k3d cluster name rules.
	p.ContextName = fmt.Sprintf("%s-%s", p.GetProviderName(), p.Name)
	return p.ContextName
}

// CreateK3sCluster create K3S cluster.
func (p *K3d) CreateK3sCluster() (err error) {
	return p.InitCluster(p.Options, nil, p.createK3d, p.obtainKubeCfg, p.rollbackK3d)
}

// JoinK3sNode join K3S node.
func (p *K3d) JoinK3sNode() (err error) {
	return p.JoinNodes(p.joinK3d, p.syncK3d, true, p.rollbackK3d)
}

// DeleteK3sCluster delete K3S cluster.
func (p *K3d) DeleteK3sCluster(f bool) (err error) {
	return p.DeleteCluster(f, p.deleteK3d)
}

// SSHK3sNode ssh K3s node.
func (p *K3d) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	return p.Connect(ip, nil, c, p.k3dStatus, p.isNodeRunning, p.attachNode)
}

// IsClusterExist determine if the cluster exists.
func (p *K3d) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	cfg := &k3d.Cluster{
		Name: p.Name,
		ServerLoadBalancer: &k3d.Loadbalancer{
			Config: &k3d.LoadbalancerConfig{},
		},
	}

	var (
		c   *k3d.Cluster
		err error
	)

	if c, err = client.ClusterGet(context.Background(), runtimes.SelectedRuntime, cfg); err != nil {
		// ignore the error.
		return false, ids, nil
	}

	for _, n := range c.Nodes {
		if n.State.Running {
			ids = append(ids, n.Name)
		}
	}

	return len(ids) > 0, ids, nil
}

// SetOptions set options.
func (p *K3d) SetOptions(opt []byte) error {
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	option := &typesk3d.Options{}
	err := json.Unmarshal(opt, option)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(option).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// GetCluster returns cluster status.
func (p *K3d) GetCluster(kubeConfig string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	return p.GetClusterStatus(kubeConfig, c, p.k3dStatus)
}

// DescribeCluster describe cluster info.
func (p *K3d) DescribeCluster(kubeConfig string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubeConfig, c, p.k3dStatus)
}

// GetProviderOptions get provider options.
func (p *K3d) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &typesk3d.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

// SetConfig set cluster config.
func (p *K3d) SetConfig(config []byte) error {
	c, err := p.SetClusterConfig(config)
	if err != nil {
		return err
	}
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	b, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	opt := &typesk3d.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

	return nil
}

// CreateCheck check create command and flags.
func (p *K3d) CreateCheck() error {
	if err := p.CheckCreateArgs(p.IsClusterExist); err != nil {
		return err
	}

	ipPorts := strings.Split(p.APIPort, ":")
	if len(ipPorts) < 2 {
		return fmt.Errorf("[%s] calling preflight error: provided `--api-ports` is invalid", p.GetProviderName())
	}

	if p.MastersMemory != "" {
		if _, err := dockerunits.RAMInBytes(p.MastersMemory); err != nil {
			return fmt.Errorf("[%s] calling preflight error: provided `--masters-memory` limit value is invalid", p.GetProviderName())
		}
	}
	if p.WorkersMemory != "" {
		if _, err := dockerunits.RAMInBytes(p.WorkersMemory); err != nil {
			return fmt.Errorf("[%s] calling preflight error: provided `--workers-memory` limit value is invalid", p.GetProviderName())
		}
	}

	return nil
}

// JoinCheck check join command and flags.
func (p *K3d) JoinCheck() error {
	if err := p.CheckJoinArgs(p.IsClusterExist); err != nil {
		return err
	}
	if p.MastersMemory != "" {
		if _, err := dockerunits.RAMInBytes(p.MastersMemory); err != nil {
			return fmt.Errorf("[%s] calling preflight error: provided `--masters-memory` limit value is invalid", p.GetProviderName())
		}
	}
	if p.WorkersMemory != "" {
		if _, err := dockerunits.RAMInBytes(p.WorkersMemory); err != nil {
			return fmt.Errorf("[%s] calling preflight error: provided `--workers-memory` limit value is invalid", p.GetProviderName())
		}
	}

	return nil
}

// GenerateMasterExtraArgs generates K3S master extra args.
func (p *K3d) GenerateMasterExtraArgs(_ *types.Cluster, _ types.Node) string {
	return ""
}

// GenerateWorkerExtraArgs generates K3S worker extra args.
func (p *K3d) GenerateWorkerExtraArgs(_ *types.Cluster, _ types.Node) string {
	return ""
}

func (p *K3d) rollbackK3d(ids []string) error {
	for _, id := range ids {
		if strings.Contains(id, p.Name) {
			if err := client.ClusterDelete(context.Background(), runtimes.SelectedRuntime, &k3d.Cluster{Name: p.Name},
				k3d.ClusterDeleteOpts{SkipRegistryCheck: true}); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func (p *K3d) syncK3d() error {
	_, err := p.k3dStatus()
	if err != nil {
		return err
	}
	return nil
}

func (p *K3d) k3dStatus() ([]types.Node, error) {
	// ids := make([]string, 0)

	cfg := &k3d.Cluster{
		Name: p.Name,
		ServerLoadBalancer: &k3d.Loadbalancer{
			Config: &k3d.LoadbalancerConfig{},
		},
	}

	var c *k3d.Cluster
	var err error
	nodes := make([]types.Node, 0)

	if c, err = client.ClusterGet(context.Background(), runtimes.SelectedRuntime, cfg); err != nil {
		return nodes, err
	}

	for _, n := range c.Nodes {
		nodeWrapper := types.Node{
			Master:            n.Role == k3d.ServerRole,
			RollBack:          false,
			InstanceID:        n.Name,
			InstanceStatus:    n.State.Status,
			InternalIPAddress: []string{},
			PublicIPAddress:   []string{},
			LocalHostname:     "",
		}
		// save to the store because of the rollback logic needed.
		p.M.Store(n.Name, nodeWrapper)
		// only scrape server and agent roles.
		if n.Role == k3d.ServerRole || n.Role == k3d.AgentRole {
			// ids = append(ids, n.Name)
			nodes = append(nodes, nodeWrapper)
		}
	}

	return nodes, nil
}

func (p *K3d) obtainKubeCfg() (kubeCfg, ip string, err error) {
	cfg := &k3d.Cluster{
		Name: p.Name,
		ServerLoadBalancer: &k3d.Loadbalancer{
			Config: &k3d.LoadbalancerConfig{},
		},
	}
	c, err := client.ClusterGet(context.Background(), runtimes.SelectedRuntime, cfg)
	if err != nil {
		return
	}

	kubeConfig, err := client.KubeconfigGet(context.Background(), runtimes.SelectedRuntime, c)
	if err != nil {
		return
	}

	bytes, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return
	}

	kubeCfg = string(bytes)
	ip = strings.Split(p.APIPort, ":")[0]

	return
}

func (p *K3d) createK3d(_ *types.SSH) (*types.Cluster, error) {
	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.Logger.Infof("[%s] %d masters and %d workers will be added", p.GetProviderName(), masterNum, workerNum)

	if p.Token == "" {
		token, err := utils.RandomToken(16)
		if err != nil {
			return nil, fmt.Errorf("[%s] generate token error: %w", p.GetProviderName(), err)
		}
		p.Token = token
	}

	cfg, err := p.wrapCliFlags(masterNum, workerNum)
	if err != nil {
		return nil, err
	}

	p.SetLogLevelAndOutput()

	if err := client.ClusterRun(context.Background(), runtimes.SelectedRuntime, cfg); err != nil {
		return nil, fmt.Errorf("[%s] cluster %s run failed: %w", p.GetProviderName(), p.Name, err)
	}

	if _, err := p.k3dStatus(); err != nil {
		return nil, err
	}

	p.M.Range(func(_, val interface{}) bool {
		v := val.(types.Node)
		v.RollBack = true
		p.M.Store(v.InstanceID, v)
		return true
	})

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	c.ContextName = p.ContextName

	return c, nil
}

func (p *K3d) joinK3d(ssh *types.SSH) (*types.Cluster, error) {
	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.Logger.Infof("[%s] %d masters and %d workers will be added", p.GetProviderName(), masterNum, workerNum)

	nodes := make([]*k3d.Node, 0)

	_, exists, err := p.IsClusterExist()
	if err != nil {
		return nil, err
	}

	serverIndexes := make([]int, 0)
	agentIndexes := make([]int, 0)

	for _, exist := range exists {
		if strings.Contains(exist, "-server-") {
			s := exist[strings.LastIndex(exist, "-")+1:]
			i, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
			serverIndexes = append(serverIndexes, i)
		} else if strings.Contains(exist, "-agent-") {
			s := exist[strings.LastIndex(exist, "-")+1:]
			i, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
			agentIndexes = append(agentIndexes, i)
		}
	}

	sort.Ints(serverIndexes)
	sort.Ints(agentIndexes)

	index := 0
	if len(serverIndexes) > 0 {
		index = serverIndexes[len(serverIndexes)-1] + 1
	}
	for i := 0; i < masterNum; i++ {
		node := &k3d.Node{
			Name:  fmt.Sprintf("%s-%s-%s-%d", k3d.DefaultObjectNamePrefix, p.Name, "server", index+i),
			Role:  k3d.ServerRole,
			Image: p.Image,
			K3sNodeLabels: map[string]string{
				k3d.LabelRole: string(k3d.ServerRole),
			},
			Restart: true,
		}
		if p.MastersMemory != "" {
			node.Memory = p.MastersMemory
		}
		nodes = append(nodes, node)
	}

	index = 0
	if len(agentIndexes) > 0 {
		index = agentIndexes[len(agentIndexes)-1] + 1
	}
	for i := 0; i < workerNum; i++ {
		node := &k3d.Node{
			Name:  fmt.Sprintf("%s-%s-%s-%d", k3d.DefaultObjectNamePrefix, p.Name, "agent", index+i),
			Role:  k3d.AgentRole,
			Image: p.Image,
			K3sNodeLabels: map[string]string{
				k3d.LabelRole: string(k3d.AgentRole),
			},
			Restart: true,
		}
		if p.WorkersMemory != "" {
			node.Memory = p.WorkersMemory
		}
		nodes = append(nodes, node)
	}

	p.SetLogLevelAndOutput()

	if err := client.NodeAddToClusterMulti(context.Background(), runtimes.SelectedRuntime, nodes, &k3d.Cluster{Name: p.Name},
		k3d.NodeCreateOpts{}); err != nil {
		return nil, err
	}

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	c.ContextName = p.ContextName
	c.SSH = *ssh

	return c, nil
}

func (p *K3d) deleteK3d(f bool) (string, error) {
	p.GenerateClusterName()
	exist, _, err := p.IsClusterExist()
	if err != nil {
		return "", fmt.Errorf("[%s] calling delete cluster error, msg: %v", p.GetProviderName(), err)
	}

	if !exist {
		p.Logger.Errorf("[%s] cluster %s is not exist", p.GetProviderName(), p.Name)
		if !f {
			return "", fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", p.GetProviderName(), p.Name)
		}
		return p.ContextName, nil
	}

	cfg := &k3d.Cluster{
		Name: p.Name,
	}

	p.SetLogLevelAndOutput()

	if err := client.ClusterDelete(context.Background(), runtimes.SelectedRuntime, cfg, k3d.ClusterDeleteOpts{}); err != nil {
		return "", fmt.Errorf("[%s] calling delete cluster error, msg: %v", p.GetProviderName(), err)
	}

	p.Logger.Infof("[%s] successfully delete cluster %s", p.GetProviderName(), p.Name)
	return p.ContextName, nil
}

func (p *K3d) isNodeRunning(status string) bool {
	return status == "running"
}

func (p *K3d) attachNode(id string, cluster *types.Cluster) error {
	var node types.Node

	for _, n := range cluster.Status.MasterNodes {
		if n.InstanceID == id {
			node = n
			break
		}
	}

	for _, n := range cluster.Status.WorkerNodes {
		if n.InstanceID == id {
			node = n
			break
		}
	}

	// init docker shell.
	shell, err := dialer.NewDockerShell(&node)
	if err != nil {
		return err
	}

	stdin, _, stderr := term.StdStreams()
	shell.SetStdio(nil, stderr, stdin)

	return shell.Terminal()
}

// func (p *K3d) getK3dContainer(node *k3d.Node) (*dockertypes.Container, error) {
// 	// (0) create docker client.
// 	docker, err := dockerutils.GetDockerClient()
// 	if err != nil {
// 		return nil, fmt.Errorf("[%s] failed to get docker client: %w", p.GetProviderName(), err)
// 	}
// 	defer func() {
// 		_ = docker.Close()
// 	}()

// 	// (1) list containers which have the default k3d labels attached.
// 	f := filters.NewArgs()
// 	for k, v := range node.K3sNodeLabels {
// 		f.Add("label", fmt.Sprintf("%s=%s", k, v))
// 	}

// 	// regex filtering for exact name match.
// 	// Assumptions:
// 	// -> container names start with a / (see https://github.com/moby/moby/issues/29997).
// 	// -> user input may or may not have the "k3d-" prefix.
// 	f.Add("name", fmt.Sprintf("^/?(%s-)?%s$", k3d.DefaultObjectNamePrefix, node.Name))

// 	containers, err := docker.ContainerList(context.Background(), containertypes.ListOptions{
// 		Filters: f,
// 		All:     true,
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("[%s] failed to list containers: %+v", p.GetProviderName(), err)
// 	}

// 	if len(containers) > 1 {
// 		return nil, fmt.Errorf("[%s] failed to get a single container for name '%s'. found: %d", p.GetProviderName(), node.Name, len(containers))
// 	}

// 	if len(containers) == 0 {
// 		return nil, fmt.Errorf("[%s] didn't find container for node '%s'", p.GetProviderName(), node.Name)
// 	}

// 	return &containers[0], nil
// }

func (p *K3d) wrapCliFlags(masters, workers int) (*k3dconf.ClusterConfig, error) {
	ipPorts := strings.Split(p.APIPort, ":")

	cfg := k3dconf.SimpleConfig{
		TypeMeta: k3dtypes.TypeMeta{
			APIVersion: config.DefaultConfigApiVersion,
			Kind:       "Simple",
		},
		ObjectMeta: k3dtypes.ObjectMeta{
			Name: p.Name,
		},
		Servers:      masters,
		Agents:       workers,
		ClusterToken: p.Token,
		Image:        p.Image,
		ExposeAPI: k3dconf.SimpleExposureOpts{
			HostIP:   ipPorts[0],
			HostPort: ipPorts[1],
		},
		Options: k3dconf.SimpleConfigOptions{
			Runtime: k3dconf.SimpleConfigOptionsRuntime{},
			K3dOptions: k3dconf.SimpleConfigOptionsK3d{
				DisableImageVolume:  p.NoImageVolume,
				DisableLoadbalancer: p.NoLB,
			},
			K3sOptions: k3dconf.SimpleConfigOptionsK3s{},
		},
	}

	if p.APIPort != "" {
		exposeAPI, err := k3dutil.ParsePortExposureSpec(p.APIPort, k3d.DefaultAPIPort)
		if err != nil {
			return nil, fmt.Errorf("[%s] cluster %s parse port config failed: %w", p.GetProviderName(), p.Name, err)
		}

		cfg.ExposeAPI.HostIP = exposeAPI.Binding.HostIP

		if exposeAPI.Binding.HostPort == "0" {
			exposeAPI, err = k3dutil.ParsePortExposureSpec("random", k3d.DefaultAPIPort)
			if err != nil {
				return nil, fmt.Errorf("[%s] cluster %s parse random port config failed: %w", p.GetProviderName(), p.Name, err)
			}
		}

		cfg.ExposeAPI.HostPort = exposeAPI.Binding.HostPort
		p.APIPort = fmt.Sprintf("%s:%s", cfg.ExposeAPI.HostIP, cfg.ExposeAPI.HostPort)
	}

	if p.GPUs != "" {
		cfg.Options.Runtime.GPURequest = p.GPUs
	}

	if p.Network != "" {
		cfg.Network = p.Network
	}

	if p.MastersMemory != "" {
		cfg.Options.Runtime.ServersMemory = p.MastersMemory
	}

	if p.WorkersMemory != "" {
		cfg.Options.Runtime.AgentsMemory = p.WorkersMemory
	}

	if p.MasterExtraArgs != "" {
		cfg.Options.K3sOptions.ExtraArgs = []k3dconf.K3sArgWithNodeFilters{}
		for _, arg := range strings.Split(p.MasterExtraArgs, " ") {
			cfg.Options.K3sOptions.ExtraArgs = append(cfg.Options.K3sOptions.ExtraArgs, k3dconf.K3sArgWithNodeFilters{
				Arg: arg,
				NodeFilters: []string{
					"server:*",
				},
			})
		}
	}

	if p.WorkerExtraArgs != "" {
		cfg.Options.K3sOptions.ExtraArgs = []k3dconf.K3sArgWithNodeFilters{}
		for _, arg := range strings.Split(p.WorkerExtraArgs, " ") {
			cfg.Options.K3sOptions.ExtraArgs = append(cfg.Options.K3sOptions.ExtraArgs, k3dconf.K3sArgWithNodeFilters{
				Arg: arg,
				NodeFilters: []string{
					"agent:*",
				},
			})
		}
	}

	registry, err := utils.VerifyRegistryFileContent(p.Registry, p.RegistryContent)
	if err != nil {
		return nil, err
	}

	content, err := utils.RegistryToString(registry)
	if err != nil {
		return nil, err
	}

	cfg.Registries.Config = content

	// volumeFilterMap will map volume mounts to applied node filters.
	if len(p.Volumes) > 0 {
		volumeFilterMap := make(map[string][]string, 1)
		for _, volumeFlag := range p.Volumes {
			// split node filter from the specified volume.
			volume, filters, err := cliutil.SplitFiltersFromFlag(volumeFlag)
			if err != nil {
				return nil, fmt.Errorf("[%s] cluster %s parse volume config failed: %w", p.GetProviderName(), p.Name, err)
			}

			// create new entry or append filter to existing entry.
			if _, exists := volumeFilterMap[volume]; exists {
				volumeFilterMap[volume] = append(volumeFilterMap[volume], filters...)
			} else {
				volumeFilterMap[volume] = filters
			}
		}

		for volume, nodeFilters := range volumeFilterMap {
			cfg.Volumes = append(cfg.Volumes, k3dconf.VolumeWithNodeFilters{
				Volume:      volume,
				NodeFilters: nodeFilters,
			})
		}
	}

	if len(p.Ports) > 0 {
		// portFilterMap will map ports to applied node filters.
		portFilterMap := make(map[string][]string, 1)
		for _, portFlag := range p.Ports {
			// split node filter from the specified volume.
			portMap, filters, err := cliutil.SplitFiltersFromFlag(portFlag)
			if err != nil {
				return nil, fmt.Errorf("[%s] cluster %s parse ports config failed: %w", p.GetProviderName(), p.Name, err)
			}

			if len(filters) > 1 {
				return nil, fmt.Errorf("[%s] cluster %s parse ports config failed: can only apply a Portmap to one node", p.GetProviderName(), p.Name)
			}

			// create new entry or append filter to existing entry.
			if _, exists := portFilterMap[portMap]; exists {
				return nil, fmt.Errorf("[%s] cluster %s parse ports config failed: same Portmapping can not be used for multiple nodes", p.GetProviderName(), p.Name)
			}

			portFilterMap[portMap] = filters
		}

		for port, nodeFilters := range portFilterMap {
			cfg.Ports = append(cfg.Ports, k3dconf.PortWithNodeFilters{
				Port:        port,
				NodeFilters: nodeFilters,
			})
		}
	}

	if len(p.Labels) > 0 {
		// labelFilterMap will add container label to applied node filters.
		labelFilterMap := make(map[string][]string, 1)
		for _, labelFlag := range p.Labels {
			// split node filter from the specified label.
			label, nodeFilters, err := cliutil.SplitFiltersFromFlag(labelFlag)
			if err != nil {
				return nil, fmt.Errorf("[%s] cluster %s parse labels config failed: %w", p.GetProviderName(), p.Name, err)
			}

			// create new entry or append filter to existing entry.
			if _, exists := labelFilterMap[label]; exists {
				labelFilterMap[label] = append(labelFilterMap[label], nodeFilters...)
			} else {
				labelFilterMap[label] = nodeFilters
			}
		}

		for label, nodeFilters := range labelFilterMap {
			cfg.Options.K3sOptions.NodeLabels = append(cfg.Options.K3sOptions.NodeLabels, k3dconf.LabelWithNodeFilters{
				Label:       label,
				NodeFilters: nodeFilters,
			})
		}
	}

	if len(p.Envs) > 0 {
		// envFilterMap will add container env vars to applied node filters.
		envFilterMap := make(map[string][]string, 1)
		for _, envFlag := range p.Envs {
			// split node filter from the specified env var.
			env, filters, err := cliutil.SplitFiltersFromFlag(envFlag)
			if err != nil {
				return nil, fmt.Errorf("[%s] cluster %s parse envs config failed: %w", p.GetProviderName(), p.Name, err)
			}

			// create new entry or append filter to existing entry.
			if _, exists := envFilterMap[env]; exists {
				envFilterMap[env] = append(envFilterMap[env], filters...)
			} else {
				envFilterMap[env] = filters
			}
		}

		for envVar, nodeFilters := range envFilterMap {
			cfg.Env = append(cfg.Env, k3dconf.EnvVarWithNodeFilters{
				EnvVar:      envVar,
				NodeFilters: nodeFilters,
			})
		}
	}

	c, err := config.TransformSimpleToClusterConfig(context.Background(), runtimes.SelectedRuntime, cfg)
	if err != nil {
		return nil, fmt.Errorf("[%s] cluster %s transform simple config failed: %w", p.GetProviderName(), p.Name, err)
	}

	c, err = config.ProcessClusterConfig(*c)
	if err != nil {
		return nil, fmt.Errorf("[%s] cluster %s process config failed: %w", p.GetProviderName(), p.Name, err)
	}

	if err := config.ValidateClusterConfig(context.Background(), runtimes.SelectedRuntime, *c); err != nil {
		return nil, fmt.Errorf("[%s] cluster %s configuration validation failed: %w", p.GetProviderName(), p.Name, err)
	}

	return c, nil
}

func (p *K3d) SetLogLevelAndOutput() {
	k3dlogger.Logger.SetLevel(p.Logger.Level)
	k3dlogger.Logger.SetOutput(p.Logger.Out)
}
