package google

import (
	"encoding/json"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/google"
	"github.com/cnrancher/autok3s/pkg/utils"
)

const createUsageExample = `  autok3s -d create \
    --provider google \
    --name <cluster name> \
    --master 1
`

const joinUsageExample = `  autok3s -d join \
    --provider google \
    --name <cluster name> \
    --worker 1
`

const deleteUsageExample = `  autok3s -d delete \
    --provider google \
    --name <cluster name>
`

const sshUsageExample = `  autok3s ssh \
    --provider google \
    --name <cluster name> \
    --region <region>
`

// GetUsageExample return cli usage example for provider
func (p *Google) GetUsageExample(action string) string {
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

// GetCreateFlags returns google create flags.
func (p *Google) GetCreateFlags() []types.Flag {
	cSSH := p.GetSSHConfig()
	p.SSH = *cSSH
	fs := p.GetClusterOptions()
	fs = append(fs, p.GetCreateOptions()...)
	return fs
}

// GetSSHConfig returns google ssh config.
func (p *Google) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		SSHUser: defaultUser,
		SSHPort: "22",
	}
	return ssh
}

// GetOptionFlags returns google option flags.
func (p *Google) GetOptionFlags() []types.Flag {
	return p.sharedFlags()
}

// GetDeleteFlags returns google option flags.
func (p *Google) GetDeleteFlags() []types.Flag {
	return []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "GCE region",
			EnvVar: "GOOGLE_REGION",
		},
	}
}

// GetJoinFlags returns google join flags.
func (p *Google) GetJoinFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, p.GetClusterOptions()...)
	return fs
}

// GetSSHFlags returns google ssh flags.
func (p *Google) GetSSHFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "GCE region",
			EnvVar: "GOOGLE_REGION",
		},
	}
	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

// GetCredentialFlags return google credential flags.
func (p *Google) GetCredentialFlags() []types.Flag {
	return []types.Flag{
		{
			Name:     "service-account-file",
			P:        &p.ServiceAccountFile,
			V:        p.ServiceAccountFile,
			Usage:    "GCE service account json file for OAuth2 validation",
			EnvVar:   "GOOGLE_SERVICE_ACCOUNT_FILE",
			Required: true,
		},
		{
			Name:     "service-account",
			P:        &p.ServiceAccount,
			V:        p.ServiceAccount,
			Usage:    "GCE service account to attach to VM (email address)",
			EnvVar:   "GOOGLE_SERVICE_ACCOUNT",
			Required: true,
		},
	}
}

// BindCredential bind google credential.
func (p *Google) BindCredential() error {
	secretMap := map[string]string{
		"service-account-file": p.ServiceAccountFile,
		"service-account":      p.ServiceAccount,
	}
	return p.SaveCredential(secretMap)
}

// MergeClusterOptions merge google cluster options.
func (p *Google) MergeClusterOptions() error {
	opt, err := p.MergeConfig()
	if err != nil {
		return err
	}
	stateOption, err := p.GetProviderOptions(opt)
	if err != nil {
		return err
	}
	option := stateOption.(*google.Options)
	p.CloudControllerManager = option.CloudControllerManager

	// merge options
	source := reflect.ValueOf(&p.Options).Elem()
	target := reflect.ValueOf(option).Elem()
	utils.MergeConfig(source, target)

	return nil
}

// GetProviderOptions return Google Cloud Provider options.
func (p *Google) GetProviderOptions(opt []byte) (interface{}, error) {
	options := &google.Options{}
	err := json.Unmarshal(opt, options)
	return options, err
}

func (p *Google) sharedFlags() []types.Flag {
	return []types.Flag{
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "GCE region",
			EnvVar: "GOOGLE_REGION",
		},
		{
			Name:   "zone",
			P:      &p.Zone,
			V:      p.Zone,
			Usage:  "GCE zone",
			EnvVar: "GOOGLE_ZONE",
		},
		{
			Name:   "machine-type",
			P:      &p.MachineType,
			V:      p.MachineType,
			Usage:  "GCE machine type",
			EnvVar: "GOOGLE_MACHINE_TYPE",
		},
		{
			Name:   "machine-image",
			P:      &p.MachineImage,
			V:      p.MachineImage,
			Usage:  "GCE machine image url",
			EnvVar: "GOOGLE_MACHINE_IMAGE",
		},
		{
			Name:     "project",
			P:        &p.Project,
			V:        p.Project,
			Usage:    "GCE Project",
			EnvVar:   "GOOGLE_PROJECT",
			Required: true,
		},
		{
			Name:   "scopes",
			P:      &p.Scopes,
			V:      p.Scopes,
			Usage:  "GCE scopes",
			EnvVar: "GOOGLE_SCOPES",
		},
		{
			Name:   "disk-size",
			P:      &p.DiskSize,
			V:      p.DiskSize,
			Usage:  "GCE instance disk size (in GB)",
			EnvVar: "GOOGLE_DISK_SIZE",
		},
		{
			Name:   "disk-type",
			P:      &p.DiskType,
			V:      p.DiskType,
			Usage:  "GCE instance disk type",
			EnvVar: "GOOGLE_DISK_TYPE",
		},
		{
			Name:   "network",
			P:      &p.Network,
			V:      p.Network,
			Usage:  "Specify network in which to provision vm",
			EnvVar: "GOOGLE_NETWORK",
		},
		{
			Name:   "subnetwork",
			P:      &p.Subnetwork,
			V:      p.Subnetwork,
			Usage:  "Specify subnetwork in which to provision vm",
			EnvVar: "GOOGLE_SUBNETWORK",
		},
		{
			Name:   "use-internal-ip-only",
			P:      &p.UseInternalIPOnly,
			V:      p.UseInternalIPOnly,
			Usage:  "Configure GCE instance to not have an external IP address",
			EnvVar: "GOOGLE_USE_INTERNAL_IP_ONLY",
		},
		{
			Name:   "preemptible",
			P:      &p.Preemptible,
			V:      p.Preemptible,
			Usage:  "GCE Instance Preemptibility",
			EnvVar: "GOOGLE_PREEMPTIBLE",
		},
		{
			Name:  "tags",
			P:     &p.Tags,
			V:     p.Tags,
			Usage: "Set instance additional tags, i.e.(--tags a=b --tags b=c), see: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html",
		},
		{
			Name:  "open-ports",
			P:     &p.OpenPorts,
			V:     p.OpenPorts,
			Usage: "Make the specified port number accessible from the Internet, e.g, --open-ports 8080/tcp --open-ports 9090/tcp",
		},
		{
			Name:  "cloud-controller-manager",
			P:     &p.CloudControllerManager,
			V:     p.CloudControllerManager,
			Usage: "Enable gcp-cloud-provider component",
		},
	}
}
