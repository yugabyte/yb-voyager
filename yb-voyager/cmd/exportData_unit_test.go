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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		expected []tableColumns
		errMsg   string
	}{
		{
			name:     "empty value",
			value:    "",
			expected: nil,
		},
		{
			name:  "qualified and bare tables, columns keep their case",
			value: "public.orders:Note,meta; items : a , B",
			expected: []tableColumns{
				{table: "public.orders", columns: []string{"Note", "meta"}},
				{table: "items", columns: []string{"a", "B"}},
			},
		},
		{
			name:  "quoted names keep case and may contain separators",
			value: `"Sales"."Orders;2024":"Geo:Point","a,b","x""y";public.t:"Mixed"`,
			expected: []tableColumns{
				{table: `"Sales"."Orders;2024"`, columns: []string{"Geo:Point", "a,b", `x"y`}},
				{table: "public.t", columns: []string{"Mixed"}},
			},
		},
		{
			name:  "trailing semicolon",
			value: "public.t:a;",
			expected: []tableColumns{
				{table: "public.t", columns: []string{"a"}},
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
			name:   "duplicate column after unquoting",
			value:  `public.t:a,"a"`,
			errMsg: `invalid --exclude-column-list entry for table "public.t": duplicate column "a"`,
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
			entries, err := parseExcludeColumnList(tt.value)
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
		`"public"."Orders"`:  {"note", "Meta", "select", `x"y`},
		`"Sales"."orders:1"`: {"geo"},
	}
	value := excludeColumnListFlagValue(recorded)
	assert.Equal(t, `"Sales"."orders:1":geo;"public"."Orders":note,"Meta","select","x""y";"public"."t2":b`, value)

	entries, err := parseExcludeColumnList(value)
	require.NoError(t, err)
	assert.Equal(t, []tableColumns{
		{table: `"Sales"."orders:1"`, columns: []string{"geo"}},
		{table: `"public"."Orders"`, columns: []string{"note", "Meta", "select", `x"y`}},
		{table: `"public"."t2"`, columns: []string{"b"}},
	}, entries)
}

func TestResolveExcludeColumnList(t *testing.T) {
	orders := excludeColumnsTestTable("public", "orders")
	lookup := func(name string) (sqlname.NameTuple, error) {
		switch name {
		case "orders", "public.orders":
			return orders, nil
		}
		return sqlname.NameTuple{}, fmt.Errorf("table %s not found", name)
	}
	entries := []tableColumns{
		{table: "orders", columns: []string{"a"}},
		{table: "public.missing", columns: []string{"b"}},
		{table: "public.orders", columns: []string{"c"}},
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
	ordersColumns := []srcdb.ColumnWithType{{Name: "id", Type: "integer"}, {Name: "note", Type: "text"}, {Name: "doc", Type: "xml"}}
	salesColumns := []srcdb.ColumnWithType{{Name: "id", Type: "integer"}, {Name: "region", Type: "text"}, {Name: "Geo", Type: "geometry"}}

	columns := utils.NewStructMap[sqlname.NameTuple, []srcdb.ColumnWithType]()
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
		tableToColumnsWithType:   columns,
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

func TestValidateExcludedColumns(t *testing.T) {
	e := newExcludeColumnsTestExport()
	excludeTableHint := "exclude the whole table with --exclude-table-list"
	notExported := excludeColumnsTestTable("public", "audit")

	tests := []struct {
		name     string
		excluded map[sqlname.NameTuple][]string
		expected []string
	}{
		{
			name:     "valid columns on a table and a partitioned root",
			excluded: map[sqlname.NameTuple][]string{e.orders: {"doc", "note"}, e.sales: {"Geo"}},
			expected: nil,
		},
		{
			name:     "column that does not exist",
			excluded: map[sqlname.NameTuple][]string{e.orders: {"foo"}},
			expected: []string{`public.orders: column "foo" does not exist`},
		},
		{
			name:     "two spellings of one column",
			excluded: map[sqlname.NameTuple][]string{e.orders: {"Doc", "doc"}},
			expected: []string{`public.orders: column "doc" is listed more than once`},
		},
		{
			name:     "leaf partition named directly",
			excluded: map[sqlname.NameTuple][]string{e.salesQ1: {"Geo"}},
			expected: []string{"public.sales_q1: is a partition of public.sales; specify the root table public.sales"},
		},
		{
			name:     "table not part of the export",
			excluded: map[sqlname.NameTuple][]string{notExported: {"a"}},
			expected: []string{"public.audit: table is not part of this export; check --table-list and --exclude-table-list"},
		},
		{
			name:     "primary key column",
			excluded: map[sqlname.NameTuple][]string{e.sales: {"region"}},
			expected: []string{`public.sales: column "region" is part of the primary key and cannot be excluded`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems, err := validateExcludedColumns(excludedColumnsMap(tt.excluded), e.exportTables, e.rootOfTable, excludeTableHint)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expected, problems)
		})
	}

	t.Run("columns are matched to the table's columns ignoring case", func(t *testing.T) {
		excluded := excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"DOC", "Note"}, e.sales: {"geo"}})
		problems, err := validateExcludedColumns(excluded, e.exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Empty(t, problems)
		ordersColumns, _ := excluded.Get(e.orders)
		assert.Equal(t, []string{"doc", "note"}, ordersColumns)
		salesColumns, _ := excluded.Get(e.sales)
		assert.Equal(t, []string{"Geo"}, salesColumns)
	})

	t.Run("a column matching several columns only by case", func(t *testing.T) {
		columns := utils.NewStructMap[sqlname.NameTuple, []srcdb.ColumnWithType]()
		columns.Put(e.orders, []srcdb.ColumnWithType{{Name: "id", Type: "integer"}, {Name: "Note", Type: "text"}, {Name: "NOTE", Type: "text"}, {Name: "note", Type: "text"}})
		exportTables := e.exportTables
		exportTables.tableToColumnsWithType = columns

		excluded := excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"nOtE", "NOTE"}})
		problems, err := validateExcludedColumns(excluded, exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Empty(t, problems, "an unquoted spelling resolves to the lowercase column, an exact spelling to itself")
		matched, _ := excluded.Get(e.orders)
		assert.Equal(t, []string{"note", "NOTE"}, matched)

		columns.Put(e.orders, []srcdb.ColumnWithType{{Name: "id", Type: "integer"}, {Name: "Note", Type: "text"}, {Name: "NOTE", Type: "text"}})
		problems, err = validateExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note"}}), exportTables, e.rootOfTable, excludeTableHint)
		require.NoError(t, err)
		assert.Equal(t, []string{`public.orders: column "note" is ambiguous; double-quote the exact column name`}, problems)
	})

	t.Run("primary key column with an unsupported datatype points to exclude-table-list", func(t *testing.T) {
		unsupported := utils.NewStructMap[sqlname.NameTuple, []string]()
		unsupported.Put(e.orders, []string{"id"})
		exportTables := e.exportTables
		exportTables.unsupportedColumns = unsupported
		problems, err := validateExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"id"}}), exportTables, e.rootOfTable, excludeTableHint)
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
			otherTables: []tableColumns{
				{table: "public.orders", columns: []string{"doc"}},
				{table: "public.sales", columns: []string{"Geo"}},
			},
		}, report)
	})

	t.Run("unsupported primary key column is reported for table exclusion only", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		e.exportTables.unsupportedColumns.Put(e.orders, []string{"id", "doc"})
		_, report, err := applyExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.sales: {"Geo"}}), e.exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{
			primaryKeyTables: []tableColumns{{table: "public.orders", columns: []string{"id"}}},
		}, report)
	})

	t.Run("table with every column excluded", func(t *testing.T) {
		e := newExcludeColumnsTestExport()
		e.exportTables.supportedColumns.Put(e.orders, []string{"note"})
		_, report, err := applyExcludedColumns(excludedColumnsMap(map[sqlname.NameTuple][]string{e.orders: {"note", "doc"}, e.sales: {"Geo"}}), e.exportTables)
		require.NoError(t, err)
		assert.Equal(t, unsupportedColumnsReport{tablesWithNoColumns: []sqlname.NameTuple{e.orders}}, report)
	})
}

func TestBuildUnsupportedColumnsError(t *testing.T) {
	e := newExcludeColumnsTestExport()
	e.exportTables.unsupportedColumns.Put(e.orders, []string{"id"})
	report := unsupportedColumnsReport{
		primaryKeyTables: []tableColumns{{table: "public.orders", columns: []string{"id"}}},
		otherTables:      []tableColumns{{table: "public.sales", columns: []string{"Geo"}}},
	}
	userExcluded := []tableColumns{{table: "public.sales", columns: []string{"region_note"}}, {table: "public.items", columns: []string{"a"}}}

	t.Run("source exporter", func(t *testing.T) {
		err := buildUnsupportedColumnsError(report, e.exportTables, userExcluded)
		assert.Equal(t, `The following primary key columns have datatypes not supported for this migration:
  public.orders: id (integer)
A table cannot be migrated without its primary key. Exclude these tables with --exclude-table-list:
  --exclude-table-list 'public.orders'

The following columns have datatypes not supported for this migration:
  public.sales: "Geo" (geometry)
To proceed, do one of the following:
  1. Exclude these columns with --exclude-column-list:
       --exclude-column-list 'public.sales:region_note,"Geo";public.items:a'
     Changes to excluded columns are not streamed: the target keeps their snapshot values, and rows inserted during streaming get NULL in them. If any of them has a NOT NULL constraint in the target database, remove it.
  2. Exclude these tables with --exclude-table-list:
       --exclude-table-list 'public.sales'`, err.Error())
	})

	t.Run("target exporter", func(t *testing.T) {
		originalRole := exporterRole
		exporterRole = TARGET_DB_EXPORTER_FB_ROLE
		t.Cleanup(func() { exporterRole = originalRole })

		err := buildUnsupportedColumnsError(report, e.exportTables, nil)
		assert.Equal(t, `The following primary key columns have datatypes not supported for this migration:
  public.orders: id (integer)
A table cannot be migrated without its primary key. These tables had to be excluded with --exclude-table-list on 'export data from source', which requires starting the migration again:
  --exclude-table-list 'public.orders'

The following columns have datatypes not supported for this migration:
  public.sales: "Geo" (geometry)
Restart 'export data from target' with these columns excluded:
  --exclude-column-list 'public.sales:"Geo"'
Changes made on the target to excluded columns are not exported: the source database keeps their values from before cutover, and rows inserted after cutover get NULL in them. If any of them has a NOT NULL constraint in the source database, remove it.`, err.Error())
	})
}
