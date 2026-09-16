//go:build unit

// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driftreport

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

func TestPhaseFor(t *testing.T) {
	cases := []struct {
		name       string
		prev, next string
		want       string
	}{
		{
			name: "export schema -> export data start",
			prev: schemasnapshot.LabelExportSchema,
			next: schemasnapshot.LabelExportDataFromSourceStart,
			want: "export data: pending",
		},
		{
			name: "export data start -> periodic",
			prev: schemasnapshot.LabelExportDataFromSourceStart,
			next: schemasnapshot.LabelExportDataFromSourcePeriodic,
			want: "export data: running",
		},
		{
			name: "periodic -> periodic",
			prev: schemasnapshot.LabelExportDataFromSourcePeriodic,
			next: schemasnapshot.LabelExportDataFromSourcePeriodic,
			want: "export data: running",
		},
		{
			name: "export data exit -> export data start (paused)",
			prev: schemasnapshot.LabelExportDataFromSourceExit,
			next: schemasnapshot.LabelExportDataFromSourceStart,
			want: "export data: paused",
		},
		{
			name: "anything -> live",
			prev: schemasnapshot.LabelExportDataFromSourcePeriodic,
			next: SeriesSourceLive,
			want: "since last capture",
		},
		{
			name: "export schema -> live",
			prev: schemasnapshot.LabelExportSchema,
			next: SeriesSourceLive,
			want: "since last capture",
		},
		{
			name: "fallback: unrelated pair",
			prev: schemasnapshot.LabelExportSchema,
			next: schemasnapshot.LabelExportDataFromSourceExit,
			want: "",
		},
		{
			name: "fallback: export data exit -> periodic (not a defined transition)",
			prev: schemasnapshot.LabelExportDataFromSourceExit,
			next: schemasnapshot.LabelExportDataFromSourcePeriodic,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev := Capture{Series: tc.prev}
			next := Capture{Series: tc.next}
			assert.Equal(t, tc.want, phaseFor(prev, next))
		})
	}
}
