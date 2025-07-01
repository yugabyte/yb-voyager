//go:build unit

package anonymizer_test

import (
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/anonymizer"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

/*
	Tests to add:
	1. Anonymizing tablename, column name, schema name in DDLs
	2. Anonymizing Index name in DDLs
	3. Anonymizing constraint name in DDLs
	4. Anonymizing alias names in DMLs
	5. Anonymizing function names in DDLs
	6. Also cover case sensitivity
*/

func newAnon(t *testing.T, exportDir string) *anonymizer.SqlAnonymizer {
	metaDB, err := testutils.CreateMetaDB(exportDir)
	testutils.FatalIfError(t, err)

	a, err := anonymizer.NewSqlAnonymizer(metaDB)
	testutils.FatalIfError(t, err)
	return a
}

func hasToken(s, prefix string) bool {
	return strings.Contains(s, prefix)
}

func TestTableAndColumnDDLs(t *testing.T) {
	tests := []struct {
		name         string
		sql          string   // input
		badStrings   []string // must disappear
		wantPrefixes []string
	}{
		{
			"simple create",
			"CREATE TABLE foo (id INT, name TEXT);",
			[]string{"foo", "id", "name"},
			[]string{anonymizer.TABLE_KIND_PREFIX, anonymizer.COLUMN_KIND_PREFIX},
		},
		{
			"schema-qualified",
			`CREATE TABLE sales.orders (OrderID int, Total numeric);`,
			[]string{"sales", "orders", "OrderID", "Total"},
			[]string{anonymizer.SCHEMA_KIND_PREFIX, anonymizer.TABLE_KIND_PREFIX, anonymizer.COLUMN_KIND_PREFIX},
		},
		{
			"quoted / mixed case",
			`CREATE TABLE "Customer"."LineItems" ("LineID" int, "ProductSKU" text);`,
			[]string{"Customer", "LineItems", "LineID", "ProductSKU"},
			[]string{anonymizer.SCHEMA_KIND_PREFIX, anonymizer.TABLE_KIND_PREFIX, anonymizer.COLUMN_KIND_PREFIX},
		},
	}

	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	a := newAnon(t, exportDir)

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			out, err := a.Anonymize(tc.sql)
			if err != nil {
				t.Fatalf("Anonymize: %v", err)
			}
			t.Logf("\nIN : %s\nOUT: %s", tc.sql, out)

			for _, bad := range tc.badStrings {
				if strings.Contains(out, bad) {
					t.Errorf("found raw identifier %q in output", bad)
				}
			}
			for _, pref := range tc.wantPrefixes {
				if !hasToken(out, pref) {
					t.Errorf("expected token with prefix %q", pref)
				}
			}
		})
	}
}

func TestIndexConstraintAlias(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		bad          []string
		wantPrefixes []string
	}{
		{
			"index + table",
			"CREATE INDEX idx_foo ON mytable(bar);",
			[]string{"idx_foo", "mytable", "bar"},
			[]string{anonymizer.INDEX_KIND_PREFIX, anonymizer.TABLE_KIND_PREFIX, anonymizer.COLUMN_KIND_PREFIX},
		},
		{
			"constraint",
			"CREATE TABLE t (id int CONSTRAINT pk_t PRIMARY KEY);",
			[]string{"t", "pk_t"},
			[]string{anonymizer.TABLE_KIND_PREFIX, anonymizer.CONSTRAINT_KIND_PREFIX},
		},
		{
			"alias reference",
			"SELECT * FROM orders o WHERE o.amount > 100;",
			[]string{"orders", "o", "amount"},
			[]string{anonymizer.ALIAS_KIND_PREFIX, anonymizer.TABLE_KIND_PREFIX, anonymizer.COLUMN_KIND_PREFIX},
		},
	}

	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	a := newAnon(t, exportDir)

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out, err := a.Anonymize(tc.sql)
			if err != nil {
				t.Fatalf("Anonymize error: %v", err)
			}
			t.Logf("\nIN : %s\nOUT: %s", tc.sql, out)

			for _, b := range tc.bad {
				if strings.Contains(out, b) {
					t.Errorf("found raw %q", b)
				}
			}
			for _, p := range tc.wantPrefixes {
				if !hasToken(out, p) {
					t.Errorf("missing token prefix %q", p)
				}
			}
		})
	}
}
