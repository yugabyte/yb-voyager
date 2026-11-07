//go:build integration

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
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type batchExecErrorCase struct {
	name string
	// table is created as (id INT PRIMARY KEY, v TEXT UNIQUE) with rows (1,'a'), (2,'b'), (3,'c').
	table  string
	events func(tableTup sqlname.NameTuple) []*Event
	// expectedStmt is set when the batch must fail in pgx preprocessing, and is the statement the error must report.
	expectedStmt string
	// expectedStep is pgx's preprocessing step: "prepare" (server-side parse) or "build" (client-side argument encoding).
	expectedStep string
	// expectedExecErrVsn and expectedSQLState are set when an event must fail while executing on the target instead.
	expectedExecErrVsn int64
	expectedSQLState   string
}

func strPtr(s string) *string { return &s }

func batchEvent(vsn int64, op string, tableTup sqlname.NameTuple, key, fields map[string]*string) *Event {
	return &Event{Vsn: vsn, Op: op, TableNameTup: tableTup, Key: key, Fields: fields}
}

func idKey(id string) map[string]*string { return map[string]*string{"id": strPtr(id)} }

// The failing event is never the first one: the first br.Exec() is where pgx reports every preprocessing failure,
// so a failure at a later position is what shows the error describes the failing statement, not the first event.
var ybBatchExecErrorCases = []batchExecErrorCase{
	{
		name:  "update with an unterminated literal fails to prepare",
		table: "batch_exec_err_update",
		events: func(tt sqlname.NameTuple) []*Event {
			return []*Event{
				batchEvent(101, "u", tt, idKey("1"), map[string]*string{"v": strPtr("'x'")}),
				batchEvent(102, "u", tt, idKey("2"), map[string]*string{"v": strPtr("'unterminated")}),
				batchEvent(103, "u", tt, idKey("3"), map[string]*string{"v": strPtr("'z'")}),
			}
		},
		expectedStmt: `UPDATE "public"."batch_exec_err_update" SET "v" = 'unterminated WHERE "id" = 2`,
		expectedStep: "prepare",
	},
	{
		// Insert events share one prepared statement per table, so pgx reports the voyager statement name; the error
		// must carry that statement's SQL instead.
		name:  "insert with a missing column fails to build its arguments",
		table: `"BatchExecErrInsert"`,
		events: func(tt sqlname.NameTuple) []*Event {
			return []*Event{
				batchEvent(201, "c", tt, idKey("10"), map[string]*string{"id": strPtr("10"), "v": strPtr("x")}),
				batchEvent(202, "c", tt, idKey("11"), map[string]*string{"id": strPtr("11")}),
				batchEvent(203, "c", tt, idKey("12"), map[string]*string{"id": strPtr("12"), "v": strPtr("z")}),
			}
		},
		expectedStmt: `INSERT INTO "public"."BatchExecErrInsert" ("id", "v") VALUES ($1, $2) ON CONFLICT ("id") DO NOTHING`,
		expectedStep: "build",
	},
	{
		name:  "delete with an extra key column fails to build its arguments",
		table: "batch_exec_err_delete",
		events: func(tt sqlname.NameTuple) []*Event {
			return []*Event{
				batchEvent(301, "d", tt, idKey("1"), nil),
				batchEvent(302, "d", tt, map[string]*string{"id": strPtr("2"), "v": strPtr("b")}, nil),
			}
		},
		expectedStmt: `DELETE FROM "public"."batch_exec_err_delete" WHERE "id" = $1`,
		expectedStep: "build",
	},
	{
		name:  "insert of an existing key fails while executing",
		table: "batch_exec_err_exec",
		events: func(tt sqlname.NameTuple) []*Event {
			return []*Event{
				batchEvent(401, "u", tt, idKey("1"), map[string]*string{"v": strPtr("'x'")}),
				// The prepared insert has ON CONFLICT ("id") DO NOTHING, so the duplicate has to be on the unique column v.
				batchEvent(402, "c", tt, idKey("20"), map[string]*string{"id": strPtr("20"), "v": strPtr("b")}),
			}
		},
		expectedExecErrVsn: 402,
		expectedSQLState:   "23505",
	},
}

func TestYugabyteDBExecuteBatchErrorDescribesFailingStatement(t *testing.T) {
	yb, ok := testYugabyteDBTarget.TargetDB.(*TargetYugabyteDB)
	require.True(t, ok, "expected a *TargetYugabyteDB")
	require.NoError(t, yb.CreateVoyagerSchema())

	pool, err := NewConnectionPool(&ConnectionParams{
		NumConnections:    1,
		NumMaxConnections: 1,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		SessionInitScript: yb.Tconf.SessionVars,
	})
	require.NoError(t, err)
	originalPool := yb.connPool
	yb.connPool = pool
	t.Cleanup(func() { yb.connPool = originalPool })

	for _, tc := range ybBatchExecErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			runBatchExecErrorCase(t, testYugabyteDBTarget, constants.YUGABYTEDB, yb.ExecuteBatch, tc)
		})
	}
}

// TargetPostgreSQL queues inserts by their SQL text rather than by statement name, so pgx reports the SQL itself.
func TestPostgresExecuteBatchErrorDescribesFailingStatement(t *testing.T) {
	pg, ok := testPostgresTarget.TargetDB.(*TargetPostgreSQL)
	require.True(t, ok, "expected a *TargetPostgreSQL")

	pool, err := NewConnectionPool(&ConnectionParams{
		NumConnections:    1,
		NumMaxConnections: 1,
		ConnUriList:       []string{testPostgresTarget.GetConnectionString()},
	})
	require.NoError(t, err)
	originalPool := pg.connPool
	pg.connPool = pool
	t.Cleanup(func() { pg.connPool = originalPool })

	tc := batchExecErrorCase{
		table: "batch_exec_err_pg_insert",
		events: func(tt sqlname.NameTuple) []*Event {
			return []*Event{
				batchEvent(501, "c", tt, idKey("10"), map[string]*string{"id": strPtr("10"), "v": strPtr("x")}),
				batchEvent(502, "c", tt, idKey("11"), map[string]*string{"id": strPtr("11")}),
			}
		},
		expectedStmt: `INSERT INTO "public"."batch_exec_err_pg_insert" ("id", "v") VALUES ($1, $2) ON CONFLICT ("id") DO NOTHING`,
		expectedStep: "build",
	}
	runBatchExecErrorCase(t, testPostgresTarget, constants.POSTGRESQL, pg.ExecuteBatch, tc)
}

func runBatchExecErrorCase(t *testing.T, target *TestDB, dbType string, executeBatch func(uuid.UUID, *EventBatch) error, tc batchExecErrorCase) {
	target.ExecuteSqls(
		fmt.Sprintf(`CREATE TABLE public.%s (id INT PRIMARY KEY, v TEXT UNIQUE);`, tc.table),
		fmt.Sprintf(`INSERT INTO public.%s VALUES (1, 'a'), (2, 'b'), (3, 'c');`, tc.table))
	t.Cleanup(func() { target.ExecuteSqls(fmt.Sprintf(`DROP TABLE public.%s;`, tc.table)) })

	o := sqlname.NewObjectName(dbType, "public", "public", tc.table)
	batch := NewEventBatch(tc.events(sqlname.NameTuple{SourceName: o, TargetName: o, CurrentName: o}), 0)

	err := executeBatch(uuid.New(), batch)
	require.Error(t, err)

	var preprocessingErr pgx.ErrPreprocessingBatch
	if tc.expectedStmt != "" {
		require.True(t, errors.As(err, &preprocessingErr), "expected pgx.ErrPreprocessingBatch, got %v", err)
		assert.Contains(t, err.Error(), fmt.Sprintf("error preprocessing stmt [%s] in batch(%s): error preprocessing batch (%s)",
			tc.expectedStmt, batch.ID(), tc.expectedStep))
		return
	}
	assert.False(t, errors.As(err, &preprocessingErr), "expected an execution error, got %v", err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected *pgconn.PgError, got %v", err)
	assert.Equal(t, tc.expectedSQLState, pgErr.Code)
	assert.Contains(t, err.Error(), fmt.Sprintf("error executing stmt for event with vsn(%d) in batch(%s)", tc.expectedExecErrVsn, batch.ID()))
}
