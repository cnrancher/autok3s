package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	startCmd = &cobra.Command{
		Use:     "start",
		Short:   "Start k3s cluster",
		Example: `  autok3s start --name cluster`,
	}
	stProvider = ""
	stP        providers.Provider
)

func init() {
	startCmd.Flags().StringVarP(&stProvider, "provider", "p", stProvider, "Provider is a module which provides an interface for managing cloud resources")
}

func StartCommand() *cobra.Command {

	pStr := common.FlagHackLookup("--provider")

	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			stP = reg
		}

		startCmd.Flags().AddFlagSet(stP.GetCredentialFlags(startCmd))
		startCmd.Flags().AddFlagSet(stP.GetStartFlags(startCmd))
	}

	startCmd.Run = func(cmd *cobra.Command, args []string) {
		common.BindPFlags(cmd, stP)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		stP.GenerateClusterName()

		if err := stP.StartK3sCluster(); err != nil {
			logrus.Fatalln(err)
		}
	}

	return startCmd
}
