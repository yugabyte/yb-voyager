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
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

type MyFormatter struct{}

var levelList = []string{
	"PANIC",
	"FATAL",
	"ERROR",
	"WARN",
	"INFO",
	"DEBUG",
	"TRACE",
}

func (mf *MyFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := levelList[int(entry.Level)]
	fileName := filepath.Base(entry.Caller.File)
	// Example log line:
	// 2022-03-23 12:16:42 INFO main.go:27 Logging initialised.
	msg := fmt.Sprintf("%s %s %s:%d %s\n",
		entry.Time.Format("2006-01-02 15:04:05"), level,
		fileName, entry.Caller.Line, entry.Message)
	return []byte(msg), nil
}

func InitLogging(logDir string, disableLogging bool) {
	// Redirect log messages to ${logDir}/yb-voyager.log if not a status command.
	if disableLogging {
		log.SetOutput(ioutil.Discard)
		return
	}
	logFileName := filepath.Join(logDir, "yb-voyager.log")
	f, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialise logging: open log file %q: %s", logFileName, err))
	}
	log.SetOutput(f)

	log.SetReportCaller(true)
	log.SetFormatter(&MyFormatter{})
	log.Info("Logging initialised.")
	log.Infof("Args: %v", os.Args)
}
