package utils

import (
	"fmt"
	"os"
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

// UserHome returns user's home dir.
func UserHome() string {
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
