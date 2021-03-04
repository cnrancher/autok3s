package cmd

import (
	"fmt"

	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a K3s cluster",
	}
	dProvider = ""
	force     = false
	dp        providers.Provider
)

func init() {
	deleteCmd.Flags().StringVarP(&dProvider, "provider", "p", dProvider, "Provider is a module which provides an interface for managing cloud resources")
	deleteCmd.Flags().BoolVarP(&force, "force", "f", force, "Force delete cluster")
}

func DeleteCommand() *cobra.Command {
	pStr := common.FlagHackLookup("--provider")

	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			dp = reg
		}

		deleteCmd.Flags().AddFlagSet(utils.ConvertFlags(deleteCmd, dp.GetCredentialFlags()))
		deleteCmd.Flags().AddFlagSet(dp.GetDeleteFlags(deleteCmd))
		deleteCmd.Example = dp.GetUsageExample("delete")
		deleteCmd.Use = fmt.Sprintf("delete -p %s", pStr)
	}

	deleteCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if dProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}
		common.InitPFlags(cmd, dp)
		err := dp.MergeClusterOptions()
		if err != nil {
			return err
		}

		return common.MakeSureCredentialFlag(cmd.Flags(), dp)
	}

	deleteCmd.Run = func(cmd *cobra.Command, args []string) {
		dp.GenerateClusterName()

		if err := dp.DeleteK3sCluster(force); err != nil {
			logrus.Fatalln(err)
		}
	}

	return deleteCmd
}
