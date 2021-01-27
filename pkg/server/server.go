package server

import (
	"net/http"
	"strings"

	"github.com/cnrancher/autok3s/pkg/server/ui"

	"github.com/gorilla/mux"
	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	"github.com/rancher/apiserver/pkg/server"
	"github.com/rancher/apiserver/pkg/store/apiroot"
	"github.com/rancher/apiserver/pkg/types"

	// pprof
	"net/http/pprof"
)

func Start() http.Handler {
	s := server.DefaultAPIServer()
	initMutual(s.Schemas)
	initProvider(s.Schemas)
	initCluster(s.Schemas)
	initCredential(s.Schemas)
	initKubeconfig(s.Schemas)
	initLogs(s.Schemas)
	apiroot.Register(s.Schemas, []string{"v1"})
	router := mux.NewRouter()
	router.UseEncodedPath()
	router.StrictSlash(true)

	middleware := responsewriter.Chain{
		responsewriter.Gzip,
		responsewriter.NoCache,
		responsewriter.DenyFrameOptions,
		responsewriter.ContentType,
		serve,
	}
	router.PathPrefix("/ui/").Handler(middleware.Handler(http.StripPrefix("/ui/", http.FileServer(ui.AssetFile()))))

	router.Path("/").HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, "/ui/", http.StatusFound)
	})

	// profiling handlers for pprof under /debug/pprof
	router.HandleFunc("/debug/pprof", pprof.Index)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)

	// Manually add support for paths linked to by index page at /debug/pprof/
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/debug/pprof/block", pprof.Handler("block"))

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

func serve(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/ui/")
		if _, err := ui.Asset(path); err != nil {
			b, _ := ui.Asset("index.html")
			rw.Write(b)
			return
		}
		next.ServeHTTP(rw, req)
	})
}
