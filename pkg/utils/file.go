package utils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const (
	homeEnv        = "HOME"
	homeDriveEnv   = "HOMEDRIVE"
	homePathEnv    = "HOMEPATH"
	userProfileEnv = "USERPROFILE"
)

// EnsureFolderExist ensures folder exist.
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

// EnsureFileExist ensures file exist.
func EnsureFileExist(file string) error {
	if err := EnsureFolderExist(filepath.Dir(file)); err != nil {
		return err
	}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		_, fileE := os.Create(file)
		if fileE != nil {
			return fileE
		}
	}

	return nil
}

// UserHome returns user's home dir.
func UserHome() string {
	u, err := user.Current()
	if err == nil {
		return u.HomeDir
	}
	if home := os.Getenv(homeEnv); home != "" {
		return home
	}
	homeDrive := os.Getenv(homeDriveEnv)
	homePath := os.Getenv(homePathEnv)
	if homeDrive != "" && homePath != "" {
		return homeDrive + homePath
	}
	return os.Getenv(userProfileEnv)
}
