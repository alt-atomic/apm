package logger

import (
	"apm/config"
	"github.com/sirupsen/logrus"
	"os"
)

var Log = logrus.New()

func InitLogger() {
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   false,
	})

	pathLogFile := config.Env.PathLogFile

	file, err := os.OpenFile(pathLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil || config.DevMode {
		Log.SetOutput(os.Stdout)
	} else {
		Log.SetOutput(file)
	}

	if config.DevMode {
		Log.SetLevel(logrus.DebugLevel)
	} else {
		Log.SetLevel(logrus.InfoLevel)
	}
}
