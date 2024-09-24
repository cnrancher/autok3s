package cmd

import (
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	upgradeCmd = &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade a K3s cluster to specified version",
	}
	uProvider     = ""
	clusterName   = ""
	channel       = ""
	version       = ""
	installScript = ""
	uPackageName  = ""
	uPackagePath  = ""
)

func init() {
	upgradeCmd.Flags().StringVarP(&uProvider, "provider", "p", uProvider, "Provider is a module which provides an interface for managing cloud resources")
	upgradeCmd.Flags().StringVarP(&clusterName, "name", "n", clusterName, "cluster name")
	upgradeCmd.Flags().StringVarP(&channel, "k3s-channel", "", channel, "Channel to use for fetching K3s download URL. Defaults to “stable”. Options include: stable, latest, testing")
	upgradeCmd.Flags().StringVarP(&version, "k3s-version", "", version, "Used to specify the version of k3s cluster, overrides k3s-channel")
	upgradeCmd.Flags().StringVarP(&installScript, "k3s-install-script", "", installScript, "Change the default upstream k3s install script address, see: https://docs.k3s.io/installation/configuration#options-for-installation-with-script")
	upgradeCmd.Flags().StringVarP(&uPackageName, "package-name", "", uPackageName, "Airgap package name which you want to upgrade k3s with")
	upgradeCmd.Flags().StringVarP(&uPackagePath, "package-path", "", uPackagePath, "Airgap package path which you want to upgrade k3s with")
}

// UpgradeCommand help upgrade a K3s cluster to specified version
func UpgradeCommand() *cobra.Command {
	upgradeCmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if clusterName == "" {
			logrus.Fatalln("`-n` or `--name` must set to specify a cluster, i.e. autok3s upgrade -n <cluster-name>")
		}
		if uProvider == "" {
			logrus.Fatalln("`-p` or `--provider` must set")
		}
		if uProvider == "k3d" {
			logrus.Fatalln("The upgrade cluster for K3d provider is not supported yet.")
		}
		return nil
	}
	upgradeCmd.Run = func(_ *cobra.Command, _ []string) {
		upgradeCluster()
	}
	return upgradeCmd
}

func upgradeCluster() {
	up, err := providers.GetProvider(uProvider)
	if err != nil {
		logrus.Fatalf("failed to get provider %v: %v", uProvider, err)
	}
	err = up.UpgradeK3sCluster(clusterName, installScript, channel, version, uPackageName, uPackagePath)
	if err != nil {
		logrus.Fatalf("[%s] failed to upgrade cluster %s, got error: %v", uProvider, clusterName, err)
	}
}
