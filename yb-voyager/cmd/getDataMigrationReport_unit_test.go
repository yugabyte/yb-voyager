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
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"
	pgconn5 "github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// fakeQueryErrTDB embeds the TargetDB interface (methods are nil at runtime but
// only Query is exercised here) and forces Query to return a fixed error.
type fakeQueryErrTDB struct {
	tgtdb.TargetDB
	queryErr error
}

func (f *fakeQueryErrTDB) Query(string) (*sql.Rows, error) { return nil, f.queryErr }

func newTestTuple(name string) sqlname.NameTuple {
	obj := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", name)
	return sqlname.NameTuple{SourceName: obj, CurrentName: obj}
}

func TestGetImportedEventsStatsForTableListMissingMetadataTable(t *testing.T) {
	origTdb := tdb
	defer func() { tdb = origTdb }()
	tdb = &fakeQueryErrTDB{queryErr: &pgconn5.PgError{Code: "42P01", Message: `relation "ybvoyager_metadata.ybvoyager_imported_event_count_by_table" does not exist`}}

	tups := []sqlname.NameTuple{newTestTuple("t1"), newTestTuple("t2")}
	state := NewImportDataState(t.TempDir())
	got, err := state.GetImportedEventsStatsForTableList(tups, uuid.New())
	require.NoError(t, err, "42P01 (table not created yet) must be treated as zero events, not an error")
	require.NotNil(t, got)
	for _, tup := range tups {
		c, ok := got.Get(tup)
		require.True(t, ok)
		require.NotNil(t, c)
		require.Zero(t, c.NumInserts)
		require.Zero(t, c.NumUpdates)
		require.Zero(t, c.NumDeletes)
	}
}

func TestGetImportedEventsStatsForTableListPropagatesOtherErrors(t *testing.T) {
	origTdb := tdb
	defer func() { tdb = origTdb }()
	tdb = &fakeQueryErrTDB{queryErr: fmt.Errorf("connection refused")}
	state := NewImportDataState(t.TempDir())
	_, err := state.GetImportedEventsStatsForTableList([]sqlname.NameTuple{newTestTuple("t1")}, uuid.New())
	require.Error(t, err, "non-42P01 errors must still propagate")
}
