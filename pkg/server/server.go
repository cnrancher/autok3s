package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/server"
	"github.com/rancher/apiserver/pkg/store/apiroot"
	"github.com/rancher/apiserver/pkg/types"
)

func Start() http.Handler {
	s := server.DefaultAPIServer()
	apiroot.Register(s.Schemas, []string{"v1"})
	router := mux.NewRouter()
	router.UseEncodedPath()
	router.StrictSlash(true)

	router.Path("/{prefix}/{type}").Handler(s)
	router.Path("/{prefix}/{type}/{name}").Queries("link", "{link}").Handler(s)
	router.Path("/{prefix}/{type}/{name}").Queries("action", "{action}").Handler(s)
	router.Path("/{prefix}/{type}/{name}").Handler(s)

	router.NotFoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.Handle(&types.APIRequest{
			Request:   r,
			Response:  rw,
			Type:      "apiRoot",
			URLPrefix: "v1",
		})
	})

	return router
}
