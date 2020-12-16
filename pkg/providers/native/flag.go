package native

import (
	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (p *Native) GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	fs = append(fs, []types.Flag{
		{
			Name:  "ui",
			P:     &p.UI,
			V:     p.UI,
			Usage: "Enable K3s UI.",
		},
		{
			Name:  "repo",
			P:     &p.Repo,
			V:     p.Repo,
			Usage: "Specify helm repo",
		},
	}...)

	return utils.ConvertFlags(cmd, fs)
}

func (p *Native) GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := p.sharedFlags()
	return utils.ConvertFlags(cmd, fs)
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

	return utils.ConvertFlags(cmd, fs)
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
			Name:     "name",
			P:        &p.Name,
			V:        p.Name,
			Usage:    "Cluster name",
			Required: true,
		},
	}

	return utils.ConvertFlags(cmd, fs)
}

func (p *Native) MergeClusterOptions() error {
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
		p.overwriteMetadata(matched)
	}
	return nil
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
			Name:  "master-ips",
			P:     &p.MasterIps,
			V:     p.MasterIps,
			Usage: "Ips of master nodes",
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

func (p *Native) overwriteMetadata(matched *types.Cluster) {
	// doesn't need to be overwrite.
	p.Status = matched.Status
	p.Token = matched.Token
	p.IP = matched.IP
	p.DataStore = matched.DataStore
	p.Mirror = matched.Mirror
	p.DockerMirror = matched.DockerMirror
	p.InstallScript = matched.InstallScript
	// needed to be overwrite.
	if p.K3sChannel == "" {
		p.K3sChannel = matched.K3sChannel
	}
	if p.K3sVersion == "" {
		p.K3sVersion = matched.K3sVersion
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
