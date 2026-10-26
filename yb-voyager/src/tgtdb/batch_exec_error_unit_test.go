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
package tgtdb

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// The pgx.ErrPreprocessingBatch branch is covered by TestExecuteBatchNamesEventOfUnpreparableStatement
// (integration), since only pgx can construct that error.
func TestDescribeBatchExecErrorNonPreprocessing(t *testing.T) {
	o := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "t")
	tableTup := sqlname.NameTuple{SourceName: o, TargetName: o, CurrentName: o}
	events := []*Event{{Vsn: 11, Op: "c", TableNameTup: tableTup}, {Vsn: 12, Op: "c", TableNameTup: tableTup}, {Vsn: 13, Op: "c", TableNameTup: tableTup}}
	batch := NewEventBatch(events, 4)
	pgxBatch := &pgx.Batch{}
	for range events {
		pgxBatch.Queue("SELECT 1")
	}
	dbErr := errors.New("duplicate key value violates unique constraint")

	tests := []struct {
		name     string
		i        int
		expected string
	}{
		{name: "first exec", i: 0, expected: "error sending batch(" + batch.ID() + ") or executing event with vsn(11)"},
		{name: "later exec", i: 2, expected: "error executing stmt for event with vsn(13) in batch(" + batch.ID() + ")"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, describeBatchExecError(dbErr, pgxBatch, batch, tt.i))
		})
	}
}
