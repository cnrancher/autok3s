package cmd

import (
	"fmt"
	"net/http"

	"github.com/cnrancher/autok3s/pkg/server"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run as daemon and serve HTTP/HTTPS request",
	}

	bindPort    = "8080"
	bindAddress = "0.0.0.0"
)

func init() {
	serveCmd.Flags().StringVar(&bindPort, "bind-port", bindPort, "HTTP/HTTPS bind port")
	serveCmd.Flags().StringVar(&bindAddress, "bind-address", bindAddress, "HTTP/HTTPS bind address")
}

func ServeCommand() *cobra.Command {
	serveCmd.Run = func(cmd *cobra.Command, args []string) {
		router := server.Start()

		logrus.Infof("run as daemon, listening on %s:%s", bindAddress, bindPort)
		logrus.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", bindAddress, bindPort), router))
	}

	return serveCmd
}
