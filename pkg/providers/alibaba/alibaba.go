package alibaba

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jason-ZW/autok3s/pkg/cluster"
	"github.com/Jason-ZW/autok3s/pkg/common"
	"github.com/Jason-ZW/autok3s/pkg/types"
	"github.com/Jason-ZW/autok3s/pkg/types/alibaba"
	"github.com/Jason-ZW/autok3s/pkg/utils"
	"github.com/Jason-ZW/autok3s/pkg/viper"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"golang.org/x/sync/syncmap"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	accessKeyID             = "access-key"
	accessKeySecret         = "access-secret"
	imageID                 = "ubuntu_18_04_x64_20G_alibase_20200618.vhd"
	instanceType            = "ecs.c6.large"
	internetMaxBandwidthOut = "50"
	diskCategory            = "cloud_ssd"
	diskSize                = "40"
	master                  = "1"
	worker                  = "1"
)

var (
	backoff = wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   2,
		Steps:    5,
	}
)

type Alibaba struct {
	types.Metadata  `json:",inline"`
	alibaba.Options `json:",inline"`
	types.Status    `json:"status"`

	c *ecs.Client
	m *sync.Map
}

func NewProvider() *Alibaba {
	return &Alibaba{
		Metadata: types.Metadata{
			Provider: "alibaba",
			Master:   master,
			Worker:   worker,
		},
		Options: alibaba.Options{
			DiskCategory:            diskCategory,
			DiskSize:                diskSize,
			Image:                   imageID,
			Type:                    instanceType,
			InternetMaxBandwidthOut: internetMaxBandwidthOut,
		},
		Status: types.Status{
			MasterNodes: make([]types.Node, 0),
			WorkerNodes: make([]types.Node, 0),
		},
		m: new(syncmap.Map),
	}
}

func (p *Alibaba) GetProviderName() string {
	return "alibaba"
}

func (p *Alibaba) CreateK3sCluster(ssh *types.SSH) error {
	s := utils.NewSpinner("Generating K3s cluster: ")
	s.Start()
	defer s.Stop()

	if err := p.generateClientSDK(); err != nil {
		return err
	}

	if err := p.preflight(); err != nil {
		return err
	}

	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	// run ecs master instances.
	if err := p.runInstances(masterNum, true); err != nil {
		return err
	}

	// run ecs worker instances.
	if err := p.runInstances(workerNum, false); err != nil {
		return err
	}

	// wait ecs instances to be running status.
	if err := p.getInstanceStatus(); err != nil {
		return err
	}

	// assemble instance status.
	var c *types.Cluster
	var err error
	if c, err = p.assembleInstanceStatus(ssh); err != nil {
		return err
	}

	// initialize k3s cluster.
	return cluster.InitK3sCluster(c)
}

func (p *Alibaba) generateClientSDK() error {
	client, err := ecs.NewClientWithAccessKey(p.Region, viper.GetString(p.GetProviderName(), accessKeyID),
		viper.GetString(p.GetProviderName(), accessKeySecret))
	if err != nil {
		return err
	}
	client.EnableAsync(5, 1000)
	p.c = client

	return nil
}

func (p *Alibaba) runInstances(num int, master bool) error {
	request := ecs.CreateRunInstancesRequest()
	request.Scheme = "https"
	request.InstanceType = p.Type
	request.ImageId = p.Image
	request.VSwitchId = p.VSwitch
	request.KeyPairName = p.KeyPair
	request.SystemDiskCategory = p.DiskCategory
	request.SystemDiskSize = p.DiskSize
	request.SecurityGroupId = p.SecurityGroup
	outBandWidth, _ := strconv.Atoi(p.InternetMaxBandwidthOut)
	request.InternetMaxBandwidthOut = requests.NewInteger(outBandWidth)
	request.Amount = requests.NewInteger(num)

	if master {
		request.InstanceName = fmt.Sprintf(common.MasterInstanceName, p.Name, 1, 1)
		request.HostName = fmt.Sprintf(common.MasterInstanceName, p.Name, 1, 1)
	} else {
		request.InstanceName = fmt.Sprintf(common.WorkerInstanceName, p.Name, 1, 1)
		request.HostName = fmt.Sprintf(common.WorkerInstanceName, p.Name, 1, 1)
	}

	response, err := p.c.RunInstances(request)
	if err != nil || len(response.InstanceIdSets.InstanceIdSet) != num {
		return errors.New(fmt.Sprintf("[%s] calling runInstances error. region=%s, "+"instanceName=%s, message=[%s]\n",
			p.GetProviderName(), p.Region, request.InstanceName, err.Error()))
	}
	for _, id := range response.InstanceIdSets.InstanceIdSet {
		if master {
			p.m.Store(id, types.Node{Master: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
		} else {
			p.m.Store(id, types.Node{Master: false, InstanceID: id, InstanceStatus: alibaba.StatusPending})
		}
	}

	return nil
}

func (p *Alibaba) getInstanceStatus() error {
	ids := make([]string, 0)
	p.m.Range(func(key, value interface{}) bool {
		ids = append(ids, key.(string))
		return true
	})

	request := ecs.CreateDescribeInstanceStatusRequest()
	request.Scheme = "https"
	request.InstanceId = &ids

	wait.ErrWaitTimeout = errors.New(fmt.Sprintf("[%s] calling getInstanceStatus error. region=%s, "+"instanceName=%s, message=not running status\n",
		p.GetProviderName(), p.Region, ids))

	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		response, err := p.c.DescribeInstanceStatus(request)
		if err != nil || !response.IsSuccess() {
			return false, nil
		}

		for _, status := range response.InstanceStatuses.InstanceStatus {
			if status.Status == alibaba.StatusRunning {
				if value, ok := p.m.Load(status.InstanceId); ok {
					v := value.(types.Node)
					v.InstanceStatus = alibaba.StatusRunning
					p.m.Store(status.InstanceId, v)
				}

			} else {
				return false, nil
			}
		}

		return true, nil
	}); err != nil {
		return err
	}

	p.Status.MasterNodes = make([]types.Node, 0)
	p.Status.WorkerNodes = make([]types.Node, 0)
	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		if v.Master {
			p.Status.MasterNodes = append(p.Status.MasterNodes, v)
		} else {
			p.Status.WorkerNodes = append(p.Status.WorkerNodes, v)
		}
		return true
	})

	return nil
}

func (p *Alibaba) assembleInstanceStatus(ssh *types.SSH) (*types.Cluster, error) {
	response, err := p.describeInstances()
	if err != nil {
		return nil, err
	}

	for _, status := range response.Instances.Instance {
		if value, ok := p.m.Load(status.InstanceId); ok {
			v := value.(types.Node)
			v.InternalIPAddress = status.VpcAttributes.PrivateIpAddress.IpAddress
			v.PublicIPAddress = status.PublicIpAddress.IpAddress
			p.m.Store(status.InstanceId, v)
		}
	}

	p.Status.MasterNodes = make([]types.Node, 0)
	p.Status.WorkerNodes = make([]types.Node, 0)
	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		v.Port = ssh.Port
		v.User = ssh.User
		v.SSHKey = ssh.SSHKey
		if v.Master {
			p.Status.MasterNodes = append(p.Status.MasterNodes, v)
		} else {
			p.Status.WorkerNodes = append(p.Status.WorkerNodes, v)
		}
		return true
	})

	return &types.Cluster{
		Metadata: p.Metadata,
		Status:   p.Status,
	}, nil
}

func (p *Alibaba) describeInstances() (*ecs.DescribeInstancesResponse, error) {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	request.InstanceName = strings.ToLower(fmt.Sprintf(common.WildcardInstanceName, p.Name))

	response, err := p.c.DescribeInstances(request)
	if err == nil && len(response.Instances.Instance) == 0 {
		return nil, errors.New(fmt.Sprintf("[%s] calling describeInstances error. region=%s, "+"instanceName=%s, message=[%s]\n",
			p.GetProviderName(), p.Region, request.InstanceName, err.Error()))
	}

	return response, nil
}

func (p *Alibaba) isClusterExist() (bool, error) {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	request.InstanceName = strings.ToLower(fmt.Sprintf(common.WildcardInstanceName, p.Name))

	response, err := p.c.DescribeInstances(request)
	if err != nil || len(response.Instances.Instance) > 0 {
		return false, err
	}

	return true, nil
}

func (p *Alibaba) preflight() error {
	exist, err := p.isClusterExist()

	if err != nil {
		return err
	}

	if !exist {
		return errors.New(fmt.Sprintf("[%s] calling preflight error: cluster name `%s` already exist\n",
			p.GetProviderName(), p.Name))
	}

	return nil
}
