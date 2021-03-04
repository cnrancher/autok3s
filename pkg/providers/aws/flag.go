package aws

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types/aws"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/cobra"
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

func (p *Amazon) GetOptionFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, []types.Flag{
		{
			Name:  "ui",
			P:     &p.UI,
			V:     p.UI,
			Usage: "Enable in-cluster kubernetes/dashboard",
		},
		{
			Name:  "cluster",
			P:     &p.Cluster,
			V:     p.Cluster,
			Usage: "Form k3s cluster using embedded etcd (Kubernetes version must be v1.19.x or higher)",
		},
	}...)
	return fs
}

func (p *Amazon) GetDeleteFlags(cmd *cobra.Command) *pflag.FlagSet {
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

	return utils.ConvertFlags(cmd, fs)
}

func (p *Amazon) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	return utils.ConvertFlags(cmd, fs)
}

func (p *Amazon) GetSSHFlags(cmd *cobra.Command) *pflag.FlagSet {
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

	return utils.ConvertFlags(cmd, fs)
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
		User: defaultUser,
		Port: "22",
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
	clusters, err := cluster.ReadFromState(&types.Cluster{
		Metadata: p.Metadata,
		Options:  p.Options,
	})
	if err != nil {
		return err
	}

	var matched *types.Cluster
	for _, c := range clusters {
		if c.Provider == p.Provider && c.Name == fmt.Sprintf("%s.%s.%s", p.Name, p.Region, p.Provider) {
			matched = &c
		}
	}

	if matched != nil {
		p.overwriteMetadata(matched)
		// delete command need merge status value.
		source := reflect.ValueOf(&p.Options).Elem()
		b, err := json.Marshal(matched.Options)
		if err != nil {
			return err
		}
		opt := &aws.Options{}
		err = json.Unmarshal(b, opt)
		if err != nil {
			return err
		}
		target := reflect.ValueOf(opt).Elem()
		utils.MergeConfig(source, target)
	}

	return nil
}

func (p *Amazon) overwriteMetadata(matched *types.Cluster) {
	// doesn't need to be overwrite.
	p.Status = matched.Status
	p.Token = matched.Token
	p.IP = matched.IP
	p.UI = matched.UI
	p.CloudControllerManager = matched.CloudControllerManager
	p.ClusterCIDR = matched.ClusterCIDR
	p.DataStore = matched.DataStore
	p.Mirror = matched.Mirror
	p.DockerMirror = matched.DockerMirror
	p.InstallScript = matched.InstallScript
	p.Network = matched.Network
	// needed to be overwrite.
	if p.K3sChannel == "" {
		p.K3sChannel = matched.K3sChannel
	}
	if p.K3sVersion == "" {
		p.K3sVersion = matched.K3sVersion
	}
	if p.InstallScript == "" {
		p.InstallScript = matched.InstallScript
	}
	if p.Registry == "" {
		p.Registry = matched.Registry
	}
	if p.MasterExtraArgs == "" {
		p.MasterExtraArgs = matched.MasterExtraArgs
	}
	if p.WorkerExtraArgs == "" {
		p.WorkerExtraArgs = matched.WorkerExtraArgs
	}
}

func (p *Amazon) sharedFlags() []types.Flag {
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
			Name:  "ip",
			P:     &p.IP,
			V:     p.IP,
			Usage: "Public IP of an existing k3s server",
		},
		{
			Name:  "k3s-version",
			P:     &p.K3sVersion,
			V:     p.K3sVersion,
			Usage: "Specify the version of k3s cluster, overrides k3s-channel",
		},
		{
			Name:  "k3s-channel",
			P:     &p.K3sChannel,
			V:     p.K3sChannel,
			Usage: "Specify the release channel of k3s. i.e.(stable, latest, or i.e. v1.18)",
		},
		{
			Name:  "k3s-install-script",
			P:     &p.InstallScript,
			V:     p.InstallScript,
			Usage: "Change the default upstream k3s install script address",
		},
		{
			Name:  "cloud-controller-manager",
			P:     &p.CloudControllerManager,
			V:     p.CloudControllerManager,
			Usage: "Enable cloud-controller-manager component",
		},
		{
			Name:  "master-extra-args",
			P:     &p.MasterExtraArgs,
			V:     p.MasterExtraArgs,
			Usage: "Master extra arguments for k3s installer, wrapped in quotes. i.e.(--master-extra-args '--no-deploy metrics-server')",
		},
		{
			Name:  "worker-extra-args",
			P:     &p.WorkerExtraArgs,
			V:     p.WorkerExtraArgs,
			Usage: "Worker extra arguments for k3s installer, wrapped in quotes. i.e.(--worker-extra-args '--node-taint key=value:NoExecute')",
		},
		{
			Name:  "registry",
			P:     &p.Registry,
			V:     p.Registry,
			Usage: "K3s registry file, see: https://rancher.com/docs/k3s/latest/en/installation/private-registry",
		},
		{
			Name:  "datastore",
			P:     &p.DataStore,
			V:     p.DataStore,
			Usage: "K3s datastore connection string to enable HA, i.e. \"mysql://username:password@tcp(hostname:3306)/database-name\"",
		},
		{
			Name:  "token",
			P:     &p.Token,
			V:     p.Token,
			Usage: "K3s master token, it will automatically generated if it is left as an empty field",
		},
		{
			Name:  "master",
			P:     &p.Master,
			V:     p.Master,
			Usage: "Number of master node",
		},
		{
			Name:  "worker",
			P:     &p.Worker,
			V:     p.Worker,
			Usage: "Number of worker node",
		},
	}

	return fs
}
