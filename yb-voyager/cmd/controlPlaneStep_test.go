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

func TestControlPlaneStepFor(t *testing.T) {
	for _, tc := range []struct {
		name         string
		commandPath  string
		exporterRole string
		importerRole string
		postSnapshot bool
		wantStep     string
		wantClaims   bool
	}{
		{name: "assess", commandPath: "yb-voyager assess-migration", wantStep: "assess-migration", wantClaims: true},
		{name: "export schema", commandPath: "yb-voyager export schema", wantStep: "export-schema", wantClaims: true},
		{name: "analyze", commandPath: "yb-voyager analyze-schema", wantStep: "analyze-schema", wantClaims: true},
		{name: "import schema", commandPath: "yb-voyager import schema", wantStep: "import-schema", wantClaims: true},
		{name: "import schema post snapshot", commandPath: "yb-voyager import schema", postSnapshot: true, wantStep: "finalize-schema", wantClaims: true},
		{name: "finalize", commandPath: "yb-voyager finalize-schema-post-data-import", wantStep: "finalize-schema", wantClaims: true},
		{name: "end migration", commandPath: "yb-voyager end migration", wantStep: "end-migration", wantClaims: true},
		{name: "export data source", commandPath: "yb-voyager export data", exporterRole: SOURCE_DB_EXPORTER_ROLE, wantStep: "export-data-from-source", wantClaims: true},
		{name: "export data from source", commandPath: "yb-voyager export data from source", exporterRole: SOURCE_DB_EXPORTER_ROLE, wantStep: "export-data-from-source", wantClaims: true},
		{name: "export data from target", commandPath: "yb-voyager export data from target", exporterRole: TARGET_DB_EXPORTER_FB_ROLE},
		{name: "export data fall-forward role", commandPath: "yb-voyager export data", exporterRole: TARGET_DB_EXPORTER_FF_ROLE},
		{name: "import data target", commandPath: "yb-voyager import data", importerRole: TARGET_DB_IMPORTER_ROLE, wantStep: "import-data-to-target", wantClaims: true},
		{name: "import data to target", commandPath: "yb-voyager import data to target", importerRole: TARGET_DB_IMPORTER_ROLE, wantStep: "import-data-to-target", wantClaims: true},
		{name: "import data to source", commandPath: "yb-voyager import data to source", importerRole: SOURCE_DB_IMPORTER_ROLE},
		{name: "import data file", commandPath: "yb-voyager import data file", importerRole: IMPORT_FILE_ROLE},
		{name: "unrelated command", commandPath: "yb-voyager status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step, claims := controlPlaneStepFor(tc.commandPath, tc.exporterRole, tc.importerRole, tc.postSnapshot)
			assert.Equal(t, tc.wantClaims, claims)
			if tc.wantClaims {
				assert.Equal(t, tc.wantStep, step)
			}
		})
	}
}
