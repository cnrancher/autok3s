package native

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/native"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/rancher/wrangler/v2/pkg/slice"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func (p *Native) GenerateMasterExtraArgs(_ *types.Cluster, _ types.Node) string {
	// no need to support.
	return ""
}

// GenerateWorkerExtraArgs generates K3S worker extra args.
func (p *Native) GenerateWorkerExtraArgs(_ *types.Cluster, _ types.Node) string {
	// no need to support.
	return ""
}

// CreateK3sCluster create K3S cluster.
func (p *Native) CreateK3sCluster() (err error) {
	// set ssh default value.
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	if p.SSHKey == "" && p.SSHKeyName == "" && p.SSHPassword == "" && p.SSHKeyPath == "" {
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

	c, err := p.assembleNodeStatus(&p.SSH)
	if err != nil {
		return err
	}

	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return err
	}
	if state == nil && p.IP != "" {
		// if cluster is not exist then save it
		c.Status.Status = common.StatusRunning
		c.Status.Standalone = true
		err = common.DefaultDB.SaveCluster(c)
		if err != nil {
			return err
		}
	}

	return p.JoinNodes(func(_ *types.SSH) (*types.Cluster, error) {
		return c, nil
	}, p.syncNodes, false, p.rollbackInstance)
}

func (p *Native) rollbackInstance(ids []string) error {
	nodes := make([]types.Node, 0)
	for _, id := range ids {
		if node, ok := p.M.Load(id); ok {
			nodes = append(nodes, node.(types.Node))
		}
	}
	kubeCfg := filepath.Join(common.CfgPath, common.KubeCfgFile)
	client, err := cluster.GetClusterConfig(p.ContextName, kubeCfg)
	if err != nil {
		p.Logger.Errorf("[%s] failed to get kubeclient for rollback: %v", p.GetProviderName(), err)
	}
	if err == nil {
		timeout := int64(5 * time.Second)
		nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{TimeoutSeconds: &timeout})
		if err == nil && nodeList != nil {
			rNodeList := []v1.Node{}
			for _, id := range ids {
				if v, ok := p.M.Load(id); ok {
					n := v.(types.Node)
					for _, node := range nodeList.Items {
						var externalIP string
						addressList := node.Status.Addresses
						for _, address := range addressList {
							switch address.Type {
							case v1.NodeExternalIP:
								externalIP = address.Address
							default:
								continue
							}
						}
						if n.PublicIPAddress[0] == externalIP {
							rNodeList = append(rNodeList, node)
						}
					}
				}
			}
			if len(rNodeList) > 0 {
				for _, rNode := range rNodeList {
					p.Logger.Infof("[%s] remove node %s for rollback", p.GetProviderName(), rNode.Name)
					e := client.CoreV1().Nodes().Delete(context.TODO(), rNode.Name, metav1.DeleteOptions{})
					if e != nil {
						p.Logger.Errorf("[%s] failed to remove node %s for rollback: %v", p.GetProviderName(), rNode.Name, err)
					}
				}
			}
		}
	}

	if err != nil {
		p.Logger.Errorf("[%s] failed to get node list for rollback: %v", p.GetProviderName(), err)
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
		return fmt.Errorf("[%s] calling preflight error: cluster must have one master when create", p.GetProviderName())
	}

	masterList := strings.Split(p.MasterIps, ",")
	if len(masterList) > 1 && !p.Cluster && p.DataStore == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--cluster` or `--datastore` for HA mode",
			p.Provider)
	}

	return p.CheckCreateArgs(func() (bool, []string, error) {
		return false, []string{}, nil
	})
}

// JoinCheck check join command and flags.
func (p *Native) JoinCheck() error {
	if p.MasterIps == "" && p.WorkerIps == "" {
		return fmt.Errorf("[%s] calling preflight error: cluster must have one node when join", p.GetProviderName())
	}
	masterList := strings.Split(p.MasterIps, ",")
	if len(masterList) > 1 && !p.Cluster && p.DataStore == "" {
		return fmt.Errorf("[%s] calling preflight error: can't join master nodes to single node cluster", p.GetProviderName())
	}
	// check --ip if cluster is not exist(for previous version)
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return err
	}
	if state == nil && p.IP == "" {
		return fmt.Errorf("[%s] calling preflight error: cluster %s is not exist", p.GetProviderName(), p.Name)
	}
	// check file exists.
	if p.SSHKeyPath != "" && !utils.IsFileExists(p.SSHKeyPath) {
		return fmt.Errorf("[%s] calling preflight error: failed to get ssh-key-path", p.GetProviderName())
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
	return p.Connect(ip, &p.SSH, c, p.syncInstanceNodes, p.isInstanceRunning, nil)
}

// DescribeCluster describe cluster info.
func (p *Native) DescribeCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubecfg, c, p.syncInstanceNodes)
}

// GetCluster returns cluster status.
func (p *Native) GetCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
	}

	return p.GetClusterStatus(kubecfg, c, p.syncInstanceNodes)
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
		p.Master = strconv.Itoa(len(masterIps))
		p.syncNodesMap(masterIps, true, ssh)
	}

	if p.WorkerIps != "" {
		workerIps := strings.Split(p.WorkerIps, ",")
		p.Worker = strconv.Itoa(len(workerIps))
		p.syncNodesMap(workerIps, false, ssh)
	}

	p.M.Range(func(_, value interface{}) bool {
		v := value.(types.Node)
		nodes := p.Status.WorkerNodes
		if v.Master {
			nodes = p.Status.MasterNodes
		}
		index, b := putil.IsExistedNodes(nodes, v.InstanceID)
		if b {
			nodes[index].Current = false
			nodes[index].RollBack = false
			v.Current = false
			v.RollBack = false
			p.M.Store(v.InstanceID, v)
		}

		return true
	})

	return &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
		SSH:      *ssh,
	}, nil
}

func (p *Native) syncNodes() error {
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
			LocalHostname:     "",
			Current:           true,
			SSH:               *ssh,
		})
	}
}

func (p *Native) syncInstanceNodes() ([]types.Node, error) {
	nodes, err := p.getInstanceNodes()
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClusterConfig(p.ContextName, filepath.Join(common.CfgPath, common.KubeCfgFile))
	if err != nil {
		return nodes, nil
	}
	nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil || nodeList == nil {
		return nodes, nil
	}
	if len(nodes) == len(nodeList.Items) {
		return nodes, nil
	}
	masterNodes := p.Status.MasterNodes
	workerNodes := p.Status.WorkerNodes
	for _, node := range nodeList.Items {
		var internalIP, externalIP string
		addressList := node.Status.Addresses
		for _, address := range addressList {
			switch address.Type {
			case v1.NodeInternalIP:
				internalIP = address.Address
			case v1.NodeExternalIP:
				externalIP = address.Address
			default:
				continue
			}
		}
		isExists := false
		for _, insNode := range nodes {
			if slice.ContainsString(insNode.PublicIPAddress, externalIP) {
				isExists = true
				break
			}
		}
		if !isExists {
			// save node information
			_, isMaster := node.Labels["node-role.kubernetes.io/master"]
			insID := strings.Replace(externalIP, ".", "-", -1)
			instanceNode := types.Node{
				InstanceID:        insID,
				InstanceStatus:    "-",
				InternalIPAddress: []string{internalIP},
				PublicIPAddress:   []string{externalIP},
				Master:            isMaster,
				Standalone:        true,
			}
			if isMaster {
				masterNodes = append(masterNodes, instanceNode)
			} else {
				workerNodes = append(workerNodes, instanceNode)
			}
			nodes = append(nodes, instanceNode)
		}
	}
	state, err := common.DefaultDB.GetCluster(p.Name, p.Provider)
	if err != nil {
		return nodes, err
	}

	// sync cluster nodes
	masterBytes, _ := json.Marshal(masterNodes)
	workerBytes, _ := json.Marshal(workerNodes)

	// check difference
	if !reflect.DeepEqual(state.MasterNodes, masterBytes) ||
		!reflect.DeepEqual(state.WorkerNodes, workerBytes) ||
		!strings.EqualFold(state.Master, strconv.Itoa(len(masterNodes))) ||
		!strings.EqualFold(state.Worker, strconv.Itoa(len(workerNodes))) {
		state.MasterNodes = masterBytes
		state.WorkerNodes = workerBytes
		state.Master = strconv.Itoa(len(masterNodes))
		state.Worker = strconv.Itoa(len(workerNodes))
		err = common.DefaultDB.SaveClusterState(state)
	}

	return nodes, err
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

func (p *Native) uninstallCluster(_ bool) (string, error) {
	// don't uninstall cluster if it's not handled by autok3s
	if p.Status.Standalone {
		p.Logger.Infof("[%s] cluster %s is not handled by autok3s, we won't uninstall the cluster automatically", p.GetProviderName(), p.Name)
		return p.ContextName, nil
	}
	warnMsg := p.UninstallK3sNodes(p.Status.MasterNodes)
	warnMsg = append(warnMsg, p.UninstallK3sNodes(p.Status.WorkerNodes)...)
	if len(warnMsg) > 0 {
		p.Logger.Warnf("[%s] %s", p.GetProviderName(), warnMsg)
	}
	return p.ContextName, nil
}

func (p *Native) isInstanceRunning(_ string) bool {
	return true
}
