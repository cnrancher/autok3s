package aws

import (
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/aws"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/pflag"
)

const createUsageExample = `  autok3s -d create \
    --provider aws \
    --name <cluster name> \
    --access-key <access-key> \
    --secret-key <secret-key> \
    --master 1
`

const joinUsageExample = `  autok3s -d join \
    --provider aws \
    --name <cluster name> \
    --access-key <access-key> \
    --secret-key <secret-key> \
    --worker 1
`

const deleteUsageExample = `  autok3s -d delete \
    --provider aws \
    --name <cluster name>
    --access-key <access-key> \
    --secret-key <secret-key> 
`

const sshUsageExample = `  autok3s ssh \
    --provider aws \
    --name <cluster name> \
    --region <region> \
    --access-key <access-key> \
    --secret-key <secret-key>
`

func (p *Amazon) GetUsageExample(action string) string {
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

func (p *Amazon) GetCreateFlags() []types.Flag {
	cSSH := p.GetSSHConfig()
	p.SSH = *cSSH
	fs := p.GetClusterOptions()
	fs = append(fs, p.GetCreateOptions()...)
	return fs
}

func (p *Amazon) GetOptionFlags() []types.Flag {
	return p.sharedFlags()
}

func (p *Amazon) GetDeleteFlags() []types.Flag {
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
			Usage:  "AWS region",
			EnvVar: "AWS_DEFAULT_REGION",
		},
	}
}

func (p *Amazon) GetJoinFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, p.GetClusterOptions()...)
	return fs
}

func (p *Amazon) GetSSHFlags() []types.Flag {
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
			Usage:  "AWS region",
			EnvVar: "AWS_DEFAULT_REGION",
		},
	}
	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

func (p *Amazon) GetCredentialFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:     "access-key",
			P:        &p.AccessKey,
			V:        p.AccessKey,
			Usage:    "AWS access key",
			Required: true,
			EnvVar:   "AWS_ACCESS_KEY_ID",
		},
		{
			Name:     "secret-key",
			P:        &p.SecretKey,
			V:        p.SecretKey,
			Usage:    "AWS secret key",
			Required: true,
			EnvVar:   "AWS_SECRET_ACCESS_KEY",
		},
	}

	return fs
}

func (p *Amazon) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		SSHUser: defaultUser,
		SSHPort: "22",
	}
	return ssh
}

func (p *Amazon) BindCredentialFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)
	nfs.StringVar(&p.AccessKey, "access-key", p.AccessKey, "AWS access key")
	nfs.StringVar(&p.SecretKey, "secret-key", p.SecretKey, "AWS secret key")
	return nfs
}

func (p *Amazon) MergeClusterOptions() error {
	opt, err := p.MergeConfig()
	if err != nil {
		return err
	}
	stateOption, err := p.GetProviderOptions(opt)
	if err != nil {
		return err
	}
	option := stateOption.(*aws.Options)
	p.CloudControllerManager = option.CloudControllerManager

	// merge options
	source := reflect.ValueOf(&p.Options).Elem()
	target := reflect.ValueOf(option).Elem()
	utils.MergeConfig(source, target)

	return nil
}

func (p *Amazon) sharedFlags() []types.Flag {
	return []types.Flag{
		{
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "AWS region",
			EnvVar: "AWS_DEFAULT_REGION",
		},
		{
			Name:   "zone",
			P:      &p.Zone,
			V:      p.Zone,
			Usage:  "AWS zone",
			EnvVar: "AWS_ZONE",
		},
		{
			Name:   "keypair-name",
			P:      &p.KeypairName,
			V:      p.KeypairName,
			Usage:  "AWS keypair to use connect to instance",
			EnvVar: "AWS_KEYPAIR_NAME",
		},
		{
			Name:   "ami",
			P:      &p.AMI,
			V:      p.AMI,
			Usage:  "Used to specify the image to be used by the instance",
			EnvVar: "AWS_AMI",
		},
		{
			Name:   "instance-type",
			P:      &p.InstanceType,
			V:      p.InstanceType,
			Usage:  "Specify the type of VM instance",
			EnvVar: "AWS_INSTANCE_TYPE",
		},
		{
			Name:   "vpc-id",
			P:      &p.VpcID,
			V:      p.VpcID,
			Usage:  "AWS VPC id",
			EnvVar: "AWS_VPC_ID",
		},
		{
			Name:   "subnet-id",
			P:      &p.SubnetID,
			V:      p.SubnetID,
			Usage:  "AWS VPC subnet id",
			EnvVar: "AWS_SUBNET_ID",
		},
		{
			Name:   "volume-type",
			P:      &p.VolumeType,
			V:      p.VolumeType,
			Usage:  "Specify the EBS volume type",
			EnvVar: "AWS_VOLUME_TYPE",
		},
		{
			Name:   "root-size",
			P:      &p.RootSize,
			V:      p.RootSize,
			Usage:  "Specify the root disk size used by the instance (in GB)",
			EnvVar: "AWS_ROOT_SIZE",
		},
		{
			Name:   "security-group",
			P:      &p.SecurityGroup,
			V:      p.SecurityGroup,
			Usage:  "Specify the security group used by the instance",
			EnvVar: "AWS_SECURITY_GROUP",
		},
		{
			Name:  "iam-instance-profile-control",
			P:     &p.IamInstanceProfileForControl,
			V:     p.IamInstanceProfileForControl,
			Usage: "AWS IAM Instance Profile for k3s control nodes to deploy AWS Cloud Provider, must set with --cloud-controller-manager",
		},
		{
			Name:  "iam-instance-profile-worker",
			P:     &p.IamInstanceProfileForWorker,
			V:     p.IamInstanceProfileForWorker,
			Usage: "AWS IAM Instance Profile for k3s worker nodes, must set with --cloud-controller-manager",
		},
		{
			Name:  "request-spot-instance",
			P:     &p.RequestSpotInstance,
			V:     p.RequestSpotInstance,
			Usage: "Request for spot instance",
		},
		{
			Name:  "spot-price",
			P:     &p.SpotPrice,
			V:     p.SpotPrice,
			Usage: "Spot instance bid price (in dollar)",
		},
		{
			Name:  "tags",
			P:     &p.Tags,
			V:     p.Tags,
			Usage: "Set instance additional tags, i.e.(--tags a=b,b=c)",
		},
		{
			Name:  "cloud-controller-manager",
			P:     &p.CloudControllerManager,
			V:     p.CloudControllerManager,
			Usage: "Enable cloud-controller-manager component",
		},
	}
}
