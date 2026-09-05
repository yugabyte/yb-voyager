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
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
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
		entry.Time.Format("2006-01-02 15:04:05.000000"), level,
		fileName, entry.Caller.Line, entry.Message)
	return []byte(msg), nil
}

func InitLogging(logDir string, logLevel string, disableLogging bool, cmdName string, logMaxSizeMB int, logMaxBackups int) error {
	// Redirect log messages to ${logDir}/yb-voyager.log if not a status command.
	if disableLogging {
		log.SetOutput(io.Discard)
		return nil
	}
	logFileName := filepath.Join(logDir, "logs", fmt.Sprintf("yb-voyager-%s.log", cmdName))

	// logRotator handles scenario where "logs" folder, or yb-voyager.log file does not exist.
	logRotator := &lumberjack.Logger{
		Filename:   logFileName,
		MaxSize:    logMaxSizeMB, // log size in MB before rotation
		MaxBackups: config.LumberjackMaxBackups(logMaxBackups),
	}
	log.SetOutput(logRotator)
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level %s: %w", logLevel, err)
	}
	log.SetLevel(level)

	log.SetReportCaller(true)
	log.SetFormatter(&MyFormatter{})
	log.Info("Logging initialised.")
	redactPasswordFromArgs()
	log.Infof("Args: %v", os.Args)
	log.Infof("\n%s", getVersionInfo())
	return nil
}

// registerLogFlags registers the flags controlling yb-voyager's logging behaviour: level
// and rotation. Shared by registerCommonGlobalFlags and by the two commands
// (assess-migration-bulk, get data-migration-report) that do not go through it.
func registerLogFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&config.LogLevel, "log-level", "l", "info",
		"log level for yb-voyager. Accepted values: (trace, debug, info, warn, error, fatal, panic)")
	cmd.PersistentFlags().IntVar(&config.LogMaxSizeMB, "log-max-size-mb", config.DefaultLogMaxSizeMB,
		"maximum size in MB of a yb-voyager log file before it is rotated")
	cmd.PersistentFlags().IntVar(&config.LogMaxBackups, "log-max-backups", config.DefaultLogMaxBackups,
		fmt.Sprintf("maximum number of rotated yb-voyager log files to retain (%d to retain all)", config.LogMaxBackupsUnlimited))
}

// logSettingsCLIArgs returns the resolved log settings as CLI args, for forwarding to a
// yb-voyager subprocess (the next iteration of an iterative cutover, an assess-migration
// child, the report spawned by end migration) that is not already inheriting them via a
// shared config file. Mirrors how --log-level is forwarded at each of those call sites.
func logSettingsCLIArgs() []string {
	return []string{
		"--log-level", config.LogLevel,
		"--log-max-size-mb", strconv.Itoa(config.LogMaxSizeMB),
		"--log-max-backups", strconv.Itoa(config.LogMaxBackups),
	}
}

func redactPasswordFromArgs() {
	for i := 0; i < len(os.Args); i++ {
		opt := os.Args[i]
		if opt == "--source-db-password" || opt == "--target-db-password" || opt == "--source-replica-db-password" {
			os.Args[i+1] = "XXX"
		}
	}
}
