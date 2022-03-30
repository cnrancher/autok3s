package tencent

import (
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/tencent"
	"github.com/cnrancher/autok3s/pkg/utils"
)

const createUsageExample = `  autok3s -d create \
    --provider tencent \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key> \
    --master 1
`

const joinUsageExample = `  autok3s -d join \
    --provider tencent \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key> \
    --master 1
`

const deleteUsageExample = `  autok3s -d delete \
    --provider tencent \
    --name <cluster name>
    --secret-id <secret-id> \
    --secret-key <secret-key>
`

const sshUsageExample = `  autok3s ssh \
    --provider tencent \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
`

// GetUsageExample returns tencent usage example prompt.
func (p *Tencent) GetUsageExample(action string) string {
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

// GetCreateFlags returns tencent create flags.
func (p *Tencent) GetCreateFlags() []types.Flag {
	cSSH := p.GetSSHConfig()
	p.SSH = *cSSH
	fs := p.GetClusterOptions()
	fs = append(fs, p.GetCreateOptions()...)
	return fs
}

// GetOptionFlags returns tencent option flags.
func (p *Tencent) GetOptionFlags() []types.Flag {
	return p.sharedFlags()
}

// GetJoinFlags returns tencent join flags.
func (p *Tencent) GetJoinFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, p.GetClusterOptions()...)
	return fs
}

// GetSSHFlags returns tencent ssh flags.
func (p *Tencent) GetSSHFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Cluster name",
			ShortHand: "n",
			Required:  true,
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "CVM region",
			Required: true,
			EnvVar:   "CVM_REGION",
		},
	}
	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

// GetDeleteFlags returns tencent delete flags.
func (p *Tencent) GetDeleteFlags() []types.Flag {
	return []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Cluster name",
			ShortHand: "n",
			Required:  true,
		},
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "CVM region",
			EnvVar: "CVM_REGION",
		},
	}
}

// MergeClusterOptions merge tencent options.
func (p *Tencent) MergeClusterOptions() error {
	opt, err := p.MergeConfig()
	if err != nil {
		return err
	}
	if opt != nil {
		stateOption, err := p.GetProviderOptions(opt)
		if err != nil {
			return err
		}
		option := stateOption.(*tencent.Options)
		p.CloudControllerManager = option.CloudControllerManager

		// merge options.
		source := reflect.ValueOf(&p.Options).Elem()
		target := reflect.ValueOf(option).Elem()
		utils.MergeConfig(source, target)
	}
	return nil
}

// GetCredentialFlags returns tencent credential flags.
func (p *Tencent) GetCredentialFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:     secretID,
			P:        &p.SecretID,
			V:        p.SecretID,
			Usage:    "User access key ID",
			Required: true,
			EnvVar:   "CVM_SECRET_ID",
		},
		{
			Name:     secretKey,
			P:        &p.SecretKey,
			V:        p.SecretKey,
			Usage:    "User access key secret",
			Required: true,
			EnvVar:   "CVM_SECRET_KEY",
		},
	}

	return fs
}

// GetSSHConfig returns tencent ssh config.
func (p *Tencent) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		SSHUser: defaultUser,
		SSHPort: "22",
	}
	return ssh
}

// BindCredential bind tencent credential.
func (p *Tencent) BindCredential() error {
	secretMap := map[string]string{
		secretID:  p.SecretID,
		secretKey: p.SecretKey,
	}
	return p.SaveCredential(secretMap)
}

func (p *Tencent) sharedFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "CVM region",
			EnvVar: "CVM_REGION",
		},
		{
			Name:   "zone",
			P:      &p.Zone,
			V:      p.Zone,
			Usage:  "CVM zone",
			EnvVar: "CVM_ZONE",
		},
		{
			Name:   "vpc",
			P:      &p.VpcID,
			V:      p.VpcID,
			Usage:  "Private network id, see: https://cloud.tencent.com/document/product/215/20046",
			EnvVar: "CVM_VPC_ID",
		},
		{
			Name:   "subnet",
			P:      &p.SubnetID,
			V:      p.SubnetID,
			Usage:  "Private network subnet id, see: https://cloud.tencent.com/document/product/215/20046#.E5.AD.90.E7.BD.91",
			EnvVar: "CVM_SUBNET_ID",
		},
		{
			Name:   "keypair-id",
			P:      &p.KeypairID,
			V:      p.KeypairID,
			Usage:  "Used to connect to an instance, see: https://cloud.tencent.com/document/product/213/6092",
			EnvVar: "CVM_SSH_KEYPAIR",
		},
		{
			Name:  "image",
			P:     &p.ImageID,
			V:     p.ImageID,
			Usage: "Specify the image to be used by the instance, see: https://cloud.tencent.com/document/product/213/4941",
		},
		{
			Name:   "instance-type",
			P:      &p.InstanceType,
			V:      p.InstanceType,
			Usage:  "Specify the type of VM instance, see: https://cloud.tencent.com/document/product/213/11518",
			EnvVar: "CVM_INSTANCE_TYPE",
		},
		{
			Name:   "disk-category",
			P:      &p.SystemDiskType,
			V:      p.SystemDiskType,
			Usage:  "Specify the system disk category used by the instance, see: https://cloud.tencent.com/document/product/362/2353",
			EnvVar: "CVM_DISK_CATEGORY",
		},
		{
			Name:   "disk-size",
			P:      &p.SystemDiskSize,
			V:      p.SystemDiskSize,
			Usage:  "Specify the system disk size used by the instance",
			EnvVar: "CVM_DISK_SIZE",
		},
		{
			Name:   "security-group",
			P:      &p.SecurityGroupIds,
			V:      p.SecurityGroupIds,
			Usage:  "Specify the security group used by the instance, see: https://cloud.tencent.com/document/product/213/12452",
			EnvVar: "CVM_SECURITY_GROUP",
		},
		{
			Name:  "internet-max-bandwidth-out",
			P:     &p.InternetMaxBandwidthOut,
			V:     p.InternetMaxBandwidthOut,
			Usage: "Specify the maximum out flow of the instance internet, see: https://cloud.tencent.com/document/product/213/12523",
		},
		{
			Name:  "tags",
			P:     &p.Tags,
			V:     p.Tags,
			Usage: "Set instance additional tags, i.e.(--tags a=b --tags b=c), see: https://cloud.tencent.com/document/product/213/17131",
		},
		{
			Name:  "router",
			P:     &p.NetworkRouteTableName,
			V:     p.NetworkRouteTableName,
			Usage: "Network route table name for tencent cloud manager, must set with --cloud-controller-manager, see: https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl",
		},
		{
			Name:  "eip",
			P:     &p.PublicIPAssignedEIP,
			V:     p.PublicIPAssignedEIP,
			Usage: "Enable eip, see: https://cloud.tencent.com/document/product/213/5733",
		},
		{
			Name:  "cloud-controller-manager",
			P:     &p.CloudControllerManager,
			V:     p.CloudControllerManager,
			Usage: "Enable cloud-controller-manager component, for more information, please check https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/blob/master/docs/getting-started.md",
		},
	}

	return fs
}
