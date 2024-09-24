package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/gorilla/mux"
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

	u.Path = strings.TrimPrefix(req.URL.Path, fmt.Sprintf("/proxy/explorer/%s", clusterID))
	u.RawQuery = req.URL.RawQuery
	req.URL.Host = req.Host

	ph := RemoteHandler{
		Location: u,
	}
	ph.ServeHTTP(rw, req)
}
