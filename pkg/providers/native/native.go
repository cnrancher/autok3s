package native

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/native"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
)

const (
	k3sVersion       = ""
	k3sChannel       = "stable"
	k3sInstallScript = "http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh"
	ui               = false
	repo             = "https://apphub.aliyuncs.com"
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
			Repo:          repo,
			K3sVersion:    k3sVersion,
			K3sChannel:    k3sChannel,
			InstallScript: k3sInstallScript,
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
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing create logic...\n", p.GetProviderName())

	// set ssh default value
	if ssh.User == "" {
		ssh.User = defaultUser
	}
	if ssh.Password == "" && ssh.SSHKeyPath == "" {
		ssh.SSHKeyPath = defaultSSHKeyPath
	}

	defer func() {
		if err == nil && len(p.Status.MasterNodes) > 0 {
			fmt.Printf(common.UsageInfo, p.Name)
			if p.UI {
				fmt.Printf("\nK3s UI URL: https://%s:8999\n", p.Status.MasterNodes[0].PublicIPAddress[0])
			}
		}
	}()

	if p.MasterIps == "" {
		return fmt.Errorf("[%s] cluster must have one master when create", p.GetProviderName())
	}

	// assemble node status.
	var c *types.Cluster
	if c, err = p.assembleNodeStatus(ssh); err != nil {
		return err
	}

	c.Mirror = k3sMirror
	c.DockerMirror = dockerMirror

	// initialize K3s cluster.
	if err = cluster.InitK3sCluster(c); err != nil {
		return
	}
	p.logger.Infof("[%s] successfully executed create logic\n", p.GetProviderName())

	return
}

func (p *Native) JoinK3sNode(ssh *types.SSH) (err error) {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing join logic...\n", p.GetProviderName())
	// set ssh default value
	if ssh.User == "" {
		ssh.User = defaultUser
	}
	if ssh.Password == "" && ssh.SSHKeyPath == "" {
		ssh.SSHKeyPath = defaultSSHKeyPath
	}

	// assemble node status.
	var merged *types.Cluster
	if merged, err = p.assembleNodeStatus(ssh); err != nil {
		return err
	}

	added := &types.Cluster{
		Metadata: merged.Metadata,
		Options:  merged.Options,
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

	for _, masterNode := range merged.Status.MasterNodes {
		masterIps = append(masterIps, masterNode.PublicIPAddress...)
	}

	for _, workerNode := range merged.Status.WorkerNodes {
		workerIps = append(workerIps, workerNode.PublicIPAddress...)
	}

	p.Options.MasterIps = strings.Join(masterIps, ",")
	p.Options.WorkerIps = strings.Join(workerIps, ",")

	// join K3s node.
	if err := cluster.JoinK3sNode(merged, added); err != nil {
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

func (p *Native) DeleteK3sCluster(f bool) error {
	return p.CommandNotSupport("delete")
}

func (p *Native) StartK3sCluster() error {
	return p.CommandNotSupport("start")
}

func (p *Native) StopK3sCluster(f bool) error {
	return p.CommandNotSupport("stop")
}

func (p *Native) SSHK3sNode(ssh *types.SSH) error {
	return p.CommandNotSupport("ssh")
}

func (p *Native) CommandNotSupport(commandName string) error {
	return fmt.Errorf("[%s] dose not support command: [%s]", p.GetProviderName(), commandName)
}

func (p *Native) GetCluster(kubecfg string) *types.ClusterInfo {
	return &types.ClusterInfo{}
}

func (p *Native) IsClusterExist() (bool, []string, error) {
	return false, []string{}, nil
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
