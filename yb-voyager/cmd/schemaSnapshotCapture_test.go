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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// baseSnapshotContent returns a small, deterministic SnapshotContent used as the
// baseline for the snapshotContentEqual comparisons below.
func baseSnapshotContent() *schemasnapshot.SnapshotContent {
	return &schemasnapshot.SnapshotContent{
		Version:      1,
		DatabaseType: constants.POSTGRESQL,
		Tables: []schemasnapshot.Table{
			{
				ObjectRef: schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
				ID:        "16401",
				Kind:      schemasnapshot.TableKindOrdinary,
			},
		},
		Columns: []schemasnapshot.Column{
			{
				Table:    schemasnapshot.ObjectRef{Schema: "public", Name: "orders"},
				ID:       "16401:1",
				Name:     "id",
				DataType: "integer",
				NotNull:  true,
			},
		},
	}
}

func TestSnapshotContentEqual_NilNil(t *testing.T) {
	assert.True(t, snapshotContentEqual(nil, nil), "nil == nil must be equal")
}

func TestSnapshotContentEqual_NilVsNonNil(t *testing.T) {
	nonNil := baseSnapshotContent()
	assert.False(t, snapshotContentEqual(nil, nonNil), "nil vs non-nil must not be equal")
	assert.False(t, snapshotContentEqual(nonNil, nil), "non-nil vs nil must not be equal")
}

func TestSnapshotContentEqual_IdenticalContent(t *testing.T) {
	a := baseSnapshotContent()
	b := baseSnapshotContent()
	assert.True(t, snapshotContentEqual(a, b), "two separately-built but identical contents must be equal")
}

func TestSnapshotContentEqual_ExtraTableDiffers(t *testing.T) {
	a := baseSnapshotContent()
	b := baseSnapshotContent()
	b.Tables = append(b.Tables, schemasnapshot.Table{
		ObjectRef: schemasnapshot.ObjectRef{Schema: "public", Name: "customers"},
		ID:        "16402",
		Kind:      schemasnapshot.TableKindOrdinary,
	})
	assert.False(t, snapshotContentEqual(a, b), "an added table must be detected as a change")
}

func TestSnapshotContentEqual_DataTypeChangeDiffers(t *testing.T) {
	a := baseSnapshotContent()
	b := baseSnapshotContent()
	b.Columns[0].DataType = "bigint"
	assert.False(t, snapshotContentEqual(a, b), "a changed column DataType must be detected as a change")
}
