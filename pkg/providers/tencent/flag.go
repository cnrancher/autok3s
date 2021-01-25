package tencent

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

const startUsageExample = `  autok3s -d start \
    --provider tencent \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
`

const stopUsageExample = `  autok3s -d stop \
    --provider tencent \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
`

const sshUsageExample = `  autok3s ssh \
    --provider tencent \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
`

func (p *Tencent) GetUsageExample(action string) string {
	switch action {
	case "create":
		return createUsageExample
	case "join":
		return joinUsageExample
	case "delete":
		return deleteUsageExample
	case "start":
		return startUsageExample
	case "stop":
		return stopUsageExample
	case "ssh":
		return sshUsageExample
	default:
		return ""
	}
}

func (p *Tencent) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	fs = append(fs, []types.Flag{
		{
			Name:  "ui",
			P:     &p.UI,
			V:     p.UI,
			Usage: "Enable K3s UI.",
		},
		{
			Name:  "eip",
			P:     &p.PublicIPAssignedEIP,
			V:     p.PublicIPAssignedEIP,
			Usage: "Enable eip",
		},
		{
			Name:  "cluster",
			P:     &p.Cluster,
			V:     p.Cluster,
			Usage: "Form k3s cluster using embedded etcd (requires K8s >= 1.19)",
		},
	}...)

	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) GetSSHFlags(cmd *cobra.Command) *pflag.FlagSet {
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
			Usage:    "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
			EnvVar:   "CVM_REGION",
		},
	}

	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) GetStartFlags(cmd *cobra.Command) *pflag.FlagSet {
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
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			EnvVar: "CVM_REGION",
		},
	}

	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) GetStopFlags(cmd *cobra.Command) *pflag.FlagSet {
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
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			EnvVar: "CVM_REGION",
		},
	}

	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) GetDeleteFlags(cmd *cobra.Command) *pflag.FlagSet {
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
			Name:   "region",
			P:      &p.Region,
			V:      p.Region,
			Usage:  "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			EnvVar: "CVM_REGION",
		},
	}

	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) MergeClusterOptions() error {
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
		p.mergeOptions(*matched)
	}
	return nil
}

func (p *Tencent) GetCredentialFlags(cmd *cobra.Command) *pflag.FlagSet {
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

	return utils.ConvertFlags(cmd, fs)
}

func (p *Tencent) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		User: defaultUser,
		Port: "22",
	}
	return ssh
}

func (p *Tencent) BindCredentialFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)
	nfs.StringVar(&p.SecretID, secretID, p.SecretID, "User access key ID")
	nfs.StringVar(&p.SecretKey, secretKey, p.SecretKey, "User access key secret")
	return nfs
}

func (p *Tencent) mergeOptions(input types.Cluster) {
	source := reflect.ValueOf(&p.Options).Elem()
	target := reflect.Indirect(reflect.ValueOf(&input.Options)).Elem()

	p.mergeValues(source, target)
}

func (p *Tencent) mergeValues(source, target reflect.Value) {
	for i := 0; i < source.NumField(); i++ {
		for _, key := range target.MapKeys() {
			if strings.Contains(source.Type().Field(i).Tag.Get("yaml"), key.String()) {
				if source.Field(i).Kind().String() == "struct" {
					p.mergeValues(source.Field(i), target.MapIndex(key).Elem())
				} else {
					switch source.Field(i).Type().Name() {
					case "bool":
						source.Field(i).SetBool(target.MapIndex(key).Interface().(bool))
					default:
						source.Field(i).SetString(target.MapIndex(key).Interface().(string))
					}
				}
			}
		}
	}
}

func (p *Tencent) overwriteMetadata(matched *types.Cluster) {
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

func (p *Tencent) sharedFlags() []types.Flag {
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
			EnvVar: "CVM_REGION",
		},
		{
			Name:   "zone",
			P:      &p.Zone,
			V:      p.Zone,
			Usage:  "Zone is physical areas with independent power grids and networks within one region. e.g.(ap-beijing-1)",
			EnvVar: "CVM_ZONE",
		},
		{
			Name:   "vpc",
			P:      &p.VpcID,
			V:      p.VpcID,
			Usage:  "Private network id",
			EnvVar: "CVM_VPC_ID",
		},
		{
			Name:   "subnet",
			P:      &p.SubnetID,
			V:      p.SubnetID,
			Usage:  "Private network subnet id",
			EnvVar: "CVM_SUBNET_ID",
		},
		{
			Name:   "key-pair",
			P:      &p.KeyIds,
			V:      p.KeyIds,
			Usage:  "Used to connect to an instance",
			EnvVar: "CVM_SSH_KEYPAIR",
		},
		{
			Name:  "image",
			P:     &p.ImageID,
			V:     p.ImageID,
			Usage: "Used to specify the image to be used by the instance",
		},
		{
			Name:   "type",
			P:      &p.InstanceType,
			V:      p.InstanceType,
			Usage:  "Used to specify the type to be used by the instance",
			EnvVar: "CVM_INSTANCE_TYPE",
		},
		{
			Name:   "disk-category",
			P:      &p.SystemDiskType,
			V:      p.SystemDiskType,
			Usage:  "Used to specify the system disk category used by the instance",
			EnvVar: "CVM_DISK_CATEGORY",
		},
		{
			Name:   "disk-size",
			P:      &p.SystemDiskSize,
			V:      p.SystemDiskSize,
			Usage:  "Used to specify the system disk size used by the instance",
			EnvVar: "CVM_DISK_SIZE",
		},
		{
			Name:   "security-group",
			P:      &p.SecurityGroupIds,
			V:      p.SecurityGroupIds,
			Usage:  "Used to specify the security group used by the instance",
			EnvVar: "CVM_SECURITY_GROUP",
		},
		{
			Name:  "internet-max-bandwidth-out",
			P:     &p.InternetMaxBandwidthOut,
			V:     p.InternetMaxBandwidthOut,
			Usage: "Used to specify the maximum out flow of the instance internet",
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
			Usage: "Used to specify the version of k3s cluster, overrides k3s-channel",
		},
		{
			Name:  "k3s-channel",
			P:     &p.K3sChannel,
			V:     p.K3sChannel,
			Usage: "Used to specify the release channel of k3s. e.g.(stable, latest, or i.e. v1.18)",
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
			Usage: "Master extra arguments for k3s installer, wrapped in quotes. e.g.(--master-extra-args '--no-deploy metrics-server')",
		},
		{
			Name:  "worker-extra-args",
			P:     &p.WorkerExtraArgs,
			V:     p.WorkerExtraArgs,
			Usage: "Worker extra arguments for k3s installer, wrapped in quotes. e.g.(--worker-extra-args '--node-taint key=value:NoExecute')",
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
			Usage: "K3s datastore, HA mode `create/join` master node needed this flag",
		},
		{
			Name:  "token",
			P:     &p.Token,
			V:     p.Token,
			Usage: "K3s master token, if empty will automatically generated",
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
		{
			Name:  "router",
			P:     &p.NetworkRouteTableName,
			V:     p.NetworkRouteTableName,
			Usage: "Network route table name for tencent cloud manager, must set with --cloud-controller-manager",
		},
	}

	return fs
}
