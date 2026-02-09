//go:build failpoint

package cmd

import (
	"os"
	"testing"

	"github.com/fatih/color"
)

func init() {
	// Force colors even when go test output isn't a TTY.
	color.NoColor = false
	color.Output = os.Stdout
}

const testLogPrefix = "[TEST] "

func logTest(t *testing.T, msg string) {
	t.Helper()
	t.Log(color.HiCyanString("%s%s", testLogPrefix, msg))
}

func logTestf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	t.Log(color.HiCyanString(testLogPrefix+format, args...))
}
