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

package schemadrift

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
)

// Diff and DriftInfo are embedded so the report's JSON stays flat: a named field
// would nest them under a key and change the contract.
func TestDriftEntryJSONIsFlat(t *testing.T) {
	entry := DriftEntry{
		Diff: Diff{
			Type:       schemadiff.ColumnTypeChanged,
			Operation:  schemadiff.OpChanged,
			ObjectType: schemadiff.ObjectTypeColumn,
			Attribute:  schemadiff.AttrType,
			Object:     objRef("public", "orders"),
			SubObject:  "amount",
			OldValue:   "integer",
			NewValue:   "bigint",
		},
		Window:    Window{From: t1(), To: t2()},
		Phase:     "export data: running",
		DriftInfo: getDriftInfo(schemadiff.ColumnTypeChanged),
	}

	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))

	assert.ElementsMatch(t, []string{
		"type", "operation", "object_type", "attribute",
		"object", "sub_object", "old_value", "new_value",
		"window", "phase",
		"severity", "impact", "action",
	}, lo.Keys(fields))
}
