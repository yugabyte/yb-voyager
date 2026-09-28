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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type mockTargetDBForPrimaryKeyColumns struct {
	tgtdb.TargetYugabyteDB
	ownPKs  map[string][]string
	leafPKs map[string][]string
	err     error

	calls                   int
	includeLeafPartitionPKs bool
}

func (m *mockTargetDBForPrimaryKeyColumns) GetPrimaryKeyColumnsForTables(tables []sqlname.NameTuple, includeLeafPartitionPKs bool) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	m.calls++
	m.includeLeafPartitionPKs = includeLeafPartitionPKs
	if m.err != nil {
		return nil, m.err
	}
	result := utils.NewStructMap[sqlname.NameTuple, []string]()
	for _, t := range tables {
		if pk, ok := m.ownPKs[t.ForKey()]; ok {
			result.Put(t, pk)
		} else if pk, ok := m.leafPKs[t.ForKey()]; ok && includeLeafPartitionPKs {
			result.Put(t, pk)
		}
	}
	return result, nil
}

func TestGetPrimaryKeyColumnsForImportTables(t *testing.T) {
	plainTable := testCdcPartitionNameTuple("public", "customers")
	leafPKOnlyRoot := testCdcPartitionNameTuple("public", "orders")

	tests := []struct {
		name             string
		importerRole     string
		importType       string
		sourceDBType     string
		usePartitionRoot bool
		tdbErr           error

		expectQueried     bool
		expectIncludeLeaf bool
		expectPKs         map[string][]string
		expectErr         string
	}{
		{
			name:         "offline skips the lookup",
			importerRole: TARGET_DB_IMPORTER_ROLE, importType: SNAPSHOT_ONLY, sourceDBType: POSTGRESQL, usePartitionRoot: true,
			expectPKs: map[string][]string{},
		},
		{
			name:         "non-PostgreSQL source skips the lookup",
			importerRole: TARGET_DB_IMPORTER_ROLE, importType: SNAPSHOT_AND_CHANGES, sourceDBType: ORACLE, usePartitionRoot: true,
			expectPKs: map[string][]string{},
		},
		{
			name:         "target with --use-partition-root false takes the leaf PK",
			importerRole: TARGET_DB_IMPORTER_ROLE, importType: SNAPSHOT_AND_CHANGES, sourceDBType: POSTGRESQL, usePartitionRoot: false,
			expectQueried: true, expectIncludeLeaf: true,
			expectPKs: map[string][]string{plainTable.ForKey(): {"id"}, leafPKOnlyRoot.ForKey(): {"order_id"}},
		},
		{
			name:         "target with --use-partition-root true refuses a root without its own PK",
			importerRole: TARGET_DB_IMPORTER_ROLE, importType: CHANGES_ONLY, sourceDBType: POSTGRESQL, usePartitionRoot: true,
			expectQueried: true,
			expectErr: "table(s) public.orders have no primary key; live migration is not allowed for these tables. " +
				"If these are partitioned tables with a primary key only on their partitions, " +
				"re-run with '--use-partition-root false' (requires YugabyteDB 2025.2.3.0 or later)",
		},
		{
			name:         "fall-back refuses a root without its own PK",
			importerRole: SOURCE_DB_IMPORTER_ROLE, importType: SNAPSHOT_AND_CHANGES, sourceDBType: POSTGRESQL, usePartitionRoot: true,
			expectQueried: true,
			expectErr: "table(s) public.orders have no primary key; live migration is not allowed for these tables. " +
				"If these are partitioned tables with a primary key only on their partitions, re-run with '--use-partition-root false'",
		},
		{
			name:         "fall-forward refuses a root without its own PK",
			importerRole: SOURCE_REPLICA_DB_IMPORTER_ROLE, importType: SNAPSHOT_AND_CHANGES, sourceDBType: POSTGRESQL, usePartitionRoot: true,
			expectQueried: true,
			expectErr: "table(s) public.orders have no primary key; live migration is not allowed for these tables. " +
				"Partitioned tables with a primary key only on their partitions are not supported with fall-forward; " +
				"add a primary key on the root table or exclude these tables from the migration",
		},
		{
			name:         "lookup failure is returned",
			importerRole: TARGET_DB_IMPORTER_ROLE, importType: SNAPSHOT_AND_CHANGES, sourceDBType: POSTGRESQL, usePartitionRoot: false,
			tdbErr:        errors.New("connection refused"),
			expectQueried: true, expectIncludeLeaf: true,
			expectErr: "error getting primary key columns for import tables: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origTdb, origRole, origImportType, origSourceDBType, origUsePartitionRoot := tdb, importerRole, importType, sourceDBType, tconf.UsePartitionRoot
			t.Cleanup(func() {
				tdb, importerRole, importType, sourceDBType, tconf.UsePartitionRoot = origTdb, origRole, origImportType, origSourceDBType, origUsePartitionRoot
			})
			mock := &mockTargetDBForPrimaryKeyColumns{
				ownPKs:  map[string][]string{plainTable.ForKey(): {"id"}},
				leafPKs: map[string][]string{leafPKOnlyRoot.ForKey(): {"order_id"}},
				err:     tt.tdbErr,
			}
			tdb, importerRole, importType, sourceDBType, tconf.UsePartitionRoot = mock, tt.importerRole, tt.importType, tt.sourceDBType, tt.usePartitionRoot

			result, err := getPrimaryKeyColumnsForImportTables([]sqlname.NameTuple{plainTable, leafPKOnlyRoot})

			assert.Equal(t, tt.expectQueried, mock.calls == 1)
			if tt.expectQueried {
				assert.Equal(t, tt.expectIncludeLeaf, mock.includeLeafPartitionPKs)
			}
			if tt.expectErr != "" {
				require.EqualError(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
			got := map[string][]string{}
			require.NoError(t, result.IterKV(func(table sqlname.NameTuple, pk []string) (bool, error) {
				got[table.ForKey()] = pk
				return true, nil
			}))
			assert.Equal(t, tt.expectPKs, got)
		})
	}
}
