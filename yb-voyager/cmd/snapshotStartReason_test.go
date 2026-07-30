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

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// TestSnapshotStartReasonFor pins the export-data start-snapshot reason classification.
// The actual data-directory state is decided BEFORE the --start-clean flag, so
// clean_restart only applies when prior export-data output actually exists to discard.
// In particular --start-clean on an empty export dir — including passing it on the very
// first run — is "initial", not a mislabeled "clean_restart".
func TestSnapshotStartReasonFor(t *testing.T) {
	tests := []struct {
		name         string
		startClean   bool
		dataDirEmpty bool
		want         string
	}{
		{"first run (empty dir, no start-clean)", false, true, schemasnapshot.ReasonInitial},
		{"start-clean on the first run (empty dir) is initial, not clean_restart", true, true, schemasnapshot.ReasonInitial},
		{"start-clean on a re-run with prior output is clean_restart", true, false, schemasnapshot.ReasonCleanRestart},
		{"prior output without start-clean is resume", false, false, schemasnapshot.ReasonResume},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, snapshotStartReasonFor(tt.startClean, tt.dataDirEmpty))
		})
	}
}
