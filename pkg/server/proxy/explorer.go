package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/urlbuilder"
)

type ExplorerHandler struct {
	next http.Handler
}

// NewExplorerProxy return proxy handler for kube-explorer
func NewExplorerProxy() http.Handler {
	return &ExplorerHandler{
		next: DynamicPrefixProxy(""),
	}
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

	ep.next.ServeHTTP(rw, req)
}

func DynamicPrefixProxy(staticClusterID string) http.Handler {
	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			DialContext: common.GetSocketDialer(),
		},
		Director: func(req *http.Request) {
			clusterID := staticClusterID
			if clusterID == "" {
				vars := mux.Vars(req)
				clusterID = vars["name"]
			}
			// prefix only used for non static cluster proxy
			var prefix string
			if staticClusterID == "" {
				prefix = fmt.Sprintf("/proxy/explorer/%s", clusterID)
			}
			scheme := urlbuilder.GetScheme(req)
			host := urlbuilder.GetHost(req, scheme)
			req.Header.Set(urlbuilder.ForwardedProtoHeader, scheme)
			req.Header.Set(urlbuilder.ForwardedHostHeader, host)
			req.URL.Scheme = scheme
			req.URL.Host = clusterID

			if prefix != "" {
				req.Header.Set(urlbuilder.PrefixHeader, prefix)
				req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}
		},
	}

	return proxy
}
