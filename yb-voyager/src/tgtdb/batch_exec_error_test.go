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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func TestExecuteBatchNamesEventOfUnpreparableStatement(t *testing.T) {
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

	testYugabyteDBTarget.ExecuteSqls(`CREATE TABLE public.batch_exec_error (id INT PRIMARY KEY, v TEXT);`,
		`INSERT INTO public.batch_exec_error VALUES (1, 'a'), (2, 'b'), (3, 'c');`)
	t.Cleanup(func() { testYugabyteDBTarget.ExecuteSqls(`DROP TABLE public.batch_exec_error;`) })

	migrationUUID := uuid.New()
	const chanNo = 0
	testYugabyteDBTarget.ExecuteSqls(fmt.Sprintf("INSERT INTO %s VALUES ('%s', %d, -1, 0, 0, 0);", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID, chanNo))

	o := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "batch_exec_error")
	tableTup := sqlname.NameTuple{SourceName: o, TargetName: o, CurrentName: o}
	updateEvent := func(vsn int64, id string, literal string) *Event {
		return &Event{Vsn: vsn, Op: "u", TableNameTup: tableTup,
			Key: map[string]*string{"id": &id}, Fields: map[string]*string{"v": &literal}}
	}
	batch := NewEventBatch([]*Event{
		updateEvent(101, "1", "'x'"),
		// Unterminated literal: YugabyteDB rejects it while pgx prepares the batch, before any event executes.
		updateEvent(102, "2", "'unterminated"),
		updateEvent(103, "3", "'z'"),
	}, chanNo)

	err = yb.ExecuteBatch(migrationUUID, batch)

	var preprocessingErr pgx.ErrPreprocessingBatch
	require.True(t, errors.As(err, &preprocessingErr), "expected pgx.ErrPreprocessingBatch, got %v", err)
	assert.Contains(t, err.Error(), fmt.Sprintf("error preprocessing statement first queued by event with vsn(102) in batch(%s)", batch.ID()))
}
