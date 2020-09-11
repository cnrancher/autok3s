package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	deleteCmd = &cobra.Command{
		Use:     "delete",
		Short:   "Delete k3s cluster",
		Example: `  autok3s delete --name cluster`,
	}
	dProvider = ""
	remove = false
	dp        providers.Provider

)

func init() {
	deleteCmd.Flags().StringVarP(&dProvider, "provider", "p", dProvider, "Provider is a module which provides an interface for managing cloud resources")
	deleteCmd.Flags().BoolVarP(&remove, "remove", "r", remove,  "Force delete cluster")
}

func DeleteCommand() *cobra.Command {

	pStr := common.FlagHackLookup("--provider")

	if pStr != "" {
		if reg, err := providers.Register(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			dp = reg
		}

		deleteCmd.Flags().AddFlagSet(dp.GetCredentialFlags(deleteCmd))
		deleteCmd.Flags().AddFlagSet(dp.GetDeleteNodeFlags(deleteCmd))
	}

	deleteCmd.Run = func(cmd *cobra.Command, args []string) {
		common.BindPFlags(cmd, dp)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		dp.GenerateClusterName()

		if err := dp.DeleteK3sNode(remove); err != nil {
			logrus.Fatalln(err)
		}
	}

	return deleteCmd
}
