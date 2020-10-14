package native

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/native"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
)

const (
	k3sVersion = "v1.19.2+k3s1"
	master     = "0"
	worker     = "0"
	ui         = false
	repo       = "https://apphub.aliyuncs.com"
	usageInfo  = `=========================== Prompt Info ===========================
Use 'autok3s kubectl config use-context %s'
Use 'autok3s kubectl get pods -A' get POD status`
)

// ProviderName is the name of this provider.
const ProviderName = "native"

var (
	k3sScript    = "http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh"
	k3sMirror    = "INSTALL_K3S_MIRROR=cn"
	dockerMirror = ""
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
			Provider:   "native",
			Master:     master,
			Worker:     worker,
			UI:         ui,
			Repo:       repo,
			K3sVersion: k3sVersion,
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
}

func (p *Native) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	return ""
}

func (p *Native) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return ""
}

func (p *Native) CreateK3sCluster(ssh *types.SSH) (err error) {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing create logic...\n", p.GetProviderName())

	defer func() {
		if err == nil && len(p.Status.MasterNodes) > 0 {
			fmt.Printf(usageInfo, p.Name)
			if p.UI {
				fmt.Printf("\nK3s UI %s URL: https://%s:8999\n", p.UI, p.Status.MasterNodes[0].PublicIPAddress[0])
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

	c.InstallScript = k3sScript
	c.Mirror = k3sMirror
	c.DockerMirror = dockerMirror

	// for rollback
	for _, master := range c.MasterNodes {
		p.m.Store(master.InstanceID, types.Node{Master: true, RollBack: true, InstanceID: master.InstanceID, InstanceStatus: master.InstanceStatus})
	}
	for _, worker := range c.WorkerNodes {
		p.m.Store(worker.InstanceID, types.Node{Master: false, RollBack: true, InstanceID: worker.InstanceID, InstanceStatus: worker.InstanceStatus})
	}

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

	// assemble node status.
	var merged *types.Cluster
	if merged, err = p.assembleNodeStatus(ssh); err != nil {
		return err
	}

	merged.InstallScript = k3sScript
	merged.Mirror = k3sMirror
	merged.DockerMirror = dockerMirror

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
			p.m.Store(v.InstanceID, types.Node{Master: v.Master, RollBack: true, InstanceID: v.InstanceID, InstanceStatus: v.InstanceStatus})
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

func (p *Native) SSHK3sNode(ssh *types.SSH) error {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing ssh logic...\n", p.GetProviderName())

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}

	ids := make(map[string]string, len(p.MasterNodes)+len(p.WorkerNodes))
	for _, masterNode := range p.MasterNodes {
		ids[masterNode.InstanceID] = masterNode.PublicIPAddress[0] + " (master)"
	}
	for _, workerNode := range p.WorkerNodes {
		ids[workerNode.InstanceID] = workerNode.PublicIPAddress[0] + " (worker)"
	}

	ip := strings.Split(utils.AskForSelectItem(fmt.Sprintf("[%s] choose ssh node to connect", p.GetProviderName()), ids), " (")[0]

	if ip == "" {
		return fmt.Errorf("[%s] choose incorrect ssh node", p.GetProviderName())
	}
	// ssh K3s node.
	if err := cluster.SSHK3sNode(ip, c, ssh); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed ssh logic\n", p.GetProviderName())

	return nil
}

func (p *Native) IsClusterExist() (bool, []string, error) {
	isExist := len(p.MasterNodes) > 0
	if isExist {
		var ids []string
		for _, masterNode := range p.MasterNodes {
			ids = append(ids, masterNode.InstanceID)
		}

		for _, workerNode := range p.WorkerNodes {
			ids = append(ids, workerNode.InstanceID)
		}

		return isExist, ids, nil
	}
	return isExist, []string{}, nil
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
		if err := cluster.UninstallK3sNodes(nodes); err != nil {
			p.logger.Warnf("[%s] rollback error: %v", p.GetProviderName(), err)
		}
	}

	p.logger.Infof("[%s] successfully executed rollback logic\n", p.GetProviderName())

	return nil
}

func (p *Native) DeleteK3sCluster(f bool) error {
	isConfirmed := true

	if !f {
		isConfirmed = utils.AskForConfirmation(fmt.Sprintf("[%s] are you sure to delete cluster %s", p.GetProviderName(), p.Name))
	}

	if isConfirmed {
		p.logger = common.NewLogger(common.Debug)
		p.logger.Infof("[%s] executing delete cluster logic...\n", p.GetProviderName())

		if err := cluster.UninstallK3sCluster(&types.Cluster{
			Metadata: p.Metadata,
			Options:  p.Options,
			Status:   p.Status,
		}); err != nil {
			return err
		}

		p.logger.Infof("[%s] successfully excuted delete cluster logic\n", p.GetProviderName())
	}
	return nil
}

func (p *Native) StartK3sCluster() error {
	return p.CommandNotSupport("start")
}

func (p *Native) StopK3sCluster(f bool) error {
	return p.CommandNotSupport("stop")
}

func (p *Native) CommandNotSupport(commandName string) error {
	return fmt.Errorf("[%s]provider `native` dose not support command:[%s]", p.GetProviderName(), commandName)
}

func (p *Native) assembleNodeStatus(ssh *types.SSH) (*types.Cluster, error) {
	if p.MasterIps != "" {
		masterIps := strings.Split(p.MasterIps, ",")
		for _, masterIP := range masterIps {
			currentID := strings.Replace(masterIP, ".", "-", -1)
			p.m.Store(currentID, types.Node{
				Master:            true,
				RollBack:          false,
				InstanceID:        currentID,
				InstanceStatus:    native.StatusRunning,
				InternalIPAddress: []string{masterIP},
				PublicIPAddress:   []string{masterIP},
				Current:           true,
			})
		}
	}

	if p.WorkerIps != "" {
		workerIps := strings.Split(p.WorkerIps, ",")

		for _, workerIP := range workerIps {
			currentID := strings.Replace(workerIP, ".", "-", -1)
			p.m.Store(currentID, types.Node{
				Master:            false,
				RollBack:          false,
				InstanceID:        currentID,
				InstanceStatus:    native.StatusRunning,
				InternalIPAddress: []string{workerIP},
				PublicIPAddress:   []string{workerIP},
				Current:           true,
			})
		}
	}

	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		v.SSH = *ssh
		if v.Master {
			index := -1
			for i, n := range p.Status.MasterNodes {
				if n.InstanceID == v.InstanceID {
					index = i
					break
				}
			}
			if index > -1 {
				p.Status.MasterNodes[index] = v
			} else {
				p.Status.MasterNodes = append(p.Status.MasterNodes, v)
			}
		} else {
			index := -1
			for i, n := range p.Status.WorkerNodes {
				if n.InstanceID == v.InstanceID {
					index = i
					break
				}
			}
			if index > -1 {
				p.Status.WorkerNodes[index] = v
			} else {
				p.Status.WorkerNodes = append(p.Status.WorkerNodes, v)
			}
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
