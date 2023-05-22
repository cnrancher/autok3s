package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/cnrancher/autok3s/pkg/settings"

	"github.com/sirupsen/logrus"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

const (
	forwardProto = "X-Forwarded-Proto"
	hostRegex    = "[A-Za-z0-9-]+"
)

var (
	httpStart  = regexp.MustCompile("^http:/([^/])")
	httpsStart = regexp.MustCompile("^https:/([^/])")
)

type proxy struct {
	prefix string
}

// NewProxy return http proxy handler
func NewProxy(prefix string) http.Handler {
	p := &proxy{
		prefix: prefix,
	}
	return &httputil.ReverseProxy{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Director: func(req *http.Request) {
			if err := p.proxy(req); err != nil {
				logrus.Infof("Failed to proxy: %v", err)
			}
		},
	}
}

func (p *proxy) isAllowed(host string) bool {
	validHosts := strings.Split(settings.WhitelistDomain.Get(), ",")
	for _, valid := range validHosts {
		if valid == host {
			return true
		}

		if strings.HasPrefix(valid, "*") && strings.HasSuffix(host, valid[1:]) {
			return true
		}

		if strings.Contains(valid, ".%.") || strings.HasPrefix(valid, "%.") {
			r := constructRegex(valid)
			if match := r.MatchString(host); match {
				return true
			}
		}
	}
	return false
}

func (p *proxy) proxy(req *http.Request) error {
	path := req.URL.String()
	index := strings.Index(path, p.prefix)
	destPath := path[index+len(p.prefix):]
	destPath, err := unescapePath(destPath)
	if err != nil {
		return err
	}

	if httpsStart.MatchString(destPath) {
		destPath = httpsStart.ReplaceAllString(destPath, "https://$1")
	} else if httpStart.MatchString(destPath) {
		destPath = httpStart.ReplaceAllString(destPath, "http://$1")
	} else {
		destPath = "https://" + destPath
	}

	destURL, err := url.Parse(destPath)
	if err != nil {
		return err
	}

	destURL.RawQuery = req.URL.RawQuery
	destURLHostname := destURL.Hostname()

	if !p.isAllowed(destURLHostname) {
		return fmt.Errorf("invalid host: %v", destURLHostname)
	}

	headerCopy := utilnet.CloneHeader(req.Header)
	if req.TLS != nil {
		headerCopy.Set(forwardProto, "https")
	}
	req.Host = destURLHostname
	req.URL = destURL
	req.Header = headerCopy

	return nil
}

func constructRegex(host string) *regexp.Regexp {
	// incoming host "ec2.%.amazonaws.com"
	// Converted to regex "^ec2\.[A-Za-z0-9-]+\.amazonaws\.com$"
	parts := strings.Split(host, ".")
	for i, part := range parts {
		if part == "%" {
			parts[i] = hostRegex
		} else {
			parts[i] = regexp.QuoteMeta(part)
		}
	}

	str := "^" + strings.Join(parts, "\\.") + "$"

	return regexp.MustCompile(str)
}

func unescapePath(destPath string) (string, error) {
	var err error
	if os.Getenv("AUTOK3S_DEV_MODE") != "" {
		destPath, err = url.QueryUnescape(destPath)
		logrus.Infof("******* proxy url: %v ********", destPath)
		if err != nil {
			return "", err
		}
	}
	return destPath, nil
}
