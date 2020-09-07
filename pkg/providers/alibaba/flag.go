package alibaba

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (p *Alibaba) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	fs = append(fs, []types.Flag{
		{
			Name:     "ui",
			P:        &p.UI,
			V:        p.UI,
			Usage:    "Specify K3s UI. e.g.(none/dashboard/octopus-ui)",
			Required: true,
		},
		{
			Name:     "repo",
			P:        &p.Repo,
			V:        p.Repo,
			Usage:    "Specify helm repo",
			Required: true,
		},
		{
			Name:     "terway",
			P:        &p.Terway.Mode,
			V:        p.Terway.Mode,
			Usage:    "Enable terway CNI plugin, currently only support ENI mode. e.g.(--terway eni)",
			Required: true,
		},
		{
			Name:     "terway-max-pool-size",
			P:        &p.Terway.MaxPoolSize,
			V:        p.Terway.MaxPoolSize,
			Usage:    "Max pool size for terway ENI mode",
			Required: true,
		},
	}...)

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVar(f.P, f.Name, f.V, f.Usage)
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVarP(f.P, f.Name, f.ShortHand, f.V, f.Usage)
			}
		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				if *f.P == "" && f.V == "" {
					errFlags = append(errFlags, f.Name)
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set\n", errFlags)
	}

	return cmd.Flags()
}

func (p *Alibaba) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()

	fs = append(fs, []types.Flag{
		{
			Name:     "url",
			P:        &p.URL,
			V:        p.URL,
			Usage:    "Specify K3s master URL",
			Required: true,
		},
	}...)

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVar(f.P, f.Name, f.V, f.Usage)
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVarP(f.P, f.Name, f.ShortHand, f.V, f.Usage)
			}
		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		clusters, err := cluster.ReadFromState(&types.Cluster{
			Metadata: p.Metadata,
			Options:  p.Options,
		})
		if err != nil {
			return err
		}

		var matched *types.Cluster
		for _, c := range clusters {
			if c.Provider == p.Provider && c.Name == fmt.Sprintf("%s.%s", p.Name, p.Region) {
				matched = &c
			}
		}

		if matched != nil {
			// join command need merge status & token value.
			p.Status = matched.Status
			p.Token = matched.Token
			p.URL = matched.URL
			p.mergeOptions(*matched)
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				if *f.P == "" && f.V == "" {
					errFlags = append(errFlags, f.Name)
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set\n", errFlags)
	}

	return cmd.Flags()
}

func (p *Alibaba) GetCredentialFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     accessKeyID,
			P:        &p.AccessKey,
			V:        p.AccessKey,
			Usage:    "User access key ID",
			Required: true,
		},
		{
			Name:     accessKeySecret,
			P:        &p.AccessSecret,
			V:        p.AccessSecret,
			Usage:    "User access key secret",
			Required: true,
		},
	}

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVar(f.P, f.Name, f.V, f.Usage)
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVarP(f.P, f.Name, f.ShortHand, f.V, f.Usage)
			}

		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				if *f.P == "" && f.V == "" {
					errFlags = append(errFlags, f.Name)
				}
			}
		}
		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set\n", errFlags)
	}

	return cmd.Flags()
}

func (p *Alibaba) BindCredentialFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)
	nfs.StringVar(&p.AccessKey, accessKeyID, p.AccessKey, "User access key ID")
	nfs.StringVar(&p.AccessSecret, accessKeySecret, p.AccessSecret, "User access key secret")
	return nfs
}

func (p *Alibaba) mergeOptions(input types.Cluster) {
	source := reflect.ValueOf(&p.Options).Elem()
	target := reflect.Indirect(reflect.ValueOf(&input.Options)).Elem()

	p.mergeValues(source, target)
}

func (p *Alibaba) mergeValues(source, target reflect.Value) {
	for i := 0; i < source.NumField(); i++ {
		for _, k := range target.MapKeys() {
			if strings.Contains(source.Type().Field(i).Tag.Get("yaml"), k.String()) {
				if source.Field(i).Kind().String() == "struct" {
					p.mergeValues(source.Field(i), target.MapIndex(k).Elem())
				} else {
					source.Field(i).SetString(fmt.Sprintf("%s", target.MapIndex(k)))
				}
			}
		}
	}
}

func (p *Alibaba) sharedFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name.",
			Required: true,
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "Physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
		},
		{
			Name:     "key-pair",
			P:        &p.KeyPair,
			V:        p.KeyPair,
			Usage:    "Used to connect to an instance",
			Required: true,
		},
		{
			Name:     "image",
			P:        &p.Image,
			V:        p.Image,
			Usage:    "Used to specify the image to be used by the instance",
			Required: true,
		},
		{
			Name:     "type",
			P:        &p.Type,
			V:        p.Type,
			Usage:    "Used to specify the type to be used by the instance",
			Required: true,
		},
		{
			Name:     "v-switch",
			P:        &p.VSwitch,
			V:        p.VSwitch,
			Usage:    "Used to specify the vSwitch to be used by the instance",
			Required: true,
		},
		{
			Name:     "disk-category",
			P:        &p.DiskCategory,
			V:        p.DiskCategory,
			Usage:    "Used to specify the system disk category used by the instance",
			Required: true,
		},
		{
			Name:     "disk-size",
			P:        &p.DiskSize,
			V:        p.DiskSize,
			Usage:    "Used to specify the system disk size used by the instance",
			Required: true,
		},
		{
			Name:     "security-group",
			P:        &p.SecurityGroup,
			V:        p.SecurityGroup,
			Usage:    "Used to specify the security group used by the instance",
			Required: true,
		},
		{
			Name:     "internet-max-bandwidth-out",
			P:        &p.InternetMaxBandwidthOut,
			V:        p.InternetMaxBandwidthOut,
			Usage:    "Used to specify the maximum out flow of the instance internet",
			Required: true,
		},
		{
			Name:     "cloud-controller-manager",
			P:        &p.CloudControllerManager,
			V:        p.CloudControllerManager,
			Usage:    "Enable cloud-controller-manager component",
			Required: true,
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
			Name:  "token",
			P:     &p.Token,
			V:     p.Token,
			Usage: "K3s master token, if empty will automatically generated",
		},
		{
			Name:     "master",
			P:        &p.Master,
			V:        p.Master,
			Usage:    "Number of master node",
			Required: true,
		},
		{
			Name:     "worker",
			P:        &p.Worker,
			V:        p.Worker,
			Usage:    "Number of worker node",
			Required: true,
		},
	}

	return fs
}
