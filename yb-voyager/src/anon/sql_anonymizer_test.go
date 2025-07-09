//go:build unit

package anon

import (
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"gotest.tools/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

var identifierRegistryTest IdentifierHasher

/*
	Tests to add:
	1. Anonymizing tablename, column name, schema name in DDLs
	2. Anonymizing Index name in DDLs
	3. Anonymizing constraint name in DDLs
	4. Anonymizing alias names in DMLs
	5. Anonymizing function names in DDLs
	6. Also cover case sensitivity
*/

func newAnon(t *testing.T, exportDir string) Anonymizer {
	salt, err := utils.GenerateAnonymisationSalt(16)
	testutils.FatalIfError(t, err)

	identifierRegistryTest, err = NewIdentifierHashRegistry(salt)
	testutils.FatalIfError(t, err)

	a := NewSqlAnonymizer(identifierRegistryTest)
	testutils.FatalIfError(t, err)

	return a
}

func getIdentifierHash(t *testing.T, a Anonymizer, orig string) string {
	sqlAnonymizer, ok := a.(*SqlAnonymizer)
	if !ok {
		t.Fatalf("expected SqlAnonymizer, got %T", a)
	}

	schemaRegistry, ok := sqlAnonymizer.registry.(*IdentifierHashRegistry)
	if !ok {
		t.Fatalf("expected IdentifierHashRegistry, got %T", sqlAnonymizer.registry)
	}

	return schemaRegistry.identifierHashMap[orig]
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
			sql:      `CREATE INDEX idx_emp_name_date ON hr.employee(last_name, first_name, hire_date);`,
			raw:      []string{"idx_emp_name_date", "hr", "employee", "last_name", "first_name", "hire_date"},
			prefixes: []string{INDEX_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
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
			nodeName: "Cluster ON (ALTER TABLE)",
			sql:      `ALTER TABLE humanresources.department CLUSTER ON "PK_Department_DepartmentID";`,
			raw:      []string{"humanresources", "department", "PK_Department_DepartmentID"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX},
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
		/* ---------- Miscellaneous ---------- */
		{
			nodeName: "CreateExtensionStmt",
			sql:      `CREATE EXTENSION IF NOT EXISTS postgis SCHEMA public`,
			raw:      []string{"public"},
			prefixes: []string{SCHEMA_KIND_PREFIX},
		},
		{
			nodeName: "CreateForeignTableStmt",
			sql:      `CREATE FOREIGN TABLE foreign_table_foo(col1 serial, col2_ts timestamptz(0) default now(), col_j json, col_t text, col_enum myenum, column_composite mycomposit) server p10 options (TABLE_name 'remote_table');`,
			raw:      []string{"foreign_table_foo", "col1", "col2_ts", "col_j", "col_t", "col_enum", "column_composite", "remote_table" /*, "p10"*/},
			prefixes: []string{TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, TYPE_KIND_PREFIX},
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

func TestSameTokenForSameObjectName(t *testing.T) {
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	a := newAnon(t, exportDir)

	cases := []struct {
		name        string
		identifiers []struct{ kind, raw string }
		sql1, sql2  string
	}{
		{
			name: "same table & columns",
			identifiers: []struct{ kind, raw string }{
				{TABLE_KIND_PREFIX, "users"},
				{COLUMN_KIND_PREFIX, "id"},
				{COLUMN_KIND_PREFIX, "name"},
			},
			sql1: "SELECT * FROM users",
			sql2: "SELECT id, name FROM users WHERE id > 10",
		},
		{
			name: "same column",
			identifiers: []struct{ kind, raw string }{
				{COLUMN_KIND_PREFIX, "password"},
				{TABLE_KIND_PREFIX, "accounts"},
				{COLUMN_KIND_PREFIX, "id"},
				{CONST_KIND_PREFIX, "x"},
			},
			sql1: "SELECT password FROM accounts",
			sql2: "UPDATE accounts SET password = 'x' WHERE id = 1",
		},
		{
			name: "same alias",
			identifiers: []struct{ kind, raw string }{
				{ALIAS_KIND_PREFIX, "cust"},
				{TABLE_KIND_PREFIX, "orders"},
				{COLUMN_KIND_PREFIX, "amount"},
				{SCHEMA_KIND_PREFIX, "cust"},
			},
			sql1: "SELECT * FROM orders cust",
			sql2: "SELECT cust.amount FROM orders cust WHERE cust.amount > 100",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual1, err := a.Anonymize(tc.sql1)
			if err != nil {
				t.Fatalf("1st Anonymize: %v", err)
			}
			actual2, err := a.Anonymize(tc.sql2)
			if err != nil {
				t.Fatalf("2nd Anonymize: %v", err)
			}

			expected1 := tc.sql1
			expected2 := tc.sql2

			// For each (kind, raw) pair, lookup token via kind+raw
			for _, e := range tc.identifiers {
				token := getIdentifierHash(t, a, e.kind+e.raw)
				if token == "" {
					t.Fatalf("no token for kind+raw %q", e.kind+e.raw)
				}
				t.Logf("Replacing raw %q with token %q", e.raw, token)
				expected1 = strings.ReplaceAll(expected1, e.raw, token)
				expected2 = strings.ReplaceAll(expected2, e.raw, token)
			}

			t.Logf("\nIN1 : %s\nOUT1: %s\nEXP1: %s", tc.sql1, actual1, expected1)
			t.Logf("\nIN2 : %s\nOUT2: %s\nEXP2: %s", tc.sql2, actual2, expected2)

			assert.Equal(t, actual1, expected1, "Anonymized SQL1 mismatch: \n got: %s\nwant: %s", actual1, expected1)
			assert.Equal(t, actual2, expected2, "Anonymized SQL2 mismatch: \n got: %s\nwant: %s", actual2, expected2)
		})
	}
}

// We have not enabled anonymization for DMLs right now
// but building test suite for that already
// func TestDMLs(t *testing.T) {
// 	sqlStatements := []string{
// 		`SELECT
//     s.section_name,
//     b.title,
//     b.author
// FROM
//     library_nested l,
//     XMLTABLE(
//         '/library/section'
//         PASSING l.lib_data
//         COLUMNS
//             section_name TEXT PATH '@name',
//             books XML PATH '.'
//     ) AS s,
//     XMLTABLE(
//         '/section/book'
//         PASSING s.books
//         COLUMNS
//             title TEXT PATH 'title',
//             author TEXT PATH 'author'
// ) AS b;`,
// 	}

// 	exportDir := testutils.CreateTempExportDir()
// 	defer testutils.RemoveTempExportDir(exportDir)
// 	a := newAnon(t, exportDir)
// 	for _, sql := range sqlStatements {
// 		t.Run("DML", func(t *testing.T) {
// 			out, err := a.Anonymize(sql)
// 			if err != nil {
// 				t.Fatalf("Anonymize error: %v", err)
// 			}
// 			t.Logf("\nIN : %s\nOUT: %s", sql, out)
// 		})
// 	}
// }
