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
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func excludeColumnsTestTable(schema, table string) sqlname.NameTuple {
	obj := sqlname.NewObjectName(POSTGRESQL, "public", schema, table)
	return sqlname.NameTuple{SourceName: obj, CurrentName: obj}
}

func TestParseExcludeColumnList(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected map[string][]string
		errMsg   string
	}{
		{
			name:     "empty value",
			value:    "",
			expected: map[string][]string{},
		},
		{
			name:  "qualified and bare tables, columns keep their case",
			value: "public.orders:Note,meta; items : a , B",
			expected: map[string][]string{
				"public.orders": {"Note", "meta"},
				"items":         {"a", "B"},
			},
		},
		{
			name:  "quoted names are kept as typed and may contain separators",
			value: `"Sales"."Orders;2024":"Geo:Point","a,b";public.t:"Mixed"`,
			expected: map[string][]string{
				`"Sales"."Orders;2024"`: {`"Geo:Point"`, `"a,b"`},
				"public.t":              {`"Mixed"`},
			},
		},
		{
			name:  "trailing semicolon",
			value: "public.t:a;",
			expected: map[string][]string{
				"public.t": {"a"},
			},
		},
		{
			name:   "entry without colon",
			value:  "public.t",
			errMsg: `invalid --exclude-column-list entry "public.t": expected format '<table>:<column1>,<column2>,...'`,
		},
		{
			name:   "entry with two colons",
			value:  "public.t:a:b",
			errMsg: `invalid --exclude-column-list entry "public.t:a:b": expected format '<table>:<column1>,<column2>,...'`,
		},
		{
			name:   "missing table name",
			value:  ":a",
			errMsg: `invalid --exclude-column-list entry ":a": table and value must both be non-empty, expected format '<table>:<column1>,<column2>,...'`,
		},
		{
			name:   "empty column name",
			value:  "public.t:a,,b",
			errMsg: `invalid --exclude-column-list entry for table "public.t": empty column name`,
		},
		{
			name:   "empty column list",
			value:  "public.t:",
			errMsg: `invalid --exclude-column-list entry "public.t:": table and value must both be non-empty, expected format '<table>:<column1>,<column2>,...'`,
		},
		{
			name:   "unbalanced quotes",
			value:  `public.t:"a`,
			errMsg: `invalid --exclude-column-list "public.t:\"a": unbalanced double quotes`,
		},
		{
			name:   "only separators",
			value:  " ; ;",
			errMsg: `invalid --exclude-column-list " ; ;": no entries, expected format '<table>:<column1>,<column2>,...'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := parseTableColumnsFlag(excludeColumnListFlagName, tt.value)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, entries)
		})
	}
}

func TestExcludeColumnListFlagValueRoundTrips(t *testing.T) {
	recorded := map[string][]string{
		`"public"."t2"`:      {"b"},
		`"public"."Orders"`:  {"note", "Meta", "select"},
		`"Sales"."orders:1"`: {"geo"},
	}
	value := formatExcludeColumnList(recorded)
	assert.Equal(t, `"Sales"."orders:1":geo;"public"."Orders":note,Meta,select;"public"."t2":b`, value)

	entries, err := parseTableColumnsFlag(excludeColumnListFlagName, value)
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		`"Sales"."orders:1"`: {"geo"},
		`"public"."Orders"`:  {"note", "Meta", "select"},
		`"public"."t2"`:      {"b"},
	}, entries)
}

func TestResolveExcludeColumnList(t *testing.T) {
	orders := excludeColumnsTestTable("public", "orders")
	lookup := func(name string) (sqlname.NameTuple, error) {
		switch name {
		case "orders", "public.orders":
			return orders, nil
		}
		return sqlname.NameTuple{}, &namereg.ErrNameNotFound{ObjectType: "table", Name: name}
	}
	entries := map[string][]string{
		"orders":         {"a"},
		"public.missing": {"b"},
		"public.orders":  {"c"},
	}
	resolved, problems := resolveExcludeColumnList(entries, lookup)
	assert.Equal(t, []string{
		"public.missing: table not found",
		"public.orders: table public.orders is already listed as orders",
	}, problems)
	columns, ok := resolved.Get(orders)
	require.True(t, ok)
	assert.Equal(t, []string{"a"}, columns)
	assert.Equal(t, 1, len(resolved.Keys()))
}

// excludeColumnsTestExport is an export of a regular table "orders" and a table "sales"
// partitioned into "sales_q1" and "sales_q2"; "Geo" is case-sensitive and unsupported.
type excludeColumnsTestExport struct {
	orders, sales, salesQ1, salesQ2 sqlname.NameTuple
	exportTables                    exportTableColumnMetadata
}

func newExcludeColumnsTestExport() excludeColumnsTestExport {
	e := excludeColumnsTestExport{
		orders:  excludeColumnsTestTable("public", "orders"),
		sales:   excludeColumnsTestTable("public", "sales"),
		salesQ1: excludeColumnsTestTable("public", "sales_q1"),
		salesQ2: excludeColumnsTestTable("public", "sales_q2"),
	}
	ordersColumns := []string{"id", "note", "doc"}
	salesColumns := []string{"id", "region", "Geo"}

	columns := utils.NewStructMap[sqlname.NameTuple, []string]()
	primaryKeyColumns := utils.NewStructMap[sqlname.NameTuple, []string]()
	supported := utils.NewStructMap[sqlname.NameTuple, []string]()
	unsupported := utils.NewStructMap[sqlname.NameTuple, []string]()
	columns.Put(e.orders, ordersColumns)
	primaryKeyColumns.Put(e.orders, []string{"id"})
	supported.Put(e.orders, []string{"id", "note"})
	unsupported.Put(e.orders, []string{"doc"})
	for _, table := range []sqlname.NameTuple{e.sales, e.salesQ1, e.salesQ2} {
		columns.Put(table, salesColumns)
		primaryKeyColumns.Put(table, []string{"id", "region"})
		supported.Put(table, []string{"id", "region"})
		unsupported.Put(table, []string{"Geo"})
	}
	e.exportTables = exportTableColumnMetadata{
		dbType:    POSTGRESQL,
		tableList: []sqlname.NameTuple{e.orders, e.salesQ1, e.salesQ2, e.sales},
		leafToRoot: map[string]string{
			"public.sales_q1": "public.sales",
			"public.sales_q2": "public.sales",
		},
		tableToColumns:           columns,
		tableToPrimaryKeyColumns: primaryKeyColumns,
		supportedColumns:         supported,
		unsupportedColumns:       unsupported,
	}
	return e
}

func (e excludeColumnsTestExport) rootOfTable(table sqlname.NameTuple) (sqlname.NameTuple, error) {
	if table.ForKey() == e.salesQ1.ForKey() || table.ForKey() == e.salesQ2.ForKey() {
		return e.sales, nil
	}
	return table, nil
}

func excludedColumnsMap(entries map[sqlname.NameTuple][]string) *utils.StructMap[sqlname.NameTuple, []string] {
	m := utils.NewStructMap[sqlname.NameTuple, []string]()
	for table, columns := range entries {
		m.Put(table, columns)
	}
	return m
}

// lookupTable resolves the fixture's tables, and public.audit, which is not exported, by their
// ForOutput names.
func (e excludeColumnsTestExport) lookupTable(name string) (sqlname.NameTuple, error) {
	for _, table := range []sqlname.NameTuple{e.orders, e.sales, e.salesQ1, e.salesQ2, excludeColumnsTestTable("public", "audit")} {
		if table.ForOutput() == name {
			return table, nil
		}
	}
	return sqlname.NameTuple{}, &namereg.ErrNameNotFound{ObjectType: "table", Name: name}
}

func TestValidateExcludedColumns(t *testing.T) {
	e := newExcludeColumnsTestExport()
	excludeTableHint := "exclude the whole table with --exclude-table-list"

	tests := []struct {
		name     string
		excluded map[string][]string
		expected []string
	}{
		{
			name:     "valid columns on a table and a partitioned root",
			excluded: map[string][]string{"public.orders": {"doc", "note"}, "public.sales": {"Geo"}},
			expected: nil,
		},
		{
			name:     "table that does not exist",
			excluded: map[string][]string{"public.missing": {"a"}},
			expected: []string{"public.missing: table not found"},
		},
		{
			name:     "column that does not exist",
			excluded: map[string][]string{"public.orders": {"foo"}},
			expected: []string{`public.orders: column "foo" does not exist`},
		},
		{
			name:     "two spellings of one column",
			excluded: map[string][]string{"public.orders": {"Doc", "doc"}},
			expected: []string{`public.orders: column "doc" is listed more than once`},
		},
		{
			name:     "leaf partition named directly",
			excluded: map[string][]string{"public.sales_q1": {"Geo"}},
			expected: []string{"public.sales_q1: is a partition of public.sales; specify the root table public.sales"},
		},
		{
			name:     "table not part of the export",
			excluded: map[string][]string{"public.audit": {"a"}},
			expected: []string{"public.audit: table is not part of this export; check --table-list and --exclude-table-list"},
		},
		{
			name:     "primary key column",
			excluded: map[string][]string{"public.sales": {"region"}},
			expected: []string{`public.sales: column "region" is part of the primary key and cannot be excluded`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, problems, err := validateExcludedColumns(tt.excluded, e.lookupTable, e.exportTables, e.rootOfTable, excludeTableHint)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expected, problems)
		})
	}

	t.Run("columns are matched to the table's columns ignoring case", func(t *testing.T) {
		excluded, problems, err := validateExcludedColumns(map[string][]string{"public.orders": {"DOC", "Note"}, "public.sales": {"geo"}},
			e.lookupTable, e.exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Empty(t, problems)
		ordersColumns, _ := excluded.Get(e.orders)
		assert.Equal(t, []string{"doc", "note"}, ordersColumns)
		salesColumns, _ := excluded.Get(e.sales)
		assert.Equal(t, []string{"Geo"}, salesColumns)
	})

	t.Run("a column matching several columns only by case", func(t *testing.T) {
		columns := utils.NewStructMap[sqlname.NameTuple, []string]()
		columns.Put(e.orders, []string{"id", "Note", "NOTE", "note"})
		exportTables := e.exportTables
		exportTables.tableToColumns = columns

		excluded, problems, err := validateExcludedColumns(map[string][]string{"public.orders": {"nOtE", "NOTE"}}, e.lookupTable, exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Empty(t, problems, "an unquoted spelling resolves to the lowercase column, an exact spelling to itself")
		matched, _ := excluded.Get(e.orders)
		assert.Equal(t, []string{"note", "NOTE"}, matched)

		columns.Put(e.orders, []string{"id", "Note", "NOTE"})
		_, problems, err = validateExcludedColumns(map[string][]string{"public.orders": {"note"}}, e.lookupTable, exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Equal(t, []string{`public.orders: column "note" is ambiguous; write the column's exact case`}, problems)
	})

	t.Run("primary key column with an unsupported datatype points to exclude-table-list", func(t *testing.T) {
		unsupported := utils.NewStructMap[sqlname.NameTuple, []string]()
		unsupported.Put(e.orders, []string{"id"})
		exportTables := e.exportTables
		exportTables.unsupportedColumns = unsupported
		_, problems, err := validateExcludedColumns(map[string][]string{"public.orders": {"id"}}, e.lookupTable, exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Equal(t, []string{`public.orders: column "id" is part of the primary key and cannot be excluded. Its datatype is not supported, so exclude the whole table with --exclude-table-list`}, problems)
	})
}

func TestApplyExcludedColumns(t *testing.T) {
	t.Run("excluded unsupported columns pass and partitions get the root's exclusions", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		excluded := excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"doc"}, e.sales: {"Geo"}})
		columnList, report, err := applyExcludedColumns(excluded, e.exportTables)
		require.NoError(t, err)
		assert.True(t, report.isEmpty())
		for table, expected := range map[sqlname.NameTuple][]string{
			e.orders:  {"id", "note"},
			e.sales:   {"id", "region"},
			e.salesQ1: {"id", "region"},
			e.salesQ2: {"id", "region"},
		} {
			columns, ok := columnList.Get(table)
			require.True(t, ok, table.ForOutput())
			assert.Equal(t, expected, columns, table.ForOutput())
		}
	})

	t.Run("excluding a supported column of a fully supported table expands *", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		e.exportTables.supportedColumns.Put(e.orders, []string{"*"})
		e.exportTables.unsupportedColumns.Delete(e.orders)
		excluded := excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note"}, e.sales: {"Geo"}})
		columnList, report, err := applyExcludedColumns(excluded, e.exportTables)
		require.NoError(t, err)
		assert.True(t, report.isEmpty())
		columns, _ := columnList.Get(e.orders)
		assert.Equal(t, []string{"id", "doc"}, columns)
	})

	t.Run("unexcluded unsupported columns are reported once per root", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		_, report, err := applyExcludedColumns(excludedColumnsMap(nil), e.exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{
			primaryKeyTables: map[string][]string{},
			otherTables:      map[string][]string{"public.orders": {"doc"}, "public.sales": {"Geo"}},
		}, report)
	})

	t.Run("unsupported primary key column is reported for table exclusion only", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		e.exportTables.unsupportedColumns.Put(e.orders, []string{"id", "doc"})
		_, report, err := applyExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.sales: {"Geo"}}), e.exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{
			primaryKeyTables: map[string][]string{"public.orders": {"id"}},
			otherTables:      map[string][]string{},
		}, report)
	})

	t.Run("table with every column excluded", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		e.exportTables.supportedColumns.Put(e.orders, []string{"note"})
		_, report, err := applyExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note", "doc"}, e.sales: {"Geo"}}), e.exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{
			primaryKeyTables:    map[string][]string{},
			otherTables:         map[string][]string{},
			tablesWithNoColumns: []sqlname.NameTuple{e.orders},
		}, report)
	})
}

func TestBuildUnsupportedColumnsError(t *testing.T) {
	e := newExcludeColumnsTestExport()
	e.exportTables.unsupportedColumns.Put(e.orders, []string{"id"})
	report := unsupportedColumnsReport{
		primaryKeyTables: map[string][]string{"public.orders": {"id"}},
		otherTables:      map[string][]string{"public.sales": {"Geo"}},
	}
	userExcluded := map[string][]string{"public.sales": {"region_note"}, "public.items": {"a"}}

	t.Run("source exporter", func(t *testing.T) {
		err := buildUnsupportedColumnsError(report, userExcluded)
		assert.Equal(t, `The following primary key columns have datatypes not supported for this migration:
  public.orders: id
A table cannot be migrated without its primary key. Exclude these tables with --exclude-table-list:
  --exclude-table-list 'public.orders'

The following columns have datatypes not supported for this migration:
  public.sales: Geo
To proceed, do one of the following:
  1. Exclude these columns with --exclude-column-list:
       --exclude-column-list 'public.items:a;public.sales:region_note,Geo'
     Changes to excluded columns are not streamed: the target keeps their snapshot phase of migration, and rows inserted during streaming get NULL in them. If any of them has a NOT NULL constraint in the target database, remove it.
  2. Exclude these tables with --exclude-table-list:
       --exclude-table-list 'public.sales'`, err.Error())
	})

	t.Run("target exporter", func(t *testing.T) {
		originalRole := exporterRole
		exporterRole = TARGET_DB_EXPORTER_FB_ROLE
		t.Cleanup(func() { exporterRole = originalRole })

		err := buildUnsupportedColumnsError(report, nil)
		assert.Equal(t, `The following primary key columns have datatypes not supported for this migration:
  public.orders: id
A table cannot be migrated without its primary key. These tables had to be excluded with --exclude-table-list on 'export data from source', which requires starting the migration again:
  --exclude-table-list 'public.orders'

The following columns have datatypes not supported for this migration:
  public.sales: Geo
Restart 'export data from target' with these columns excluded:
  --exclude-column-list 'public.sales:Geo'
Changes made on the target to excluded columns are not exported: the source database keeps their values from before cutover, and rows inserted after cutover get NULL in them. If any of them has a NOT NULL constraint in the source database, remove it.`, err.Error())
	})
}

// useExcludeColumnsTestMetaDB points the metaDB and exporterRole globals at a fresh export dir for
// the duration of the test.
func useExcludeColumnsTestMetaDB(t *testing.T, role string) {
	savedMetaDB, savedExporterRole := metaDB, exporterRole
	t.Cleanup(func() { metaDB, exporterRole = savedMetaDB, savedExporterRole })
	metaDB = initMetaDB(t.TempDir())
	exporterRole = role
}

func excludedColumnsRecord(t *testing.T) metadb.ExcludedColumnsRecord {
	msr, err := metaDB.GetMigrationStatusRecord()
	require.NoError(t, err)
	require.NotNil(t, msr)
	return msr.ExcludedColumns
}

func TestRecordExcludedColumns(t *testing.T) {
	e := newExcludeColumnsTestExport()

	t.Run("first run records the set sorted, and a re-run with the same set in another order passes", func(t *testing.T) {
		useExcludeColumnsTestMetaDB(t, SOURCE_DB_EXPORTER_ROLE)
		require.NoError(t, recordExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note", "doc"}, e.sales: {"Geo"}})))
		expected := metadb.ExcludedColumnsRecord{
			FromSource: map[string][]string{e.orders.ForKey(): {"doc", "note"}, e.sales.ForKey(): {"Geo"}},
		}
		assert.Equal(t, expected, excludedColumnsRecord(t))

		require.NoError(t, recordExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.sales: {"Geo"}, e.orders: {"doc", "note"}})))
		assert.Equal(t, expected, excludedColumnsRecord(t))
	})

	t.Run("a re-run with a different set fails and leaves the record unchanged", func(t *testing.T) {
		useExcludeColumnsTestMetaDB(t, SOURCE_DB_EXPORTER_ROLE)
		require.NoError(t, recordExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"doc"}})))

		err := recordExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"doc", "note"}}))
		require.Error(t, err)
		assert.Equal(t, "--exclude-column-list cannot change once export data has started.\n"+
			"  started with: '\"public\".\"orders\":doc'\n"+
			"  now:          '\"public\".\"orders\":doc,note'\n"+
			"Re-run with the value it started with.", err.Error())
		assert.Equal(t, map[string][]string{e.orders.ForKey(): {"doc"}}, excludedColumnsRecord(t).FromSource)
	})

	t.Run("a first run without the flag records an empty set, so exclusions cannot be added on resume", func(t *testing.T) {
		useExcludeColumnsTestMetaDB(t, SOURCE_DB_EXPORTER_ROLE)
		require.NoError(t, recordExcludedColumns(utils.NewStructMap[sqlname.NameTuple, []string]()))
		recorded := excludedColumnsRecord(t)
		assert.NotNil(t, recorded.FromSource)
		assert.Empty(t, recorded.FromSource)
		assert.Nil(t, recorded.FromTarget)

		err := recordExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"doc"}}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "started with: ''")
	})

	t.Run("the target exporter records its own set without touching the source's", func(t *testing.T) {
		useExcludeColumnsTestMetaDB(t, TARGET_DB_EXPORTER_FB_ROLE)
		fromSource := map[string][]string{e.orders.ForKey(): {"doc"}}
		require.NoError(t, metaDB.UpdateMigrationStatusRecord(func(r *metadb.MigrationStatusRecord) {
			r.ExcludedColumns.FromSource = fromSource
		}))

		require.NoError(t, recordExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note"}})))
		assert.Equal(t, metadb.ExcludedColumnsRecord{
			FromSource: fromSource,
			FromTarget: map[string][]string{e.orders.ForKey(): {"note"}},
		}, excludedColumnsRecord(t))
	})
}

func TestHandleExcludedColumnForTargetExporter(t *testing.T) {
	e := newExcludeColumnsTestExport()
	finalTableList := e.exportTables.tableList
	leafToRoot := e.exportTables.leafToRoot
	userExcluded := excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note"}})

	t.Run("the target exporter adds the source's set per root table, skipping leaves", func(t *testing.T) {
		useExcludeColumnsTestMetaDB(t, TARGET_DB_EXPORTER_FB_ROLE)
		require.NoError(t, metaDB.UpdateMigrationStatusRecord(func(r *metadb.MigrationStatusRecord) {
			r.ExcludedColumns.FromSource = map[string][]string{e.orders.ForKey(): {"doc", "note"}, e.sales.ForKey(): {"Geo"}}
		}))

		excluded, err := handleExcludedColumnForTargetExporter(userExcluded, finalTableList, leafToRoot)
		require.NoError(t, err)
		ordersColumns, _ := excluded.Get(e.orders)
		assert.Equal(t, []string{"doc", "note"}, ordersColumns)
		salesColumns, _ := excluded.Get(e.sales)
		assert.Equal(t, []string{"Geo"}, salesColumns)
		assert.ElementsMatch(t, []string{e.orders.ForKey(), e.sales.ForKey()}, lo.Map(excluded.ActualKeys(), func(t sqlname.NameTuple, _ int) string { return t.ForKey() }))
	})

	t.Run("only the columns the target connector cannot stream and the source did not exclude are left", func(t *testing.T) {
		useExcludeColumnsTestMetaDB(t, TARGET_DB_EXPORTER_FB_ROLE)
		require.NoError(t, metaDB.UpdateMigrationStatusRecord(func(r *metadb.MigrationStatusRecord) {
			r.ExcludedColumns.FromSource = map[string][]string{e.orders.ForKey(): {"doc"}, e.sales.ForKey(): {"Geo"}}
		}))
		exportTables := e.exportTables
		columns := utils.NewStructMap[sqlname.NameTuple, []string]()
		_ = e.exportTables.tableToColumns.IterKV(func(table sqlname.NameTuple, cols []string) (bool, error) {
			columns.Put(table, cols)
			return true, nil
		})
		ordersColumns, _ := columns.Get(e.orders)
		columns.Put(e.orders, append(slices.Clone(ordersColumns), "search"))
		exportTables.tableToColumns = columns
		unsupported := utils.NewStructMap[sqlname.NameTuple, []string]()
		unsupported.Put(e.orders, []string{"doc", "search"})
		for _, table := range []sqlname.NameTuple{e.sales, e.salesQ1, e.salesQ2} {
			unsupported.Put(table, []string{"Geo"})
		}
		exportTables.unsupportedColumns = unsupported

		excluded, err := handleExcludedColumnForTargetExporter(utils.NewStructMap[sqlname.NameTuple, []string](), finalTableList, leafToRoot)
		require.NoError(t, err)
		_, report, err := applyExcludedColumns(excluded, exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{primaryKeyTables: map[string][]string{}, otherTables: map[string][]string{e.orders.ForOutput(): {"search"}}}, report)
	})
}

// A partitioned root may have no primary key of its own when every partition has the same one;
// the exclude-column-list checks must treat the partitions' key as the root's.
func TestExcludeColumnsOnPartitionedRootWithoutPrimaryKey(t *testing.T) {
	newExportWithKeylessSalesRoot := func(t *testing.T) (excludeColumnsTestExport, exportTableColumnMetadata) {
		e := newExcludeColumnsTestExport()
		exportTables := e.exportTables
		primaryKeyColumns := utils.NewStructMap[sqlname.NameTuple, []string]()
		primaryKeyColumns.Put(e.orders, []string{"id"})
		primaryKeyColumns.Put(e.salesQ1, []string{"id", "region"})
		primaryKeyColumns.Put(e.salesQ2, []string{"id", "region"})
		exportTables.tableToPrimaryKeyColumns = primaryKeyColumns
		require.NoError(t, exportTables.addPrimaryKeyToRootTableIfRequired())
		return e, exportTables
	}

	t.Run("the root takes its partitions' primary key", func(t *testing.T) {
		e, exportTables := newExportWithKeylessSalesRoot(t)
		salesPrimaryKey, ok := exportTables.tableToPrimaryKeyColumns.Get(e.sales)
		require.True(t, ok)
		assert.Equal(t, []string{"id", "region"}, salesPrimaryKey)
	})

	t.Run("excluding a column of the partitions' primary key is rejected", func(t *testing.T) {
		e, exportTables := newExportWithKeylessSalesRoot(t)
		_, problems, err := validateExcludedColumns(map[string][]string{"public.sales": {"id"}}, e.lookupTable, exportTables, e.rootOfTable, "exclude the whole table with --exclude-table-list")
		require.NoError(t, err)
		assert.Equal(t, []string{`public.sales: column "id" is part of the primary key and cannot be excluded`}, problems)
	})

	t.Run("an unsupported column in the partitions' primary key points to exclude-table-list", func(t *testing.T) {
		e, exportTables := newExportWithKeylessSalesRoot(t)
		unsupported := utils.NewStructMap[sqlname.NameTuple, []string]()
		for _, table := range []sqlname.NameTuple{e.sales, e.salesQ1, e.salesQ2} {
			unsupported.Put(table, []string{"region"})
		}
		exportTables.unsupportedColumns = unsupported

		_, report, err := applyExcludedColumns(utils.NewStructMap[sqlname.NameTuple, []string](), exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{primaryKeyTables: map[string][]string{e.sales.ForOutput(): {"region"}}, otherTables: map[string][]string{}}, report)
	})
}

type excludeColumnsStubSourceDB struct {
	srcdb.SourceDB
	columns     *utils.StructMap[sqlname.NameTuple, []string]
	primaryKeys *utils.StructMap[sqlname.NameTuple, []string]
}

func (s excludeColumnsStubSourceDB) GetTableToColumns([]sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	return s.columns, nil
}

func (s excludeColumnsStubSourceDB) GetPrimaryKeyColumns([]sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	return s.primaryKeys, nil
}

// finalizeExcludeColumnsTest is an export of public.orders and the case-sensitive "Sales"."Orders",
// with the globals finalizeColumnListForLiveMigration reads pointed at a temp metaDB and an
// in-memory name registry for role.
type finalizeExcludeColumnsTest struct {
	orders, salesOrders sqlname.NameTuple
	sourceDB            excludeColumnsStubSourceDB
	supported           *utils.StructMap[sqlname.NameTuple, []string]
	unsupported         *utils.StructMap[sqlname.NameTuple, []string]
}

func newFinalizeExcludeColumnsTest(t *testing.T, role string) finalizeExcludeColumnsTest {
	useExcludeColumnsTestMetaDB(t, role)
	savedNameReg, savedSourceDBType, savedSqlnameSourceDBType := namereg.NameReg, source.DBType, sqlname.SourceDBType
	savedEntries := userSpecifiedExcludeColumnListEntries
	t.Cleanup(func() {
		namereg.NameReg, source.DBType, sqlname.SourceDBType = savedNameReg, savedSourceDBType, savedSqlnameSourceDBType
		userSpecifiedExcludeColumnListEntries = savedEntries
	})
	source.DBType = POSTGRESQL
	sqlname.SourceDBType = POSTGRESQL

	regFile := filepath.Join(t.TempDir(), "name_registry.json")
	require.NoError(t, os.WriteFile(regFile, []byte(`{
  "SourceDBType": "postgresql",
  "SourceDBSchemaNames": ["public", "Sales"],
  "DefaultSourceDBSchemaName": "public",
  "SourceDBTableNames": {"public": ["orders"], "Sales": ["Orders"]},
  "YBSchemaNames": ["public", "Sales"],
  "DefaultYBSchemaName": "public",
  "YBTableNames": {"public": ["orders"], "Sales": ["Orders"]}
}`), 0644))
	require.NoError(t, namereg.InitNameRegistry(namereg.NameRegistryParams{FilePath: regFile, Role: role}))

	f := finalizeExcludeColumnsTest{}
	var err error
	f.orders, err = namereg.NameReg.LookupTableName("public.orders")
	require.NoError(t, err)
	f.salesOrders, err = namereg.NameReg.LookupTableName(`"Sales"."Orders"`)
	require.NoError(t, err)

	columns := utils.NewStructMap[sqlname.NameTuple, []string]()
	columns.Put(f.orders, []string{"id", "note", "doc"})
	columns.Put(f.salesOrders, []string{"id", "note", "region"})
	primaryKeys := utils.NewStructMap[sqlname.NameTuple, []string]()
	primaryKeys.Put(f.orders, []string{"id"})
	primaryKeys.Put(f.salesOrders, []string{"id"})
	f.sourceDB = excludeColumnsStubSourceDB{columns: columns, primaryKeys: primaryKeys}

	f.supported = utils.NewStructMap[sqlname.NameTuple, []string]()
	f.supported.Put(f.orders, []string{"id", "note"})
	f.supported.Put(f.salesOrders, []string{"*"})
	f.unsupported = utils.NewStructMap[sqlname.NameTuple, []string]()
	f.unsupported.Put(f.orders, []string{"doc"})
	return f
}

func (f finalizeExcludeColumnsTest) run(t *testing.T, flagValue string) (*utils.StructMap[sqlname.NameTuple, []string], error) {
	entries, err := parseTableColumnsFlag(excludeColumnListFlagName, flagValue)
	require.NoError(t, err)
	userSpecifiedExcludeColumnListEntries = entries
	noPartitions := func(table sqlname.NameTuple) (sqlname.NameTuple, error) { return table, nil }
	return finalizeColumnListForLiveMigration(f.sourceDB, noPartitions, []sqlname.NameTuple{f.orders, f.salesOrders}, map[string]string{}, f.supported, f.unsupported)
}

func TestFinalizeColumnListForLiveMigration(t *testing.T) {
	t.Run("source exporter: first run, resume with the value passed to the next iteration, and a changed set", func(t *testing.T) {
		f := newFinalizeExcludeColumnsTest(t, SOURCE_DB_EXPORTER_ROLE)

		columnList, err := f.run(t, `public.orders:DOC;"Sales"."Orders":note`)
		require.NoError(t, err)
		ordersColumns, _ := columnList.Get(f.orders)
		assert.Equal(t, []string{"id", "note"}, ordersColumns)
		salesOrdersColumns, _ := columnList.Get(f.salesOrders)
		assert.Equal(t, []string{"id", "region"}, salesOrdersColumns, `"*" is expanded so that the excluded column is left out`)
		recorded := map[string][]string{f.orders.ForKey(): {"doc"}, f.salesOrders.ForKey(): {"note"}}
		assert.Equal(t, metadb.ExcludedColumnsRecord{FromSource: recorded}, excludedColumnsRecord(t))

		// startExportDataFromSourceOnNextIteration passes the recorded set back in this form
		_, err = f.run(t, formatExcludeColumnList(recorded))
		require.NoError(t, err)

		_, err = f.run(t, "public.orders:doc")
		require.Error(t, err)
		assert.Equal(t, "--exclude-column-list cannot change once export data has started.\n"+
			"  started with: '"+formatExcludeColumnList(recorded)+"'\n"+
			"  now:          '"+formatExcludeColumnList(map[string][]string{f.orders.ForKey(): {"doc"}})+"'\n"+
			"Re-run with the value it started with.", err.Error())
		assert.Equal(t, metadb.ExcludedColumnsRecord{FromSource: recorded}, excludedColumnsRecord(t))
	})

	t.Run("source exporter: problems in the value fail before anything is recorded", func(t *testing.T) {
		f := newFinalizeExcludeColumnsTest(t, SOURCE_DB_EXPORTER_ROLE)
		_, err := f.run(t, `"Sales".missing:note;public.orders:id,foo`)
		require.Error(t, err)
		assert.Equal(t, "invalid --exclude-column-list:\n"+
			"  \"Sales\".missing: table not found\n"+
			"  public.orders: column \"foo\" does not exist\n"+
			"  public.orders: column \"id\" is part of the primary key and cannot be excluded", err.Error())
		assert.Equal(t, metadb.ExcludedColumnsRecord{}, excludedColumnsRecord(t))
	})

	t.Run("source exporter: an unsupported column left on a case-sensitive table fails and records nothing", func(t *testing.T) {
		f := newFinalizeExcludeColumnsTest(t, SOURCE_DB_EXPORTER_ROLE)
		f.unsupported.Put(f.salesOrders, []string{"region"})
		_, err := f.run(t, "public.orders:doc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "  "+f.salesOrders.ForOutput()+": region\n")
		assert.Contains(t, err.Error(), "--exclude-column-list '"+f.salesOrders.ForOutput()+":region;public.orders:doc'")
		assert.Contains(t, err.Error(), "--exclude-table-list '"+f.salesOrders.ForOutput()+"'")
		assert.Equal(t, metadb.ExcludedColumnsRecord{}, excludedColumnsRecord(t))
	})

	t.Run("target exporter: inherits the source's columns and adds its own", func(t *testing.T) {
		f := newFinalizeExcludeColumnsTest(t, TARGET_DB_EXPORTER_FB_ROLE)
		fromSource := map[string][]string{f.orders.ForKey(): {"doc"}}
		require.NoError(t, metaDB.UpdateMigrationStatusRecord(func(r *metadb.MigrationStatusRecord) {
			r.ExcludedColumns.FromSource = fromSource
		}))
		f.unsupported.Put(f.salesOrders, []string{"note"})

		columnList, err := f.run(t, `"Sales"."Orders":note`)
		require.NoError(t, err)
		ordersColumns, _ := columnList.Get(f.orders)
		assert.Equal(t, []string{"id", "note"}, ordersColumns)
		salesOrdersColumns, _ := columnList.Get(f.salesOrders)
		assert.Equal(t, []string{"id", "region"}, salesOrdersColumns)
		record := excludedColumnsRecord(t)
		assert.Equal(t, fromSource, record.FromSource)
		assert.Equal(t, map[string][]string{f.orders.ForKey(): {"doc"}, f.salesOrders.ForKey(): {"note"}}, record.FromTarget)
	})
}
func TestLookupPerTableFlagTables(t *testing.T) {
	origNameReg := namereg.NameReg
	origSourceDBType := sqlname.SourceDBType
	t.Cleanup(func() {
		namereg.NameReg = origNameReg
		sqlname.SourceDBType = origSourceDBType
	})
	sqlname.SourceDBType = POSTGRESQL

	regFile := filepath.Join(t.TempDir(), "name_registry.json")
	content := `{
  "SourceDBType": "postgresql",
  "SourceDBSchemaNames": ["test_schema"],
  "DefaultSourceDBSchemaName": "test_schema",
  "SourceDBTableNames": {"test_schema": ["Orders", "ORDERS", "events"]},
  "YBSchemaNames": ["test_schema"],
  "DefaultYBSchemaName": "test_schema",
  "YBTableNames": {"test_schema": ["Orders", "ORDERS", "events"]}
}`
	require.NoError(t, os.WriteFile(regFile, []byte(content), 0644))
	require.NoError(t, namereg.InitNameRegistry(namereg.NameRegistryParams{
		FilePath: regFile,
		Role:     namereg.TARGET_DB_IMPORTER_ROLE,
	}))

	resolved, problems := lookupPerTableFlagTables([]string{
		"test_schema.missing",
		"test_schema.orders",
		`test_schema."Orders"`,
		"a.b.c",
		"events",
	}, namereg.NameReg.LookupTableName)

	assert.Equal(t, []string{
		"test_schema.missing: table not found",
		`test_schema.orders: matches several tables (Orders, ORDERS); double-quote the exact table name`,
		"a.b.c: invalid table name: a.b.c",
	}, problems)
	assert.ElementsMatch(t, []string{`test_schema."Orders"`, "events"}, lo.Keys(resolved))
}
