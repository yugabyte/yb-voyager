/*
Copyright (c) YugaByte, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type LogFormatter struct {
	logrus.TextFormatter
}

//Format details
func (s *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := time.Now().Local().Format("2006-01-02 15:04:05")

	//TODO: If we want the file and the line of the caller
	// var file string
	// var line int
	// if entry.Caller != nil {
	// 	file = filepath.Base(entry.Caller.File)
	// 	line = entry.Caller.Line
	// }

	msg := fmt.Sprintf("%s [%s] %s\n", timestamp, strings.ToUpper(entry.Level.String()), entry.Message)
	return []byte(msg), nil
}

func GetLogger() *logrus.Logger {
	var logger = logrus.New()
	logger.SetLevel(logrus.TraceLevel)

	customFormatter := new(LogFormatter) //create a custom formatter instead of Text Formatter for better output

	logger.SetFormatter(customFormatter)
	logger.SetReportCaller(true)

	return logger
}

func CheckError(err error, executedCommand string, possibleReason string, stop bool) {
	if err != nil {
		if executedCommand != "" {
			log.Infof("Error Command: %s", executedCommand)
		}

		if possibleReason != "" {
			log.Infof("%s", possibleReason)
		}
		if stop {
			log.Fatalf("%s", err)
		} else {
			log.Infof("%s", err)
		}
	}
}
