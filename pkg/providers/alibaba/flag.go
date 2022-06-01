package alibaba

import (
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"
)

const createUsageExample = `  autok3s -d create \
    --provider alibaba \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --master 1
`

const joinUsageExample = `  autok3s -d join \
    --provider alibaba \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --master 1
`

const deleteUsageExample = `  autok3s -d delete \
    --provider alibaba \
    --name <cluster name>
    --access-key <access-key> \
    --access-secret <access-secret>
`

const sshUsageExample = `  autok3s ssh \
    --provider alibaba \
    --name <cluster name> \
    --region <region> \
    --access-key <access-key> \
    --access-secret <access-secret>
`

// GetUsageExample returns alibaba usage example prompt.
func (p *Alibaba) GetUsageExample(action string) string {
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

// GetCreateFlags returns alibaba create flags.
func (p *Alibaba) GetCreateFlags() []types.Flag {
	cSSH := p.GetSSHConfig()
	p.SSH = *cSSH
	fs := p.GetClusterOptions()
	fs = append(fs, p.GetCreateOptions()...)
	return fs
}

// GetOptionFlags returns alibaba option flags.
func (p *Alibaba) GetOptionFlags() []types.Flag {
	return p.sharedFlags()
}

// GetDeleteFlags returns alibaba delete flags.
func (p *Alibaba) GetDeleteFlags() []types.Flag {
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
			Usage:  "ECS region",
			EnvVar: "ECS_REGION",
		},
	}
}

// MergeClusterOptions merge alibaba cluster options.
func (p *Alibaba) MergeClusterOptions() error {
	opt, err := p.MergeConfig()
	if err != nil {
		return err
	}
	if opt != nil {
		stateOption, err := p.GetProviderOptions(opt)
		if err != nil {
			return err
		}
		option := stateOption.(*alibaba.Options)
		p.CloudControllerManager = option.CloudControllerManager

		// merge options.
		source := reflect.ValueOf(&p.Options).Elem()
		target := reflect.ValueOf(option).Elem()
		utils.MergeConfig(source, target)
	}
	return nil
}

// GetJoinFlags returns alibaba join flags.
func (p *Alibaba) GetJoinFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, p.GetClusterOptions()...)
	return fs
}

// GetSSHFlags returns alibaba ssh flags.
func (p *Alibaba) GetSSHFlags() []types.Flag {
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
			Usage:  "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			EnvVar: "ECS_REGION",
		},
	}
	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

// GetCredentialFlags returns alibaba credential flags.
func (p *Alibaba) GetCredentialFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:     accessKeyID,
			P:        &p.AccessKey,
			V:        p.AccessKey,
			Usage:    "User access key ID",
			Required: true,
			EnvVar:   "ECS_ACCESS_KEY_ID",
		},
		{
			Name:     accessKeySecret,
			P:        &p.AccessSecret,
			V:        p.AccessSecret,
			Usage:    "User access key secret",
			Required: true,
			EnvVar:   "ECS_ACCESS_KEY_SECRET",
		},
	}

	return fs
}

// GetSSHConfig returns alibaba ssh config.
func (p *Alibaba) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		SSHUser: defaultUser,
		SSHPort: "22",
	}
	return ssh
}

// BindCredential bind alibaba credential.
func (p *Alibaba) BindCredential() error {
	secretMap := map[string]string{
		accessKeyID:     p.AccessKey,
		accessKeySecret: p.AccessSecret,
	}
	return p.SaveCredential(secretMap)
}

func (p *Alibaba) sharedFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "ECS region",
			EnvVar: "ECS_REGION",
		},
		{
			Name:   "zone",
			P:      &p.Zone,
			V:      p.Zone,
			Usage:  "ECS zone",
			EnvVar: "ECS_ZONE",
		},
		{
			Name:   "key-pair",
			P:      &p.KeyPair,
			V:      p.KeyPair,
			Usage:  "Used to connect to an instance, see: https://help.aliyun.com/document_detail/51792.html?spm=a2c4g.11186623.6.947.50d950c5Mr0XWg",
			EnvVar: "ECS_SSH_KEYPAIR",
		},
		{
			Name:   "image",
			P:      &p.Image,
			V:      p.Image,
			Usage:  "Specify the image to be used by the instance, see: https://help.aliyun.com/document_detail/25389.html?spm=a2c4g.11186623.6.764.5e063ebbsJtMNf",
			EnvVar: "ECS_IMAGE_ID",
		},
		{
			Name:   "instance-type",
			P:      &p.InstanceType,
			V:      p.InstanceType,
			Usage:  "Specify the type of VM instance, see: https://help.aliyun.com/document_detail/25378.html?spm=a2c4g.11186623.6.605.455c6da7QzI8xc",
			EnvVar: "ECS_INSTANCE_TYPE",
		},
		{
			Name:   "v-switch",
			P:      &p.VSwitch,
			V:      p.VSwitch,
			Usage:  "Specify the vSwitch to be used by the instance, see: https://help.aliyun.com/document_detail/100380.html?spm=a2c4g.11186623.6.563.733b103bRTApHj",
			EnvVar: "ECS_VSWITCH_ID",
		},
		{
			Name:   "disk-category",
			P:      &p.DiskCategory,
			V:      p.DiskCategory,
			Usage:  "Specify the system disk category used by the instance, see: https://help.aliyun.com/document_detail/25383.htm?spm=a2c4g.11186623.2.8.24382763SzqaxO#concept-n1s-rzb-wdb",
			EnvVar: "ECS_DISK_CATEGORY",
		},
		{
			Name:   "disk-size",
			P:      &p.DiskSize,
			V:      p.DiskSize,
			Usage:  "Specify the system disk size used by the instance",
			EnvVar: "ECS_SYSTEM_DISK_SIZE",
		},
		{
			Name:   "security-group",
			P:      &p.SecurityGroup,
			V:      p.SecurityGroup,
			Usage:  "Specify the security group used by the instance, see: https://help.aliyun.com/document_detail/25387.html?spm=a2c4g.11186623.6.922.1f8d6c01V9Md8g",
			EnvVar: "ECS_SECURITY_GROUP",
		},
		{
			Name:  "internet-max-bandwidth-out",
			P:     &p.InternetMaxBandwidthOut,
			V:     p.InternetMaxBandwidthOut,
			Usage: "Specify the maximum out flow of the instance internet, see: https://help.aliyun.com/document_detail/25412.htm?spm=a2c4g.11186623.2.8.21f4bb57lQgHgE#BandwidthQuota1",
		},
		{
			Name:  "eip",
			P:     &p.EIP,
			V:     p.EIP,
			Usage: "Allocate EIP for instance, see: https://help.aliyun.com/document_detail/113775.html?spm=a2c4g.11186623.6.974.39323647OLWuwe",
		},
		{
			Name:  "tags",
			P:     &p.Tags,
			V:     p.Tags,
			Usage: "Set instance additional tags, i.e.(--tags a=b --tags b=c), see: https://help.aliyun.com/document_detail/25477.html?spm=a2c4g.11186623.6.1053.5fb621c6ENd1Hp",
		},
		{
			Name:  "cloud-controller-manager",
			P:     &p.CloudControllerManager,
			V:     p.CloudControllerManager,
			Usage: "Enable cloud-controller-manager component, for more information, please check https://github.com/kubernetes/cloud-provider-alibaba-cloud/blob/master/docs/getting-started.md",
		},
		{
			Name:  "terway",
			P:     &p.Terway,
			V:     p.Terway,
			Usage: "Enable terway CNI plugin, currently only support ENI mode. i.e.(--terway eni), see: https://github.com/AliyunContainerService/terway/blob/v1.0.10/docs/usage.md",
		},
		{
			Name:  "terway-max-pool-size",
			P:     &p.TerwayMaxPoolSize,
			V:     p.TerwayMaxPoolSize,
			Usage: "Max pool size for terway ENI mode",
		},
		{
			Name:  "user-data-path",
			P:     &p.UserDataPath,
			V:     p.UserDataPath,
			Usage: "file path of user data to make available to the ECS instance. For more information, see: https://help.aliyun.com/document_detail/108461.html",
		},
		{
			Name:  "user-data-content",
			P:     &p.UserDataContent,
			V:     p.UserDataContent,
			Usage: "user data content, must be base64-encoded text. For more information, see: https://help.aliyun.com/document_detail/108461.html",
		},
	}

	return fs
}
