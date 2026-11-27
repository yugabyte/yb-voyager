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
package cp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStepForCommand(t *testing.T) {
	for _, tc := range []struct {
		name         string
		commandPath  string
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
		{name: "export data", commandPath: "yb-voyager export data", wantStep: "export-data-from-source", wantClaims: true},
		{name: "export data from source", commandPath: "yb-voyager export data from source", wantStep: "export-data-from-source", wantClaims: true},
		{name: "export data from target", commandPath: "yb-voyager export data from target"},
		{name: "import data", commandPath: "yb-voyager import data", wantStep: "import-data-to-target", wantClaims: true},
		{name: "import data to target", commandPath: "yb-voyager import data to target", wantStep: "import-data-to-target", wantClaims: true},
		{name: "import data to source", commandPath: "yb-voyager import data to source"},
		{name: "import data to source-replica", commandPath: "yb-voyager import data to source-replica"},
		{name: "import data file", commandPath: "yb-voyager import data file"},
		{name: "unrelated command", commandPath: "yb-voyager status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step, claims := StepForCommand(tc.commandPath, tc.postSnapshot)
			assert.Equal(t, tc.wantClaims, claims)
			if tc.wantClaims {
				assert.Equal(t, tc.wantStep, step)
			}
		})
	}
}
