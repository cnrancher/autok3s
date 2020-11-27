package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete k3s cluster",
		Example: `  autok3s delete \
    --provider alibaba \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>`,
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

		deleteCmd.Flags().AddFlagSet(dp.GetCredentialFlags(deleteCmd))
		deleteCmd.Flags().AddFlagSet(dp.GetDeleteFlags(deleteCmd))
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
