/*
Copyright (c) YugabyteDB, Inc.

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
	"io"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
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

func InitLogging(logDir string, disableLogging bool, cmdName string) {
	// Redirect log messages to ${logDir}/yb-voyager.log if not a status command.
	if disableLogging {
		log.SetOutput(io.Discard)
		return
	}
	logFileName := filepath.Join(logDir, "logs", fmt.Sprintf("yb-voyager-%s.log", cmdName))

	// logRotator handles scenario where "logs" folder, or yb-voyager.log file does not exist.
	logRotator := &lumberjack.Logger{
		Filename:   logFileName,
		MaxSize:    200, // 200 MB log size before rotation
		MaxBackups: 10,  // Allow upto 10 logs at once before deleting oldest logs.
	}
	log.SetOutput(logRotator)

	log.SetReportCaller(true)
	log.SetFormatter(&MyFormatter{})
	log.Info("Logging initialised.")
	redactPasswordFromArgs()
	log.Infof("Args: %v", os.Args)
	log.Infof("\n%s", getVersionInfo())
}

func redactPasswordFromArgs() {
	for i := 0; i < len(os.Args); i++ {
		opt := os.Args[i]
		if opt == "--source-db-password" || opt == "--target-db-password" || opt == "--ff-db-password" {
			os.Args[i+1] = "XXX"
		}
	}
}
