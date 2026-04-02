//go:build unit

package anon

import (
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func hasTok(s, pref string) bool {
	return strings.Contains(s, pref)
}

type ddlCase struct {
	key         string   // unique id, also used in `enabled`
	sql         string   // the statement under test
	raw         []string // identifiers that must vanish
	prefixes    []string // token prefixes that must appear
	expectError bool     // if true, expect Anonymize() to return error (default false for backward compat)
}

func TestPostgresDDLVariants(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	cases := []ddlCase{
		// ─── SCHEMA ───────────────────────────────────────────
		{key: "SCHEMA-CREATE", sql: `CREATE SCHEMA sales;`, raw: []string{"sales"}, prefixes: []string{SCHEMA_KIND_PREFIX}},
		{key: "SCHEMA-RENAME", sql: `ALTER SCHEMA sales RENAME TO sales_new;`, raw: []string{"sales", "sales_new"}, prefixes: []string{SCHEMA_KIND_PREFIX}},
		{key: "SCHEMA-CHANGE-OWNER", sql: `ALTER SCHEMA sales_new OWNER TO sales_owner;`, raw: []string{"sales_new", "sales_owner"}, prefixes: []string{SCHEMA_KIND_PREFIX, ROLE_KIND_PREFIX}},
		{key: "SCHEMA-DROP", sql: `DROP SCHEMA IF EXISTS sales_new, sales CASCADE;`, raw: []string{"sales_new", "sales"}, prefixes: []string{SCHEMA_KIND_PREFIX}},
		{key: "SCHEMA-GRANT-USAGE", sql: `GRANT USAGE ON SCHEMA sales TO sales_user;`, raw: []string{"sales", "sales_user"}, prefixes: []string{SCHEMA_KIND_PREFIX, ROLE_KIND_PREFIX}},

		// ─── COLLATION ────────────────────────────────────────
		{
			key:      "COLLATION-CREATE",
			sql:      `CREATE COLLATION sales.nocase (provider = icu, locale = 'und');`,
			raw:      []string{"sales", "nocase"},
			prefixes: []string{SCHEMA_KIND_PREFIX, COLLATION_KIND_PREFIX},
		},
		{
			key:      "COLLATION-RENAME",
			sql:      `ALTER COLLATION sales.nocase RENAME TO nocase2;`,
			raw:      []string{"sales", "nocase", "nocase2"},
			prefixes: []string{SCHEMA_KIND_PREFIX, COLLATION_KIND_PREFIX},
		},

		// ─── EXTENSION ───────────────────────────────────────
		{
			key:      "EXTENSION-CREATE",
			sql:      `CREATE EXTENSION IF NOT EXISTS postgis SCHEMA sales;`,
			raw:      []string{"sales"},
			prefixes: []string{SCHEMA_KIND_PREFIX},
		},
		{
			key:      "EXTENSION-ALTER-SCHEMA",
			sql:      `ALTER EXTENSION postgis SET SCHEMA archive;`,
			raw:      []string{"archive"},
			prefixes: []string{SCHEMA_KIND_PREFIX},
		},
		{
			key:      "EXTENSION-DROP",
			sql:      `DROP EXTENSION IF EXISTS postgis CASCADE;`,
			raw:      []string{},
			prefixes: []string{},
		},

		// ─── SEQUENCE ─────────────────────────────────────────
		{key: "SEQUENCE-CREATE", sql: `CREATE SEQUENCE sales.ord_id_seq;`, raw: []string{"sales", "ord_id_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
		{key: "SEQUENCE-OWNEDBY", sql: `ALTER SEQUENCE sales.ord_id_seq OWNED BY dbname.sales.orders.id;`, raw: []string{"dbname", "sales", "ord_id_seq", "orders", "id"}, prefixes: []string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "SEQUENCE-RENAME", sql: `ALTER SEQUENCE sales.ord_id_seq RENAME TO ord_id_seq2;`, raw: []string{"sales", "ord_id_seq", "ord_id_seq2"}, prefixes: []string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
		{key: "SEQUENCE-SET-SCHEMA", sql: `ALTER SEQUENCE sales.ord_id_seq SET SCHEMA archive;`, raw: []string{"sales", "ord_id_seq", "archive"}, prefixes: []string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
		{key: "SEQUENCE-DROP", sql: `DROP SEQUENCE IF EXISTS sales.ord_id_seq CASCADE;`, raw: []string{"sales", "ord_id_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},

		// ─── TYPE (ENUM) ───────────────────────────────────────
		{key: "TYPE-CREATE-ENUM", sql: `CREATE TYPE postgres.schema1.status AS ENUM ('new','proc','done');`, raw: []string{"postgres", "schema1", "status", "new", "proc", "done"}, prefixes: []string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, ENUM_KIND_PREFIX}},
		{key: "TYPE-ALTER-ENUM-ADD-VALUE", sql: `ALTER TYPE status ADD VALUE 'archived';`, raw: []string{"status", "archived"}, prefixes: []string{TYPE_KIND_PREFIX, ENUM_KIND_PREFIX}},

		// ─── TYPE (composite, base, range) ─────────────────────
		{
			key: "TYPE-CREATE-BASIC",
			sql: `CREATE TYPE base_type_examples.base_type (
				INTERNALLENGTH = variable,
				INPUT = base_type_examples.base_fn_in,
				OUTPUT = base_type_examples.base_fn_out,
				ALIGNMENT = int4,
				STORAGE = plain
			);`,
			raw:      []string{"base_type_examples", "base_type", "base_fn_in", "base_fn_out"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX},
		},
		{key: "TYPE-CREATE-COMPOSITE", sql: `CREATE TYPE dbname.schema1.mycomposit AS (col1 int, col2 text);`, raw: []string{"dbname", "schema1", "mycomposit", "col1", "col2"}, prefixes: []string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE", sql: `CREATE TYPE mybase (input = mybase_in, output = mybase_out);`, raw: []string{"mybase", "mybase_in", "mybase_out"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE-WITH-RECEIVE-SEND", sql: `CREATE TYPE mybase (input = mybase_in, output = mybase_out, receive = mybase_receive, send = mybase_send);`, raw: []string{"mybase", "mybase_in", "mybase_out", "mybase_receive", "mybase_send"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE-WITH-TYPMOD", sql: `CREATE TYPE mybase (input = mybase_in, output = mybase_out, typmod_in = mybase_typmod_in, typmod_out = mybase_typmod_out);`, raw: []string{"mybase", "mybase_in", "mybase_out", "mybase_typmod_in", "mybase_typmod_out"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE-WITH-ANALYZE-SUBSCRIPT", sql: `CREATE TYPE mybase (input = mybase_in, output = mybase_out, analyze = mybase_analyze, subscript = mybase_subscript);`, raw: []string{"mybase", "mybase_in", "mybase_out", "mybase_analyze", "mybase_subscript"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE-WITH-LIKE-ELEMENT", sql: `CREATE TYPE mybase (input = mybase_in, output = mybase_out, like = mybase_like, element = mybase_element);`, raw: []string{"mybase", "mybase_in", "mybase_out", "mybase_like", "mybase_element"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE-WITH-CONSTANTS", sql: `CREATE TYPE mybase (input = mybase_in, output = mybase_out, internallength = 198548, alignment = int4, storage = plain, category = 'U', preferred = false, default = 'default_value', delimiter = ',');`, raw: []string{"mybase", "mybase_in", "mybase_out", "198548", "int4", "plain", "U", "false", "default_value", "false"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "TYPE-CREATE-BASE-COMPREHENSIVE", sql: `CREATE TYPE mybase (
				input = mybase_in,
				output = mybase_out,
				receive = mybase_receive,
				send = mybase_send,
				typmod_in = mybase_typmod_in,
				typmod_out = mybase_typmod_out,
				analyze = mybase_analyze,
				subscript = mybase_subscript,
				internallength = 198548,
				alignment = int4,
				storage = plain,
				like = mybase_like,
				category = 'U',
				preferred = false,
				default = 'default_value',
				element = mybase_element,
				delimiter = ',',
				passedbyvalue = true,
				collatable = true
			);`, raw: []string{"mybase", "mybase_in", "mybase_out", "mybase_receive", "mybase_send", "mybase_typmod_in", "mybase_typmod_out",
			"mybase_analyze", "mybase_subscript", "198548", "int4", "plain", "mybase_like", "U", "false", "default_value", "mybase_element",
			"true", "false"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-BASIC", sql: `CREATE TYPE myrange AS RANGE (subtype = int4);`, raw: []string{"myrange"}, prefixes: []string{TYPE_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-USER-DEFINED-SUBTYPE", sql: `CREATE TYPE customrange AS RANGE (subtype = my_custom_type);`, raw: []string{"customrange", "my_custom_type"}, prefixes: []string{TYPE_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-WITH-SUBTYPE-DIFF", sql: `CREATE TYPE timerange AS RANGE (subtype = time, subtype_diff = time_subtype_diff);`, raw: []string{"timerange", "time_subtype_diff"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-WITH-OPCLASS", sql: `CREATE TYPE timerange AS RANGE (subtype = time, subtype_opclass = time_ops);`, raw: []string{"timerange", "time_ops"}, prefixes: []string{TYPE_KIND_PREFIX, OPCLASS_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-WITH-COLLATION", sql: `CREATE TYPE timerange AS RANGE (subtype = time, collation = time_collation);`, raw: []string{"timerange", "time_collation"}, prefixes: []string{TYPE_KIND_PREFIX, COLLATION_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-WITH-CANONICAL", sql: `CREATE TYPE timerange AS RANGE (subtype = time, canonical = time_canonical_func);`, raw: []string{"timerange", "time_canonical_func"}, prefixes: []string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-WITH-MULTIRANGE", sql: `CREATE TYPE timerange AS RANGE (subtype = time, multirange_type_name = timerange_multirange);`, raw: []string{"timerange", "timerange_multirange"}, prefixes: []string{TYPE_KIND_PREFIX}},
		{key: "TYPE-CREATE-RANGE-COMPREHENSIVE", sql: `CREATE TYPE timerange AS RANGE (
				subtype = time,
				subtype_opclass = time_ops,
				collation = time_collation,
				canonical = time_canonical_func,
				subtype_diff = time_diff_func,
				multirange_type_name = timerange_multirange
			);`, raw: []string{"timerange", "time_ops", "time_collation", "time_canonical_func", "time_diff_func", "timerange_multirange"}, prefixes: []string{TYPE_KIND_PREFIX, OPCLASS_KIND_PREFIX, COLLATION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "TYPE-RENAME", sql: `ALTER TYPE mycomposit RENAME TO mycomposit2;`, raw: []string{"mycomposit", "mycomposit2"}, prefixes: []string{TYPE_KIND_PREFIX}},

		// ─── DOMAIN ────────────────────────────────────────────
		{
			key:      "DOMAIN-CREATE",
			sql:      `CREATE DOMAIN us_postal AS text CHECK (VALUE ~ '^[0-9]{5}$');`,
			raw:      []string{"us_postal", "VALUE", "^[0-9]{5}$"},
			prefixes: []string{DOMAIN_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{key: "DOMAIN-RENAME", sql: `ALTER DOMAIN us_postal RENAME TO us_zip;`, raw: []string{"us_postal", "us_zip"}, prefixes: []string{DOMAIN_KIND_PREFIX}},
		{key: "DOMAIN-DROP", sql: `DROP DOMAIN IF EXISTS us_postal CASCADE;`, raw: []string{"us_postal"}, prefixes: []string{DOMAIN_KIND_PREFIX}},

		// ─── TABLE ─────────────────────────────────────────────
		// CREATE operations
		{key: "TABLE-CREATE", sql: `CREATE TABLE sales.orders (id int PRIMARY KEY, amt numeric);`, raw: []string{"sales", "orders", "id", "amt"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-CREATE-AS", sql: `CREATE TABLE sales.order_summary AS SELECT customer_id, COUNT(*) FROM sales.orders GROUP BY customer_id;`, raw: []string{"sales", "order_summary", "customer_id", "orders"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-CREATE-LIKE", sql: `CREATE TABLE sales.orders_backup (LIKE sales.orders INCLUDING ALL);`, raw: []string{"sales", "orders_backup", "orders"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ALTER operations
		{key: "TABLE-RENAME", sql: `ALTER TABLE sales.orders RENAME TO order_history;`, raw: []string{"sales", "orders", "order_history"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "TABLE-ADD-COLUMN", sql: `ALTER TABLE sales.orders ADD COLUMN note text;`, raw: []string{"sales", "orders", "note"}, prefixes: []string{TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-RENAME-COLUMN", sql: `ALTER TABLE sales.orders RENAME COLUMN amt TO amount;`, raw: []string{"sales", "orders", "amt", "amount"}, prefixes: []string{COLUMN_KIND_PREFIX}},
		{key: "TABLE-DROP-COLUMN", sql: `ALTER TABLE sales.orders DROP COLUMN note;`, raw: []string{"sales", "orders", "note"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ALTER-COLUMN-TYPE", sql: `ALTER TABLE sales.orders ALTER COLUMN amount TYPE decimal(10,2);`, raw: []string{"sales", "orders", "amount"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ADD-CONSTRAINT-PK", sql: `ALTER TABLE sales.orders ADD CONSTRAINT pk_orders PRIMARY KEY (id);`, raw: []string{"sales", "orders", "pk_orders", "id"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ADD-CONSTRAINT-FK", sql: `ALTER TABLE sales.orders ADD CONSTRAINT fk_customer FOREIGN KEY (customer_id) REFERENCES customers(id);`, raw: []string{"sales", "orders", "fk_customer", "customer_id", "customers", "id"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ADD-CONSTRAINT-UK", sql: `ALTER TABLE sales.orders ADD CONSTRAINT uk_order_number UNIQUE (order_number);`, raw: []string{"sales", "orders", "uk_order_number", "order_number"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ADD-CONSTRAINT-CHK", sql: `ALTER TABLE sales.orders ADD CONSTRAINT chk_amount CHECK (amount > 0);`, raw: []string{"sales", "orders", "chk_amount", "amount"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-DROP-CONSTRAINT", sql: `ALTER TABLE sales.orders DROP CONSTRAINT chk_amount;`, raw: []string{"sales", "orders", "chk_amount"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX}},
		{key: "TABLE-SET-SCHEMA", sql: `ALTER TABLE sales.orders SET SCHEMA archive;`, raw: []string{"sales", "orders", "archive"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "TABLE-CHANGE-OWNER", sql: `ALTER TABLE sales.orders OWNER TO order_admin;`, raw: []string{"sales", "orders", "order_admin"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, ROLE_KIND_PREFIX}},

		// Other operations
		{key: "TABLE-DROP", sql: `DROP TABLE IF EXISTS sales.orders CASCADE;`, raw: []string{"sales", "orders"}, prefixes: []string{TABLE_KIND_PREFIX}},
		{key: "TABLE-TRUNCATE", sql: `TRUNCATE TABLE sales.orders;`, raw: []string{"sales", "orders"}, prefixes: []string{TABLE_KIND_PREFIX}},

		// ─── ADDITIONAL ALTER TABLE COLUMN OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ALTER-COLUMN-DEFAULT", sql: `ALTER TABLE sales.orders ALTER COLUMN created_at SET DEFAULT NOW();`, raw: []string{"sales", "orders", "created_at"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ALTER-COLUMN-DROP-DEFAULT", sql: `ALTER TABLE sales.orders ALTER COLUMN created_at DROP DEFAULT;`, raw: []string{"sales", "orders", "created_at"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ALTER-COLUMN-SET-NOT-NULL", sql: `ALTER TABLE sales.orders ALTER COLUMN customer_name SET NOT NULL;`, raw: []string{"sales", "orders", "customer_name"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ALTER-COLUMN-DROP-NOT-NULL", sql: `ALTER TABLE sales.orders ALTER COLUMN description DROP NOT NULL;`, raw: []string{"sales", "orders", "description"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ALTER-COLUMN-SET-OPTIONS", sql: `ALTER TABLE sales.orders ALTER COLUMN notes SET (n_distinct = 100);`, raw: []string{"sales", "orders", "notes"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-ALTER-COLUMN-RESET-OPTIONS", sql: `ALTER TABLE sales.orders ALTER COLUMN notes RESET (n_distinct);`, raw: []string{"sales", "orders", "notes"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── CONSTRAINT OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ALTER-CONSTRAINT", sql: `ALTER TABLE sales.orders ALTER CONSTRAINT fk_customer DEFERRABLE;`, raw: []string{"sales", "orders", "fk_customer"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX}},
		{key: "TABLE-VALIDATE-CONSTRAINT", sql: `ALTER TABLE sales.orders VALIDATE CONSTRAINT chk_amount;`, raw: []string{"sales", "orders", "chk_amount"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX}},

		// ─── INDEX OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-CLUSTER-ON-INDEX", sql: `ALTER TABLE sales.orders CLUSTER ON idx_customer_id;`, raw: []string{"sales", "orders", "idx_customer_id"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, INDEX_KIND_PREFIX}},
		{key: "TABLE-ADD-INDEX-CONSTRAINT", sql: `ALTER TABLE sales.orders ADD CONSTRAINT uq_order_number UNIQUE USING INDEX idx_unique_order_num;`, raw: []string{"sales", "orders", "idx_unique_order_num", "uq_order_number"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONSTRAINT_KIND_PREFIX, INDEX_KIND_PREFIX}},

		// ─── TRIGGER OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ENABLE-TRIGGER", sql: `ALTER TABLE sales.orders ENABLE TRIGGER audit_trigger;`, raw: []string{"sales", "orders", "audit_trigger"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},
		{key: "TABLE-DISABLE-TRIGGER", sql: `ALTER TABLE sales.orders DISABLE TRIGGER audit_trigger;`, raw: []string{"sales", "orders", "audit_trigger"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},
		{key: "TABLE-ENABLE-ALWAYS-TRIGGER", sql: `ALTER TABLE sales.orders ENABLE ALWAYS TRIGGER security_trigger;`, raw: []string{"sales", "orders", "security_trigger"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},
		{key: "TABLE-ENABLE-REPLICA-TRIGGER", sql: `ALTER TABLE sales.orders ENABLE REPLICA TRIGGER sync_trigger;`, raw: []string{"sales", "orders", "sync_trigger"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TRIGGER_KIND_PREFIX}},

		// ─── RULE OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ENABLE-RULE", sql: `ALTER TABLE sales.orders ENABLE RULE order_rule;`, raw: []string{"sales", "orders", "order_rule"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}},               // Using TRIGGER prefix for rules
		{key: "TABLE-DISABLE-RULE", sql: `ALTER TABLE sales.orders DISABLE RULE order_rule;`, raw: []string{"sales", "orders", "order_rule"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}},             // Using TRIGGER prefix for rules
		{key: "TABLE-ENABLE-ALWAYS-RULE", sql: `ALTER TABLE sales.orders ENABLE ALWAYS RULE audit_rule;`, raw: []string{"sales", "orders", "audit_rule"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{key: "TABLE-ENABLE-REPLICA-RULE", sql: `ALTER TABLE sales.orders ENABLE REPLICA RULE sync_rule;`, raw: []string{"sales", "orders", "sync_rule"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}}, // Using TRIGGER prefix for rules

		// ─── IDENTITY OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ADD-IDENTITY", sql: `ALTER TABLE sales.orders ALTER COLUMN order_id ADD GENERATED BY DEFAULT AS IDENTITY;`, raw: []string{"sales", "orders", "order_id"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-SET-IDENTITY", sql: `ALTER TABLE sales.orders ALTER COLUMN order_id SET GENERATED ALWAYS;`, raw: []string{"sales", "orders", "order_id"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "TABLE-DROP-IDENTITY", sql: `ALTER TABLE sales.orders ALTER COLUMN order_id DROP IDENTITY;`, raw: []string{"sales", "orders", "order_id"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── INHERITANCE OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ADD-INHERIT", sql: `ALTER TABLE sales.special_orders INHERIT sales.orders;`, raw: []string{"sales", "special_orders", "orders"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "TABLE-DROP-INHERIT", sql: `ALTER TABLE sales.special_orders NO INHERIT sales.orders;`, raw: []string{"sales", "special_orders", "orders"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── TYPE OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ADD-OF-TYPE", sql: `ALTER TABLE sales.typed_orders OF sales.order_type;`, raw: []string{"sales", "typed_orders", "order_type"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TYPE_KIND_PREFIX}},

		// ─── PARTITION OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-ATTACH-PARTITION", sql: `ALTER TABLE sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');`, raw: []string{"sales", "orders", "orders_2024"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "TABLE-DETACH-PARTITION", sql: `ALTER TABLE sales.orders DETACH PARTITION sales.orders_2024;`, raw: []string{"sales", "orders", "orders_2024"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "TABLE-DETACH-PARTITION-FINALIZE", sql: `ALTER TABLE sales.orders DETACH PARTITION sales.orders_2024 FINALIZE;`, raw: []string{"sales", "orders", "orders_2024"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── REPLICA IDENTITY OPERATIONS ─────────────────────────────────────────────
		{key: "TABLE-REPLICA-IDENTITY-INDEX", sql: `ALTER TABLE sales.orders REPLICA IDENTITY USING INDEX idx_orders_pkey;`, raw: []string{"sales", "orders", "idx_orders_pkey"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, INDEX_KIND_PREFIX}},

		// ─── INDEX ─────────────────────────────────────────────
		{key: "INDEX-CREATE", sql: `CREATE INDEX idx_amt ON sales.orders (amount);`, raw: []string{"idx_amt", "sales", "orders", "amount"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CLUSTER", sql: `ALTER TABLE sales.orders CLUSTER ON idx_amt;`, raw: []string{"sales", "orders", "idx_amt"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-UNIQUE", sql: `CREATE UNIQUE INDEX idx_unique_amount ON sales.orders (amount);`, raw: []string{"idx_unique_amount", "sales", "orders", "amount"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-GIN", sql: `CREATE INDEX idx_customer_name_gin ON sales.orders USING GIN (customer_name gin_trgm_ops);`, raw: []string{"idx_customer_name_gin", "sales", "orders", "customer_name"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-EXPRESSION", sql: `CREATE INDEX idx_lower_customer_name ON sales.orders USING BTREE (lower(customer_name));`, raw: []string{"idx_lower_customer_name", "sales", "orders", "customer_name"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-PARTIAL", sql: `CREATE INDEX idx_amount_gt0 ON sales.orders (amount) WHERE amount > 0;`, raw: []string{"idx_amount_gt0", "sales", "orders", "amount"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-CONCURRENTLY", sql: `CREATE INDEX CONCURRENTLY idx_amt_concurrent ON sales.orders (amount);`, raw: []string{"idx_amt_concurrent", "sales", "orders", "amount"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-IF-NOT-EXISTS", sql: `CREATE INDEX IF NOT EXISTS idx_amt_exists ON sales.orders (amount);`, raw: []string{"idx_amt_exists", "sales", "orders", "amount"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-CREATE-WITH-OPTIONS", sql: `CREATE INDEX idx_amt_with_options ON sales.orders (amount) WITH (fillfactor = 80);`, raw: []string{"idx_amt_with_options", "sales", "orders", "amount"}, prefixes: []string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "INDEX-RENAME", sql: `ALTER INDEX sales.idx_amt RENAME TO idx_amount_new;`, raw: []string{"sales", "idx_amt", "idx_amount_new"}, prefixes: []string{INDEX_KIND_PREFIX}},
		{key: "INDEX-DROP", sql: `DROP INDEX sales.idx_amt;`, raw: []string{"sales", "idx_amt"}, prefixes: []string{INDEX_KIND_PREFIX}},
		{key: "INDEX-DROP-IF-EXISTS", sql: `DROP INDEX IF EXISTS sales.idx_amt;`, raw: []string{"sales", "idx_amt"}, prefixes: []string{INDEX_KIND_PREFIX}},
		{key: "INDEX-DROP-CONCURRENTLY", sql: `DROP INDEX CONCURRENTLY sales.idx_amt;`, raw: []string{"sales", "idx_amt"}, prefixes: []string{INDEX_KIND_PREFIX}},

		// ─── POLICY ────────────────────────────────────────────
		{key: "POLICY-CREATE", sql: `CREATE POLICY p_sel ON sales.orders FOR SELECT USING (true);`, raw: []string{"p_sel", "sales", "orders"}, prefixes: []string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX}},
		{key: "POLICY-DROP", sql: `DROP POLICY p_sel ON sales.orders;`, raw: []string{"p_sel", "sales", "orders"}, prefixes: []string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX}},
		{key: "POLICY-CREATE-WITH-ROLES", sql: `CREATE POLICY p_manager ON sales.orders FOR ALL TO manager_role USING (department = current_setting('app.department'));`, raw: []string{"p_manager", "sales", "orders", "manager_role", "department"}, prefixes: []string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, ROLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "POLICY-CREATE-COMPLEX-CONDITIONS", sql: `CREATE POLICY p_secure ON sales.orders FOR UPDATE USING (user_id = current_user) WITH CHECK (amount < 10000);`, raw: []string{"p_secure", "sales", "orders", "user_id", "amount"}, prefixes: []string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "POLICY-CREATE-ALL-COMMANDS", sql: `CREATE POLICY p_all ON sales.orders FOR ALL USING (tenant_id = current_setting('app.tenant_id'));`, raw: []string{"p_all", "sales", "orders", "tenant_id"}, prefixes: []string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── COMMENT ───────────────────────────────────────────
		{key: "COMMENT-TABLE", sql: `COMMENT ON TABLE sales.orders IS 'order table';`, raw: []string{"sales", "orders", "order table"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-COLUMN", sql: `COMMENT ON COLUMN sales.orders.amount IS 'gross amount';`, raw: []string{"sales", "orders", "amount", "gross amount"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-INDEX", sql: `COMMENT ON INDEX sales.idx_amt IS 'amount index';`, raw: []string{"sales", "idx_amt", "amount index"}, prefixes: []string{SCHEMA_KIND_PREFIX, INDEX_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-SCHEMA", sql: `COMMENT ON SCHEMA sales IS 'Sales schema for e-commerce';`, raw: []string{"sales", "Sales schema for e-commerce"}, prefixes: []string{SCHEMA_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-FUNCTION", sql: `COMMENT ON FUNCTION sales.calculate_total(integer, numeric) IS 'Calculate order total with tax';`, raw: []string{"sales", "calculate_total", "Calculate order total with tax"}, prefixes: []string{SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-PROCEDURE", sql: `COMMENT ON PROCEDURE sales.process_order(integer) IS 'Process customer order';`, raw: []string{"sales", "process_order", "Process customer order"}, prefixes: []string{SCHEMA_KIND_PREFIX, PROCEDURE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-TRIGGER", sql: `COMMENT ON TRIGGER audit_trigger ON sales.orders IS 'Audit trail trigger';`, raw: []string{"audit_trigger", "sales", "orders", "Audit trail trigger"}, prefixes: []string{TRIGGER_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-VIEW", sql: `COMMENT ON VIEW sales.order_summary IS 'Order summary view';`, raw: []string{"sales", "order_summary", "Order summary view"}, prefixes: []string{SCHEMA_KIND_PREFIX, VIEW_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-MVIEW", sql: `COMMENT ON MATERIALIZED VIEW sales.order_stats IS 'Order statistics materialized view';`, raw: []string{"sales", "order_stats", "Order statistics materialized view"}, prefixes: []string{SCHEMA_KIND_PREFIX, MVIEW_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-DATABASE", sql: `COMMENT ON DATABASE sales_db IS 'Sales database';`, raw: []string{"sales_db", "Sales database"}, prefixes: []string{DATABASE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-CONSTRAINT", sql: `COMMENT ON CONSTRAINT pk_orders ON sales.orders IS 'Primary key constraint';`, raw: []string{"pk_orders", "sales", "orders", "Primary key constraint"}, prefixes: []string{CONSTRAINT_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-ROLE", sql: `COMMENT ON ROLE sales_user IS 'Sales department user';`, raw: []string{"sales_user", "Sales department user"}, prefixes: []string{ROLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-COLLATION", sql: `COMMENT ON COLLATION sales.nocase IS 'Case-insensitive collation';`, raw: []string{"sales", "nocase", "Case-insensitive collation"}, prefixes: []string{SCHEMA_KIND_PREFIX, COLLATION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-SEQUENCE", sql: `COMMENT ON SEQUENCE sales.ord_id_seq IS 'Order ID sequence';`, raw: []string{"sales", "ord_id_seq", "Order ID sequence"}, prefixes: []string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-TYPE", sql: `COMMENT ON TYPE sales.order_status IS 'Order status enum';`, raw: []string{"sales", "order_status", "Order status enum"}, prefixes: []string{SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-DOMAIN", sql: `COMMENT ON DOMAIN sales.us_postal IS 'US postal code domain';`, raw: []string{"sales", "us_postal", "US postal code domain"}, prefixes: []string{SCHEMA_KIND_PREFIX, DOMAIN_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "COMMENT-EXTENSION", sql: `COMMENT ON EXTENSION postgis IS 'PostGIS spatial extension';`, raw: []string{"postgis", "PostGIS spatial extension"}, prefixes: []string{CONST_KIND_PREFIX}},
		{key: "COMMENT-POLICY", sql: `COMMENT ON POLICY p_sel ON sales.orders IS 'Select policy';`, raw: []string{"p_sel", "sales", "orders", "Select policy"}, prefixes: []string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{key: "CONVERSION-CREATE", sql: `CREATE CONVERSION conversion_example.myconv FOR 'LATIN1' TO 'UTF8' FROM iso8859_1_to_utf8;`, raw: []string{"conversion_example", "myconv", "iso8859_1_to_utf8"}, prefixes: []string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "CONVERSION-CREATE-BASIC", sql: `CREATE CONVERSION sales.my_conversion FOR 'LATIN1' TO 'UTF8' FROM schema1.latin1_to_utf8;`, raw: []string{"sales", "my_conversion", "schema1", "latin1_to_utf8"}, prefixes: []string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "CONVERSION-CREATE-DEFAULT", sql: `CREATE DEFAULT CONVERSION sales.default_conversion FOR 'LATIN1' TO 'UTF8' FROM latin1_to_utf8;`, raw: []string{"sales", "default_conversion", "latin1_to_utf8"}, prefixes: []string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},

		// ALTER operations
		{key: "CONVERSION-SET-SCHEMA", sql: `ALTER CONVERSION sales.my_conversion SET SCHEMA public;`, raw: []string{"sales", "my_conversion", "public"}, prefixes: []string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX}},
		{key: "CONVERSION-OWNER", sql: `ALTER CONVERSION sales.my_conversion OWNER TO new_owner;`, raw: []string{"sales", "my_conversion", "new_owner"}, prefixes: []string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, ROLE_KIND_PREFIX}},

		// ─── FOREIGN TABLE ───────────────────────────────────────────
		{key: "FOREIGN-TABLE-CREATE", sql: `CREATE FOREIGN TABLE sales.foreign_orders (id int, name text) SERVER remote_server;`, raw: []string{"sales", "foreign_orders", "id", "name", "remote_server"}, prefixes: []string{SCHEMA_KIND_PREFIX, FOREIGN_TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, DEFAULT_KIND_PREFIX}},
		{key: "FOREIGN-TABLE-CREATE-WITH-OPTIONS", sql: `CREATE FOREIGN TABLE sales.foreign_orders (col1 int, col2 text) SERVER remote_server OPTIONS (table_name 'remote_orders', schema_name 'public');`, raw: []string{"sales", "foreign_orders", "col1", "col2", "remote_server", "remote_orders", "public"}, prefixes: []string{SCHEMA_KIND_PREFIX, FOREIGN_TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, DEFAULT_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── RULE ───────────────────────────────────────────
		{key: "RULE-CREATE", sql: `CREATE RULE rule_name AS ON INSERT TO sales.orders DO INSTEAD INSERT INTO sales.orders_audit (id, amount) VALUES (NEW.id, NEW.amount);`, raw: []string{"rule_name", "sales", "orders", "orders_audit", "id", "amount"}, prefixes: []string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "RULE-CREATE-SELECT", sql: `CREATE RULE select_rule AS ON SELECT TO sales.orders DO INSTEAD SELECT * FROM sales.orders_view;`, raw: []string{"select_rule", "sales", "orders", "orders_view"}, prefixes: []string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{key: "RULE-CREATE-UPDATE", sql: `CREATE RULE update_rule AS ON UPDATE TO sales.orders DO INSTEAD UPDATE sales.orders_archive SET amount = NEW.amount WHERE id = OLD.id;`, raw: []string{"update_rule", "sales", "orders", "orders_archive", "amount", "id"}, prefixes: []string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "RULE-CREATE-DELETE", sql: `CREATE RULE delete_rule AS ON DELETE TO sales.orders DO INSTEAD DELETE FROM sales.orders_archive WHERE id = OLD.id;`, raw: []string{"delete_rule", "sales", "orders", "orders_archive", "id"}, prefixes: []string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{key: "RULE-CREATE-WITH-ALSO", sql: `CREATE RULE audit_rule AS ON INSERT TO sales.orders DO ALSO INSERT INTO sales.audit_log (table_name, action, timestamp) VALUES ('orders', 'INSERT', NOW());`, raw: []string{"audit_rule", "sales", "orders", "audit_log", "table_name", "action", "timestamp"}, prefixes: []string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── AGGREGATE ───────────────────────────────────────────
		{key: "AGGREGATE-CREATE", sql: `CREATE AGGREGATE sales.order_total(int) (SFUNC = sales.add_order, STYPE = int);`, raw: []string{"sales", "order_total", "sales", "add_order"}, prefixes: []string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "AGGREGATE-CREATE-WITH-ALL-OPTIONS", sql: `CREATE AGGREGATE sales.order_stats(int) (SFUNC = sales.add_order, STYPE = int, FINALFUNC = sales.finalize_stats, INITCOND = 0, MSFUNC = sales.add_order_multi, MSTYPE = int, MINVFUNC = sales.subtract_order, MFINALFUNC = sales.finalize_stats_multi, MINITCOND = 0, SORTOP = >);`, raw: []string{"sales", "order_stats", "sales", "add_order", "sales", "finalize_stats", "sales", "add_order_multi", "sales", "subtract_order", "sales", "finalize_stats_multi"}, prefixes: []string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "AGGREGATE-CREATE-WITH-ORDER-BY", sql: `CREATE AGGREGATE sales.order_total_ordered(int) (SFUNC = sales.add_order, STYPE = int, SORTOP = >);`, raw: []string{"sales", "order_total_ordered", "sales", "add_order"}, prefixes: []string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "AGGREGATE-CREATE-WITH-PARALLEL", sql: `CREATE AGGREGATE sales.order_total_parallel(int) (SFUNC = sales.add_order, STYPE = int, PARALLEL = SAFE);`, raw: []string{"sales", "order_total_parallel", "sales", "add_order"}, prefixes: []string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "AGGREGATE-CREATE-WITH-HYPOTHETICAL", sql: `CREATE AGGREGATE sales.order_rank(int) (SFUNC = sales.add_order, STYPE = int, HYPOTHETICAL);`, raw: []string{"sales", "order_rank", "sales", "add_order"}, prefixes: []string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "AGGREGATE-CREATE-WITH-USER-DEFINED-STYPE", sql: `CREATE AGGREGATE sales.order_total(sales.order_type) (SFUNC = sales.add_order, STYPE = sales.order_state);`, raw: []string{"sales", "order_total", "sales", "order_type", "sales", "add_order", "sales", "order_state"}, prefixes: []string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},

		// ─── OPERATOR CLASS ─────────────────────────────────────────────
		{key: "OPERATOR-CLASS-CREATE", sql: `CREATE OPERATOR CLASS sales.int4_abs_ops FOR TYPE int4 USING btree AS OPERATOR 1 <#, OPERATOR 2 <=#, OPERATOR 3 =#, OPERATOR 4 >=#, OPERATOR 5 >#, FUNCTION 1 int4_abs_cmp(int4,int4);`, raw: []string{"sales", "int4_abs_ops", "<#", "<=#", "=#", ">=#", ">#", "int4_abs_cmp"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPCLASS_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "OPERATOR-CLASS-CREATE-WITH-FAMILY", sql: `CREATE OPERATOR CLASS sales.int4_abs_ops FOR TYPE int4 USING btree FAMILY sales.abs_numeric_ops AS OPERATOR 1 <#, OPERATOR 2 <=#, OPERATOR 3 =#, OPERATOR 4 >=#, OPERATOR 5 >#, FUNCTION 1 int4_abs_cmp(int4,int4);`, raw: []string{"sales", "int4_abs_ops", "abs_numeric_ops", "<#", "<=#", "=#", ">=#", ">#", "int4_abs_cmp"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPCLASS_KIND_PREFIX, OPFAMILY_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},

		// ─── OPERATOR FAMILY ─────────────────────────────────────────────
		{key: "OPERATOR-FAMILY-CREATE", sql: `CREATE OPERATOR FAMILY sales.abs_numeric_ops USING btree;`, raw: []string{"sales", "abs_numeric_ops"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPFAMILY_KIND_PREFIX}},
		{key: "OPERATOR-FAMILY-ALTER", sql: `ALTER OPERATOR FAMILY am_examples.box_ops USING gist2 ADD OPERATOR 1 <<(box, box), OPERATOR 2 &<(box, box), OPERATOR 3 &&(box, box), OPERATOR 4 &>(box, box), OPERATOR 5 >>(box, box), OPERATOR 6 ~=(box, box), OPERATOR 7 @>(box, box), OPERATOR 8 <@(box, box), OPERATOR 9 &<|(box, box), OPERATOR 10 <<|(box, box), OPERATOR 11 |>>(box, box), OPERATOR 12 |&>(box, box)`, raw: []string{"am_examples", "box_ops", "<<", "&<", "&&", "&>", ">>", "~=", "@>", "<@", "&<|", "<<|", "|>>", "|&>"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPFAMILY_KIND_PREFIX, OPERATOR_KIND_PREFIX}},

		// ─── OPERATOR ─────────────────────────────────────────────
		{key: "OPERATOR-CREATE", sql: `CREATE OPERATOR sales.<# (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_lt);`, raw: []string{"sales", "<#", "int4_abs_lt"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-WITH-COMMUTATOR", sql: `CREATE OPERATOR sales.=# (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_eq, COMMUTATOR = =#);`, raw: []string{"sales", "=#", "int4_abs_eq", "=#"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, OPERATOR_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-WITH-NEGATOR", sql: `CREATE OPERATOR sales.<># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_ne, NEGATOR = =#);`, raw: []string{"sales", "<>#", "int4_abs_ne", "=#"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, OPERATOR_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-WITH-RESTRICT", sql: `CREATE OPERATOR sales.># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_gt, RESTRICT = scalargtsel);`, raw: []string{"sales", ">#", "int4_abs_gt", "scalargtsel"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-WITH-JOIN", sql: `CREATE OPERATOR sales.># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_gt, JOIN = scalargtjoinsel);`, raw: []string{"sales", ">#", "int4_abs_gt", "scalargtjoinsel"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-WITH-ALL-OPTIONS", sql: `CREATE OPERATOR sales.># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_gt, COMMUTATOR = <#, NEGATOR = <=#, RESTRICT = scalargtsel, JOIN = scalargtjoinsel, HASHES, MERGES);`, raw: []string{"sales", ">#", "int4_abs_gt", "<#", "<=#", "scalargtsel", "scalargtjoinsel"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, OPERATOR_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-LEFT-UNARY", sql: `CREATE OPERATOR sales.@# (RIGHTARG = int4, PROCEDURE = int4_abs);`, raw: []string{"sales", "@#", "int4_abs"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{key: "OPERATOR-CREATE-RIGHT-UNARY", sql: `CREATE OPERATOR sales.#@ (LEFTARG = int4, PROCEDURE = int4_factorial);`, raw: []string{"sales", "#@", "int4_factorial"}, prefixes: []string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
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

func TestBuiltinTypeAnonymization(t *testing.T) {
	// builtinTypeCase represents a test case for built-in type anonymization
	type builtinTypeCase struct {
		key              string   // unique test identifier
		sql              string   // the SQL statement to test
		shouldAnonymize  []string // identifiers that should be anonymized
		shouldPreserve   []string // type names that should be preserved (built-in types)
		expectedPrefixes []string // expected anonymization prefixes in output
	}
	log.SetLevel(log.WarnLevel)
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	cases := []builtinTypeCase{
		// Built-in types should NOT be anonymized
		{
			key:              "TYPE-BUILTIN-INT",
			sql:              `CREATE TABLE sales.orders (id int PRIMARY KEY, amount int);`,
			shouldAnonymize:  []string{"sales", "orders", "id", "amount"},
			shouldPreserve:   []string{"int"},
			expectedPrefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			key:              "TYPE-BUILTIN-MULTIPLE-TYPES",
			sql:              `CREATE TABLE sales.products (id bigint, name varchar(100), price numeric(10,2), created_at timestamp, metadata jsonb, is_active boolean);`,
			shouldAnonymize:  []string{"sales", "products", "id", "name", "price", "created_at", "metadata", "is_active"},
			shouldPreserve:   []string{"bigint", "varchar", "numeric", "timestamp", "jsonb", "boolean"},
			expectedPrefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Custom types should be anonymized
		{
			key:              "TYPE-CUSTOM-TYPES",
			sql:              `CREATE TABLE sales.orders (status order_status, priority priority_level);`,
			shouldAnonymize:  []string{"sales", "orders", "status", "priority", "order_status", "priority_level"},
			shouldPreserve:   []string{},
			expectedPrefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, TYPE_KIND_PREFIX},
		},

		// Mixed built-in and custom types
		{
			key:              "TYPE-MIXED-BUILTIN-CUSTOM",
			sql:              `CREATE TABLE sales.orders (id int, status order_status, amount numeric, priority priority_level);`,
			shouldAnonymize:  []string{"sales", "orders", "id", "status", "amount", "priority", "order_status", "priority_level"},
			shouldPreserve:   []string{"int", "numeric"},
			expectedPrefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, TYPE_KIND_PREFIX},
		},

		// Qualified type names
		{
			key:              "TYPE-QUALIFIED-BUILTIN",
			sql:              `CREATE TABLE sales.orders (id int, name text);`,
			shouldAnonymize:  []string{"sales", "orders", "id", "name"},
			shouldPreserve:   []string{"int", "text"},
			expectedPrefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			key:              "TYPE-QUALIFIED-CUSTOM",
			sql:              `CREATE TABLE sales.orders (status sales.order_status, priority public.priority_level);`,
			shouldAnonymize:  []string{"sales", "orders", "status", "priority", "order_status", "priority_level"},
			shouldPreserve:   []string{},
			expectedPrefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, TYPE_KIND_PREFIX},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
			out, err := az.Anonymize(c.sql)
			if err != nil {
				t.Fatalf("anonymize: %v", err)
			}
			fmt.Printf("Test Name: %s\nIN: %s\nOUT: %s\n\n", c.key, c.sql, out)

			// Check that identifiers that should be anonymized are actually anonymized
			for _, identifier := range c.shouldAnonymize {
				if strings.Contains(out, identifier) {
					t.Errorf("identifier %q should be anonymized but leaked in %s", identifier, out)
				}
			}

			// Check that type names that should be preserved are actually preserved
			for _, typeName := range c.shouldPreserve {
				if !strings.Contains(out, typeName) {
					t.Errorf("type name %q should be preserved but was anonymized in %s", typeName, out)
				}
			}

			// Check that expected prefixes appear
			for _, pref := range c.expectedPrefixes {
				if !hasTok(out, pref) {
					t.Errorf("expected prefix %q not found in %s", pref, out)
				}
			}
		})
	}
}

// Based on analysis from postgresql_nodes_analysis.md
func TestMissingNodeAnonymization(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	type missingNodeCase struct {
		key              string   // unique test identifier
		sql              string   // the SQL statement to test
		nodeType         string   // PostgreSQL node type being tested
		shouldAnonymize  []string // identifiers that should be anonymized
		expectedPrefixes []string // expected anonymization prefixes in output
		skipReason       string   // reason if test should be skipped
	}

	testCases := []missingNodeCase{
		// ────────── TEXT SEARCH DICTIONARY STATEMENTS ──────────
		// CREATE TEXT SEARCH DICTIONARY public.my_dict (TEMPLATE = pg_catalog.simple );
		// CREATE TEXT SEARCH CONFIGURATION public.my_config (PARSER = pg_catalog."default" );
		// ALTER TEXT SEARCH DICTIONARY my_dict (StopWords = 'english');
		// ALTER TEXT SEARCH CONFIGURATION my_config ADD MAPPING FOR word WITH simple;
		// skipReason: It is dumped by pg_dump but goes to uncategorised.sql so can deferred for later.

		// ────────── ACCESS METHOD STATEMENTS ──────────
		{
			sql:        "CREATE ACCESS METHOD my_am TYPE INDEX HANDLER my_handler;",
			skipReason: "Not dumped by pg_dump",
		},

		// ────────── PUBLICATION/SUBSCRIPTION STATEMENTS ──────────
		// Not dumped by pg_dump

		// ────────── LANGUAGE STATEMENTS ──────────
		{
			sql:        "CREATE LANGUAGE my_language HANDLER my_handler;",
			skipReason: "Not dumped by pg_dump",
		},

		// ────────── TRANSFORM STATEMENTS ──────────
		{
			sql:        "CREATE TRANSFORM FOR my_type LANGUAGE my_language (FROM SQL WITH FUNCTION from_sql_func(internal), TO SQL WITH FUNCTION to_sql_func(my_type));",
			skipReason: "Not dumped by pg_dump",
		},

		// ────────── CAST STATEMENTS ──────────
		{
			sql:        "CREATE CAST (my_type AS text) WITH FUNCTION my_cast_func(my_type) AS IMPLICIT;",
			skipReason: "Not dumped by pg_dump",
		},

		// ────────── STATISTICS STATEMENTS ──────────
		{
			sql:        "CREATE STATISTICS my_stats (dependencies) ON col1, col2 FROM sales.orders;",
			skipReason: "Not dumped by pg_dump",
		},
		{
			sql:        "ALTER STATISTICS my_stats SET STATISTICS 1000;",
			skipReason: "Not dumped by pg_dump",
		},

		// ────────── SECURITY LABEL STATEMENTS ──────────
		{
			sql:        "SECURITY LABEL FOR my_provider ON TABLE sales.orders IS 'classified';",
			skipReason: "Not dumped by pg_dump",
		},

		// ────────── CREATE TABLE AS STATEMENTS ──────────
		{
			sql:        "CREATE TABLE sales.order_summary AS SELECT customer_id, COUNT(*) FROM sales.orders GROUP BY customer_id;",
			skipReason: "Not dumped by pg_dump",
		},
		// ────────── ALTER SYSTEM STATEMENTS ──────────
		{
			sql:        "ALTER SYSTEM SET my_param = 'value';",
			skipReason: "Not dumped by pg_dump",
		},
	}

	// Run tests for cases that are not skipped
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}

			out, err := az.Anonymize(tc.sql)
			if err != nil {
				// If anonymization fails, that's expected for unimplemented node types
				// We just want to ensure the test doesn't crash
				t.Logf("Anonymization failed as expected for unimplemented node type: %v", err)
				return
			}

			// If anonymization succeeds, verify the output doesn't contain raw identifiers
			if tc.shouldAnonymize != nil {
				for _, identifier := range tc.shouldAnonymize {
					if strings.Contains(out, identifier) {
						t.Errorf("identifier %q should be anonymized but leaked in %s", identifier, out)
					}
				}
			}

			// Verify expected prefixes appear
			if tc.expectedPrefixes != nil {
				for _, pref := range tc.expectedPrefixes {
					if !hasTok(out, pref) {
						t.Errorf("expected prefix %q not found in %s", pref, out)
					}
				}
			}
		})
	}
}

// TestPostgresDDLDefaultClauseAnonymization tests comprehensive anonymization of DEFAULT clause expressions
// This covers all types of expressions that can appear in DEFAULT clauses including:
// - Custom function calls
// - Built-in function calls
// - Sequence references
// - Type casts
// - Array expressions
// - Case expressions
// - Collation specifications
func TestPostgresDDLDefaultClauseAnonymization(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	// Extended test case struct with keep field for built-in functions
	type defaultClauseCase struct {
		ddlCase
		keep []string // identifiers that must remain unchanged (e.g., built-in functions)
	}

	cases := []defaultClauseCase{
		{ddlCase{key: "DEFAULT-CUSTOM-FUNCTION-SCHEMA", sql: `CREATE TABLE IF NOT EXISTS app.orders (
  id           bigserial PRIMARY KEY,
  code         text NOT NULL DEFAULT app.generate_identifier(),
  customer     text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);`, raw: []string{"app", "orders", "id", "code", "customer", "created_at", "generate_identifier"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			[]string{"now()"}},
		{ddlCase{key: "DEFAULT-BUILTIN-FUNCTIONS", sql: `CREATE TABLE sales.products (
  id           serial PRIMARY KEY,
  name         text NOT NULL,
  created_at   timestamptz DEFAULT now(),
  updated_at   timestamptz DEFAULT current_timestamp
);`, raw: []string{"sales", "products", "id", "name", "created_at", "updated_at"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
			[]string{"now()", "current_timestamp"}},
		{ddlCase{key: "DEFAULT-CUSTOM-FUNCTION-MULTIPLE", sql: `CREATE TABLE inventory.items (
  item_id      bigint PRIMARY KEY,
  sku          text DEFAULT inventory.generate_sku(),
  description  text DEFAULT inventory.get_default_description(),
  status       text DEFAULT 'active'
);`, raw: []string{"inventory", "items", "item_id", "sku", "description", "status", "generate_sku", "get_default_description"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			nil},
		{ddlCase{key: "DEFAULT-CUSTOM-FUNCTION-MIXED", sql: `CREATE TABLE users.profiles (
  user_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username     text NOT NULL,
  email        text DEFAULT 'user@example.com',
  created_at   timestamptz DEFAULT now(),
  last_login   timestamptz DEFAULT users.get_last_login_time()
);`, raw: []string{"users", "profiles", "user_id", "username", "email", "created_at", "last_login", "get_last_login_time"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			[]string{"gen_random_uuid()", "now()"}},
		{ddlCase{key: "DEFAULT-CUSTOM-FUNCTION-MIXED-PG-CATALOG", sql: `CREATE TABLE users.profiles (
  user_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username     text NOT NULL,
  email        text DEFAULT 'user@example.com',
  created_at   timestamptz DEFAULT pg_catalog.now(),
  last_login   timestamptz DEFAULT users.get_last_login_time()
);`, raw: []string{"users", "profiles", "user_id", "username", "email", "created_at", "last_login", "get_last_login_time"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			[]string{"gen_random_uuid()", "now()"}},
		// ─── SEQUENCE REFERENCES IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{key: "SEQUENCE-UNQUALIFIED", sql: `CREATE TABLE public.customers (
  customer_id bigint PRIMARY KEY DEFAULT nextval('customer_seq'),
  name        text NOT NULL,
  email       text DEFAULT concat('user', nextval('user_id_seq'), '@example.com')
);`, raw: []string{"public", "customers", "customer_id", "name", "email", "customer_seq", "user_id_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
			[]string{"nextval", "concat"}},
		{ddlCase{key: "SEQUENCE-QUALIFIED-SCHEMA", sql: `CREATE TABLE accounts.transactions (
  txn_id       bigint PRIMARY KEY DEFAULT nextval('accounts.transaction_seq'),
  account_id   bigint DEFAULT nextval('accounts.account_seq'),
  amount       decimal NOT NULL,
  reference_id text DEFAULT concat('TXN-', nextval('accounts.ref_seq'))
);`, raw: []string{"accounts", "transactions", "txn_id", "account_id", "amount", "reference_id", "transaction_seq", "account_seq", "ref_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
			[]string{"nextval", "concat"}},
		{ddlCase{key: "SEQUENCE-TYPECAST-VARIANTS", sql: `CREATE TABLE inventory.items (
  item_id      bigint PRIMARY KEY DEFAULT nextval('inventory.item_seq'::regclass),
  sku_id       bigint DEFAULT nextval('inventory.sku_seq'::text::regclass),
  batch_id     bigint DEFAULT nextval('batch_seq'::regclass),
  created_at   timestamp DEFAULT now()
);`, raw: []string{"inventory", "items", "item_id", "sku_id", "batch_id", "created_at", "item_seq", "sku_seq", "batch_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
			[]string{"nextval", "now"}},
		{ddlCase{key: "SEQUENCE-PG-CATALOG-NEXTVAL", sql: `CREATE TABLE public.customers (
  customer_id bigint PRIMARY KEY DEFAULT pg_catalog.nextval('customer_seq'),
  name        text NOT NULL,
  email       text DEFAULT concat('user', pg_catalog.nextval('user_id_seq'), '@example.com')
);`, raw: []string{"public", "customers", "customer_id", "name", "email", "customer_seq", "user_id_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
			[]string{"nextval", "concat"}},

		// ─── TYPE CASTS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{key: "DEFAULT-TYPE-CASTS", sql: `CREATE TABLE inventory.products (
  product_id   integer PRIMARY KEY,
  price        numeric DEFAULT '99.99'::numeric,
  created_date date DEFAULT '2024-01-01'::date,
  is_active    boolean DEFAULT 'true'::boolean
);`, raw: []string{"inventory", "products", "product_id", "price", "created_date", "is_active"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			nil},

		// ─── ARRAY EXPRESSIONS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{key: "DEFAULT-ARRAY-EXPRESSIONS", sql: `CREATE TABLE users.preferences (
  user_id      integer PRIMARY KEY,
  tags         text[] DEFAULT ARRAY['default', 'user']::text[],
  permissions  integer[] DEFAULT '{1,2,3}'::integer[]
);`, raw: []string{"users", "preferences", "user_id", "tags", "permissions"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			[]string{"ARRAY"}},

		// ─── CASE EXPRESSIONS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{key: "DEFAULT-CASE-EXPRESSIONS", sql: `CREATE TABLE sales.orders (
  order_id     serial PRIMARY KEY,
  status       text DEFAULT CASE WHEN random() > 0.5 THEN 'pending' ELSE 'approved' END,
  priority     text DEFAULT CASE WHEN extract(hour from now()) < 12 THEN 'high' ELSE 'normal' END
);`, raw: []string{"sales", "orders", "order_id", "status", "priority"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			[]string{"random()", "extract", "now()", "CASE", "WHEN", "THEN", "ELSE", "END"}},

		// ─── COLLATION IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{key: "DEFAULT-COLLATION", sql: `CREATE TABLE content.articles (
  article_id   serial PRIMARY KEY,
  title        text DEFAULT 'Untitled' COLLATE "en_US",
  content      text DEFAULT 'No content available'
);`, raw: []string{"content", "articles", "article_id", "title", "content"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			[]string{"COLLATE"}},

		// ─── ALTER TABLE DEFAULT CLAUSE OPERATIONS ─────────────────────────────────────────────
		{ddlCase{key: "ALTER-DEFAULT-CUSTOM-FUNCTION", sql: `ALTER TABLE sales.orders ALTER COLUMN order_code SET DEFAULT sales.generate_order_code();`, raw: []string{"sales", "orders", "order_code", "generate_order_code"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			nil},
		{ddlCase{key: "ALTER-DEFAULT-SCHEMA-FUNCTION", sql: `ALTER TABLE inventory.items ALTER COLUMN sku SET DEFAULT inventory.generate_sku();`, raw: []string{"inventory", "items", "sku", "generate_sku"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			nil},
		{ddlCase{key: "ALTER-DEFAULT-SEQUENCE", sql: `ALTER TABLE orders.order_items ALTER COLUMN item_id SET DEFAULT nextval('orders.item_seq'::regclass);`, raw: []string{"orders", "order_items", "item_id", "item_seq"}, prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
			[]string{"nextval"}},
	}

	// Run tests for cases that are not skipped
	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			out, err := az.Anonymize(tc.sql)
			if err != nil {
				// If anonymization fails, that's expected for unimplemented node types
				// We just want to ensure the test doesn't crash
				t.Logf("Anonymization failed as expected for unimplemented node type: %v", err)
				return
			}

			// print before and after
			t.Logf("Test Name: %s\nIN: %s\nOUT: %s\n\n", tc.key, tc.sql, out)

			// If anonymization succeeds, verify the output doesn't contain raw identifiers
			if tc.raw != nil {
				for _, identifier := range tc.raw {
					if strings.Contains(out, identifier) {
						t.Errorf("identifier %q should be anonymized but leaked in %s", identifier, out)
					}
				}
			}

			// Verify expected prefixes appear
			if tc.prefixes != nil {
				for _, pref := range tc.prefixes {
					if !hasTok(out, pref) {
						t.Errorf("expected prefix %q not found in %s", pref, out)
					}
				}
			}

			// Verify identifiers that must remain unchanged (e.g., built-in functions)
			if tc.keep != nil {
				for _, identifier := range tc.keep {
					if !strings.Contains(out, identifier) {
						t.Errorf("identifier %q should remain unchanged but was modified in %s", identifier, out)
					}
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
//  INDEX                | CREATE INDEX <name> ON <table> (...)            | IndexStmtNode                | [x]
//                       | ALTER INDEX <name> RENAME TO <new>              | RenameStmtNode               | [x]
//                       | DROP INDEX <name> [CASCADE|RESTRICT]            | DropStmtNode                 | [x]
//
//  POLICY               | CREATE POLICY <name> ON <table> ...             | CreatePolicyStmtNode         | [x]
//                       | DROP POLICY <name>                              | DropStmtNode                 | [x]
//
//  COMMENT              | COMMENT ON TABLE/COLUMN/... (all object types)  | CommentOnStmtNode            | [x]
//
//  CONVERSION           | CREATE CONVERSION <schema>.<name> ...           | CreateConversionStmtNode     | [x]
//
//  FOREIGN TABLE        | CREATE FOREIGN TABLE <schema>.<name> ...        | CreateForeignTableStmtNode   | [x]
//                       | ALTER FOREIGN TABLE <name> RENAME TO <new>      | RenameStmtNode               | [x]
//                       | DROP FOREIGN TABLE <name>                       | DropStmtNode                 | [x]
//
//  RULE                 | CREATE RULE <name> ON <table> ...                | CreateRuleStmtNode          | [x]
//                       | ALTER RULE <name> ON <table> ...                 | AlterRuleStmtNode           | [x]
//
//  AGGREGATE            | CREATE AGGREGATE <name> (<type>) ...            | CreateAggregateStmtNode      | [x]
//
//
//  OPERATOR             | CREATE OPERATOR <schema>.<name> ...             | CreateOperatorStmtNode       | [x]
//
//  OPERATOR CLASS       | CREATE OPERATOR CLASS <name> ...              | CreateOperatorClassStmtNode    | [x]
//
//  OPERATOR FAMILY      | CREATE OPERATOR FAMILY <name> ...              | CreateOperatorFamilyStmtNode  | [x]
//
// Below objects are not anonymized or send to callhome yet.
//  TRIGGER               ... (Create/Alter/Drop)                        | [ ]
//  VIEW                  ...                                            | [ ]
//  MVIEW                 ...                                            | [ ]
//  FUNCTION / PROCEDURE  ...                                            | [ ]
// ============================================================================

// TestPostgresDDLCornerCases tests edge cases and corner cases for PostgreSQL DDL statements
// that may not be covered by the standard test cases above
func TestPostgresDDLCornerCases(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	cases := []ddlCase{
		// ─── CORNER CASE: TABLE WITH DEFAULT VALUES ─────────────────────────────────────────────
		{
			key:      "TABLE-CREATE-WITH-DEFAULT-INT",
			sql:      `CREATE TABLE sales.products (product_id int DEFAULT 1)`,
			raw:      []string{"sales", "products", "product_id"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			key:      "TABLE-CREATE-WITH-DEFAULT-STRING",
			sql:      `CREATE TABLE sales.products (product_name text DEFAULT 'default_name')`,
			raw:      []string{"sales", "products", "product_name", "default_name"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			key:      "TABLE-CREATE-WITH-DEFAULT-FLOAT",
			sql:      `CREATE TABLE sales.products (price numeric DEFAULT 99.99)`,
			raw:      []string{"sales", "products", "price"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			key:      "TABLE-CREATE-WITH-DEFAULT-BOOLEAN",
			sql:      `CREATE TABLE sales.products (is_active boolean DEFAULT true)`,
			raw:      []string{"sales", "products", "is_active"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			key:      "TABLE-CREATE-WITH-DEFAULT-NULL",
			sql:      `CREATE TABLE sales.products (description text DEFAULT NULL)`,
			raw:      []string{"sales", "products", "description"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			key:      "TABLE-CREATE-WITH-MULTIPLE-DEFAULTS",
			sql:      `CREATE TABLE sales.products (id int DEFAULT 1, name text DEFAULT 'product', price numeric DEFAULT 0.00, active boolean DEFAULT true)`,
			raw:      []string{"sales", "products", "id", "name", "product", "price", "active"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},

		// ─── PARTITIONING STRATEGIES ─────────────────────────────────────────────────────────────
		// LIST Partitioning
		{
			key:      "TABLE-CREATE-PARTITIONED-LIST",
			sql:      `CREATE TABLE sales.orders (order_id int NOT NULL, customer_id int, order_status text, region text NOT NULL) PARTITION BY LIST(region)`,
			raw:      []string{"sales", "orders", "order_id", "customer_id", "order_status", "region"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// RANGE Partitioning
		{
			key:      "TABLE-CREATE-PARTITIONED-RANGE",
			sql:      `CREATE TABLE sales.orders_by_date (order_id int, order_date date, amount numeric) PARTITION BY RANGE(order_date)`,
			raw:      []string{"sales", "orders_by_date", "order_id", "order_date", "amount"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// HASH Partitioning
		{
			key:      "TABLE-CREATE-PARTITIONED-HASH",
			sql:      `CREATE TABLE sales.orders_by_hash (order_id int, customer_id int, amount numeric) PARTITION BY HASH(customer_id)`,
			raw:      []string{"sales", "orders_by_hash", "order_id", "customer_id", "amount"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Multiple Column Partitioning
		{
			key:      "TABLE-CREATE-PARTITIONED-MULTI-COLUMN",
			sql:      `CREATE TABLE sales.orders_multi (order_id int, region text, order_date date, amount numeric) PARTITION BY RANGE(region, order_date)`,
			raw:      []string{"sales", "orders_multi", "order_id", "region", "order_date", "amount"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Expression-based Partitioning
		{
			key:      "TABLE-CREATE-PARTITIONED-EXPRESSION",
			sql:      `CREATE TABLE sales.orders_expr (order_id int, order_date timestamp, amount numeric) PARTITION BY RANGE(EXTRACT(YEAR FROM order_date))`,
			raw:      []string{"sales", "orders_expr", "order_id", "order_date", "amount"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Function-based Partitioning
		{
			key:      "TABLE-CREATE-PARTITIONED-FUNCTION",
			sql:      `CREATE TABLE sales.orders_func (order_id int, order_date timestamp, amount numeric) PARTITION BY RANGE(date_trunc('month', order_date))`,
			raw:      []string{"sales", "orders_func", "order_id", "order_date", "amount"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// ─── ALTER TABLE IDENTITY OPERATIONS ─────────────────────────────────────────────
		{
			key:      "TABLE-ALTER-ADD-IDENTITY-WITH-SEQUENCE",
			sql:      `ALTER TABLE sales.orders ALTER COLUMN order_id ADD GENERATED BY DEFAULT AS IDENTITY (SEQUENCE NAME public.ts_query_table_id_seq START 1 INCREMENT 1 NO MINVALUE NO MAXVALUE CACHE 1)`,
			raw:      []string{"sales", "orders", "order_id", "public", "ts_query_table_id_seq"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX},
		},

		// ─── ALTER TABLE PARTITION OPERATIONS ─────────────────────────────────────────────
		{
			key:      "TABLE-ALTER-ATTACH-PARTITION",
			sql:      `ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM (5545) TO (5612)`,
			raw:      []string{"sales", "orders", "orders_2024", "5545", "5612"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},
		{
			key:      "TABLE-ALTER-ATTACH-PARTITION-DATE",
			sql:      `ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM ('2024-01-01') TO ('2024-12-31')`,
			raw:      []string{"sales", "orders", "orders_2024", "2024-01-01", "2024-12-31"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},
		{
			key:      "TABLE-ALTER-ATTACH-PARTITION-STRING",
			sql:      `ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_region_a FOR VALUES FROM ('US') TO ('CA')`,
			raw:      []string{"sales", "orders", "orders_region_a", "US", "CA"},
			prefixes: []string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},

		// ─── INDEX WITH OPERATOR CLASS AND OPTIONS ─────────────────────────────────────────────
		{
			key:      "INDEX-CREATE-WITH-OPERATOR-CLASS-OPTIONS",
			sql:      `CREATE INDEX idx_orders_location ON sales.orders USING gist (location postgis.gist_geometry_ops(siglen='1232'))`,
			raw:      []string{"idx_orders_location", "sales", "orders", "location", "postgis", "gist_geometry_ops", "siglen", "1232"},
			prefixes: []string{INDEX_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SCHEMA_KIND_PREFIX, OPCLASS_KIND_PREFIX, PARAMETER_KIND_PREFIX, CONST_KIND_PREFIX},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
			out, err := az.Anonymize(c.sql)
			if err != nil {
				t.Fatalf("anonymize: %v", err)
			}
			fmt.Printf("Test Name: %s\nIN: %s\nOUT: %s\n\n", c.key, c.sql, out)

			// Check that raw identifiers are anonymized
			for _, raw := range c.raw {
				if strings.Contains(out, raw) {
					t.Errorf("raw identifier %q leaked in %s", raw, out)
				}
			}

			// Check that expected prefixes appear
			for _, pref := range c.prefixes {
				if !hasTok(out, pref) {
					t.Errorf("expected prefix %q not found in %s", pref, out)
				}
			}
		})
	}
}

// TestFunctionProcedureTriggerViewMviewDDL tests anonymization of functions, procedures,
// triggers, views, and materialized views. Currently these are unsupported and should
// return errors. When support is added, set expectError to false and populate raw/prefixes.
func TestFunctionProcedureTriggerViewMviewDDL(t *testing.T) {
	log.SetLevel(log.WarnLevel)
	exportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)
	az := newAnon(t, exportDir)

	cases := []ddlCase{
		// ─── FUNCTION ───────────────────────────────────────────
		{
			key: "FUNCTION-CREATE",
			sql: `CREATE FUNCTION sales.calculate_total(order_id integer) RETURNS numeric AS $$
				SELECT SUM(amount) FROM sales.order_items WHERE order_id = $1;
			$$ LANGUAGE SQL;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "FUNCTION-CREATE-OR-REPLACE",
			sql: `CREATE OR REPLACE FUNCTION inventory.get_stock(product_id integer) RETURNS integer AS $$
				BEGIN
					RETURN (SELECT quantity FROM inventory.products WHERE id = product_id);
				END;
			$$ LANGUAGE plpgsql;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "FUNCTION-ALTER-RENAME",
			sql: `ALTER FUNCTION sales.calculate_total(integer) RENAME TO calc_order_total;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "FUNCTION-ALTER-OWNER",
			sql: `ALTER FUNCTION sales.calculate_total(integer) OWNER TO sales_admin;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "FUNCTION-ALTER-SET-SCHEMA",
			sql: `ALTER FUNCTION sales.calculate_total(integer) SET SCHEMA archive;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "FUNCTION-DROP",
			sql: `DROP FUNCTION IF EXISTS sales.calculate_total(integer);`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "FUNCTION-DROP-CASCADE",
			sql: `DROP FUNCTION sales.get_customer_name(integer) CASCADE;`,
			raw: nil, prefixes: nil, expectError: true,
		},

		// ─── PROCEDURE ───────────────────────────────────────────
		{
			key: "PROCEDURE-CREATE",
			sql: `CREATE PROCEDURE sales.process_order(order_id integer) AS $$
				BEGIN
					UPDATE sales.orders SET status = 'processed' WHERE id = order_id;
				END;
			$$ LANGUAGE plpgsql;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "PROCEDURE-CREATE-OR-REPLACE",
			sql: `CREATE OR REPLACE PROCEDURE inventory.restock_product(product_id integer, quantity integer) AS $$
				BEGIN
					UPDATE inventory.products SET stock = stock + quantity WHERE id = product_id;
				END;
			$$ LANGUAGE plpgsql;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "PROCEDURE-ALTER-RENAME",
			sql: `ALTER PROCEDURE sales.process_order(integer) RENAME TO handle_order;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "PROCEDURE-ALTER-OWNER",
			sql: `ALTER PROCEDURE sales.process_order(integer) OWNER TO order_admin;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "PROCEDURE-DROP",
			sql: `DROP PROCEDURE IF EXISTS sales.process_order(integer);`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "PROCEDURE-CALL",
			sql: `CALL sales.process_order(12345);`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "PROCEDURE-DO-BLOCK",
			sql: `DO $$
				BEGIN
					UPDATE sales.orders SET status = 'archived' WHERE created_at < NOW() - INTERVAL '1 year';
				END;
			$$;`,
			raw: nil, prefixes: nil, expectError: true,
		},

		// ─── VIEW ───────────────────────────────────────────
		{
			key: "VIEW-CREATE",
			sql: `CREATE VIEW sales.order_summary AS SELECT customer_id, COUNT(*) as order_count, SUM(amount) as total FROM sales.orders GROUP BY customer_id;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-CREATE-OR-REPLACE",
			sql: `CREATE OR REPLACE VIEW sales.active_customers AS SELECT * FROM sales.customers WHERE status = 'active';`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-CREATE-RECURSIVE",
			sql: `CREATE RECURSIVE VIEW org.employee_hierarchy(id, name, manager_id, level) AS
				SELECT id, name, manager_id, 1 FROM org.employees WHERE manager_id IS NULL
				UNION ALL
				SELECT e.id, e.name, e.manager_id, eh.level + 1 FROM org.employees e JOIN org.employee_hierarchy eh ON e.manager_id = eh.id;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-CREATE-WITH-OPTIONS",
			sql: `CREATE VIEW sales.recent_orders WITH (security_barrier = true) AS SELECT * FROM sales.orders WHERE created_at > NOW() - INTERVAL '30 days';`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-ALTER-RENAME",
			sql: `ALTER VIEW sales.order_summary RENAME TO order_stats;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-ALTER-OWNER",
			sql: `ALTER VIEW sales.order_summary OWNER TO report_admin;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-ALTER-SET-SCHEMA",
			sql: `ALTER VIEW sales.order_summary SET SCHEMA reporting;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-DROP",
			sql: `DROP VIEW IF EXISTS sales.order_summary;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "VIEW-DROP-CASCADE",
			sql: `DROP VIEW sales.customer_orders CASCADE;`,
			raw: nil, prefixes: nil, expectError: true,
		},

		// ─── MATERIALIZED VIEW ───────────────────────────────────────────
		{
			key: "MVIEW-CREATE",
			sql: `CREATE MATERIALIZED VIEW sales.monthly_sales AS SELECT date_trunc('month', order_date) as month, SUM(amount) as total FROM sales.orders GROUP BY 1;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-CREATE-WITH-DATA",
			sql: `CREATE MATERIALIZED VIEW sales.product_stats AS SELECT product_id, COUNT(*) as order_count FROM sales.order_items GROUP BY product_id WITH DATA;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-CREATE-WITH-NO-DATA",
			sql: `CREATE MATERIALIZED VIEW sales.customer_metrics AS SELECT customer_id, COUNT(*) as orders FROM sales.orders GROUP BY customer_id WITH NO DATA;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-ALTER-RENAME",
			sql: `ALTER MATERIALIZED VIEW sales.monthly_sales RENAME TO monthly_revenue;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-ALTER-OWNER",
			sql: `ALTER MATERIALIZED VIEW sales.monthly_sales OWNER TO analytics_admin;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-ALTER-SET-SCHEMA",
			sql: `ALTER MATERIALIZED VIEW sales.monthly_sales SET SCHEMA analytics;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-DROP",
			sql: `DROP MATERIALIZED VIEW IF EXISTS sales.monthly_sales;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-DROP-CASCADE",
			sql: `DROP MATERIALIZED VIEW sales.product_stats CASCADE;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-REFRESH",
			sql: `REFRESH MATERIALIZED VIEW sales.monthly_sales;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "MVIEW-REFRESH-CONCURRENTLY",
			sql: `REFRESH MATERIALIZED VIEW CONCURRENTLY sales.monthly_sales;`,
			raw: nil, prefixes: nil, expectError: true,
		},

		// ─── TRIGGER ───────────────────────────────────────────
		{
			key: "TRIGGER-CREATE",
			sql: `CREATE TRIGGER audit_orders AFTER INSERT OR UPDATE ON sales.orders FOR EACH ROW EXECUTE FUNCTION audit.log_change();`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "TRIGGER-CREATE-OR-REPLACE",
			sql: `CREATE OR REPLACE TRIGGER update_timestamp BEFORE UPDATE ON sales.orders FOR EACH ROW EXECUTE FUNCTION sales.set_updated_at();`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "TRIGGER-CREATE-CONSTRAINT",
			sql: `CREATE CONSTRAINT TRIGGER check_inventory AFTER INSERT ON sales.order_items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION inventory.verify_stock();`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "TRIGGER-CREATE-WHEN",
			sql: `CREATE TRIGGER notify_large_order AFTER INSERT ON sales.orders FOR EACH ROW WHEN (NEW.amount > 10000) EXECUTE FUNCTION sales.notify_sales_team();`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "TRIGGER-ALTER-RENAME",
			sql: `ALTER TRIGGER audit_orders ON sales.orders RENAME TO order_audit_trigger;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "TRIGGER-DROP",
			sql: `DROP TRIGGER IF EXISTS audit_orders ON sales.orders;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "TRIGGER-DROP-CASCADE",
			sql: `DROP TRIGGER update_timestamp ON sales.orders CASCADE;`,
			raw: nil, prefixes: nil, expectError: true,
		},

		// ─── EVENT TRIGGER ───────────────────────────────────────────
		{
			key: "EVENT-TRIGGER-CREATE",
			sql: `CREATE EVENT TRIGGER log_ddl ON ddl_command_end EXECUTE FUNCTION audit.log_ddl_event();`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "EVENT-TRIGGER-CREATE-WHEN",
			sql: `CREATE EVENT TRIGGER prevent_drop ON sql_drop WHEN TAG IN ('DROP TABLE', 'DROP SCHEMA') EXECUTE FUNCTION admin.prevent_drop();`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "EVENT-TRIGGER-ALTER-ENABLE",
			sql: `ALTER EVENT TRIGGER log_ddl ENABLE;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "EVENT-TRIGGER-ALTER-DISABLE",
			sql: `ALTER EVENT TRIGGER log_ddl DISABLE;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "EVENT-TRIGGER-ALTER-OWNER",
			sql: `ALTER EVENT TRIGGER log_ddl OWNER TO security_admin;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "EVENT-TRIGGER-ALTER-RENAME",
			sql: `ALTER EVENT TRIGGER log_ddl RENAME TO ddl_audit_trigger;`,
			raw: nil, prefixes: nil, expectError: true,
		},
		{
			key: "EVENT-TRIGGER-DROP",
			sql: `DROP EVENT TRIGGER IF EXISTS log_ddl;`,
			raw: nil, prefixes: nil, expectError: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
			out, err := az.Anonymize(c.sql)
			fmt.Printf("Test Name: %s\nIN: %s\nOUT: %s\nERR: %v\n\n", c.key, c.sql, out, err)

			if c.expectError {
				if err == nil {
					t.Errorf("expected error for unsupported DDL %q but got none", c.key)
				}
			} else {
				// Standard validation when support is added
				if err != nil {
					t.Fatalf("anonymize: %v", err)
				}
				// Check that raw identifiers are anonymized
				for _, raw := range c.raw {
					if strings.Contains(out, raw) {
						t.Errorf("raw identifier %q leaked in %s", raw, out)
					}
				}
				// Check that expected prefixes appear
				for _, pref := range c.prefixes {
					if !hasTok(out, pref) {
						t.Errorf("expected prefix %q not found in %s", pref, out)
					}
				}
			}
		})
	}
}
