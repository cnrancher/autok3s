package ui

import (
	"crypto/tls"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed static
var assets embed.FS

var (
	localUI        = "./static"
	uiIndex        = "https://autok3s-ui.s3-ap-southeast-2.amazonaws.com/static/index.html"
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

func ServeAsset() http.Handler {
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

func ServeAssetNotFound(next http.Handler) http.Handler {
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

func Serve() http.Handler {
	mode := os.Getenv("AUTOK3S_UI_MODE")
	if mode == "dev" {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_ = serveIndex(writer)
		})
	}
	return ServeAsset()
}

func ServeNotFound(next http.Handler) http.Handler {
	mode := os.Getenv("AUTOK3S_UI_MODE")
	if mode == "dev" {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_ = serveIndex(writer)
		})
	}
	return ServeAssetNotFound(next)
}

func serveIndex(resp io.Writer) error {
	uiPath := os.Getenv("AUTOK3S_UI_INDEX")
	if uiPath != "" {
		uiIndex = uiPath
	}
	r, err := insecureClient.Get(uiIndex)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	_, err = io.Copy(resp, r.Body)
	return err
}
