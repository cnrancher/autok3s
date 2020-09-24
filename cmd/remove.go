package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	removeCmd = &cobra.Command{
		Use:     "remove",
		Short:   "Remove k3s node(s)",
		Example: `  autok3s remove --name cluster --node-names <node-name-a,node-name-b>`,
	}
	rmProvider = ""
	nodeNames  = ""
	rmForce    = false
	rmP        providers.Provider
)

func init() {
	removeCmd.Flags().StringVarP(&rmProvider, "provider", "p", rmProvider, "Provider is a module which provides an interface for managing cloud resources")
	removeCmd.Flags().StringVarP(&nodeNames, "node-names", "n", nodeNames, "Name of the nodes to be deleted, use commas to separate multiple nodes")
	removeCmd.Flags().BoolVarP(&rmForce, "force", "f", rmForce, "Force delete node(s)")
}

func RemoveCommand() *cobra.Command {

	pStr := common.FlagHackLookup("--provider")

	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			rmP = reg
		}

		removeCmd.Flags().AddFlagSet(rmP.GetCredentialFlags(removeCmd))
		removeCmd.Flags().AddFlagSet(rmP.GetRemoveFlags(removeCmd))
	}

	removeCmd.Run = func(cmd *cobra.Command, args []string) {
		common.BindPFlags(cmd, rmP)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		rmP.GenerateClusterName()

		if err := rmP.RemoveK3sNodes(nodeNames, rmForce); err != nil {
			logrus.Fatalln(err)
		}
	}

	return removeCmd
}
