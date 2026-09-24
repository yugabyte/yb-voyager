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

package schemasnapshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLabelReason_Legal(t *testing.T) {
	cases := []struct{ label, reason string }{
		{LabelExportSchema, ""},
		{LabelExportDataFromSourceStart, "initial"},
		{LabelExportDataFromSourceStart, "resume"},
		{LabelExportDataFromSourceStart, "clean_restart"},
		{LabelExportDataFromSourcePeriodic, ""},
		{LabelExportDataFromSourceExit, "cutover"},
		{LabelExportDataFromSourceExit, "complete"},
		{LabelExportDataFromSourceExit, "interrupt"},
		{LabelExportDataFromSourceExit, "error"},
	}
	for _, c := range cases {
		t.Run(c.label+"/"+c.reason, func(t *testing.T) {
			assert.NoError(t, ValidateLabelReason(c.label, c.reason))
		})
	}
}

func TestValidateLabelReason_Illegal(t *testing.T) {
	cases := []struct {
		label  string
		reason string
		errMsg string
	}{
		{"bad_label", "", "unknown snapshot label"},
		{LabelExportSchema, "anything", "does not accept a reason"},
		{LabelExportDataFromSourcePeriodic, "bogus", "does not accept a reason"},
		{LabelExportDataFromSourceStart, "", "requires a reason"},
		{LabelExportDataFromSourceExit, "bogus", "does not accept reason"},
		{LabelExportDataFromSourceExit, "", "requires a reason"},
	}
	for _, c := range cases {
		t.Run(c.label+"/"+c.reason, func(t *testing.T) {
			err := ValidateLabelReason(c.label, c.reason)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errMsg)
		})
	}
}
