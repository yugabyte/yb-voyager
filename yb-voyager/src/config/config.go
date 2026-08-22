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
package config

import (
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
)

const (
	TRACE = "trace"
	DEBUG = "debug"
	INFO  = "info"
	WARN  = "warn"
	ERROR = "error"
	FATAL = "fatal"
	PANIC = "panic"

	// DefaultLogMaxSizeMB and DefaultLogMaxBackups match lumberjack's pre-existing hardcoded
	// values in InitLogging, kept as the defaults now that they are configurable.
	DefaultLogMaxSizeMB  = 200
	DefaultLogMaxBackups = 10

	// LogMaxBackupsUnlimited is the sentinel value for --log-max-backups that means
	// "never delete rotated log files". Chosen instead of allowing 0 for this because
	// lumberjack.Logger itself treats MaxBackups == 0 as "unlimited", which would
	// silently contradict a user's intent to disable rotation retention.
	LogMaxBackupsUnlimited = -1
)

var (
	LogLevel       string
	LogDir         string
	LogMaxSizeMB   int
	LogMaxBackups  int
	validLogLevels = []string{TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC}
)

func ValidateLogLevel() error {
	LogLevel = strings.ToLower(LogLevel)
	if !lo.Contains(validLogLevels, LogLevel) {
		return goerrors.Errorf("invalid log level: %s. Valid log levels = %v", LogLevel, validLogLevels)
	}
	return nil
}

func ValidateLogSettings() error {
	if LogMaxSizeMB <= 0 {
		return goerrors.Errorf("invalid log-max-size-mb: %d. Must be a positive integer", LogMaxSizeMB)
	}
	if LogMaxBackups != LogMaxBackupsUnlimited && LogMaxBackups <= 0 {
		return goerrors.Errorf("invalid log-max-backups: %d. Must be a positive integer, or %d to retain all rotated files", LogMaxBackups, LogMaxBackupsUnlimited)
	}
	return nil
}

func IsLogLevelDebugOrBelow() bool {
	return lo.Contains([]string{TRACE, DEBUG}, LogLevel)
}

func IsLogLevelErrorOrAbove() bool {
	return lo.Contains([]string{ERROR, FATAL, PANIC}, LogLevel)
}
