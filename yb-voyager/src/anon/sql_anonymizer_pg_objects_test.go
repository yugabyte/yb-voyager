//go:build unit

package anon

import (
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// enabling one by one as implementation is complete
var enabled = map[string]bool{
	"SCHEMA-CREATE":          true,
	"SCHEMA-RENAME":          true,
	"SCHEMA-CHANGE-OWNER":    true,
	"SCHEMA-DROP":            true,
	"SCHEMA-GRANT-USAGE":     true,
	"COLLATION-CREATE":       true,
	"COLLATION-RENAME":       true,
	"COLLATION-DROP":         true,
	"EXTENSION-CREATE":       true,
	"EXTENSION-ALTER-SCHEMA": true,
	"SEQUENCE-CREATE":        true,
	"SEQUENCE-OWNEDBY":       true,
	"SEQUENCE-RENAME":        true,
	"SEQUENCE-SET-SCHEMA":    true,
	"SEQUENCE-DROP":          true,
	"TYPE-CREATE-ENUM":       true,
	// "TYPE-ADD-VALUE":         true,
	// "TYPE-CREATE-COMPOSITE":  true,
	// "TYPE-CREATE-BASE":       true,
	// "TYPE-CREATE-RANGE":      true,
	// "TYPE-RENAME":            true,
	// "DOMAIN-CREATE":          true,
	// "DOMAIN-RENAME":          true,
}

func hasTok(s, pref string) bool { return strings.Contains(s, pref) }

type ddlCase struct {
	key      string   // unique id, also used in `enabled`
	sql      string   // the statement under test
	raw      []string // identifiers that must vanish
	prefixes []string // token prefixes that must appear
}

func TestPostgresDDLVariants(t *testing.T) {
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	cases := []ddlCase{
		// ─── SCHEMA ───────────────────────────────────────────
		{"SCHEMA-CREATE",
			`CREATE SCHEMA sales;`,
			[]string{"sales"}, []string{SCHEMA_KIND_PREFIX}},
		{"SCHEMA-RENAME",
			`ALTER SCHEMA sales RENAME TO sales_new;`,
			[]string{"sales", "sales_new"}, []string{SCHEMA_KIND_PREFIX}},
		{"SCHEMA-CHANGE-OWNER",
			`ALTER SCHEMA sales_new OWNER TO sales_owner;`,
			[]string{"sales_new", "sales_owner"},
			[]string{SCHEMA_KIND_PREFIX, ROLE_KIND_PREFIX}},
		{"SCHEMA-DROP",
			`DROP SCHEMA IF EXISTS sales_new, sales CASCADE;`,
			[]string{"sales_new", "sales"},
			[]string{SCHEMA_KIND_PREFIX}},
		{"SCHEMA-GRANT-USAGE",
			`GRANT USAGE ON SCHEMA sales TO sales_user;`,
			[]string{"sales", "sales_user"},
			[]string{SCHEMA_KIND_PREFIX, ROLE_KIND_PREFIX}},

		// ─── COLLATION ────────────────────────────────────────
		{
			"COLLATION-CREATE",
			`CREATE COLLATION sales.nocase (provider = icu, locale = 'und');`,
			[]string{"sales", "nocase"},
			[]string{SCHEMA_KIND_PREFIX, COLLATION_KIND_PREFIX},
		},
		{
			"COLLATION-RENAME",
			`ALTER COLLATION sales.nocase RENAME TO nocase2;`,
			[]string{"sales", "nocase", "nocase2"},
			[]string{SCHEMA_KIND_PREFIX, COLLATION_KIND_PREFIX},
		},

		// ─── EXTENSION ───────────────────────────────────────
		{
			"EXTENSION-CREATE",
			`CREATE EXTENSION IF NOT EXISTS postgis SCHEMA sales;`,
			[]string{"sales"},
			[]string{SCHEMA_KIND_PREFIX},
		},
		{
			"EXTENSION-ALTER-SCHEMA",
			`ALTER EXTENSION postgis SET SCHEMA archive;`,
			[]string{"archive"},
			[]string{SCHEMA_KIND_PREFIX},
		},
		{
			"EXTENSION-DROP",
			`DROP EXTENSION IF EXISTS postgis CASCADE;`,
			[]string{},
			[]string{},
		},

		// ─── SEQUENCE ─────────────────────────────────────────
		{"SEQUENCE-CREATE",
			`CREATE SEQUENCE sales.ord_id_seq;`,
			[]string{"sales", "ord_id_seq"},
			[]string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
		{"SEQUENCE-OWNEDBY",
			`ALTER SEQUENCE sales.ord_id_seq OWNED BY dbname.sales.orders.id;`,
			[]string{"dbname", "sales", "ord_id_seq", "orders", "id"},
			[]string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"SEQUENCE-RENAME",
			`ALTER SEQUENCE sales.ord_id_seq RENAME TO ord_id_seq2;`,
			[]string{"sales", "ord_id_seq", "ord_id_seq2"},
			[]string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
		{"SEQUENCE-SET-SCHEMA",
			`ALTER SEQUENCE sales.ord_id_seq SET SCHEMA archive;`,
			[]string{"sales", "ord_id_seq", "archive"},
			[]string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
		{"SEQUENCE-DROP",
			`DROP SEQUENCE IF EXISTS sales.ord_id_seq CASCADE;`,
			[]string{"sales", "ord_id_seq"},
			[]string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},

		// ─── TYPE / ENUM ───────────────────────────────────────
		{"TYPE-CREATE-ENUM",
			`CREATE TYPE postgres.schema1.status AS ENUM ('new','proc','done');`,
			[]string{"postgres", "schema1", "status", "new", "proc", "done"},
			[]string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, ENUM_KIND_PREFIX}},
		{"TYPE-ADD-VALUE",
			`ALTER TYPE status ADD VALUE 'archived';`,
			[]string{"status", "archived"},
			[]string{TYPE_KIND_PREFIX, ENUM_KIND_PREFIX}},
		// ─── TYPE (composite, base, range) ─────────────────────
		{"TYPE-CREATE-COMPOSITE",
			`CREATE TYPE mycomposit AS (a int, b text);`,
			[]string{"mycomposit", "a", "b"},
			[]string{TYPE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TYPE-CREATE-BASE",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out);`,
			[]string{"mybase", "mybase_in", "mybase_out"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE",
			`CREATE TYPE myrange AS RANGE (subtype = int4);`,
			[]string{"myrange", "int4"},
			[]string{TYPE_KIND_PREFIX, TYPE_KIND_PREFIX}},
		{"TYPE-RENAME",
			`ALTER TYPE mycomposit RENAME TO mycomposit2;`,
			[]string{"mycomposit", "mycomposit2"},
			[]string{TYPE_KIND_PREFIX}},

		// ─── DOMAIN ────────────────────────────────────────────
		{"DOMAIN-CREATE",
			`CREATE DOMAIN us_postal AS text CHECK (VALUE ~ '^[0-9]{5}$');`,
			[]string{"us_postal"}, []string{TYPE_KIND_PREFIX}},
		{"DOMAIN-RENAME",
			`ALTER DOMAIN us_postal RENAME TO us_zip;`,
			[]string{"us_postal", "us_zip"}, []string{TYPE_KIND_PREFIX}},

		// ─── TABLE ─────────────────────────────────────────────
		{"TABLE-CREATE",
			`CREATE TABLE sales.orders (id int PRIMARY KEY, amt numeric);`,
			[]string{"sales", "orders", "id", "amt"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ADD-COLUMN",
			`ALTER TABLE sales.orders ADD COLUMN note text;`,
			[]string{"sales", "orders", "note"},
			[]string{TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-RENAME-COLUMN",
			`ALTER TABLE sales.orders RENAME COLUMN amt TO amount;`,
			[]string{"sales", "orders", "amt", "amount"},
			[]string{COLUMN_KIND_PREFIX}},
		{"TABLE-SET-TABLESPACE",
			`ALTER TABLE sales.orders SET TABLESPACE fastspace;`,
			[]string{"sales", "orders", "fastspace"},
			[]string{TABLE_KIND_PREFIX, DEFAULT_KIND_PREFIX}},

		// ─── INDEX ─────────────────────────────────────────────
		{"INDEX-CREATE",
			`CREATE INDEX idx_amt ON sales.orders (amount);`,
			[]string{"idx_amt", "sales", "orders", "amount"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CLUSTER",
			`ALTER TABLE sales.orders CLUSTER ON idx_amt;`,
			[]string{"sales", "orders", "idx_amt"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── POLICY ────────────────────────────────────────────
		{"POLICY-CREATE",
			`CREATE POLICY p_sel ON sales.orders FOR SELECT USING (true);`,
			[]string{"p_sel", "sales", "orders"},
			[]string{CONSTRAINT_KIND_PREFIX, SCHEMA_KIND_PREFIX}},
		{"POLICY-RENAME",
			`ALTER POLICY p_sel ON sales.orders RENAME TO p_sel2;`,
			[]string{"p_sel", "p_sel2"}, []string{CONSTRAINT_KIND_PREFIX}},

		// ─── COMMENT ───────────────────────────────────────────
		{"COMMENT-TABLE",
			`COMMENT ON TABLE sales.orders IS 'order table';`,
			[]string{"sales", "orders"},
			[]string{TABLE_KIND_PREFIX}},
		{"COMMENT-COLUMN",
			`COMMENT ON COLUMN sales.orders.amount IS 'gross amount';`,
			[]string{"sales", "orders", "amount"},
			[]string{COLUMN_KIND_PREFIX}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
			if !enabled[c.key] {
				t.Skip("disabled until anonymizer supports " + c.key)
			}
			out, err := az.Anonymize(c.sql)
			if err != nil {
				t.Fatalf("anonymize: %v", err)
			}
			fmt.Printf("Test Name: %s\nIN: %s\nOUT: %s\n\n", c.key, c.sql, out)
			for _, raw := range c.raw {
				if strings.Contains(out, raw) {
					t.Errorf("raw identifier %q leaked in %s", raw, out)
				}
			}
			for _, pref := range c.prefixes {
				if !hasTok(out, pref) {
					t.Errorf("expected prefix %q not found in %s", pref, out)
				}
			}
		})
	}
}

// ============================================================================
//                          DDL ANONYMIZATION COVERAGE MATRIX
//
//  Object Type          | DDL Variant                                     | Parse Node                   | Status
//  ---------------------|-------------------------------------------------|------------------------------|---------
//  SCHEMA               | CREATE SCHEMA <name>                            | CreateSchemaStmtNode         | [x]
//                       | ALTER SCHEMA <name> RENAME TO <newname>         | RenameStmtNode               | [x]
//                       | ALTER SCHEMA <name> OWNER TO <role>             | AlterOwnerStmtNode           | [x]
//                       | DROP SCHEMA [IF EXISTS] <name>[CASCADE|RESTRICT]| DropStmtNode                 | [x]
//                       | CREATE SCHEMA ... AUTHORIZATION <role>          | CreateSchemaStmtNode         | [ ]
//                       | COMMENT ON SCHEMA <name>                        | CommentOnStmtNode            | [ ]
//                       | GRANT USAGE ON SCHEMA <name> TO <role>          | GrantStmtNode                | [x]
//                       | REVOKE ... ON SCHEMA <name> FROM <role>         | RevokeStmtNode               | [ ]
//
//  COLLATION            | CREATE COLLATION <schema>.<name>                | CreateCollationStmtNode      | [x]
//                       | ALTER COLLATION ... RENAME TO <new>             | RenameStmtNode               | [x]
//                       | DROP COLLATION ...                              | DropStmtNode                 | [x]
//
//  EXTENSION            | CREATE EXTENSION IF NOT EXISTS <name>           | CreateExtensionStmtNode      | [ ]
//                       | ALTER EXTENSION <name> SET SCHEMA <schema>      | AlterExtensionStmtNode       | [ ]
//                       | DROP EXTENSION <name>                           | DropStmtNode                 | [ ]
//
//  TYPE (ENUM)          | CREATE TYPE <name> AS ENUM (...)                | CreateEnumStmtNode           | [x]
//                       | ALTER TYPE <name> ADD VALUE <val>               | AlterEnumStmtNode            | [x]
//                       | ALTER TYPE <name> RENAME TO <new>               | RenameStmtNode               | [ ]
//                       | DROP TYPE <name>                                | DropStmtNode                 | [ ]
//
//  DOMAIN               | CREATE DOMAIN <name> AS <base> ...              | CreateDomainStmtNode         | [ ]
//                       | ALTER DOMAIN <name> RENAME TO <new>             | RenameStmtNode               | [ ]
//                       | DROP DOMAIN <name>                              | DropStmtNode                 | [ ]
//
//  SEQUENCE             | CREATE SEQUENCE <schema>.<name>                 | CreateSeqStmtNode            | [ ]
//                       | ALTER SEQUENCE <schema>.<name> OWNED BY ...     | AlterSeqStmtNode             | [ ]
//                       | ALTER SEQUENCE RENAME TO <new>                  | RenameStmtNode               | [ ]
//                       | DROP SEQUENCE <schema>.<name>                   | DropStmtNode                 | [ ]
//
//  TABLE                | CREATE TABLE <schema>.<name> (...)              | CreateStmtNode               | [ ]
//                       | ALTER TABLE <name> ADD COLUMN ...               | AlterTableStmtNode           | [ ]
//                       | ALTER TABLE <name> RENAME TO <new>              | RenameStmtNode               | [ ]
//                       | ALTER TABLE <name> SET TABLESPACE <ts>          | AlterTableStmtNode           | [ ]
//                       | DROP TABLE <name> [CASCADE|RESTRICT]            | DropStmtNode                 | [ ]
//                       | TRUNCATE TABLE <name>                           | TruncateStmtNode             | [ ]
//
//  INDEX                | CREATE INDEX <name> ON <table> (...)            | IndexStmtNode                | [ ]
//                       | ALTER INDEX <name> RENAME TO <new>              | RenameStmtNode               | [ ]
//                       | DROP INDEX <name> [CASCADE|RESTRICT]            | DropStmtNode                 | [ ]
//
//  POLICY               | CREATE POLICY <name> ON <table> ...             | CreatePolicyStmtNode         | [ ]
//                       | ALTER POLICY <name> RENAME TO <new>             | AlterPolicyStmtNode          | [ ]
//                       | DROP POLICY <name>                              | DropStmtNode                 | [ ]
//
//  COMMENT              | COMMENT ON TABLE/COLUMN/...                     | CommentOnStmtNode            | [ ]
//
//  CONVERSION           | CREATE CONVERSION <schema>.<name> ...           | CreateConversionStmtNode     | [ ]
//                       | ALTER CONVERSION <name> RENAME TO <new>         | RenameStmtNode               | [ ]
//                       | DROP CONVERSION <name>                          | DropStmtNode                 | [ ]
//
//  FOREIGN TABLE        | CREATE FOREIGN TABLE <schema>.<name> ...        | CreateForeignTableStmtNode   | [ ]
//                       | ALTER FOREIGN TABLE <name> RENAME TO <new>      | RenameStmtNode               | [ ]
//                       | DROP FOREIGN TABLE <name>                       | DropStmtNode                 | [ ]
//
//  OPERATOR             | CREATE OPERATOR <schema>.<name> ...             | CreateOperatorStmtNode       | [ ]
//                       | ALTER OPERATOR <name> RENAME TO <new>           | RenameStmtNode               | [ ]
//                       | DROP OPERATOR <schema>.<name>                   | DropStmtNode                 | [ ]
//
//  TRIGGER               ... (Create/Alter/Drop)                          | [ ]
//  VIEW                  ...                                            | [ ]
//  MVIEW                 ...                                            | [ ]
//  RULE                  ...                                            | [ ]
//  FUNCTION / PROCEDURE  ...                                            | [ ]
//  AGGREGATE             ...                                            | [ ]
//  OPERATOR CLASS / FAMILY                                                  | [ ]
//
//  NOTE: After implementing a specific case, flip its [ ] to [x] above.
// ============================================================================
