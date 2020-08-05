package utils

import (
	"errors"
	"os"
	"strings"
)

func EnsureCfgFileExist(path string) error {
	if path == "" {
		return errors.New("cfg path cannot be empty")
	}

	dir := path[0:strings.LastIndex(path, "/")]
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dirE := os.MkdirAll(dir, os.ModePerm)
		if dirE != nil {
			return dirE
		}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		_, fileE := os.Create(path)
		if fileE != nil {
			return fileE
		}
	}

	return nil
}
