package main

import (
	"log"

	bindata "github.com/go-bindata/go-bindata"
)

func main() {
	bc := &bindata.Config{
		Input: []bindata.InputConfig{
			{
				Path:      "static",
				Recursive: true,
			},
		},
		Package:        "ui",
		Prefix:         "static/",
		HttpFileSystem: true,
		Output:         "pkg/server/ui/zz_generated_bindata.go",
	}
	if err := bindata.Translate(bc); err != nil {
		log.Fatal(err)
	}
}
