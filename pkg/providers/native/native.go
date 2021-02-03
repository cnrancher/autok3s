package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/native"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
)

const (
	k3sVersion       = ""
	k3sChannel       = "stable"
	k3sInstallScript = "http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh"
	ui               = false
)

// ProviderName is the name of this provider.
const ProviderName = "native"

var (
	k3sMirror         = "INSTALL_K3S_MIRROR=cn"
	dockerMirror      = ""
	defaultUser       = "root"
	defaultSSHKeyPath = "~/.ssh/id_rsa"
)

type Native struct {
	types.Metadata `json:",inline"`
	native.Options `json:",inline"`
	types.Status   `json:"status"`

	m      *sync.Map
	logger *logrus.Logger
}

func init() {
	providers.RegisterProvider(ProviderName, func() (providers.Provider, error) {
		return NewProvider(), nil
	})
}

func NewProvider() *Native {
	return &Native{
		Metadata: types.Metadata{
			Provider:      ProviderName,
			UI:            ui,
			K3sVersion:    k3sVersion,
			K3sChannel:    k3sChannel,
			InstallScript: k3sInstallScript,
			Cluster:       false,
		},
		Options: native.Options{
			MasterIps: "",
			WorkerIps: "",
		},
		Status: types.Status{
			MasterNodes: make([]types.Node, 0),
			WorkerNodes: make([]types.Node, 0),
		},
		m: new(syncmap.Map),
	}
}

func (p *Native) GetProviderName() string {
	return "native"
}

func (p *Native) GenerateClusterName() {
	// no need to support.
}

func (p *Native) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	// no need to support.
	return ""
}

func (p *Native) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	// no need to support.
	return ""
}

func (p *Native) CreateK3sCluster(ssh *types.SSH) (err error) {
	logFile, err := common.GetLogFile(p.Name)
	if err != nil {
		return err
	}
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	defer func() {
		if err != nil {
			p.logger.Errorf("[%s] failed to create cluster: %v", p.GetProviderName(), err)
			if c == nil {
				c = &types.Cluster{
					Metadata: p.Metadata,
					Options:  p.Options,
					Status:   p.Status,
				}
			}
			c.Status.Status = common.StatusFailed
			cluster.SaveClusterState(c, common.StatusFailed)
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusCreating)))
		}
		if err == nil && len(p.Status.MasterNodes) > 0 {
			p.logger.Info(common.UsageInfoTitle)
			p.logger.Infof(common.UsageContext, p.Name)
			p.logger.Info(common.UsagePods)
			if p.UI {
				p.logger.Infof("K3s UI URL: https://%s:8999", p.Status.MasterNodes[0].PublicIPAddress[0])
			}
			cluster.SaveClusterState(c, common.StatusRunning)
			// remove creating state file and save running state
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusCreating)))
		}
		logFile.Close()
	}()
	p.logger = common.NewLogger(common.Debug, logFile)
	p.logger.Infof("[%s] executing create logic...\n", p.GetProviderName())

	// set ssh default value
	if ssh.User == "" {
		ssh.User = defaultUser
	}
	if ssh.Password == "" && ssh.SSHKeyPath == "" {
		ssh.SSHKeyPath = defaultSSHKeyPath
	}
	c.Status.Status = common.StatusCreating
	err = cluster.SaveClusterState(c, common.StatusCreating)
	if err != nil {
		return err
	}

	// assemble node status.
	if c, err = p.assembleNodeStatus(ssh); err != nil {
		return err
	}

	c.Mirror = k3sMirror
	c.DockerMirror = dockerMirror
	c.Logger = p.logger
	// initialize K3s cluster.
	if err = cluster.InitK3sCluster(c); err != nil {
		return
	}
	p.logger.Infof("[%s] successfully executed create logic\n", p.GetProviderName())
	return nil
}

func (p *Native) JoinK3sNode(ssh *types.SSH) (err error) {
	if p.m == nil {
		p.m = new(syncmap.Map)
	}
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	logFile, err := common.GetLogFile(p.Name)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if c != nil {
				c.Status.Status = common.StatusFailed
				cluster.SaveClusterState(c, common.StatusFailed)
			}
		} else {
			cluster.SaveClusterState(c, common.StatusRunning)
		}
		// remove join state file and save running state
		os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusJoin)))
		logFile.Close()
	}()
	p.logger = common.NewLogger(common.Debug, logFile)
	p.logger.Infof("[%s] executing join logic...\n", p.GetProviderName())
	// set ssh default value
	if ssh.User == "" {
		ssh.User = defaultUser
	}
	if ssh.Password == "" && ssh.SSHKeyPath == "" {
		ssh.SSHKeyPath = defaultSSHKeyPath
	}

	c.Status.Status = "upgrading"
	err = cluster.SaveClusterState(c, common.StatusJoin)
	if err != nil {
		return err
	}

	// assemble node status.
	if c, err = p.assembleNodeStatus(ssh); err != nil {
		return err
	}

	added := &types.Cluster{
		Metadata: c.Metadata,
		Options:  c.Options,
		Status:   types.Status{},
	}

	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		// filter the number of nodes that are not generated by current command.
		if v.Current {
			if v.Master {
				added.Status.MasterNodes = append(added.Status.MasterNodes, v)
			} else {
				added.Status.WorkerNodes = append(added.Status.WorkerNodes, v)
			}
			// for rollback
			p.m.Store(v.InstanceID, types.Node{Master: v.Master, RollBack: true, InstanceID: v.InstanceID, InstanceStatus: v.InstanceStatus, PublicIPAddress: v.PublicIPAddress, InternalIPAddress: v.InternalIPAddress, SSH: v.SSH})
		}
		return true
	})

	var (
		masterIps []string
		workerIps []string
	)

	for _, masterNode := range c.Status.MasterNodes {
		masterIps = append(masterIps, masterNode.PublicIPAddress...)
	}

	for _, workerNode := range c.Status.WorkerNodes {
		workerIps = append(workerIps, workerNode.PublicIPAddress...)
	}

	p.Options.MasterIps = strings.Join(masterIps, ",")
	p.Options.WorkerIps = strings.Join(workerIps, ",")
	c.Logger = p.logger
	added.Logger = p.logger
	// join K3s node.
	if err := cluster.JoinK3sNode(c, added); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed join logic\n", p.GetProviderName())
	return nil
}

func (p *Native) Rollback() error {
	p.logger.Infof("[%s] executing rollback logic...\n", p.GetProviderName())

	ids := make([]string, 0)
	nodes := make([]types.Node, 0)
	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		if v.RollBack {
			ids = append(ids, key.(string))
			nodes = append(nodes, v)
		}
		return true
	})

	p.logger.Debugf("[%s] nodes %s will be rollback\n", p.GetProviderName(), ids)

	if len(ids) > 0 {
		warnMsg := cluster.UninstallK3sNodes(nodes)
		for _, w := range warnMsg {
			p.logger.Warnf("[%s] %s\n", p.GetProviderName(), w)
		}
	}

	p.logger.Infof("[%s] successfully executed rollback logic\n", p.GetProviderName())

	return nil
}

func (p *Native) CreateCheck(ssh *types.SSH) error {
	if p.MasterIps == "" {
		return fmt.Errorf("[%s] cluster must have one master when create", p.GetProviderName())
	}
	return nil
}

func (p *Native) DeleteK3sCluster(f bool) error {
	return p.CommandNotSupport("delete")
}

func (p *Native) SSHK3sNode(ssh *types.SSH, ip string) error {
	return p.CommandNotSupport("ssh")
}

func (p *Native) CommandNotSupport(commandName string) error {
	return fmt.Errorf("[%s] dose not support command: [%s]", p.GetProviderName(), commandName)
}

func (p *Native) DescribeCluster(kubecfg string) *types.ClusterInfo {
	return &types.ClusterInfo{}
}

func (p *Native) GetCluster(kubecfg string) *types.ClusterInfo {
	return &types.ClusterInfo{}
}

func (p *Native) IsClusterExist() (bool, []string, error) {
	return false, []string{}, nil
}

func (p *Native) GetClusterConfig() (map[string]schemas.Field, error) {
	config := p.GetSSHConfig()
	sshConfig, err := utils.ConvertToFields(*config)
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

func (p *Native) GetProviderOption() (map[string]schemas.Field, error) {
	return utils.ConvertToFields(p.Options)
}

func (p *Native) SetConfig(config []byte) error {
	c := types.Cluster{}
	err := json.Unmarshal(config, &c)
	if err != nil {
		return err
	}
	sourceMeta := reflect.ValueOf(&p.Metadata).Elem()
	targetMeta := reflect.ValueOf(&c.Metadata).Elem()
	utils.MergeConfig(sourceMeta, targetMeta)
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

func (p *Native) assembleNodeStatus(ssh *types.SSH) (*types.Cluster, error) {
	if p.MasterIps != "" {
		masterIps := strings.Split(p.MasterIps, ",")
		p.syncNodesMap(masterIps, true, ssh)
	}

	if p.WorkerIps != "" {
		workerIps := strings.Split(p.WorkerIps, ",")
		p.syncNodesMap(workerIps, false, ssh)
	}

	p.m.Range(func(key, value interface{}) bool {
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
	}, nil
}

func (p *Native) syncNodesMap(ipList []string, master bool, ssh *types.SSH) {
	for _, ip := range ipList {
		currentID := strings.Replace(ip, ".", "-", -1)
		p.m.Store(currentID, types.Node{
			Master:            master,
			RollBack:          true,
			InstanceID:        currentID,
			InstanceStatus:    native.StatusRunning,
			InternalIPAddress: []string{ip},
			PublicIPAddress:   []string{ip},
			Current:           true,
			SSH:               *ssh,
		})
	}
}
