//go:build unit

package testutils

import (
	"slices"
	"testing"
)

// TestSupportsSendDiagnosticsFlag tests the supportsSendDiagnosticsFlag function
func TestSupportsSendDiagnosticsFlag(t *testing.T) {
	testCases := []struct {
		cmdName    string
		expectFlag bool
	}{
		// Commands that SHOULD have the flag
		{"export data", true},
		{"import data", true},
		{"export schema", true},
		{"import schema", true},
		{"assess-migration", true},
		{"analyze-schema", true},
		{"initiate cutover to target", true},
		{"initiate cutover to source", true},
		{"end migration", true},
		{"archive changes", true},
		{"compare-performance", true},
		{"export data from source", true},
		{"export data from target", true},
		{"import data to target", true},
		{"import data to source", true},
		{"import data file", true},
		{"finalize-schema-post-data-import", true},
		{"segment-cleanup", true},

		// Commands that should NOT have the flag
		{"export data status", false},
		{"import data status", false},
		{"get data-migration-report", false},
		{"cutover status", false},
		{"version", false},
		{"help", false},
	}

	for _, tc := range testCases {
		t.Run(tc.cmdName, func(t *testing.T) {
			result := supportsSendDiagnosticsFlag(tc.cmdName)
			if result != tc.expectFlag {
				t.Errorf("supportsSendDiagnosticsFlag(%q) = %v, want %v", tc.cmdName, result, tc.expectFlag)
			}
		})
	}
}

// TestSendDiagnosticsFlagAddedForNonContainerCommands tests that the flag is added
// for commands that don't require a container (like initiate cutover, analyze-schema)
func TestSendDiagnosticsFlagAddedForNonContainerCommands(t *testing.T) {
	// These commands don't require source/target containers
	testCases := []struct {
		cmdName    string
		expectFlag bool
	}{
		{"analyze-schema", true},
		{"initiate cutover to target", true},
		{"initiate cutover to source", true},
		{"end migration", true},
		{"archive changes", true},
		{"segment-cleanup", true},
		{"export data status", false},
		{"import data status", false},
		{"get data-migration-report", false},
		{"cutover status", false},
	}

	for _, tc := range testCases {
		t.Run(tc.cmdName, func(t *testing.T) {
			runner := NewVoyagerCommandRunner(nil, tc.cmdName, []string{"--export-dir", "/tmp/test"}, nil, false)
			err := runner.Prepare()
			if err != nil {
				t.Fatalf("Prepare() failed: %v", err)
			}

			hasFlag := slices.Contains(runner.finalArgs, "--send-diagnostics")
			if hasFlag != tc.expectFlag {
				t.Errorf("Command %q: expected --send-diagnostics=%v, got %v. FinalArgs: %v",
					tc.cmdName, tc.expectFlag, hasFlag, runner.finalArgs)
			}

			// If flag is expected, verify it's set to "false"
			if tc.expectFlag {
				idx := slices.Index(runner.finalArgs, "--send-diagnostics")
				if idx >= 0 && idx+1 < len(runner.finalArgs) {
					if runner.finalArgs[idx+1] != "false" {
						t.Errorf("Expected --send-diagnostics value to be 'false', got %q",
							runner.finalArgs[idx+1])
					}
				}
			}
		})
	}
}

// TestSendDiagnosticsFlagNotDuplicatedIfAlreadyPresent tests that the flag is not
// added if the user already provided it
func TestSendDiagnosticsFlagNotDuplicatedIfAlreadyPresent(t *testing.T) {
	runner := NewVoyagerCommandRunner(nil, "analyze-schema", []string{
		"--export-dir", "/tmp/test",
		"--send-diagnostics", "true", // User explicitly set it
	}, nil, false)

	err := runner.Prepare()
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	// Count occurrences of --send-diagnostics
	count := 0
	for _, arg := range runner.finalArgs {
		if arg == "--send-diagnostics" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected exactly 1 --send-diagnostics flag, got %d. FinalArgs: %v", count, runner.finalArgs)
	}
}
