package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ghodss/yaml"
)

const (
	HomeEnv        = "HOME"
	HomeDriveEnv   = "HOMEDRIVE"
	HomePathEnv    = "HOMEPATH"
	UserProfileEnv = "USERPROFILE"
)

func EnsureFileExist(path string) error {
	if path == "" {
		return errors.New(fmt.Sprintf("path %s cannot be empty\n", path))
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

func UserHome() string {
	if home := os.Getenv(HomeEnv); home != "" {
		return home
	}
	homeDrive := os.Getenv(HomeDriveEnv)
	homePath := os.Getenv(HomePathEnv)
	if homeDrive != "" && homePath != "" {
		return homeDrive + homePath
	}
	return os.Getenv(UserProfileEnv)
}

func WriteYaml(source interface{}, path, name string) error {
	b, err := yaml.Marshal(source)
	if err != nil {
		return err
	}

	n := fmt.Sprintf("%s/%s", path, name)

	if _, err := os.Stat(n); os.IsNotExist(err) {
		f, err := os.Create(n)
		if err != nil {
			return err
		}

		defer func() {
			_ = f.Close()
		}()
	}

	return ioutil.WriteFile(n, b, 0644)
}

func ReadYaml(path, name string) (i []interface{}, err error) {
	n := fmt.Sprintf("%s/%s", path, name)

	b, err := ioutil.ReadFile(n)
	if err != nil {
		if os.IsNotExist(err) {
			return i, nil
		}
		return nil, err
	}

	if len(b) != 0 {
		err = yaml.Unmarshal(b, &i)
		if err != nil {
			return nil, err
		}
	}

	return i, nil
}
