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

func logHighlight(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	t.Log(color.New(color.FgHiCyan).Sprintf(format, args...))
}
