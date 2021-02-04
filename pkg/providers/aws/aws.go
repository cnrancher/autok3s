package aws

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	typesaws "github.com/cnrancher/autok3s/pkg/types/aws"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/cnrancher/autok3s/pkg/viper"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/syncmap"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	providerName = "aws"

	defaultUser              = "ubuntu"
	k3sVersion               = ""
	k3sChannel               = "stable"
	k3sInstallScript         = "https://get.k3s.io"
	ami                      = "ami-00ddb0e5626798373" // Ubuntu Server 18.04 LTS (HVM) x86 64
	instanceType             = "t2.micro"              // 1 vCPU, 1 GiB
	volumeType               = "gp2"
	diskSize                 = "16"
	master                   = "0"
	worker                   = "0"
	ui                       = false
	cloudControllerManager   = false
	defaultRegion            = "us-east-1"
	ipRange                  = "0.0.0.0/0"
	defaultZoneID            = "us-east-1a"
	defaultSecurityGroupName = "autok3s"
	defaultDeviceName        = "/dev/sda1"
	requestSpotInstance      = false
	defaultSpotPrice         = "0.50"
)

const (
	deployCCMCommand = "echo \"%s\" | base64 -d | sudo tee \"%s/cloud-controller-manager.yaml\""
)

type Amazon struct {
	types.Metadata   `json:",inline"`
	typesaws.Options `json:",inline"`
	types.Status     `json:"status"`

	client *ec2.EC2
	m      *sync.Map
	logger *logrus.Logger
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Amazon {
	return &Amazon{
		Metadata: types.Metadata{
			Provider:               providerName,
			Master:                 master,
			Worker:                 worker,
			UI:                     ui,
			CloudControllerManager: cloudControllerManager,
			K3sVersion:             k3sVersion,
			K3sChannel:             k3sChannel,
			InstallScript:          k3sInstallScript,
			Cluster:                false,
		},
		Options: typesaws.Options{
			Region:              defaultRegion,
			Zone:                defaultZoneID,
			VolumeType:          volumeType,
			RootSize:            diskSize,
			InstanceType:        instanceType,
			AMI:                 ami,
			RequestSpotInstance: requestSpotInstance,
		},
		Status: types.Status{
			MasterNodes: make([]types.Node, 0),
			WorkerNodes: make([]types.Node, 0),
		},
		m: new(syncmap.Map),
	}
}

type checkFun func() error

func (p *Amazon) GetProviderName() string {
	return p.Provider
}

func (p *Amazon) GenerateClusterName() {
	p.Name = fmt.Sprintf("%s.%s.%s", p.Name, p.Region, p.GetProviderName())
}

func (p *Amazon) CreateK3sCluster(ssh *types.SSH) (err error) {
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
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusCreating)))
			if c == nil {
				c = &types.Cluster{
					Metadata: p.Metadata,
					Options:  p.Options,
					Status:   p.Status,
				}
				c.Status.Status = common.StatusFailed
				cluster.SaveClusterState(c, common.StatusFailed)
			}
		}
		if err == nil && len(p.Status.MasterNodes) > 0 {
			p.logger.Infof(common.UsageInfo, p.Name)
			if p.UI {
				if p.CloudControllerManager {
					p.logger.Infof("\nK3s UI URL: https://<using `kubectl get svc -A` get UI address>:8999\n")
				} else {
					p.logger.Infof("\nK3s UI URL: https://%s:8999\n", p.Status.MasterNodes[0].PublicIPAddress[0])
				}
			}
			fmt.Println("")
		}
		logFile.Close()
	}()

	p.logger = common.NewLogger(common.Debug, logFile)
	p.logger.Infof("[%s] executing create logic...", p.GetProviderName())
	if ssh.User == "" {
		ssh.User = defaultUser
	}
	if ssh.Port == "" {
		ssh.Port = "22"
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
		return err
	}
	c.Logger = p.logger
	if err = cluster.InitK3sCluster(c); err != nil {
		return err
	}
	p.logger.Infof("[%s] successfully executed create logic\n", p.GetProviderName())

	if c.CloudControllerManager {
		extraManifests := []string{fmt.Sprintf(deployCCMCommand,
			base64.StdEncoding.EncodeToString([]byte(amazonCCMTmpl)), common.K3sManifestsDir)}
		p.logger.Infof("[%s] start deploy aws additional manifests\n", p.GetProviderName())
		if err := cluster.DeployExtraManifest(c, extraManifests); err != nil {
			return err
		}
		p.logger.Infof("[%s] successfully deploy aws additional manifests\n", p.GetProviderName())
	}
	// remove creating state file and save running state
	os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusCreating)))
	return cluster.SaveClusterState(c, common.StatusRunning)
}

func (p *Amazon) JoinK3sNode(ssh *types.SSH) (err error) {
	if p.m == nil {
		p.m = new(syncmap.Map)
	}
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	defer func() {
		if err != nil {
			os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusJoin)))
			if c != nil {
				c.Status.Status = common.StatusFailed
				cluster.SaveClusterState(c, common.StatusFailed)
			}
			return
		}
	}()
	logFile, err := common.GetLogFile(p.Name)
	if err != nil {
		return err
	}
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
	logFile.Close()
	// remove join state file and save running state
	os.Remove(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", p.Name, common.StatusJoin)))
	return cluster.SaveClusterState(c, common.StatusRunning)
}

func (p *Amazon) DeleteK3sCluster(f bool) (err error) {
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
		p.newClient()
		err = p.deleteCluster(f)
		if err != nil {
			return err
		}
		p.logger.Infof("[%s] successfully excuted delete cluster logic\n", p.GetProviderName())
	}
	return nil
}

func (p *Amazon) SSHK3sNode(ssh *types.SSH, ip string) error {
	p.logger = common.NewLogger(common.Debug, nil)
	p.logger.Infof("[%s] executing ssh logic...\n", p.GetProviderName())
	p.newClient()
	instanceList, err := p.syncClusterInstance(ssh)
	if err != nil {
		return err
	}
	ids := make(map[string]string, len(instanceList))
	if ip == "" {
		// generate node name
		for _, instance := range instanceList {
			instanceInfo := ""
			instanceInfo = *instance.PublicIpAddress
			if instanceInfo != "" {
				for _, t := range instance.Tags {
					if aws.StringValue(t.Key) != "master" && aws.StringValue(t.Key) != "worker" {
						continue
					}
					if aws.StringValue(t.Value) == "true" {
						instanceInfo = fmt.Sprintf("%s (%s)", instanceInfo, aws.StringValue(t.Key))
						break
					}
				}
				if aws.StringValue(instance.State.Name) != ec2.InstanceStateNameRunning {
					instanceInfo = fmt.Sprintf("%s - Unhealthy(instance is %s)", instanceInfo, *instance.State.Name)
				}
				ids[aws.StringValue(instance.InstanceId)] = instanceInfo
			}
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
	if ip == "" {
		ip = strings.Split(utils.AskForSelectItem(fmt.Sprintf("[%s] choose ssh node to connect", p.GetProviderName()), ids), " (")[0]
	}

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

func (p *Amazon) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	if p.client == nil {
		p.newClient()
	}

	output, err := p.describeInstances()
	if err != nil {
		return false, ids, err
	}

	for _, instance := range output {
		if aws.StringValue(instance.State.Name) != ec2.InstanceStateNameTerminated &&
			aws.StringValue(instance.State.Name) != ec2.InstanceStateNameShuttingDown {
			ids = append(ids, *instance.InstanceId)
		}
	}

	return len(ids) > 0, ids, nil
}

func (p *Amazon) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	if option, ok := cluster.Options.(typesaws.Options); ok {
		if cluster.CloudControllerManager {
			return fmt.Sprintf(" --kubelet-arg=cloud-provider=external --kubelet-arg=provider-id=aws:///%s/%s --node-name='$(hostname -f)'", option.Zone, master.InstanceID)
		}
	}
	return ""
}

func (p *Amazon) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return p.GenerateMasterExtraArgs(cluster, worker)
}

func (p *Amazon) GetCluster(kubecfg string) *types.ClusterInfo {
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
	if p.client == nil {
		p.newClient()
	}

	output, err := p.describeInstances()
	if err != nil || len(output) == 0 {
		p.logger.Errorf("[%s] failed to get instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
		c.Master = "0"
		c.Worker = "0"
		return c
	}
	masterCount := 0
	workerCount := 0
	for _, ins := range output {
		if aws.StringValue(ins.State.Name) == ec2.InstanceStateNameTerminated {
			continue
		}
		isMaster := false
		for _, tag := range ins.Tags {
			if strings.EqualFold(*tag.Key, "master") && strings.EqualFold(*tag.Value, "true") {
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

func (p *Amazon) GetClusterConfig() (map[string]schemas.Field, error) {
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

func (p *Amazon) DescribeCluster(kubecfg string) *types.ClusterInfo {
	p.logger = common.NewLogger(common.Debug, nil)
	c := &types.ClusterInfo{
		Name:     strings.Split(p.Name, ".")[0],
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
	if p.client == nil {
		p.newClient()
	}

	output, err := p.describeInstances()
	if err != nil || len(output) == 0 {
		p.logger.Errorf("[%s] failed to get instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
		c.Master = "0"
		c.Worker = "0"
		return c
	}
	instanceNodes := make([]types.ClusterNode, 0)
	masterCount := 0
	workerCount := 0
	for _, instance := range output {
		if aws.StringValue(instance.State.Name) == ec2.InstanceStateNameTerminated {
			continue
		}
		n := types.ClusterNode{
			InstanceID:              aws.StringValue(instance.InstanceId),
			InstanceStatus:          aws.StringValue(instance.State.Name),
			InternalIP:              []string{aws.StringValue(instance.PrivateIpAddress)},
			ExternalIP:              []string{aws.StringValue(instance.PublicIpAddress)},
			Status:                  types.ClusterStatusUnknown,
			ContainerRuntimeVersion: types.ClusterStatusUnknown,
			Version:                 types.ClusterStatusUnknown,
		}
		isMaster := false
		for _, tag := range instance.Tags {
			if strings.EqualFold(*tag.Key, "master") && strings.EqualFold(*tag.Value, "true") {
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

func (p *Amazon) GetProviderOption() (map[string]schemas.Field, error) {
	return utils.ConvertToFields(p.Options)
}

func (p *Amazon) SetConfig(config []byte) error {
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
	opt := &typesaws.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

	return nil
}

func (p *Amazon) Rollback() error {
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
		tags := []*ec2.Tag{
			{
				Key:   aws.String("autok3s"),
				Value: aws.String("true"),
			},
			{
				Key:   aws.String("cluster"),
				Value: aws.String(common.TagClusterPrefix + p.Name),
			},
		}
		tagInput := &ec2.DeleteTagsInput{}
		tagInput.SetTags(tags)
		tagInput.SetResources(aws.StringSlice(ids))
		_, err = p.client.DeleteTags(tagInput)
		if err != nil {
			return err
		}
		input := &ec2.TerminateInstancesInput{}
		input.SetInstanceIds(aws.StringSlice(ids))
		_, err = p.client.TerminateInstances(input)
		if err != nil {
			return err
		}
		if p.CloudControllerManager {
			err = p.removeTagsForCCMResource()
			if err != nil {
				return err
			}
		}
	}

	p.logger.Infof("[%s] successfully executed rollback logic\n", p.GetProviderName())

	return logFile.Close()
}

func (p *Amazon) generateInstance(fn checkFun, ssh *types.SSH) (*types.Cluster, error) {
	p.newClient()
	if err := fn(); err != nil {
		return nil, err
	}
	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.logger.Infof("[%s] %d masters and %d workers will be added in region %s\n", p.GetProviderName(), masterNum, workerNum, p.Region)

	if err := p.createKeyPair(ssh); err != nil {
		return nil, err
	}

	if p.SecurityGroup == "" {
		if err := p.configSecurityGroup(); err != nil {
			return nil, err
		}
	}

	// run ecs master instances.
	if masterNum > 0 {
		p.logger.Debugf("[%s] prepare for %d of master instances \n", p.GetProviderName(), masterNum)
		if err := p.runInstances(masterNum, true); err != nil {
			return nil, err
		}
		p.logger.Debugf("[%s] %d of master instances created successfully \n", p.GetProviderName(), masterNum)
	}

	// run ecs worker instances.
	if workerNum > 0 {
		p.logger.Debugf("[%s] prepare for %d of worker instances \n", p.GetProviderName(), workerNum)
		if err := p.runInstances(workerNum, false); err != nil {
			return nil, err
		}
		p.logger.Debugf("[%s] %d of worker instances created successfully \n", p.GetProviderName(), workerNum)
	}

	if err := p.getInstanceStatus(ec2.InstanceStateNameRunning); err != nil {
		return nil, err
	}

	c, err := p.assembleInstanceStatus(ssh)

	if c.CloudControllerManager {
		// generate tags for security group and subnet
		// https://rancher.com/docs/rancher/v2.x/en/cluster-provisioning/rke-clusters/cloud-providers/amazon/#2-configure-the-clusterid
		err = p.addTagsForCCMResource()
		if err != nil {
			return nil, err
		}
		c.MasterExtraArgs += " --disable-cloud-controller --no-deploy servicelb,traefik,local-storage"
	}

	return c, err
}

func (p *Amazon) newClient() {
	if p.AccessKey == "" {
		p.AccessKey = viper.GetString(p.GetProviderName(), "access-key")
	}

	if p.SecretKey == "" {
		p.SecretKey = viper.GetString(p.GetProviderName(), "secret-key")
	}
	config := aws.NewConfig()
	config = config.WithRegion(p.Region)
	config = config.WithCredentials(credentials.NewStaticCredentials(p.AccessKey, p.SecretKey, ""))
	sess := session.Must(session.NewSession(config))
	p.client = ec2.New(sess)
}

func (p *Amazon) runInstances(num int, master bool) error {
	rootSize, err := strconv.ParseInt(p.RootSize, 10, 64)
	if err != nil {
		return fmt.Errorf("[%s] --root-size is invalid %v, must be integer: %v", p.GetProviderName(), p.RootSize, err)
	}
	bdm := &ec2.BlockDeviceMapping{
		DeviceName: aws.String(defaultDeviceName),
		Ebs: &ec2.EbsBlockDevice{
			VolumeSize:          aws.Int64(rootSize),
			VolumeType:          aws.String(p.VolumeType),
			DeleteOnTermination: aws.Bool(true),
		},
	}
	netSpecs := []*ec2.InstanceNetworkInterfaceSpecification{{
		DeviceIndex:              aws.Int64(0), // eth0
		Groups:                   aws.StringSlice([]string{p.SecurityGroup}),
		SubnetId:                 &p.SubnetID,
		AssociatePublicIpAddress: aws.Bool(true),
	}}

	var iamProfile *ec2.IamInstanceProfileSpecification
	if master {
		iamProfile = &ec2.IamInstanceProfileSpecification{
			Name: &p.IamInstanceProfileForControl,
		}
	} else {
		iamProfile = &ec2.IamInstanceProfileSpecification{
			Name: &p.IamInstanceProfileForWorker,
		}
	}

	var instanceList []*ec2.Instance
	if p.RequestSpotInstance {
		if p.SpotPrice == "" {
			p.SpotPrice = defaultSpotPrice
		}
		req := ec2.RequestSpotInstancesInput{
			LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
				ImageId: &p.AMI,
				Placement: &ec2.SpotPlacement{
					AvailabilityZone: &p.Zone,
				},
				KeyName:             &p.KeypairName,
				InstanceType:        &p.InstanceType,
				NetworkInterfaces:   netSpecs,
				IamInstanceProfile:  iamProfile,
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{bdm},
			},
			InstanceCount: aws.Int64(int64(num)),
			SpotPrice:     &p.SpotPrice,
		}

		spotInstanceRequest, err := p.client.RequestSpotInstances(&req)
		if err != nil {
			return fmt.Errorf("[%s] failed request spot instance: %v", p.GetProviderName(), err)
		}
		requestID := spotInstanceRequest.SpotInstanceRequests[0].SpotInstanceRequestId
		p.logger.Debugf("[%s] waiting for spot instance full filled", p.GetProviderName())
		err = utils.WaitFor(func() (bool, error) {
			err := p.client.WaitUntilSpotInstanceRequestFulfilled(&ec2.DescribeSpotInstanceRequestsInput{
				SpotInstanceRequestIds: []*string{requestID},
			})
			if err != nil {
				if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "InvalidSpotInstanceRequestID.NotFound" {
					return false, nil
				}
				return false, err
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("[%s] wait for fulfilling spot request error: %v", p.GetProviderName(), err)
		}
		p.logger.Debugf("[%s] resolve instance information by spot request id %s", p.GetProviderName(), *requestID)
		err = utils.WaitFor(func() (bool, error) {
			spotInstance, err := p.client.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
				SpotInstanceRequestIds: []*string{requestID},
			})
			if err != nil {
				return false, err
			}
			if spotInstance != nil && spotInstance.SpotInstanceRequests != nil {
				instanceIDs := []*string{}
				for _, spotIns := range spotInstance.SpotInstanceRequests {
					instanceIDs = append(instanceIDs, spotIns.InstanceId)
				}
				output, err := p.client.DescribeInstances(&ec2.DescribeInstancesInput{
					InstanceIds: instanceIDs,
				})
				if err != nil {
					return false, err
				}
				for _, ins := range output.Reservations {
					instanceList = append(instanceList, ins.Instances[0])
				}
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			return fmt.Errorf("[%s] failed to get instance by spot instance: %v", p.GetProviderName(), err)
		}
	} else {
		input := &ec2.RunInstancesInput{
			ImageId:  &p.AMI,
			MinCount: aws.Int64(int64(num)),
			MaxCount: aws.Int64(int64(num)),
			Placement: &ec2.Placement{
				AvailabilityZone: &p.Zone,
			},
			KeyName:             &p.KeypairName,
			InstanceType:        &p.InstanceType,
			NetworkInterfaces:   netSpecs,
			IamInstanceProfile:  iamProfile,
			BlockDeviceMappings: []*ec2.BlockDeviceMapping{bdm},
		}

		inst, err := p.client.RunInstances(input)

		if err != nil || len(inst.Instances) != num {
			return fmt.Errorf("[%s] calling runInstances error. region: %s, zone: %s, msg: [%v]",
				p.GetProviderName(), p.Region, p.Zone, err)
		}
		instanceList = inst.Instances
	}

	ids := []*string{}
	for _, ins := range instanceList {
		ids = append(ids, ins.InstanceId)
		p.m.Store(aws.StringValue(ins.InstanceId), types.Node{Master: master, RollBack: true, InstanceID: aws.StringValue(ins.InstanceId), InstanceStatus: aws.StringValue(ins.State.Name)})
	}

	return p.setInstanceTags(master, ids)
}

func (p *Amazon) setInstanceTags(master bool, instanceIDs []*string) error {
	tags := []*ec2.Tag{
		{
			Key:   aws.String("autok3s"),
			Value: aws.String("true"),
		},
		{
			Key:   aws.String("cluster"),
			Value: aws.String(common.TagClusterPrefix + p.Name),
		},
		{
			Key:   aws.String("master"),
			Value: aws.String(strconv.FormatBool(master)),
		},
	}

	if master {
		tags = append(tags, &ec2.Tag{
			Key:   aws.String("Name"),
			Value: aws.String(fmt.Sprintf(common.MasterInstanceName, p.Name)),
		})
	} else {
		tags = append(tags, &ec2.Tag{
			Key:   aws.String("Name"),
			Value: aws.String(fmt.Sprintf(common.WorkerInstanceName, p.Name)),
		})
	}

	if p.CloudControllerManager {
		tags = append(tags, &ec2.Tag{
			Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", p.Name)),
			Value: aws.String("owned"),
		})
	}
	_, err := p.client.CreateTags(&ec2.CreateTagsInput{
		Resources: instanceIDs,
		Tags:      tags,
	})
	return err
}

func (p *Amazon) getInstanceStatus(aimStatus string) error {
	ids := make([]string, 0)
	p.m.Range(func(key, value interface{}) bool {
		ids = append(ids, key.(string))
		return true
	})

	if len(ids) > 0 {
		p.logger.Debugf("[%s] waiting for the instances %s to be in `%s` status...\n", p.GetProviderName(), ids, aimStatus)
		wait.ErrWaitTimeout = fmt.Errorf("[%s] calling getInstanceStatus error. region: %s, zone: %s, instanceName: %s, message: not `%s` status",
			p.GetProviderName(), p.Region, p.Zone, ids, aimStatus)

		if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			instances, err := p.client.DescribeInstances(&ec2.DescribeInstancesInput{
				InstanceIds: aws.StringSlice(ids),
			})
			if err != nil {
				return false, err
			}

			for _, status := range instances.Reservations[0].Instances {
				if aws.StringValue(status.State.Name) == aimStatus {
					if value, ok := p.m.Load(aws.StringValue(status.InstanceId)); ok {
						v := value.(types.Node)
						v.InstanceStatus = aimStatus
						p.m.Store(aws.StringValue(status.InstanceId), v)
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

func (p *Amazon) assembleInstanceStatus(ssh *types.SSH) (*types.Cluster, error) {
	output, err := p.describeInstances()
	if err != nil || len(output) == 0 {
		return nil, fmt.Errorf("[%s] there's no instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
	}

	for _, instance := range output {
		if aws.StringValue(instance.State.Name) == ec2.InstanceStateNameTerminated {
			continue
		}
		if value, ok := p.m.Load(aws.StringValue(instance.InstanceId)); ok {
			v := value.(types.Node)
			// add only nodes that run the current command.
			v.Current = true
			v.InternalIPAddress = []string{aws.StringValue(instance.PrivateIpAddress)}
			v.PublicIPAddress = []string{aws.StringValue(instance.PublicIpAddress)}
			v.SSH = *ssh
			p.m.Store(aws.StringValue(instance.InstanceId), v)
			continue
		}
		master := false
		for _, tag := range instance.Tags {
			if strings.EqualFold(*tag.Key, "master") && strings.EqualFold(*tag.Value, "true") {
				master = true
				break
			}
		}
		p.m.Store(aws.StringValue(instance.InstanceId), types.Node{
			Master:            master,
			RollBack:          false,
			InstanceID:        aws.StringValue(instance.InstanceId),
			InstanceStatus:    aws.StringValue(instance.State.Name),
			InternalIPAddress: []string{aws.StringValue(instance.PrivateIpAddress)},
			PublicIPAddress:   []string{aws.StringValue(instance.PublicIpAddress)}})
	}

	p.syncNodeStatusWithInstance(ssh)

	return &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}, nil

}

func (p *Amazon) syncNodeStatusWithInstance(ssh *types.SSH) {
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

func (p *Amazon) describeInstances() ([]*ec2.Instance, error) {
	describeInput := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:autok3s"),
				Values: aws.StringSlice([]string{"true"}),
			},
			{
				Name:   aws.String("tag:cluster"),
				Values: aws.StringSlice([]string{common.TagClusterPrefix + p.Name}),
			},
		},
		MaxResults: aws.Int64(int64(50)),
	}

	instanceList := []*ec2.Instance{}
	for {
		output, err := p.client.DescribeInstances(describeInput)
		if output == nil || err != nil {
			return nil, fmt.Errorf("[%s] failed to get instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
		}

		if len(output.Reservations) > 0 {
			for _, reservation := range output.Reservations {
				for _, instance := range reservation.Instances {
					instanceList = append(instanceList, instance)
				}
			}
		}
		if aws.StringValue(output.NextToken) == "" {
			break
		}
		describeInput.NextToken = output.NextToken
	}
	return instanceList, nil
}

func (p *Amazon) CreateCheck(ssh *types.SSH) error {
	if p.KeypairName != "" && ssh.SSHKeyPath == "" {
		return fmt.Errorf("[%s] calling preflight error: must set --ssh-key-path with --keypair-name %s", p.GetProviderName(), p.KeypairName)
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

	if masterNum > 0 && p.CloudControllerManager && p.IamInstanceProfileForControl == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--iam-instance-profile-control` if enabled Amazon Cloud Controller Manager", p.GetProviderName())
	}

	workerNum, err := strconv.Atoi(p.Worker)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--worker` must be number",
			p.GetProviderName())
	}
	if workerNum > 0 && p.CloudControllerManager && p.IamInstanceProfileForWorker == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--iam-instance-profile-worker` if enabled Amazon Cloud Controller Manager", p.GetProviderName())
	}

	exist, _, err := p.IsClusterExist()
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` is already exist",
			p.GetProviderName(), p.Name)
	}

	// check key pair
	if p.KeypairName != "" {
		_, err = p.client.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
			KeyNames: []*string{&p.KeypairName},
		})
		if err != nil {
			return fmt.Errorf("[%s] failed to get keypair by name %s, got error: %v", p.GetProviderName(), p.KeypairName, err)
		}
	}

	// check vpc and subnet
	if p.VpcID == "" {
		p.VpcID, err = p.getDefaultVPCId()
		if err != nil {
			return err
		}
	}

	if p.SubnetID == "" && p.VpcID == "" {
		return fmt.Errorf("[%s] calling preflight error: can't generate instance without vpc and subnet", p.GetProviderName())
	}

	if p.SubnetID != "" && p.VpcID != "" {
		// check subnet is belongs to vpc
		subnetFilter := []*ec2.Filter{
			{
				Name:   aws.String("subnet-id"),
				Values: []*string{&p.SubnetID},
			},
		}

		subnets, err := p.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
			Filters: subnetFilter,
		})
		if err != nil {
			return err
		}

		if subnets == nil || len(subnets.Subnets) == 0 {
			return fmt.Errorf("[%s] there's not subnet found by id %s", p.GetProviderName(), p.SubnetID)
		}

		if *subnets.Subnets[0].VpcId != p.VpcID {
			return fmt.Errorf("[%s] subnetId %s does not belong to VpcId: %s", p.GetProviderName(), p.SubnetID, p.VpcID)
		}
	}

	if p.SubnetID == "" {
		filters := []*ec2.Filter{
			{
				Name:   aws.String("availability-zone"),
				Values: []*string{&p.Zone},
			},
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{&p.VpcID},
			},
		}

		subnets, err := p.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
			Filters: filters,
		})
		if err != nil {
			return err
		}

		if len(subnets.Subnets) == 0 {
			return fmt.Errorf("can't get subnets for vpc %s at zone %s", p.VpcID, p.Zone)
		}

		// find default subnet
		if len(subnets.Subnets) > 1 {
			for _, subnet := range subnets.Subnets {
				if subnet.DefaultForAz != nil && *subnet.DefaultForAz {
					p.SubnetID = *subnet.SubnetId
					break
				}
			}
		}

		if p.SubnetID == "" {
			p.SubnetID = *subnets.Subnets[0].SubnetId
		}
	}
	return nil
}

func (p *Amazon) joinCheck() error {
	if strings.Contains(p.MasterExtraArgs, "--datastore-endpoint") && p.DataStore != "" {
		return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
			p.GetProviderName())
	}

	masterNum, err := strconv.Atoi(p.Master)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--master` must be number",
			p.GetProviderName())
	}
	workerNum, err := strconv.Atoi(p.Worker)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--worker` must be number",
			p.GetProviderName())
	}
	if masterNum < 1 && workerNum < 1 {
		return fmt.Errorf("[%s] calling preflight error: `--master` or `--worker` number must >= 1", p.GetProviderName())
	}

	if masterNum > 0 && p.CloudControllerManager && p.IamInstanceProfileForControl == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--iam-instance-profile-control` if enabled Amazon Cloud Controller Manager", p.GetProviderName())
	}

	if workerNum > 0 && p.CloudControllerManager && p.IamInstanceProfileForWorker == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--iam-instance-profile-worker` if enabled Amazon Cloud Controller Manager", p.GetProviderName())
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
	masters := make([]types.Node, 0)
	for _, m := range p.MasterNodes {
		for _, e := range ids {
			if e == m.InstanceID {
				masters = append(masters, m)
				break
			}
		}
	}
	p.WorkerNodes = workers
	p.MasterNodes = masters

	return nil
}

func (p *Amazon) createKeyPair(ssh *types.SSH) error {
	if p.KeypairName != "" && ssh.SSHKeyPath == "" && p.KeypairName != p.Name {
		return fmt.Errorf("[%s] calling preflight error: --ssh-key-path must set with --key-pair %s", p.GetProviderName(), p.KeypairName)
	}

	// check upload keypair
	if ssh.SSHKeyPath == "" {
		if _, err := os.Stat(common.GetDefaultSSHKeyPath(p.Name, p.GetProviderName())); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			pk, err := putil.CreateKeyPair(ssh, p.GetProviderName(), p.Name, p.KeypairName)
			if err != nil {
				return err
			}

			if pk != nil {
				keyName := p.Name
				p.logger.Debugf("creating key pair: %s", keyName)
				_, err = p.client.ImportKeyPair(&ec2.ImportKeyPairInput{
					KeyName:           &keyName,
					PublicKeyMaterial: pk,
				})
				if err != nil {
					return err
				}
				p.KeypairName = keyName
			}
		}
		p.KeypairName = p.Name
		ssh.SSHKeyPath = common.GetDefaultSSHKeyPath(p.Name, p.GetProviderName())
	}

	return nil
}

func (p *Amazon) getDefaultVPCId() (string, error) {
	output, err := p.client.DescribeAccountAttributes(&ec2.DescribeAccountAttributesInput{})
	if err != nil {
		return "", err
	}

	for _, attribute := range output.AccountAttributes {
		if aws.StringValue(attribute.AttributeName) == "default-vpc" {
			value := aws.StringValue(attribute.AttributeValues[0].AttributeValue)
			if value == "none" {
				return "", errors.New("there's 'none' for default vpc")
			}
			return value, nil
		}
	}

	return "", errors.New("couldn't get default vpc")
}

func (p *Amazon) configSecurityGroup() error {
	p.logger.Infof("[%s] config default security group for %s in region %s\n", p.GetProviderName(), p.VpcID, p.Region)

	filters := []*ec2.Filter{
		{
			Name:   aws.String("group-name"),
			Values: aws.StringSlice([]string{defaultSecurityGroupName}),
		},
		{
			Name:   aws.String("vpc-id"),
			Values: aws.StringSlice([]string{p.VpcID}),
		},
	}
	groups, err := p.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	})
	if err != nil {
		return err
	}

	var securityGroup *ec2.SecurityGroup
	if len(groups.SecurityGroups) > 0 {
		// get default security group
		securityGroup = groups.SecurityGroups[0]
	}

	if securityGroup == nil {
		p.logger.Infof("creating security group (%s) in %s", defaultSecurityGroupName, p.VpcID)
		groupResp, err := p.client.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
			GroupName:   aws.String(defaultSecurityGroupName),
			Description: aws.String("default security group generated by autok3s"),
			VpcId:       aws.String(p.VpcID),
		})
		if err != nil {
			return err
		}
		// Manually translate into the security group construct
		securityGroup = &ec2.SecurityGroup{
			GroupId:   groupResp.GroupId,
			VpcId:     aws.String(p.VpcID),
			GroupName: aws.String(defaultSecurityGroupName),
		}
		// wait until created (dat eventual consistency)
		p.logger.Debugf("waiting for group (%s) to become available", *securityGroup.GroupId)
		err = utils.WaitFor(func() (bool, error) {
			s, err := p.getSecurityGroup(groupResp.GroupId)
			if s != nil && err == nil {
				return true, nil
			}
			return false, err
		})
		if err != nil {
			return err
		}
	}
	p.SecurityGroup = aws.StringValue(securityGroup.GroupId)
	permissionList := p.configPermission(securityGroup)
	if len(permissionList) != 0 {
		p.logger.Debugf("authorizing group %s with permissions: %v", defaultSecurityGroupName, permissionList)
		_, err := p.client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       securityGroup.GroupId,
			IpPermissions: permissionList,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Amazon) getSecurityGroup(id *string) (*ec2.SecurityGroup, error) {
	securityGroups, err := p.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{id},
	})
	if err != nil {
		return nil, err
	}
	return securityGroups.SecurityGroups[0], nil
}

func (p *Amazon) configPermission(group *ec2.SecurityGroup) []*ec2.IpPermission {
	perms := []*ec2.IpPermission{}
	hasPorts := make(map[string]bool)
	for _, p := range group.IpPermissions {
		if p.FromPort != nil {
			hasPorts[fmt.Sprintf("%d/%s", *p.FromPort, *p.IpProtocol)] = true
		}
	}

	if !hasPorts["22/tcp"] {
		perms = append(perms, &ec2.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(int64(22)),
			ToPort:     aws.Int64(int64(22)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(ipRange)}},
		})
	}

	if !hasPorts["6443/tcp"] {
		perms = append(perms, &ec2.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(int64(6443)),
			ToPort:     aws.Int64(int64(6443)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(ipRange)}},
		})
	}

	if !hasPorts["10250/tcp"] {
		perms = append(perms, &ec2.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(int64(10250)),
			ToPort:     aws.Int64(int64(10250)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(ipRange)}},
		})
	}

	if (p.Network == "" || p.Network == "vxlan") && !hasPorts["8472/udp"] {
		if !hasPorts["8472/udp"] {
			// udp 8472 for flannel vxlan
			perms = append(perms, &ec2.IpPermission{
				IpProtocol: aws.String("udp"),
				FromPort:   aws.Int64(int64(8472)),
				ToPort:     aws.Int64(int64(8472)),
				IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(ipRange)}},
			})
		}
	}

	if p.Cluster && (!hasPorts["2379/tcp"] || !hasPorts["2380/tcp"]) {
		cidr, err := p.getSubnetCIDR()
		if err != nil || cidr == "" {
			p.logger.Errorf("[%s] failed to get subnet cidr with id %s, error: %v", p.GetProviderName(), p.SubnetID, err)
			cidr = ipRange
		}
		perms = append(perms, &ec2.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(int64(2379)),
			ToPort:     aws.Int64(int64(2379)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(cidr)}},
		})
		perms = append(perms, &ec2.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(int64(2380)),
			ToPort:     aws.Int64(int64(2380)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(cidr)}},
		})
	}

	if p.UI && !hasPorts["8999/tcp"] {
		perms = append(perms, &ec2.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int64(int64(8999)),
			ToPort:     aws.Int64(int64(8999)),
			IpRanges:   []*ec2.IpRange{{CidrIp: aws.String(ipRange)}},
		})
	}

	return perms
}

func (p *Amazon) syncClusterInstance(ssh *types.SSH) ([]*ec2.Instance, error) {
	output, err := p.describeInstances()
	if err != nil || len(output) == 0 {
		return nil, fmt.Errorf("[%s] there's no exist instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
	}

	for _, instance := range output {
		if aws.StringValue(instance.State.Name) == ec2.InstanceStateNameTerminated {
			continue
		}
		// sync all instance that belongs to current clusters
		master := false
		for _, tag := range instance.Tags {
			if strings.EqualFold(aws.StringValue(tag.Key), "master") && strings.EqualFold(aws.StringValue(tag.Value), "true") {
				master = true
				break
			}
		}

		p.m.Store(aws.StringValue(instance.InstanceId), types.Node{
			Master:            master,
			InstanceID:        aws.StringValue(instance.InstanceId),
			InstanceStatus:    aws.StringValue(instance.State.Name),
			InternalIPAddress: []string{aws.StringValue(instance.PrivateIpAddress)},
			PublicIPAddress:   []string{aws.StringValue(instance.PublicIpAddress)},
			SSH:               *ssh,
		})
	}

	p.syncNodeStatusWithInstance(ssh)

	return output, nil
}

func (p *Amazon) deleteCluster(f bool) error {
	exist, ids, err := p.IsClusterExist()
	if err != nil && !f {
		return fmt.Errorf("[%s] calling deleteCluster error, msg: %v", p.GetProviderName(), err)
	}
	if !exist {
		p.logger.Errorf("[%s] cluster %s is not exist", p.GetProviderName(), p.Name)
		if !f {
			return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", p.GetProviderName(), p.Name)
		}
		return nil
	}
	if p.UI && p.CloudControllerManager {
		// remove ui manifest to release ELB
		masterIP := p.IP
		for _, n := range p.Status.MasterNodes {
			if n.InternalIPAddress[0] == masterIP {
				dialer, err := hosts.SSHDialer(&hosts.Host{Node: n})
				if err != nil {
					return err
				}
				tunnel, err := dialer.OpenTunnel(true)
				if err != nil {
					return err
				}
				var (
					stdout bytes.Buffer
					stderr bytes.Buffer
				)
				tunnel.Writer = p.logger.Out
				tunnel.Cmd(fmt.Sprintf("sudo kubectl delete -f %s/ui.yaml", common.K3sManifestsDir))
				tunnel.Cmd(fmt.Sprintf("sudo rm %s/ui.yaml", common.K3sManifestsDir))
				tunnel.SetStdio(&stdout, &stderr).Run()
				tunnel.Close()
				break
			}
		}
	}
	if len(ids) > 0 {
		tags := []*ec2.Tag{
			{
				Key:   aws.String("autok3s"),
				Value: aws.String("true"),
			},
			{
				Key:   aws.String("cluster"),
				Value: aws.String(common.TagClusterPrefix + p.Name),
			},
		}
		tagInput := &ec2.DeleteTagsInput{}
		tagInput.SetTags(tags)
		tagInput.SetResources(aws.StringSlice(ids))
		_, err := p.client.DeleteTags(tagInput)
		if err != nil {
			return err
		}
		p.logger.Infof("[%s] terminate instance %v", p.GetProviderName(), ids)
		input := &ec2.TerminateInstancesInput{}
		input.SetInstanceIds(aws.StringSlice(ids))
		_, err = p.client.TerminateInstances(input)
		if err != nil {
			return err
		}
		if p.CloudControllerManager {
			err = p.removeTagsForCCMResource()
			if err != nil {
				return err
			}
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

	p.logger.Infof("[%s] successfully deleted cluster %s\n", p.GetProviderName(), p.Name)
	return nil
}

func (p *Amazon) getSubnetCIDR() (string, error) {
	subnetFilter := []*ec2.Filter{
		{
			Name:   aws.String("subnet-id"),
			Values: []*string{&p.SubnetID},
		},
	}

	subnets, err := p.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: subnetFilter,
	})
	if err != nil {
		return "", err
	}

	if subnets == nil || len(subnets.Subnets) == 0 {
		return "", fmt.Errorf("[%s] there's not subnet found by id %s", p.GetProviderName(), p.SubnetID)
	}

	return aws.StringValue(subnets.Subnets[0].CidrBlock), nil
}

func (p *Amazon) addTagsForCCMResource() error {
	// get security group
	result, err := p.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: aws.StringSlice([]string{p.SecurityGroup}),
	})
	if err != nil || result == nil || len(result.SecurityGroups) == 0 {
		return fmt.Errorf("[%s] failed to get security group %s with error: %v", p.GetProviderName(), p.SecurityGroup, err)
	}
	tags := result.SecurityGroups[0].Tags
	tags = append(tags, &ec2.Tag{
		Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", p.Name)),
		Value: aws.String("shared"),
	})
	_, err = p.client.CreateTags(&ec2.CreateTagsInput{
		Resources: aws.StringSlice([]string{p.SecurityGroup}),
		Tags:      tags,
	})
	if err != nil {
		return err
	}

	// get subnet
	subnetFilter := []*ec2.Filter{
		{
			Name:   aws.String("subnet-id"),
			Values: []*string{&p.SubnetID},
		},
	}
	subnets, err := p.client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: subnetFilter,
	})
	if err != nil || subnets == nil || len(subnets.Subnets) == 0 {
		return fmt.Errorf("[%s] failed to get subnets %s, error: %v", p.GetProviderName(), p.SubnetID, err)
	}
	subnetTags := subnets.Subnets[0].Tags
	subnetTags = append(subnetTags, &ec2.Tag{
		Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", p.Name)),
		Value: aws.String("shared"),
	})
	_, err = p.client.CreateTags(&ec2.CreateTagsInput{
		Resources: aws.StringSlice([]string{p.SubnetID}),
		Tags:      subnetTags,
	})

	return err
}

func (p *Amazon) removeTagsForCCMResource() error {
	deletedTags := []*ec2.Tag{
		{
			Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", p.Name)),
			Value: aws.String("shared"),
		},
	}
	_, err := p.client.DeleteTags(&ec2.DeleteTagsInput{
		Resources: aws.StringSlice([]string{p.SecurityGroup}),
		Tags:      deletedTags,
	})
	if err != nil {
		return err
	}
	_, err = p.client.DeleteTags(&ec2.DeleteTagsInput{
		Resources: aws.StringSlice([]string{p.SubnetID}),
		Tags:      deletedTags,
	})
	return err
}
