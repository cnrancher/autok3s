package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/urlbuilder"
)

type ExplorerHandler struct {
}

// NewExplorerProxy return proxy handler for kube-explorer
func NewExplorerProxy() http.Handler {
	return &ExplorerHandler{}
}

// ServeHTTP handles the proxy request for kube-explorer
func (ep *ExplorerHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	clusterID := vars["name"]
	if clusterID == "" {
		rw.WriteHeader(http.StatusNotFound)
		_, _ = rw.Write([]byte("cluster context name can't be empty"))
		return
	}

	// get kube-explorer url
	explorer, err := common.DefaultDB.GetExplorer(clusterID)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(err.Error()))
		return
	}
	if explorer == nil || !explorer.Enabled {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = rw.Write([]byte(fmt.Sprintf("cluster %s is not enable kube-explorer", clusterID)))
		return
	}

	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/", explorer.Port))
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(err.Error()))
		return
	}
	prefix := fmt.Sprintf("/proxy/explorer/%s", clusterID)

	proxy := &httputil.ReverseProxy{}
	proxy.Director = func(req *http.Request) {
		scheme := urlbuilder.GetScheme(req)
		host := urlbuilder.GetHost(req, scheme)
		req.Header.Set(urlbuilder.ForwardedProtoHeader, scheme)
		req.Header.Set(urlbuilder.ForwardedHostHeader, host)
		req.Header.Set(urlbuilder.PrefixHeader, prefix)
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}
	proxy.ServeHTTP(rw, req)
}
