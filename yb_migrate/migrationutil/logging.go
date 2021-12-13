package migrationutil

import (
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

func CheckError(err error, executedCommand string, possibleReason string, stop bool) {
	if err != nil {
		if executedCommand != "" {
			log.Printf("Error Command: %s\n", executedCommand)
		}

		if possibleReason != "" {
			log.Printf("%s\n", possibleReason)
		}
		if stop {
			log.Fatalf("%s \n", err)
		} else {
			log.Printf("%s \n", err)
		}
	}
}

func GetLogger() *logrus.Logger {
	log.SetLevel(log.InfoLevel)

	return &log.Logger
}
