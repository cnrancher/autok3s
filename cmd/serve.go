package cmd

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/server"
	"github.com/rancher/apiserver/pkg/store/apiroot"
	"github.com/rancher/apiserver/pkg/types"
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
		s := server.DefaultAPIServer()

		apiroot.Register(s.Schemas, []string{"v1"})

		router := mux.NewRouter()
		router.Handle("/{prefix}/{type}", s)
		router.Handle("/{prefix}/{type}/{id}", s)

		router.NotFoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			s.Handle(&types.APIRequest{
				Request:   r,
				Response:  rw,
				Type:      "apiRoot",
				URLPrefix: "v1",
			})
		})

		logrus.Infof("run as daemon, listening on %s:%s", bindAddress, bindPort)
		logrus.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", bindAddress, bindPort), router))
	}

	return serveCmd
}
