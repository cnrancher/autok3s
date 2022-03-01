package native

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/native"
	"github.com/cnrancher/autok3s/pkg/utils"
)

// providerName is the name of this provider.
const providerName = "native"

var (
	defaultUser       = "root"
	defaultSSHKeyPath = "~/.ssh/id_rsa"
)

// Native provider native struct.
type Native struct {
	*cluster.ProviderBase `json:",inline"`
	native.Options        `json:",inline"`
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Native {
	base := cluster.NewBaseProvider()
	base.Provider = providerName
	return &Native{
		ProviderBase: base,
	}
}

// GetProviderName returns provider name.
func (p *Native) GetProviderName() string {
	return p.Provider
}

// GenerateClusterName generates and returns cluster name.
func (p *Native) GenerateClusterName() string {
	p.ContextName = p.Name
	return p.ContextName
}

// GenerateManifest generates manifest deploy command.
func (p *Native) GenerateManifest() []string {
	// no need to support.
	return nil
}

// GenerateMasterExtraArgs generates K3S master extra args.
func (p *Native) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	// no need to support.
	return ""
}

// GenerateWorkerExtraArgs generates K3S worker extra args.
func (p *Native) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	// no need to support.
	return ""
}

// CreateK3sCluster create K3S cluster.
func (p *Native) CreateK3sCluster() (err error) {
	// set ssh default value.
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	if p.SSHPassword == "" && p.SSHKeyPath == "" {
		p.SSHKeyPath = defaultSSHKeyPath
	}

	return p.InitCluster(p.Options, p.GenerateManifest, p.assembleNodeStatus, nil, p.rollbackInstance)
}

// JoinK3sNode join K3S node.
func (p *Native) JoinK3sNode() (err error) {
	// set ssh default value.
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	if p.SSHPassword == "" && p.SSHKeyPath == "" {
		p.SSHKeyPath = defaultSSHKeyPath
	}

	return p.JoinNodes(p.assembleNodeStatus, p.syncNodes, false, p.rollbackInstance)
}

func (p *Native) rollbackInstance(ids []string) error {
	nodes := make([]types.Node, 0)
	for _, id := range ids {
		if node, ok := p.M.Load(id); ok {
			nodes = append(nodes, node.(types.Node))
		}
	}
	warnMsg := p.UninstallK3sNodes(nodes)
	for _, w := range warnMsg {
		p.Logger.Warnf("[%s] %s", p.GetProviderName(), w)
	}
	return nil
}

// CreateCheck check create command and flags.
func (p *Native) CreateCheck() error {
	if p.MasterIps == "" {
		return fmt.Errorf("[%s] cluster must have one master when create", p.GetProviderName())
	}

	// check file exists.
	if p.SSHKeyPath != "" {
		sshPrivateKey := p.SSHKeyPath
		if strings.HasPrefix(sshPrivateKey, "~/") {
			baseDir := getUserHomeDir()
			if baseDir == "" {
				return fmt.Errorf("[%s] failed to get user home directory for %s, please set with absolute file path", p.GetProviderName(), sshPrivateKey)
			}
			sshPrivateKey = filepath.Join(baseDir, sshPrivateKey[2:])
		}
		if _, err := os.Stat(sshPrivateKey); err != nil {
			return err
		}
	}

	// check name exist.
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return err
	}

	if state != nil && state.Status != common.StatusFailed {
		return fmt.Errorf("[%s] cluster %s is already exist", p.GetProviderName(), p.Name)
	}

	return nil
}

// JoinCheck check join command and flags.
func (p *Native) JoinCheck() error {
	if p.MasterIps == "" && p.WorkerIps == "" {
		return fmt.Errorf("[%s] cluster must have one node when join", p.GetProviderName())
	}
	// check file exists.
	if p.SSHKeyPath != "" {
		sshPrivateKey := p.SSHKeyPath
		if strings.HasPrefix(sshPrivateKey, "~/") {
			baseDir := getUserHomeDir()
			if baseDir == "" {
				return fmt.Errorf("[%s] failed to get user home directory for %s, please set with absolute file path", p.GetProviderName(), sshPrivateKey)
			}
			sshPrivateKey = filepath.Join(baseDir, sshPrivateKey[2:])
		}
		if _, err := os.Stat(sshPrivateKey); err != nil {
			return err
		}
	}
	return nil
}

// DeleteK3sCluster delete K3S cluster.
func (p *Native) DeleteK3sCluster(f bool) error {
	return p.DeleteCluster(f, p.uninstallCluster)
}

// SSHK3sNode ssh K3s node.
func (p *Native) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	return p.Connect(ip, &p.SSH, c, p.getInstanceNodes, p.isInstanceRunning, nil)
}

// DescribeCluster describe cluster info.
func (p *Native) DescribeCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubecfg, c, p.getInstanceNodes)
}

// GetCluster returns cluster status.
func (p *Native) GetCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	if kubecfg == "" {
		return c
	}

	return p.GetClusterStatus(kubecfg, c, p.getInstanceNodes)
}

// IsClusterExist determine if the cluster exists.
func (p *Native) IsClusterExist() (bool, []string, error) {
	c, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return false, []string{}, err
	}
	return c != nil, []string{}, nil
}

// SetConfig set cluster config.
func (p *Native) SetConfig(config []byte) error {
	c, err := p.SetClusterConfig(config)
	if err != nil {
		return err
	}
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	b, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	opt := &native.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// SetOptions set options.
func (p *Native) SetOptions(opt []byte) error {
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	option := &native.Options{}
	err := json.Unmarshal(opt, option)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(option).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// GetProviderOptions get provider options.
func (p *Native) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &native.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

func (p *Native) assembleNodeStatus(ssh *types.SSH) (*types.Cluster, error) {
	if p.MasterIps != "" {
		masterIps := strings.Split(p.MasterIps, ",")
		p.syncNodesMap(masterIps, true, ssh)
	}

	if p.WorkerIps != "" {
		workerIps := strings.Split(p.WorkerIps, ",")
		p.syncNodesMap(workerIps, false, ssh)
	}

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
			nodes[index].Current = false
			nodes[index].RollBack = false
		}

		if v.Master {
			p.Status.MasterNodes = nodes
		} else {
			p.Status.WorkerNodes = nodes
		}
		return true
	})

	p.Master = strconv.Itoa(len(p.MasterNodes))
	p.Worker = strconv.Itoa(len(p.WorkerNodes))

	return &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
		SSH:      *ssh,
	}, nil
}

func (p *Native) syncNodes() error {
	masterNodes := p.Status.MasterNodes
	for _, node := range masterNodes {
		if value, ok := p.M.Load(node.InstanceID); ok {
			v := value.(types.Node)
			_, b := putil.IsExistedNodes(masterNodes, v.InstanceID)
			if b {
				v.Current = false
				v.RollBack = false
				p.M.Store(node.InstanceID, v)
				continue
			}
		}
	}

	workerNodes := p.Status.MasterNodes
	for _, node := range workerNodes {
		if value, ok := p.M.Load(node.InstanceID); ok {
			v := value.(types.Node)
			_, b := putil.IsExistedNodes(workerNodes, v.InstanceID)
			if b {
				v.Current = false
				v.RollBack = false
				p.M.Store(node.InstanceID, v)
				continue
			}
		}
	}

	return nil
}

func (p *Native) syncNodesMap(ipList []string, master bool, ssh *types.SSH) {
	for _, ip := range ipList {
		currentID := strings.Replace(ip, ".", "-", -1)
		p.M.Store(currentID, types.Node{
			Master:            master,
			RollBack:          true,
			InstanceID:        currentID,
			InstanceStatus:    "-",
			InternalIPAddress: []string{ip},
			PublicIPAddress:   []string{ip},
			Current:           true,
			SSH:               *ssh,
		})
	}
}

func (p *Native) getInstanceNodes() ([]types.Node, error) {
	nodes := []types.Node{}
	_, err := p.MergeConfig()
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, p.Status.MasterNodes...)
	nodes = append(nodes, p.Status.WorkerNodes...)

	return nodes, nil
}

func (p *Native) uninstallCluster(f bool) (string, error) {
	warnMsg := p.UninstallK3sNodes(p.Status.MasterNodes)
	warnMsg = append(warnMsg, p.UninstallK3sNodes(p.Status.WorkerNodes)...)
	if len(warnMsg) > 0 {
		p.Logger.Warnf("[%s] %s", p.GetProviderName(), warnMsg)
	}
	return p.ContextName, nil
}

func (p *Native) isInstanceRunning(state string) bool {
	return true
}

func getUserHomeDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		if u, err := user.Current(); err == nil {
			return u.HomeDir
		}
	}
	return home
}
