package alibaba

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts/dialer"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/errors"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	k3sInstallScript         = "https://rancher-mirror.rancher.cn/k3s/k3s-install.sh"
	accessKeyID              = "access-key"
	accessKeySecret          = "access-secret"
	resourceTypeEip          = "EIP"
	eipStatusAvailable       = "Available"
	eipStatusInUse           = "InUse"
	vpcCidrBlock             = "10.0.0.0/8"
	vSwitchCidrBlock         = "10.3.0.0/20"
	ipRange                  = "0.0.0.0/0"
	vpcName                  = "autok3s-aliyun-vpc"
	vSwitchName              = "autok3s-aliyun-vswitch"
	defaultSecurityGroupName = "autok3s"
	vpcStatusAvailable       = "Available"
	defaultUser              = "root"
)

// providerName is the name of this provider.
const providerName = "alibaba"

var (
	k3sMirror        = "INSTALL_K3S_MIRROR=cn"
	deployCCMCommand = "echo \"%s\" | base64 -d | tee \"%s/cloud-controller-manager.yaml\""
)

// Alibaba provider alibaba struct.
type Alibaba struct {
	*cluster.ProviderBase `json:",inline"`
	alibaba.Options       `json:",inline"`
	VpcCIDR               string

	c *ecs.Client
	v *vpc.Client
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Alibaba {
	base := cluster.NewBaseProvider()
	base.Provider = providerName
	base.InstallScript = k3sInstallScript
	alibabaProvider := &Alibaba{
		ProviderBase: base,
	}
	if opt, ok := common.DefaultTemplates[providerName]; ok {
		alibabaProvider.Options = opt.(alibaba.Options)
	}
	return alibabaProvider
}

// GetProviderName returns provider name.
func (p *Alibaba) GetProviderName() string {
	return p.Provider
}

// GenerateClusterName generates and returns cluster name.
func (p *Alibaba) GenerateClusterName() string {
	p.ContextName = fmt.Sprintf("%s.%s.%s", p.Name, p.Region, p.GetProviderName())
	return p.ContextName
}

// GenerateManifest generates manifest deploy command.
func (p *Alibaba) GenerateManifest() []string {
	extraManifests := make([]string, 0)
	if p.CloudControllerManager {
		// deploy additional Alibaba cloud-controller-manager manifests.
		aliCCM := &alibaba.CloudControllerManager{
			Region:       p.Region,
			AccessKey:    p.AccessKey,
			AccessSecret: p.AccessSecret,
		}
		tmpl := fmt.Sprintf(alibabaCCMTmpl, base64.StdEncoding.EncodeToString([]byte(aliCCM.AccessKey)), base64.StdEncoding.EncodeToString([]byte(aliCCM.AccessSecret)), p.ClusterCidr, aliCCM.Region)
		extraManifests = append(extraManifests, fmt.Sprintf(deployCCMCommand,
			base64.StdEncoding.EncodeToString([]byte(tmpl)), common.K3sManifestsDir))
	}
	return extraManifests
}

// CreateK3sCluster create K3S cluster.
func (p *Alibaba) CreateK3sCluster() (err error) {
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	return p.InitCluster(p.Options, p.GenerateManifest, p.generateInstance, nil, p.rollbackInstance)
}

// JoinK3sNode join K3S node.
func (p *Alibaba) JoinK3sNode() (err error) {
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	return p.JoinNodes(p.generateInstance, func() error { return nil }, false, p.rollbackInstance)
}

func (p *Alibaba) rollbackInstance(ids []string) error {
	if len(ids) > 0 {
		p.releaseEipAddresses(true)

		request := ecs.CreateDeleteInstancesRequest()
		request.Scheme = "https"
		request.InstanceId = &ids
		request.Force = requests.NewBoolean(true)

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

// DeleteK3sCluster delete K3S cluster.
func (p *Alibaba) DeleteK3sCluster(f bool) error {
	return p.DeleteCluster(f, p.deleteInstance)
}

// SSHK3sNode ssh K3s node.
func (p *Alibaba) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	return p.Connect(ip, &p.SSH, c, p.getInstanceNodes, p.isInstanceRunning, nil)
}

// IsClusterExist determine if the cluster exists.
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
	request.Tag = &[]ecs.ListTagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.ContextName}}

	response, err := p.c.ListTagResources(request)
	if err != nil {
		if e, ok := err.(*errors.ServerError); ok {
			if e.ErrorCode() == "InvalidAccessKeyId.NotFound" {
				return false, nil, fmt.Errorf("[%s] invalid credential: %s", p.GetProviderName(), e.Message())
			} else if e.ErrorCode() == "Forbidden.AccessKeyDisabled" {
				return false, nil, fmt.Errorf("[%s] your access key is disabled", p.GetProviderName())
			}
		}
		return false, nil, err
	}
	if len(response.TagResources.TagResource) > 0 {
		for _, resource := range response.TagResources.TagResource {
			ids = append(ids, resource.ResourceId)
		}
		// ecs will return multiple instance ids based on the value of tag key.n by n, so duplicate items need to be removed.
		return true, utils.UniqueArray(ids), nil
	}
	return false, nil, nil
}

// GenerateMasterExtraArgs generates K3S master extra args.
func (p *Alibaba) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	if option, ok := cluster.Options.(alibaba.Options); ok {
		if option.CloudControllerManager {
			extraArgs := fmt.Sprintf(" --kubelet-arg=cloud-provider=external --kubelet-arg=provider-id=%s.%s --node-name=%s.%s",
				option.Region, master.InstanceID, option.Region, master.InstanceID)
			return extraArgs
		}
	}
	return ""
}

// GenerateWorkerExtraArgs generates K3S worker extra args.
func (p *Alibaba) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return p.GenerateMasterExtraArgs(cluster, worker)
}

// GetCluster returns cluster status.
func (p *Alibaba) GetCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Region:   p.Region,
		Zone:     p.Zone,
		Provider: p.GetProviderName(),
	}

	return p.GetClusterStatus(kubecfg, c, p.getInstanceNodes)
}

// DescribeCluster describe cluster info.
func (p *Alibaba) DescribeCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		Name:     p.Name,
		Region:   p.Region,
		Zone:     p.Zone,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubecfg, c, p.getInstanceNodes)
}

// SetConfig set cluster config.
func (p *Alibaba) SetConfig(config []byte) error {
	c, err := p.SetClusterConfig(config)
	if err != nil {
		return err
	}
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	b, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	opt := &alibaba.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

	return nil
}

// SetOptions set options.
func (p *Alibaba) SetOptions(opt []byte) error {
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	option := &alibaba.Options{}
	err := json.Unmarshal(opt, option)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(option).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// GetProviderOptions get provider options.
func (p *Alibaba) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &alibaba.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

func (p *Alibaba) generateClientSDK() error {
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

func (p *Alibaba) runInstances(num int, master bool, password string) error {
	request := ecs.CreateRunInstancesRequest()
	request.Scheme = "https"
	request.InstanceType = p.InstanceType
	request.ImageId = p.Image
	request.VSwitchId = p.VSwitch
	request.KeyPairName = p.KeyPair
	request.SystemDiskCategory = p.DiskCategory
	request.SystemDiskSize = p.DiskSize
	request.SecurityGroupId = p.SecurityGroup
	request.Amount = requests.NewInteger(num)
	request.UniqueSuffix = requests.NewBoolean(false)
	request.UserData = p.UserDataContent
	request.SpotStrategy = p.SpotStrategy
	request.SpotDuration = requests.NewInteger(p.SpotDuration)
	request.SpotPriceLimit = requests.NewFloat(p.SpotPriceLimit)
	// check `--eip` value
	if !p.EIP {
		bandwidth, err := strconv.Atoi(p.InternetMaxBandwidthOut)
		if err != nil {
			p.Logger.Warnf("[%s] `--internet-max-bandwidth-out` value %s is invalid, "+
				"need to be integer, will use default value to create instance", p.GetProviderName(), p.InternetMaxBandwidthOut)
			bandwidth = 5
		}
		request.InternetMaxBandwidthOut = requests.NewInteger(bandwidth)
	}

	if p.Zone != "" {
		request.ZoneId = p.Zone
	}
	if password != "" {
		request.Password = password
	}

	tags := []ecs.RunInstancesTag{
		{Key: "autok3s", Value: "true"},
		{Key: "cluster", Value: common.TagClusterPrefix + p.ContextName},
	}

	for _, v := range p.Tags {
		ss := strings.Split(v, "=")
		if len(ss) != 2 {
			return fmt.Errorf("tags %s invalid", v)
		}
		tags = append(tags, ecs.RunInstancesTag{
			Key:   ss[0],
			Value: ss[1],
		})
	}

	if master {
		request.InstanceName = fmt.Sprintf(common.MasterInstanceName, p.ContextName)
		tags = append(tags, ecs.RunInstancesTag{Key: "master", Value: "true"})
	} else {
		request.InstanceName = fmt.Sprintf(common.WorkerInstanceName, p.ContextName)
		tags = append(tags, ecs.RunInstancesTag{Key: "worker", Value: "true"})
	}
	request.Tag = &tags

	response, err := p.c.RunInstances(request)
	if err != nil || len(response.InstanceIdSets.InstanceIdSet) != num {
		return fmt.Errorf("[%s] calling runInstances error. region: %s, zone: %s, "+"instanceName: %s, msg: [%v]",
			p.GetProviderName(), p.Region, p.Zone, request.InstanceName, err)
	}
	for _, id := range response.InstanceIdSets.InstanceIdSet {
		p.M.Store(id, types.Node{Master: master, RollBack: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
	}

	return nil
}

func (p *Alibaba) deleteInstance(f bool) (string, error) {
	exist, ids, err := p.IsClusterExist()
	if err != nil && !f {
		return "", fmt.Errorf("[%s] calling deleteCluster error, msg: %v", p.GetProviderName(), err)
	}
	if !exist {
		p.Logger.Errorf("[%s] cluster %s is not exist", p.GetProviderName(), p.Name)
		if !f {
			return "", fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", p.GetProviderName(), p.Name)
		}
		return p.ContextName, nil
	}

	ui := p.UI
	for _, comp := range p.Enable {
		if !ui && comp == "dashboard" {
			ui = true
		}
	}
	// This is for backward compatibility as we don't support deploying kubernetes dashboard anymore.
	// CCM will create elb for kubernetes dashboard so we need to delete dashboard before delete instance/cluster.
	if ui && p.CloudControllerManager {
		if err = p.ReleaseManifests(); err != nil {
			return "", err
		}
	}
	p.releaseEipAddresses(false)
	if err == nil && len(ids) > 0 {
		p.Logger.Infof("[%s] cluster %s will be deleted", p.GetProviderName(), p.Name)

		request := ecs.CreateDeleteInstancesRequest()
		request.Scheme = "https"
		request.RegionId = p.Region
		request.InstanceId = &ids
		request.Force = requests.NewBoolean(true)
		request.TerminateSubscription = requests.NewBoolean(true)

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			response, err := p.c.DeleteInstances(request)
			if err != nil || !response.IsSuccess() {
				return false, nil
			}
			return true, nil
		}); err != nil {
			return "", fmt.Errorf("[%s] calling deleteInstance error, msg: %v", p.GetProviderName(), err)
		}
	}

	// remove default key-pair folder
	err = os.RemoveAll(common.GetClusterPath(p.ContextName, p.GetProviderName()))
	if err != nil && !f {
		return "", fmt.Errorf("[%s] remove cluster store folder (%s) error, msg: %v", p.GetProviderName(), common.GetClusterPath(p.ContextName, p.GetProviderName()), err)
	}

	return p.ContextName, nil
}

func (p *Alibaba) getInstanceStatus(aimStatus string) error {
	ids := make([]string, 0)
	p.M.Range(func(key, _ interface{}) bool {
		ids = append(ids, key.(string))
		return true
	})

	if len(ids) > 0 {
		p.Logger.Infof("[%s] waiting for the instances %s to be in `%s` status...", p.GetProviderName(), ids, aimStatus)
		request := ecs.CreateDescribeInstanceStatusRequest()
		request.Scheme = "https"
		request.InstanceId = &ids

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			response, err := p.c.DescribeInstanceStatus(request)
			if err != nil || !response.IsSuccess() || len(response.InstanceStatuses.InstanceStatus) <= 0 {
				return false, err
			}

			for _, status := range response.InstanceStatuses.InstanceStatus {
				if status.Status == aimStatus {
					if value, ok := p.M.Load(status.InstanceId); ok {
						v := value.(types.Node)
						v.InstanceStatus = aimStatus
						p.M.Store(status.InstanceId, v)
					}
					continue
				}
				return false, nil
			}
			return true, nil
		}); err != nil {
			return err
		}
	}

	p.Logger.Infof("[%s] instances %s are in `%s` status", p.GetProviderName(), ids, aimStatus)

	return nil
}

func (p *Alibaba) isInstanceRunning(state string) bool {
	return state == alibaba.StatusRunning
}

func (p *Alibaba) assembleInstanceStatus(ssh *types.SSH, uploadKeyPair bool, publicKey string) error {
	instanceList, err := p.describeInstances()
	if err != nil {
		return err
	}

	for _, status := range instanceList {
		publicIPAddress := status.PublicIpAddress.IpAddress
		eip := make([]string, 0)
		if p.EIP {
			publicIPAddress = []string{status.EipAddress.IpAddress}
			eip = []string{status.EipAddress.AllocationId}
		}
		if value, ok := p.M.Load(status.InstanceId); ok {
			v := value.(types.Node)
			// add only nodes that run the current command.
			v.Current = true
			v.InternalIPAddress = status.VpcAttributes.PrivateIpAddress.IpAddress
			v.PublicIPAddress = publicIPAddress
			v.EipAllocationIds = eip
			v.LocalHostname = status.HostName
			v.SSH = *ssh
			// check upload keypair
			if uploadKeyPair {
				p.Logger.Infof("[%s] Waiting for upload keypair...", p.GetProviderName())
				if err := p.uploadKeyPair(v, publicKey); err != nil {
					return err
				}
				v.SSH.SSHPassword = ""
			}
			p.M.Store(status.InstanceId, v)
			continue
		}
		master := false
		for _, tag := range status.Tags.Tag {
			if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
				master = true
				break
			}
		}
		p.M.Store(status.InstanceId, types.Node{
			Master:            master,
			RollBack:          false,
			InstanceID:        status.InstanceId,
			InstanceStatus:    status.Status,
			InternalIPAddress: status.VpcAttributes.PrivateIpAddress.IpAddress,
			EipAllocationIds:  publicIPAddress,
			PublicIPAddress:   eip})
	}
	return nil
}

func (p *Alibaba) describeInstances() ([]ecs.Instance, error) {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	pageSize := 20
	request.PageSize = requests.NewInteger(pageSize)
	request.Tag = &[]ecs.DescribeInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.ContextName}}
	if p.Zone != "" {
		request.ZoneId = p.Zone
	}
	instanceList := make([]ecs.Instance, 0)
	totalPage := 0
	for {
		response, err := p.c.DescribeInstances(request)
		if err != nil || !response.IsSuccess() {
			return nil, fmt.Errorf("[%s] failed to get all instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
		}
		total := response.TotalCount
		pageNum := response.PageNumber
		if totalPage == 0 {
			totalPage = total / pageSize
			if total%pageSize != 0 {
				totalPage = totalPage + 1
			}
		}
		instanceList = append(instanceList, response.Instances.Instance...)
		if (pageNum + 1) > totalPage {
			break
		}
		request.PageNumber = requests.NewInteger(pageNum + 1)
	}

	if len(instanceList) == 0 {
		return nil, fmt.Errorf("[%s] there's no instance for cluster %s at region: %s, zone: %s",
			p.GetProviderName(), p.Name, p.Region, p.Zone)
	}

	return instanceList, nil
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

// func (p *Alibaba) getVpcCIDR() (string, error) {
// 	vpcID, vSwitchCIDR, err := p.getVSwitchCIDR()
// 	if err != nil {
// 		return "", fmt.Errorf("[%s] calling preflight error: vswitch %s cidr not be found",
// 			p.GetProviderName(), p.VSwitch)
// 	}

// 	p.ClusterCidr = vSwitchCIDR

// 	request := ecs.CreateDescribeVpcsRequest()
// 	request.Scheme = "https"
// 	request.VpcId = vpcID

// 	response, err := p.c.DescribeVpcs(request)
// 	if err != nil || !response.IsSuccess() || len(response.Vpcs.Vpc) != 1 {
// 		return "", fmt.Errorf("[%s] calling describeVpcs error. region: %s, "+"instanceName: %s, msg: [%v]",
// 			p.GetProviderName(), p.Region, p.Vpc, err)
// 	}

// 	return response.Vpcs.Vpc[0].CidrBlock, nil
// }

// CreateCheck check create command and flags.
func (p *Alibaba) CreateCheck() error {
	if err := p.CheckCreateArgs(p.IsClusterExist); err != nil {
		return err
	}

	if err := p.ValidateRequireSSHPrivateKey(); p.KeyPair != "" && err != nil {
		return fmt.Errorf("[%s] calling preflight error: %s with --key-pair %s", p.GetProviderName(), err.Error(), p.KeyPair)
	}

	if p.UserDataPath != "" {
		_, err := os.Stat(p.UserDataPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// JoinCheck check join command and flags.
func (p *Alibaba) JoinCheck() error {
	return p.CheckJoinArgs(p.IsClusterExist)
}

func (p *Alibaba) describeEipAddresses(allocationIds []string) ([]vpc.EipAddress, error) {
	request := vpc.CreateDescribeEipAddressesRequest()
	request.Scheme = "https"

	pageSize := 20
	request.PageSize = requests.NewInteger(pageSize)
	index := 0
	count := len(allocationIds) / pageSize
	if len(allocationIds)%pageSize != 0 {
		count = count + 1
	}
	var pagedAllocationIds []string
	eipList := make([]vpc.EipAddress, 0)
	for i := 0; i < count; i++ {
		if (index + pageSize) > len(allocationIds) {
			pagedAllocationIds = allocationIds[index:]
		} else {
			pagedAllocationIds = allocationIds[index : index+pageSize]
		}
		index = index + pageSize
		request.AllocationId = strings.Join(pagedAllocationIds, ",")

		response, err := p.v.DescribeEipAddresses(request)
		if err != nil || !response.IsSuccess() || len(response.EipAddresses.EipAddress) <= 0 {
			return nil, err
		}
		eipList = append(eipList, response.EipAddresses.EipAddress...)
	}

	return eipList, nil
}

func (p *Alibaba) allocateEipAddresses(num int) ([]vpc.EipAddress, error) {
	var eips []vpc.EipAddress
	var eipIds []string
	for i := 0; i < num; i++ {
		eip, err := p.allocateEipAddress()
		if err != nil {
			return nil, fmt.Errorf("error when allocate eip addresses %v", err)
		}
		eips = append(eips, vpc.EipAddress{
			IpAddress:    eip.EipAddress,
			AllocationId: eip.AllocationId,
		})
		eipIds = append(eipIds, eip.AllocationId)
	}

	// add tags for eips.
	tag := []vpc.TagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.ContextName}}
	p.Logger.Infof("[%s] tagging eip(s): %s", p.GetProviderName(), eipIds)

	if err := p.tagVpcResources(resourceTypeEip, eipIds, tag); err != nil {
		p.Logger.Errorf("[%s] error when tag eip(s): %s", p.GetProviderName(), err)
	}

	p.Logger.Infof("[%s] successfully tagged eip(s): %s", p.GetProviderName(), eipIds)
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
	request.Bandwidth = p.InternetMaxBandwidthOut

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

	if rollBack {
		// unassociate master eip address.
		for _, master := range p.MasterNodes {
			if master.RollBack == rollBack {
				p.Logger.Infof("[%s] unassociating eip address for %d master(s)", p.GetProviderName(), len(p.MasterNodes))

				for _, allocationID := range master.EipAllocationIds {
					if err := p.unassociateEipAddress(allocationID); err != nil {
						p.Logger.Errorf("[%s] error when unassociating eip address %s: %v", p.GetProviderName(), allocationID, err)
					}
					releaseEipIds = append(releaseEipIds, allocationID)
				}

				p.Logger.Infof("[%s] successfully unassociated eip address for master(s)", p.GetProviderName())
			}
		}

		// unassociate worker eip address.
		for _, worker := range p.WorkerNodes {
			if worker.RollBack == rollBack {
				p.Logger.Infof("[%s] unassociating eip address for %d worker(s)", p.GetProviderName(), len(p.WorkerNodes))

				for _, allocationID := range worker.EipAllocationIds {
					if err := p.unassociateEipAddress(allocationID); err != nil {
						p.Logger.Errorf("[%s] error when unassociating eip address %s: %v", p.GetProviderName(), allocationID, err)
					}
					releaseEipIds = append(releaseEipIds, allocationID)
				}

				p.Logger.Infof("[%s] successfully unassociated eip address for worker(s)", p.GetProviderName())
			}
		}

		// no eip need rollback.
		if len(releaseEipIds) == 0 {
			p.Logger.Infof("[%s] no eip need execute rollback logic", p.GetProviderName())
			return
		}
	}

	// list eips with tags.
	tags := []vpc.ListTagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.ContextName}}
	allocationIds, err := p.listVpcTagResources(resourceTypeEip, releaseEipIds, tags)
	if err != nil {
		p.Logger.Errorf("[%s] error when query eip address: %v", p.GetProviderName(), err)
	}

	if !rollBack {
		for _, allocationID := range allocationIds {
			if err := p.unassociateEipAddress(allocationID); err != nil {
				p.Logger.Errorf("[%s] error when unassociating eip address %s: %v", p.GetProviderName(), allocationID, err)
			}
		}
	}
	if len(allocationIds) == 0 {
		return
	}

	// eip can be released only when status is `Available`.
	// wait eip to be `Available` status.
	if err := p.getEipStatus(allocationIds, eipStatusAvailable); err != nil {
		p.Logger.Errorf("[%s] error when query eip status: %v", p.GetProviderName(), err)
	}

	// release eips.
	for _, allocationID := range allocationIds {
		p.Logger.Infof("[%s] releasing eip: %s", p.GetProviderName(), allocationID)

		if err := p.releaseEipAddress(allocationID); err != nil {
			p.Logger.Errorf("[%s] error when releasing eip address %s: %v", p.GetProviderName(), allocationID, err)
		} else {
			p.Logger.Infof("[%s] successfully released eip: %s", p.GetProviderName(), allocationID)
		}
	}
}

func (p *Alibaba) getEipStatus(allocationIds []string, aimStatus string) error {
	if len(allocationIds) == 0 {
		return fmt.Errorf("[%s] allocationIds can not be empty", p.GetProviderName())
	}

	p.Logger.Infof("[%s] waiting eip(s) to be in `%s` status...", p.GetProviderName(), aimStatus)

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		eipList, err := p.describeEipAddresses(allocationIds)
		if err != nil || eipList == nil {
			return false, err
		}

		for _, eip := range eipList {
			p.Logger.Infof("[%s] eip(s) [%s: %s] is in `%s` status", p.GetProviderName(), eip.AllocationId, eip.IpAddress, eip.Status)

			if eip.Status != aimStatus {
				return false, nil
			}
		}

		return true, nil
	}); err != nil {
		return fmt.Errorf("[%s] error in querying eip(s) %s status of [%s], msg: [%v]", p.GetProviderName(), aimStatus, allocationIds, err)
	}

	p.Logger.Infof("[%s] eip(s) are in `%s` status", p.GetProviderName(), aimStatus)

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

func (p *Alibaba) generateInstance(ssh *types.SSH) (*types.Cluster, error) {
	if err := p.generateClientSDK(); err != nil {
		return nil, err
	}

	// create key pair.
	pk, err := p.createKeyPair(ssh)
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to create key pair: %v", p.GetProviderName(), err)
	}

	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.Logger.Infof("[%s] %d masters and %d workers will be added in region %s", p.GetProviderName(), masterNum, workerNum, p.Region)

	if p.VSwitch == "" {
		// get default vpc and vswitch.
		err := p.configNetwork()
		if err != nil {
			return nil, err
		}
	}

	if p.SecurityGroup == "" {
		// get default security group.
		err := p.configSecurityGroup()
		if err != nil {
			return nil, err
		}
	}

	needUploadKeyPair := false
	if ssh.SSHPassword == "" && p.KeyPair == "" {
		needUploadKeyPair = true
		ssh.SSHPassword = putil.RandomPassword()
		p.Logger.Debugf("[%s] launching instance with auto-generated password...", p.GetProviderName())
	}

	if p.UserDataPath != "" {
		userDataBytes, err := os.ReadFile(p.UserDataPath)
		if err != nil {
			return nil, err
		}
		if len(userDataBytes) > 0 {
			p.UserDataContent = base64.StdEncoding.EncodeToString(userDataBytes)
		}
	}

	// run ecs master instances.
	if masterNum > 0 {
		p.Logger.Infof("[%s] prepare for %d of master instances", p.GetProviderName(), masterNum)
		if err := p.runInstances(masterNum, true, ssh.SSHPassword); err != nil {
			return nil, err
		}
		p.Logger.Infof("[%s] %d of master instances created successfully", p.GetProviderName(), masterNum)
	}

	// run ecs worker instances.
	if workerNum > 0 {
		p.Logger.Infof("[%s] prepare for %d of worker instances", p.GetProviderName(), workerNum)
		if err := p.runInstances(workerNum, false, ssh.SSHPassword); err != nil {
			return nil, err
		}
		p.Logger.Infof("[%s] %d of worker instances created successfully", p.GetProviderName(), workerNum)
	}

	// wait ecs instances to be running status.
	if err = p.getInstanceStatus(alibaba.StatusRunning); err != nil {
		return nil, err
	}

	if p.EIP {
		// 1. ensure all instances are successfully created
		// 2. allocate eip for master and associate
		// 3. allocate eip for worker and associate
		// Make sure each step is successful before proceeding to the next step.
		// Otherwise, the `Rollback()` will cause the eip fail to be released.
		var associatedEipIds []string

		// allocate eip for master
		if masterNum > 0 {
			eipIds, err := p.assignEIPToInstance(masterNum, true)
			if err != nil {
				return nil, err
			}
			associatedEipIds = append(associatedEipIds, eipIds...)
		}

		// allocate eip for worker.
		if workerNum > 0 {
			eipIds, err := p.assignEIPToInstance(workerNum, false)
			if err != nil {
				return nil, err
			}
			associatedEipIds = append(associatedEipIds, eipIds...)
		}

		// wait eip to be InUse status.
		if err = p.getEipStatus(associatedEipIds, eipStatusInUse); err != nil {
			return nil, err
		}
	}

	if err = p.assembleInstanceStatus(ssh, needUploadKeyPair, pk); err != nil {
		return nil, err
	}
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	c.ContextName = p.ContextName
	c.Mirror = k3sMirror

	if _, ok := c.Options.(alibaba.Options); ok {
		if p.CloudControllerManager {
			c.MasterExtraArgs += " --disable-cloud-controller --disable servicelb,traefik"
		}
	}
	c.SSH = *ssh

	return c, nil
}

func (p *Alibaba) assignEIPToInstance(num int, master bool) ([]string, error) {
	var e error
	eipIds := make([]string, 0)
	eips, err := p.allocateEipAddresses(num)
	if err != nil {
		return nil, err
	}
	// associate eip with instance.
	if eips != nil {
		p.Logger.Infof("[%s] prepare for associating %d eip(s) for instance(s)", p.GetProviderName(), num)

		p.M.Range(func(key, value interface{}) bool {
			v := value.(types.Node)
			if v.Master == master && v.PublicIPAddress == nil {
				eipIds = append(eipIds, eips[0].AllocationId)
				v.EipAllocationIds = append(v.EipAllocationIds, eips[0].AllocationId)
				v.PublicIPAddress = append(v.PublicIPAddress, eips[0].IpAddress)
				e = p.associateEipAddress(v.InstanceID, eips[0].AllocationId)
				if e != nil {
					return false
				}
				eips = eips[1:]
				p.M.Store(v.InstanceID, v)
			}
			return true
		})
		p.Logger.Infof("[%s] associated %d eip(s) for instance(s) successfully", p.GetProviderName(), num)
	}

	return eipIds, nil
}

func (p *Alibaba) getInstanceNodes() ([]types.Node, error) {
	if err := p.generateClientSDK(); err != nil {
		return nil, err
	}
	output, err := p.describeInstances()
	if err != nil || len(output) == 0 {
		return nil, fmt.Errorf("[%s] there's no instance for cluster %s: %v", p.GetProviderName(), p.ContextName, err)
	}
	nodes := make([]types.Node, 0)
	for _, instance := range output {
		// sync all instance that belongs to current clusters.
		master := false
		for _, tag := range instance.Tags.Tag {
			if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
				master = true
				break
			}
		}

		node := types.Node{
			Master:            master,
			InstanceID:        instance.InstanceId,
			InstanceStatus:    instance.Status,
			InternalIPAddress: instance.VpcAttributes.PrivateIpAddress.IpAddress,
			PublicIPAddress:   instance.PublicIpAddress.IpAddress,
		}
		if p.EIP {
			node.PublicIPAddress = []string{instance.EipAddress.IpAddress}
			node.EipAllocationIds = []string{instance.EipAddress.AllocationId}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (p *Alibaba) createKeyPair(ssh *types.SSH) (string, error) {
	if p.KeyPair != "" && ssh.SSHKeyPath == "" {
		return "", fmt.Errorf("[%s] calling preflight error: --ssh-key-path must set with --key-pair %s", p.GetProviderName(), p.KeyPair)
	}
	pk, err := putil.CreateKeyPair(ssh, p.GetProviderName(), p.ContextName, p.KeyPair)
	return string(pk), err
}

func (p *Alibaba) generateDefaultVPC() error {
	p.Logger.Infof("[%s] generate default vpc %s in region %s", p.GetProviderName(), vpcName, p.Region)
	request := vpc.CreateCreateVpcRequest()
	request.Scheme = "https"
	request.RegionId = p.Region
	request.CidrBlock = vpcCidrBlock
	request.VpcName = vpcName
	request.Description = "default vpc created by autok3s"

	response, err := p.v.CreateVpc(request)
	if err != nil {
		return fmt.Errorf("[%s] error create default vpc in region %s, got error: %v", p.GetProviderName(), p.Region, err)
	}

	p.Vpc = response.VpcId

	args := vpc.CreateTagResourcesRequest()
	args.Scheme = "https"
	args.ResourceType = "vpc"
	args.ResourceId = &[]string{response.VpcId}
	args.Tag = &[]vpc.TagResourcesTag{
		{
			Key:   "autok3s",
			Value: "true",
		},
	}

	_, err = p.v.TagResources(args)
	if err != nil {
		return fmt.Errorf("[%s] error tag default vpc %s, got error: %v", p.GetProviderName(), response.VpcId, err)
	}

	p.Logger.Infof("[%s] waiting for vpc %s available", p.GetProviderName(), p.Vpc)
	// wait for vpc available.
	err = utils.WaitFor(p.isVPCAvailable)

	return err
}

func (p *Alibaba) isVPCAvailable() (bool, error) {
	request := vpc.CreateDescribeVpcAttributeRequest()
	request.Scheme = "https"
	request.VpcId = p.Vpc
	request.RegionId = p.Region
	resp, err := p.v.DescribeVpcAttribute(request)
	if err != nil {
		return false, err
	}
	if resp.Status == vpcStatusAvailable {
		return true, nil
	}
	return false, nil
}

func (p *Alibaba) isVSwitchAvailable() (bool, error) {
	request := vpc.CreateDescribeVSwitchAttributesRequest()
	request.Scheme = "https"
	request.VSwitchId = p.VSwitch

	response, err := p.v.DescribeVSwitchAttributes(request)
	if err != nil {
		return false, err
	}
	if response.Status == vpcStatusAvailable {
		return true, nil
	}
	return false, nil
}

func (p *Alibaba) generateDefaultVSwitch(cidr string) error {
	vsName := fmt.Sprintf("%s-%s", vSwitchName, p.Zone)
	p.Logger.Infof("[%s] generate default vswitch %s for vpc %s in region %s, zone %s", p.GetProviderName(), vsName, vpcName, p.Region, p.Zone)
	request := vpc.CreateCreateVSwitchRequest()
	request.Scheme = "https"

	request.RegionId = p.Region
	request.ZoneId = p.Zone
	if cidr == "" {
		cidr = vSwitchCidrBlock
	}
	request.CidrBlock = cidr
	request.VpcId = p.Vpc
	request.VSwitchName = vsName
	request.Description = "default vswitch created by autok3s"

	response, err := p.v.CreateVSwitch(request)
	if err != nil {
		return fmt.Errorf("[%s] error create default vswitch for vpc %s in region %s, zone %s, got error: %v", p.GetProviderName(), p.Vpc, p.Region, p.Zone, err)
	}
	p.VSwitch = response.VSwitchId

	args := vpc.CreateTagResourcesRequest()
	args.Scheme = "https"

	args.ResourceType = "vswitch"
	args.ResourceId = &[]string{response.VSwitchId}
	args.Tag = &[]vpc.TagResourcesTag{
		{
			Key:   "autok3s",
			Value: "true",
		},
	}

	_, err = p.v.TagResources(args)
	if err != nil {
		return fmt.Errorf("[%s] error tag default vswitch %s, got error: %v", p.GetProviderName(), p.VSwitch, err)
	}

	p.Logger.Infof("[%s] waiting for vswitch %s available", p.GetProviderName(), p.VSwitch)
	// wait for vswitch available.
	err = utils.WaitFor(p.isVSwitchAvailable)

	return err
}

func (p *Alibaba) configNetwork() error {
	// find default vpc and vswitch.
	request := vpc.CreateDescribeVpcsRequest()
	request.Scheme = "https"
	request.RegionId = p.Region
	request.VpcName = vpcName

	response, err := p.v.DescribeVpcs(request)
	if err != nil {
		return err
	}

	if response != nil && response.TotalCount > 0 {
		//get default vswitch.
		defaultVPC := response.Vpcs.Vpc[0]
		p.Vpc = defaultVPC.VpcId
		err = utils.WaitFor(p.isVPCAvailable)
		if err != nil {
			return err
		}
		req := vpc.CreateDescribeVSwitchesRequest()
		req.Scheme = "https"
		req.RegionId = p.Region
		//req.ZoneId = p.Zone
		req.VpcId = defaultVPC.VpcId
		resp, err := p.v.DescribeVSwitches(req)
		if err != nil {
			return err
		}
		randCidr := fmt.Sprintf("10.%d.0.0/20", utils.GenerateRand())
		if resp != nil && resp.TotalCount > 0 {
			vswitchList := resp.VSwitches.VSwitch
			for _, vswitch := range vswitchList {
				// check zone and name for default vswitch and regenerate default cidr block if random cidr is exists.
				if vswitch.ZoneId == p.Zone && (vswitch.VSwitchName == vSwitchName || vswitch.VSwitchName == fmt.Sprintf("%s-%s", vSwitchName, p.Zone)) {
					p.VSwitch = vswitch.VSwitchId
					break
				} else if vswitch.CidrBlock == randCidr {
					randCidr = fmt.Sprintf("10.%d.0.0/20", utils.GenerateRand())
				}
			}
		}

		if p.VSwitch == "" {
			err = p.generateDefaultVSwitch(randCidr)
			if err != nil {
				return err
			}
		} else {
			return utils.WaitFor(p.isVSwitchAvailable)
		}
	} else {
		err = p.generateDefaultVPC()
		if err != nil {
			return err
		}
		err = p.generateDefaultVSwitch("")
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Alibaba) configSecurityGroup() error {
	p.Logger.Infof("[%s] config default security group for %s in region %s", p.GetProviderName(), p.Vpc, p.Region)

	if p.Vpc == "" {
		// if user didn't set security group, get vpc from vswitch to config default security group.
		vpcID, _, err := p.getVSwitchCIDR()
		if err != nil {
			return err
		}
		p.Vpc = vpcID
	}

	var securityGroup *ecs.DescribeSecurityGroupAttributeResponse

	request := ecs.CreateDescribeSecurityGroupsRequest()
	request.Scheme = "https"
	request.VpcId = p.Vpc
	request.RegionId = p.Region
	request.SecurityGroupName = defaultSecurityGroupName
	request.Tag = &[]ecs.DescribeSecurityGroupsTag{
		{
			Key:   "autok3s",
			Value: "true",
		},
	}
	response, err := p.c.DescribeSecurityGroups(request)
	if err != nil {
		return err
	}
	if response.TotalCount > 0 {
		securityGroup, _ = p.getSecurityGroup(response.SecurityGroups.SecurityGroup[0].SecurityGroupId)
	}

	if securityGroup == nil {
		// create default security group.
		p.Logger.Infof("[%s] create default security group %s for %s in region %s", p.GetProviderName(), defaultSecurityGroupName, p.Vpc, p.Region)
		req := ecs.CreateCreateSecurityGroupRequest()
		req.Scheme = "https"
		req.RegionId = p.Region
		req.SecurityGroupName = defaultSecurityGroupName
		req.VpcId = p.Vpc
		req.Description = "default security group generated by autok3s"
		req.Tag = &[]ecs.CreateSecurityGroupTag{
			{
				Key:   "autok3s",
				Value: "true",
			},
		}
		resp, err := p.c.CreateSecurityGroup(req)
		if err != nil {
			return fmt.Errorf("[%s] create default security group %s for %s in region %s error: %v", p.GetProviderName(), defaultSecurityGroupName, p.Vpc, p.Region, err)
		}
		securityGroupID := resp.SecurityGroupId
		p.Logger.Infof("[%s] waiting for security group %s available", p.GetProviderName(), securityGroupID)
		err = utils.WaitFor(func() (bool, error) {
			s, err := p.getSecurityGroup(securityGroupID)
			if s != nil && err == nil {
				return true, nil
			}
			return false, err
		})
		if err != nil {
			return err
		}
		securityGroup, err = p.getSecurityGroup(securityGroupID)
		if err != nil {
			return err
		}
	}

	p.SecurityGroup = securityGroup.SecurityGroupId
	permissionList := p.configDefaultSecurityPermissions(securityGroup)
	for _, perm := range permissionList {
		args := ecs.CreateAuthorizeSecurityGroupRequest()
		args.Scheme = "https"
		args.RegionId = p.Region
		args.SecurityGroupId = securityGroup.SecurityGroupId
		args.IpProtocol = perm.IpProtocol
		args.PortRange = perm.PortRange
		args.SourceCidrIp = ipRange
		args.Description = perm.Description
		_, err = p.c.AuthorizeSecurityGroup(args)
		if err != nil {
			p.Logger.Errorf("[%s] Add permission %v to securityGroup %s error: %v", p.GetProviderName(), perm, securityGroup.SecurityGroupId, err)
			continue
		}
	}

	return nil
}

func (p *Alibaba) configDefaultSecurityPermissions(sg *ecs.DescribeSecurityGroupAttributeResponse) []ecs.Permission {
	hasSSHPort := false
	hasAPIServerPort := false
	hasKubeletPort := false
	for _, perm := range sg.Permissions.Permission {
		portRange := strings.Split(perm.PortRange, "/")

		p.Logger.Infof("[%s] get portRange %v for security group %s", p.GetProviderName(), portRange, sg.SecurityGroupId)
		fromPort, _ := strconv.Atoi(portRange[0])
		switch fromPort {
		case 22:
			hasSSHPort = true
		case 6443:
			hasAPIServerPort = true
		case 10250:
			hasKubeletPort = true
		}
	}

	perms := make([]ecs.Permission, 0)

	if !hasSSHPort {
		perms = append(perms, ecs.Permission{
			IpProtocol:  "tcp",
			PortRange:   "22/22",
			Description: "accept for ssh(generated by autok3s)",
		})
	}

	if p.Network == "" || p.Network == "vxlan" {
		// udp 8472 for flannel vxlan.
		perms = append(perms, ecs.Permission{
			IpProtocol:  "udp",
			PortRange:   "8472/8472",
			Description: "accept for k3s vxlan(generated by autok3s)",
		})
	}

	// port 6443 for kubernetes api-server.
	if !hasAPIServerPort {
		perms = append(perms, ecs.Permission{
			IpProtocol:  "tcp",
			PortRange:   "6443/6443",
			Description: "accept for kube api-server(generated by autok3s)",
		})
	}

	// 10250 for kubelet.
	if !hasKubeletPort {
		perms = append(perms, ecs.Permission{
			IpProtocol:  "tcp",
			PortRange:   "10250/10250",
			Description: "accept for kubelet(generated by autok3s)",
		})
	}

	return perms
}

func (p *Alibaba) getSecurityGroup(id string) (*ecs.DescribeSecurityGroupAttributeResponse, error) {
	request := ecs.CreateDescribeSecurityGroupAttributeRequest()
	request.Scheme = "https"
	request.RegionId = p.Region
	request.SecurityGroupId = id
	return p.c.DescribeSecurityGroupAttribute(request)
}

func (p *Alibaba) uploadKeyPair(node types.Node, publicKey string) error {
	dialer, err := dialer.NewSSHDialer(&node, true, p.Logger)
	if err != nil {
		return err
	}

	defer func() {
		_ = dialer.Close()
	}()

	command := fmt.Sprintf("mkdir -p ~/.ssh; echo '%s' > ~/.ssh/authorized_keys", strings.Trim(publicKey, "\n"))
	p.Logger.Debugf("[%s] upload the public key with command: %s", p.GetProviderName(), command)
	output, err := dialer.ExecuteCommands(command)
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}

	p.Logger.Debugf("[%s] upload keypair with output: %s", p.GetProviderName(), output)

	return nil
}
