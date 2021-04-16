package ui

import (
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

var localUI = "./static"

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
