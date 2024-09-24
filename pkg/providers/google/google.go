package google

import (
	"context"
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
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	typesgoogle "github.com/cnrancher/autok3s/pkg/types/google"
	"github.com/cnrancher/autok3s/pkg/utils"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	raw "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	providerName = "google"

	defaultUser = "autok3s"

	defaultSecurityGroup = "autok3s"
	apiURL               = "https://www.googleapis.com/compute/v1/projects"
	statusRunning        = "RUNNING"

	deployCCMCommand = "echo \"%s\" | base64 -d | tee \"%s/gcp-cloud-controller-manager.yaml\""
)

// Google provider
type Google struct {
	*cluster.ProviderBase `json:",inline"`
	typesgoogle.Options   `json:",inline"`
	client                *raw.Service
	globalURL             string
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Google {
	base := cluster.NewBaseProvider()
	base.Provider = providerName
	googleProvider := &Google{
		ProviderBase: base,
	}
	if opt, ok := common.DefaultTemplates[providerName]; ok {
		googleProvider.Options = opt.(typesgoogle.Options)
	}
	return googleProvider
}

// GetProviderName returns provider name.
func (p *Google) GetProviderName() string {
	return p.Provider
}

// GenerateClusterName generates and returns cluster name.
func (p *Google) GenerateClusterName() string {
	p.ContextName = fmt.Sprintf("%s.%s.%s", p.Name, p.Region, p.GetProviderName())
	return p.ContextName
}

func (p *Google) GenerateManifest() []string {
	if p.CloudControllerManager {
		tmp := fmt.Sprintf(googleCCMTmpl, p.ClusterCidr)
		return []string{fmt.Sprintf(deployCCMCommand,
			base64.StdEncoding.EncodeToString([]byte(tmp)), common.K3sManifestsDir)}
	}
	return nil
}

// CreateK3sCluster create K3S cluster on Google Cloud Provider.
func (p *Google) CreateK3sCluster() error {
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	if p.client == nil {
		if err := p.newClient(); err != nil {
			return err
		}
	}
	return p.InitCluster(p.Options, p.GenerateManifest, p.generateInstance, nil, p.rollbackInstance)
}

// JoinK3sNode join K3S node for exist cluster on Google Cloud Provider.
func (p *Google) JoinK3sNode() error {
	if p.SSHUser == "" {
		p.SSHUser = defaultUser
	}
	if p.client == nil {
		if err := p.newClient(); err != nil {
			return err
		}
	}
	return p.JoinNodes(p.generateInstance, p.syncInstances, false, p.rollbackInstance)
}

// DeleteK3sCluster delete K3S cluster.
func (p *Google) DeleteK3sCluster(f bool) error {
	return p.DeleteCluster(f, p.remove)
}

func (p *Google) remove(force bool) (string, error) {
	err := p.newClient()
	if err != nil {
		return "", err
	}
	p.GenerateClusterName()
	exist, ids, err := p.IsClusterExist()
	if err != nil {
		return "", fmt.Errorf("[%s] calling describe instance error, msg: %v", p.GetProviderName(), err)
	}
	if !exist {
		p.Logger.Errorf("[%s] cluster %s is not exist", p.GetProviderName(), p.Name)
		if !force {
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
		p.Logger.Infof("[%s] release manifests", p.GetProviderName())
		if err := p.ReleaseManifests(); err != nil {
			return "", err
		}
	}

	_ = p.rollbackInstance(ids)
	p.Logger.Infof("[%s] successfully terminate instances for cluster %s", p.GetProviderName(), p.Name)

	return p.ContextName, nil
}

func (p *Google) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	return p.Connect(ip, &p.SSH, c, p.getInstanceNodes, p.isInstanceRunning, nil)
}

// IsClusterExist determine if the cluster exists.
func (p *Google) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	if p.client == nil {
		if err := p.newClient(); err != nil {
			return false, nil, err
		}
	}

	instanceList, err := p.describeInstances()
	if err != nil {
		return false, ids, err
	}

	for _, ins := range instanceList {
		ids = append(ids, ins.Name)
	}

	return len(ids) > 0, ids, nil
}

func (p *Google) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	if option, ok := cluster.Options.(typesgoogle.Options); ok {
		if option.CloudControllerManager {
			return fmt.Sprintf(" --kubelet-arg=cloud-provider=external --kubelet-arg=provider-id=gce://%s/%s/%s --node-name=%s", option.Project, option.Zone, master.InstanceID, master.InstanceID)
		}
	}
	return ""
}

func (p *Google) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return p.GenerateMasterExtraArgs(cluster, worker)
}

// SetOptions merge option struct for Google Cloud Provider
func (p *Google) SetOptions(opt []byte) error {
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	option := &typesgoogle.Options{}
	err := json.Unmarshal(opt, option)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(option).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// GetCluster returns cluster status.
func (p *Google) GetCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       p.ContextName,
		Name:     p.Name,
		Provider: p.GetProviderName(),
		Region:   p.Region,
	}

	return p.GetClusterStatus(kubecfg, c, p.getInstanceNodes)
}

// DescribeCluster describe cluster info.
func (p *Google) DescribeCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		Name:     p.Name,
		Region:   p.Region,
		Zone:     p.Zone,
		Provider: p.GetProviderName(),
	}
	return p.Describe(kubecfg, c, p.getInstanceNodes)
}

// SetConfig merge cluster config for Google Cloud Provider.
func (p *Google) SetConfig(config []byte) error {
	c, err := p.SetClusterConfig(config)
	if err != nil {
		return err
	}
	sourceOption := reflect.ValueOf(&p.Options).Elem()
	b, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	opt := &typesgoogle.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

	return nil
}

// CreateCheck check create command and flags.
func (p *Google) CreateCheck() error {
	if p.client == nil {
		if err := p.newClient(); err != nil {
			return err
		}
	}

	if err := p.CheckCreateArgs(p.IsClusterExist); err != nil {
		return err
	}

	// check project exist
	if _, err := p.client.Projects.Get(p.Project).Do(); err != nil {
		return fmt.Errorf("[%s] GCE project id %s is not found: %v", p.GetProviderName(), p.Project, err)
	}

	// check startup script file exist
	if p.StartupScriptPath != "" {
		_, err := os.Stat(p.StartupScriptPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// JoinCheck check join command and flags.
func (p *Google) JoinCheck() error {
	if p.client == nil {
		if err := p.newClient(); err != nil {
			return err
		}
	}

	return p.CheckJoinArgs(p.IsClusterExist)
}

func (p *Google) newClient() error {
	credJSON, err := os.ReadFile(p.ServiceAccountFile)
	if err != nil {
		return err
	}
	ctx := context.TODO()
	ts, err := google.CredentialsFromJSON(ctx, credJSON, raw.ComputeScope)
	if err != nil {
		return err
	}
	client := oauth2.NewClient(ctx, ts.TokenSource)
	service, err := raw.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return err
	}
	p.client = service
	p.globalURL = fmt.Sprintf("%s/%s/global", apiURL, p.Project)
	return nil
}

func (p *Google) generateInstance(ssh *types.SSH) (*types.Cluster, error) {
	masterNum, _ := strconv.Atoi(p.Master)
	workerNum, _ := strconv.Atoi(p.Worker)

	p.Logger.Infof("[%s] %d masters and %d workers will be added in region %s", p.GetProviderName(), masterNum, workerNum, p.Region)

	// generate ssh key
	_, err := putil.CreateKeyPair(ssh, p.GetProviderName(), p.ContextName, "")
	if err != nil {
		return nil, err
	}

	// open firewall ports for VM
	if err = p.ensureFirewall(); err != nil {
		return nil, err
	}

	// set startup script
	if p.StartupScriptContent != "" {
		scriptByte, err := base64.StdEncoding.DecodeString(p.StartupScriptContent)
		if err != nil {
			return nil, err
		}
		p.StartupScriptContent = string(scriptByte)
	} else if p.StartupScriptPath != "" {
		userDataBytes, err := os.ReadFile(p.StartupScriptPath)
		if err != nil {
			return nil, err
		}
		p.StartupScriptContent = string(userDataBytes)
	}

	// create instance
	if masterNum > 0 {
		p.Logger.Infof("[%s] prepare for %d master nodes", p.GetProviderName(), masterNum)
		if err = p.startInstance(masterNum, true); err != nil {
			return nil, err
		}
	}

	if workerNum > 0 {
		p.Logger.Infof("[%s] prepare for %d worker nodes", p.GetProviderName(), workerNum)
		if err = p.startInstance(workerNum, false); err != nil {
			return nil, err
		}
	}

	c := &types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
		Status:   p.Status,
	}
	c.ContextName = p.ContextName
	if p.CloudControllerManager {
		c.MasterExtraArgs += " --disable-cloud-controller --disable servicelb,traefik,local-storage"
	}
	c.SSH = *ssh

	return c, nil
}

func (p *Google) syncInstances() error {
	instanceList, err := p.describeInstances()
	if err != nil || len(instanceList) == 0 {
		return fmt.Errorf("[%s] there's no instance for cluster %s: %v", p.GetProviderName(), p.Name, err)
	}

	for _, instance := range instanceList {
		networkInterface := instance.NetworkInterfaces[0]
		if value, ok := p.M.Load(instance.Name); ok {
			v := value.(types.Node)
			v.InternalIPAddress = []string{networkInterface.NetworkIP}
			v.PublicIPAddress = []string{networkInterface.AccessConfigs[0].NatIP}
			p.M.Store(instance.Name, v)
			continue
		}
		master := false
		for key, value := range instance.Labels {
			if strings.EqualFold(key, "master") && strings.EqualFold(value, "true") {
				master = true
				break
			}
		}
		p.M.Store(instance.Name, types.Node{
			Master:            master,
			RollBack:          false,
			Current:           false,
			InstanceID:        instance.Name,
			InstanceStatus:    instance.Status,
			InternalIPAddress: []string{networkInterface.NetworkIP},
			PublicIPAddress:   []string{networkInterface.AccessConfigs[0].NatIP}})
	}

	return nil
}

func (p *Google) startInstance(num int, master bool) error {
	var vmNet string
	if strings.Contains(p.VMNetwork, "/networks/") {
		vmNet = p.VMNetwork
	} else {
		vmNet = fmt.Sprintf("%s/networks/%s", p.globalURL, p.VMNetwork)
	}

	instance := &raw.Instance{
		Description: "AutoK3s managed VM",
		MachineType: fmt.Sprintf("%s/%s/zones/%s/machineTypes/%s", apiURL, p.Project, p.Zone, p.MachineType),
		Disks: []*raw.AttachedDisk{
			{
				Boot:       true,
				AutoDelete: true,
				Type:       "PERSISTENT",
				Mode:       "READ_WRITE",
			},
		},
		NetworkInterfaces: []*raw.NetworkInterface{
			{
				Network: vmNet,
			},
		},
		Labels: p.generateLabels(master),
		Tags: &raw.Tags{
			Items: []string{"autok3s"},
		},
		ServiceAccounts: []*raw.ServiceAccount{
			{
				Email:  p.ServiceAccount,
				Scopes: strings.Split(p.Scopes, ","),
			},
		},
		Scheduling: &raw.Scheduling{
			Preemptible: p.Preemptible,
		},
		Metadata: &raw.Metadata{},
	}

	if strings.Contains(p.Subnetwork, "/subnetworks/") {
		instance.NetworkInterfaces[0].Subnetwork = p.Subnetwork
	} else if p.Subnetwork != "" {
		instance.NetworkInterfaces[0].Subnetwork = "projects/" + p.Project + "/regions/" + p.Region + "/subnetworks/" + p.Subnetwork
	}

	if !p.UseInternalIPOnly {
		cfg := &raw.AccessConfig{
			Type: "ONE_TO_ONE_NAT",
			Name: "External NAT",
		}
		instance.NetworkInterfaces[0].AccessConfigs = append(instance.NetworkInterfaces[0].AccessConfigs, cfg)
	}

	if p.StartupScriptContent != "" || p.StartupScriptURL != "" {
		if instance.Metadata.Items == nil {
			instance.Metadata.Items = []*raw.MetadataItems{}
		}
		if p.StartupScriptContent != "" {
			instance.Metadata.Items = append(instance.Metadata.Items, &raw.MetadataItems{
				Key:   "startup-script",
				Value: &p.StartupScriptContent,
			})
		}
		if p.StartupScriptURL != "" {
			instance.Metadata.Items = append(instance.Metadata.Items, &raw.MetadataItems{
				Key:   "startup-script-url",
				Value: &p.StartupScriptURL,
			})
		}
	}

	for i := 0; i < num; i++ {
		var instanceName string
		if master {
			instanceName = strings.ReplaceAll(fmt.Sprintf("%s-%s", fmt.Sprintf(common.MasterInstanceName, p.Name), rand.String(5)), ".", "-")
		} else {
			instanceName = strings.ReplaceAll(fmt.Sprintf("%s-%s", fmt.Sprintf(common.WorkerInstanceName, p.Name), rand.String(5)), ".", "-")
		}
		instance.Name = instanceName
		diskName := fmt.Sprintf("%s-disk", instance.Name)
		disk, err := p.getDisk(diskName)
		if disk == nil || err != nil {
			instance.Disks[0].InitializeParams = &raw.AttachedDiskInitializeParams{
				DiskName:    diskName,
				SourceImage: "https://www.googleapis.com/compute/v1/projects/" + p.MachineImage,
				DiskSizeGb:  int64(p.DiskSize),
				DiskType:    fmt.Sprintf("%s/%s/zones/%s/diskTypes/%s", apiURL, p.Project, p.Zone, p.DiskType),
			}
		} else {
			instance.Disks[0].Source = fmt.Sprintf("%s/%s/zones/%s/disks/%s", apiURL, p.Project, p.Zone, diskName)
		}
		p.Logger.Infof("[%s] create instance %s", p.GetProviderName(), instanceName)
		op, err := p.client.Instances.Insert(p.Project, p.Zone, instance).Do()
		if err != nil {
			return err
		}
		p.Logger.Infof("[%s] waiting for instance %s", p.GetProviderName(), instanceName)
		if err = p.waitForRegionalOp(op.Name); err != nil {
			return err
		}

		ins, err := p.instance(instanceName)
		if err != nil {
			return err
		}
		if err = p.uploadKeyPair(ins, common.GetDefaultSSHKeyPath(p.ContextName, p.GetProviderName())); err != nil {
			return err
		}
		networkInterface := ins.NetworkInterfaces[0]
		p.M.Store(ins.Name, types.Node{
			Master:            master,
			Current:           true,
			RollBack:          true,
			InstanceID:        ins.Name,
			InstanceStatus:    ins.Status,
			InternalIPAddress: []string{networkInterface.NetworkIP},
			PublicIPAddress:   []string{networkInterface.AccessConfigs[0].NatIP},
			LocalHostname:     ins.Hostname,
			SSH:               p.SSH,
		})
	}
	return nil
}

func (p *Google) rollbackInstance(ids []string) error {
	for _, name := range ids {
		if err := p.deleteInstance(name); err != nil {
			p.Logger.Errorf("[%s] remove instance %s error: %v", p.GetProviderName(), name, err)
			continue
		}
		if err := p.deleteDisk(name); err != nil {
			p.Logger.Errorf("[%s] remove disk for instance %s error: %v", p.GetProviderName(), name, err)
			continue
		}
	}
	return nil
}

func (p *Google) ensureFirewall() error {
	p.Logger.Infof("[%s] ensure firewall with opening ports", p.GetProviderName())
	firewall, _ := p.client.Firewalls.Get(p.Project, defaultSecurityGroup).Do()
	create := false
	if firewall == nil {
		//create new firewall
		create = true
		firewall = &raw.Firewall{
			Name:         defaultSecurityGroup,
			Allowed:      []*raw.FirewallAllowed{},
			SourceRanges: []string{"0.0.0.0/0"},
			TargetTags:   []string{"autok3s"},
			Network:      p.globalURL + "/networks/" + p.VMNetwork,
		}
	}

	missingPorts := p.configPorts(firewall)
	if len(missingPorts) == 0 {
		return nil
	}
	for proto, ports := range missingPorts {
		firewall.Allowed = append(firewall.Allowed, &raw.FirewallAllowed{
			IPProtocol: proto,
			Ports:      ports,
		})
	}

	var op *raw.Operation
	var err error
	if create {
		op, err = p.client.Firewalls.Insert(p.Project, firewall).Do()
	} else {
		op, err = p.client.Firewalls.Update(p.Project, defaultSecurityGroup, firewall).Do()
	}

	if err != nil {
		return err
	}

	return p.waitForGlobalOp(op.Name)
}

func (p *Google) configPorts(firewall *raw.Firewall) map[string][]string {
	ports := []string{"22/tcp", "6443/tcp", "10250/tcp"}
	if p.Network == "" || p.Network == "vxlan" {
		ports = append(ports, "8472/udp")
	}
	if p.Cluster {
		ports = append(ports, "2379/tcp", "2380/tcp")
	}
	if p.OpenPorts != nil {
		ports = append(ports, p.OpenPorts...)
	}

	missing := map[string][]string{}
	opened := map[string]bool{}
	for _, allowPorts := range firewall.Allowed {
		for _, allowedPort := range allowPorts.Ports {
			opened[fmt.Sprintf("%s/%s", allowedPort, allowPorts.IPProtocol)] = true
		}
	}

	for _, openPort := range ports {
		if !opened[openPort] {
			port, protocol := splitPortProto(openPort)
			missing[protocol] = append(missing[protocol], port)
		}
	}

	return missing
}

func (p *Google) waitForGlobalOp(name string) error {
	return p.waitForOp(func() (*raw.Operation, error) {
		return p.client.GlobalOperations.Get(p.Project, name).Do()
	})
}

func (p *Google) waitForRegionalOp(name string) error {
	return p.waitForOp(func() (*raw.Operation, error) {
		return p.client.ZoneOperations.Get(p.Project, p.Zone, name).Do()
	})
}

func (p *Google) waitForOp(opGetter func() (*raw.Operation, error)) error {
	for {
		op, err := opGetter()
		if err != nil {
			return err
		}

		p.Logger.Debugf("Operation %q status: %s", op.Name, op.Status)
		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("[%s] wating for operation %q for status %s error: %v", p.GetProviderName(), op.Name, op.Status, *op.Error.Errors[0])
			}
			break
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func (p *Google) generateLabels(master bool) map[string]string {
	defaultLabels := map[string]string{
		"autok3s": "true",
		"master":  strconv.FormatBool(master),
		"cluster": common.TagClusterPrefix + p.formatContextName(),
	}
	if p.Tags != nil {
		for _, additionalLabel := range p.Tags {
			ss := strings.Split(additionalLabel, "=")
			if len(ss) != 2 {
				p.Logger.Warnf("[%s] --tags value %v is not valid", p.GetProviderName(), additionalLabel)
				continue
			}
			defaultLabels[ss[0]] = ss[1]
		}
	}

	return defaultLabels
}

func (p *Google) getDisk(diskName string) (*raw.Disk, error) {
	return p.client.Disks.Get(p.Project, p.Zone, diskName).Do()
}

func (p *Google) instance(name string) (*raw.Instance, error) {
	return p.client.Instances.Get(p.Project, p.Zone, name).Do()
}

func (p *Google) uploadKeyPair(instance *raw.Instance, sshKeyPath string) error {
	p.Logger.Infof("[%s] uploading ssh key...", p.GetProviderName())
	sshKey, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return err
	}

	metaDataValue := fmt.Sprintf("%s:%s %s\n", p.SSHUser, strings.TrimSpace(string(sshKey)), p.SSHUser)

	meta := instance.Metadata
	if meta.Items == nil {
		meta.Items = []*raw.MetadataItems{}
	}
	meta.Items = append(meta.Items, &raw.MetadataItems{
		Key:   "sshKeys",
		Value: &metaDataValue,
	})

	op, err := p.client.Instances.SetMetadata(p.Project, p.Zone, instance.Name, meta).Do()
	if err != nil {
		return err
	}

	return p.waitForRegionalOp(op.Name)
}

func (p *Google) deleteInstance(instanceName string) error {
	p.Logger.Infof("[%s] remove instance...", p.GetProviderName())
	op, err := p.client.Instances.Delete(p.Project, p.Zone, instanceName).Do()
	if err != nil {
		return err
	}

	p.Logger.Infof("[%s] waiting for instance %s to delete...", p.GetProviderName(), instanceName)
	return p.waitForRegionalOp(op.Name)
}

func (p *Google) deleteDisk(instanceName string) error {
	diskName := fmt.Sprintf("%s-disk", instanceName)
	disk, _ := p.getDisk(diskName)
	if disk == nil {
		return nil
	}

	p.Logger.Infof("[%s] deleting disk for instance %s", p.GetProviderName(), instanceName)
	op, err := p.client.Disks.Delete(p.Project, p.Zone, diskName).Do()
	if err != nil {
		return err
	}

	p.Logger.Infof("[%s] waiting for disk to delete", p.GetProviderName())
	return p.waitForRegionalOp(op.Name)
}

func (p *Google) describeInstances() ([]*raw.Instance, error) {
	insList, err := p.client.Instances.List(p.Project, p.Zone).Filter(fmt.Sprintf("labels.autok3s=true AND %s",
		fmt.Sprintf("labels.cluster=%s", common.TagClusterPrefix+p.formatContextName()))).Do()
	if err != nil {
		return nil, err
	}
	// TODO: add paginating
	instanceList := make([]*raw.Instance, 0)
	if insList != nil {
		if len(insList.Items) > 0 {
			instanceList = append(instanceList, insList.Items...)
		}
	}
	return instanceList, err
}

func (p *Google) getInstanceNodes() ([]types.Node, error) {
	if p.client == nil {
		if err := p.newClient(); err != nil {
			return nil, err
		}
	}
	instanceList, err := p.describeInstances()
	if err != nil || len(instanceList) == 0 {
		return nil, fmt.Errorf("[%s] there's no instance for cluster %s: %v", p.GetProviderName(), p.ContextName, err)
	}
	nodes := make([]types.Node, 0)
	for _, instance := range instanceList {
		master := false
		for key, value := range instance.Labels {
			if strings.EqualFold(key, "master") && strings.EqualFold(value, "true") {
				master = true
			}
		}
		networkInterface := instance.NetworkInterfaces[0]
		nodes = append(nodes, types.Node{
			Master:            master,
			RollBack:          false,
			InstanceID:        instance.Name,
			InstanceStatus:    instance.Status,
			InternalIPAddress: []string{networkInterface.NetworkIP},
			PublicIPAddress:   []string{networkInterface.AccessConfigs[0].NatIP}})
	}
	return nodes, nil
}

func (p *Google) formatContextName() string {
	return strings.ReplaceAll(p.ContextName, ".", "-")
}

func (p *Google) isInstanceRunning(state string) bool {
	return state == statusRunning
}

func splitPortProto(raw string) (port string, protocol string) {
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) == 1 {
		return parts[0], "tcp"
	}
	return parts[0], parts[1]
}
