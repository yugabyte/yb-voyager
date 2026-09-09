//go:build unit

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
package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSchemaSnapshotCaptureInterval pins the flag validation. An interval of 0
// or less would silently disable periodic capture, so it must fail at startup instead.
//
// The last case pins the flag-presence scoping: with the flag absent from the command,
// the guard no-ops whatever the global holds. It sets 0 explicitly to exercise that,
// which a real `export data from target` run would not -- IntVar writes 60 into the
// global at registration, so that scoping is defensive rather than load-bearing.
func TestValidateSchemaSnapshotCaptureInterval(t *testing.T) {
	orig := schemaSnapshotCaptureInterval
	t.Cleanup(func() { schemaSnapshotCaptureInterval = orig })

	withFlag := func() *cobra.Command {
		cmd := &cobra.Command{Use: "export-data-stub"}
		registerSchemaSnapshotIntervalFlag(cmd)
		return cmd
	}

	tests := []struct {
		name     string
		cmd      func() *cobra.Command
		interval int
		wantErr  bool
	}{
		{"default interval passes", withFlag, 60, false},
		{"the minimum interval passes", withFlag, 1, false},
		{"zero is rejected", withFlag, 0, true},
		{"negative is rejected", withFlag, -5, true},
		{
			"a command without the flag is not validated (export data from target)",
			func() *cobra.Command { return &cobra.Command{Use: "export-data-from-target-stub"} },
			0,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmd()
			// registerSchemaSnapshotIntervalFlag binds the global and resets it to the
			// flag default, so set the value under test after building the command.
			schemaSnapshotCaptureInterval = tt.interval

			err := validateSchemaSnapshotCaptureInterval(cmd)
			if tt.wantErr {
				require.Error(t, err, "an interval of %d must be rejected", tt.interval)
				assert.Contains(t, err.Error(), "must be at least 1",
					"the error must say what a valid interval is")
				return
			}
			require.NoError(t, err)
		})
	}
}
