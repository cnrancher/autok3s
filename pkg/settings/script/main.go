package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cnrancher/autok3s/pkg/settings"

	"github.com/sirupsen/logrus"
)

const (
	defaultFileName = "install.sh"
)

func main() {
	if len(os.Args) != 2 {
		logrus.Fatal("target path should be specified")
	}
	targetPath := os.Args[1]
	info, err := os.Lstat(targetPath)
	if err != nil && !os.IsNotExist(err) {
		logrus.Fatal(err)
	}
	if err == nil && info.IsDir() {
		targetPath = filepath.Join(strings.TrimSuffix(targetPath, "/"), defaultFileName)
	}
	targetFile, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		logrus.Fatalf("failed to create script file, %v", err)
	}
	defer targetFile.Close()

	if err := settings.GetScriptFromSource(targetFile); err != nil {
		logrus.Fatalf("failed to get script from source, %v", err)
	}
}
