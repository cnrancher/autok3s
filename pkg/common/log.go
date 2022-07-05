package common

import (
	"io"
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

// GetLogPath returns log path.
func GetLogPath() string {
	return filepath.Join(CfgPath, "logs")
}

// GetLogFile open and return log file.
func GetLogFile(name string) (logFile *os.File, err error) {
	logFilePath := filepath.Join(GetLogPath(), name)
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
