package tencent

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
	"github.com/cnrancher/autok3s/pkg/types/tencent"
	"github.com/cnrancher/autok3s/pkg/utils"

	tencentCommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	tag "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tag/v20180813"
	tke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	k3sInstallScript         = "https://rancher-mirror.rancher.cn/k3s/k3s-install.sh"
	secretID                 = "secret-id"
	secretKey                = "secret-key"
	spotInstanceChargeType   = "SPOTPAID"
	internetChargeType       = "TRAFFIC_POSTPAID_BY_HOUR"
	defaultSecurityGroupName = "autok3s"
	vpcName                  = "autok3s-tencent-vpc"
	subnetName               = "autok3s-tencent-subnet"
	vpcCidrBlock             = "192.168.0.0/16"
	subnetCidrBlock          = "192.168.3.0/24"
	ipRange                  = "0.0.0.0/0"
	defaultUser              = "ubuntu"
)

// providerName is the name of this provider.
const providerName = "tencent"

var (
	k3sMirror        = "INSTALL_K3S_MIRROR=cn"
	deployCCMCommand = "echo \"%s\" | base64 -d | tee \"%s/cloud-controller-manager.yaml\""
)

// Tencent provider tencent struct.
type Tencent struct {
	*cluster.ProviderBase `json:",inline"`
	tencent.Options       `json:",inline"`

	c *cvm.Client
	v *vpc.Client
	t *tag.Client
	r *tke.Client
	// m *sync.Map
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Tencent {
	base := cluster.NewBaseProvider()
	base.Provider = providerName
	base.InstallScript = k3sInstallScript
	tencentProvider := &Tencent{
		ProviderBase: base,
	}
	if opt, ok := common.DefaultTemplates[providerName]; ok {
		tencentProvider.Options = opt.(tencent.Options)
	}
	return tencentProvider
}

// GetProviderName returns provider name.
func (p *Tencent) GetProviderName() string {
	return providerName
}

// GenerateClusterName generates and returns cluster name.
func (p *Tencent) GenerateClusterName() string {
	p.ContextName = fmt.Sprintf("%s.%s.%s", p.Name, p.Region, p.GetProviderName())
	return p.ContextName
}

// GenerateManifest generates manifest deploy command.
func (p *Tencent) GenerateManifest() []string {
	if p.CloudControllerManager {
		// deploy additional Tencent cloud-controller-manager manifests.
		tencentCCM := &tencent.CloudControllerManager{
			Region:                base64.StdEncoding.EncodeToString([]byte(p.Region)),
			VpcID:                 base64.StdEncoding.EncodeToString([]byte(p.VpcID)),
			NetworkRouteTableName: base64.StdEncoding.EncodeToString([]byte(p.NetworkRouteTableName)),
		}
		tencentCCM.SecretID = base64.StdEncoding.EncodeToString([]byte(p.SecretID))
		tencentCCM.SecretKey = base64.StdEncoding.EncodeToString([]byte(p.SecretKey))
		tmpl := fmt.Sprintf(tencentCCMTmpl, tencentCCM.Region, tencentCCM.SecretID, tencentCCM.SecretKey,
			tencentCCM.VpcID, tencentCCM.NetworkRouteTableName, p.ClusterCidr)

		extraManifests := []string{fmt.Sprintf(deployCCMCommand,
			base64.StdEncoding.EncodeToString([]byte(tmpl)), common.K3sManifestsDir)}
		return extraManifests
	}
	return nil
}

// CreateK3sCluster create K3S cluster.
func (p *Tencent) CreateK3sCluster() (err error) {
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	return p.InitCluster(p.Options, p.GenerateManifest, p.generateInstance, nil, p.rollbackInstance)
}

// JoinK3sNode join K3S node.
func (p *Tencent) JoinK3sNode() (err error) {
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}

	return p.JoinNodes(p.generateInstance, func() error { return nil }, false, p.rollbackInstance)
}

func (p *Tencent) rollbackInstance(ids []string) error {
	if len(ids) > 0 {
		if p.PublicIPAssignedEIP {
			eips, err := p.describeAddresses(nil, tencentCommon.StringPtrs(ids))
			if err != nil {
				p.Logger.Errorf("[%s] error when query eip info", p.GetProviderName())
			}
			var (
				eipIds  []string
				taskIds []uint64
			)
			for _, eip := range eips {
				eipIds = append(eipIds, *eip.AddressId)
				if taskID, err := p.disassociateAddress(*eip.AddressId); err != nil {
					p.Logger.Warnf("[%s] disassociate eip [%s] error", p.GetProviderName(), *eip.AddressId)
				} else {
					taskIds = append(taskIds, taskID)
				}
			}
			for _, taskID := range taskIds {
				if err := p.describeVpcTaskResult(taskID); err != nil {
					p.Logger.Warnf("[%s] disassociate eip task [%d] error", p.GetProviderName(), taskID)
				}
			}
			taskID, err := p.releaseAddresses(eipIds)
			if err != nil {
				p.Logger.Warnf("[%s] release eip [%s] error", p.GetProviderName(), eipIds)
			}
			if err := p.describeVpcTaskResult(taskID); err != nil {
				p.Logger.Warnf("[%s] release eip task [%d] error", p.GetProviderName(), taskID)
			}
		}

		// retry 5 times, total 120 seconds.
		backoff := wait.Backoff{
			Duration: 30 * time.Second,
			Factor:   1,
			Steps:    5,
		}

		if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
			if err := p.terminateInstances(ids); err != nil {
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
func (p *Tencent) DeleteK3sCluster(f bool) error {
	return p.DeleteCluster(f, p.deleteInstance)
}

// SSHK3sNode ssh K3s node.
func (p *Tencent) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	return p.Connect(ip, &p.SSH, c, p.getInstanceNodes, p.isInstanceRunning, nil)
}

func (p *Tencent) isInstanceRunning(state string) bool {
	return state == tencent.Running
}

func (p *Tencent) getInstanceNodes() ([]types.Node, error) {
	if err := p.generateClientSDK(); err != nil {
		return nil, err
	}
	instanceList, err := p.describeInstances()
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to list instance for cluster %s, region: %s, zone: %s: %v",
			p.GetProviderName(), p.Name, p.Region, p.Zone, err)
	}
	nodes := make([]types.Node, 0)
	for _, instance := range instanceList {
		instanceID := *instance.InstanceId
		instanceState := *instance.InstanceState
		master := false
		for _, tagPtr := range instance.Tags {
			if strings.EqualFold(*tagPtr.Key, "master") && strings.EqualFold(*tagPtr.Value, "true") {
				master = true
				break
			}
		}
		nodes = append(nodes, types.Node{
			Master:            master,
			InstanceID:        instanceID,
			InstanceStatus:    instanceState,
			InternalIPAddress: tencentCommon.StringValues(instance.PrivateIpAddresses),
			PublicIPAddress:   tencentCommon.StringValues(instance.PublicIpAddresses),
		})
	}
	return nodes, nil
}

// IsClusterExist determine if the cluster exists.
func (p *Tencent) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	if p.c == nil {
		if err := p.generateClientSDK(); err != nil {
			return false, ids, err
		}
	}
	instanceList, err := p.describeInstances()
	if err != nil {
		if te, ok := err.(*errors.TencentCloudSDKError); ok {
			if te.Code == "AuthFailure.InvalidSecretId" || te.Code == "AuthFailure.SecretIdNotFound" {
				return false, ids, fmt.Errorf("[%s] invalid credential: %s", p.GetProviderName(), te.Message)
			}
		}
		return false, ids, err
	}
	if len(instanceList) > 0 {
		for _, resource := range instanceList {
			ids = append(ids, *resource.InstanceId)
		}
		return true, ids, nil
	}
	return false, ids, nil
}

// GenerateMasterExtraArgs generates K3S master extra args.
func (p *Tencent) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	if option, ok := cluster.Options.(tencent.Options); ok {
		if option.CloudControllerManager {
			extraArgs := fmt.Sprintf(" --kubelet-arg=cloud-provider=external --kubelet-arg=node-status-update-frequency=30s --kubelet-arg=provider-id=tencentcloud:///%s/%s --node-name=%s",
				option.Zone, master.InstanceID, master.InternalIPAddress[0])
			return extraArgs
		}
	}
	return ""
}

// GenerateWorkerExtraArgs generates K3S worker extra args.
func (p *Tencent) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return p.GenerateMasterExtraArgs(cluster, worker)
}

// GetCluster returns cluster status.
func (p *Tencent) GetCluster(kubecfg string) *types.ClusterInfo {
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
func (p *Tencent) DescribeCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		Name:     p.Name,
		Region:   p.Region,
		Zone:     p.Zone,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubecfg, c, p.getInstanceNodes)
}

// SetOptions set options.
func (p *Tencent) SetOptions(opt []byte) error {
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	option := &tencent.Options{}
	err := json.Unmarshal(opt, option)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(option).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// SetConfig set cluster config.
func (p *Tencent) SetConfig(config []byte) error {
	c, err := p.SetClusterConfig(config)
	if err != nil {
		return err
	}
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	b, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	opt := &tencent.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

	return nil
}

// GetProviderOptions get provider options.
func (p *Tencent) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &tencent.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

func (p *Tencent) generateClientSDK() error {
	credential := tencentCommon.NewCredential(
		p.SecretID,
		p.SecretKey,
	)
	cpf := profile.NewClientProfile()
	if p.EndpointURL != "" {
		cpf.HttpProfile.Endpoint = p.EndpointURL
	}
	if client, err := cvm.NewClient(credential, p.Region, cpf); err == nil {
		p.c = client
	} else {
		return err
	}

	if vpcClient, err := vpc.NewClient(credential, p.Region, cpf); err == nil {
		p.v = vpcClient
	} else {
		return err
	}

	// region for tag clients is not necessary.
	if tagClient, err := tag.NewClient(credential, p.Region, cpf); err == nil {
		p.t = tagClient
	} else {
		return err
	}

	if tkeClient, err := tke.NewClient(credential, p.Region, cpf); err == nil {
		p.r = tkeClient
	} else {
		return err
	}
	return nil
}

func (p *Tencent) generateInstance(ssh *types.SSH) (*types.Cluster, error) {
	var err error
	if err = p.generateClientSDK(); err != nil {
		return nil, err
	}

	// create key pair.
	pk, err := putil.CreateKeyPair(ssh, p.GetProviderName(), p.ContextName, p.KeypairID)
	if err != nil {
		return nil, fmt.Errorf("[%s] Failed to create key pair: %v", p.GetProviderName(), err)
	}

	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.Logger.Infof("[%s] %d masters and %d workers will be added", p.GetProviderName(), masterNum, workerNum)

	if p.VpcID == "" {
		// config default vpc and subnet.
		err = p.configNetwork()
		if err != nil {
			return nil, err
		}
	}

	if p.SecurityGroupIds == "" {
		// config default security groups.
		err = p.configSecurityGroup()
		if err != nil {
			return nil, err
		}
	}

	needUploadKeyPair := false
	if ssh.SSHPassword == "" && p.KeypairID == "" {
		needUploadKeyPair = true
		ssh.SSHPassword = putil.RandomPassword()
		p.Logger.Infof("[%s] launching instance with auto-generated password...", p.GetProviderName())
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

	if p.Spot {
		p.InstanceChargeType = spotInstanceChargeType
	}

	// run ecs master instances.
	if masterNum > 0 {
		p.Logger.Infof("[%s] %d number of master instances will be created", p.GetProviderName(), masterNum)
		if err := p.runInstances(masterNum, true, ssh.SSHPassword); err != nil {
			return nil, err
		}
		p.Logger.Infof("[%s] %d number of master instances successfully created", p.GetProviderName(), masterNum)
	}

	// run ecs worker instances.
	if workerNum > 0 {
		p.Logger.Infof("[%s] %d number of worker instances will be created", p.GetProviderName(), workerNum)
		if err := p.runInstances(workerNum, false, ssh.SSHPassword); err != nil {
			return nil, err
		}
		p.Logger.Infof("[%s] %d number of worker instances successfully created", p.GetProviderName(), workerNum)
	}

	// wait ecs instances to be running status.
	if err = p.getInstanceStatus(tencent.StatusRunning); err != nil {
		return nil, err
	}

	var eipTaskIds []uint64

	// allocate eip for master.
	if masterNum > 0 && p.PublicIPAssignedEIP {
		taskIDs, err := p.allocateEIPForInstance(masterNum, true)
		if err != nil {
			return nil, err
		}
		eipTaskIds = append(eipTaskIds, taskIDs...)
	}

	// allocate eip for worker.
	if workerNum > 0 && p.PublicIPAssignedEIP {
		taskIDs, err := p.allocateEIPForInstance(workerNum, false)
		if err != nil {
			return nil, err
		}
		eipTaskIds = append(eipTaskIds, taskIDs...)
	}

	// wait eip to be InUse status.
	if p.PublicIPAssignedEIP {
		for _, taskID := range eipTaskIds {
			if err = p.describeVpcTaskResult(taskID); err != nil {
				return nil, err
			}
		}
	}

	// assemble instance status.
	if err = p.assembleInstanceStatus(ssh, needUploadKeyPair, string(pk)); err != nil {
		return nil, err
	}

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	c.ContextName = p.ContextName
	c.Mirror = k3sMirror

	if _, ok := c.Options.(tencent.Options); ok {
		if p.CloudControllerManager {
			c.MasterExtraArgs += " --disable-cloud-controller --disable servicelb,traefik"
		}
	}
	c.SSH = *ssh

	return c, nil
}

func (p *Tencent) deleteInstance(f bool) (string, error) {
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

	taggedResource, err := p.describeResourcesByTags()
	if err != nil {
		p.Logger.Errorf("[%s] error when query tagged eip(s), message: %v", p.GetProviderName(), err)
	}
	var eipIds []string
	if len(taggedResource) > 0 {
		var taskIds []uint64
		for _, resource := range taggedResource {
			if strings.EqualFold(tencent.ServiceTypeEIP, *resource.ServiceType) &&
				strings.EqualFold(tencent.ResourcePrefixEIP, *resource.ResourcePrefix) {
				eipID := *resource.ResourceId
				if eipID != "" {
					taskID, err := p.disassociateAddress(eipID)
					if err != nil {
						p.Logger.Errorf("[%s] error when query task eip disassociate progress, message: %v", p.GetProviderName(), err)
					}
					eipIds = append(eipIds, eipID)
					if taskID != 0 {
						taskIds = append(taskIds, taskID)
					}
				}
			}
		}
		for _, taskID := range taskIds {
			if err := p.describeVpcTaskResult(taskID); err != nil {
				p.Logger.Errorf("[%s] error when query eip disassociate task result, message: %v", p.GetProviderName(), err)
			}
		}
	}
	if len(eipIds) > 0 {
		taskID, err := p.releaseAddresses(eipIds)
		if err != nil {
			p.Logger.Errorf("[%s] failed to release tagged eip, message: %v", p.GetProviderName(), err)
		}
		if err := p.describeVpcTaskResult(taskID); err != nil {
			p.Logger.Errorf("[%s] failed to query release eip task result, message: %v", p.GetProviderName(), err)
		}
	}

	if len(ids) > 0 {
		p.Logger.Infof("[%s] cluster %s will be deleted", p.GetProviderName(), p.Name)

		err := p.terminateInstances(ids)
		if err != nil {
			return "", fmt.Errorf("[%s] calling deleteInstance error, msg: %v", p.GetProviderName(), err)
		}
	}
	// remove default key-pair folder.
	err = os.RemoveAll(common.GetClusterPath(p.ContextName, p.GetProviderName()))
	if err != nil && !f {
		return "", fmt.Errorf("[%s] remove cluster store folder (%s) error, msg: %v", p.GetProviderName(), common.GetClusterPath(p.ContextName, p.GetProviderName()), err)
	}
	return p.ContextName, nil
}

// CreateCheck check create command and flags.
func (p *Tencent) CreateCheck() error {
	if err := p.CheckCreateArgs(p.IsClusterExist); err != nil {
		return err
	}

	if p.UserDataPath != "" {
		_, err := os.Stat(p.UserDataPath)
		if err != nil {
			return err
		}
	}
	if err := p.ValidateRequireSSHPrivateKey(); p.KeypairID != "" && err != nil {
		return fmt.Errorf("[%s] calling preflight error: %s with --key-pair %s", p.GetProviderName(), err.Error(), p.KeypairID)
	}

	if p.CloudControllerManager && p.NetworkRouteTableName == "" {
		return fmt.Errorf("[%s] calling preflight error: must set `--router` if enabled tencent cloud manager",
			p.GetProviderName())
	}

	return nil
}

// JoinCheck check join command and flags.
func (p *Tencent) JoinCheck() error {
	return p.CheckJoinArgs(p.IsClusterExist)
}

func (p *Tencent) assembleInstanceStatus(ssh *types.SSH, uploadKeyPair bool, publicKey string) error {
	instanceList, err := p.describeInstances()
	if err != nil {
		return fmt.Errorf("[%s] failed to list instance for cluster %s, region: %s, zone: %s: %v",
			p.GetProviderName(), p.Name, p.Region, p.Zone, err)
	}

	for _, status := range instanceList {
		InstanceID := *status.InstanceId
		var eip []string
		if p.PublicIPAssignedEIP {
			eipInfos, err := p.describeAddresses(nil, []*string{status.InstanceId})
			if err != nil {
				p.Logger.Errorf("[%s] error when query eip info of instance:[%s]", p.GetProviderName(), *status.InstanceId)
				return err
			}
			for _, eipInfo := range eipInfos {
				eip = append(eip, *eipInfo.AddressId)
			}
		}
		if value, ok := p.M.Load(InstanceID); ok {
			v := value.(types.Node)
			// add only nodes that run the current command.
			v.Current = true
			v.InternalIPAddress = tencentCommon.StringValues(status.PrivateIpAddresses)
			v.PublicIPAddress = tencentCommon.StringValues(status.PublicIpAddresses)
			v.LocalHostname = ""
			v.EipAllocationIds = eip

			v.SSH = *ssh
			// check upload keypair.
			if uploadKeyPair {
				p.Logger.Infof("[%s] waiting for upload keypair...", p.GetProviderName())
				if err := p.uploadKeyPair(v, publicKey); err != nil {
					return err
				}
				v.SSH.SSHPassword = ""
				ssh.SSHPassword = ""
			}
			p.M.Store(InstanceID, v)
			continue
		}

		master := false
		for _, tagPtr := range status.Tags {
			if strings.EqualFold(*tagPtr.Key, "master") && strings.EqualFold(*tagPtr.Value, "true") {
				master = true
				break
			}
		}
		p.M.Store(InstanceID, types.Node{
			Master:            master,
			RollBack:          false,
			InstanceID:        InstanceID,
			InstanceStatus:    tencent.StatusRunning,
			InternalIPAddress: tencentCommon.StringValues(status.PrivateIpAddresses),
			EipAllocationIds:  eip,
			PublicIPAddress:   tencentCommon.StringValues(status.PublicIpAddresses)})

	}
	return nil
}

func (p *Tencent) runInstances(num int, master bool, password string) error {
	request := cvm.NewRunInstancesRequest()

	diskSize, _ := strconv.ParseInt(p.SystemDiskSize, 10, 64)
	bandwidth, _ := strconv.ParseInt(p.InternetMaxBandwidthOut, 10, 64)

	request.UserData = tencentCommon.StringPtr(p.UserDataContent)
	request.InstanceCount = tencentCommon.Int64Ptr(int64(num))
	request.ImageId = tencentCommon.StringPtr(p.ImageID)
	request.InstanceType = tencentCommon.StringPtr(p.InstanceType)
	request.Placement = &cvm.Placement{
		Zone: tencentCommon.StringPtr(p.Zone),
	}
	request.InstanceChargeType = tencentCommon.StringPtr(p.InstanceChargeType)
	request.SecurityGroupIds = tencentCommon.StringPtrs(strings.Split(p.SecurityGroupIds, ","))
	request.VirtualPrivateCloud = &cvm.VirtualPrivateCloud{
		SubnetId: tencentCommon.StringPtr(p.SubnetID),
		VpcId:    tencentCommon.StringPtr(p.VpcID),
	}
	request.SystemDisk = &cvm.SystemDisk{
		DiskType: tencentCommon.StringPtr(p.SystemDiskType),
		DiskSize: tencentCommon.Int64Ptr(diskSize),
	}
	loginSettings := &cvm.LoginSettings{}
	if password != "" {
		loginSettings.Password = tencentCommon.StringPtr(password)
	}
	if p.KeypairID != "" {
		// only support bind one though it's array.
		loginSettings.KeyIds = tencentCommon.StringPtrs([]string{p.KeypairID})
	}
	request.LoginSettings = loginSettings
	request.InternetAccessible = &cvm.InternetAccessible{
		InternetChargeType:      tencentCommon.StringPtr(internetChargeType),
		InternetMaxBandwidthOut: tencentCommon.Int64Ptr(bandwidth),
		PublicIpAssigned:        tencentCommon.BoolPtr(!p.PublicIPAssignedEIP),
	}

	// set instance tags.
	tags := []*cvm.Tag{
		{Key: tencentCommon.StringPtr("autok3s"), Value: tencentCommon.StringPtr("true")},
		{Key: tencentCommon.StringPtr("cluster"), Value: tencentCommon.StringPtr(common.TagClusterPrefix + p.ContextName)},
	}

	for _, v := range p.Tags {
		ss := strings.Split(v, "=")
		if len(ss) != 2 {
			return fmt.Errorf("tags %s invalid", v)
		}
		tags = append(tags, &cvm.Tag{Key: tencentCommon.StringPtr(ss[0]), Value: tencentCommon.StringPtr(ss[1])})
	}

	if master {
		request.InstanceName = tencentCommon.StringPtr(fmt.Sprintf(common.MasterInstanceName, p.ContextName))
		tags = append(tags, &cvm.Tag{Key: tencentCommon.StringPtr("master"), Value: tencentCommon.StringPtr("true")})
	} else {
		request.InstanceName = tencentCommon.StringPtr(fmt.Sprintf(common.WorkerInstanceName, p.ContextName))
		tags = append(tags, &cvm.Tag{Key: tencentCommon.StringPtr("worker"), Value: tencentCommon.StringPtr("true")})
	}
	request.TagSpecification = []*cvm.TagSpecification{{ResourceType: tencentCommon.StringPtr("instance"), Tags: tags}}

	response, err := p.c.RunInstances(request)
	if err != nil || len(response.Response.InstanceIdSet) != num {
		return fmt.Errorf("[%s] calling runInstances error. region: %s, zone: %s, "+"instanceName: %s, msg: [%v]",
			p.GetProviderName(), p.Region, p.Zone, *request.InstanceName, err)
	}
	for _, id := range response.Response.InstanceIdSet {
		p.M.Store(*id, types.Node{Master: master, RollBack: true, InstanceID: *id, InstanceStatus: tencent.StatusPending})
	}

	return nil
}

func (p *Tencent) describeInstances() ([]*cvm.Instance, error) {
	request := cvm.NewDescribeInstancesRequest()

	limit := int64(20)
	request.Limit = tencentCommon.Int64Ptr(limit)
	// If there are multiple Filters, between the Filters is a logical AND (AND).
	// If there are multiple Values in the same Filter, between Values under the same Filter is a logical OR (OR).
	request.Filters = []*cvm.Filter{
		{Name: tencentCommon.StringPtr("tag:autok3s"), Values: tencentCommon.StringPtrs([]string{"true"})},
		{Name: tencentCommon.StringPtr("tag:cluster"), Values: tencentCommon.StringPtrs([]string{common.TagClusterPrefix + p.ContextName})},
	}
	offset := int64(0)
	index := int64(0)
	instanceList := make([]*cvm.Instance, 0)
	for {
		response, err := p.c.DescribeInstances(request)
		if err != nil {
			return nil, err
		}
		if response.Response == nil || response.Response.InstanceSet == nil || len(response.Response.InstanceSet) == 0 {
			break
		}
		total := *response.Response.TotalCount
		instanceList = append(instanceList, response.Response.InstanceSet...)
		offset = limit*index + limit
		index = index + 1
		if offset >= total {
			break
		}
		request.Offset = tencentCommon.Int64Ptr(offset)
	}

	return instanceList, nil
}

func (p *Tencent) terminateInstances(instanceIds []string) error {
	request := cvm.NewTerminateInstancesRequest()

	request.InstanceIds = tencentCommon.StringPtrs(instanceIds)

	_, err := p.c.TerminateInstances(request)

	if err != nil {
		return fmt.Errorf("[%s] calling deleteInstance error, msg: %v", p.GetProviderName(), err)
	}

	return nil
}

func (p *Tencent) allocateAddresses(num int) ([]*string, uint64, error) {
	request := vpc.NewAllocateAddressesRequest()

	request.AddressCount = tencentCommon.Int64Ptr(int64(num))
	request.InternetChargeType = tencentCommon.StringPtr(internetChargeType)
	internetMaxBandwidthOut, _ := strconv.ParseInt(p.InternetMaxBandwidthOut, 10, 64)
	request.InternetMaxBandwidthOut = tencentCommon.Int64Ptr(internetMaxBandwidthOut)
	request.Tags = []*vpc.Tag{
		{Key: tencentCommon.StringPtr("autok3s"), Value: tencentCommon.StringPtr("true")},
		{Key: tencentCommon.StringPtr("cluster"), Value: tencentCommon.StringPtr(common.TagClusterPrefix + p.ContextName)},
	}
	response, err := p.v.AllocateAddresses(request)
	if err != nil {
		return nil, 0, fmt.Errorf("[%s] calling allocateAddresses error, msg: %v", p.GetProviderName(), err)
	}
	taskID, err := strconv.ParseUint(*response.Response.TaskId, 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("[%s] error when convert taskID: %s", p.GetProviderName(), *response.Response.TaskId)
	}
	return response.Response.AddressSet, taskID, nil
}

func (p *Tencent) releaseAddresses(addressIds []string) (uint64, error) {
	request := vpc.NewReleaseAddressesRequest()

	request.AddressIds = tencentCommon.StringPtrs(addressIds)

	response, err := p.v.ReleaseAddresses(request)

	if err != nil {
		return 0, fmt.Errorf("[%s] calling releaseAddresses error, msg: %v", p.GetProviderName(), err)
	}
	taskID, err := strconv.ParseUint(*response.Response.TaskId, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("[%s] error when convert taskID: %s", p.GetProviderName(), *response.Response.TaskId)
	}
	return taskID, nil
}

func (p *Tencent) describeAddresses(addressIds, instanceIds []*string) ([]*vpc.Address, error) {
	request := vpc.NewDescribeAddressesRequest()

	if len(instanceIds) <= 0 {
		request.AddressIds = addressIds
	} else {
		filters := []*vpc.Filter{
			{Name: tencentCommon.StringPtr("instance-id"), Values: instanceIds},
		}
		if len(addressIds) > 0 {
			filters = append(filters, &vpc.Filter{Name: tencentCommon.StringPtr("address-id"), Values: addressIds})
		}
		request.Filters = filters
	}

	response, err := p.v.DescribeAddresses(request)

	if err != nil {
		return nil, fmt.Errorf("[%s] calling describeAddresses error, msg: %v", p.GetProviderName(), err)
	}

	return response.Response.AddressSet, nil
}

func (p *Tencent) associateAddress(addressID, instanceID string) (uint64, error) {
	request := vpc.NewAssociateAddressRequest()

	request.AddressId = tencentCommon.StringPtr(addressID)
	request.InstanceId = tencentCommon.StringPtr(instanceID)

	response, err := p.v.AssociateAddress(request)

	if err != nil {
		return 0, fmt.Errorf("[%s] calling associateAddress error, msg: %v", p.GetProviderName(), err)
	}
	taskID, err := strconv.ParseUint(*response.Response.TaskId, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("[%s] error when convert taskID: %s", p.GetProviderName(), *response.Response.TaskId)
	}
	return taskID, nil
}

func (p *Tencent) disassociateAddress(addressID string) (uint64, error) {
	request := vpc.NewDisassociateAddressRequest()

	request.AddressId = tencentCommon.StringPtr(addressID)

	response, err := p.v.DisassociateAddress(request)

	if err != nil {
		return 0, fmt.Errorf("[%s] calling associateAddress error, msg: %v", p.GetProviderName(), err)
	}
	if response.Response.TaskId == nil {
		return 0, nil
	}
	taskID, err := strconv.ParseUint(*response.Response.TaskId, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("[%s] error when convert taskID: %s", p.GetProviderName(), *response.Response.TaskId)
	}
	return taskID, nil
}

func (p *Tencent) describeVpcTaskResult(taskID uint64) error {
	request := vpc.NewDescribeTaskResultRequest()
	request.TaskId = tencentCommon.Uint64Ptr(taskID)

	return wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		response, err := p.v.DescribeTaskResult(request)
		if err != nil {
			return false, nil
		}

		switch strings.ToUpper(*response.Response.Result) {
		case tencent.Running:
			return false, nil
		case tencent.Success:
			return true, nil
		case tencent.Failed:
			return true, fmt.Errorf("[%s] task failed %d", p.GetProviderName(), taskID)
		}

		return true, nil
	})
}

func (p *Tencent) getInstanceStatus(aimStatus string) error {
	ids := make([]string, 0)
	p.M.Range(func(key, value interface{}) bool {
		ids = append(ids, key.(string))
		return true
	})

	if len(ids) > 0 {
		p.Logger.Infof("[%s] waiting for the instances %s to be in `%s` status...", p.GetProviderName(), ids, aimStatus)
		request := cvm.NewDescribeInstancesStatusRequest()
		request.InstanceIds = tencentCommon.StringPtrs(ids)

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			response, err := p.c.DescribeInstancesStatus(request)
			if err != nil || len(response.Response.InstanceStatusSet) <= 0 {
				return false, nil
			}

			for _, status := range response.Response.InstanceStatusSet {
				if *status.InstanceState == aimStatus {
					instanceID := *status.InstanceId
					if value, ok := p.M.Load(instanceID); ok {
						v := value.(types.Node)
						v.InstanceStatus = aimStatus
						p.M.Store(instanceID, v)
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

	p.Logger.Infof("[%s] instances %s are in `%s` status", p.GetProviderName(), ids, aimStatus)

	return nil
}

func (p *Tencent) describeResourcesByTags() ([]*tag.ResourceTag, error) {
	request := tag.NewDescribeResourcesByTagsRequest()

	request.TagFilters = []*tag.TagFilter{
		{TagKey: tencentCommon.StringPtr("autok3s"), TagValue: tencentCommon.StringPtrs([]string{"true"})},
		{TagKey: tencentCommon.StringPtr("cluster"), TagValue: tencentCommon.StringPtrs([]string{common.TagClusterPrefix + p.ContextName})},
	}

	response, err := p.t.DescribeResourcesByTags(request)
	if err != nil {
		return nil, err
	}
	return response.Response.Rows, err
}

func (p *Tencent) configNetwork() error {
	// find default vpc and subnet.
	request := vpc.NewDescribeVpcsRequest()

	request.Filters = []*vpc.Filter{
		{
			Values: tencentCommon.StringPtrs([]string{vpcName}),
			Name:   tencentCommon.StringPtr("vpc-name"),
		},
		{
			Name:   tencentCommon.StringPtr("tag:autok3s"),
			Values: tencentCommon.StringPtrs([]string{"true"}),
		},
	}
	response, err := p.v.DescribeVpcs(request)
	if err != nil {
		return err
	}

	if response != nil && response.Response != nil && len(response.Response.VpcSet) > 0 {
		p.Logger.Infof("[%s] find existed default vpc %s for autok3s", p.GetProviderName(), vpcName)
		defaultVPC := response.Response.VpcSet[0]
		p.VpcID = *defaultVPC.VpcId
		// find default subnet.
		args := vpc.NewDescribeSubnetsRequest()

		args.Filters = []*vpc.Filter{
			{
				Name:   tencentCommon.StringPtr("tag:autok3s"),
				Values: tencentCommon.StringPtrs([]string{"true"}),
			},
		}

		resp, err := p.v.DescribeSubnets(args)
		if err != nil {
			return err
		}

		cidr := fmt.Sprintf("192.168.%d.0/24", utils.GenerateRand())
		if resp != nil && resp.Response != nil && len(resp.Response.SubnetSet) > 0 {
			p.Logger.Infof("[%s] find existed default subnet for vpc %s", p.GetProviderName(), vpcName)
			for _, subnet := range resp.Response.SubnetSet {
				if *subnet.Zone == p.Zone && (*subnet.SubnetName == subnetName || *subnet.SubnetName == fmt.Sprintf("%s-%s", subnetName, p.Zone)) {
					p.SubnetID = *subnet.SubnetId
					break
				} else if *subnet.CidrBlock == cidr {
					cidr = fmt.Sprintf("192.168.%d.0/24", utils.GenerateRand())
				}
			}
		}
		if p.SubnetID == "" {
			return p.generateDefaultSubnet(cidr)
		}

	} else {
		err := p.generateDefaultVPC()
		if err != nil {
			return err
		}
		err = p.generateDefaultSubnet("")
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Tencent) generateDefaultVPC() error {
	p.Logger.Infof("[%s] generate default vpc %s in region %s", p.GetProviderName(), vpcName, p.Region)
	request := vpc.NewCreateVpcRequest()
	request.VpcName = tencentCommon.StringPtr(vpcName)
	request.CidrBlock = tencentCommon.StringPtr(vpcCidrBlock)
	request.Tags = []*vpc.Tag{
		{
			Key:   tencentCommon.StringPtr("autok3s"),
			Value: tencentCommon.StringPtr("true"),
		},
	}
	response, err := p.v.CreateVpc(request)
	if err != nil {
		return fmt.Errorf("[%s] fail to create default vpc %s in region %s: %v", p.GetProviderName(), vpcName, p.Region, err)
	}

	p.VpcID = *response.Response.Vpc.VpcId
	p.Logger.Infof("[%s] generate default vpc %s in region %s successfully", p.GetProviderName(), vpcName, p.Region)

	return err
}

func (p *Tencent) generateDefaultSubnet(cidr string) error {
	vsName := fmt.Sprintf("%s-%s", subnetName, p.Zone)
	p.Logger.Infof("[%s] generate default subnet %s for vpc %s in region %s", p.GetProviderName(), vsName, vpcName, p.Region)
	request := vpc.NewCreateSubnetRequest()

	request.Tags = []*vpc.Tag{
		{
			Key:   tencentCommon.StringPtr("autok3s"),
			Value: tencentCommon.StringPtr("true"),
		},
	}
	request.VpcId = tencentCommon.StringPtr(p.VpcID)
	request.SubnetName = tencentCommon.StringPtr(vsName)
	request.Zone = tencentCommon.StringPtr(p.Zone)
	if cidr == "" {
		cidr = subnetCidrBlock
	}
	request.CidrBlock = tencentCommon.StringPtr(cidr)

	response, err := p.v.CreateSubnet(request)
	if err != nil {
		return fmt.Errorf("[%s] fail to create default subnet for vpc %s in region %s, zone %s: %v", p.GetProviderName(), p.VpcID, p.Region, p.Zone, err)
	}
	p.SubnetID = *response.Response.Subnet.SubnetId
	p.Logger.Infof("[%s] generate default subnet %s for vpc %s in region %s successfully", p.GetProviderName(), subnetName, vpcName, p.Region)
	return nil
}

func (p *Tencent) configSecurityGroup() error {
	p.Logger.Infof("[%s] check default security group %s in region %s", p.GetProviderName(), defaultSecurityGroupName, p.Region)
	// find default security group.
	request := vpc.NewDescribeSecurityGroupsRequest()

	request.Filters = []*vpc.Filter{
		{
			Values: tencentCommon.StringPtrs([]string{"true"}),
			Name:   tencentCommon.StringPtr("tag:autok3s"),
		},
		{
			Values: tencentCommon.StringPtrs([]string{defaultSecurityGroupName}),
			Name:   tencentCommon.StringPtr("security-group-name"),
		},
	}
	response, err := p.v.DescribeSecurityGroups(request)
	if err != nil {
		return err
	}

	var securityGroupID string
	if response != nil && response.Response != nil && len(response.Response.SecurityGroupSet) > 0 {
		securityGroupID = *response.Response.SecurityGroupSet[0].SecurityGroupId
		p.SecurityGroupIds = securityGroupID
	}

	if securityGroupID == "" {
		// create default security group.
		p.Logger.Infof("[%s] create default security group %s in region %s", p.GetProviderName(), defaultSecurityGroupName, p.Region)
		err = p.generateDefaultSecurityGroup()
		if err != nil {
			return fmt.Errorf("[%s] fail to create default security group %s: %v", p.GetProviderName(), defaultSecurityGroupName, err)
		}
	}
	err = p.configDefaultSecurityPermission()

	return err
}

func (p *Tencent) generateDefaultSecurityGroup() error {
	request := vpc.NewCreateSecurityGroupRequest()

	request.Tags = []*vpc.Tag{
		{
			Key:   tencentCommon.StringPtr("autok3s"),
			Value: tencentCommon.StringPtr("true"),
		},
	}
	request.GroupName = tencentCommon.StringPtr(defaultSecurityGroupName)
	request.GroupDescription = tencentCommon.StringPtr("generated by autok3s")

	response, err := p.v.CreateSecurityGroup(request)
	if err != nil {
		return err
	}

	p.SecurityGroupIds = *response.Response.SecurityGroup.SecurityGroupId

	return nil
}

func (p *Tencent) configDefaultSecurityPermission() error {
	p.Logger.Infof("[%s] check rules of security group %s", p.GetProviderName(), defaultSecurityGroupName)
	// get security group rules.
	request := vpc.NewDescribeSecurityGroupPoliciesRequest()
	request.SecurityGroupId = tencentCommon.StringPtr(p.SecurityGroupIds)
	response, err := p.v.DescribeSecurityGroupPolicies(request)
	if err != nil {
		return err
	}
	// check subnet cidr.
	var cidr string
	if p.SubnetID != "" {
		cidr, err = p.getSubnetCidr()
		if err != nil {
			return err
		}
	} else {
		cidr = subnetCidrBlock
	}
	hasSSHPort := false
	hasAPIServerPort := false
	hasKubeletPort := false
	hasVXlanPort := false
	hasEgress := false
	hasEtcdServerPort := false
	hasEtcdPeerPort := false
	if response != nil && response.Response != nil &&
		response.Response.SecurityGroupPolicySet != nil && response.Response.SecurityGroupPolicySet.Ingress != nil {
		rules := response.Response.SecurityGroupPolicySet.Ingress
		for _, rule := range rules {
			ports := *rule.Port
			portArray := strings.Split(ports, ",")
			for _, p := range portArray {
				fromPort, _ := strconv.Atoi(p)
				switch fromPort {
				case 22:
					hasSSHPort = true
				case 6443:
					hasAPIServerPort = true
				case 10250:
					hasKubeletPort = true
				case 8472:
					hasVXlanPort = true
				case 2379:
					if *rule.CidrBlock == cidr || *rule.CidrBlock == ipRange {
						hasEtcdServerPort = true
					}
				case 2380:
					if *rule.CidrBlock == cidr || *rule.CidrBlock == ipRange {
						hasEtcdPeerPort = true
					}
				}
			}

		}
		eRules := response.Response.SecurityGroupPolicySet.Egress
		if len(eRules) > 0 {
			hasEgress = true
		}
	}

	perms := make([]*vpc.SecurityGroupPolicy, 0)

	if !hasSSHPort {
		perms = append(perms, &vpc.SecurityGroupPolicy{
			Protocol:          tencentCommon.StringPtr("TCP"),
			Port:              tencentCommon.StringPtr("22"),
			CidrBlock:         tencentCommon.StringPtr(ipRange),
			Action:            tencentCommon.StringPtr("ACCEPT"),
			PolicyDescription: tencentCommon.StringPtr("accept for ssh(generated by autok3s)"),
		})
	}

	if (p.Network == "" || p.Network == "vxlan") && !hasVXlanPort {
		// udp 8472 for flannel vxLan.
		perms = append(perms, &vpc.SecurityGroupPolicy{
			Protocol:          tencentCommon.StringPtr("UDP"),
			Port:              tencentCommon.StringPtr("8472"),
			CidrBlock:         tencentCommon.StringPtr(ipRange),
			Action:            tencentCommon.StringPtr("ACCEPT"),
			PolicyDescription: tencentCommon.StringPtr("accept for k3s vxlan(generated by autok3s)"),
		})
	}

	// port 6443 for kubernetes api-server.
	if !hasAPIServerPort {
		perms = append(perms, &vpc.SecurityGroupPolicy{
			Protocol:          tencentCommon.StringPtr("TCP"),
			Port:              tencentCommon.StringPtr("6443"),
			CidrBlock:         tencentCommon.StringPtr(ipRange),
			Action:            tencentCommon.StringPtr("ACCEPT"),
			PolicyDescription: tencentCommon.StringPtr("accept for kube api-server(generated by autok3s)"),
		})
	}

	// 10250 for kubelet.
	if !hasKubeletPort {
		perms = append(perms, &vpc.SecurityGroupPolicy{
			Protocol:          tencentCommon.StringPtr("TCP"),
			Port:              tencentCommon.StringPtr("10250"),
			CidrBlock:         tencentCommon.StringPtr(ipRange),
			Action:            tencentCommon.StringPtr("ACCEPT"),
			PolicyDescription: tencentCommon.StringPtr("accept for kubelet(generated by autok3s)"),
		})
	}

	if !hasEtcdServerPort || !hasEtcdPeerPort {
		perms = append(perms, &vpc.SecurityGroupPolicy{
			Protocol:          tencentCommon.StringPtr("TCP"),
			Port:              tencentCommon.StringPtr("2379"),
			CidrBlock:         tencentCommon.StringPtr(cidr),
			Action:            tencentCommon.StringPtr("ACCEPT"),
			PolicyDescription: tencentCommon.StringPtr("accept for etcd(generated by autok3s)"),
		})
		perms = append(perms, &vpc.SecurityGroupPolicy{
			Protocol:          tencentCommon.StringPtr("TCP"),
			Port:              tencentCommon.StringPtr("2380"),
			CidrBlock:         tencentCommon.StringPtr(cidr),
			Action:            tencentCommon.StringPtr("ACCEPT"),
			PolicyDescription: tencentCommon.StringPtr("accept for etcd(generated by autok3s)"),
		})
	}

	if len(perms) > 0 {
		args := vpc.NewCreateSecurityGroupPoliciesRequest()
		args.SecurityGroupId = tencentCommon.StringPtr(p.SecurityGroupIds)
		args.SecurityGroupPolicySet = &vpc.SecurityGroupPolicySet{
			Ingress: perms,
		}
		_, err = p.v.CreateSecurityGroupPolicies(args)
		if err != nil {
			return err
		}
	}

	// check egress.
	if !hasEgress {
		args := vpc.NewCreateSecurityGroupPoliciesRequest()
		args.SecurityGroupId = tencentCommon.StringPtr(p.SecurityGroupIds)
		args.SecurityGroupPolicySet = &vpc.SecurityGroupPolicySet{
			Egress: []*vpc.SecurityGroupPolicy{
				{
					Protocol:          tencentCommon.StringPtr("ALL"),
					Port:              tencentCommon.StringPtr("all"),
					CidrBlock:         tencentCommon.StringPtr(ipRange),
					Action:            tencentCommon.StringPtr("ACCEPT"),
					PolicyDescription: tencentCommon.StringPtr("allow all egress(generated by autok3s)"),
				},
			},
		}
		_, err = p.v.CreateSecurityGroupPolicies(args)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Tencent) allocateEIPForInstance(num int, master bool) ([]uint64, error) {
	eipIds := make([]uint64, 0)
	eips, taskID, err := p.allocateAddresses(num)
	if err != nil {
		return nil, err
	}
	if err = p.describeVpcTaskResult(taskID); err != nil {
		p.Logger.Errorf("[%s] failed to allocate eip(s) for instance(s): taskId:[%d]", p.GetProviderName(), taskID)
		return nil, err
	}
	eipAddresses, err := p.describeAddresses(eips, nil)
	if err != nil {
		p.Logger.Errorf("[%s] error when query eip info:[%s]", p.GetProviderName(), tencentCommon.StringValues(eips))
		return nil, err
	}

	if eipAddresses != nil {
		p.Logger.Infof("[%s] associating %d eip(s) for instance(s)", p.GetProviderName(), num)
		p.M.Range(func(key, value interface{}) bool {
			v := value.(types.Node)
			if v.Master == master && v.PublicIPAddress == nil {
				v.EipAllocationIds = append(v.EipAllocationIds, *eipAddresses[0].AddressId)
				v.PublicIPAddress = append(v.PublicIPAddress, *eipAddresses[0].AddressIp)
				taskID, err = p.associateAddress(*eipAddresses[0].AddressId, v.InstanceID)
				if err != nil {
					return false
				}
				eipIds = append(eipIds, taskID)
				eipAddresses = eipAddresses[1:]
				p.M.Store(v.InstanceID, v)
			}
			return true
		})
		p.Logger.Infof("[%s] successfully associated %d eip(s) for instance(s)", p.GetProviderName(), num)
	}

	return eipIds, nil
}

func (p *Tencent) uploadKeyPair(node types.Node, publicKey string) error {
	dialer, err := dialer.NewSSHDialer(&node, true, p.Logger)
	if err != nil {
		return err
	}

	defer func() {
		_ = dialer.Close()
	}()

	command := fmt.Sprintf("mkdir -p ~/.ssh; echo '%s' > ~/.ssh/authorized_keys", strings.Trim(publicKey, "\n"))

	p.Logger.Infof("[%s] upload the public key with command: %s", p.GetProviderName(), command)
	output, err := dialer.ExecuteCommands(command)
	if err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}

	p.Logger.Infof("[%s] upload keypair with output: %s", p.GetProviderName(), output)

	return nil
}

func (p *Tencent) getSubnetCidr() (string, error) {
	request := vpc.NewDescribeSubnetsRequest()
	request.SubnetIds = tencentCommon.StringPtrs([]string{p.SubnetID})
	response, err := p.v.DescribeSubnets(request)
	if err != nil {
		return "", err
	}

	if response != nil && response.Response != nil && response.Response.SubnetSet != nil {
		return *response.Response.SubnetSet[0].CidrBlock, nil
	}
	return "", nil
}
