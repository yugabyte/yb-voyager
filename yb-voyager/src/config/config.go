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

	// DefaultLogMaxSizeMB and DefaultLogMaxBackups are the values yb-voyager used to
	// hardcode when configuring lumberjack, kept as the defaults now that they are
	// configurable, so unset behaviour is unchanged.
	DefaultLogMaxSizeMB  = 200
	DefaultLogMaxBackups = 10

	// LogMaxBackupsUnlimited is the --log-max-backups sentinel meaning "never delete a
	// rotated log file". A dedicated sentinel is used rather than accepting 0 because
	// lumberjack itself treats MaxBackups == 0 as "retain all", which would silently
	// invert the intent of a user passing 0 to cap disk usage.
	LogMaxBackupsUnlimited = -1
)

var (
	LogLevel       string
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

// ValidateLogSettings checks the log rotation settings resolved from the CLI flags and
// config file. It must run before any code path initialises logging with them.
func ValidateLogSettings() error {
	if LogMaxSizeMB <= 0 {
		return goerrors.Errorf("invalid log-max-size-mb: %d. Must be a positive integer", LogMaxSizeMB)
	}
	if LogMaxBackups <= 0 && LogMaxBackups != LogMaxBackupsUnlimited {
		return goerrors.Errorf("invalid log-max-backups: %d. Must be a positive integer, or %d to retain all rotated log files", LogMaxBackups, LogMaxBackupsUnlimited)
	}
	return nil
}

// LumberjackMaxBackups translates the LogMaxBackupsUnlimited sentinel into the value
// lumberjack.Logger expects for "retain all rotated files". lumberjack has no sentinel of
// its own: with MaxBackups and MaxAge both 0 and compression off, it never deletes a
// rotated file.
func LumberjackMaxBackups(maxBackups int) int {
	if maxBackups == LogMaxBackupsUnlimited {
		return 0
	}
	return maxBackups
}

func IsLogLevelDebugOrBelow() bool {
	return lo.Contains([]string{TRACE, DEBUG}, LogLevel)
}

func IsLogLevelErrorOrAbove() bool {
	return lo.Contains([]string{ERROR, FATAL, PANIC}, LogLevel)
}
