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
)

// TestSchemaDriftErrorHintLeadIn pins which commands get a drift hint on failure.
// The post-cutover importers and the target exporter must stay out: they run past the
// last source capture, so the hint would point at a report that cannot cover them.
func TestSchemaDriftErrorHintLeadIn(t *testing.T) {
	origRole := exporterRole
	t.Cleanup(func() { exporterRole = origRole })

	tests := []struct {
		name        string
		commandPath string
		role        string
		wantLeadIn  string
		wantOK      bool
	}{
		{
			name:        "import data",
			commandPath: importDataCmd.CommandPath(),
			wantLeadIn:  "import data exited with an error.",
			wantOK:      true,
		},
		{
			name:        "import data to target",
			commandPath: importDataToTargetCmd.CommandPath(),
			wantLeadIn:  "import data exited with an error.",
			wantOK:      true,
		},
		{
			name:        "import data to source is post-cutover",
			commandPath: importDataToSourceCmd.CommandPath(),
			wantOK:      false,
		},
		{
			name:        "import data to source-replica is post-cutover",
			commandPath: importDataToSourceReplicaCmd.CommandPath(),
			wantOK:      false,
		},
		{
			name:        "export data as the source exporter",
			commandPath: exportDataCmd.CommandPath(),
			role:        SOURCE_DB_EXPORTER_ROLE,
			wantLeadIn:  "export data exited with an error.",
			wantOK:      true,
		},
		{
			name:        "export data from source",
			commandPath: exportDataFromSrcCmd.CommandPath(),
			role:        SOURCE_DB_EXPORTER_ROLE,
			wantLeadIn:  "export data exited with an error.",
			wantOK:      true,
		},
		{
			name:        "export data under the fall-back target exporter",
			commandPath: exportDataCmd.CommandPath(),
			role:        TARGET_DB_EXPORTER_FB_ROLE,
			wantOK:      false,
		},
		{
			name:        "export data from target",
			commandPath: exportDataFromTargetCmd.CommandPath(),
			role:        TARGET_DB_EXPORTER_FB_ROLE,
			wantOK:      false,
		},
		{
			name:        "import data file has no source schema",
			commandPath: importDataFileCmd.CommandPath(),
			wantOK:      false,
		},
		{
			name:        "import schema is out of scope",
			commandPath: importSchemaCmd.CommandPath(),
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporterRole = tt.role
			leadIn, ok := schemaDriftErrorHintLeadIn(tt.commandPath)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				// A caller that ignored ok must not get a printable sentence.
				assert.Equal(t, "", leadIn)
				return
			}
			assert.Equal(t, tt.wantLeadIn, leadIn)
		})
	}
}

// TestSchemaDriftGuidanceIsUsefulWithoutMetaDB covers the commands that die before the
// export dir is opened. metaDB is nil there, and a hint would point at a report that
// no snapshot backs.
func TestSchemaDriftGuidanceIsUsefulWithoutMetaDB(t *testing.T) {
	orig := metaDB
	t.Cleanup(func() { metaDB = orig })

	metaDB = nil
	assert.False(t, schemaDriftGuidanceIsUseful())
}
