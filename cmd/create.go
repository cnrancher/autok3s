package cmd

import (
	"github.com/Jason-ZW/autok3s/cmd/common"
	"github.com/Jason-ZW/autok3s/pkg/providers"
	"github.com/Jason-ZW/autok3s/pkg/utils"

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
)

func init() {
	createCmd.Flags().StringVarP(&provider, "provider", "p", provider, "Provider is a module which provides an interface for managing cloud resources")
}

func CreateCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := utils.FlagHackLookup("--provider")
	if pStr != "" {
		p = providers.Register(pStr)
		createCmd.Flags().AddFlagSet(p.GetCredentialFlags())
		createCmd.Flags().AddFlagSet(p.GetCreateFlags())
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
		// must bind after dynamic provider flags loaded.
		common.BindPFlags(createCmd, p)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		p.CreateK3sCluster()
	}

	return createCmd
}
