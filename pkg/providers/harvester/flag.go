package harvester

import (
	"encoding/json"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types"
	harvestertypes "github.com/cnrancher/autok3s/pkg/types/harvester"
	"github.com/cnrancher/autok3s/pkg/utils"
)

const createUsageExample = `  autok3s -d create \
    --provider harvester \
    --name <cluster name> \
    --kubeconfig-file ./config.yaml \
	--image-name <harvester image> \
    --network-name <vlan network name> \
    --master 1
`

const joinUsageExample = `  autok3s -d join \
    --provider harvester \
    --name <cluster name> \
    --worker 1
`

const deleteUsageExample = `  autok3s -d delete \
    --provider harvester \
    --name <cluster name> 
`

const sshUsageExample = `  autok3s ssh \
    --provider harvester \
    --name <cluster name> 
`

// GetUsageExample returns harvester usage example prompt.
func (h *Harvester) GetUsageExample(action string) string {
	switch action {
	case "create":
		return createUsageExample
	case "join":
		return joinUsageExample
	case "delete":
		return deleteUsageExample
	case "ssh":
		return sshUsageExample
	default:
		return ""
	}
}

// GetCreateFlags returns harvester create flags.
func (h *Harvester) GetCreateFlags() []types.Flag {
	cSSH := h.GetSSHConfig()
	h.SSH = *cSSH
	fs := h.GetClusterOptions()
	fs = append(fs, h.GetCreateOptions()...)
	return fs
}

// GetSSHConfig returns harvester ssh config.
func (h *Harvester) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		SSHUser: defaultUser,
		SSHPort: "22",
	}
	return ssh
}

// GetOptionFlags returns harvester option flags.
func (h *Harvester) GetOptionFlags() []types.Flag {
	return h.sharedFlags()
}

// GetDeleteFlags returns harvester option flags.
func (h *Harvester) GetDeleteFlags() []types.Flag {
	return []types.Flag{
		{
			Name:      "name",
			P:         &h.Name,
			V:         h.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
	}
}

// GetJoinFlags returns harvester join flags.
func (h *Harvester) GetJoinFlags() []types.Flag {
	fs := h.sharedFlags()
	fs = append(fs, h.GetClusterOptions()...)
	return fs
}

// GetSSHFlags returns harvester ssh flags.
func (h *Harvester) GetSSHFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:      "name",
			P:         &h.Name,
			V:         h.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
	}
	fs = append(fs, h.GetSSHOptions()...)
	return fs
}

// GetCredentialFlags return harvester credential flags.
func (h *Harvester) GetCredentialFlags() []types.Flag {
	return nil
}

// BindCredential bind harvester credential.
func (h *Harvester) BindCredential() error {
	return nil
}

// MergeClusterOptions merge harvester cluster options.
func (h *Harvester) MergeClusterOptions() error {
	opt, err := h.MergeConfig()
	if err != nil {
		return err
	}
	stateOption, err := h.GetProviderOptions(opt)
	if err != nil {
		return err
	}
	option := stateOption.(*harvestertypes.Options)

	// merge options.
	source := reflect.ValueOf(&h.Options).Elem()
	target := reflect.ValueOf(option).Elem()
	utils.MergeConfig(source, target)

	return nil
}

// GetProviderOptions get provider options.
func (h *Harvester) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &harvestertypes.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

func (h *Harvester) sharedFlags() []types.Flag {
	return []types.Flag{
		{
			Name:   "kubeconfig-content",
			P:      &h.KubeConfigContent,
			V:      h.KubeConfigContent,
			Usage:  "contents of kubeconfig file for harvester cluster, base64 is supported",
			EnvVar: "HARVESTER_KUBECONFIG_CONTENT",
		},
		{
			Name:   "kubeconfig-file",
			P:      &h.KubeConfigFile,
			V:      h.KubeConfigFile,
			Usage:  "kubeconfig file path for harvester cluster",
			EnvVar: "HARVESTER_KUBECONFIG_FILE",
		},
		{
			Name:   "vm-namespace",
			P:      &h.VMNamespace,
			V:      h.VMNamespace,
			Usage:  "harvester vm namespace",
			EnvVar: "HARVESTER_VM_NAMESPACE",
		},
		{
			Name:   "cpu-count",
			P:      &h.CPUCount,
			V:      h.CPUCount,
			Usage:  "number of CPUs for machine",
			EnvVar: "HARVESTER_CPU_COUNT",
		},
		{
			Name:   "memory-size",
			P:      &h.MemorySize,
			V:      h.MemorySize,
			Usage:  "size of memory for machine, e.g. --memory-size 1Gi",
			EnvVar: "HARVESTER_MEMORY_SIZE",
		},
		{
			Name:   "disk-size",
			P:      &h.DiskSize,
			V:      h.DiskSize,
			Usage:  "size of disk for machine, e.g. --disk-size 20Gi",
			EnvVar: "HARVESTER_DISK_SIZE",
		},
		{
			Name:   "disk-bus",
			P:      &h.DiskBus,
			V:      h.DiskBus,
			Usage:  "bus of disk for machine",
			EnvVar: "HARVESTER_DISK_BUS",
		},
		{
			Name:   "image-name",
			P:      &h.ImageName,
			V:      h.ImageName,
			Usage:  "harvester image name",
			EnvVar: "HARVESTER_IMAGE_NAME",
		},
		{
			Name:   "keypair-name",
			P:      &h.KeypairName,
			V:      h.KeypairName,
			Usage:  "harvester keypair name",
			EnvVar: "HARVESTER_KEY_PAIR_NAME",
		},
		{
			Name:   "network-type",
			P:      &h.NetworkType,
			V:      h.NetworkType,
			Usage:  "harvester network type",
			EnvVar: "HARVESTER_NETWORK_TYPE",
		},
		{
			Name:   "network-name",
			P:      &h.NetworkName,
			V:      h.NetworkName,
			Usage:  "harvester network name",
			EnvVar: "HARVESTER_NETWORK_NAME",
		},
		{
			Name:   "network-model",
			P:      &h.NetworkModel,
			V:      h.NetworkModel,
			Usage:  "harvester network model",
			EnvVar: "HARVESTER_NETWORK_MODEL",
		},
		{
			Name:  "interface-type",
			P:     &h.InterfaceType,
			V:     h.InterfaceType,
			Usage: "harvester network interface type, include bridge, masquerade",
		},
		{
			Name:   "network-data",
			P:      &h.NetworkData,
			V:      h.NetworkData,
			Usage:  "networkData content of cloud-init for machine, base64 is supported",
			EnvVar: "HARVESTER_NETWORK_DATA",
		},
		{
			Name:   "user-data",
			P:      &h.UserData,
			V:      h.UserData,
			Usage:  "userData content of cloud-init for machine, base64 is supported",
			EnvVar: "HARVESTER_USER_DATA",
		},
	}
}
