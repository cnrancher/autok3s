package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	stopCmd = &cobra.Command{
		Use:   "stop",
		Short: "Stop k3s cluster",
	}
	spProvider = ""
	spForce    = false
	spP        providers.Provider
)

func init() {
	stopCmd.Flags().StringVarP(&spProvider, "provider", "p", spProvider, "Provider is a module which provides an interface for managing cloud resources")
	stopCmd.Flags().BoolVarP(&spForce, "force", "f", spForce, "Force stop cluster")
}

func StopCommand() *cobra.Command {
	pStr := common.FlagHackLookup("--provider")

	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			spP = reg
		}

		stopCmd.Flags().AddFlagSet(spP.GetCredentialFlags(stopCmd))
		stopCmd.Flags().AddFlagSet(spP.GetStopFlags(stopCmd))
		stopCmd.Example = spP.GetUsageExample("stop")
	}

	stopCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if spProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}
		common.InitPFlags(cmd, spP)
		err := spP.MergeClusterOptions()
		if err != nil {
			return err
		}

		return common.MakeSureCredentialFlag(cmd.Flags(), spP)
	}

	stopCmd.Run = func(cmd *cobra.Command, args []string) {
		spP.GenerateClusterName()

		if err := spP.StopK3sCluster(spForce); err != nil {
			logrus.Fatalln(err)
		}
	}

	return stopCmd
}
