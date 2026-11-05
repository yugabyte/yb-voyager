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
package sqlname

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

func TestFindBestMatchingColumnName(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		column   string
		columns  []string
		expected string
		errMsg   string
	}{
		{name: "exact match", dbType: constants.POSTGRESQL, column: "Note", columns: []string{"note", "Note"}, expected: "Note"},
		{name: "quoted exact match", dbType: constants.POSTGRESQL, column: `"Note"`, columns: []string{"note", "Note"}, expected: "Note"},
		{name: "case-insensitive single match", dbType: constants.POSTGRESQL, column: "NOTE", columns: []string{"id", "Note"}, expected: "Note"},
		{name: "several case variants prefer lowercase on PostgreSQL", dbType: constants.POSTGRESQL, column: "nOtE", columns: []string{"Note", "note"}, expected: "note"},
		{name: "several case variants prefer lowercase on YugabyteDB", dbType: constants.YUGABYTEDB, column: "nOtE", columns: []string{"Note", "note"}, expected: "note"},
		{name: "several case variants prefer uppercase on Oracle", dbType: constants.ORACLE, column: "nOtE", columns: []string{"Note", "NOTE"}, expected: "NOTE"},
		{
			name: "ambiguous", dbType: constants.POSTGRESQL, column: "nOtE", columns: []string{"Note", "NOTE"},
			errMsg: `ambiguous column name "nOtE" in table: found column names: Note, NOTE`,
		},
		{
			name: "not found", dbType: constants.POSTGRESQL, column: "missing", columns: []string{"id"},
			errMsg: `column name "missing" not found amongst table columns [id]`,
		},
		{
			name: "single quote character is not treated as quoted", dbType: constants.POSTGRESQL, column: `"`, columns: []string{"id"},
			errMsg: `column name "\"" not found amongst table columns [id]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column, err := FindBestMatchingColumnName(tt.dbType, tt.column, tt.columns)
			if tt.errMsg != "" {
				assert.EqualError(t, err, tt.errMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, column)
		})
	}
}
