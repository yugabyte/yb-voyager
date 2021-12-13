package utils

import (
	"github.com/sirupsen/logrus"
)

func CheckError(err error, executedCommand string, possibleReason string, stop bool) {
	if err != nil {
		if executedCommand != "" {
			log.Infof("Error Command: %s", executedCommand)
		}

		if possibleReason != "" {
			log.Infof("%s", possibleReason)
		}
		if stop {
			log.Infof("%s", err)
		} else {
			log.Infof("%s", err)
		}
	}
}

func GetLogger() *logrus.Logger {
	var logger = logrus.New()
	logger.SetLevel(logrus.TraceLevel)

	customFormatter := new(logrus.TextFormatter)
	customFormatter.DisableTimestamp = true
	customFormatter.DisableColors = true

	logger.SetFormatter(customFormatter)
	logger.SetReportCaller(true)

	return logger
}
