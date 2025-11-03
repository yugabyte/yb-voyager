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

var originalErrExit func(formatString string, args ...interface{})

var ErrExitErr error

var ErrExit = func(formatString string, args ...interface{}) {
	ErrExitErr = fmt.Errorf(formatString, args...)
	formatString = strings.Replace(formatString, "%w", "%s", -1)
	fmt.Fprintf(os.Stderr, formatString+"\n", args...)
	log.Errorf(formatString+"\n", args...)
	atexit.Exit(1)
}

func PrintAndLogf(formatString string, args ...interface{}) {
	log.Infof(formatString, args...)
	if !strings.HasSuffix(formatString, "\n") {
		formatString = formatString + "\n"
	}
	fmt.Printf(formatString, args...)
}

func PrintAndLog(message string) {
	log.Info(message)
	if !strings.HasSuffix(message, "\n") {
		message = message + "\n"
	}
	fmt.Print(message)
}

func MonkeyPatchUtilsErrExitWithPanic() {
	monkeyPatchUtilsErrExit(func(formatString string, args ...interface{}) {
		panic("utils.ErrExit was called with: " + fmt.Sprintf(formatString, args...))
	})
}

func MonkeyPatchUtilsErrExitToIgnore() {
	monkeyPatchUtilsErrExit(func(formatString string, args ...interface{}) {
		// do nothing
	})
}

// monkeyPatchUtilsErrExit allows monkey patching of the utils.ErrExit function for testing purposes.
// It replaces the original function with a new one provided by the caller.
func monkeyPatchUtilsErrExit(newErrExit func(formatString string, args ...interface{})) {
	originalErrExit = ErrExit
	ErrExit = newErrExit
}

// RestoreUtilsErrExit restores the original utils.ErrExit function after monkey patching.
func RestoreUtilsErrExit() {
	if originalErrExit != nil {
		ErrExit = originalErrExit
	}
}
