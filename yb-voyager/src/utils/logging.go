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

	"github.com/fatih/color"
	goerrors "github.com/go-errors/errors"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"
)

var originalErrExit func(formatString string, args ...interface{})

var ErrExitErr error

// errorColor renders fatal errors in red on stderr. fatih/color's global
// NoColor is derived from stdout, so it cannot decide coloring for stderr on its
// own: when stdout is a TTY but stderr is redirected (e.g. `yb-voyager ... 2>
// err.log`), it would still emit ANSI escape codes into the file. We therefore
// disable color when stderr itself is not a TTY.
var errorColor = func() *color.Color {
	c := color.New(color.FgRed)
	if !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		c.DisableColor()
	}
	return c
}()

var ErrExit = func(formatString string, args ...interface{}) {
	ErrExitErr = goerrors.Errorf(formatString, args...)
	formatString = strings.Replace(formatString, "%w", "%s", -1)
	message := fmt.Sprintf(formatString, args...)
	// Console: render the error in red so failures stand out and visually match
	// the red "✗" markers printed by the progress/preflight UX. errorColor
	// degrades to plain text automatically when stderr is not a TTY or color is
	// globally disabled. color.Error is the colorable stderr writer, so ANSI is
	// translated correctly on Windows too.
	errorColor.Fprintln(color.Error, message) // Log file: keep plain text only (never embed ANSI escape codes in logs).
	log.Error(message)
	atexit.Exit(1)
}

// ErrExitPreLog prints the message to stderr and exits with status 1.
// Use it instead of ErrExit when logging has not been initialized yet (e.g. flag
// validation that runs before InitLogging(), or when logging init itself fails).
// It deliberately does not call log.Errorf: doing so before logging is redirected
// to the log file would print the message twice (once here and again via logrus's
// default stderr output).
var ErrExitPreLog = func(formatString string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatString+"\n", args...)
	atexit.Exit(1)
}

// OutputLogLevel represents different types of console messages
type OutputLogLevel int

const (
	OutputLogLevelSuccess OutputLogLevel = iota
	OutputLogLevelInfo    OutputLogLevel = iota
	OutputLogLevelWarning OutputLogLevel = iota
	OutputLogLevelError   OutputLogLevel = iota
	OutputLogLevelNormal  OutputLogLevel = iota
	OutputLogLevelTitle   OutputLogLevel = iota
)

var (
	SuccessColor = color.New(color.FgGreen)
	InfoColor    = color.New(color.FgCyan)
	WarningColor = color.New(color.FgYellow)
	ErrorColor   = color.New(color.FgRed)
	Path         = color.New(color.Underline)
	TitleColor   = color.New(color.Bold)
)

// PrintAndLogFormatted prints a formatted message to console with specified log level and also logs it
// OutputLogLevel determines the color/style of the message:
//   - OutputLogLevelSuccess: Green text
//   - OutputLogLevelInfo: Cyan text
//   - OutputLogLevelWarning: Yellow text
//   - OutputLogLevelError: Red text
//   - OutputLogLevelNormal: Plain white text
func PrintAndLogFormatted(OutputLogLevel OutputLogLevel, formatString string, args ...interface{}) {
	// Determine the message based on format string and args
	message := fmt.Sprintf(formatString, args...)

	// Ensure newline if not present
	if !strings.HasSuffix(formatString, "\n") {
		message = message + "\n"
	}

	// Print to console with appropriate color
	var printer *color.Color
	switch OutputLogLevel {
	case OutputLogLevelSuccess:
		printer = SuccessColor
		log.Info(message)
	case OutputLogLevelInfo:
		printer = InfoColor
		log.Info(message)
	case OutputLogLevelWarning:
		printer = WarningColor
		log.Warn(message)
	case OutputLogLevelError:
		printer = ErrorColor
		log.Error(message)
	case OutputLogLevelTitle:
		printer = TitleColor
		log.Info(message)
	default: // OutputLogLevelNormal
		fmt.Print(message)
		log.Info(message)
		return
	}

	if OutputLogLevel == OutputLogLevelTitle {
		printTitle(message, printer)
	} else {
		printer.Print(message)
	}
	return
}

func PrintFormatted(OutputLogLevel OutputLogLevel, formatString string, args ...interface{}) {
	message := fmt.Sprintf(formatString, args...)
	if !strings.HasSuffix(formatString, "\n") {
		message = message + "\n"
	}
	switch OutputLogLevel {
	case OutputLogLevelSuccess:
		SuccessColor.Print(message)
	case OutputLogLevelInfo:
		InfoColor.Print(message)
	case OutputLogLevelWarning:
		WarningColor.Print(message)
	case OutputLogLevelError:
		ErrorColor.Print(message)
	case OutputLogLevelTitle:
		printTitle(message, TitleColor)
	default:
		fmt.Print(message)
	}
}

// printTitle formats and prints a title message with separators
/*
e.g.
=============
Title Message
=============

*/
func printTitle(message string, printer *color.Color) {
	// Remove trailing newline if present for formatting
	message = strings.TrimSuffix(message, "\n")
	borderLen := len(message)
	border := strings.Repeat("=", borderLen)
	printer.Printf("\n%s", border)
	printer.Printf("%s\n", message)
	printer.Printf("%s\n", border)
}

// Convenience functions for common use cases
func PrintAndLogfSuccess(formatString string, args ...interface{}) {
	PrintAndLogFormatted(OutputLogLevelSuccess, formatString, args...)
}

func PrintAndLogfInfo(formatString string, args ...interface{}) {
	PrintAndLogFormatted(OutputLogLevelInfo, formatString, args...)
}

func PrintAndLogfWarning(formatString string, args ...interface{}) {
	PrintAndLogFormatted(OutputLogLevelWarning, formatString, args...)
}

func PrintAndLogfError(formatString string, args ...interface{}) {
	PrintAndLogFormatted(OutputLogLevelError, formatString, args...)
}

func PrintAndLogfPhase(formatString string, args ...interface{}) {
	PrintAndLogFormatted(OutputLogLevelTitle, formatString, args...)
}

func PrintfInfo(formatString string, args ...interface{}) {
	PrintFormatted(OutputLogLevelInfo, formatString, args...)
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
	MonkeyPatchUtilsErrExit(func(formatString string, args ...interface{}) {
		panic("utils.ErrExit was called with: " + fmt.Sprintf(formatString, args...))
	})
}

func MonkeyPatchUtilsErrExitToIgnore() {
	MonkeyPatchUtilsErrExit(func(formatString string, args ...interface{}) {
		// do nothing
	})
}

// MonkeyPatchUtilsErrExit allows monkey patching of the utils.ErrExit function for testing purposes.
// It replaces the original function with a new one provided by the caller.
func MonkeyPatchUtilsErrExit(newErrExit func(formatString string, args ...interface{})) {
	originalErrExit = ErrExit
	ErrExit = newErrExit
}

// RestoreUtilsErrExit restores the original utils.ErrExit function after monkey patching.
func RestoreUtilsErrExit() {
	if originalErrExit != nil {
		ErrExit = originalErrExit
	}
}
