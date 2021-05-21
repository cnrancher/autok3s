package k3d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	typesk3d "github.com/cnrancher/autok3s/pkg/types/k3d"
	"github.com/cnrancher/autok3s/pkg/utils"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerunits "github.com/docker/go-units"
	"github.com/moby/term"
	cliutil "github.com/rancher/k3d/v4/cmd/util"
	k3dutil "github.com/rancher/k3d/v4/cmd/util"
	"github.com/rancher/k3d/v4/pkg/client"
	"github.com/rancher/k3d/v4/pkg/config"
	conf "github.com/rancher/k3d/v4/pkg/config/v1alpha2"
	"github.com/rancher/k3d/v4/pkg/runtimes"
	dockerutils "github.com/rancher/k3d/v4/pkg/runtimes/docker"
	k3d "github.com/rancher/k3d/v4/pkg/types"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	providerName = "k3d"

	k3dConfigVersion = "k3d.io/v1alpha2"
	k3dConfigKind    = "Simple"
	k3dImage         = "rancher/k3s:v1.20.5-k3s1"
	k3dAPIPort       = "0.0.0.0:0"
)

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

func (p *K3d) GetProviderName() string {
	return p.Provider
}

func (p *K3d) GenerateClusterName() string {
	// must comply with the k3d cluster name rules.
	p.ContextName = fmt.Sprintf("%s-%s", p.GetProviderName(), p.Name)
	return p.ContextName
}

func (p *K3d) CreateK3sCluster() (err error) {
	return p.InitCluster(p.Options, nil, p.createK3d, p.obtainKubeCfg, p.rollbackK3d)
}

func (p *K3d) JoinK3sNode() (err error) {
	return p.JoinNodes(p.joinK3d, p.syncK3d, true, p.rollbackK3d)
}

func (p *K3d) DeleteK3sCluster(f bool) (err error) {
	return p.DeleteCluster(f, p.deleteK3d)
}

func (p *K3d) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	return p.Connect(ip, nil, c, p.k3dStatus, p.isNodeRunning, p.attachNode)
}

func (p *K3d) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	cfg := &k3d.Cluster{
		Name: p.Name,
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

func (p *K3d) GetCluster(kubeConfig string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	if kubeConfig == "" {
		return c
	}
	return p.GetClusterStatus(kubeConfig, c, p.k3dStatus)
}

func (p *K3d) DescribeCluster(kubeConfig string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubeConfig, c, p.k3dStatus)
}

func (p *K3d) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &typesk3d.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

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

func (p *K3d) CreateCheck() error {
	masterNum, err := strconv.Atoi(p.Master)
	if masterNum < 1 || err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--master` number must >= 1",
			p.GetProviderName())
	}

	exist, _, err := p.IsClusterExist()
	if err != nil && !errors.Is(err, client.ClusterGetNoNodesFoundError) {
		return err
	}

	if exist {
		return fmt.Errorf("[%s] calling preflight error: cluster `%s` is already exist",
			p.GetProviderName(), p.Name)
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

func (p *K3d) JoinCheck() error {
	// check cluster exist.
	exist, _, err := p.IsClusterExist()

	if err != nil {
		return err
	}

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist",
			p.GetProviderName(), p.ContextName)
	}

	// check flags.
	masterNum, err := strconv.Atoi(p.Master)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--master` must be number",
			p.GetProviderName())
	}
	workerNum, err := strconv.Atoi(p.Worker)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--worker` must be number",
			p.GetProviderName())
	}
	if masterNum < 1 && workerNum < 1 {
		return fmt.Errorf("[%s] calling preflight error: `--master` or `--worker` number must >= 1", p.GetProviderName())
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

func (p *K3d) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	return ""
}

func (p *K3d) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
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
	ids := make([]string, 0)

	cfg := &k3d.Cluster{
		Name: p.Name,
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
		}
		// save to the store because of the rollback logic needed.
		p.M.Store(n.Name, nodeWrapper)
		// only scrape server and agent roles.
		if n.Role == k3d.ServerRole || n.Role == k3d.AgentRole {
			ids = append(ids, n.Name)
			nodes = append(nodes, nodeWrapper)
		}
	}

	return nodes, nil
}

func (p *K3d) obtainKubeCfg() (cfg, ip string, err error) {
	c, err := client.ClusterGet(context.Background(), runtimes.SelectedRuntime, &k3d.Cluster{Name: p.Name})
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

	cfg = string(bytes)
	ip = strings.Split(p.APIPort, ":")[0]

	return
}

func (p *K3d) createK3d(ssh *types.SSH) (*types.Cluster, error) {
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

	p.redirectLogs()
	defer p.resetRedirectLogs()

	if err := client.ClusterRun(context.Background(), runtimes.SelectedRuntime, cfg); err != nil {
		return nil, fmt.Errorf("[%s] cluster %s run failed: %w", p.GetProviderName(), p.Name, err)
	}

	if _, err := p.k3dStatus(); err != nil {
		return nil, err
	}

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

	for i := 0; i < masterNum; i++ {
		index := 0
		if len(serverIndexes)-1 > 0 {
			index = serverIndexes[len(serverIndexes)-1]
		}
		node := &k3d.Node{
			Name:  fmt.Sprintf("%s-%s-%s-%d", k3d.DefaultObjectNamePrefix, p.Name, "server", index+1+i),
			Role:  k3d.ServerRole,
			Image: p.Image,
			Labels: map[string]string{
				k3d.LabelRole: "server",
			},
			Restart: true,
		}
		if p.MastersMemory != "" {
			node.Memory = p.MastersMemory
		}
		nodes = append(nodes, node)
	}

	for i := 0; i < workerNum; i++ {
		index := 0
		if len(agentIndexes)-1 > 0 {
			index = agentIndexes[len(agentIndexes)-1]
		}
		node := &k3d.Node{
			Name:  fmt.Sprintf("%s-%s-%s-%d", k3d.DefaultObjectNamePrefix, p.Name, "agent", index+1+i),
			Role:  k3d.AgentRole,
			Image: p.Image,
			Labels: map[string]string{
				k3d.LabelRole: "agent",
			},
			Restart: true,
		}
		if p.WorkersMemory != "" {
			node.Memory = p.WorkersMemory
		}
		nodes = append(nodes, node)
	}

	p.redirectLogs()
	defer p.resetRedirectLogs()

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

	p.redirectLogs()
	defer p.resetRedirectLogs()

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

	// init docker dialer.
	dialer, err := hosts.NewDockerDialer(&node)
	if err != nil {
		return err
	}

	stdin, _, stderr := term.StdStreams()
	dialer.SetWriter(p.Logger.Out).SetStdio(nil, stderr, stdin)

	return dialer.Terminal()
}

func (p *K3d) getK3dContainer(node *k3d.Node) (*dockertypes.Container, error) {
	// (0) create docker client.
	docker, err := dockerutils.GetDockerClient()
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to get docker client: %w", p.GetProviderName(), err)
	}
	defer func() {
		_ = docker.Close()
	}()

	// (1) list containers which have the default k3d labels attached.
	f := filters.NewArgs()
	for k, v := range node.Labels {
		f.Add("label", fmt.Sprintf("%s=%s", k, v))
	}

	// regex filtering for exact name match.
	// Assumptions:
	// -> container names start with a / (see https://github.com/moby/moby/issues/29997).
	// -> user input may or may not have the "k3d-" prefix.
	f.Add("name", fmt.Sprintf("^/?(%s-)?%s$", k3d.DefaultObjectNamePrefix, node.Name))

	containers, err := docker.ContainerList(context.Background(), dockertypes.ContainerListOptions{
		Filters: f,
		All:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to list containers: %+v", p.GetProviderName(), err)
	}

	if len(containers) > 1 {
		return nil, fmt.Errorf("[%s] failed to get a single container for name '%s'. found: %d", p.GetProviderName(), node.Name, len(containers))
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("[%s] didn't find container for node '%s'", p.GetProviderName(), node.Name)
	}

	return &containers[0], nil
}

func (p *K3d) wrapCliFlags(masters, workers int) (*conf.ClusterConfig, error) {
	ipPorts := strings.Split(p.APIPort, ":")

	cfg := conf.SimpleConfig{
		TypeMeta: conf.TypeMeta{
			APIVersion: k3dConfigVersion,
			Kind:       k3dConfigKind,
		},
		Name:         p.Name,
		Servers:      masters,
		Agents:       workers,
		ClusterToken: p.Token,
		Image:        p.Image,
		ExposeAPI: conf.SimpleExposureOpts{
			HostIP:   ipPorts[0],
			HostPort: ipPorts[1],
		},
		Options: conf.SimpleConfigOptions{
			Runtime: conf.SimpleConfigOptionsRuntime{},
			K3dOptions: conf.SimpleConfigOptionsK3d{
				DisableImageVolume:         p.NoImageVolume,
				DisableLoadbalancer:        p.NoLB,
				PrepDisableHostIPInjection: p.NoHostIP,
			},
			K3sOptions: conf.SimpleConfigOptionsK3s{},
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
		cfg.Options.K3sOptions.ExtraServerArgs = strings.Split(p.MasterExtraArgs, " ")
	}

	if p.WorkerExtraArgs != "" {
		cfg.Options.K3sOptions.ExtraAgentArgs = strings.Split(p.WorkerExtraArgs, " ")
	}

	if p.Registry != "" {
		cfg.Registries.Config = p.Registry
	}

	// volumeFilterMap will map volume mounts to applied node filters.
	if len(p.Volumes) > 0 {
		volumeFilterMap := make(map[string][]string, 1)
		for _, volumeFlag := range p.Volumes {
			// split node filter from the specified volume
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
			cfg.Volumes = append(cfg.Volumes, conf.VolumeWithNodeFilters{
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
			cfg.Ports = append(cfg.Ports, conf.PortWithNodeFilters{
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
			cfg.Labels = append(cfg.Labels, conf.LabelWithNodeFilters{
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
			cfg.Env = append(cfg.Env, conf.EnvVarWithNodeFilters{
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

func (p *K3d) redirectLogs() {
	logrus.SetOutput(p.Logger.Out)
}

func (p *K3d) resetRedirectLogs() {
	logrus.SetOutput(os.Stderr)
}
