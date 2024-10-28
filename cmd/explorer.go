package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/server/proxy"

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
	explorerPort = 8080
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
	explorerCmd.Run = func(cmd *cobra.Command, _ []string) {
		if err := common.CheckCommandExist(common.KubeExplorerCommand); err != nil {
			logrus.Fatalln(err)
		}

		wait, err := common.StartKubeExplorer(cmd.Context(), clusterID)
		if err != nil {
			logrus.Fatalln(err)
		}

		server := http.Server{
			Addr:    fmt.Sprintf(":%d", explorerPort),
			Handler: proxy.DynamicPrefixProxy(clusterID),
			BaseContext: func(_ net.Listener) context.Context {
				return cmd.Context()
			},
		}
		go func() {
			logrus.Infof("autok3s serving kube-explorer on %s", server.Addr)
			_ = server.ListenAndServe()
		}()
		<-wait
		_ = server.Shutdown(context.Background())
	}

	return explorerCmd
}
