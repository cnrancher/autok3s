package cmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/server"

	"github.com/pkg/browser"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run as daemon and serve HTTP/HTTPS request",
	}

	bindPort    = "8080"
	bindAddress = "127.0.0.1"
)

func init() {
	serveCmd.Flags().StringVar(&bindPort, "bind-port", bindPort, "HTTP/HTTPS bind port")
	serveCmd.Flags().StringVar(&bindAddress, "bind-address", bindAddress, "HTTP/HTTPS bind address")
}

// ServeCommand serve command.
func ServeCommand() *cobra.Command {
	serveCmd.Run = func(_ *cobra.Command, _ []string) {
		common.IsCLI = false
		router := server.Start()
		addr := fmt.Sprintf("%s:%s", bindAddress, bindPort)

		// start kube-explorer for K3s clusters
		go func(ctx context.Context) {
			common.InitExplorer(ctx)
		}(serveCmd.Context())
		// start helm-dashboard server
		go func(ctx context.Context) {
			common.InitDashboard(ctx)
		}(serveCmd.Context())

		stopChan := make(chan struct{})
		go func(c chan struct{}) {
			logrus.Infof("run as daemon, listening on %s:%s", bindAddress, bindPort)
			if err := http.ListenAndServe(addr, router); err != nil {
				logrus.Error(err)
			}
			close(c)
		}(stopChan)
		if err := browser.OpenURL("http://" + addr); err != nil {
			logrus.Warnf("failed to open browser to addr %s", addr)
		}
		<-stopChan
	}

	return serveCmd
}
