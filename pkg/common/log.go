package common

import "github.com/sirupsen/logrus"

var logger *logrus.Logger

func NewLogger(debug bool) *logrus.Logger {
	if logger == nil {
		logger = logrus.New()
		if debug {
			logger.SetLevel(logrus.DebugLevel)
		}
	}
	return logger
}
