package alibaba

import (
	"errors"
	"fmt"

	"github.com/Jason-ZW/autok3s/pkg/types"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (p *Alibaba) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			ShortHand: "n",
			Usage:     "Cluster name.",
			Required:  true,
		},
		{
			Name:      "region",
			P:         &p.Region,
			V:         p.Region,
			ShortHand: "r",
			Usage:     "Regions are physical locations (data centers) that spread all over the world to reduce the network latency",
			Required:  true,
		},
		{
			Name:      "keyPairName",
			P:         &p.KeyPairName,
			V:         p.KeyPairName,
			ShortHand: "k",
			Usage:     "KeyPairName is used to connect to an instance",
			Required:  true,
		},
		{
			Name:      "imageID",
			P:         &p.ImageID,
			V:         p.ImageID,
			ShortHand: "i",
			Usage:     "ImageID is used to specify the image to be used by the instance",
			Required:  true,
		},
		{
			Name:      "instanceType",
			P:         &p.InstanceType,
			V:         p.InstanceType,
			ShortHand: "t",
			Usage:     "InstanceType is used to specify the type to be used by the instance",
			Required:  true,
		},
		{
			Name:      "vSwitchID",
			P:         &p.VSwitchID,
			V:         p.VSwitchID,
			ShortHand: "v",
			Usage:     "VSwitchID is used to specify the vSwitch to be used by the instance",
			Required:  true,
		},
		{
			Name:     "diskCategory",
			P:        &p.DiskCategory,
			V:        p.DiskCategory,
			Usage:    "diskCategory is used to specify the system disk category used by the instance",
			Required: true,
		},
		{
			Name:     "diskSize",
			P:        &p.DiskSize,
			V:        p.DiskSize,
			Usage:    "diskSize is used to specify the system disk size used by the instance",
			Required: true,
		},
		{
			Name:      "securityGroupID",
			P:         &p.SecurityGroupID,
			V:         p.SecurityGroupID,
			ShortHand: "s",
			Usage:     "securityGroupID is used to specify the security group used by the instance",
			Required:  true,
		},
		{
			Name:      "InternetMaxBandwidthOut",
			P:         &p.InternetMaxBandwidthOut,
			V:         p.InternetMaxBandwidthOut,
			ShortHand: "o",
			Usage:     "internetMaxBandwidthOut is used to specify the maximum out flow of the instance internet",
			Required:  true,
		},
		{
			Name:      "master",
			P:         &p.Master,
			V:         p.Master,
			ShortHand: "m",
			Usage:     "Number of master node",
			Required:  true,
		},
		{
			Name:      "worker",
			P:         &p.Worker,
			V:         p.Worker,
			ShortHand: "w",
			Usage:     "Number of worker node",
			Required:  true,
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

func (p *Alibaba) GetCredentialFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     accessKeyID,
			P:        &p.AccessKeyID,
			V:        p.AccessKeyID,
			Usage:    "User access key ID.",
			Required: true,
		},
		{
			Name:     accessKeySecret,
			P:        &p.AccessKeySecret,
			V:        p.AccessKeySecret,
			Usage:    "User access key secret.",
			Required: true,
		},
	}

	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVar(f.P, f.Name, f.V, f.Usage)
				nfs.StringVar(f.P, f.Name, f.V, f.Usage)
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				cmd.Flags().StringVarP(f.P, f.Name, f.ShortHand, f.V, f.Usage)
				nfs.StringVarP(f.P, f.Name, f.ShortHand, f.V, f.Usage)
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

	return nfs
}
