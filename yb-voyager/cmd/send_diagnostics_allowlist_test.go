//go:build unit

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestSendDiagnosticsAllowlistMatchesRegisteredCommands walks all subcommands of rootCmd,
// finds which ones have the "send-diagnostics" flag registered, and verifies that the
// allowlist in test/utils/voyager_cmd_runner.go stays in sync.
//
// If this test fails, a new command was added that registers --send-diagnostics
// (via registerCommonGlobalFlags) but is missing from the allowlist. Update
// commandsSupportingSendDiagnostics in yb-voyager/test/utils/voyager_cmd_runner.go.
func TestSendDiagnosticsAllowlistMatchesRegisteredCommands(t *testing.T) {
	// Collect all commands that have the "send-diagnostics" flag registered.
	registered := map[string]bool{}
	walkCommands(rootCmd, func(cmd *cobra.Command) {
		if cmd == rootCmd {
			return
		}
		if f := cmd.Flags().Lookup("send-diagnostics"); f != nil {
			// Use the subcommand path without the root prefix, e.g. "export data"
			name := strings.TrimPrefix(cmd.CommandPath(), rootCmd.Name()+" ")
			registered[name] = true
		}
	})

	// This must exactly match commandsSupportingSendDiagnostics in
	// yb-voyager/test/utils/voyager_cmd_runner.go. Keep both in sync.
	allowlist := map[string]bool{
		"export schema":                    true,
		"export data":                      true,
		"export data from source":          true,
		"export data from target":          true,
		"import schema":                    true,
		"import data":                      true,
		"import data to target":            true,
		"import data to source":            true,
		"import data to source-replica":    true,
		"import data file":                 true,
		"analyze-schema":                   true,
		"assess-migration":                 true,
		"finalize-schema-post-data-import": true,
		"compare-performance":              true,
		"end migration":                    true,
		"archive changes":                  true,
		"assess-migration-bulk":            true,
	}

	for cmd := range registered {
		if !allowlist[cmd] {
			t.Errorf("Command %q registers --send-diagnostics but is missing from the allowlist.\n"+
				"Add it to commandsSupportingSendDiagnostics in yb-voyager/test/utils/voyager_cmd_runner.go "+
				"and update the allowlist in this test.", cmd)
		}
	}

	for cmd := range allowlist {
		if !registered[cmd] {
			t.Errorf("Command %q is in the allowlist but does not register --send-diagnostics.\n"+
				"Remove it from commandsSupportingSendDiagnostics in yb-voyager/test/utils/voyager_cmd_runner.go "+
				"and from the allowlist in this test.", cmd)
		}
	}
}

func walkCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, child := range cmd.Commands() {
		walkCommands(child, fn)
	}
}
