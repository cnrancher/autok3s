package cmd

import (
	"github.com/Jason-ZW/autok3s/cmd/common"
	"github.com/Jason-ZW/autok3s/pkg/providers"

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
)

func init() {
	createCmd.Flags().StringVarP(&common.Provider, "provider", "p", common.Provider, "Provider is a module which provides an interface for managing cloud resources")
	createCmd.Flags().StringVar(&common.SSH.User, "user", common.SSH.User, "SSH user for host")
	createCmd.Flags().StringVar(&common.SSH.Port, "ssh-port", common.SSH.Port, "SSH port for host")
	createCmd.Flags().StringVar(&common.SSH.SSHKey, "ssh-key", common.SSH.SSHKey, "SSH private key path")
}

func CreateCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.Register(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			common.P = reg
		}

		createCmd.Flags().AddFlagSet(common.P.GetCredentialFlags(createCmd))
		createCmd.Flags().AddFlagSet(common.P.GetCreateFlags(createCmd))
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
		// must bind after dynamic provider flags loaded.
		common.BindPFlags(cmd, common.P)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		if err := common.P.CreateK3sCluster(common.SSH); err != nil {
			logrus.Fatalln(err)
		}
	}

	return createCmd
}
