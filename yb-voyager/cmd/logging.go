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

// logFileSettings groups the log file destination and rotation settings used by InitLogging.
type logFileSettings struct {
	Dir        string
	MaxSizeMB  int
	MaxBackups int
}

func InitLogging(exportDir string, logLevel string, disableLogging bool, cmdName string, fileSettings logFileSettings) error {
	// Redirect log messages to ${logDir}/yb-voyager.log if not a status command.
	if disableLogging {
		log.SetOutput(io.Discard)
		return nil
	}
	logDir := fileSettings.Dir
	if logDir == "" {
		logDir = filepath.Join(exportDir, "logs")
	}
	logFileName := filepath.Join(logDir, fmt.Sprintf("yb-voyager-%s.log", cmdName))

	// logRotator handles scenario where the log directory, or yb-voyager.log file does not exist.
	logRotator := &lumberjack.Logger{
		Filename:   logFileName,
		MaxSize:    fileSettings.MaxSizeMB,
		MaxBackups: fileSettings.MaxBackups,
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

// currentLogFileSettings builds a logFileSettings from the CLI/config-resolved values.
func currentLogFileSettings() logFileSettings {
	return logFileSettings{
		Dir:        config.LogDir,
		MaxSizeMB:  config.LogMaxSizeMB,
		MaxBackups: config.LogMaxBackups,
	}
}

// logFileSettingsCLIArgs returns the current log-dir/log-max-size-mb/log-max-backups
// settings as CLI args, for forwarding to a spawned yb-voyager subprocess (e.g. the next
// iteration of an iterative cutover) that isn't already inheriting them via a shared
// config file. Mirrors how --log-level is forwarded alongside these at every call site.
func logFileSettingsCLIArgs() []string {
	args := []string{
		"--log-max-size-mb", fmt.Sprintf("%d", config.LogMaxSizeMB),
		"--log-max-backups", fmt.Sprintf("%d", config.LogMaxBackups),
	}
	if config.LogDir != "" {
		args = append(args, "--log-dir", config.LogDir)
	}
	return args
}

// registerLogFlags registers the CLI flags that control yb-voyager's logging behavior
// (level, destination directory, and rotation) on the given command.
func registerLogFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&config.LogLevel, "log-level", "l", "info",
		"log level for yb-voyager. Accepted values: (trace, debug, info, warn, error, fatal, panic)")
	cmd.PersistentFlags().StringVar(&config.LogDir, "log-dir", "",
		"directory to store yb-voyager log files (default: <export-dir>/logs)")
	cmd.PersistentFlags().IntVar(&config.LogMaxSizeMB, "log-max-size-mb", config.DefaultLogMaxSizeMB,
		"maximum size in MB of a log file before it is rotated")
	cmd.PersistentFlags().IntVar(&config.LogMaxBackups, "log-max-backups", config.DefaultLogMaxBackups,
		"maximum number of rotated log files to retain")
}

func redactPasswordFromArgs() {
	for i := 0; i < len(os.Args); i++ {
		opt := os.Args[i]
		if opt == "--source-db-password" || opt == "--target-db-password" || opt == "--source-replica-db-password" {
			os.Args[i+1] = "XXX"
		}
	}
}
