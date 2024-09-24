package common

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// NewLogger returns new logger struct.
func NewLogger(w *os.File) (logger *logrus.Logger) {
	if w != nil {
		mw := io.MultiWriter(os.Stderr, w)
		logger = logrus.New()
		InitLogger(logger)
		logger.SetOutput(mw)
	} else {
		logger = logrus.StandardLogger()
	}

	return
}

// GetOldLogPath returns old log path.
func GetOldLogPath() string {
	return filepath.Join(CfgPath, "logs")
}

func GetClusterLogFilePath(clusterName string) string {
	return filepath.Join(GetClusterContextPath(clusterName), "log")
}

func GetClusterContextPath(clusterName string) string {
	return filepath.Join(CfgPath, clusterName)
}

// GetLogFile open and return log file.
func GetLogFile(clusterName string) (logFile *os.File, err error) {
	logFilePath := GetClusterLogFilePath(clusterName)
	if err = os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return nil, err
	}
	// check file exist
	_, err = os.Stat(logFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		logFile, err = os.Create(logFilePath)
	} else {
		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	}
	return logFile, err
}

func InitLogger(logger *logrus.Logger) {
	if Debug {
		logger.SetLevel(logrus.DebugLevel)
	}
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func MoveLogs() error {
	oldRoot := GetOldLogPath()
	_, err := os.Stat(oldRoot)
	if os.IsNotExist(err) {
		return nil
	}
	newRoot := CfgPath

	if err := filepath.Walk(oldRoot, func(path string, info fs.FileInfo, _ error) error {
		// skip all the dirs because we store all the logs with cluster context name and no dirs exists in logs dir
		if info.IsDir() {
			return nil
		}
		// assuming all the relative path should only be logs file
		rel, _ := filepath.Rel(oldRoot, path)
		if err := os.MkdirAll(filepath.Join(newRoot, rel), 0755); err != nil {
			return err
		}

		return os.Rename(path, GetClusterLogFilePath(rel))
	}); err != nil {
		return err
	}
	return os.RemoveAll(oldRoot)
}
