package alibaba

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/cnrancher/autok3s/pkg/viper"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	k3sVersion               = ""
	k3sChannel               = "stable"
	k3sInstallScript         = "http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh"
	accessKeyID              = "access-key"
	accessKeySecret          = "access-secret"
	imageID                  = "ubuntu_18_04_x64_20G_alibase_20200618.vhd"
	instanceType             = "ecs.c6.large"
	internetMaxBandwidthOut  = "5"
	diskCategory             = "cloud_ssd"
	diskSize                 = "40"
	master                   = "0"
	worker                   = "0"
	ui                       = false
	terway                   = "none"
	terwayMaxPoolSize        = "5"
	cloudControllerManager   = false
	resourceTypeEip          = "EIP"
	eipStatusAvailable       = "Available"
	eipStatusInUse           = "InUse"
	defaultRegion            = "cn-hangzhou"
	vpcCidrBlock             = "10.0.0.0/8"
	vSwitchCidrBlock         = "10.3.0.0/20"
	ipRange                  = "0.0.0.0/0"
	vpcName                  = "autok3s-aliyun-vpc"
	vSwitchName              = "autok3s-aliyun-vswitch"
	defaultZoneID            = "cn-hangzhou-i"
	defaultSecurityGroupName = "autok3s"
	vpcStatusAvailable       = "Available"
	defaultCidr              = "10.42.0.0/16"
	defaultUser              = "root"
)

// ProviderName is the name of this provider.
const ProviderName = "alibaba"

var (
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
		return newProvider(), nil
	})
}

func newProvider() *Alibaba {
	return &Alibaba{
		Metadata: types.Metadata{
			Provider:               ProviderName,
			Master:                 master,
			Worker:                 worker,
			UI:                     ui,
			CloudControllerManager: cloudControllerManager,
			K3sVersion:             k3sVersion,
			K3sChannel:             k3sChannel,
			InstallScript:          k3sInstallScript,
			Cluster:                false,
		},
		Options: alibaba.Options{
			DiskCategory:            diskCategory,
			DiskSize:                diskSize,
			Image:                   imageID,
			Terway:                  alibaba.Terway{Mode: terway, MaxPoolSize: terwayMaxPoolSize},
			Type:                    instanceType,
			InternetMaxBandwidthOut: internetMaxBandwidthOut,
			Region:                  defaultRegion,
			Zone:                    defaultZoneID,
			EIP:                     false,
		},
		Status: types.Status{
			MasterNodes: make([]types.Node, 0),
			WorkerNodes: make([]types.Node, 0),
		},
		m: new(syncmap.Map),
	}
}

func (p *Alibaba) GetProviderName() string {
	return p.Provider
}

func (p *Alibaba) GenerateClusterName() {
	p.Name = fmt.Sprintf("%s.%s.%s", p.Name, p.Region, p.GetProviderName())
}

func (p *Alibaba) CreateK3sCluster(ssh *types.SSH) (err error) {
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
				if p.CloudControllerManager {
					p.logger.Infof("K3s UI URL: https://<using `kubectl get svc -A` get UI address>:8999")
				} else {
					p.logger.Infof("K3s UI URL: https://%s:8999", p.Status.MasterNodes[0].PublicIPAddress[0])
				}
			}
			cluster.SaveClusterState(c, common.StatusRunning)
			// remove creating state file and save running state
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusCreating)))
		}
		logFile.Close()
	}()
	os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusFailed)))

	p.logger = common.NewLogger(common.Debug, logFile)
	p.logger.Infof("[%s] executing create logic...\n", p.GetProviderName())
	p.logger.Infof("[%s] begin to create cluster %s \n", p.GetProviderName(), p.Name)
	if ssh.User == "" {
		ssh.User = defaultUser
	}

	c.Status.Status = common.StatusCreating
	err = cluster.SaveClusterState(c, common.StatusCreating)
	if err != nil {
		return err
	}

	c, err = p.generateInstance(func() error {
		return nil
	}, ssh)
	if err != nil {
		return
	}

	c.Logger = p.logger
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
		if c.CloudControllerManager {
			// deploy additional Alibaba cloud-controller-manager manifests.
			aliCCM := &alibaba.CloudControllerManager{
				Region:       option.Region,
				AccessKey:    option.AccessKey,
				AccessSecret: option.AccessSecret,
			}
			if c.ClusterCIDR == "" {
				c.ClusterCIDR = defaultCidr
			}
			tmpl := fmt.Sprintf(alibabaCCMTmpl, aliCCM.AccessKey, aliCCM.AccessSecret, c.ClusterCIDR, aliCCM.Region)
			extraManifests = append(extraManifests, fmt.Sprintf(deployCCMCommand,
				base64.StdEncoding.EncodeToString([]byte(tmpl)), common.K3sManifestsDir))
		}
		p.logger.Infof("[%s] start deploy Alibaba additional manifests\n", p.GetProviderName())
		if err := cluster.DeployExtraManifest(c, extraManifests); err != nil {
			return err
		}
		p.logger.Infof("[%s] successfully deploy Alibaba additional manifests\n", p.GetProviderName())
	}

	return nil
}

func (p *Alibaba) JoinK3sNode(ssh *types.SSH) (err error) {
	if p.m == nil {
		p.m = new(syncmap.Map)
	}
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
		if err == nil {
			cluster.SaveClusterState(c, common.StatusRunning)
		}
		// remove join state file and save running state
		os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusJoin)))
		logFile.Close()
	}()

	p.logger = common.NewLogger(common.Debug, logFile)
	p.logger.Infof("[%s] executing join logic...\n", p.GetProviderName())
	if ssh.User == "" {
		ssh.User = defaultUser
	}
	c.Status.Status = "upgrading"
	err = cluster.SaveClusterState(c, common.StatusJoin)
	if err != nil {
		return err
	}

	c, err = p.generateInstance(p.joinCheck, ssh)
	if err != nil {
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
		}
		return true
	})

	c.Logger = p.logger
	added.Logger = p.logger
	// join K3s node.
	if err := cluster.JoinK3sNode(c, added); err != nil {
		return err
	}

	p.logger.Infof("[%s] successfully executed join logic\n", p.GetProviderName())
	return nil
}

func (p *Alibaba) Rollback() error {
	logFile, err := common.GetLogFile(p.Name)
	if err != nil {
		return err
	}
	p.logger = common.NewLogger(common.Debug, logFile)
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

	// remove default key-pair folder
	err = os.RemoveAll(common.GetClusterPath(p.Name, p.GetProviderName()))
	if err != nil {
		return fmt.Errorf("[%s] remove cluster store folder (%s) error, msg: %v", p.GetProviderName(), common.GetClusterPath(p.Name, p.GetProviderName()), err)
	}

	p.logger.Infof("[%s] successfully executed rollback logic\n", p.GetProviderName())

	return logFile.Close()
}

func (p *Alibaba) DeleteK3sCluster(f bool) error {
	isConfirmed := true

	if !f {
		isConfirmed = utils.AskForConfirmation(fmt.Sprintf("[%s] are you sure to delete cluster %s", p.GetProviderName(), p.Name))
	}

	if isConfirmed {
		logFile, err := common.GetLogFile(p.Name)
		if err != nil {
			return err
		}
		defer func() {
			logFile.Close()
			// remove log file
			os.Remove(filepath.Join(common.GetLogPath(), p.Name))
			// remove state file
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusRunning)))
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusFailed)))
		}()
		p.logger = common.NewLogger(common.Debug, logFile)
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

func (p *Alibaba) SSHK3sNode(ssh *types.SSH, node string) error {
	p.logger = common.NewLogger(common.Debug, nil)
	p.logger.Infof("[%s] executing ssh logic...\n", p.GetProviderName())

	if err := p.generateClientSDK(); err != nil {
		return err
	}

	instanceList, err := p.syncClusterInstance(ssh)
	if err != nil {
		return err
	}

	ids := make(map[string]string, len(instanceList))
	for _, instance := range instanceList {
		instanceInfo := ""
		if instance.EipAddress.IpAddress != "" {
			instanceInfo = instance.EipAddress.IpAddress
		} else if instance.EipAddress.IpAddress == "" && len(instance.PublicIpAddress.IpAddress) > 0 {
			instanceInfo = instance.PublicIpAddress.IpAddress[0]
		}
		if instanceInfo != "" {
			for _, t := range instance.Tags.Tag {
				if t.TagKey != "master" && t.TagKey != "worker" {
					continue
				}
				if t.TagValue == "true" {
					instanceInfo = fmt.Sprintf("%s (%s)", instanceInfo, t.TagKey)
					break
				}
			}
			if instance.Status != alibaba.StatusRunning {
				instanceInfo = fmt.Sprintf("%s - Unhealthy(instance is %s)", instanceInfo, instance.Status)
			}
			ids[instance.InstanceId] = instanceInfo
		}
	}

	// sync master/worker count
	p.Metadata.Master = strconv.Itoa(len(p.Status.MasterNodes))
	p.Metadata.Worker = strconv.Itoa(len(p.Status.WorkerNodes))
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	err = cluster.SaveState(c)

	if err != nil {
		return fmt.Errorf("[%s] synchronizing .state file error, msg: [%v]", p.GetProviderName(), err)
	}
	if node == "" {
		node = strings.Split(utils.AskForSelectItem(fmt.Sprintf("[%s] choose ssh node to connect", p.GetProviderName()), ids), " (")[0]
	}

	if node == "" {
		return fmt.Errorf("[%s] choose incorrect ssh node", p.GetProviderName())
	}

	// ssh K3s node.
	if err := cluster.SSHK3sNode(node, c, ssh); err != nil {
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
	if err != nil || !response.IsSuccess() {
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

func (p *Alibaba) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	if option, ok := cluster.Options.(alibaba.Options); ok {
		if cluster.CloudControllerManager {
			extraArgs := fmt.Sprintf(" --kubelet-arg=cloud-provider=external --kubelet-arg=provider-id=%s.%s --node-name=%s.%s",
				option.Region, master.InstanceID, option.Region, master.InstanceID)
			return extraArgs
		}
	}
	return ""
}

func (p *Alibaba) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return p.GenerateMasterExtraArgs(cluster, worker)
}

func (p *Alibaba) GetCluster(kubecfg string) *types.ClusterInfo {
	p.logger = common.NewLogger(common.Debug, nil)
	c := &types.ClusterInfo{
		Name:     p.Name,
		Region:   p.Region,
		Zone:     p.Zone,
		Provider: p.GetProviderName(),
	}
	client, err := cluster.GetClusterConfig(p.Name, kubecfg)
	if err != nil {
		p.logger.Errorf("[%s] failed to generate kube client for cluster %s: %v", p.GetProviderName(), p.Name, err)
		c.Status = types.ClusterStatusUnknown
		c.Version = types.ClusterStatusUnknown
		return c
	}
	c.Status = cluster.GetClusterStatus(client)
	if c.Status == types.ClusterStatusRunning {
		c.Version = cluster.GetClusterVersion(client)
	} else {
		c.Version = types.ClusterStatusUnknown
	}
	if p.c == nil {
		if err := p.generateClientSDK(); err != nil {
			p.logger.Errorf("[%s] failed to generate alibaba client sdk for cluster %s: %v", p.GetProviderName(), p.Name, err)
			c.Master = "0"
			c.Worker = "0"
			return c
		}
	}
	instanceList, err := p.describeInstances()
	if err != nil {
		p.logger.Errorf("[%s] failed to get instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
		c.Master = "0"
		c.Worker = "0"
		return c
	}
	masterCount := 0
	workerCount := 0
	for _, ins := range instanceList {
		isMaster := false
		for _, tag := range ins.Tags.Tag {
			if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
				isMaster = true
				masterCount++
				break
			}
		}
		if !isMaster {
			workerCount++
		}
	}
	c.Master = strconv.Itoa(masterCount)
	c.Worker = strconv.Itoa(workerCount)

	return c
}

func (p *Alibaba) DescribeCluster(kubecfg string) *types.ClusterInfo {
	p.logger = common.NewLogger(common.Debug, nil)
	c := &types.ClusterInfo{
		Name:     p.Name,
		Region:   p.Region,
		Zone:     p.Zone,
		Provider: p.GetProviderName(),
	}
	client, err := cluster.GetClusterConfig(p.Name, kubecfg)
	if err != nil {
		p.logger.Errorf("[%s] failed to generate kube client for cluster %s: %v", p.GetProviderName(), p.Name, err)
		c.Status = types.ClusterStatusUnknown
		c.Version = types.ClusterStatusUnknown
		return c
	}
	c.Status = cluster.GetClusterStatus(client)
	if p.c == nil {
		if err := p.generateClientSDK(); err != nil {
			p.logger.Errorf("[%s] failed to generate alibaba client sdk for cluster %s: %v", p.GetProviderName(), p.Name, err)
			c.Master = "0"
			c.Worker = "0"
			return c
		}
	}
	instanceList, err := p.describeInstances()
	if err != nil {
		p.logger.Errorf("[%s] failed to get instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
		c.Master = "0"
		c.Worker = "0"
		return c
	}
	instanceNodes := make([]types.ClusterNode, 0)
	masterCount := 0
	workerCount := 0
	for _, instance := range instanceList {
		n := types.ClusterNode{
			InstanceID:              instance.InstanceId,
			InstanceStatus:          instance.Status,
			InternalIP:              instance.VpcAttributes.PrivateIpAddress.IpAddress,
			ExternalIP:              []string{instance.EipAddress.IpAddress},
			Status:                  types.ClusterStatusUnknown,
			ContainerRuntimeVersion: types.ClusterStatusUnknown,
			Version:                 types.ClusterStatusUnknown,
		}
		isMaster := false
		for _, tag := range instance.Tags.Tag {
			if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
				isMaster = true
				masterCount++
				break
			}
		}
		if !isMaster {
			workerCount++
		}
		instanceNodes = append(instanceNodes, n)
	}
	c.Master = strconv.Itoa(masterCount)
	c.Worker = strconv.Itoa(workerCount)
	c.Nodes = instanceNodes
	if c.Status == types.ClusterStatusRunning {
		c.Version = cluster.GetClusterVersion(client)
		nodes, err := cluster.DescribeClusterNodes(client, instanceNodes)
		if err != nil {
			p.logger.Errorf("[%s] failed to list nodes of cluster %s: %v", p.GetProviderName(), p.Name, err)
			return c
		}
		c.Nodes = nodes
	} else {
		c.Version = types.ClusterStatusUnknown
	}
	return c
}

func (p *Alibaba) GetClusterConfig() (map[string]schemas.Field, error) {
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

func (p *Alibaba) GetProviderOption() (map[string]schemas.Field, error) {
	return utils.ConvertToFields(p.Options)
}

func (p *Alibaba) SetConfig(config []byte) error {
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
	opt := &alibaba.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

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
	request.InstanceType = p.Type
	request.ImageId = p.Image
	request.VSwitchId = p.VSwitch
	request.KeyPairName = p.KeyPair
	request.SystemDiskCategory = p.DiskCategory
	request.SystemDiskSize = p.DiskSize
	request.SecurityGroupId = p.SecurityGroup
	request.Amount = requests.NewInteger(num)
	request.UniqueSuffix = requests.NewBoolean(false)
	// check `--eip` value
	if !p.EIP {
		bandwidth, err := strconv.Atoi(p.InternetMaxBandwidthOut)
		if err != nil {
			p.logger.Warnf("[%s] `--internet-max-bandwidth-out` value %s is invalid, "+
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

	tag := []ecs.RunInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}

	for k, v := range p.Tags {
		tag = append(tag, ecs.RunInstancesTag{Key: k, Value: v})
	}

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
		p.m.Store(id, types.Node{Master: master, RollBack: true, InstanceID: id, InstanceStatus: alibaba.StatusPending})
	}

	return nil
}

func (p *Alibaba) deleteCluster(f bool) error {
	exist, ids, err := p.IsClusterExist()
	if err != nil && !f {
		return fmt.Errorf("[%s] calling deleteCluster error, msg: %v", p.GetProviderName(), err)
	}
	if !exist {
		if !f {
			return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", p.GetProviderName(), p.Name)
		}
		return nil
	}

	p.releaseEipAddresses(false)
	if err == nil && len(ids) > 0 {
		p.logger.Debugf("[%s] cluster %s will be deleted\n", p.GetProviderName(), p.Name)

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
			return fmt.Errorf("[%s] calling deleteInstance error, msg: %v", p.GetProviderName(), err)
		}
	}

	err = cluster.OverwriteCfg(p.Name)

	if err != nil && !f {
		return fmt.Errorf("[%s] synchronizing .cfg file error, msg: %v", p.GetProviderName(), err)
	}

	err = cluster.DeleteState(p.Name, p.Provider)

	if err != nil && !f {
		return fmt.Errorf("[%s] synchronizing .state file error, msg: %v", p.GetProviderName(), err)
	}

	// remove default key-pair folder
	err = os.RemoveAll(common.GetClusterPath(p.Name, p.GetProviderName()))
	if err != nil && !f {
		return fmt.Errorf("[%s] remove cluster store folder (%s) error, msg: %v", p.GetProviderName(), common.GetClusterPath(p.Name, p.GetProviderName()), err)
	}
	// remove log file
	os.Remove(filepath.Join(common.GetLogPath(), p.Name))
	p.logger.Debugf("[%s] successfully deleted cluster %s\n", p.GetProviderName(), p.Name)

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

		wait.ErrWaitTimeout = fmt.Errorf("[%s] calling getInstanceStatus error. region: %s, zone: %s, instanceName: %s, message: not `%s` status",
			p.GetProviderName(), p.Region, p.Zone, ids, aimStatus)

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			response, err := p.c.DescribeInstanceStatus(request)
			if err != nil || !response.IsSuccess() || len(response.InstanceStatuses.InstanceStatus) <= 0 {
				return false, err
			}

			for _, status := range response.InstanceStatuses.InstanceStatus {
				if status.Status == aimStatus {
					if value, ok := p.m.Load(status.InstanceId); ok {
						v := value.(types.Node)
						v.InstanceStatus = aimStatus
						p.m.Store(status.InstanceId, v)
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

	p.logger.Debugf("[%s] instances %s are in `%s` status\n", p.GetProviderName(), ids, aimStatus)

	return nil
}

func (p *Alibaba) assembleInstanceStatus(ssh *types.SSH, uploadKeyPair bool, publicKey string) (*types.Cluster, error) {
	instanceList, err := p.describeInstances()
	if err != nil {
		return nil, err
	}

	for _, status := range instanceList {
		publicIPAddress := status.PublicIpAddress.IpAddress
		eip := []string{}
		if p.EIP {
			publicIPAddress = []string{status.EipAddress.IpAddress}
			eip = []string{status.EipAddress.AllocationId}
		}
		if value, ok := p.m.Load(status.InstanceId); ok {
			v := value.(types.Node)
			// add only nodes that run the current command.
			v.Current = true
			v.InternalIPAddress = status.VpcAttributes.PrivateIpAddress.IpAddress
			v.PublicIPAddress = publicIPAddress
			v.EipAllocationIds = eip
			v.SSH = *ssh
			// check upload keypair
			if uploadKeyPair {
				p.logger.Debugf("[%s] Waiting for upload keypair...\n", p.GetProviderName())
				if err := p.uploadKeyPair(v, publicKey); err != nil {
					return nil, err
				}
				v.SSH.Password = ""
			}
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
		p.m.Store(status.InstanceId, types.Node{
			Master:            master,
			RollBack:          false,
			InstanceID:        status.InstanceId,
			InstanceStatus:    status.Status,
			InternalIPAddress: status.VpcAttributes.PrivateIpAddress.IpAddress,
			EipAllocationIds:  publicIPAddress,
			PublicIPAddress:   eip})
	}

	p.syncNodeStatusWithInstance(ssh)

	return &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}, nil
}

func (p *Alibaba) describeInstances() ([]ecs.Instance, error) {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	pageSize := 20
	request.PageSize = requests.NewInteger(pageSize)
	request.Tag = &[]ecs.DescribeInstancesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}
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
		for _, ins := range response.Instances.Instance {
			instanceList = append(instanceList, ins)
		}
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

func (p *Alibaba) getVpcCIDR() (string, error) {
	vpcID, vSwitchCIDR, err := p.getVSwitchCIDR()
	if err != nil {
		return "", fmt.Errorf("[%s] calling preflight error: vswitch %s cidr not be found",
			p.GetProviderName(), p.VSwitch)
	}

	p.ClusterCIDR = vSwitchCIDR

	request := ecs.CreateDescribeVpcsRequest()
	request.Scheme = "https"
	request.VpcId = vpcID

	response, err := p.c.DescribeVpcs(request)
	if err != nil || !response.IsSuccess() || len(response.Vpcs.Vpc) != 1 {
		return "", fmt.Errorf("[%s] calling describeVpcs error. region: %s, "+"instanceName: %s, msg: [%v]",
			p.GetProviderName(), p.Region, p.Vpc, err)
	}

	return response.Vpcs.Vpc[0].CidrBlock, nil
}

func (p *Alibaba) CreateCheck(ssh *types.SSH) error {
	if p.KeyPair != "" && ssh.SSHKeyPath == "" {
		return fmt.Errorf("[%s] calling preflight error: must set --ssh-key-path with --key-pair %s", p.GetProviderName(), p.KeyPair)
	}

	masterNum, err := strconv.Atoi(p.Master)
	if masterNum < 1 || err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--master` number must >= 1",
			p.GetProviderName())
	}
	if masterNum > 1 && !p.Cluster && p.DataStore == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--cluster` or `--datastore` when `--master` number > 1",
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
		context := strings.Split(p.Name, ".")
		return fmt.Errorf("[%s] calling preflight error: cluster `%s` at region %s is already exist",
			p.GetProviderName(), context[0], p.Region)
	}

	if p.Region != defaultRegion && p.Zone == defaultZoneID && p.VSwitch == "" {
		return fmt.Errorf("[%s] calling preflight error: must set `--zone` in specified region %s to create default vswitch or set exist `--vswitch` in specified region", p.GetProviderName(), p.Region)
	}

	return nil
}

func (p *Alibaba) joinCheck() error {
	if p.Master == "0" && p.Worker == "0" {
		return fmt.Errorf("[%s] calling preflight error: `--master` or `--worker` number must >= 1", p.GetProviderName())
	}
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
		for _, eip := range response.EipAddresses.EipAddress {
			eipList = append(eipList, eip)
		}
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

	// add tags for eips
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

		// no eip need rollback
		if len(releaseEipIds) == 0 {
			p.logger.Debugf("[%s] no eip need execute rollback logic\n", p.GetProviderName())
			return
		}
	}

	// list eips with tags.
	tags := []vpc.ListTagResourcesTag{{Key: "autok3s", Value: "true"}, {Key: "cluster", Value: common.TagClusterPrefix + p.Name}}
	allocationIds, err := p.listVpcTagResources(resourceTypeEip, releaseEipIds, tags)
	if err != nil {
		p.logger.Errorf("[%s] error when query eip address: %v\n", p.GetProviderName(), err)
	}

	if !rollBack {
		for _, allocationID := range allocationIds {
			if err := p.unassociateEipAddress(allocationID); err != nil {
				p.logger.Errorf("[%s] error when unassociating eip address %s: %v\n", p.GetProviderName(), allocationID, err)
			}
		}
	}
	if len(allocationIds) == 0 {
		return
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
	if allocationIds == nil || len(allocationIds) == 0 {
		return fmt.Errorf("[%s] allocationIds can not be empty", p.GetProviderName())
	}

	p.logger.Debugf("[%s] waiting eip(s) to be in `%s` status...\n", p.GetProviderName(), aimStatus)

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		eipList, err := p.describeEipAddresses(allocationIds)
		if err != nil || eipList == nil {
			return false, err
		}

		for _, eip := range eipList {
			p.logger.Debugf("[%s] eip(s) [%s: %s] is in `%s` status\n", p.GetProviderName(), eip.AllocationId, eip.IpAddress, eip.Status)

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
		err error
	)

	if err = p.generateClientSDK(); err != nil {
		return nil, err
	}

	if err = fn(); err != nil {
		return nil, err
	}

	// create key pair
	pk, err := p.createKeyPair(ssh)
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to create key pair: %v", p.GetProviderName(), err)
	}

	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.logger.Debugf("[%s] %d masters and %d workers will be added in region %s\n", p.GetProviderName(), masterNum, workerNum, p.Region)

	if p.VSwitch == "" {
		// get default vpc and vswitch
		err := p.configNetwork()
		if err != nil {
			return nil, err
		}
	}

	if p.SecurityGroup == "" {
		// get default security group
		err := p.configSecurityGroup()
		if err != nil {
			return nil, err
		}
	}

	needUploadKeyPair := false
	if ssh.Password == "" && p.KeyPair == "" {
		needUploadKeyPair = true
		ssh.Password = putil.RandomPassword()
		p.logger.Infof("[%s] launching instance with auto-generated password...", p.GetProviderName())
	}

	if p.Terway.Mode != "none" {
		vpcCIDR, err := p.getVpcCIDR()
		if err != nil {
			return nil, fmt.Errorf("[%s] calling preflight error: vpc %s cidr not be found",
				p.GetProviderName(), p.Vpc)
		}

		p.Options.Terway.CIDR = vpcCIDR
	}

	// run ecs master instances.
	if masterNum > 0 {
		p.logger.Debugf("[%s] prepare for %d of master instances \n", p.GetProviderName(), masterNum)
		if err := p.runInstances(masterNum, true, ssh.Password); err != nil {
			return nil, err
		}
		p.logger.Debugf("[%s] %d of master instances created successfully \n", p.GetProviderName(), masterNum)
	}

	// run ecs worker instances.
	if workerNum > 0 {
		p.logger.Debugf("[%s] prepare for %d of worker instances \n", p.GetProviderName(), workerNum)
		if err := p.runInstances(workerNum, false, ssh.Password); err != nil {
			return nil, err
		}
		p.logger.Debugf("[%s] %d of worker instances created successfully \n", p.GetProviderName(), workerNum)
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

		// allocate eip for worker
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

	// assemble instance status.
	var c *types.Cluster
	if c, err = p.assembleInstanceStatus(ssh, needUploadKeyPair, pk); err != nil {
		return nil, err
	}

	c.Mirror = k3sMirror
	c.DockerMirror = dockerMirror

	if option, ok := c.Options.(alibaba.Options); ok {
		if strings.EqualFold(option.Terway.Mode, "eni") {
			c.Network = "none"
		}
		if c.CloudControllerManager {
			c.MasterExtraArgs += " --disable-cloud-controller --no-deploy servicelb,traefik"
		}
	}

	return c, nil
}

func (p *Alibaba) assignEIPToInstance(num int, master bool) ([]string, error) {
	var e error
	eipIds := make([]string, 0)
	eips, err := p.allocateEipAddresses(num)
	if err != nil {
		return nil, err
	}
	// associate eip with instance
	if eips != nil {
		p.logger.Debugf("[%s] prepare for associating %d eip(s) for instance(s)\n", p.GetProviderName(), num)

		p.m.Range(func(key, value interface{}) bool {
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
				p.m.Store(v.InstanceID, v)
			}
			return true
		})
		p.logger.Debugf("[%s] associated %d eip(s) for instance(s) successfully\n", p.GetProviderName(), num)
	}

	return eipIds, nil
}

func (p *Alibaba) syncClusterInstance(ssh *types.SSH) ([]ecs.Instance, error) {
	instanceList, err := p.describeInstances()
	if err != nil {
		return nil, err
	}

	for _, instance := range instanceList {
		// sync all instance that belongs to current clusters
		master := false
		for _, tag := range instance.Tags.Tag {
			if strings.EqualFold(tag.TagKey, "master") && strings.EqualFold(tag.TagValue, "true") {
				master = true
				break
			}
		}

		p.m.Store(instance.InstanceId, types.Node{
			Master:            master,
			InstanceID:        instance.InstanceId,
			InstanceStatus:    instance.Status,
			InternalIPAddress: instance.VpcAttributes.PrivateIpAddress.IpAddress,
			PublicIPAddress:   []string{instance.EipAddress.IpAddress},
			EipAllocationIds:  []string{instance.EipAddress.AllocationId},
			SSH:               *ssh,
		})
	}

	p.syncNodeStatusWithInstance(ssh)

	return instanceList, nil
}

func (p *Alibaba) createKeyPair(ssh *types.SSH) (string, error) {
	if p.KeyPair != "" && ssh.SSHKeyPath == "" {
		return "", fmt.Errorf("[%s] calling preflight error: --ssh-key-path must set with --key-pair %s", p.GetProviderName(), p.KeyPair)
	}
	pk, err := putil.CreateKeyPair(ssh, p.GetProviderName(), p.Name, p.KeyPair)
	return string(pk), err
}

func (p *Alibaba) generateDefaultVPC() error {
	p.logger.Debugf("[%s] generate default vpc %s in region %s\n", p.GetProviderName(), vpcName, p.Region)
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

	p.logger.Debugf("[%s] waiting for vpc %s available\n", p.GetProviderName(), p.Vpc)
	// wait for vpc available
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

func (p *Alibaba) generateDefaultVSwitch() error {
	p.logger.Debugf("[%s] generate default vswitch %s for vpc %s in region %s, zone %s\n", p.GetProviderName(), vSwitchName, vpcName, p.Region, p.Zone)
	request := vpc.CreateCreateVSwitchRequest()
	request.Scheme = "https"

	request.RegionId = p.Region
	request.ZoneId = p.Zone
	request.CidrBlock = vSwitchCidrBlock
	request.VpcId = p.Vpc
	request.VSwitchName = vSwitchName
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

	p.logger.Debugf("[%s] waiting for vswitch %s available\n", p.GetProviderName(), p.VSwitch)
	// wait for vswitch available
	err = utils.WaitFor(p.isVSwitchAvailable)

	return err
}

func (p *Alibaba) configNetwork() error {
	// find default vpc and vswitch
	request := vpc.CreateDescribeVpcsRequest()
	request.Scheme = "https"
	request.RegionId = p.Region
	request.VpcName = vpcName

	response, err := p.v.DescribeVpcs(request)
	if err != nil {
		return err
	}

	if response != nil && response.TotalCount > 0 {
		//get default vswitch
		defaultVPC := response.Vpcs.Vpc[0]
		p.Vpc = defaultVPC.VpcId
		err = utils.WaitFor(p.isVPCAvailable)
		if err != nil {
			return err
		}
		req := vpc.CreateDescribeVSwitchesRequest()
		req.Scheme = "https"
		req.RegionId = p.Region
		req.ZoneId = p.Zone
		req.VSwitchName = vSwitchName
		req.VpcId = defaultVPC.VpcId
		resp, err := p.v.DescribeVSwitches(req)
		if err != nil {
			return err
		}
		if resp != nil && resp.TotalCount > 0 {
			vswitchList := resp.VSwitches.VSwitch
			// check zone
			for _, vswitch := range vswitchList {
				if vswitch.ZoneId == p.Zone {
					p.VSwitch = vswitch.VSwitchId
					break
				}
			}
		}

		if p.VSwitch == "" {
			err = p.generateDefaultVSwitch()
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
		err = p.generateDefaultVSwitch()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Alibaba) configSecurityGroup() error {
	p.logger.Debugf("[%s] config default security group for %s in region %s\n", p.GetProviderName(), p.Vpc, p.Region)

	if p.Vpc == "" {
		// if user didn't set security group, get vpc from vswitch to config default security group
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
		// create default security group
		p.logger.Debugf("[%s] create default security group %s for %s in region %s\n", p.GetProviderName(), defaultSecurityGroupName, p.Vpc, p.Region)
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
		p.logger.Debugf("[%s] waiting for security group %s available\n", p.GetProviderName(), securityGroupID)
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
		_, err := p.c.AuthorizeSecurityGroup(args)
		if err != nil {
			p.logger.Errorf("[%s] Add permission %v to securityGroup %s error: %v", p.GetProviderName(), perm, securityGroup.SecurityGroupId, err)
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

		p.logger.Debugf("[%s] get portRange %v for security group %s\n", p.GetProviderName(), portRange, sg.SecurityGroupId)
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
		// udp 8472 for flannel vxlan
		perms = append(perms, ecs.Permission{
			IpProtocol:  "udp",
			PortRange:   "8472/8472",
			Description: "accept for k3s vxlan(generated by autok3s)",
		})
	}

	// port 6443 for kubernetes api-server
	if !hasAPIServerPort {
		perms = append(perms, ecs.Permission{
			IpProtocol:  "tcp",
			PortRange:   "6443/6443",
			Description: "accept for kube api-server(generated by autok3s)",
		})
	}

	// 10250 for kubelet
	if !hasKubeletPort {
		perms = append(perms, ecs.Permission{
			IpProtocol:  "tcp",
			PortRange:   "10250/10250",
			Description: "accept for kubelet(generated by autok3s)",
		})
	}

	if p.UI {
		perms = append(perms, ecs.Permission{
			IpProtocol:  "tcp",
			PortRange:   "8999/8999",
			Description: "accept for dashboard(generated by autok3s)",
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
	dialer, err := hosts.SSHDialer(&hosts.Host{Node: node})
	if err != nil {
		return err
	}
	tunnel, err := dialer.OpenTunnel(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Close()
	}()
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	command := fmt.Sprintf("mkdir -p ~/.ssh; echo '%s' > ~/.ssh/authorized_keys", strings.Trim(publicKey, "\n"))

	p.logger.Debugf("[%s] upload the public key with command: %s\n", p.GetProviderName(), command)

	tunnel.Cmd(command)

	if err := tunnel.SetStdio(&stdout, &stderr).Run(); err != nil || stderr.String() != "" {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	p.logger.Debugf("[%s] upload keypair with output: %s\n", p.GetProviderName(), stdout.String())
	return nil
}

func (p *Alibaba) syncNodeStatusWithInstance(ssh *types.SSH) {
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
			node := nodes[index]
			if ssh != nil {
				if node.SSH.User == "" || node.SSH.Port == "" || (node.SSH.Password == "" && node.SSH.SSHKeyPath == "") {
					node.SSH = *ssh
				}
			}
			node.InstanceStatus = v.InstanceStatus
			nodes[index] = node
		}
		if v.Master {
			p.Status.MasterNodes = nodes
		} else {
			p.Status.WorkerNodes = nodes
		}
		return true
	})
}
