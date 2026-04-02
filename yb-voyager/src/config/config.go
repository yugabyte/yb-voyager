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
)

var (
	LogLevel       string
	validLogLevels = []string{TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC}
)

func ValidateLogLevel() error {
	LogLevel = strings.ToLower(LogLevel)
	if !lo.Contains(validLogLevels, LogLevel) {
		return goerrors.Errorf("invalid log level: %s. Valid log levels = %v", LogLevel, validLogLevels)
	}
	return nil
}

func IsLogLevelDebugOrBelow() bool {
	return lo.Contains([]string{TRACE, DEBUG}, LogLevel)
}

func IsLogLevelErrorOrAbove() bool {
	return lo.Contains([]string{ERROR, FATAL, PANIC}, LogLevel)
}
