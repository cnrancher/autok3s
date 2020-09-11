package alibaba

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/cnrancher/autok3s/pkg/viper"

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
	ui                      = "none"
	repo                    = "https://apphub.aliyuncs.com"
	terway                  = "none"
	terwayMaxPoolSize       = "5"
	cloudControllerManager  = "false"
	usageInfo               = `================ Prompt Info ================
Use 'autok3s kubectl config use-context %s'
Use 'autok3s kubectl get pods -A' get POD status`
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
			Provider:               "alibaba",
			Master:                 master,
			Worker:                 worker,
			UI:                     ui,
			Repo:                   repo,
			CloudControllerManager: cloudControllerManager,
		},
		Options: alibaba.Options{
			DiskCategory:            diskCategory,
			DiskSize:                diskSize,
			Image:                   imageID,
			Terway:                  alibaba.Terway{Mode: terway, MaxPoolSize: terwayMaxPoolSize},
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

func (p *Alibaba) GenerateClusterName() {
	p.Name = fmt.Sprintf("%s.%s", p.Name, p.Region)
}

func (p *Alibaba) CreateK3sCluster(ssh *types.SSH) (err error) {
	s := utils.NewSpinner("Generating K3s cluster: ")
	s.Start()
	defer func() {
		s.Stop()
		if err == nil && len(p.Status.MasterNodes) > 0 {
			fmt.Printf(usageInfo, p.Name)
			if p.UI != "none" {
				if strings.EqualFold(p.CloudControllerManager, "true") {
					fmt.Printf("K3s UI %s URL: https://<using `kubectl get svc -A` get UI address>:8999\n", p.UI)
				} else {
					fmt.Printf("K3s UI %s URL: https://%s:8999\n", p.UI, p.Status.MasterNodes[0].PublicIPAddress[0])
				}
			}
		}
	}()

	if err = p.generateClientSDK(); err != nil {
		return
	}

	if err = p.createCheck(); err != nil {
		return
	}

	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	// run ecs master instances.
	if err = p.runInstances(masterNum, true); err != nil {
		return
	}

	// run ecs worker instances.
	if workerNum != 0 {
		if err = p.runInstances(workerNum, false); err != nil {
			return
		}
	}

	// wait ecs instances to be running status.
	if err = p.getInstanceStatus(); err != nil {
		return
	}

	// assemble instance status.
	var c *types.Cluster
	if c, err = p.assembleInstanceStatus(ssh); err != nil {
		return
	}

	// initialize K3s cluster.
	if err = cluster.InitK3sCluster(c); err != nil {
		return
	}

	return
}

func (p *Alibaba) JoinK3sNode(ssh *types.SSH) error {
	s := utils.NewSpinner("Joining K3s node: ")
	s.Start()
	defer s.Stop()

	if err := p.generateClientSDK(); err != nil {
		return err
	}

	if err := p.joinCheck(); err != nil {
		return err
	}

	// TODO: join master node will be added soon.
	workerNum, _ := strconv.Atoi(p.Worker)

	// run ecs worker instances.
	if err := p.runInstances(workerNum, false); err != nil {
		return err
	}

	// wait ecs instances to be running status.
	if err := p.getInstanceStatus(); err != nil {
		return err
	}

	// assemble instance status.
	var merged *types.Cluster
	var err error
	if merged, err = p.assembleInstanceStatus(ssh); err != nil {
		return err
	}

	added := &types.Cluster{
		Metadata: merged.Metadata,
		Options:  merged.Options,
		Status:   types.Status{},
	}

	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		if !v.Master {
			added.Status.WorkerNodes = append(added.Status.WorkerNodes, v)
		}
		return true
	})

	// join K3s node.
	return cluster.JoinK3sNode(merged, added)
}

func (p *Alibaba) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	if p.c == nil {
		if err := p.generateClientSDK(); err != nil {
			return false, ids, err
		}
	}

	request := ecs.CreateListTagResourcesRequest()
	request.Scheme = "https"
	request.ResourceType = "instance"
	request.Tag = &[]ecs.ListTagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: p.Name}}

	response, err := p.c.ListTagResources(request)
	if err != nil || len(response.TagResources.TagResource) > 0 {
		for _, resource := range response.TagResources.TagResource {
			ids = append(ids, resource.ResourceId)
		}
		return true, ids, err
	}

	return false, ids, nil
}

func (p *Alibaba) Rollback() error {
	s := utils.NewSpinner("Executing rollback process: ")
	s.Start()
	defer s.Stop()
	ids := make([]string, 0)

	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		if v.RollBack {
			ids = append(ids, key.(string))
		}
		return true
	})

	if len(ids) > 0 {
		request := ecs.CreateDeleteInstancesRequest()
		request.Scheme = "https"
		request.InstanceId = &ids
		request.Force = requests.NewBoolean(true)

		wait.ErrWaitTimeout = fmt.Errorf("[%s] calling rollback error, please remove the cloud provider instances manually. region=%s, "+
			"instanceName=%s, message=the maximum number of attempts reached\n", p.GetProviderName(), p.Region, ids)

		// retry 5 times, total 120 seconds.
		backoff := wait.Backoff{
			Duration: 30 * time.Second,
			Factor:   1,
			Steps:    5,
		}

		if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
			response, err := p.c.DeleteInstances(request)
			if err != nil || !response.IsSuccess() {
				return false, nil
			}
			return true, nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (p *Alibaba) generateClientSDK() error {
	if p.AccessKey == "" {
		p.AccessKey = viper.GetString(p.GetProviderName(), accessKeyID)
	}

	if p.AccessSecret == "" {
		p.AccessSecret = viper.GetString(p.GetProviderName(), accessKeySecret)
	}

	client, err := ecs.NewClientWithAccessKey(p.Region, p.AccessKey, p.AccessSecret)
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
	request.UniqueSuffix = requests.NewBoolean(true)

	tag := []ecs.RunInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: p.Name}}
	if master {
		// TODO: HA mode will be added soon, temporary set master number to 1.
		request.Amount = requests.NewInteger(1)
		tag = append(tag, ecs.RunInstancesTag{Key: "master", Value: "true"})
	} else {
		tag = append(tag, ecs.RunInstancesTag{Key: "worker", Value: "true"})
	}
	request.Tag = &tag

	response, err := p.c.RunInstances(request)
	if err != nil || len(response.InstanceIdSets.InstanceIdSet) != num {
		return fmt.Errorf("[%s] calling runInstances error. region=%s, "+"instanceName=%s, message=[%s]\n",
			p.GetProviderName(), p.Region, request.InstanceName, err)
	}
	for _, id := range response.InstanceIdSets.InstanceIdSet {
		if master {
			p.m.Store(id, types.Node{Master: true, RollBack: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
		} else {
			p.m.Store(id, types.Node{Master: false, RollBack: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
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

	if len(ids) > 0 {
		request := ecs.CreateDescribeInstanceStatusRequest()
		request.Scheme = "https"
		request.InstanceId = &ids

		wait.ErrWaitTimeout = fmt.Errorf("[%s] calling getInstanceStatus error. region=%s, "+"instanceName=%s, message=not running status\n",
			p.GetProviderName(), p.Region, ids)

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			response, err := p.c.DescribeInstanceStatus(request)
			if err != nil || !response.IsSuccess() || len(response.InstanceStatuses.InstanceStatus) <= 0 {
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
	}

	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
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
			continue
		}

		master := false
		for _, tag := range status.Tags.Tag {
			if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
				master = true
				break
			}
		}

		if master {
			p.m.Store(status.InstanceId, types.Node{
				Master:            true,
				RollBack:          false,
				InstanceID:        status.InstanceId,
				InstanceStatus:    alibaba.StatusRunning,
				InternalIPAddress: status.VpcAttributes.PrivateIpAddress.IpAddress,
				PublicIPAddress:   status.PublicIpAddress.IpAddress})
		} else {
			p.m.Store(status.InstanceId, types.Node{
				Master:            false,
				RollBack:          false,
				InstanceID:        status.InstanceId,
				InstanceStatus:    alibaba.StatusRunning,
				InternalIPAddress: status.VpcAttributes.PrivateIpAddress.IpAddress,
				PublicIPAddress:   status.PublicIpAddress.IpAddress})
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

	return &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}, nil
}

func (p *Alibaba) describeInstances() (*ecs.DescribeInstancesResponse, error) {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	request.Tag = &[]ecs.DescribeInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: p.Name}}

	response, err := p.c.DescribeInstances(request)
	if err == nil && len(response.Instances.Instance) == 0 {
		return nil, fmt.Errorf("[%s] calling describeInstances error. region=%s, "+"instanceName=%s, message=[%s]\n",
			p.GetProviderName(), p.Region, request.InstanceName, err)
	}

	return response, nil
}

func (p *Alibaba) getVSwitchCIDR() (string, string, error) {
	request := ecs.CreateDescribeVSwitchesRequest()
	request.Scheme = "https"
	request.VSwitchId = p.VSwitch

	response, err := p.c.DescribeVSwitches(request)
	if err != nil || !response.IsSuccess() || len(response.VSwitches.VSwitch) < 1 {
		return "", "", fmt.Errorf("[%s] calling describeVSwitches error. region=%s, "+"instanceName=%s, message=[%s]\n",
			p.GetProviderName(), p.Region, p.VSwitch, err)
	}

	return response.VSwitches.VSwitch[0].VpcId, response.VSwitches.VSwitch[0].CidrBlock, nil
}

func (p *Alibaba) getVpcCIDR() (string, error) {
	request := ecs.CreateDescribeVpcsRequest()
	request.Scheme = "https"
	request.VpcId = p.Vpc

	response, err := p.c.DescribeVpcs(request)
	if err != nil || !response.IsSuccess() || len(response.Vpcs.Vpc) != 1 {
		return "", fmt.Errorf("[%s] calling describeVpcs error. region=%s, "+"instanceName=%s, message=[%s]\n",
			p.GetProviderName(), p.Region, p.Vpc, err)
	}

	return response.Vpcs.Vpc[0].CidrBlock, nil
}

func (p *Alibaba) createCheck() error {
	masterNum, _ := strconv.Atoi(p.Master)
	if masterNum != 1 {
		return fmt.Errorf("[%s] calling preflight error: currently `--master` number only support 1\n",
			p.GetProviderName())
	}

	exist, _, err := p.IsClusterExist()
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` already exist\n",
			p.GetProviderName(), p.Name)
	}

	if p.Terway.Mode != "none" {
		vpc, vSwitchCIDR, err := p.getVSwitchCIDR()
		if err != nil {
			return fmt.Errorf("[%s] calling preflight error: vswitch %s cidr not be found\n",
				p.GetProviderName(), p.VSwitch)
		}

		p.Vpc = vpc
		p.ClusterCIDR = vSwitchCIDR

		vpcCIDR, err := p.getVpcCIDR()
		if err != nil {
			return fmt.Errorf("[%s] calling preflight error: vpc %s cidr not be found\n",
				p.GetProviderName(), p.Vpc)
		}

		p.Options.Terway.CIDR = vpcCIDR
	}

	return nil
}

func (p *Alibaba) joinCheck() error {
	workerNum, _ := strconv.Atoi(p.Worker)
	if workerNum < 1 {
		return fmt.Errorf("[%s] calling preflight error: currently `--worker` must greater than 1\n",
			p.GetProviderName())
	}

	exist, ids, err := p.IsClusterExist()

	if err != nil {
		return err
	}

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist\n",
			p.GetProviderName(), p.Name)
	} else {
		// remove invalid worker nodes from .state file.
		workers := make([]types.Node, 0)
		for _, w := range p.WorkerNodes {
			for _, e := range ids {
				if e == w.InstanceID {
					workers = append(workers, w)
					break
				}
			}
		}
		p.WorkerNodes = workers
	}

	return nil
}
