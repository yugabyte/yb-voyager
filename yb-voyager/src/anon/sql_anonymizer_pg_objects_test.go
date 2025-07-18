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
	"SCHEMA-CREATE":             true,
	"SCHEMA-RENAME":             true,
	"SCHEMA-CHANGE-OWNER":       true,
	"SCHEMA-DROP":               true,
	"SCHEMA-GRANT-USAGE":        true,
	"COLLATION-CREATE":          true,
	"COLLATION-RENAME":          true,
	"COLLATION-DROP":            true,
	"EXTENSION-CREATE":          true,
	"EXTENSION-ALTER-SCHEMA":    true,
	"SEQUENCE-CREATE":           true,
	"SEQUENCE-OWNEDBY":          true,
	"SEQUENCE-RENAME":           true,
	"SEQUENCE-SET-SCHEMA":       true,
	"SEQUENCE-DROP":             true,
	"TYPE-CREATE-ENUM":          true,
	"TYPE-ALTER-ENUM-ADD-VALUE": true,
	"TYPE-CREATE-COMPOSITE":     true,
	// "TYPE-CREATE-BASE":       true,
	// "TYPE-CREATE-RANGE":      true,
	"TYPE-RENAME":   true,
	"DOMAIN-CREATE": true,
	"DOMAIN-RENAME": true,
	"DOMAIN-DROP":   true,

	"TABLE-CREATE":             true,
	"TABLE-CREATE-AS":          true,
	"TABLE-CREATE-LIKE":        true,
	"TABLE-RENAME":             true,
	"TABLE-ADD-COLUMN":         true,
	"TABLE-RENAME-COLUMN":      true,
	"TABLE-DROP-COLUMN":        true,
	"TABLE-ALTER-COLUMN-TYPE":  true,
	"TABLE-ADD-CONSTRAINT-PK":  true,
	"TABLE-ADD-CONSTRAINT-FK":  true,
	"TABLE-ADD-CONSTRAINT-UK":  true,
	"TABLE-ADD-CONSTRAINT-CHK": true,
	"TABLE-DROP-CONSTRAINT":    true,
	"TABLE-SET-SCHEMA":         true,
	"TABLE-CHANGE-OWNER":       true,
	"TABLE-DROP":               true,
	"TABLE-TRUNCATE":           true,

	// Additional ALTER TABLE operations
	"TABLE-ALTER-COLUMN-DEFAULT":       true,
	"TABLE-ALTER-COLUMN-DROP-DEFAULT":  true,
	"TABLE-ALTER-COLUMN-SET-NOT-NULL":  true,
	"TABLE-ALTER-COLUMN-DROP-NOT-NULL": true,
	"TABLE-ALTER-COLUMN-SET-OPTIONS":   true,
	"TABLE-ALTER-COLUMN-RESET-OPTIONS": true,
	"TABLE-ALTER-CONSTRAINT":           true,
	"TABLE-VALIDATE-CONSTRAINT":        true,
	"TABLE-CLUSTER-ON-INDEX":           true,
	"TABLE-ADD-INDEX-CONSTRAINT":       true,
	"TABLE-ENABLE-TRIGGER":             true,
	"TABLE-DISABLE-TRIGGER":            true,
	"TABLE-ENABLE-ALWAYS-TRIGGER":      true,
	"TABLE-ENABLE-REPLICA-TRIGGER":     true,
	"TABLE-ENABLE-RULE":                true,
	"TABLE-DISABLE-RULE":               true,
	"TABLE-ENABLE-ALWAYS-RULE":         true,
	"TABLE-ENABLE-REPLICA-RULE":        true,
	"TABLE-ADD-IDENTITY":               true,
	"TABLE-SET-IDENTITY":               true,
	"TABLE-DROP-IDENTITY":              true,
	"TABLE-ADD-INHERIT":                true,
	"TABLE-DROP-INHERIT":               true,
	"TABLE-ADD-OF-TYPE":                true,
	"TABLE-ATTACH-PARTITION":           true,
	"TABLE-DETACH-PARTITION":           true,
	"TABLE-DETACH-PARTITION-FINALIZE":  true,
	"TABLE-REPLICA-IDENTITY-INDEX":     true,
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

		// ─── TYPE (ENUM) ───────────────────────────────────────
		{"TYPE-CREATE-ENUM",
			`CREATE TYPE postgres.schema1.status AS ENUM ('new','proc','done');`,
			[]string{"postgres", "schema1", "status", "new", "proc", "done"},
			[]string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, ENUM_KIND_PREFIX}},
		{"TYPE-ALTER-ENUM-ADD-VALUE",
			`ALTER TYPE status ADD VALUE 'archived';`,
			[]string{"status", "archived"},
			[]string{TYPE_KIND_PREFIX, ENUM_KIND_PREFIX}},

		// ─── TYPE (composite, base, range) ─────────────────────
		{"TYPE-CREATE-COMPOSITE",
			`CREATE TYPE dbname.schema1.mycomposit AS (col1 int, col2 text);`,
			[]string{"dbname", "schema1", "mycomposit", "col1", "col2"},
			[]string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
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
			[]string{"us_postal", "text", "VALUE", "^[0-9]{5}$"},
			[]string{DOMAIN_KIND_PREFIX, TYPE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{"DOMAIN-RENAME",
			`ALTER DOMAIN us_postal RENAME TO us_zip;`,
			[]string{"us_postal", "us_zip"},
			[]string{DOMAIN_KIND_PREFIX}},
		{"DOMAIN-DROP",
			`DROP DOMAIN IF EXISTS us_postal CASCADE;`,
			[]string{"us_postal"},
			[]string{DOMAIN_KIND_PREFIX}},

		// ─── TABLE ─────────────────────────────────────────────
		// CREATE operations
		{"TABLE-CREATE",
			`CREATE TABLE sales.orders (id int PRIMARY KEY, amt numeric);`,
			[]string{"sales", "orders", "id", "amt"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-CREATE-AS",
			`CREATE TABLE sales.order_summary AS SELECT customer_id, COUNT(*) FROM sales.orders GROUP BY customer_id;`,
			[]string{"sales", "order_summary", "customer_id", "orders"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-CREATE-LIKE",
			`CREATE TABLE sales.orders_backup (LIKE sales.orders INCLUDING ALL);`,
			[]string{"sales", "orders_backup", "orders"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ALTER operations
		{"TABLE-RENAME",
			`ALTER TABLE sales.orders RENAME TO order_history;`,
			[]string{"sales", "orders", "order_history"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"TABLE-ADD-COLUMN",
			`ALTER TABLE sales.orders ADD COLUMN note text;`,
			[]string{"sales", "orders", "note"},
			[]string{TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-RENAME-COLUMN",
			`ALTER TABLE sales.orders RENAME COLUMN amt TO amount;`,
			[]string{"sales", "orders", "amt", "amount"},
			[]string{COLUMN_KIND_PREFIX}},
		{"TABLE-DROP-COLUMN",
			`ALTER TABLE sales.orders DROP COLUMN note;`,
			[]string{"sales", "orders", "note"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ALTER-COLUMN-TYPE",
			`ALTER TABLE sales.orders ALTER COLUMN amount TYPE decimal(10,2);`,
			[]string{"sales", "orders", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ADD-CONSTRAINT-PK",
			`ALTER TABLE sales.orders ADD CONSTRAINT pk_orders PRIMARY KEY (id);`,
			[]string{"sales", "orders", "pk_orders", "id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ADD-CONSTRAINT-FK",
			`ALTER TABLE sales.orders ADD CONSTRAINT fk_customer FOREIGN KEY (customer_id) REFERENCES customers(id);`,
			[]string{"sales", "orders", "fk_customer", "customer_id", "customers", "id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ADD-CONSTRAINT-UK",
			`ALTER TABLE sales.orders ADD CONSTRAINT uk_order_number UNIQUE (order_number);`,
			[]string{"sales", "orders", "uk_order_number", "order_number"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ADD-CONSTRAINT-CHK",
			`ALTER TABLE sales.orders ADD CONSTRAINT chk_amount CHECK (amount > 0);`,
			[]string{"sales", "orders", "chk_amount", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-DROP-CONSTRAINT",
			`ALTER TABLE sales.orders DROP CONSTRAINT chk_amount;`,
			[]string{"sales", "orders", "chk_amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX}},
		{"TABLE-SET-SCHEMA",
			`ALTER TABLE sales.orders SET SCHEMA archive;`,
			[]string{"sales", "orders", "archive"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"TABLE-CHANGE-OWNER",
			`ALTER TABLE sales.orders OWNER TO order_admin;`,
			[]string{"sales", "orders", "order_admin"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, ROLE_KIND_PREFIX}},

		// Other operations
		{"TABLE-DROP",
			`DROP TABLE IF EXISTS sales.orders CASCADE;`,
			[]string{"sales", "orders"},
			[]string{TABLE_KIND_PREFIX}},
		{"TABLE-TRUNCATE",
			`TRUNCATE TABLE sales.orders;`,
			[]string{"sales", "orders"},
			[]string{TABLE_KIND_PREFIX}},

		// ─── ADDITIONAL ALTER TABLE COLUMN OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ALTER-COLUMN-DEFAULT",
			`ALTER TABLE sales.orders ALTER COLUMN created_at SET DEFAULT NOW();`,
			[]string{"sales", "orders", "created_at"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ALTER-COLUMN-DROP-DEFAULT",
			`ALTER TABLE sales.orders ALTER COLUMN created_at DROP DEFAULT;`,
			[]string{"sales", "orders", "created_at"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ALTER-COLUMN-SET-NOT-NULL",
			`ALTER TABLE sales.orders ALTER COLUMN customer_name SET NOT NULL;`,
			[]string{"sales", "orders", "customer_name"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ALTER-COLUMN-DROP-NOT-NULL",
			`ALTER TABLE sales.orders ALTER COLUMN description DROP NOT NULL;`,
			[]string{"sales", "orders", "description"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ALTER-COLUMN-SET-OPTIONS",
			`ALTER TABLE sales.orders ALTER COLUMN notes SET (n_distinct = 100);`,
			[]string{"sales", "orders", "notes"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-ALTER-COLUMN-RESET-OPTIONS",
			`ALTER TABLE sales.orders ALTER COLUMN notes RESET (n_distinct);`,
			[]string{"sales", "orders", "notes"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── CONSTRAINT OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ALTER-CONSTRAINT",
			`ALTER TABLE sales.orders ALTER CONSTRAINT fk_customer DEFERRABLE;`,
			[]string{"sales", "orders", "fk_customer"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX}},
		{"TABLE-VALIDATE-CONSTRAINT",
			`ALTER TABLE sales.orders VALIDATE CONSTRAINT chk_amount;`,
			[]string{"sales", "orders", "chk_amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX}},

		// ─── INDEX OPERATIONS ─────────────────────────────────────────────
		{"TABLE-CLUSTER-ON-INDEX",
			`ALTER TABLE sales.orders CLUSTER ON idx_customer_id;`,
			[]string{"sales", "orders", "idx_customer_id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, INDEX_KIND_PREFIX}},
		{"TABLE-ADD-INDEX-CONSTRAINT",
			`ALTER TABLE sales.orders ADD CONSTRAINT uq_order_number UNIQUE USING INDEX idx_unique_order_num;`,
			[]string{"sales", "orders", "idx_unique_order_num", "uq_order_number"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, INDEX_KIND_PREFIX}},

		// ─── TRIGGER OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ENABLE-TRIGGER",
			`ALTER TABLE sales.orders ENABLE TRIGGER audit_trigger;`,
			[]string{"sales", "orders", "audit_trigger"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},
		{"TABLE-DISABLE-TRIGGER",
			`ALTER TABLE sales.orders DISABLE TRIGGER audit_trigger;`,
			[]string{"sales", "orders", "audit_trigger"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},
		{"TABLE-ENABLE-ALWAYS-TRIGGER",
			`ALTER TABLE sales.orders ENABLE ALWAYS TRIGGER security_trigger;`,
			[]string{"sales", "orders", "security_trigger"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},
		{"TABLE-ENABLE-REPLICA-TRIGGER",
			`ALTER TABLE sales.orders ENABLE REPLICA TRIGGER sync_trigger;`,
			[]string{"sales", "orders", "sync_trigger"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},

		// ─── RULE OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ENABLE-RULE",
			`ALTER TABLE sales.orders ENABLE RULE order_rule;`,
			[]string{"sales", "orders", "order_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{"TABLE-DISABLE-RULE",
			`ALTER TABLE sales.orders DISABLE RULE order_rule;`,
			[]string{"sales", "orders", "order_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{"TABLE-ENABLE-ALWAYS-RULE",
			`ALTER TABLE sales.orders ENABLE ALWAYS RULE audit_rule;`,
			[]string{"sales", "orders", "audit_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{"TABLE-ENABLE-REPLICA-RULE",
			`ALTER TABLE sales.orders ENABLE REPLICA RULE sync_rule;`,
			[]string{"sales", "orders", "sync_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}}, // Using TRIGGER prefix for rules

		// ─── IDENTITY OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ADD-IDENTITY",
			`ALTER TABLE sales.orders ALTER COLUMN order_id ADD GENERATED BY DEFAULT AS IDENTITY;`,
			[]string{"sales", "orders", "order_id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-SET-IDENTITY",
			`ALTER TABLE sales.orders ALTER COLUMN order_id SET GENERATED ALWAYS;`,
			[]string{"sales", "orders", "order_id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TABLE-DROP-IDENTITY",
			`ALTER TABLE sales.orders ALTER COLUMN order_id DROP IDENTITY;`,
			[]string{"sales", "orders", "order_id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── INHERITANCE OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ADD-INHERIT",
			`ALTER TABLE sales.special_orders INHERIT sales.orders;`,
			[]string{"sales", "special_orders", "orders"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"TABLE-DROP-INHERIT",
			`ALTER TABLE sales.special_orders NO INHERIT sales.orders;`,
			[]string{"sales", "special_orders", "orders"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── TYPE OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ADD-OF-TYPE",
			`ALTER TABLE sales.typed_orders OF sales.order_type;`,
			[]string{"sales", "typed_orders", "order_type"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TYPE_KIND_PREFIX}},

		// ─── PARTITION OPERATIONS ─────────────────────────────────────────────
		{"TABLE-ATTACH-PARTITION",
			`ALTER TABLE sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');`,
			[]string{"sales", "orders", "orders_2024"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"TABLE-DETACH-PARTITION",
			`ALTER TABLE sales.orders DETACH PARTITION sales.orders_2024;`,
			[]string{"sales", "orders", "orders_2024"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"TABLE-DETACH-PARTITION-FINALIZE",
			`ALTER TABLE sales.orders DETACH PARTITION sales.orders_2024 FINALIZE;`,
			[]string{"sales", "orders", "orders_2024"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── REPLICA IDENTITY OPERATIONS ─────────────────────────────────────────────
		{"TABLE-REPLICA-IDENTITY-INDEX",
			`ALTER TABLE sales.orders REPLICA IDENTITY USING INDEX idx_orders_pkey;`,
			[]string{"sales", "orders", "idx_orders_pkey"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, INDEX_KIND_PREFIX}},

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
//  EXTENSION            | CREATE EXTENSION IF NOT EXISTS <name>           | CreateExtensionStmtNode      | [x]
//                       | ALTER EXTENSION <name> SET SCHEMA <schema>      | AlterExtensionStmtNode       | [x]
//                       | DROP EXTENSION <name>                           | DropStmtNode                 | [x]
//
//  Verify with variants of TYPE at this - https://www.postgresql.org/docs/current/sql-createtype.html
//  TYPE (ENUM)          | CREATE TYPE <name> AS ENUM (...)                | CreateEnumStmtNode           | [x]
//                       | ALTER TYPE <name> RENAME TO <new>               | RenameStmtNode               | [x]
//                       | DROP TYPE <name>                                | DropStmtNode                 | [x]
//
//  DOMAIN               | CREATE DOMAIN <name> AS <base> ...              | CreateDomainStmtNode         | [x]
//                       | ALTER DOMAIN <name> RENAME TO <new>             | RenameStmtNode               | [x]
//                       | DROP DOMAIN <name>                              | DropStmtNode                 | [x]
//
//  SEQUENCE             | CREATE SEQUENCE <schema>.<name>                 | CreateSeqStmtNode            | [x]
//                       | ALTER SEQUENCE <schema>.<name> OWNED BY ...     | AlterSeqStmtNode             | [x]
//                       | ALTER SEQUENCE RENAME TO <new>                  | RenameStmtNode               | [x]
//                       | DROP SEQUENCE <schema>.<name>                   | DropStmtNode                 | [x]
//
//  TABLE                | CREATE TABLE <schema>.<name> (...)              | CreateStmtNode               | [x]
//                       | ALTER TABLE <name> ADD COLUMN ...               | AlterTableStmtNode           | [x]
//                       | ALTER TABLE <name> RENAME TO <new>              | RenameStmtNode               | [x]
//                       | DROP TABLE <name> [CASCADE|RESTRICT]            | DropStmtNode                 | [x]
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
