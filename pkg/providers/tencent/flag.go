package tencent

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (p *Tencent) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	fs = append(fs, []types.Flag{
		{
			Name:     "ui",
			P:        &p.UI,
			V:        p.UI,
			Usage:    "Enable K3s UI.",
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
			Name:  "eip",
			P:     &p.PublicIPAssignedEIP,
			V:     p.PublicIPAssignedEIP,
			Usage: "Enable eip",
		},
	}...)

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
			}
		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
}

func (p *Tencent) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
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
			p.Status = matched.Status
			p.overwriteMetadata(matched)
			p.mergeOptions(*matched)
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
}

func (p *Tencent) GetSSHFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
			Required: true,
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
		},
	}

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
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
			p.Status = matched.Status
			p.mergeOptions(*matched)
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required && f.Name == "name" {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
}

func (p *Tencent) GetStartFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
			Required: true,
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
		},
	}

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
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
			p.overwriteMetadata(matched)
			p.mergeOptions(*matched)
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required && f.Name == "name" {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
}

func (p *Tencent) GetStopFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
			Required: true,
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
		},
	}

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
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
			p.overwriteMetadata(matched)
			p.mergeOptions(*matched)
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required && f.Name == "name" {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
}

func (p *Tencent) GetDeleteFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:  "name",
			P:     &p.Name,
			V:     p.Name,
			Usage: "Cluster name",
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
		},
	}

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
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
			p.Status = matched.Status
			p.mergeOptions(*matched)
			p.overwriteMetadata(matched)
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required && f.Name == "name" {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}

		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
}

func (p *Tencent) GetCredentialFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     secretID,
			P:        &p.SecretID,
			V:        p.SecretID,
			Usage:    "User access key ID",
			Required: true,
		},
		{
			Name:     secretKey,
			P:        &p.SecretKey,
			V:        p.SecretKey,
			Usage:    "User access key secret",
			Required: true,
		},
	}

	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					cmd.Flags().StringVar(f.P.(*string), f.Name, t, f.Usage)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				switch t := f.V.(type) {
				case bool:
					cmd.Flags().BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					cmd.Flags().StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				}
			}
		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				p, ok := f.P.(*string)
				if ok {
					if *p == "" && f.V.(string) == "" {
						errFlags = append(errFlags, f.Name)
					}
				}
			}
		}
		if len(errFlags) == 0 {
			return nil
		}

		return fmt.Errorf("required flags(s) \"%s\" not set", errFlags)
	}

	return cmd.Flags()
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
	p.Repo = matched.Repo
	p.Network = matched.Network
	// needed to be overwrite.
	if p.K3sChannel == "" {
		p.K3sChannel = matched.K3sChannel
	}
	if p.K3sVersion == "" {
		p.K3sVersion = matched.K3sVersion
	}
	if p.Registries == "" {
		p.Registries = matched.Registries
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
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
			Required: true,
		},
		{
			Name:     "region",
			P:        &p.Region,
			V:        p.Region,
			Usage:    "Region is physical locations (data centers) that spread all over the world to reduce the network latency",
			Required: true,
		},
		{
			Name:     "zone",
			P:        &p.Zone,
			V:        p.Zone,
			Usage:    "Zone is physical areas with independent power grids and networks within one region. e.g.(ap-beijing-1)",
			Required: true,
		},
		{
			Name:     "vpc",
			P:        &p.VpcID,
			V:        p.VpcID,
			Usage:    "Private network id",
			Required: true,
		},
		{
			Name:     "subnet",
			P:        &p.SubnetID,
			V:        p.SubnetID,
			Usage:    "Private network subnet id",
			Required: true,
		},
		{
			Name:  "key-pair",
			P:     &p.KeyIds,
			V:     p.KeyIds,
			Usage: "Used to connect to an instance",
		},
		{
			Name:  "password",
			P:     &p.Password,
			V:     p.Password,
			Usage: "Used to connect to an instance",
		},
		{
			Name:     "image",
			P:        &p.ImageID,
			V:        p.ImageID,
			Usage:    "Used to specify the image to be used by the instance",
			Required: true,
		},
		{
			Name:     "type",
			P:        &p.InstanceType,
			V:        p.InstanceType,
			Usage:    "Used to specify the type to be used by the instance",
			Required: true,
		},
		{
			Name:     "disk-category",
			P:        &p.SystemDiskType,
			V:        p.SystemDiskType,
			Usage:    "Used to specify the system disk category used by the instance",
			Required: true,
		},
		{
			Name:     "disk-size",
			P:        &p.SystemDiskSize,
			V:        p.SystemDiskSize,
			Usage:    "Used to specify the system disk size used by the instance",
			Required: true,
		},
		{
			Name:     "security-group",
			P:        &p.SecurityGroupIds,
			V:        p.SecurityGroupIds,
			Usage:    "Used to specify the security group used by the instance",
			Required: true,
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
			Usage: "Specify K3s master/lb ip",
		},
		{
			Name:  "k3s-version",
			P:     &p.K3sVersion,
			V:     p.K3sVersion,
			Usage: "Used to specify the version of k3s cluster, overrides k3s-channel",
		},
		{
			Name:     "k3s-channel",
			P:        &p.K3sChannel,
			V:        p.K3sChannel,
			Usage:    "Used to specify the release channel of k3s. e.g.(stable, latest, or i.e. v1.18)",
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
			Name:  "registries",
			P:     &p.Registries,
			V:     p.Registries,
			Usage: "K3s registries, use commas to separate multiple entries",
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
