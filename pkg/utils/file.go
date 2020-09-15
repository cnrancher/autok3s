package utils

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
)

const (
	HomeEnv        = "HOME"
	HomeDriveEnv   = "HOMEDRIVE"
	HomePathEnv    = "HOMEPATH"
	UserProfileEnv = "USERPROFILE"
)

func EnsureFolderExist(path string) error {
	if path == "" {
		return fmt.Errorf("path %s cannot be empty", path)
	}
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func EnsureFileExist(path, file string) error {
	if err := EnsureFolderExist(path); err != nil {
		return err
	}
	n := fmt.Sprintf("%s/%s", path, file)
	if _, err := os.Stat(n); os.IsNotExist(err) {
		_, fileE := os.Create(n)
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

func WriteBytesToYaml(b []byte, path, name string) error {
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
