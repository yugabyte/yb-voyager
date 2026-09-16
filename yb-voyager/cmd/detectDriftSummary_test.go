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

// TestWrapDriftValue pins the wrapping behind the summary block's hanging indent:
// values wrap on whitespace so continuation lines can be indented to the value
// column, and a token with nothing to break on is never split — a qualified table
// name or a report path has to stay copy-pasteable.
func TestWrapDriftValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		width int
		want  []string
	}{
		{
			name:  "short value stays on one line",
			value: "all 2 — sales.orders, sales.customers",
			width: 60,
			want:  []string{"all 2 — sales.orders, sales.customers"},
		},
		{
			name:  "long value wraps on whitespace",
			value: "all 4 — sales.aaaa, sales.bbbb, sales.cccc, sales.dddd",
			width: 25,
			want: []string{
				"all 4 — sales.aaaa,",
				"sales.bbbb, sales.cccc,",
				"sales.dddd",
			},
		},
		{
			name:  "an unbreakable token is never split",
			value: "/very/long/path/to/exportdir/reports/drift_analysis_report.html",
			width: 20,
			want:  []string{"/very/long/path/to/exportdir/reports/drift_analysis_report.html"},
		},
		{
			name:  "empty value yields no lines",
			value: "",
			width: 40,
			want:  nil,
		},
		{
			name:  "non-positive width degrades to a single line rather than looping",
			value: "all 2 — sales.orders, sales.customers",
			width: 0,
			want:  []string{"all 2 — sales.orders, sales.customers"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapDriftValue(tt.value, tt.width)
			assert.Equal(t, tt.want, got)
			for _, l := range got {
				assert.NotEqual(t, " ", l[:1], "a wrapped line must not start with padding; the caller adds the indent")
			}
		})
	}
}
