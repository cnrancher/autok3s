package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start k3s cluster",
		Example: `  autok3s start \
    --provider alibaba \
    --name <cluster name> \
    --region <region> \
    --access-key <access-key> \
    --access-secret <access-secret>`,
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

	startCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if stProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}
		common.InitPFlags(cmd, stP)
		err := stP.MergeClusterOptions()
		if err != nil {
			return err
		}

		return common.MakeSureCredentialFlag(cmd.Flags(), stP)
	}

	startCmd.Run = func(cmd *cobra.Command, args []string) {
		stP.GenerateClusterName()

		if err := stP.StartK3sCluster(); err != nil {
			logrus.Fatalln(err)
		}
	}

	return startCmd
}
