package ui

import (
	"crypto/tls"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	localUI        = "./static"
	uiDefaultIndex = "https://autok3s-ui.s3-ap-southeast-2.amazonaws.com/static/index.html"
)

var (
	insecureClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

type fsFunc func(name string) (fs.File, error)

func (f fsFunc) Open(name string) (fs.File, error) {
	return f(name)
}

// Serve serve ui component.
func Serve() http.Handler {
	if uiMode() == "dev" {
		return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_ = serveIndex(writer)
		})
	}
	return serveAsset()
}

// ServeNotFound server ui component, not found will be return index.html.
func ServeNotFound(next http.Handler) http.Handler {
	if uiMode() == "dev" {
		return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_ = serveIndex(writer)
		})
	}
	return serveAssetNotFound(next)
}

func ServeJavascript(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// This is used to enforce application/javascript MIME on Windows (https://github.com/cnrancher/autok3s/issues/426)
		// refer to: https://github.com/golang/go/issues/32350
		if strings.HasSuffix(r.URL.Path, ".js") {
			rw.Header().Set("Content-Type", "application/javascript")
		}
		next.ServeHTTP(rw, r)
	})
}

func serveAsset() http.Handler {
	handler := fsFunc(func(name string) (fs.File, error) {
		assetPath := path.Join(localUI, name)
		file, err := assets.Open(assetPath)
		if err != nil {
			return nil, err
		}
		return file, err
	})

	return http.FileServer(http.FS(handler))
}

func serveAssetNotFound(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, "/ui/")
		_, err := assets.Open(path.Join(localUI, p))
		if err != nil && os.IsNotExist(err) {
			f, _ := assets.Open(path.Join(localUI, "index.html"))
			_, _ = io.Copy(rw, f)
			_ = f.Close()
			return
		}
		next.ServeHTTP(rw, req)
	})
}

func serveIndex(resp io.Writer) error {
	uiIndex := os.Getenv("AUTOK3S_UI_INDEX")
	if uiIndex == "" {
		uiIndex = uiDefaultIndex
	}
	r, err := insecureClient.Get(uiIndex)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}

func uiMode() string {
	mode := os.Getenv("AUTOK3S_UI_MODE")
	if mode == "" {
		return DefaultMode
	}
	return mode
}
