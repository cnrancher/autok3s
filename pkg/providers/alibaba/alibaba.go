package alibaba

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/cnrancher/autok3s/pkg/viper"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	k3sVersion              = "v1.19.2+k3s1"
	accessKeyID             = "access-key"
	accessKeySecret         = "access-secret"
	imageID                 = "ubuntu_18_04_x64_20G_alibase_20200618.vhd"
	instanceType            = "ecs.c6.large"
	internetMaxBandwidthOut = "50"
	diskCategory            = "cloud_ssd"
	diskSize                = "40"
	master                  = "0"
	worker                  = "0"
	ui                      = "none"
	repo                    = "https://apphub.aliyuncs.com"
	terway                  = "none"
	terwayMaxPoolSize       = "5"
	cloudControllerManager  = "false"
	resourceTypeEip         = "EIP"
	eipStatusAvailable      = "Available"
	eipStatusInUse          = "InUse"
	usageInfo               = `=========================== Prompt Info ===========================
Use 'autok3s kubectl config use-context %s'
Use 'autok3s kubectl get pods -A' get POD status`
)

// ProviderName is the name of this provider.
const ProviderName = "alibaba"

var (
	k3sScript           = "http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh"
	k3sMirror           = "INSTALL_K3S_MIRROR=cn"
	dockerMirror        = "--mirror Aliyun"
	deployCCMCommand    = "echo \"%s\" | base64 -d > \"%s/cloud-controller-manager.yaml\""
	deployTerwayCommand = "echo \"%s\" | base64 -d > \"%s/terway.yaml\""
)

type checkFun func() error

type Alibaba struct {
	types.Metadata  `json:",inline"`
	alibaba.Options `json:",inline"`
	types.Status    `json:"status"`

	c      *ecs.Client
	v      *vpc.Client
	m      *sync.Map
	logger *logrus.Logger
}

func init() {
	providers.RegisterProvider(ProviderName, func() (providers.Provider, error) {
		return NewProvider(), nil
	})
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
			K3sVersion:             k3sVersion,
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
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing create logic...\n", p.GetProviderName())

	defer func() {
		if err == nil && len(p.Status.MasterNodes) > 0 {
			fmt.Printf(usageInfo, p.Name)
			if p.UI != "none" {
				if strings.EqualFold(p.CloudControllerManager, "true") {
					fmt.Printf("\nK3s UI %s URL: https://<using `kubectl get svc -A` get UI address>:8999\n", p.UI)
				} else {
					fmt.Printf("\nK3s UI %s URL: https://%s:8999\n", p.UI, p.Status.MasterNodes[0].PublicIPAddress[0])
				}
			}
			fmt.Println("")
		}
	}()

	c, err := p.generateInstance(p.createCheck, ssh)
	if err != nil {
		return
	}

	// initialize K3s cluster.
	if err = cluster.InitK3sCluster(c); err != nil {
		return
	}
	p.logger.Infof("[%s] successfully executed create logic\n", p.GetProviderName())

	if option, ok := c.Options.(alibaba.Options); ok {
		extraManifests := make([]string, 0)
		if strings.EqualFold(option.Terway.Mode, "eni") {
			// deploy additional Terway manifests.
			terway := &alibaba.Terway{
				Mode:          option.Terway.Mode,
				AccessKey:     option.AccessKey,
				AccessSecret:  option.AccessSecret,
				CIDR:          option.Terway.CIDR,
				SecurityGroup: option.SecurityGroup,
				VSwitches:     fmt.Sprintf(`{"%s":["%s"]}`, option.Region, option.VSwitch),
				MaxPoolSize:   option.Terway.MaxPoolSize,
			}
			tmpl := fmt.Sprintf(terwayTmpl, terway.AccessKey, terway.AccessSecret, terway.SecurityGroup, terway.CIDR,
				terway.VSwitches, terway.MaxPoolSize)
			extraManifests = append(extraManifests, fmt.Sprintf(deployTerwayCommand,
				base64.StdEncoding.EncodeToString([]byte(tmpl)), common.K3sManifestsDir))
		}
		if strings.EqualFold(c.CloudControllerManager, "true") {
			// deploy additional Alibaba cloud-controller-manager manifests.
			aliCCM := &alibaba.CloudControllerManager{
				Region:       option.Region,
				AccessKey:    option.AccessKey,
				AccessSecret: option.AccessSecret,
			}
			var tmpl string
			if c.ClusterCIDR == "" {
				tmpl = fmt.Sprintf(alibabaCCMTmpl, aliCCM.AccessKey, aliCCM.AccessSecret, "10.42.0.0/16")
			} else {
				tmpl = fmt.Sprintf(alibabaCCMTmpl, aliCCM.AccessKey, aliCCM.AccessSecret, c.ClusterCIDR)
			}
			extraManifests = append(extraManifests, fmt.Sprintf(deployCCMCommand,
				base64.StdEncoding.EncodeToString([]byte(tmpl)), common.K3sManifestsDir))
		}
		p.logger.Infof("[%s] start deploy Alibaba additional manifests\n", p.GetProviderName())
		if err := cluster.DeployExtraManifest(c, extraManifests); err != nil {
			return err
		}
		p.logger.Infof("[%s] successfully deploy Alibaba additional manifests\n", p.GetProviderName())
	}

	return
}

func (p *Alibaba) JoinK3sNode(ssh *types.SSH) error {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing join logic...\n", p.GetProviderName())

	merged, err := p.generateInstance(p.joinCheck, ssh)
	if err != nil {
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
		}
		return true
	})

	// join K3s node.
	if err := cluster.JoinK3sNode(merged, added); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed join logic\n", p.GetProviderName())

	return nil
}

func (p *Alibaba) Rollback() error {
	p.logger.Infof("[%s] executing rollback logic...\n", p.GetProviderName())

	ids := make([]string, 0)
	p.m.Range(func(key, value interface{}) bool {
		v := value.(types.Node)
		if v.RollBack {
			ids = append(ids, key.(string))
		}
		return true
	})

	p.logger.Debugf("[%s] instances %s will be rollback\n", p.GetProviderName(), ids)

	if len(ids) > 0 {
		p.releaseEipAddresses(true)

		request := ecs.CreateDeleteInstancesRequest()
		request.Scheme = "https"
		request.InstanceId = &ids
		request.Force = requests.NewBoolean(true)

		wait.ErrWaitTimeout = fmt.Errorf("[%s] calling rollback error, please remove the cloud provider instances manually. region: %s, "+
			"instanceName: %s, msg: the maximum number of attempts reached", p.GetProviderName(), p.Region, ids)

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

	p.logger.Infof("[%s] successfully executed rollback logic\n", p.GetProviderName())

	return nil
}

func (p *Alibaba) DeleteK3sCluster(f bool) error {
	isConfirmed := true

	if !f {
		isConfirmed = utils.AskForConfirmation(fmt.Sprintf("[%s] are you sure to delete cluster %s", p.GetProviderName(), p.Name))
	}

	if isConfirmed {
		p.logger = common.NewLogger(common.Debug)
		p.logger.Infof("[%s] executing delete cluster logic...\n", p.GetProviderName())

		if err := p.generateClientSDK(); err != nil {
			return err
		}

		if err := p.deleteCluster(f); err != nil {
			return err
		}

		p.logger.Infof("[%s] successfully excuted delete cluster logic\n", p.GetProviderName())
	}

	return nil
}

func (p *Alibaba) StartK3sCluster() error {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing start logic...\n", p.GetProviderName())

	if err := p.generateClientSDK(); err != nil {
		return err
	}

	if err := p.startCluster(); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed start logic\n", p.GetProviderName())

	return nil
}

func (p *Alibaba) StopK3sCluster(f bool) error {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing stop logic...\n", p.GetProviderName())

	if err := p.generateClientSDK(); err != nil {
		return err
	}

	if err := p.stopCluster(f); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed stop logic\n", p.GetProviderName())

	return nil
}

func (p *Alibaba) SSHK3sNode(ssh *types.SSH) error {
	p.logger = common.NewLogger(common.Debug)
	p.logger.Infof("[%s] executing ssh logic...\n", p.GetProviderName())

	if err := p.generateClientSDK(); err != nil {
		return err
	}

	response, err := p.describeInstances()
	if err != nil {
		return err
	}
	if len(response.Instances.Instance) < 1 {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist",
			p.GetProviderName(), p.Name)
	}

	ids := make(map[string]string, len(response.Instances.Instance))
	for _, instance := range response.Instances.Instance {
		if instance.EipAddress.IpAddress != "" {
			for _, t := range instance.Tags.Tag {
				switch t.TagKey {
				case "master":
					if t.TagValue == "true" {
						ids[instance.InstanceId] = instance.EipAddress.IpAddress + " (master)"
					}
				case "worker":
					if t.TagValue == "true" {
						ids[instance.InstanceId] = instance.EipAddress.IpAddress + " (worker)"
					}
				default:
					continue
				}
			}
		} else if instance.EipAddress.IpAddress == "" && len(instance.PublicIpAddress.IpAddress) > 0 {
			for _, t := range instance.Tags.Tag {
				switch t.TagKey {
				case "master":
					if t.TagValue == "true" {
						ids[instance.InstanceId] = instance.PublicIpAddress.IpAddress[0] + " (master)"
					}
				case "worker":
					if t.TagValue == "true" {
						ids[instance.InstanceId] = instance.PublicIpAddress.IpAddress[0] + " (worker)"
					}
				default:
					continue
				}
			}

		}
	}

	ip := strings.Split(utils.AskForSelectItem(fmt.Sprintf("[%s] choose ssh node to connect", p.GetProviderName()), ids), " (")[0]

	if ip == "" {
		return fmt.Errorf("[%s] choose incorrect ssh node", p.GetProviderName())
	}

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}

	// ssh K3s node.
	if err := cluster.SSHK3sNode(ip, c, ssh); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed ssh logic\n", p.GetProviderName())

	return nil
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
	request.Tag = &[]ecs.ListTagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}

	response, err := p.c.ListTagResources(request)
	if err != nil || len(response.TagResources.TagResource) > 0 {
		for _, resource := range response.TagResources.TagResource {
			ids = append(ids, resource.ResourceId)
		}
		// ecs will return multiple instance ids based on the value of tag key.n by n, so duplicate items need to be removed.
		return true, utils.UniqueArray(ids), err
	}
	return false, utils.UniqueArray(ids), nil
}

func (p *Alibaba) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	if option, ok := cluster.Options.(alibaba.Options); ok {
		if strings.EqualFold(cluster.CloudControllerManager, "true") {
			extraArgs := fmt.Sprintf(" --kubelet-arg=provider-id=alicloud://%s.%s --node-name=%s.%s",
				option.Region, master.InstanceID, option.Region, master.InstanceID)
			return extraArgs
		}
	}
	return ""
}

func (p *Alibaba) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return p.GenerateMasterExtraArgs(cluster, worker)
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

	vpcClient, err := vpc.NewClientWithAccessKey(p.Region, p.AccessKey, p.AccessSecret)
	if err != nil {
		return err
	}
	p.v = vpcClient

	return nil
}

func (p *Alibaba) runInstances(num int, master bool) error {
	if master {
		p.logger.Debugf("[%s] %d number of master instances will be created\n", p.GetProviderName(), num)
	} else {
		p.logger.Debugf("[%s] %d number of worker instances will be created\n", p.GetProviderName(), num)
	}

	request := ecs.CreateRunInstancesRequest()
	request.Scheme = "https"
	request.InstanceType = p.Type
	request.ImageId = p.Image
	request.VSwitchId = p.VSwitch
	request.KeyPairName = p.KeyPair
	request.SystemDiskCategory = p.DiskCategory
	request.SystemDiskSize = p.DiskSize
	request.SecurityGroupId = p.SecurityGroup
	request.Amount = requests.NewInteger(num)
	request.UniqueSuffix = requests.NewBoolean(false)
	if p.Zone != "" {
		request.ZoneId = p.Zone
	}

	tag := []ecs.RunInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}
	if master {
		request.InstanceName = fmt.Sprintf(common.MasterInstanceName, p.Name)
		tag = append(tag, ecs.RunInstancesTag{Key: "master", Value: "true"})
	} else {
		request.InstanceName = fmt.Sprintf(common.WorkerInstanceName, p.Name)
		tag = append(tag, ecs.RunInstancesTag{Key: "worker", Value: "true"})
	}
	request.Tag = &tag

	response, err := p.c.RunInstances(request)
	if err != nil || len(response.InstanceIdSets.InstanceIdSet) != num {
		return fmt.Errorf("[%s] calling runInstances error. region: %s, zone: %s, "+"instanceName: %s, msg: [%v]",
			p.GetProviderName(), p.Region, p.Zone, request.InstanceName, err)
	}
	for _, id := range response.InstanceIdSets.InstanceIdSet {
		if master {
			p.m.Store(id, types.Node{Master: true, RollBack: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
		} else {
			p.m.Store(id, types.Node{Master: false, RollBack: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
		}
	}

	if master {
		p.logger.Debugf("[%s] %d number of master instances successfully created\n", p.GetProviderName(), num)
	} else {
		p.logger.Debugf("[%s] %d number of worker instances successfully created\n", p.GetProviderName(), num)
	}

	return nil
}

func (p *Alibaba) deleteCluster(f bool) error {
	exist, ids, err := p.IsClusterExist()

	if !exist && !f {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", p.GetProviderName(), p.Name)
	}

	if err == nil && len(ids) > 0 {
		p.logger.Debugf("[%s] cluster %s will be deleted\n", p.GetProviderName(), p.Name)

		p.releaseEipAddresses(false)

		request := ecs.CreateDeleteInstancesRequest()
		request.Scheme = "https"
		request.RegionId = p.Region
		request.InstanceId = &ids
		request.Force = requests.NewBoolean(true)
		request.TerminateSubscription = requests.NewBoolean(true)

		_, err := p.c.DeleteInstances(request)
		if err != nil {
			return fmt.Errorf("[%s] calling deleteInstance error, msg: %v", p.GetProviderName(), err)
		}
	}

	if err != nil && !f {
		return fmt.Errorf("[%s] calling deleteInstance error, msg: %v", p.GetProviderName(), err)
	}

	err = cluster.OverwriteCfg(p.Name)

	if err != nil && !f {
		return fmt.Errorf("[%s] synchronizing .cfg file error, msg: %v", p.GetProviderName(), err)
	}

	err = cluster.DeleteState(p.Name, p.Provider)

	if err != nil && !f {
		return fmt.Errorf("[%s] synchronizing .state file error, msg: %v", p.GetProviderName(), err)
	}

	p.logger.Debugf("[%s] successfully deleted cluster %s\n", p.GetProviderName(), p.Name)

	return nil
}

func (p *Alibaba) startCluster() error {
	exist, ids, err := p.IsClusterExist()

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist",
			p.GetProviderName(), p.Name)
	}

	if err == nil && len(ids) > 0 {
		// ensure that the status of all instances is stopped.
		if err := p.startAndStopCheck(alibaba.StatusStopped); err != nil {
			return err
		}
		request := ecs.CreateStartInstancesRequest()
		request.Scheme = "https"
		request.InstanceId = &ids

		if _, err := p.c.StartInstances(request); err != nil {
			return fmt.Errorf("[%s] calling startInstance error, msg: [%v]", p.GetProviderName(), err)
		}
	}

	// wait ecs instances to be running status.
	if err = p.getInstanceStatus(alibaba.StatusRunning); err != nil {
		return err
	}

	err = cluster.SaveState(&types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	})

	if err != nil {
		return fmt.Errorf("[%s] synchronizing .state file error, msg: [%v]", p.GetProviderName(), err)
	}
	return nil
}

func (p *Alibaba) stopCluster(f bool) error {
	exist, ids, err := p.IsClusterExist()

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", p.GetProviderName(), p.Name)
	}

	if err == nil && len(ids) > 0 {
		// ensure that the status of all instances is running.
		if err := p.startAndStopCheck(alibaba.StatusRunning); err != nil {
			return err
		}
		request := ecs.CreateStopInstancesRequest()
		request.Scheme = "https"
		request.InstanceId = &ids

		if f {
			// similar to power-off operation.
			// all cached data not written to the storage device will be lost.
			request.ForceStop = requests.NewBoolean(f)
		}

		if _, err := p.c.StopInstances(request); err != nil {
			return fmt.Errorf("[%s] calling stopInstance error, msg: [%v]", p.GetProviderName(), err)
		}
	}

	// wait ecs instances to be stopped status.
	if err = p.getInstanceStatus(alibaba.StatusStopped); err != nil {
		return err
	}

	err = cluster.SaveState(&types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	})

	if err != nil {
		return fmt.Errorf("[%s] synchronizing .state file error, msg: [%v]", p.GetProviderName(), err)
	}

	return nil
}

func (p *Alibaba) getInstanceStatus(aimStatus string) error {
	ids := make([]string, 0)
	p.m.Range(func(key, value interface{}) bool {
		ids = append(ids, key.(string))
		return true
	})

	if len(ids) > 0 {
		p.logger.Debugf("[%s] waiting for the instances %s to be in `%s` status...\n", p.GetProviderName(), ids, aimStatus)
		request := ecs.CreateDescribeInstanceStatusRequest()
		request.Scheme = "https"
		request.InstanceId = &ids
		if p.Zone != "" {
			request.ZoneId = p.Zone
		}

		wait.ErrWaitTimeout = fmt.Errorf("[%s] calling getInstanceStatus error. region: %s, zone: %s, instanceName: %s, message: not `%s` status",
			p.GetProviderName(), p.Region, p.Zone, ids, aimStatus)

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			response, err := p.c.DescribeInstanceStatus(request)
			if err != nil || !response.IsSuccess() || len(response.InstanceStatuses.InstanceStatus) <= 0 {
				return false, nil
			}

			for _, status := range response.InstanceStatuses.InstanceStatus {
				if status.Status == aimStatus {
					if value, ok := p.m.Load(status.InstanceId); ok {
						v := value.(types.Node)
						v.InstanceStatus = aimStatus
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

	p.logger.Debugf("[%s] instances %s are in `%s` status\n", p.GetProviderName(), ids, aimStatus)

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
			// add only nodes that run the current command.
			v.Current = true
			v.InternalIPAddress = status.VpcAttributes.PrivateIpAddress.IpAddress
			v.PublicIPAddress = []string{status.EipAddress.IpAddress}
			v.EipAllocationIds = []string{status.EipAddress.AllocationId}
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
				EipAllocationIds:  []string{status.EipAddress.AllocationId},
				PublicIPAddress:   []string{status.EipAddress.IpAddress}})
		} else {
			p.m.Store(status.InstanceId, types.Node{
				Master:            false,
				RollBack:          false,
				InstanceID:        status.InstanceId,
				InstanceStatus:    alibaba.StatusRunning,
				InternalIPAddress: status.VpcAttributes.PrivateIpAddress.IpAddress,
				EipAllocationIds:  []string{status.EipAddress.AllocationId},
				PublicIPAddress:   []string{status.EipAddress.IpAddress}})
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
	request.Tag = &[]ecs.DescribeInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}
	if p.Zone != "" {
		request.ZoneId = p.Zone
	}

	response, err := p.c.DescribeInstances(request)
	if err == nil && len(response.Instances.Instance) == 0 {
		return nil, fmt.Errorf("[%s] calling describeInstances error. region: %s, zone: %s, "+"cluster: %s, message: [%s]",
			p.GetProviderName(), p.Region, p.Zone, p.Name, err)
	}
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (p *Alibaba) getVSwitchCIDR() (string, string, error) {
	request := ecs.CreateDescribeVSwitchesRequest()
	request.Scheme = "https"
	request.VSwitchId = p.VSwitch
	if p.Zone != "" {
		request.ZoneId = p.Zone
	}

	response, err := p.c.DescribeVSwitches(request)
	if err != nil || !response.IsSuccess() || len(response.VSwitches.VSwitch) < 1 {
		return "", "", fmt.Errorf("[%s] calling describeVSwitches error. region: %s, zone: %s, "+"instanceName: %s, msg: [%v]",
			p.GetProviderName(), p.Region, p.Zone, p.VSwitch, err)
	}

	return response.VSwitches.VSwitch[0].VpcId, response.VSwitches.VSwitch[0].CidrBlock, nil
}

func (p *Alibaba) getVpcCIDR() (string, error) {
	request := ecs.CreateDescribeVpcsRequest()
	request.Scheme = "https"
	request.VpcId = p.Vpc

	response, err := p.c.DescribeVpcs(request)
	if err != nil || !response.IsSuccess() || len(response.Vpcs.Vpc) != 1 {
		return "", fmt.Errorf("[%s] calling describeVpcs error. region: %s, "+"instanceName: %s, msg: [%v]",
			p.GetProviderName(), p.Region, p.Vpc, err)
	}

	return response.Vpcs.Vpc[0].CidrBlock, nil
}

func (p *Alibaba) createCheck() error {
	masterNum, _ := strconv.Atoi(p.Master)
	if masterNum < 1 {
		return fmt.Errorf("[%s] calling preflight error: `--master` number must >= 1",
			p.GetProviderName())
	}

	if strings.Contains(p.MasterExtraArgs, "--datastore-endpoint") && p.DataStore != "" {
		return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
			p.GetProviderName())
	}

	exist, _, err := p.IsClusterExist()
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` already exist",
			p.GetProviderName(), p.Name)
	}

	if p.Terway.Mode != "none" {
		vpcID, vSwitchCIDR, err := p.getVSwitchCIDR()
		if err != nil {
			return fmt.Errorf("[%s] calling preflight error: vswitch %s cidr not be found",
				p.GetProviderName(), p.VSwitch)
		}

		p.Vpc = vpcID
		p.ClusterCIDR = vSwitchCIDR

		vpcCIDR, err := p.getVpcCIDR()
		if err != nil {
			return fmt.Errorf("[%s] calling preflight error: vpc %s cidr not be found",
				p.GetProviderName(), p.Vpc)
		}

		p.Options.Terway.CIDR = vpcCIDR
	}

	return nil
}

func (p *Alibaba) joinCheck() error {
	if strings.Contains(p.MasterExtraArgs, "--datastore-endpoint") && p.DataStore != "" {
		return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
			p.GetProviderName())
	}

	exist, ids, err := p.IsClusterExist()

	if err != nil {
		return err
	}

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist",
			p.GetProviderName(), p.Name)
	}

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

	return nil
}

func (p *Alibaba) startAndStopCheck(aimStatus string) error {
	response, err := p.describeInstances()
	if err != nil {
		return err
	}
	if response.IsSuccess() && len(response.Instances.Instance) > 0 {
		masterCnt := 0
		unexpectedStatusCnt := 0
		for _, instance := range response.Instances.Instance {
			if instance.Status != aimStatus {
				unexpectedStatusCnt++
				p.logger.Warnf("[%s] instance [%s] status is %s, but it is expected to be %s\n",
					p.GetProviderName(), instance.InstanceId, instance.Status, aimStatus)
			}
			master := false
			for _, tag := range instance.Tags.Tag {
				if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
					master = true
					masterCnt++
					break
				}
			}
			p.m.Store(instance.InstanceId, types.Node{
				Master:            master,
				InstanceID:        instance.InstanceId,
				InstanceStatus:    instance.Status,
				InternalIPAddress: instance.InnerIpAddress.IpAddress,
				PublicIPAddress:   []string{instance.EipAddress.IpAddress},
				EipAllocationIds:  []string{instance.EipAddress.AllocationId},
			})
		}
		if unexpectedStatusCnt > 0 {
			return fmt.Errorf("[%s] status of %d instance(s) is unexpected", p.GetProviderName(), unexpectedStatusCnt)
		}
		p.Master = strconv.Itoa(masterCnt)
		p.Worker = strconv.Itoa(len(response.Instances.Instance) - masterCnt)
		return nil
	}
	return fmt.Errorf("[%s] unable to confirm the current status of instance(s)", p.GetProviderName())
}

func (p *Alibaba) describeEipAddresses(allocationIds []string) (*vpc.DescribeEipAddressesResponse, error) {
	if allocationIds == nil {
		return nil, fmt.Errorf("[%s] allocationID can not be empty", p.GetProviderName())
	}
	request := vpc.CreateDescribeEipAddressesRequest()
	request.Scheme = "https"

	request.PageSize = requests.NewInteger(50)
	request.AllocationId = strings.Join(allocationIds, ",")

	response, err := p.v.DescribeEipAddresses(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (p *Alibaba) allocateEipAddresses(num int) ([]vpc.EipAddress, error) {
	var eips []vpc.EipAddress
	for i := 0; i < num; i++ {
		eip, err := p.allocateEipAddress()
		if err != nil {
			return nil, fmt.Errorf("error when allocate eip addresses %v", err)
		}
		eips = append(eips, vpc.EipAddress{
			IpAddress:    eip.EipAddress,
			AllocationId: eip.AllocationId,
		})
	}

	// add tags for eips
	var eipIds []string
	for _, eip := range eips {
		eipIds = append(eipIds, eip.AllocationId)
	}
	tag := []vpc.TagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}
	p.logger.Debugf("[%s] tagging eip(s): %s\n", p.GetProviderName(), eipIds)

	if err := p.tagVpcResources(resourceTypeEip, eipIds, tag); err != nil {
		p.logger.Errorf("[%s] error when tag eip(s): %s\n", p.GetProviderName(), err)
	}

	p.logger.Debugf("[%s] successfully tagged eip(s): %s\n", p.GetProviderName(), eipIds)
	return eips, nil
}

func (p *Alibaba) associateEipAddress(instanceID, allocationID string) error {
	request := vpc.CreateAssociateEipAddressRequest()
	request.Scheme = "https"

	request.InstanceId = instanceID
	request.AllocationId = allocationID

	if _, err := p.v.AssociateEipAddress(request); err != nil {
		return err
	}
	return nil
}

func (p *Alibaba) unassociateEipAddress(allocationID string) error {
	if allocationID == "" {
		return fmt.Errorf("[%s] allocationID can not be empty", p.GetProviderName())
	}
	request := vpc.CreateUnassociateEipAddressRequest()
	request.Scheme = "https"

	request.AllocationId = allocationID

	if _, err := p.v.UnassociateEipAddress(request); err != nil {
		return err
	}
	return nil
}

func (p *Alibaba) allocateEipAddress() (*vpc.AllocateEipAddressResponse, error) {
	request := vpc.CreateAllocateEipAddressRequest()
	request.Scheme = "https"

	response, err := p.v.AllocateEipAddress(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (p *Alibaba) releaseEipAddress(allocationID string) error {
	if allocationID == "" {
		return fmt.Errorf("[%s] allocationID can not be empty", p.GetProviderName())
	}
	request := vpc.CreateReleaseEipAddressRequest()
	request.Scheme = "https"

	request.AllocationId = allocationID

	if _, err := p.v.ReleaseEipAddress(request); err != nil {
		return err
	}
	return nil
}

func (p *Alibaba) releaseEipAddresses(rollBack bool) {
	var releaseEipIds []string

	// unassociate master eip address.
	for _, master := range p.MasterNodes {
		if master.RollBack == rollBack {
			p.logger.Debugf("[%s] unassociating eip address for %d master(s)\n", p.GetProviderName(), len(p.MasterNodes))

			for _, allocationID := range master.EipAllocationIds {
				if err := p.unassociateEipAddress(allocationID); err != nil {
					p.logger.Errorf("[%s] error when unassociating eip address %s: %v\n", p.GetProviderName(), allocationID, err)
				}
				releaseEipIds = append(releaseEipIds, allocationID)
			}

			p.logger.Debugf("[%s] successfully unassociated eip address for master(s)\n", p.GetProviderName())
		}
	}

	// unassociate worker eip address.
	for _, worker := range p.WorkerNodes {
		if worker.RollBack == rollBack {
			p.logger.Debugf("[%s] unassociating eip address for %d worker(s)\n", p.GetProviderName(), len(p.WorkerNodes))

			for _, allocationID := range worker.EipAllocationIds {
				if err := p.unassociateEipAddress(allocationID); err != nil {
					p.logger.Errorf("[%s] error when unassociating eip address %s: %v\n", p.GetProviderName(), allocationID, err)
				}
				releaseEipIds = append(releaseEipIds, allocationID)
			}

			p.logger.Debugf("[%s] successfully unassociated eip address for worker(s)\n", p.GetProviderName())
		}
	}

	// list eips with tags.
	tags := []vpc.ListTagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}
	allocationIds, err := p.listVpcTagResources(resourceTypeEip, releaseEipIds, tags)
	if err != nil {
		p.logger.Errorf("[%s] error when query eip address: %v\n", p.GetProviderName(), err)
	}

	// run delete command without a state file
	if !rollBack && len(p.MasterNodes) == 0 && len(p.WorkerNodes) == 0 {
		for _, allocationID := range allocationIds {
			if err := p.unassociateEipAddress(allocationID); err != nil {
				p.logger.Errorf("[%s] error when unassociating eip address %s: %v\n", p.GetProviderName(), allocationID, err)
			}
		}
	}

	// eip can be released only when status is `Available`.
	// wait eip to be `Available` status.
	if err := p.getEipStatus(allocationIds, eipStatusAvailable); err != nil {
		p.logger.Errorf("[%s] error when query eip status: %v\n", p.GetProviderName(), err)
	}

	// release eips.
	for _, allocationID := range allocationIds {
		p.logger.Debugf("[%s] releasing eip: %s\n", p.GetProviderName(), allocationID)

		if err := p.releaseEipAddress(allocationID); err != nil {
			p.logger.Errorf("[%s] error when releasing eip address %s: %v\n", p.GetProviderName(), allocationID, err)
		} else {
			p.logger.Debugf("[%s] successfully released eip: %s\n", p.GetProviderName(), allocationID)
		}
	}
}

func (p *Alibaba) getEipStatus(allocationIds []string, aimStatus string) error {
	if allocationIds == nil {
		return fmt.Errorf("[%s] allocationIds can not be empty", p.GetProviderName())
	}

	p.logger.Debugf("[%s] waiting eip(s) to be in `%s` status...\n", p.GetProviderName(), aimStatus)

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		response, err := p.describeEipAddresses(allocationIds)
		if err != nil || !response.IsSuccess() || len(response.EipAddresses.EipAddress) <= 0 {
			return false, nil
		}

		for _, eip := range response.EipAddresses.EipAddress {
			p.logger.Debugf("[%s] eip(s) [%s] is in `%s` status\n", p.GetProviderName(), eip.AllocationId, eip.Status)

			if eip.Status != aimStatus {
				return false, nil
			}
		}

		return true, nil
	}); err != nil {
		return fmt.Errorf("[%s] error in querying eip(s) %s status of [%s], msg: [%v]", p.GetProviderName(), aimStatus, allocationIds, err)
	}

	p.logger.Debugf("[%s] eip(s) are in `%s` status\n", p.GetProviderName(), aimStatus)

	return nil
}

func (p *Alibaba) listVpcTagResources(resourceType string, resourceID []string, tag []vpc.ListTagResourcesTag) ([]string, error) {
	request := vpc.CreateListTagResourcesRequest()
	request.Scheme = "https"

	request.ResourceType = resourceType
	if resourceID != nil {
		request.ResourceId = &resourceID
	}
	if tag != nil {
		request.Tag = &tag
	}

	response, err := p.v.ListTagResources(request)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, resource := range response.TagResources.TagResource {
		ids = append(ids, resource.ResourceId)
	}

	return utils.UniqueArray(ids), nil

}

func (p *Alibaba) tagVpcResources(resourceType string, resourceIds []string, tag []vpc.TagResourcesTag) error {
	request := vpc.CreateTagResourcesRequest()
	request.Scheme = "https"

	request.ResourceType = resourceType
	request.ResourceId = &resourceIds
	request.Tag = &tag

	if _, err := p.v.TagResources(request); err != nil {
		return err
	}
	return nil
}

func (p *Alibaba) generateInstance(fn checkFun, ssh *types.SSH) (*types.Cluster, error) {
	var (
		masterEips []vpc.EipAddress
		workerEips []vpc.EipAddress
		err        error
	)

	if err = p.generateClientSDK(); err != nil {
		return nil, err
	}

	if err = fn(); err != nil {
		return nil, err
	}

	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.logger.Debugf("[%s] %d masters and %d workers will be added\n", p.GetProviderName(), masterNum, workerNum)
	if masterNum > 0 {
		if masterEips, err = p.allocateEipAddresses(masterNum); err != nil {
			return nil, err
		}
		p.logger.Debugf("[%s] successfully allocated %d eip(s) for master(s)\n", p.GetProviderName(), masterNum)
	}

	if workerNum > 0 {
		p.logger.Debugf("[%s] allocating %d eip(s) for worker(s)\n", p.GetProviderName(), workerNum)
		if workerEips, err = p.allocateEipAddresses(workerNum); err != nil {
			return nil, err
		}
		p.logger.Debugf("[%s] successfully allocated %d eip(s) for worker(s)\n", p.GetProviderName(), workerNum)
	}

	// run ecs master instances.
	if masterNum > 0 {
		if err := p.runInstances(masterNum, true); err != nil {
			return nil, err
		}
	}

	// run ecs worker instances.
	if workerNum > 0 {
		if err := p.runInstances(workerNum, false); err != nil {
			return nil, err
		}
	}

	// wait ecs instances to be running status.
	if err = p.getInstanceStatus(alibaba.StatusRunning); err != nil {
		return nil, err
	}

	var associatedEipIds []string

	// associate master eip
	if masterEips != nil {
		p.logger.Debugf("[%s] associating %d eip(s) for master(s)\n", p.GetProviderName(), masterNum)

		j := 0
		for i, master := range p.Status.MasterNodes {
			if p.Status.MasterNodes[i].PublicIPAddress == nil {
				err := p.associateEipAddress(master.InstanceID, masterEips[j].AllocationId)
				if err != nil {
					return nil, err
				}
				p.Status.MasterNodes[i].EipAllocationIds = append(p.Status.MasterNodes[i].EipAllocationIds, masterEips[j].AllocationId)
				p.Status.MasterNodes[i].PublicIPAddress = append(p.Status.MasterNodes[i].PublicIPAddress, masterEips[j].IpAddress)
				associatedEipIds = append(associatedEipIds, masterEips[j].AllocationId)
				j++
			}
		}
		p.logger.Debugf("[%s] successfully associated %d eip(s) for master(s)\n", p.GetProviderName(), masterNum)
	}

	// associate worker eip
	if workerEips != nil {
		p.logger.Debugf("[%s] associating %d eip(s) for worker(s)\n", p.GetProviderName(), workerNum)

		j := 0
		for i, worker := range p.Status.WorkerNodes {
			if p.Status.WorkerNodes[i].PublicIPAddress == nil {
				err := p.associateEipAddress(worker.InstanceID, workerEips[j].AllocationId)
				if err != nil {
					return nil, err
				}
				p.Status.WorkerNodes[i].EipAllocationIds = append(p.Status.WorkerNodes[i].EipAllocationIds, workerEips[j].AllocationId)
				p.Status.WorkerNodes[i].PublicIPAddress = append(p.Status.WorkerNodes[i].PublicIPAddress, workerEips[j].IpAddress)
				associatedEipIds = append(associatedEipIds, workerEips[j].AllocationId)
				j++
			}
		}
		p.logger.Debugf("[%s] successfully associated %d eip(s) for worker(s)\n", p.GetProviderName(), workerNum)
	}

	// wait eip to be InUse status.
	if err = p.getEipStatus(associatedEipIds, eipStatusInUse); err != nil {
		return nil, err
	}

	// assemble instance status.
	var c *types.Cluster
	if c, err = p.assembleInstanceStatus(ssh); err != nil {
		return nil, err
	}

	c.InstallScript = k3sScript
	c.Mirror = k3sMirror
	c.DockerMirror = dockerMirror

	if option, ok := c.Options.(alibaba.Options); ok {
		if strings.EqualFold(option.Terway.Mode, "eni") {
			c.Network = "none"
		}
		if strings.EqualFold(c.CloudControllerManager, "true") {
			c.MasterExtraArgs += " --disable-cloud-controller --no-deploy servicelb --kubelet-arg=cloud-provider=external"
		}
	}

	return c, nil
}
