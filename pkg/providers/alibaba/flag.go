package alibaba

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/cluster"
	"github.com/Jason-ZW/autok3s/pkg/types"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (p *Alibaba) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()

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

		return errors.New(fmt.Sprintf("required flags(s) \"%s\" not set\n", errFlags))
	}

	return cmd.Flags()
}

func (p *Alibaba) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()

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
			if c.Provider == p.Provider && c.Name == p.Name {
				matched = &c
			}
		}

		if matched != nil {
			// join command need merge status & token value.
			p.Status = matched.Status
			p.Token = matched.Token
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

		return errors.New(fmt.Sprintf("required flags(s) \"%s\" not set\n", errFlags))
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

		return errors.New(fmt.Sprintf("required flags(s) \"%s\" not set\n", errFlags))
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

	for i := 0; i < source.NumField(); i++ {
		for _, k := range target.MapKeys() {
			if strings.Contains(source.Type().Field(i).Tag.Get("yaml"), k.String()) {
				source.Field(i).SetString(fmt.Sprintf("%s", target.MapIndex(k)))
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
			Name:      "key-pair",
			P:         &p.KeyPair,
			V:         p.KeyPair,
			Usage:     "Used to connect to an instance",
			Required:  true,
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
