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
package srcdb

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// TestGetSchemaListQuotedVsUnquoted pins the distinction between the two schema-list
// accessors, which is invisible for ordinary lowercase names and load-bearing for any
// name that needs quoting.
//
// GetSchemaList returns the min-QUOTED form, for interpolating into SQL as an
// identifier. GetSchemaListUnquoted returns the RAW catalog value, for comparing
// against catalog data such as pg_namespace.nspname. Using the quoted form there
// makes a `nspname IN (...)` predicate match nothing, which silently captured an
// empty schema snapshot until this was split (verified against PostgreSQL).
func TestGetSchemaListQuotedVsUnquoted(t *testing.T) {
	tests := []struct {
		name         string
		schema       string
		wantQuoted   string
		wantUnquoted string
	}{
		{"lowercase name: both forms identical", "sales", "sales", "sales"},
		{"name with a space must be quoted for SQL, raw for the catalog", "Odd Schema", `"Odd Schema"`, "Odd Schema"},
		{"mixed-case name must be quoted for SQL, raw for the catalog", "SalesOps", `"SalesOps"`, "SalesOps"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{
				DBType:  "postgresql",
				Schemas: []sqlname.Identifier{sqlname.NewIdentifier("postgresql", tt.schema)},
			}
			assert.Equal(t, []string{tt.wantQuoted}, s.GetSchemaList(), "GetSchemaList must be SQL-quotable")
			assert.Equal(t, []string{tt.wantUnquoted}, s.GetSchemaListUnquoted(), "GetSchemaListUnquoted must be the raw catalog value")
		})
	}
}
