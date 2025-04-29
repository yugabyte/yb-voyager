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
package utils

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"
)

var (
	// ErrExitErr holds the last error passed to ErrExit (unchanged) - used for final status to callhome
	ErrExitErr error

	// exitHook is what ErrExit calls to terminate.
	// Default = atexit.Exit, but you can override it.
	exitHook = atexit.Exit
)

// SetExitHook lets callers replace the termination behaviour.
// Pass nil to restore the default (atexit.Exit).
func SetExitHook(h func(code int)) {
	if h == nil {
		exitHook = atexit.Exit
	} else {
		exitHook = h
	}
}

// ErrExit prints the formatted error and then terminates via exitHook.
func ErrExit(format string, args ...interface{}) {
	ErrExitErr = fmt.Errorf(format, args...)

	format = strings.Replace(format, "%w", "%s", -1)
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	log.Errorf(format+"\n", args...)

	exitHook(1)
}

func PrintAndLog(formatString string, args ...interface{}) {
	log.Infof(formatString, args...)
	if !strings.HasSuffix(formatString, "\n") {
		formatString = formatString + "\n"
	}
	fmt.Printf(formatString, args...)
}
