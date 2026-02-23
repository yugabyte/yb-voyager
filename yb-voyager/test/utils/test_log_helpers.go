package testutils

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

func LogTest(t *testing.T, msg string) {
	t.Helper()
	t.Log(color.HiCyanString("%s%s", testLogPrefix, msg))
}

func LogTestf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Log(color.HiCyanString(testLogPrefix+format, args...))
}
