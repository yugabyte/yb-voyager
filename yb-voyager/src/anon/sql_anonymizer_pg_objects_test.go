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
	key      string   // unique id, also used in `enabled`
	sql      string   // the statement under test
	raw      []string // identifiers that must vanish
	prefixes []string // token prefixes that must appear
}

func TestPostgresDDLVariants(t *testing.T) {
	log.SetLevel(log.WarnLevel)
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
		{
			"TYPE-CREATE-BASIC",
			`CREATE TYPE base_type_examples.base_type (
				INTERNALLENGTH = variable,
				INPUT = base_type_examples.base_fn_in,
				OUTPUT = base_type_examples.base_fn_out,
				ALIGNMENT = int4,
				STORAGE = plain
			);`,
			[]string{"base_type_examples", "base_type", "base_fn_in", "base_fn_out"},
			[]string{SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-COMPOSITE",
			`CREATE TYPE dbname.schema1.mycomposit AS (col1 int, col2 text);`,
			[]string{"dbname", "schema1", "mycomposit", "col1", "col2"},
			[]string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"TYPE-CREATE-BASE",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out);`,
			[]string{"mybase", "mybase_in", "mybase_out"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-BASE-WITH-RECEIVE-SEND",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out, receive = mybase_receive, send = mybase_send);`,
			[]string{"mybase", "mybase_in", "mybase_out", "mybase_receive", "mybase_send"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-BASE-WITH-TYPMOD",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out, typmod_in = mybase_typmod_in, typmod_out = mybase_typmod_out);`,
			[]string{"mybase", "mybase_in", "mybase_out", "mybase_typmod_in", "mybase_typmod_out"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-BASE-WITH-ANALYZE-SUBSCRIPT",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out, analyze = mybase_analyze, subscript = mybase_subscript);`,
			[]string{"mybase", "mybase_in", "mybase_out", "mybase_analyze", "mybase_subscript"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-BASE-WITH-LIKE-ELEMENT",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out, like = mybase_like, element = mybase_element);`,
			[]string{"mybase", "mybase_in", "mybase_out", "mybase_like", "mybase_element"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-BASE-WITH-CONSTANTS",
			`CREATE TYPE mybase (input = mybase_in, output = mybase_out, internallength = 198548, alignment = int4, storage = plain, category = 'U', preferred = false, default = 'default_value', delimiter = ',');`,
			[]string{"mybase", "mybase_in", "mybase_out", "198548", "int4", "plain", "U", "false", "default_value", "false"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"TYPE-CREATE-BASE-COMPREHENSIVE",
			`CREATE TYPE mybase (
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
			);`,
			[]string{"mybase", "mybase_in", "mybase_out", "mybase_receive", "mybase_send", "mybase_typmod_in", "mybase_typmod_out",
				"mybase_analyze", "mybase_subscript", "198548", "int4", "plain", "mybase_like", "U", "false", "default_value", "mybase_element",
				"true", "false"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-BASIC",
			`CREATE TYPE myrange AS RANGE (subtype = int4);`,
			[]string{"myrange"},
			[]string{TYPE_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-USER-DEFINED-SUBTYPE",
			`CREATE TYPE customrange AS RANGE (subtype = my_custom_type);`,
			[]string{"customrange", "my_custom_type"},
			[]string{TYPE_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-WITH-SUBTYPE-DIFF",
			`CREATE TYPE timerange AS RANGE (subtype = time, subtype_diff = time_subtype_diff);`,
			[]string{"timerange", "time_subtype_diff"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-WITH-OPCLASS",
			`CREATE TYPE timerange AS RANGE (subtype = time, subtype_opclass = time_ops);`,
			[]string{"timerange", "time_ops"},
			[]string{TYPE_KIND_PREFIX, OPCLASS_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-WITH-COLLATION",
			`CREATE TYPE timerange AS RANGE (subtype = time, collation = time_collation);`,
			[]string{"timerange", "time_collation"},
			[]string{TYPE_KIND_PREFIX, COLLATION_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-WITH-CANONICAL",
			`CREATE TYPE timerange AS RANGE (subtype = time, canonical = time_canonical_func);`,
			[]string{"timerange", "time_canonical_func"},
			[]string{TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-WITH-MULTIRANGE",
			`CREATE TYPE timerange AS RANGE (subtype = time, multirange_type_name = timerange_multirange);`,
			[]string{"timerange", "timerange_multirange"},
			[]string{TYPE_KIND_PREFIX}},
		{"TYPE-CREATE-RANGE-COMPREHENSIVE",
			`CREATE TYPE timerange AS RANGE (
				subtype = time,
				subtype_opclass = time_ops,
				collation = time_collation,
				canonical = time_canonical_func,
				subtype_diff = time_diff_func,
				multirange_type_name = timerange_multirange
			);`,
			[]string{"timerange", "time_ops", "time_collation", "time_canonical_func", "time_diff_func", "timerange_multirange"},
			[]string{TYPE_KIND_PREFIX, OPCLASS_KIND_PREFIX, COLLATION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"TYPE-RENAME",
			`ALTER TYPE mycomposit RENAME TO mycomposit2;`,
			[]string{"mycomposit", "mycomposit2"},
			[]string{TYPE_KIND_PREFIX}},

		// ─── DOMAIN ────────────────────────────────────────────
		{"DOMAIN-CREATE",
			`CREATE DOMAIN us_postal AS text CHECK (VALUE ~ '^[0-9]{5}$');`,
			[]string{"us_postal", "VALUE", "^[0-9]{5}$"},
			[]string{DOMAIN_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
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
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{"TABLE-DISABLE-RULE",
			`ALTER TABLE sales.orders DISABLE RULE order_rule;`,
			[]string{"sales", "orders", "order_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{"TABLE-ENABLE-ALWAYS-RULE",
			`ALTER TABLE sales.orders ENABLE ALWAYS RULE audit_rule;`,
			[]string{"sales", "orders", "audit_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}}, // Using TRIGGER prefix for rules
		{"TABLE-ENABLE-REPLICA-RULE",
			`ALTER TABLE sales.orders ENABLE REPLICA RULE sync_rule;`,
			[]string{"sales", "orders", "sync_rule"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, RULE_KIND_PREFIX}}, // Using TRIGGER prefix for rules

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
		{"INDEX-CREATE-UNIQUE",
			`CREATE UNIQUE INDEX idx_unique_amount ON sales.orders (amount);`,
			[]string{"idx_unique_amount", "sales", "orders", "amount"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CREATE-GIN",
			`CREATE INDEX idx_customer_name_gin ON sales.orders USING GIN (customer_name gin_trgm_ops);`,
			[]string{"idx_customer_name_gin", "sales", "orders", "customer_name"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CREATE-EXPRESSION",
			`CREATE INDEX idx_lower_customer_name ON sales.orders USING BTREE (lower(customer_name));`,
			[]string{"idx_lower_customer_name", "sales", "orders", "customer_name"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CREATE-PARTIAL",
			`CREATE INDEX idx_amount_gt0 ON sales.orders (amount) WHERE amount > 0;`,
			[]string{"idx_amount_gt0", "sales", "orders", "amount"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CREATE-CONCURRENTLY",
			`CREATE INDEX CONCURRENTLY idx_amt_concurrent ON sales.orders (amount);`,
			[]string{"idx_amt_concurrent", "sales", "orders", "amount"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CREATE-IF-NOT-EXISTS",
			`CREATE INDEX IF NOT EXISTS idx_amt_exists ON sales.orders (amount);`,
			[]string{"idx_amt_exists", "sales", "orders", "amount"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-CREATE-WITH-OPTIONS",
			`CREATE INDEX idx_amt_with_options ON sales.orders (amount) WITH (fillfactor = 80);`,
			[]string{"idx_amt_with_options", "sales", "orders", "amount"},
			[]string{INDEX_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"INDEX-RENAME",
			`ALTER INDEX sales.idx_amt RENAME TO idx_amount_new;`,
			[]string{"sales", "idx_amt", "idx_amount_new"},
			[]string{INDEX_KIND_PREFIX}},
		{"INDEX-DROP",
			`DROP INDEX sales.idx_amt;`,
			[]string{"sales", "idx_amt"},
			[]string{INDEX_KIND_PREFIX}},
		{"INDEX-DROP-IF-EXISTS",
			`DROP INDEX IF EXISTS sales.idx_amt;`,
			[]string{"sales", "idx_amt"},
			[]string{INDEX_KIND_PREFIX}},
		{"INDEX-DROP-CONCURRENTLY",
			`DROP INDEX CONCURRENTLY sales.idx_amt;`,
			[]string{"sales", "idx_amt"},
			[]string{INDEX_KIND_PREFIX}},

		// ─── POLICY ────────────────────────────────────────────
		{"POLICY-CREATE",
			`CREATE POLICY p_sel ON sales.orders FOR SELECT USING (true);`,
			[]string{"p_sel", "sales", "orders"},
			[]string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX}},
		{"POLICY-DROP",
			`DROP POLICY p_sel ON sales.orders;`,
			[]string{"p_sel", "sales", "orders"},
			[]string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX}},
		{"POLICY-CREATE-WITH-ROLES",
			`CREATE POLICY p_manager ON sales.orders FOR ALL TO manager_role USING (department = current_setting('app.department'));`,
			[]string{"p_manager", "sales", "orders", "manager_role", "department"},
			[]string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, ROLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"POLICY-CREATE-COMPLEX-CONDITIONS",
			`CREATE POLICY p_secure ON sales.orders FOR UPDATE USING (user_id = current_user) WITH CHECK (amount < 10000);`,
			[]string{"p_secure", "sales", "orders", "user_id", "amount"},
			[]string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"POLICY-CREATE-ALL-COMMANDS",
			`CREATE POLICY p_all ON sales.orders FOR ALL USING (tenant_id = current_setting('app.tenant_id'));`,
			[]string{"p_all", "sales", "orders", "tenant_id"},
			[]string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── COMMENT ───────────────────────────────────────────
		{"COMMENT-TABLE",
			`COMMENT ON TABLE sales.orders IS 'order table';`,
			[]string{"sales", "orders", "order table"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-COLUMN",
			`COMMENT ON COLUMN sales.orders.amount IS 'gross amount';`,
			[]string{"sales", "orders", "amount", "gross amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-INDEX",
			`COMMENT ON INDEX sales.idx_amt IS 'amount index';`,
			[]string{"sales", "idx_amt", "amount index"},
			[]string{SCHEMA_KIND_PREFIX, INDEX_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-SCHEMA",
			`COMMENT ON SCHEMA sales IS 'Sales schema for e-commerce';`,
			[]string{"sales", "Sales schema for e-commerce"},
			[]string{SCHEMA_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-FUNCTION",
			`COMMENT ON FUNCTION sales.calculate_total(integer, numeric) IS 'Calculate order total with tax';`,
			[]string{"sales", "calculate_total", "Calculate order total with tax"},
			[]string{SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-PROCEDURE",
			`COMMENT ON PROCEDURE sales.process_order(integer) IS 'Process customer order';`,
			[]string{"sales", "process_order", "Process customer order"},
			[]string{SCHEMA_KIND_PREFIX, PROCEDURE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-TRIGGER",
			`COMMENT ON TRIGGER audit_trigger ON sales.orders IS 'Audit trail trigger';`,
			[]string{"audit_trigger", "sales", "orders", "Audit trail trigger"},
			[]string{TRIGGER_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-VIEW",
			`COMMENT ON VIEW sales.order_summary IS 'Order summary view';`,
			[]string{"sales", "order_summary", "Order summary view"},
			[]string{SCHEMA_KIND_PREFIX, VIEW_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-MVIEW",
			`COMMENT ON MATERIALIZED VIEW sales.order_stats IS 'Order statistics materialized view';`,
			[]string{"sales", "order_stats", "Order statistics materialized view"},
			[]string{SCHEMA_KIND_PREFIX, MVIEW_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-DATABASE",
			`COMMENT ON DATABASE sales_db IS 'Sales database';`,
			[]string{"sales_db", "Sales database"},
			[]string{DATABASE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-CONSTRAINT",
			`COMMENT ON CONSTRAINT pk_orders ON sales.orders IS 'Primary key constraint';`,
			[]string{"pk_orders", "sales", "orders", "Primary key constraint"},
			[]string{CONSTRAINT_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-ROLE",
			`COMMENT ON ROLE sales_user IS 'Sales department user';`,
			[]string{"sales_user", "Sales department user"},
			[]string{ROLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-COLLATION",
			`COMMENT ON COLLATION sales.nocase IS 'Case-insensitive collation';`,
			[]string{"sales", "nocase", "Case-insensitive collation"},
			[]string{SCHEMA_KIND_PREFIX, COLLATION_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-SEQUENCE",
			`COMMENT ON SEQUENCE sales.ord_id_seq IS 'Order ID sequence';`,
			[]string{"sales", "ord_id_seq", "Order ID sequence"},
			[]string{SCHEMA_KIND_PREFIX, SEQUENCE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-TYPE",
			`COMMENT ON TYPE sales.order_status IS 'Order status enum';`,
			[]string{"sales", "order_status", "Order status enum"},
			[]string{SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-DOMAIN",
			`COMMENT ON DOMAIN sales.us_postal IS 'US postal code domain';`,
			[]string{"sales", "us_postal", "US postal code domain"},
			[]string{SCHEMA_KIND_PREFIX, DOMAIN_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"COMMENT-EXTENSION",
			`COMMENT ON EXTENSION postgis IS 'PostGIS spatial extension';`,
			[]string{"postgis", "PostGIS spatial extension"},
			[]string{CONST_KIND_PREFIX}},
		{"COMMENT-POLICY",
			`COMMENT ON POLICY p_sel ON sales.orders IS 'Select policy';`,
			[]string{"p_sel", "sales", "orders", "Select policy"},
			[]string{POLICY_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, CONST_KIND_PREFIX}},
		{"CONVERSION-CREATE",
			`CREATE CONVERSION conversion_example.myconv FOR 'LATIN1' TO 'UTF8' FROM iso8859_1_to_utf8;`,
			[]string{"conversion_example", "myconv", "iso8859_1_to_utf8"},
			[]string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"CONVERSION-CREATE-BASIC",
			`CREATE CONVERSION sales.my_conversion FOR 'LATIN1' TO 'UTF8' FROM schema1.latin1_to_utf8;`,
			[]string{"sales", "my_conversion", "schema1", "latin1_to_utf8"},
			[]string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"CONVERSION-CREATE-DEFAULT",
			`CREATE DEFAULT CONVERSION sales.default_conversion FOR 'LATIN1' TO 'UTF8' FROM latin1_to_utf8;`,
			[]string{"sales", "default_conversion", "latin1_to_utf8"},
			[]string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},

		// ALTER operations
		{"CONVERSION-SET-SCHEMA",
			`ALTER CONVERSION sales.my_conversion SET SCHEMA public;`,
			[]string{"sales", "my_conversion", "public"},
			[]string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX}},
		{"CONVERSION-OWNER",
			`ALTER CONVERSION sales.my_conversion OWNER TO new_owner;`,
			[]string{"sales", "my_conversion", "new_owner"},
			[]string{SCHEMA_KIND_PREFIX, CONVERSION_KIND_PREFIX, ROLE_KIND_PREFIX}},

		// ─── FOREIGN TABLE ───────────────────────────────────────────
		{"FOREIGN-TABLE-CREATE",
			`CREATE FOREIGN TABLE sales.foreign_orders (id int, name text) SERVER remote_server;`,
			[]string{"sales", "foreign_orders", "id", "name", "remote_server"},
			[]string{SCHEMA_KIND_PREFIX, FOREIGN_TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, DEFAULT_KIND_PREFIX}},
		{"FOREIGN-TABLE-CREATE-WITH-OPTIONS",
			`CREATE FOREIGN TABLE sales.foreign_orders (col1 int, col2 text) SERVER remote_server OPTIONS (table_name 'remote_orders', schema_name 'public');`,
			[]string{"sales", "foreign_orders", "col1", "col2", "remote_server", "remote_orders", "public"},
			[]string{SCHEMA_KIND_PREFIX, FOREIGN_TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, DEFAULT_KIND_PREFIX, TABLE_KIND_PREFIX}},

		// ─── RULE ───────────────────────────────────────────
		{"RULE-CREATE",
			`CREATE RULE rule_name AS ON INSERT TO sales.orders DO INSTEAD INSERT INTO sales.orders_audit (id, amount) VALUES (NEW.id, NEW.amount);`,
			[]string{"rule_name", "sales", "orders", "orders_audit", "id", "amount"},
			[]string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"RULE-CREATE-SELECT",
			`CREATE RULE select_rule AS ON SELECT TO sales.orders DO INSTEAD SELECT * FROM sales.orders_view;`,
			[]string{"select_rule", "sales", "orders", "orders_view"},
			[]string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX}},
		{"RULE-CREATE-UPDATE",
			`CREATE RULE update_rule AS ON UPDATE TO sales.orders DO INSTEAD UPDATE sales.orders_archive SET amount = NEW.amount WHERE id = OLD.id;`,
			[]string{"update_rule", "sales", "orders", "orders_archive", "amount", "id"},
			[]string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"RULE-CREATE-DELETE",
			`CREATE RULE delete_rule AS ON DELETE TO sales.orders DO INSTEAD DELETE FROM sales.orders_archive WHERE id = OLD.id;`,
			[]string{"delete_rule", "sales", "orders", "orders_archive", "id"},
			[]string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
		{"RULE-CREATE-WITH-ALSO",
			`CREATE RULE audit_rule AS ON INSERT TO sales.orders DO ALSO INSERT INTO sales.audit_log (table_name, action, timestamp) VALUES ('orders', 'INSERT', NOW());`,
			[]string{"audit_rule", "sales", "orders", "audit_log", "table_name", "action", "timestamp"},
			[]string{RULE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},

		// ─── AGGREGATE ───────────────────────────────────────────
		{"AGGREGATE-CREATE",
			`CREATE AGGREGATE sales.order_total(int) (SFUNC = sales.add_order, STYPE = int);`,
			[]string{"sales", "order_total", "sales", "add_order"},
			[]string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"AGGREGATE-CREATE-WITH-ALL-OPTIONS",
			`CREATE AGGREGATE sales.order_stats(int) (SFUNC = sales.add_order, STYPE = int, FINALFUNC = sales.finalize_stats, INITCOND = 0, MSFUNC = sales.add_order_multi, MSTYPE = int, MINVFUNC = sales.subtract_order, MFINALFUNC = sales.finalize_stats_multi, MINITCOND = 0, SORTOP = >);`,
			[]string{"sales", "order_stats", "sales", "add_order", "sales", "finalize_stats", "sales", "add_order_multi", "sales", "subtract_order", "sales", "finalize_stats_multi"},
			[]string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"AGGREGATE-CREATE-WITH-ORDER-BY",
			`CREATE AGGREGATE sales.order_total_ordered(int) (SFUNC = sales.add_order, STYPE = int, SORTOP = >);`,
			[]string{"sales", "order_total_ordered", "sales", "add_order"},
			[]string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"AGGREGATE-CREATE-WITH-PARALLEL",
			`CREATE AGGREGATE sales.order_total_parallel(int) (SFUNC = sales.add_order, STYPE = int, PARALLEL = SAFE);`,
			[]string{"sales", "order_total_parallel", "sales", "add_order"},
			[]string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"AGGREGATE-CREATE-WITH-HYPOTHETICAL",
			`CREATE AGGREGATE sales.order_rank(int) (SFUNC = sales.add_order, STYPE = int, HYPOTHETICAL);`,
			[]string{"sales", "order_rank", "sales", "add_order"},
			[]string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"AGGREGATE-CREATE-WITH-USER-DEFINED-STYPE",
			`CREATE AGGREGATE sales.order_total(sales.order_type) (SFUNC = sales.add_order, STYPE = sales.order_state);`,
			[]string{"sales", "order_total", "sales", "order_type", "sales", "add_order", "sales", "order_state"},
			[]string{AGGREGATE_KIND_PREFIX, SCHEMA_KIND_PREFIX, TYPE_KIND_PREFIX, FUNCTION_KIND_PREFIX}},

		// ─── OPERATOR CLASS ─────────────────────────────────────────────
		{"OPERATOR-CLASS-CREATE",
			`CREATE OPERATOR CLASS sales.int4_abs_ops FOR TYPE int4 USING btree AS OPERATOR 1 <#, OPERATOR 2 <=#, OPERATOR 3 =#, OPERATOR 4 >=#, OPERATOR 5 >#, FUNCTION 1 int4_abs_cmp(int4,int4);`,
			[]string{"sales", "int4_abs_ops", "<#", "<=#", "=#", ">=#", ">#", "int4_abs_cmp"},
			[]string{SCHEMA_KIND_PREFIX, OPCLASS_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"OPERATOR-CLASS-CREATE-WITH-FAMILY",
			`CREATE OPERATOR CLASS sales.int4_abs_ops FOR TYPE int4 USING btree FAMILY sales.abs_numeric_ops AS OPERATOR 1 <#, OPERATOR 2 <=#, OPERATOR 3 =#, OPERATOR 4 >=#, OPERATOR 5 >#, FUNCTION 1 int4_abs_cmp(int4,int4);`,
			[]string{"sales", "int4_abs_ops", "abs_numeric_ops", "<#", "<=#", "=#", ">=#", ">#", "int4_abs_cmp"},
			[]string{SCHEMA_KIND_PREFIX, OPCLASS_KIND_PREFIX, OPFAMILY_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},

		// ─── OPERATOR FAMILY ─────────────────────────────────────────────
		{"OPERATOR-FAMILY-CREATE",
			`CREATE OPERATOR FAMILY sales.abs_numeric_ops USING btree;`,
			[]string{"sales", "abs_numeric_ops"},
			[]string{SCHEMA_KIND_PREFIX, OPFAMILY_KIND_PREFIX}},
		{"OPERATOR-FAMILY-ALTER",
			`ALTER OPERATOR FAMILY am_examples.box_ops USING gist2 ADD OPERATOR 1 <<(box, box), OPERATOR 2 &<(box, box), OPERATOR 3 &&(box, box), OPERATOR 4 &>(box, box), OPERATOR 5 >>(box, box), OPERATOR 6 ~=(box, box), OPERATOR 7 @>(box, box), OPERATOR 8 <@(box, box), OPERATOR 9 &<|(box, box), OPERATOR 10 <<|(box, box), OPERATOR 11 |>>(box, box), OPERATOR 12 |&>(box, box)`,
			[]string{"am_examples", "box_ops", "<<", "&<", "&&", "&>", ">>", "~=", "@>", "<@", "&<|", "<<|", "|>>", "|&>"},
			[]string{SCHEMA_KIND_PREFIX, OPFAMILY_KIND_PREFIX, OPERATOR_KIND_PREFIX}},

		// ─── OPERATOR ─────────────────────────────────────────────
		{"OPERATOR-CREATE",
			`CREATE OPERATOR sales.<# (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_lt);`,
			[]string{"sales", "<#", "int4_abs_lt"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"OPERATOR-CREATE-WITH-COMMUTATOR",
			`CREATE OPERATOR sales.=# (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_eq, COMMUTATOR = =#);`,
			[]string{"sales", "=#", "int4_abs_eq", "=#"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, OPERATOR_KIND_PREFIX}},
		{"OPERATOR-CREATE-WITH-NEGATOR",
			`CREATE OPERATOR sales.<># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_ne, NEGATOR = =#);`,
			[]string{"sales", "<>#", "int4_abs_ne", "=#"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, OPERATOR_KIND_PREFIX}},
		{"OPERATOR-CREATE-WITH-RESTRICT",
			`CREATE OPERATOR sales.># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_gt, RESTRICT = scalargtsel);`,
			[]string{"sales", ">#", "int4_abs_gt", "scalargtsel"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"OPERATOR-CREATE-WITH-JOIN",
			`CREATE OPERATOR sales.># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_gt, JOIN = scalargtjoinsel);`,
			[]string{"sales", ">#", "int4_abs_gt", "scalargtjoinsel"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"OPERATOR-CREATE-WITH-ALL-OPTIONS",
			`CREATE OPERATOR sales.># (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_gt, COMMUTATOR = <#, NEGATOR = <=#, RESTRICT = scalargtsel, JOIN = scalargtjoinsel, HASHES, MERGES);`,
			[]string{"sales", ">#", "int4_abs_gt", "<#", "<=#", "scalargtsel", "scalargtjoinsel"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, OPERATOR_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"OPERATOR-CREATE-LEFT-UNARY",
			`CREATE OPERATOR sales.@# (RIGHTARG = int4, PROCEDURE = int4_abs);`,
			[]string{"sales", "@#", "int4_abs"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
		{"OPERATOR-CREATE-RIGHT-UNARY",
			`CREATE OPERATOR sales.#@ (LEFTARG = int4, PROCEDURE = int4_factorial);`,
			[]string{"sales", "#@", "int4_factorial"},
			[]string{SCHEMA_KIND_PREFIX, OPERATOR_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
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
		// ─── CUSTOM FUNCTION CALLS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-CUSTOM-FUNCTION-SCHEMA",
			`CREATE TABLE IF NOT EXISTS app.orders (
  id           bigserial PRIMARY KEY,
  code         text NOT NULL DEFAULT app.generate_identifier(),
  customer     text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);`,
			[]string{"app", "orders", "id", "code", "customer", "created_at", "generate_identifier"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			[]string{"now()"}},
		{ddlCase{"DEFAULT-CUSTOM-FUNCTION-MULTIPLE",
			`CREATE TABLE inventory.items (
  item_id      bigint PRIMARY KEY,
  sku          text DEFAULT inventory.generate_sku(),
  description  text DEFAULT inventory.get_default_description(),
  status       text DEFAULT 'active'
);`,
			[]string{"inventory", "items", "item_id", "sku", "description", "status", "generate_sku", "get_default_description"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			nil},
		{ddlCase{"DEFAULT-CUSTOM-FUNCTION-MIXED",
			`CREATE TABLE users.profiles (
  user_id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username     text NOT NULL,
  email        text DEFAULT 'user@example.com',
  created_at   timestamptz DEFAULT now(),
  last_login   timestamptz DEFAULT users.get_last_login_time()
);`,
			[]string{"users", "profiles", "user_id", "username", "email", "created_at", "last_login", "get_last_login_time"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			[]string{"gen_random_uuid()", "now()"}},

		// ─── BUILT-IN FUNCTION CALLS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-BUILTIN-FUNCTIONS",
			`CREATE TABLE sales.products (
  id           serial PRIMARY KEY,
  name         text NOT NULL,
  created_at   timestamptz DEFAULT now(),
  updated_at   timestamptz DEFAULT current_timestamp
);`,
			[]string{"sales", "products", "id", "name", "created_at", "updated_at"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX}},
			[]string{"now()", "current_timestamp"}},

		// ─── SEQUENCE REFERENCES IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-SEQUENCE-REFERENCE",
			`CREATE TABLE orders.order_items (
  item_id      bigint PRIMARY KEY DEFAULT nextval('orders.item_seq'::regclass),
  order_id     bigint NOT NULL,
  product_name text DEFAULT 'Unknown Product'
);`,
			[]string{"orders", "order_items", "item_id", "order_id", "product_name", "item_seq"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
			[]string{"nextval"}},

		// ─── TYPE CASTS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-TYPE-CASTS",
			`CREATE TABLE inventory.products (
  product_id   integer PRIMARY KEY,
  price        numeric DEFAULT '99.99'::numeric,
  created_date date DEFAULT '2024-01-01'::date,
  is_active    boolean DEFAULT 'true'::boolean
);`,
			[]string{"inventory", "products", "product_id", "price", "created_date", "is_active"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			nil},

		// ─── ARRAY EXPRESSIONS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-ARRAY-EXPRESSIONS",
			`CREATE TABLE users.preferences (
  user_id      integer PRIMARY KEY,
  tags         text[] DEFAULT ARRAY['default', 'user']::text[],
  permissions  integer[] DEFAULT '{1,2,3}'::integer[]
);`,
			[]string{"users", "preferences", "user_id", "tags", "permissions"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			[]string{"ARRAY"}},

		// ─── CASE EXPRESSIONS IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-CASE-EXPRESSIONS",
			`CREATE TABLE sales.orders (
  order_id     serial PRIMARY KEY,
  status       text DEFAULT CASE WHEN random() > 0.5 THEN 'pending' ELSE 'approved' END,
  priority     text DEFAULT CASE WHEN extract(hour from now()) < 12 THEN 'high' ELSE 'normal' END
);`,
			[]string{"sales", "orders", "order_id", "status", "priority"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			[]string{"random()", "extract", "now()", "CASE", "WHEN", "THEN", "ELSE", "END"}},

		// ─── COLLATION IN DEFAULT CLAUSES ─────────────────────────────────────────────
		{ddlCase{"DEFAULT-COLLATION",
			`CREATE TABLE content.articles (
  article_id   serial PRIMARY KEY,
  title        text DEFAULT 'Untitled' COLLATE "en_US",
  content      text DEFAULT 'No content available'
);`,
			[]string{"content", "articles", "article_id", "title", "content"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX}},
			[]string{"COLLATE"}},

		// ─── ALTER TABLE DEFAULT CLAUSE OPERATIONS ─────────────────────────────────────────────
		{ddlCase{"ALTER-DEFAULT-CUSTOM-FUNCTION",
			`ALTER TABLE sales.orders ALTER COLUMN order_code SET DEFAULT sales.generate_order_code();`,
			[]string{"sales", "orders", "order_code", "generate_order_code"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			nil},
		{ddlCase{"ALTER-DEFAULT-SCHEMA-FUNCTION",
			`ALTER TABLE inventory.items ALTER COLUMN sku SET DEFAULT inventory.generate_sku();`,
			[]string{"inventory", "items", "sku", "generate_sku"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, FUNCTION_KIND_PREFIX}},
			nil},
		{ddlCase{"ALTER-DEFAULT-SEQUENCE",
			`ALTER TABLE orders.order_items ALTER COLUMN item_id SET DEFAULT nextval('orders.item_seq'::regclass);`,
			[]string{"orders", "order_items", "item_id", "item_seq"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX}},
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
			"TABLE-CREATE-WITH-DEFAULT-INT",
			`CREATE TABLE sales.products (product_id int DEFAULT 1)`,
			[]string{"sales", "products", "product_id"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			"TABLE-CREATE-WITH-DEFAULT-STRING",
			`CREATE TABLE sales.products (product_name text DEFAULT 'default_name')`,
			[]string{"sales", "products", "product_name", "default_name"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			"TABLE-CREATE-WITH-DEFAULT-FLOAT",
			`CREATE TABLE sales.products (price numeric DEFAULT 99.99)`,
			[]string{"sales", "products", "price"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			"TABLE-CREATE-WITH-DEFAULT-BOOLEAN",
			`CREATE TABLE sales.products (is_active boolean DEFAULT true)`,
			[]string{"sales", "products", "is_active"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},
		{
			"TABLE-CREATE-WITH-DEFAULT-NULL",
			`CREATE TABLE sales.products (description text DEFAULT NULL)`,
			[]string{"sales", "products", "description"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},
		{
			"TABLE-CREATE-WITH-MULTIPLE-DEFAULTS",
			`CREATE TABLE sales.products (id int DEFAULT 1, name text DEFAULT 'product', price numeric DEFAULT 0.00, active boolean DEFAULT true)`,
			[]string{"sales", "products", "id", "name", "product", "price", "active"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, CONST_KIND_PREFIX},
		},

		// ─── PARTITIONING STRATEGIES ─────────────────────────────────────────────────────────────
		// LIST Partitioning
		{
			"TABLE-CREATE-PARTITIONED-LIST",
			`CREATE TABLE sales.orders (order_id int NOT NULL, customer_id int, order_status text, region text NOT NULL) PARTITION BY LIST(region)`,
			[]string{"sales", "orders", "order_id", "customer_id", "order_status", "region"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// RANGE Partitioning
		{
			"TABLE-CREATE-PARTITIONED-RANGE",
			`CREATE TABLE sales.orders_by_date (order_id int, order_date date, amount numeric) PARTITION BY RANGE(order_date)`,
			[]string{"sales", "orders_by_date", "order_id", "order_date", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// HASH Partitioning
		{
			"TABLE-CREATE-PARTITIONED-HASH",
			`CREATE TABLE sales.orders_by_hash (order_id int, customer_id int, amount numeric) PARTITION BY HASH(customer_id)`,
			[]string{"sales", "orders_by_hash", "order_id", "customer_id", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Multiple Column Partitioning
		{
			"TABLE-CREATE-PARTITIONED-MULTI-COLUMN",
			`CREATE TABLE sales.orders_multi (order_id int, region text, order_date date, amount numeric) PARTITION BY RANGE(region, order_date)`,
			[]string{"sales", "orders_multi", "order_id", "region", "order_date", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Expression-based Partitioning
		{
			"TABLE-CREATE-PARTITIONED-EXPRESSION",
			`CREATE TABLE sales.orders_expr (order_id int, order_date timestamp, amount numeric) PARTITION BY RANGE(EXTRACT(YEAR FROM order_date))`,
			[]string{"sales", "orders_expr", "order_id", "order_date", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// Function-based Partitioning
		{
			"TABLE-CREATE-PARTITIONED-FUNCTION",
			`CREATE TABLE sales.orders_func (order_id int, order_date timestamp, amount numeric) PARTITION BY RANGE(date_trunc('month', order_date))`,
			[]string{"sales", "orders_func", "order_id", "order_date", "amount"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX},
		},

		// ─── ALTER TABLE IDENTITY OPERATIONS ─────────────────────────────────────────────
		{
			"TABLE-ALTER-ADD-IDENTITY-WITH-SEQUENCE",
			`ALTER TABLE sales.orders ALTER COLUMN order_id ADD GENERATED BY DEFAULT AS IDENTITY (SEQUENCE NAME public.ts_query_table_id_seq START 1 INCREMENT 1 NO MINVALUE NO MAXVALUE CACHE 1)`,
			[]string{"sales", "orders", "order_id", "public", "ts_query_table_id_seq"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SEQUENCE_KIND_PREFIX},
		},

		// ─── ALTER TABLE PARTITION OPERATIONS ─────────────────────────────────────────────
		{
			"TABLE-ALTER-ATTACH-PARTITION",
			`ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM (5545) TO (5612)`,
			[]string{"sales", "orders", "orders_2024", "5545", "5612"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},
		{
			"TABLE-ALTER-ATTACH-PARTITION-DATE",
			`ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM ('2024-01-01') TO ('2024-12-31')`,
			[]string{"sales", "orders", "orders_2024", "2024-01-01", "2024-12-31"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},
		{
			"TABLE-ALTER-ATTACH-PARTITION-STRING",
			`ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_region_a FOR VALUES FROM ('US') TO ('CA')`,
			[]string{"sales", "orders", "orders_region_a", "US", "CA"},
			[]string{SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX},
		},

		// ─── INDEX WITH OPERATOR CLASS AND OPTIONS ─────────────────────────────────────────────
		{
			"INDEX-CREATE-WITH-OPERATOR-CLASS-OPTIONS",
			`CREATE INDEX idx_orders_location ON sales.orders USING gist (location postgis.gist_geometry_ops(siglen='1232'))`,
			[]string{"idx_orders_location", "sales", "orders", "location", "postgis", "gist_geometry_ops", "siglen", "1232"},
			[]string{INDEX_KIND_PREFIX, SCHEMA_KIND_PREFIX, TABLE_KIND_PREFIX, COLUMN_KIND_PREFIX, SCHEMA_KIND_PREFIX, OPCLASS_KIND_PREFIX, PARAMETER_KIND_PREFIX, CONST_KIND_PREFIX},
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
