package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	stopCmd = &cobra.Command{
		Use:   "stop",
		Short: "Stop k3s cluster",
		Example: `  autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>`,
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
	}

	stopCmd.Run = func(cmd *cobra.Command, args []string) {
		if spProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}

		common.BindPFlags(cmd, spP)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		spP.GenerateClusterName()

		if err := spP.StopK3sCluster(spForce); err != nil {
			logrus.Fatalln(err)
		}
	}

	return stopCmd
}
