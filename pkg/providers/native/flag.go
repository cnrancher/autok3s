package native

import (
	"fmt"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (p *Native) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
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
				p, ok := f.P.(string)
				if ok {
					if p == "" && f.V.(string) == "" {
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

func (p *Native) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
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
			if c.Provider == p.Provider && c.Name == p.Name {
				matched = &c
			}
		}

		if matched != nil {
			// join command need merge status & token value.
			p.Status = matched.Status
			p.Token = matched.Token
			p.IP = matched.IP
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required {
				p, ok := f.P.(string)
				if ok {
					if p == "" && f.V.(string) == "" {
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

func (p *Native) GetSSHFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
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
			if c.Provider == p.Provider && c.Name == p.Name {
				matched = &c
			}
		}

		if matched != nil {
			// ssh command need merge status value.
			p.Status = matched.Status
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required && f.Name == "name" {
				p, ok := f.P.(string)
				if ok {
					if p == "" && f.V.(string) == "" {
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

func (p *Native) GetStopFlags(cmd *cobra.Command) *pflag.FlagSet {
	return cmd.Flags()
}

func (p *Native) GetStartFlags(cmd *cobra.Command) *pflag.FlagSet {
	return cmd.Flags()
}

func (p *Native) GetDeleteFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := []types.Flag{
		{
			Name:  "name",
			P:     &p.Name,
			V:     p.Name,
			Usage: "Cluster name",
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
			if c.Provider == p.Provider && c.Name == p.Name {
				matched = &c
			}
		}

		if matched != nil {
			// delete command need merge status value.
			p.Status = matched.Status
		}

		errFlags := make([]string, 0)
		for _, f := range fs {
			if f.Required && f.Name == "name" {
				p, ok := f.P.(string)
				if ok {
					if p == "" && f.V.(string) == "" {
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

func (p *Native) GetCredentialFlags(cmd *cobra.Command) *pflag.FlagSet {
	return cmd.Flags()
}

func (p *Native) BindCredentialFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)
	return nfs
}

func (p *Native) sharedFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
			Required: true,
		},
		{
			Name:  "ip",
			P:     &p.IP,
			V:     p.IP,
			Usage: "Specify K3s master/lb ip",
		},
		{
			Name:     "k3s-version",
			P:        &p.K3sVersion,
			V:        p.K3sVersion,
			Usage:    "Used to specify the version of k3s cluster",
			Required: true,
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
			Name:  "master",
			P:     &p.Master,
			V:     p.Master,
			Usage: "Number of master node",
		},
		{
			Name:  "master-ips",
			P:     &p.MasterIps,
			V:     p.MasterIps,
			Usage: "Ips of master nodes",
		},
		{
			Name:  "worker",
			P:     &p.Worker,
			V:     p.Worker,
			Usage: "Number of worker node",
		},
		{
			Name:  "worker-ips",
			P:     &p.WorkerIps,
			V:     p.WorkerIps,
			Usage: "Ips of worker nodes",
		},
	}

	return fs
}
