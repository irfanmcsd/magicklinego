package config

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

type LoggerResult struct {
	Logger *logrus.Logger
}

func InitLogger(isDebug bool) (*LoggerResult, error) {
	logger := logrus.New()

	// Console formatter (colorized)
	consoleFormatter := &logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	}

	// File logging setup
	file, err := os.OpenFile("application.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// fallback to console only
		logger.SetOutput(os.Stdout)
		logger.SetFormatter(consoleFormatter)
		logger.Warn("⚠️ Failed to open log file, logging to console only")
	} else {
		// log to both console and file
		logger.SetOutput(io.MultiWriter(os.Stdout, file))

		// This formatter only affects the default logger, so use hooks if per-output formatting is required
		logger.SetFormatter(consoleFormatter)
	}

	// Set log level
	if isDebug {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	return &LoggerResult{
		Logger: logger,
	}, nil
}
