package cmd

import (
	"github.com/cnrancher/autok3s/pkg/common"

	k3dutil "github.com/k3d-io/k3d/v5/cmd/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	explorerCmd = &cobra.Command{
		Use:     "explorer",
		Short:   "Enable kube-explorer for K3s cluster",
		Example: "autok3s explorer --context myk3s",
	}
	clusterID    = ""
	explorerPort = 0
)

func init() {
	explorerCmd.Flags().StringVarP(&clusterID, "context", "", clusterID, "Set context to start kube-explorer")
	explorerCmd.Flags().IntVarP(&explorerPort, "port", "", explorerPort, "Set http port for kube-explorer")
}

// ExplorerCommand will start a kube-explorer server for specified K3s cluster
func ExplorerCommand() *cobra.Command {
	explorerCmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if clusterID == "" {
			logrus.Fatalln("required flag(s) \"--context\" not set")
		}
		return nil
	}
	explorerCmd.Run = func(_ *cobra.Command, _ []string) {
		if err := common.CheckCommandExist(common.KubeExplorerCommand); err != nil {
			logrus.Fatalln(err)
		}
		if explorerPort == 0 {
			port, err := k3dutil.GetFreePort()
			if err != nil {
				logrus.Fatalf("failed to get free port for kube-explorer: %v", err)
			}
			explorerPort = port
		}
		_ = common.StartKubeExplorer(explorerCmd.Context(), clusterID, explorerPort)
	}

	return explorerCmd
}
