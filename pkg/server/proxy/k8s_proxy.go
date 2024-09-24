package proxy

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	k8sproxy "k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sProxyHandler struct{}

func NewK8sProxy() http.Handler {
	return &K8sProxyHandler{}
}

type errorResponder struct{}

func (r *errorResponder) Error(w http.ResponseWriter, _ *http.Request, err error) {
	logrus.Errorf("Error while proxying request: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (kh *K8sProxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	config := req.Header.Get("config")
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(utils.StringSupportBase64(config)))
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(err.Error()))
		return
	}
	host := cfg.Host
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}
	u, err := url.Parse(host)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(err.Error()))
		return
	}

	u.Path = strings.TrimPrefix(req.URL.Path, "/k8s/proxy")
	u.RawQuery = req.URL.RawQuery
	req.URL.Host = req.Host
	responder := &errorResponder{}
	transport, err := rest.TransportFor(cfg)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(err.Error()))
		return
	}
	handler := k8sproxy.NewUpgradeAwareHandler(u, transport, true, false, responder)
	handler.ServeHTTP(rw, req)
}
