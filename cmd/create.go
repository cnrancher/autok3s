package cmd

import (
	"github.com/Jason-ZW/autok3s/cmd/common"
	"github.com/Jason-ZW/autok3s/pkg/providers"
	"github.com/Jason-ZW/autok3s/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create k3s cluster",
		Example: `  autok3s create --provider alibaba`,
	}

	provider = ""
	p        providers.Provider

	ssh = &types.SSH{
		SSHKeyPath: "~/.ssh/id_rsa",
		User:       "root",
		Port:       "22",
	}
)

func init() {
	createCmd.Flags().StringVarP(&provider, "provider", "p", provider, "Provider is a module which provides an interface for managing cloud resources")
	createCmd.Flags().StringVar(&ssh.User, "sshUser", ssh.User, "SSH user for host")
	createCmd.Flags().StringVar(&ssh.Port, "sshPort", ssh.Port, "SSH port for host")
	createCmd.Flags().StringVar(&ssh.SSHKeyPath, "sshKeyPath", ssh.SSHKeyPath, "SSH private key path")
}

func CreateCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.Register(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			p = reg
		}

		createCmd.Flags().AddFlagSet(p.GetCredentialFlags(createCmd))
		createCmd.Flags().AddFlagSet(p.GetCreateFlags(createCmd))
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
		// must bind after dynamic provider flags loaded.
		common.BindPFlags(cmd, p)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		if err := p.CreateK3sCluster(ssh); err != nil {
			logrus.Fatalln(err)
		}
	}

	return createCmd
}
