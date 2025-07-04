//go:build unit

package anon

import (
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

var schemaTokenRegistry TokenRegistry

/*
	Tests to add:
	1. Anonymizing tablename, column name, schema name in DDLs
	2. Anonymizing Index name in DDLs
	3. Anonymizing constraint name in DDLs
	4. Anonymizing alias names in DMLs
	5. Anonymizing function names in DDLs
	6. Also cover case sensitivity
*/

func createMetaDB(exportDir string) (*metadb.MetaDB, error) {
	err := metadb.CreateAndInitMetaDBIfRequired(exportDir)
	if err != nil {
		return nil, fmt.Errorf("could not create and init meta db: %w", err)
	}

	metaDBInstance, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MetaDB: %w", err)
	}

	err = metaDBInstance.InitMigrationStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize migration status record: %w", err)
	}
	return metaDBInstance, nil
}

func newAnon(t *testing.T, exportDir string) Anonymizer {
	metaDB, err := createMetaDB(exportDir)
	testutils.FatalIfError(t, err)

	schemaTokenRegistry, err = NewSchemaTokenRegistry(metaDB)
	testutils.FatalIfError(t, err)

	a := NewSqlAnonymizer(schemaTokenRegistry)
	testutils.FatalIfError(t, err)
	return a
}

func hasToken(s, prefix string) bool {
	return strings.Contains(s, prefix)
}

// One test for each of the sql anonymization processor case implemented in anon.go
func TestAllAnonymizationProcessorCases(t *testing.T) {
	cases := []struct {
		nodeName string
		sql      string   // input SQL
		raw      []string // identifiers or literals that MUST be gone
		prefixes []string // token prefixes that MUST appear
	}{
		/* ---------- Identifier nodes ---------- */
		{
			nodeName: "RangeVar (schema + table)",
			sql:      `SELECT * FROM sales.orders;`,
			raw:      []string{"sales", "orders"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},
		{
			nodeName: "ColumnDef",
			sql:      `CREATE TABLE t(secret_col TEXT);`,
			raw:      []string{"secret_col"},
			prefixes: []string{COLUMN_KIND_PREFIX},
		},
		{
			nodeName: "ColumnRef",
			sql:      `SELECT password FROM users;`,
			raw:      []string{"password", "users"},
			prefixes: []string{COLUMN_KIND_PREFIX, TABLE_KIND_PREFIX},
		},
		{
			nodeName: "IndexElem",
			sql:      `CREATE INDEX idx_ab ON tbl(col1);`,
			raw:      []string{"idx_ab", "tbl", "col1"},
			prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			nodeName: "Constraint",
			sql:      `CREATE TABLE tbl(id INT CONSTRAINT pk_t PRIMARY KEY);`,
			raw:      []string{"pk_t"},
			prefixes: []string{CONSTRAINT_KIND_PREFIX},
		},
		{
			nodeName: "Constraint (ALTER TABLE)",
			sql:      `ALTER TABLE ONLY public.foo ADD CONSTRAINT unique_1 UNIQUE (column1, column2) DEFERRABLE;`,
			raw:      []string{"unique_1", "foo", "column1", "column2"},
			prefixes: []string{CONSTRAINT_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			nodeName: "ResTarget",
			sql:      `SELECT salary AS emp_sal FROM emp;`,
			raw:      []string{"emp_sal"},
			prefixes: []string{ALIAS_KIND_PREFIX},
		},
		{
			nodeName: "Alias",
			sql:      `SELECT * FROM customers cust;`,
			raw:      []string{"cust"},
			prefixes: []string{ALIAS_KIND_PREFIX},
		},
		{
			nodeName: "TypeName",
			sql:      `CREATE TABLE t(id my_custom_type);`,
			raw:      []string{"my_custom_type"},
			prefixes: []string{TYPE_KIND_PREFIX}, // flip once you anonymize TypeName.names
		},
		{
			nodeName: "RoleSpec",
			sql:      `GRANT SELECT ON foo TO reporting_user;`,
			raw:      []string{"reporting_user"},
			prefixes: []string{ROLE_KIND_PREFIX},
		},

		/* ---------- Literal nodes ---------- */
		{
			nodeName: "A_Const (string literal)",
			sql:      `INSERT INTO foo VALUES ('superSecret');`,
			raw:      []string{"superSecret"},
			prefixes: []string{CONST_KIND_PREFIX},
		},
		{
			nodeName: "A_ArrayExpr (wrapping A_Const)",
			sql:      `INSERT INTO t(arr) VALUES (ARRAY['abc','xyz','123']);`,
			raw:      []string{"abc", "xyz", "123"},
			prefixes: []string{CONST_KIND_PREFIX},
		},
		{
			nodeName: "A_Indirection (json key)",
			sql:      `SELECT data->'password' FROM foo;`,
			raw:      []string{"password"},
			prefixes: []string{CONST_KIND_PREFIX},
		},
	}

	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	a := newAnon(t, exportDir)

	for _, tc := range cases {
		tc := tc // capture
		t.Run(tc.nodeName, func(t *testing.T) {
			out, err := a.Anonymize(tc.sql)
			if err != nil {
				t.Fatalf("Anonymize: %v", err)
			}
			t.Logf("\nNODE : %s\nIN   : %s\nOUT  : %s", tc.nodeName, tc.sql, out)

			// Raw strings must disappear
			for _, r := range tc.raw {
				if strings.Contains(out, r) {
					t.Errorf("raw identifier/literal %q leaked", r)
				}
			}
			// Expected token prefixes must appear
			for _, p := range tc.prefixes {
				if !hasToken(out, p) {
					t.Errorf("missing token prefix %q", p)
				}
			}
		})
	}
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
			[]string{TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			"schema-qualified",
			`CREATE TABLE sales.orders (OrderID int, Total numeric);`,
			[]string{"sales", "orders", "OrderID", "Total"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			"quoted / mixed case",
			`CREATE TABLE "Customer"."LineItems" ("LineID" int, "ProductSKU" text);`,
			[]string{"Customer", "LineItems", "LineID", "ProductSKU"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
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
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			"constraint",
			"CREATE TABLE foo (id int CONSTRAINT pk_foo PRIMARY KEY);",
			[]string{"foo", "pk_foo"},
			[]string{TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX},
		},
		{
			"alias reference",
			"SELECT * FROM orders order_alias WHERE o.amount > 100;",
			[]string{"orders", "order_alias", "amount"},
			[]string{ALIAS_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
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
