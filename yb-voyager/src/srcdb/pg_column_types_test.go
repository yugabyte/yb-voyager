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
)

func TestIsColumnTypeUnsupported(t *testing.T) {
	unsupportedTypeNames := []string{"XML", "PG_LSN"}
	unsupportedTypTypes := []string{pgTypTypeRange}

	testCases := []struct {
		name        string
		columnTypes []columnType
		expected    bool
	}{
		{"supported declared type", []columnType{{"int4", "b"}}, false},
		{"unsupported declared type, case-insensitive", []columnType{{"xml", "b"}}, true},
		{"unsupported array element", []columnType{{"_xml", "b"}, {"xml", "b"}}, true},
		{"unsupported type under stacked domains", []columnType{{"lsn_dom_outer", "d"}, {"lsn_dom", "d"}, {"pg_lsn", "b"}}, true},
		{"unsupported type in the middle of the chain", []columnType{{"range_dom", "d"}, {"public.float_range", "r"}, {"float8", "b"}}, true},
		{"unsupported composite field", []columnType{{"public.geo", "c"}, {"text", "b"}, {"pg_lsn", "b"}}, true},
		{"composite of supported fields", []columnType{{"public.address", "c"}, {"text", "b"}, {"int4", "b"}}, false},
		{"name match is exact, not a prefix", []columnType{{"xml_dom", "d"}, {"text", "b"}}, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isColumnTypeUnsupported(tc.columnTypes, unsupportedTypeNames, unsupportedTypTypes))
		})
	}
}
