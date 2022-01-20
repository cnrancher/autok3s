package harvester

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	putil "github.com/cnrancher/autok3s/pkg/providers/utils"
	"github.com/cnrancher/autok3s/pkg/types"
	harvestertypes "github.com/cnrancher/autok3s/pkg/types/harvester"
	"github.com/cnrancher/autok3s/pkg/utils"

	harvsterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/harvester/harvester/pkg/builder"
	harvesterutil "github.com/harvester/harvester/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
)

const (
	providerName = "harvester"

	defaultNamespace    = "default"
	defaultCPU          = 2
	defaultMemorySize   = "4Gi"
	defaultDiskSize     = "40Gi"
	defaultDiskBus      = "virtio"
	defaultNetworkModel = "virtio"
	networkTypePod      = "pod"
	networkTypeDHCP     = "dhcp"
	rootDiskName        = "disk-0"
	interfaceName       = "nic-0"

	defaultUser = "root"
)

type Harvester struct {
	*cluster.ProviderBase  `json:",inline"`
	harvestertypes.Options `json:",inline"`

	ctx    context.Context
	client *Client
}

func init() {
	providers.RegisterProvider(providerName, func() (providers.Provider, error) {
		return newProvider(), nil
	})
}

func newProvider() *Harvester {
	base := cluster.NewBaseProvider()
	base.Provider = providerName
	return &Harvester{
		ctx:          context.Background(),
		ProviderBase: base,
		Options: harvestertypes.Options{
			VMNamespace:   defaultNamespace,
			CPUCount:      defaultCPU,
			MemorySize:    defaultMemorySize,
			DiskSize:      defaultDiskSize,
			DiskBus:       defaultDiskBus,
			NetworkType:   networkTypeDHCP,
			NetworkModel:  defaultNetworkModel,
			InterfaceType: builder.NetworkInterfaceTypeBridge,
		},
	}
}

// GetProviderName returns provider name.
func (h *Harvester) GetProviderName() string {
	return h.Provider
}

// GenerateClusterName generates and returns cluster name.
func (h *Harvester) GenerateClusterName() string {
	h.ContextName = fmt.Sprintf("%s.%s.%s", h.Name, h.VMNamespace, h.GetProviderName())
	return h.ContextName
}

// GenerateManifest generates manifest deploy command.
func (h *Harvester) GenerateManifest() []string {
	return nil
}

// CreateK3sCluster create K3S cluster.
func (h *Harvester) CreateK3sCluster() (err error) {
	if h.SSHUser == "" {
		h.SSHUser = defaultUser
	}
	return h.InitCluster(h.Options, h.GenerateManifest, h.generateInstance, nil, h.removeInstances)
}

// JoinK3sNode join K3S node.
func (h *Harvester) JoinK3sNode() (err error) {
	if h.SSHUser == "" {
		h.SSHUser = defaultUser
	}
	return h.JoinNodes(h.generateInstance, h.syncInstances, false, h.removeInstances)
}

// DeleteK3sCluster delete K3S cluster.
func (h *Harvester) DeleteK3sCluster(f bool) (err error) {
	return h.DeleteCluster(f, h.deleteInstance)
}

// SSHK3sNode ssh K3s node.
func (h *Harvester) SSHK3sNode(ip string) error {
	c := &types.Cluster{
		Metadata: h.Metadata,
		Options:  h.Options,
		Status:   h.Status,
	}
	return h.Connect(ip, &h.SSH, c, h.getInstanceNodes, h.isInstanceRunning, nil)
}

// IsClusterExist determine if the cluster exists.
func (h *Harvester) IsClusterExist() (bool, []string, error) {
	ids := make([]string, 0)

	vmiList, err := h.getVMIByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
	if err != nil || vmiList == nil || len(vmiList.Items) == 0 {
		return false, nil, err
	}

	for _, vmi := range vmiList.Items {
		ids = append(ids, vmi.Name)
	}

	return len(ids) > 0, ids, nil
}

// GenerateMasterExtraArgs generates K3S master extra args.
func (h *Harvester) GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string {
	return ""
}

// GenerateWorkerExtraArgs generates K3S worker extra args.
func (h *Harvester) GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string {
	return h.GenerateMasterExtraArgs(cluster, worker)
}

// SetOptions set options.
func (h *Harvester) SetOptions(opt []byte) error {
	sourceOption := reflect.ValueOf(&h.Options).Elem()
	option := &harvestertypes.Options{}
	err := json.Unmarshal(opt, option)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(option).Elem()
	utils.MergeConfig(sourceOption, targetOption)
	return nil
}

// GetCluster returns cluster status.
func (h *Harvester) GetCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		ID:       h.ContextName,
		Name:     h.Name,
		Provider: h.GetProviderName(),
	}
	if kubecfg == "" {
		return c
	}

	return h.GetClusterStatus(kubecfg, c, h.getInstanceNodes)
}

// DescribeCluster describe cluster info.
func (h *Harvester) DescribeCluster(kubecfg string) *types.ClusterInfo {
	c := &types.ClusterInfo{
		Name:     h.Name,
		Provider: h.GetProviderName(),
	}
	return h.Describe(kubecfg, c, h.getInstanceNodes)
}

// SetConfig set cluster config.
func (h *Harvester) SetConfig(config []byte) error {
	c, err := h.SetClusterConfig(config)
	if err != nil {
		return err
	}
	sourceOption := reflect.ValueOf(&h.Options).Elem()
	b, err := json.Marshal(c.Options)
	if err != nil {
		return err
	}
	opt := &harvestertypes.Options{}
	err = json.Unmarshal(b, opt)
	if err != nil {
		return err
	}
	targetOption := reflect.ValueOf(opt).Elem()
	utils.MergeConfig(sourceOption, targetOption)

	return nil
}

// CreateCheck check create command and flags.
func (h *Harvester) CreateCheck() error {
	if h.KubeConfigFile == "" && h.KubeConfigContent == "" {
		return fmt.Errorf("[%s] calling preflight error: must set --kubeconfig-file or --kubeconfig-content", h.GetProviderName())
	}
	if h.ImageName == "" {
		return fmt.Errorf("[%s] calling preflight error: must specify harvester image name", h.GetProviderName())
	}
	if h.KeypairName != "" && h.SSHKeyPath == "" {
		return fmt.Errorf("[%s] calling preflight error: must specify the ssh private key path of the harvester key pair", h.GetProviderName())
	}

	masterNum, err := strconv.Atoi(h.Master)
	if masterNum < 1 || err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--master` number must >= 1",
			h.GetProviderName())
	}
	if masterNum > 1 && !h.Cluster && h.DataStore == "" {
		return fmt.Errorf("[%s] calling preflight error: need to set `--cluster` or `--datastore` when `--master` number > 1",
			h.GetProviderName())
	}

	if strings.Contains(h.MasterExtraArgs, "--datastore-endpoint") && h.DataStore != "" {
		return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
			h.GetProviderName())
	}

	// check name exist.
	state, err := common.DefaultDB.GetCluster(h.Name, h.Provider)
	if err != nil {
		return err
	}

	if state != nil && state.Status != common.StatusFailed {
		return fmt.Errorf("[%s] cluster %s is already exist", h.GetProviderName(), h.Name)
	}

	exist, _, err := h.IsClusterExist()
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("[%s] calling preflight error: cluster `%s` is already exist",
			h.GetProviderName(), h.Name)
	}

	switch h.NetworkType {
	case networkTypePod:
	case networkTypeDHCP:
		if h.NetworkName == "" {
			return fmt.Errorf("[%s] calling preflight error: must specify harvester network name if using network type as dhcp", h.GetProviderName())
		}
		// check network exist
		if _, err := h.getNetwork(); err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("[%s] calling preflight error: network %s doesn't exist", h.GetProviderName(), h.NetworkName)
			}
			return err
		}
	default:
		return fmt.Errorf("[%s] calling preflight error: unknown network type %s", h.GetProviderName(), h.NetworkType)
	}

	if _, err := h.getImage(); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("[%s] calling preflight error: image %s doesn't exist", h.GetProviderName(), h.ImageName)
		}
		return err
	}

	if h.KeypairName != "" {
		if h.SSHKeyPath == "" {
			return fmt.Errorf("[%s] calling preflight error: ssh-key-path can't be empty for keypair-name %s", h.GetProviderName(), h.KeypairName)
		}
		keypair, err := h.getKeyPair()
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("keypair %s doesn't exist", h.KeypairName)
			}
			return err
		}

		// keypair validated
		keypairValidated := false
		for _, condition := range keypair.Status.Conditions {
			if condition.Type == harvsterv1.KeyPairValidated && condition.Status == corev1.ConditionTrue {
				keypairValidated = true
			}
		}
		if !keypairValidated {
			return fmt.Errorf("keypair %s is not validated", keypair.Name)
		}

		h.SSHPublicKey = keypair.Spec.PublicKey
	}

	return nil
}

// JoinCheck check join command and flags.
func (h *Harvester) JoinCheck() error {
	// check cluster exist.
	exist, _, err := h.IsClusterExist()

	if err != nil {
		return err
	}

	if !exist {
		return fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist",
			h.GetProviderName(), h.ContextName)
	}

	// check flags.
	if strings.Contains(h.MasterExtraArgs, "--datastore-endpoint") && h.DataStore != "" {
		return fmt.Errorf("[%s] calling preflight error: `--masterExtraArgs='--datastore-endpoint'` is duplicated with `--datastore`",
			h.GetProviderName())
	}

	masterNum, err := strconv.Atoi(h.Master)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--master` must be number",
			h.GetProviderName())
	}
	workerNum, err := strconv.Atoi(h.Worker)
	if err != nil {
		return fmt.Errorf("[%s] calling preflight error: `--worker` must be number",
			h.GetProviderName())
	}
	if masterNum < 1 && workerNum < 1 {
		return fmt.Errorf("[%s] calling preflight error: `--master` or `--worker` number must >= 1", h.GetProviderName())
	}

	return nil
}

func (h *Harvester) generateInstance(ssh *types.SSH) (*types.Cluster, error) {
	masterNum, _ := strconv.Atoi(h.Master)
	workerNum, _ := strconv.Atoi(h.Worker)

	h.Logger.Infof("[%s] %d masters and %d workers will be added to cluster %s", h.GetProviderName(), masterNum, workerNum, h.Name)
	if ssh.SSHPassword == "" && ssh.SSHKeyPath == "" {
		if err := h.createKeypair(ssh); err != nil {
			return nil, err
		}
	}

	if masterNum > 0 {
		h.Logger.Infof("[%s] prepare for %d of master instances", h.GetProviderName(), masterNum)
		if err := h.runInstances(masterNum, true, ssh); err != nil {
			return nil, err
		}
		h.Logger.Infof("[%s] %d of master instances created successfully", h.GetProviderName(), masterNum)
	}

	if workerNum > 0 {
		h.Logger.Infof("[%s] prepare for %d for worker instances", h.GetProviderName(), workerNum)
		if err := h.runInstances(workerNum, false, ssh); err != nil {
			return nil, err
		}
		h.Logger.Infof("[%s] %d of worker instances created successfully", h.GetProviderName(), workerNum)
	}

	// wait for VM running
	if err := h.getInstanceStatus("Running"); err != nil {
		return nil, err
	}
	// wait for VM instance IP
	if err := h.waitForInstanceIP(); err != nil {
		return nil, err
	}

	c := &types.Cluster{
		Metadata: h.Metadata,
		Options:  h.Options,
		Status:   h.Status,
	}
	c.ContextName = h.ContextName
	c.SSH = *ssh

	return c, nil
}

func (h *Harvester) runInstances(num int, master bool, ssh *types.SSH) error {
	for i := 0; i < num; i++ {
		vmName := fmt.Sprintf("autok3s-%s-%s", h.Name, rand.String(5))
		// create vm
		cloudInitSource, cloudConfigSecret, err := h.buildCloudInit(vmName)
		if err != nil {
			return err
		}
		imageNamespace, imageName, err := NamespacedNamePartsByDefault(h.ImageName, h.VMNamespace)
		if err != nil {
			return err
		}
		pvcOption := &builder.PersistentVolumeClaimOption{
			ImageID:          fmt.Sprintf("%s/%s", imageNamespace, imageName),
			VolumeMode:       corev1.PersistentVolumeBlock,
			AccessMode:       corev1.ReadWriteMany,
			StorageClassName: pointer.StringPtr(builder.BuildImageStorageClassName("", imageName)),
		}
		vmBuilder := builder.NewVMBuilder("autok3s-harvester-provider").
			Namespace(h.VMNamespace).Name(vmName).CPU(h.CPUCount).Memory(h.MemorySize).
			Labels(h.setVMLabels(master)).
			PVCDisk(rootDiskName, builder.DiskBusVirtio, false, false, 1, h.DiskSize, "", pvcOption).
			CloudInitDisk(builder.CloudInitDiskName, builder.DiskBusVirtio, false, 0, *cloudInitSource).
			EvictionStrategy(true).DefaultPodAntiAffinity().Run(false)

		if h.KeypairName != "" {
			vmBuilder = vmBuilder.SSHKey(h.KeypairName)
		}

		networkName := h.NetworkName
		if h.NetworkType == networkTypePod {
			networkName = ""
		}
		vm, err := vmBuilder.NetworkInterface(interfaceName, h.NetworkModel, "", h.InterfaceType, networkName).VM()
		if err != nil {
			return err
		}
		vm.Kind = kubevirtv1.VirtualMachineGroupVersionKind.Kind
		vm.APIVersion = kubevirtv1.GroupVersion.String()
		h.Logger.Debugf("[%s] Generated VM %++v", h.GetProviderName(), vm)
		createdVM, err := h.createVM(vm)
		if err != nil {
			return err
		}
		// create secret
		if cloudConfigSecret != nil {
			cloudConfigSecret.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: vm.APIVersion,
					Kind:       vm.Kind,
					Name:       vm.Name,
					UID:        createdVM.UID,
				},
			}
			if _, err = h.createSecret(cloudConfigSecret); err != nil {
				return err
			}
		}

		if err := h.putVMSubResource(vmName, "start"); err != nil {
			return err
		}

		h.M.Store(vmName,
			types.Node{Master: master,
				Current:        true,
				RollBack:       true,
				InstanceID:     vmName,
				InstanceStatus: "Pending",
				SSH:            *ssh})
	}
	return nil
}

func (h *Harvester) setVMLabels(master bool) map[string]string {
	return map[string]string{
		"autok3s": "true",
		"cluster": common.TagClusterPrefix + h.ContextName,
		"master":  strconv.FormatBool(master),
	}
}

func (h *Harvester) getInstanceStatus(aimStatus string) error {
	h.Logger.Infof("[%s] waiting for the vm to be in `%s` status...", h.GetProviderName(), aimStatus)

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		vmList, err := h.getVMByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
		if err != nil || vmList == nil || len(vmList.Items) == 0 {
			return false, err
		}
		vmiList, err := h.getVMIByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
		if err != nil || vmiList == nil || len(vmiList.Items) == 0 {
			return false, err
		}

		for _, vmi := range vmiList.Items {
			if getStateFormVMI(&vmi) == aimStatus {
				if value, ok := h.M.Load(vmi.Name); ok {
					v := value.(types.Node)
					v.InstanceStatus = aimStatus
					h.M.Store(vmi.Name, v)
				}
				continue
			}
			return false, nil
		}

		return true, nil
	}); err != nil {
		return err
	}

	h.Logger.Infof("[%s] instances are in `%s` status", h.GetProviderName(), aimStatus)

	return nil
}

func (h *Harvester) waitForInstanceIP() error {
	h.Logger.Infof("[%s] waiting for VM Instance IP", h.GetProviderName())

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		vmList, err := h.getVMByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
		if err != nil || vmList == nil || len(vmList.Items) == 0 {
			return false, err
		}
		vmiList, err := h.getVMIByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
		if err != nil || vmiList == nil || len(vmiList.Items) == 0 {
			return false, err
		}

		for _, vmi := range vmiList.Items {
			addr := strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
			if ip := net.ParseIP(addr); ip == nil || ip.To4() == nil {
				return false, nil
			}
			if addr != "" {
				if value, ok := h.M.Load(vmi.Name); ok {
					v := value.(types.Node)
					v.PublicIPAddress = []string{addr}
					v.InternalIPAddress = []string{addr}
					h.M.Store(vmi.Name, v)
				}
				continue
			}
			return false, nil
		}

		return true, nil
	}); err != nil {
		return err
	}

	return nil
}

func (h *Harvester) removeInstances(ids []string) error {
	if len(ids) > 0 {
		for _, id := range ids {
			vm, err := h.getVM(id)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
			removedPVCs := make([]string, 0, len(vm.Spec.Template.Spec.Volumes))
			for _, volume := range vm.Spec.Template.Spec.Volumes {
				if volume.PersistentVolumeClaim == nil {
					continue
				}
				removedPVCs = append(removedPVCs, volume.PersistentVolumeClaim.ClaimName)
			}
			vmCopy := vm.DeepCopy()
			vmCopy.Annotations[harvesterutil.RemovedPVCsAnnotationKey] = strings.Join(removedPVCs, ",")
			if _, err = h.updateVM(vmCopy); err != nil {
				return err
			}
			if err = h.deleteVM(id); err != nil {
				return err
			}
		}

		// wait removed
		return wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
			for _, id := range ids {
				_, err := h.getVM(id)
				if err != nil && apierrors.IsNotFound(err) {
					continue
				}
				return false, err
			}
			return true, nil
		})
	}
	return nil
}

func (h *Harvester) createKeypair(ssh *types.SSH) error {
	var pk []byte
	var err error
	if ssh.SSHKeyPath == "" {
		h.Logger.Debugf("[%s] creating new ssh key", h.GetProviderName())
		pk, err = putil.CreateKeyPair(ssh, h.GetProviderName(), h.ContextName, h.KeypairName)
	} else {
		if h.KeypairName != "" {
			h.Logger.Debugf("Using existing harvester key pair: %s", h.KeypairName)
			return nil
		}
		publicKeyFile := ssh.SSHKeyPath + ".pub"
		pk, err = ioutil.ReadFile(publicKeyFile)
	}
	if err != nil {
		return err
	}
	h.SSHPublicKey = string(pk)
	return nil
}

func (h *Harvester) syncInstances() error {
	vmiList, err := h.getVMIByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
	if err != nil || vmiList == nil || len(vmiList.Items) == 0 {
		return fmt.Errorf("[%s] there's no instance for cluster %s: %v", h.GetProviderName(), h.ContextName, err)
	}

	for _, vmi := range vmiList.Items {
		if value, ok := h.M.Load(vmi.Name); ok {
			v := value.(types.Node)
			addr := strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
			if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
				v.PublicIPAddress = []string{addr}
				v.InternalIPAddress = []string{addr}
			}
			h.M.Store(vmi.Name, v)
			continue
		}
		master := false
		for key, value := range vmi.Labels {
			if strings.EqualFold("master", key) && strings.EqualFold("true", value) {
				master = true
				break
			}
		}
		node := types.Node{
			Master:         master,
			RollBack:       false,
			InstanceID:     vmi.Name,
			InstanceStatus: getStateFormVMI(&vmi),
		}
		addr := strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
		if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
			node.PublicIPAddress = []string{addr}
			node.InternalIPAddress = []string{addr}
		}
		h.M.Store(vmi.Name, node)
	}

	return nil
}

func (h *Harvester) getInstanceNodes() ([]types.Node, error) {
	vmiList, err := h.getVMIByLabel(fmt.Sprintf("autok3s=true,cluster=%s", common.TagClusterPrefix+h.ContextName))
	if err != nil || vmiList == nil || len(vmiList.Items) == 0 {
		return nil, fmt.Errorf("[%s] there's no instance for cluster %s: %v", h.GetProviderName(), h.ContextName, err)
	}
	nodes := make([]types.Node, 0)
	for _, vmi := range vmiList.Items {
		master := false
		for key, value := range vmi.Labels {
			if strings.EqualFold("master", key) && strings.EqualFold("true", value) {
				master = true
				break
			}
		}
		node := types.Node{
			Master:         master,
			RollBack:       false,
			InstanceID:     vmi.Name,
			InstanceStatus: getStateFormVMI(&vmi),
		}
		addr := strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
		if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
			node.PublicIPAddress = []string{addr}
			node.InternalIPAddress = []string{addr}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (h *Harvester) deleteInstance(f bool) (string, error) {
	h.GenerateClusterName()
	exist, ids, err := h.IsClusterExist()
	if err != nil {
		return "", fmt.Errorf("[%s] calling describe instance error, msg: %v", h.GetProviderName(), err)
	}
	if !exist {
		h.Logger.Errorf("[%s] cluster %s is not exist", h.GetProviderName(), h.Name)
		if !f {
			return "", fmt.Errorf("[%s] calling preflight error: cluster name `%s` do not exist", h.GetProviderName(), h.Name)
		}
		return h.ContextName, nil
	}
	if err = h.removeInstances(ids); err != nil {
		return "", err
	}
	h.Logger.Infof("[%s] successfully terminate instances for cluster %s", h.GetProviderName(), h.Name)
	return h.ContextName, nil
}

func (h *Harvester) isInstanceRunning(state string) bool {
	return state == "Running"
}

func getStateFormVMI(vmi *kubevirtv1.VirtualMachineInstance) string {
	switch vmi.Status.Phase {
	case "Pending", "Scheduling", "Scheduled":
		return "Starting"
	case "Running":
		return "Running"
	case "Succeeded":
		return "Stopping"
	case "Failed":
		return "Error"
	default:
		return "None"
	}
}

func stringSupportBase64(value string) string {
	if value == "" {
		return value
	}
	valueByte, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		valueByte = []byte(value)
	}
	return string(valueByte)
}
