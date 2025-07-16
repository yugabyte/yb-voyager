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
package queryissue

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const (
	stmt1 = `CREATE OR REPLACE FUNCTION list_high_earners(threshold NUMERIC) RETURNS public.emp1.salary%TYPE AS $$
DECLARE
    emp_name employees.name%TYPE;
    emp_salary employees.salary%TYPE;
BEGIN
    FOR emp_name, emp_salary IN 
        SELECT name, salary FROM employees WHERE salary > threshold
    LOOP
        RAISE NOTICE 'Employee: %, Salary: %', emp_name, emp_salary;
    END LOOP;
	EXECUTE 'ALTER TABLE employees CLUSTER ON idx;';
	PERFORM pg_advisory_unlock(sender_id);
	PERFORM pg_advisory_unlock(receiver_id);

	 -- Conditional logic
	IF balance >= withdrawal THEN
		RAISE NOTICE 'Sufficient balance, processing withdrawal.';
		-- Add the amount to the receiver's account
		UPDATE accounts SET balance = balance + amount WHERE account_id = receiver;
	ELSIF balance > 0 AND balance < withdrawal THEN
		RAISE NOTICE 'Insufficient balance, consider reducing the amount.';
		-- Add the amount to the receiver's account
		UPDATE accounts SET balance = balance + amount WHERE account_id = receiver;
	ELSE
		-- Add the amount to the receiver's account
		UPDATE accounts SET balance = balance + amount WHERE account_id = receiver;
		RAISE NOTICE 'No funds available.';
	END IF;

    SELECT id, xpath('/person/name/text()', data) AS name FROM test_xml_type;

    SELECT * FROM employees e WHERE e.xmax = (SELECT MAX(xmax) FROM employees WHERE department = e.department);
	RETURN emp_salary;

END;
$$ LANGUAGE plpgsql;`
	stmt2 = `CREATE OR REPLACE FUNCTION process_order(orderid orders.id%TYPE) RETURNS VOID AS $$
DECLARE
    lock_acquired BOOLEAN;
BEGIN
    lock_acquired := pg_try_advisory_lock(orderid); -- not able to report this as it is an assignment statement TODO: fix when support this 

    IF NOT lock_acquired THEN
        RAISE EXCEPTION 'Order % already being processed by another session', orderid;
    END IF;

    UPDATE orders
    SET processed_at = NOW()
    WHERE orders.order_id = orderid;

    RAISE NOTICE 'Order % processed successfully', orderid;

	EXECUTE 'ALTER TABLE ONLY public.example ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor=70)';

	EXECUTE 'CREATE UNLOGGED TABLE tbl_unlog (id int, val text);';

	EXECUTE 'CREATE INDEX idx_example ON example_table USING gin(name, name1);';

	EXECUTE 'CREATE INDEX idx_example ON schema1.example_table USING gist(name);';

    PERFORM pg_advisory_unlock(orderid);
END;
$$ LANGUAGE plpgsql;`
	stmt3 = `CREATE INDEX abc ON public.example USING btree (new_id) WITH (fillfactor='70');`
	stmt4 = `ALTER TABLE public.example DISABLE RULE example_rule;`
	stmt5 = `ALTER TABLE abc ADD CONSTRAINT cnstr_id UNIQUE (id) DEFERRABLE;`
	stmt6 = `SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(600) IS TRUE AND salary > 700;`
	stmt7 = `SELECT xmin, COUNT(*) FROM employees GROUP BY xmin HAVING COUNT(*) > 1;`
	stmt8 = `SELECT  id, xml_column, xpath('/root/element/@attribute', xml_column) as xpath_resuls FROM xml_documents;`
	stmt9 = `CREATE TABLE order_details (
    detail_id integer NOT NULL,
    quantity integer,
    price_per_unit numeric,
    amount numeric GENERATED ALWAYS AS (((quantity)::numeric * price_per_unit)) STORED
);`
	stmt10 = `CREATE TABLE test_non_pk_multi_column_list (
	id numeric NOT NULL PRIMARY KEY,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50)
) PARTITION BY LIST (country_code, record_type) ;`

	stmt11 = `CREATE TABLE "Test"(
	id int, 
	room_id int, 
	time_range trange,
	roomid int,
	timerange tsrange, 
	EXCLUDE USING gist (room_id WITH =, time_range WITH &&),
	CONSTRAINT no_time_overlap_constr EXCLUDE USING gist (roomid WITH =, timerange WITH &&)
);`
	stmt12 = `CREATE TABLE test_dt (id int, d daterange);`
	stmt13 = `CREATE INDEX idx_on_daterange on test_dt (d);`
	stmt14 = `CREATE MATERIALIZED VIEW public.sample_data_view AS
 SELECT sample_data.id,
    sample_data.name,
    sample_data.description,
    XMLFOREST(sample_data.name AS name, sample_data.description AS description) AS xml_data,
    pg_try_advisory_lock((sample_data.id)::bigint) AS lock_acquired,
    sample_data.ctid AS row_ctid,
    sample_data.xmin AS xmin_value
   FROM public.sample_data
  WITH NO DATA;`
	stmt15 = `CREATE VIEW public.orders_view AS
 SELECT orders.order_id,
    orders.customer_name,
    orders.product_name,
    orders.quantity,
    orders.price,
    XMLELEMENT(NAME "OrderDetails", XMLELEMENT(NAME "Customer", orders.customer_name), XMLELEMENT(NAME "Product", orders.product_name), XMLELEMENT(NAME "Quantity", orders.quantity), XMLELEMENT(NAME "TotalPrice", (orders.price * (orders.quantity)::numeric))) AS order_xml,
    XMLCONCAT(XMLELEMENT(NAME "Customer", orders.customer_name), XMLELEMENT(NAME "Product", orders.product_name)) AS summary_xml,
    pg_try_advisory_lock((hashtext((orders.customer_name || orders.product_name)))::bigint) AS lock_acquired,
    orders.ctid AS row_ctid,
    orders.xmin AS transaction_id
   FROM public.orders
   WITH LOCAL CHECK OPTION;`
	stmt16 = `CREATE TABLE public.xml_data_example (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
	d daterange Unique,
    description XML DEFAULT xmlparse(document '<product><name>Default Product</name><price>100.00</price><category>Electronics</category></product>'),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
) PARTITION BY LIST(id, name);`
	stmt17 = `ALTER TABLE invoices
ADD CONSTRAINT valid_invoice_structure
CHECK (xpath_exists('/invoice/customer', data));`
	stmt18 = `CREATE INDEX idx_invoices on invoices (xpath('/invoice/customer/text()', data));`
	stmt19 = `create table test_lo_default (id int, raster lo DEFAULT lo_import('3242'));`
	stmt20 = `CREATE VIEW public.view_explicit_security_invoker
	WITH (security_invoker = true) AS
	SELECT employee_id, first_name
	FROM public.employees;`
	stmt21 = `CREATE COLLATION case_insensitive (provider = icu, locale = 'und-u-ks-level2', deterministic = false);`
	stmt22 = `CREATE COLLATION new_schema.ignore_accents (provider = icu, locale = 'und-u-ks-level1-kc-true', deterministic = false);`
	stmt23 = `CREATE COLLATION upperfirst (provider = icu, locale = 'en-u-kf-upper', deterministic = true);`
	stmt24 = `CREATE COLLATION special (provider = icu, locale = 'en-u-kf-upper-kr-grek-latn');`
	stmt25 = `CREATE TABLE public.products (
    id INTEGER PRIMARY KEY,
    product_name VARCHAR(100),
    serial_number TEXT,
    UNIQUE NULLS NOT DISTINCT (product_name, serial_number)
	);`
	stmt26 = `ALTER TABLE public.products ADD CONSTRAINT unique_product_name UNIQUE NULLS NOT DISTINCT (product_name);`
	stmt27 = `CREATE UNIQUE INDEX unique_email_idx ON users (email) NULLS NOT DISTINCT;`

	// Foreign Key Datatype Mismatch Cases
	// Case 1: BIGINT → INTEGER (Simple mismatch)
	stmt28 = `CREATE TABLE users (id INTEGER PRIMARY KEY);`
	stmt29 = `CREATE TABLE orders (order_id SERIAL PRIMARY KEY, user_id BIGINT, FOREIGN KEY (user_id) REFERENCES users(id));`

	// Case 2: UUID → TEXT mismatch
	stmt30 = `CREATE TABLE payments (payment_id UUID PRIMARY KEY);`
	stmt31 = `CREATE TABLE invoices (invoice_id SERIAL PRIMARY KEY, payment_id TEXT);`
	stmt32 = `ALTER TABLE invoices ADD CONSTRAINT fk_payment_id FOREIGN KEY (payment_id) REFERENCES payments(payment_id);`

	// Case 3: Composite FK with partial mismatch (CHAR vs VARCHAR, INT vs TEXT)
	stmt33 = `CREATE TABLE shipments (shipment_id INTEGER, shipment_code CHAR(5), country_code INTEGER, PRIMARY KEY (shipment_id, shipment_code, country_code));`
	stmt34 = `CREATE TABLE delivery_tracking (tracking_id SERIAL PRIMARY KEY, shipment_id INTEGER, shipment_code VARCHAR(10), country_code TEXT);`
	stmt35 = `ALTER TABLE delivery_tracking ADD CONSTRAINT fk_shipment_ref FOREIGN KEY (shipment_id, shipment_code, country_code) REFERENCES shipments (shipment_id, shipment_code, country_code);`

	// Case 4: VARCHAR(10) → VARCHAR(5) mismatch
	stmt36 = `CREATE TABLE customers2 (customer_code VARCHAR(5) PRIMARY KEY);`
	stmt37 = `CREATE TABLE orders2 (order_id SERIAL PRIMARY KEY, customer_code VARCHAR(10));`
	stmt38 = `ALTER TABLE orders2 ADD CONSTRAINT fk_customer_code FOREIGN KEY (customer_code) REFERENCES customers2(customer_code);`

	// Case 5: NUMERIC(10,2) → NUMERIC(8,2) mismatch
	stmt39 = `CREATE TABLE products2 (product_id SERIAL PRIMARY KEY, price NUMERIC(10,2));`
	stmt40 = `CREATE TABLE orders3 (order_id SERIAL PRIMARY KEY, product_price NUMERIC(8,2));`
	stmt41 = `ALTER TABLE orders3 ADD CONSTRAINT fk_price FOREIGN KEY (product_price) REFERENCES products2(price);`

	// Case 6: Case sensitive tables and columns, INTERGER → BIGINT mismatch
	stmt42 = `CREATE TABLE "Accounts" ("UserID" INTEGER PRIMARY KEY);`
	stmt43 = `CREATE TABLE "Sessions" ("SessionID" SERIAL PRIMARY KEY, "UserID" BIGINT);`
	stmt44 = `ALTER TABLE "Sessions" ADD CONSTRAINT fk_userid FOREIGN KEY ("UserID") REFERENCES "Accounts"("UserID");`

	// Case 7: Partitioned table (single level), BIGINT → INTEGER mismatch
	stmt45 = `CREATE TABLE customers_root (id INTEGER PRIMARY KEY);`
	stmt46 = `CREATE TABLE orders_partitioned (
		order_id SERIAL,
		customer_id BIGINT
	) PARTITION BY RANGE (order_id);`
	stmt47 = `CREATE TABLE orders_p1 PARTITION OF orders_partitioned FOR VALUES FROM (1) TO (10000);`
	stmt48 = `ALTER TABLE orders_partitioned ADD CONSTRAINT fk_customer_id FOREIGN KEY (customer_id) REFERENCES customers_root(id);`

	// Case 8: Multi-level partitioned table, BIGINT → INTEGER mismatch
	stmt49 = `CREATE TABLE products_root (id INTEGER PRIMARY KEY);`
	stmt50 = `CREATE TABLE sales (
		sale_id INT,
		product_id BIGINT
	) PARTITION BY RANGE (sale_id);`
	stmt51 = `CREATE TABLE sales_q1 PARTITION OF sales FOR VALUES FROM (1) TO (1000) PARTITION BY RANGE (sale_id);`
	stmt52 = `CREATE TABLE sales_q1_jan PARTITION OF sales_q1 FOR VALUES FROM (1) TO (250);`
	stmt53 = `ALTER TABLE sales ADD CONSTRAINT fk_product_id FOREIGN KEY (product_id) REFERENCES products_root(id);`

	// Case 9: Inherited table (multi-level), BIGINT → INTEGER mismatch
	stmt54 = `CREATE TABLE base_items (item_id INTEGER PRIMARY KEY);`
	stmt55 = `CREATE TABLE derived_items_1 (extra_info TEXT) INHERITS (base_items);`
	stmt56 = `CREATE TABLE derived_items_2 (notes TEXT, item_id BIGINT) INHERITS (derived_items_1);`
	stmt57 = `ALTER TABLE derived_items_2 ADD CONSTRAINT fk_item_id FOREIGN KEY (item_id) REFERENCES base_items(item_id);`

	// Case 10: Cross-schema foreign key with BIGINT → INTEGER mismatch
	stmt58 = `CREATE SCHEMA schema1;`
	stmt59 = `CREATE SCHEMA schema2;`
	stmt60 = `CREATE TABLE schema2.departments (dept_id INTEGER PRIMARY KEY);`
	stmt61 = `CREATE TABLE schema1.employees (emp_id SERIAL PRIMARY KEY, dept_id BIGINT);`
	stmt62 = `ALTER TABLE schema1.employees ADD CONSTRAINT fk_dept FOREIGN KEY (dept_id) REFERENCES schema2.departments(dept_id);`

	// Additional type mapping test cases
	// Case 11: SMALLINT → INTEGER mismatch (testing int2 mapping)
	stmt63 = `CREATE TABLE small_users (id SMALLINT PRIMARY KEY);`
	stmt64 = `CREATE TABLE small_orders (order_id SERIAL PRIMARY KEY, user_id INTEGER);`
	stmt65 = `ALTER TABLE small_orders ADD CONSTRAINT fk_small_user FOREIGN KEY (user_id) REFERENCES small_users(id);`

	// Case 12: REAL → DOUBLE PRECISION mismatch (testing float4/float8 mapping)
	stmt66 = `CREATE TABLE measurements (id SERIAL PRIMARY KEY, value REAL);`
	stmt67 = `CREATE TABLE precise_measurements (id SERIAL PRIMARY KEY, value DOUBLE PRECISION);`
	stmt68 = `ALTER TABLE precise_measurements ADD CONSTRAINT fk_measurement FOREIGN KEY (value) REFERENCES measurements(value);`

	// Case 13: BOOLEAN → TEXT mismatch (testing bool mapping)
	stmt69 = `CREATE TABLE flags (id SERIAL PRIMARY KEY, is_active BOOLEAN);`
	stmt70 = `CREATE TABLE flag_logs (id SERIAL PRIMARY KEY, status TEXT);`
	stmt71 = `ALTER TABLE flag_logs ADD CONSTRAINT fk_flag_status FOREIGN KEY (status) REFERENCES flags(is_active);`

	// Case 14: SERIAL → BIGSERIAL mismatch (testing serial mapping)
	stmt72 = `CREATE TABLE simple_items (id SERIAL PRIMARY KEY);`
	stmt73 = `CREATE TABLE complex_items (id BIGSERIAL PRIMARY KEY);`
	stmt74 = `ALTER TABLE complex_items ADD CONSTRAINT fk_simple_item FOREIGN KEY (id) REFERENCES simple_items(id);`

	// Case 15: BIT(10) → BIT(8) mismatch
	stmt75 = `CREATE TABLE bit_flags (id SERIAL PRIMARY KEY, flags BIT(10));`
	stmt76 = `CREATE TABLE bit_logs (id SERIAL PRIMARY KEY, status_flags BIT(8));`
	stmt77 = `ALTER TABLE bit_logs ADD CONSTRAINT fk_bit_flags FOREIGN KEY (status_flags) REFERENCES bit_flags(flags);`

	// Case 16: TIMESTAMP(6) → TIMESTAMP(3) mismatch
	stmt78 = `CREATE TABLE precise_events (id SERIAL PRIMARY KEY, event_time TIMESTAMP(6));`
	stmt79 = `CREATE TABLE event_logs (id SERIAL PRIMARY KEY, log_time TIMESTAMP(3));`
	stmt80 = `ALTER TABLE event_logs ADD CONSTRAINT fk_event_time FOREIGN KEY (log_time) REFERENCES precise_events(event_time);`

	// Case 17: TIME(6) → TIME(3) mismatch
	stmt81 = `CREATE TABLE precise_schedules (id SERIAL PRIMARY KEY, start_time TIME(6));`
	stmt82 = `CREATE TABLE schedule_logs (id SERIAL PRIMARY KEY, log_time TIME(3));`
	stmt83 = `ALTER TABLE schedule_logs ADD CONSTRAINT fk_schedule_time FOREIGN KEY (log_time) REFERENCES precise_schedules(start_time);`

	// Case 18: INTERVAL(6) → INTERVAL(3) mismatch
	stmt84 = `CREATE TABLE precise_durations (id SERIAL PRIMARY KEY, duration INTERVAL(6));`
	stmt85 = `CREATE TABLE duration_logs (id SERIAL PRIMARY KEY, log_duration INTERVAL(3));`
	stmt86 = `ALTER TABLE duration_logs ADD CONSTRAINT fk_duration FOREIGN KEY (log_duration) REFERENCES precise_durations(duration);`

	// Case 19: TIMESTAMP WITH TIME ZONE precision mismatch
	stmt87 = `CREATE TABLE tz_events (id SERIAL PRIMARY KEY, event_time TIMESTAMP(6) WITH TIME ZONE);`
	stmt88 = `CREATE TABLE tz_logs (id SERIAL PRIMARY KEY, log_time TIMESTAMP(0) WITH TIME ZONE);`
	stmt89 = `ALTER TABLE tz_logs ADD CONSTRAINT fk_tz_event_time FOREIGN KEY (log_time) REFERENCES tz_events(event_time);`

	// Case 20: TIME WITH TIME ZONE precision mismatch
	stmt90 = `CREATE TABLE tz_schedules (id SERIAL PRIMARY KEY, start_time TIME(6) WITH TIME ZONE);`
	stmt91 = `CREATE TABLE tz_schedule_logs (id SERIAL PRIMARY KEY, log_time TIME(0) WITH TIME ZONE);`
	stmt92 = `ALTER TABLE tz_schedule_logs ADD CONSTRAINT fk_tz_schedule_time FOREIGN KEY (log_time) REFERENCES tz_schedules(start_time);`

	// Case 21: VARBIT(10) → VARBIT(8) mismatch
	stmt93 = `CREATE TABLE varbit_flags (id SERIAL PRIMARY KEY, flags VARBIT(10));`
	stmt94 = `CREATE TABLE varbit_logs (id SERIAL PRIMARY KEY, status_flags VARBIT(8));`
	stmt95 = `ALTER TABLE varbit_logs ADD CONSTRAINT fk_varbit_flags FOREIGN KEY (status_flags) REFERENCES varbit_flags(flags);`
)

func modifiedIssuesforPLPGSQL(issues []QueryIssue, objType string, objName string) []QueryIssue {
	return lo.Map(issues, func(i QueryIssue, _ int) QueryIssue {
		i.ObjectType = objType
		i.ObjectName = objName
		return i
	})
}
func TestAllIssues(t *testing.T) {
	requiredDDLs := []string{stmt12}
	parserIssueDetector := NewParserIssueDetector()
	stmtsWithExpectedIssues := map[string][]QueryIssue{
		stmt1: []QueryIssue{
			NewPercentTypeSyntaxIssue("FUNCTION", "list_high_earners", "public.emp1.salary%TYPE"),
			NewPercentTypeSyntaxIssue("FUNCTION", "list_high_earners", "employees.name%TYPE"),
			NewPercentTypeSyntaxIssue("FUNCTION", "list_high_earners", "employees.salary%TYPE"),
			NewClusterONIssue("TABLE", "employees", "ALTER TABLE employees CLUSTER ON idx;"),
			NewAdvisoryLocksIssue("DML_QUERY", "", "SELECT pg_advisory_unlock(sender_id);"),
			NewAdvisoryLocksIssue("DML_QUERY", "", "SELECT pg_advisory_unlock(receiver_id);"),
			NewXmlFunctionsIssue("DML_QUERY", "", "SELECT id, xpath('/person/name/text()', data) AS name FROM test_xml_type;"),
			NewXmaxSystemColumnIssue("DML_QUERY", "", "SELECT * FROM employees e WHERE e.xmax = (SELECT MAX(xmax) FROM employees WHERE department = e.department);"),
		},
		stmt2: []QueryIssue{
			NewPercentTypeSyntaxIssue("FUNCTION", "process_order", "orders.id%TYPE"),
			NewStorageParameterIssue("TABLE", "public.example", "ALTER TABLE ONLY public.example ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor=70);"),
			NewMultiColumnGinIndexIssue("INDEX", "idx_example ON example_table", "CREATE INDEX idx_example ON example_table USING gin(name, name1);"),
			NewUnsupportedGistIndexMethodIssue("INDEX", "idx_example ON schema1.example_table", "CREATE INDEX idx_example ON schema1.example_table USING gist(name);"),
			NewAdvisoryLocksIssue("DML_QUERY", "", "SELECT pg_advisory_unlock(orderid);"),
		},
		stmt3: []QueryIssue{
			NewStorageParameterIssue("INDEX", "abc ON public.example", stmt3),
		},
		stmt4: []QueryIssue{
			NewAlterTableDisableRuleIssue("TABLE", "public.example", stmt4, "example_rule"),
		},
		stmt5: []QueryIssue{
			NewDeferrableConstraintIssue("TABLE", "abc", stmt5, "cnstr_id"),
		},
		stmt6: []QueryIssue{
			NewAdvisoryLocksIssue("DML_QUERY", "", stmt6),
		},
		stmt7: []QueryIssue{
			NewXminSystemColumnIssue("DML_QUERY", "", stmt7),
		},
		stmt8: []QueryIssue{
			NewXmlFunctionsIssue("DML_QUERY", "", stmt8),
		},
		stmt9: []QueryIssue{
			NewGeneratedColumnsIssue("TABLE", "order_details", stmt9, []string{"amount"}),
		},
		stmt10: []QueryIssue{
			NewMultiColumnListPartition("TABLE", "test_non_pk_multi_column_list", stmt10),
			NewInsufficientColumnInPKForPartition("TABLE", "test_non_pk_multi_column_list", stmt10, []string{"country_code", "record_type"}),
		},
		stmt11: []QueryIssue{
			NewExclusionConstraintIssue("TABLE", "Test", stmt11, "Test_room_id_time_range_excl"),
			NewExclusionConstraintIssue("TABLE", "Test", stmt11, "no_time_overlap_constr"),
		},
		stmt13: []QueryIssue{
			NewIndexOnDaterangeDatatypeIssue("INDEX", "idx_on_daterange ON test_dt", stmt13),
		},
	}

	//Should modify it in similar way we do it actual code as the particular DDL issue in plpgsql can have different Details map on the basis of objectType
	stmtsWithExpectedIssues[stmt1] = modifiedIssuesforPLPGSQL(stmtsWithExpectedIssues[stmt1], "FUNCTION", "list_high_earners")

	stmtsWithExpectedIssues[stmt2] = modifiedIssuesforPLPGSQL(stmtsWithExpectedIssues[stmt2], "FUNCTION", "process_order")

	for _, stmt := range requiredDDLs {
		err := parserIssueDetector.ParseAndProcessDDL(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}

	parserIssueDetector.FinalizeColumnMetadata()

	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}

}

func TestDDLIssues(t *testing.T) {
	requiredDDLs := []string{stmt16, stmt28, stmt29, stmt30, stmt31, stmt32, stmt33, stmt34, stmt35, stmt36, stmt37, stmt38, stmt39, stmt40, stmt41, stmt42, stmt43, stmt44, stmt45, stmt46, stmt47, stmt48, stmt49, stmt50, stmt51, stmt52, stmt53, stmt54, stmt55, stmt56, stmt57, stmt58, stmt59, stmt60, stmt61, stmt62, stmt63, stmt64, stmt65, stmt66, stmt67, stmt68, stmt69, stmt70, stmt71, stmt72, stmt73, stmt74, stmt75, stmt76, stmt77, stmt78, stmt79, stmt80, stmt81, stmt82, stmt83, stmt84, stmt85, stmt86, stmt87, stmt88, stmt89, stmt90, stmt91, stmt92, stmt93, stmt94, stmt95}
	parserIssueDetector := NewParserIssueDetector()
	stmtsWithExpectedIssues := map[string][]QueryIssue{
		stmt14: []QueryIssue{
			NewAdvisoryLocksIssue("MVIEW", "public.sample_data_view", stmt14),
			NewCtidSystemColumnIssue("MVIEW", "public.sample_data_view", stmt14),
			NewXminSystemColumnIssue("MVIEW", "public.sample_data_view", stmt14),
			NewXmlFunctionsIssue("MVIEW", "public.sample_data_view", stmt14),
		},
		stmt15: []QueryIssue{
			NewAdvisoryLocksIssue("VIEW", "public.orders_view", stmt15),
			NewCtidSystemColumnIssue("VIEW", "public.orders_view", stmt15),
			NewXminSystemColumnIssue("VIEW", "public.orders_view", stmt15),
			NewXmlFunctionsIssue("VIEW", "public.orders_view", stmt15),
			//TODO: Add CHECK OPTION issue when we move it from regex to parser logic
		},
		stmt16: []QueryIssue{
			NewXmlFunctionsIssue("TABLE", "public.xml_data_example", stmt16),
			NewPrimaryOrUniqueConstraintOnDaterangeDatatypeIssue("TABLE", "public.xml_data_example", stmt16, "daterange", "xml_data_example_d_key"),
			NewMultiColumnListPartition("TABLE", "public.xml_data_example", stmt16),
			NewInsufficientColumnInPKForPartition("TABLE", "public.xml_data_example", stmt16, []string{"name"}),
			NewXMLDatatypeIssue("TABLE", "public.xml_data_example", stmt16, "XML", "description"),
		},
		stmt17: []QueryIssue{
			NewXmlFunctionsIssue("TABLE", "invoices", stmt17),
		},
		stmt18: []QueryIssue{
			NewXmlFunctionsIssue("INDEX", "idx_invoices ON invoices", stmt18),
		},
		stmt19: []QueryIssue{
			NewLODatatypeIssue("TABLE", "test_lo_default", stmt19, "LARGE OBJECT", "raster"),
			NewLOFuntionsIssue("TABLE", "test_lo_default", stmt19, []string{"lo_import"}),
		},
		stmt20: []QueryIssue{
			NewSecurityInvokerViewIssue("VIEW", "public.view_explicit_security_invoker", stmt20),
		},
		stmt21: []QueryIssue{
			NewNonDeterministicCollationIssue("COLLATION", "case_insensitive", stmt21),
		},
		stmt22: []QueryIssue{
			NewNonDeterministicCollationIssue("COLLATION", "new_schema.ignore_accents", stmt22),
		},
		stmt23: []QueryIssue{
			NewDeterministicOptionCollationIssue("COLLATION", "upperfirst", stmt23),
		},
		stmt24: []QueryIssue{},
		stmt25: []QueryIssue{
			NewUniqueNullsNotDistinctIssue("TABLE", "public.products", stmt25),
		},
		stmt26: []QueryIssue{
			NewUniqueNullsNotDistinctIssue("TABLE", "public.products", stmt26),
		},
		stmt27: []QueryIssue{
			NewUniqueNullsNotDistinctIssue("INDEX", "unique_email_idx ON users", stmt27),
		},
		stmt29: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "orders", stmt29, "orders.user_id", "users.id", "bigint", "integer"),
		},
		stmt32: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "invoices", stmt32, "invoices.payment_id", "payments.payment_id", "text", "uuid"),
		},
		stmt35: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "delivery_tracking", stmt35, "delivery_tracking.shipment_code", "shipments.shipment_code", "varchar(10)", "char(5)"),
			NewForeignKeyDatatypeMismatchIssue("TABLE", "delivery_tracking", stmt35, "delivery_tracking.country_code", "shipments.country_code", "text", "integer"),
		},
		stmt38: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "orders2", stmt38, "orders2.customer_code", "customers2.customer_code", "varchar(10)", "varchar(5)"),
		},
		stmt41: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "orders3", stmt41, "orders3.product_price", "products2.price", "numeric(8,2)", "numeric(10,2)"),
		},
		stmt44: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "Sessions", stmt44, "Sessions.UserID", "Accounts.UserID", "bigint", "integer"),
		},
		stmt48: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "orders_partitioned", stmt48, "orders_partitioned.customer_id", "customers_root.id", "bigint", "integer"),
		},
		stmt53: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "sales", stmt53, "sales.product_id", "products_root.id", "bigint", "integer"),
		},
		stmt57: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "derived_items_2", stmt57, "derived_items_2.item_id", "base_items.item_id", "bigint", "integer"),
		},
		stmt62: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "schema1.employees", stmt62, "schema1.employees.dept_id", "schema2.departments.dept_id", "bigint", "integer"),
		},
		stmt65: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "small_orders", stmt65, "small_orders.user_id", "small_users.id", "integer", "smallint"),
		},
		stmt68: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "precise_measurements", stmt68, "precise_measurements.value", "measurements.value", "double precision", "real"),
		},
		stmt71: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "flag_logs", stmt71, "flag_logs.status", "flags.is_active", "text", "boolean"),
		},
		stmt74: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "complex_items", stmt74, "complex_items.id", "simple_items.id", "bigserial", "serial"),
		},
		stmt77: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "bit_logs", stmt77, "bit_logs.status_flags", "bit_flags.flags", "bit(8)", "bit(10)"),
		},
		stmt80: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "event_logs", stmt80, "event_logs.log_time", "precise_events.event_time", "timestamp(3)", "timestamp(6)"),
		},
		stmt83: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "schedule_logs", stmt83, "schedule_logs.log_time", "precise_schedules.start_time", "time(3)", "time(6)"),
		},
		stmt86: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "duration_logs", stmt86, "duration_logs.log_duration", "precise_durations.duration", "interval(3)", "interval(6)"),
		},
		stmt89: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "tz_logs", stmt89, "tz_logs.log_time", "tz_events.event_time", "timestamptz(0)", "timestamptz(6)"),
		},
		stmt92: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "tz_schedule_logs", stmt92, "tz_schedule_logs.log_time", "tz_schedules.start_time", "timetz(0)", "timetz(6)"),
		},
		stmt95: []QueryIssue{
			NewForeignKeyDatatypeMismatchIssue("TABLE", "varbit_logs", stmt95, "varbit_logs.status_flags", "varbit_flags.flags", "varbit(8)", "varbit(10)"),
		},
	}
	for _, stmt := range requiredDDLs {
		err := parserIssueDetector.ParseAndProcessDDL(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}

	parserIssueDetector.FinalizeColumnMetadata()

	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetDDLIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s. \nFound: %v", expectedIssue, stmt, issues)
		}
	}
}

func TestUnloggedTableIssueReportedInOlderVersion(t *testing.T) {
	stmt := "CREATE UNLOGGED TABLE tbl_unlog (id int, val text);"
	parserIssueDetector := NewParserIssueDetector()

	// Not reported by default
	issues, err := parserIssueDetector.GetDDLIssues(stmt, ybversion.LatestStable)
	testutils.FatalIfError(t, err)
	assert.Equal(t, 0, len(issues))

	// older version should report the issue
	issues, err = parserIssueDetector.GetDDLIssues(stmt, ybversion.V2024_1_0_0)
	testutils.FatalIfError(t, err)
	assert.Equal(t, 1, len(issues))
	assert.True(t, cmp.Equal(issues[0], NewUnloggedTableIssue("TABLE", "tbl_unlog", stmt)))
}

func TestLargeObjectIssues(t *testing.T) {
	sqls := []string{
		`CREATE OR REPLACE FUNCTION manage_large_object(loid OID) RETURNS VOID AS $$
BEGIN
    IF loid IS NOT NULL THEN
        -- Unlink the large object to free up storage
        PERFORM lo_unlink(loid);
    END IF;
END;
$$ LANGUAGE plpgsql;`,
		`CREATE OR REPLACE FUNCTION import_file_to_table(file_path TEXT, doc_title TEXT)
RETURNS VOID AS $$
DECLARE
    loid OID;
BEGIN
    -- Import the file and get the large object OID
    loid := lo_import(file_path); -- NOT DETECTED 

    -- Insert the file metadata and OID into the table
    INSERT INTO documents (title, content_oid) VALUES (doc_title, lo_import(file_path));

    RAISE NOTICE 'File imported with OID % and linked to title %', loid, doc_title;
END;
$$ LANGUAGE plpgsql;
`,
		`CREATE OR REPLACE FUNCTION export_large_object(doc_title TEXT, file_path TEXT)
RETURNS VOID AS $$
DECLARE
    loid OID;
BEGIN
    -- Retrieve the OID of the large object associated with the given title
    SELECT content_oid INTO loid FROM documents WHERE title = doc_title;

    -- Check if the large object exists
    IF loid IS NULL THEN
        RAISE EXCEPTION 'No large object found for title %', doc_title;
    END IF;

    -- Export the large object to the specified file
    PERFORM lo_export(loid, file_path);

    RAISE NOTICE 'Large object with OID % exported to %', loid, file_path;
END;
$$ LANGUAGE plpgsql;
`,
		`CREATE OR REPLACE PROCEDURE read_large_object(doc_title TEXT)
AS $$
DECLARE
    loid OID;
    fd INTEGER;
    buffer BYTEA;
    content TEXT;
BEGIN
    -- Retrieve the OID of the large object associated with the given title
    SELECT content_oid INTO loid FROM documents WHERE title = doc_title;

    -- Check if the large object exists
    IF loid IS NULL THEN
        RAISE EXCEPTION 'No large object found for title %', doc_title;
    END IF;

    -- Open the large object for reading
    fd := lo_open(loid, 262144); -- 262144 = INV_READ

    -- Read data from the large object
    buffer := lo_get(fd);
    content := convert_from(buffer, 'UTF8');

    -- Close the large object
    PERFORM lo_close(fd);

END;
$$ LANGUAGE plpgsql;
`,
		`CREATE OR REPLACE FUNCTION write_to_large_object(doc_title TEXT, new_data TEXT)
RETURNS VOID AS $$
DECLARE
    loid OID;
    fd INTEGER;
BEGIN
    -- Create the table if it doesn't already exist
    EXECUTE 'CREATE TABLE IF NOT EXISTS test_large_objects(id INT, raster lo DEFAULT lo_import(3242));';

    -- Retrieve the OID of the large object associated with the given title
    SELECT content_oid INTO loid FROM documents WHERE title = doc_title;

    -- Check if the large object exists
    IF loid IS NULL THEN
        RAISE EXCEPTION 'No large object found for title %', doc_title;
    END IF;

    -- Open the large object for writing
    fd := lo_open(loid, 524288); -- 524288 = INV_WRITE

    -- Write new data to the large object
    PERFORM lo_put(fd, convert_to(new_data, 'UTF8'));

    -- Close the large object
    PERFORM lo_close(fd);

    RAISE NOTICE 'Data written to large object with OID %', loid;
END;
$$ LANGUAGE plpgsql;
`,
		`CREATE TRIGGER t_raster BEFORE UPDATE OR DELETE ON image
    FOR EACH ROW EXECUTE FUNCTION lo_manage(raster);`,
	}

	expectedSQLsWithIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewLOFuntionsIssue("DML_QUERY", "", "SELECT lo_unlink(loid);", []string{"lo_unlink"}),
		},
		sqls[1]: []QueryIssue{
			NewLOFuntionsIssue("DML_QUERY", "", "INSERT INTO documents (title, content_oid) VALUES (doc_title, lo_import(file_path));", []string{"lo_import"}),
		},
		sqls[2]: []QueryIssue{
			NewLOFuntionsIssue("DML_QUERY", "", "SELECT lo_export(loid, file_path);", []string{"lo_export"}),
		},
		sqls[3]: []QueryIssue{
			NewLOFuntionsIssue("DML_QUERY", "", "SELECT lo_close(fd);", []string{"lo_close"}),
		},
		sqls[4]: []QueryIssue{
			NewLOFuntionsIssue("DML_QUERY", "", "SELECT lo_put(fd, convert_to(new_data, 'UTF8'));", []string{"lo_put"}),
			NewLOFuntionsIssue("DML_QUERY", "", "SELECT lo_close(fd);", []string{"lo_close"}),
			NewLODatatypeIssue("TABLE", "test_large_objects", "CREATE TABLE IF NOT EXISTS test_large_objects(id INT, raster lo DEFAULT lo_import(3242));", "LARGE OBJECT", "raster"),
			NewLOFuntionsIssue("TABLE", "test_large_objects", "CREATE TABLE IF NOT EXISTS test_large_objects(id INT, raster lo DEFAULT lo_import(3242));", []string{"lo_import"}),
		},
		sqls[5]: []QueryIssue{
			NewLOFuntionsIssue("TRIGGER", "t_raster ON image", sqls[5], []string{"lo_manage"}),
		},
	}
	expectedSQLsWithIssues[sqls[0]] = modifiedIssuesforPLPGSQL(expectedSQLsWithIssues[sqls[0]], "FUNCTION", "manage_large_object")
	expectedSQLsWithIssues[sqls[1]] = modifiedIssuesforPLPGSQL(expectedSQLsWithIssues[sqls[1]], "FUNCTION", "import_file_to_table")
	expectedSQLsWithIssues[sqls[2]] = modifiedIssuesforPLPGSQL(expectedSQLsWithIssues[sqls[2]], "FUNCTION", "export_large_object")
	expectedSQLsWithIssues[sqls[3]] = modifiedIssuesforPLPGSQL(expectedSQLsWithIssues[sqls[3]], "PROCEDURE", "read_large_object")
	expectedSQLsWithIssues[sqls[4]] = modifiedIssuesforPLPGSQL(expectedSQLsWithIssues[sqls[4]], "FUNCTION", "write_to_large_object")

	parserIssueDetector := NewParserIssueDetector()

	for stmt, expectedIssues := range expectedSQLsWithIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		fmt.Printf("%v", issues)

		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)
		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

// currently, both FuncCallDetector and XmlExprDetector can detect XMLFunctionsIssue
// statement below has both XML functions and XML expressions.
// but we want to only return one XMLFunctionsIssue from parserIssueDetector.getDMLIssues
// and there is some workaround in place to avoid returning multiple issues in .genericIssues method
func TestSingleXMLIssueIsDetected(t *testing.T) {
	stmt := `
	SELECT e.id, x.employee_xml
		FROM employees e
		JOIN (
			SELECT xmlelement(name "employee", xmlattributes(e.id AS "id"), e.name) AS employee_xml
			FROM employees e
		) x ON x.employee_xml IS NOT NULL
		WHERE xmlexists('//employee[name="John Doe"]' PASSING BY REF x.employee_xml);`

	parserIssueDetector := NewParserIssueDetector()
	issues, err := parserIssueDetector.getDMLIssues(stmt)
	testutils.FatalIfError(t, err)
	assert.Equal(t, 1, len(issues))
}

func TestJsonUnsupportedFeatures(t *testing.T) {
	sqls := []string{
		`SELECT department, JSON_ARRAYAGG(name) AS employees_json
	FROM employees
	GROUP BY department;`,
		`INSERT INTO movies (details)
VALUES (
    JSON_OBJECT('title' VALUE 'Dune', 'director' VALUE 'Denis Villeneuve', 'year' VALUE 2021)
);`,
		`SELECT json_objectagg(k VALUE v) AS json_result
	FROM (VALUES ('a', 1), ('b', 2), ('c', 3)) AS t(k, v);`,
		`SELECT JSON_OBJECT(
  'movie' VALUE JSON_OBJECT('code' VALUE 'P123', 'title' VALUE 'Jaws'),
  'director' VALUE 'Steven Spielberg'
) AS nested_json_object;`,
		`select JSON_ARRAYAGG('[1, "2", null]');`,
		`SELECT JSON_OBJECT(
    'code' VALUE 'P123',
    'title' VALUE 'Jaws',
    'price' VALUE 19.99,
    'available' VALUE TRUE
) AS json_obj;`,
		`SELECT id, JSON_QUERY(details, '$.author') AS author
FROM books;`,
		`SELECT jt.* FROM
 my_films,
 JSON_TABLE (js, '$.favorites[*]' COLUMNS (
   id FOR ORDINALITY,
   kind text PATH '$.kind',
   title text PATH '$.films[*].title' WITH WRAPPER,
   director text PATH '$.films[*].director' WITH WRAPPER)) AS jt;`,
		`SELECT jt.* FROM
 my_films,
 JSON_TABLE (js, $1 COLUMNS (
   id FOR ORDINALITY,
   kind text PATH '$.kind',
   title text PATH '$.films[*].title' WITH WRAPPER,
   director text PATH '$.films[*].director' WITH WRAPPER)) AS jt;`,
		`SELECT id, details
FROM books
WHERE JSON_EXISTS(details, '$.author');`,
		`SELECT id, JSON_QUERY(details, '$.author') AS author
FROM books;`,
		`SELECT 
    id, 
    JSON_VALUE(details, '$.title') AS title,
    JSON_VALUE(details, '$.price')::NUMERIC AS price
FROM books;`,
		`SELECT id, JSON_VALUE(details, '$.title') AS title
FROM books
WHERE JSON_EXISTS(details, '$.price ? (@ > $price)' PASSING 30 AS price);`,
		`SELECT js, js IS JSON "json?", js IS JSON SCALAR "scalar?", js IS JSON OBJECT "object?", js IS JSON ARRAY "array?" 
FROM (VALUES ('123'), ('"abc"'), ('{"a": "b"}'), ('[1,2]'),('abc')) foo(js);`,
		`SELECT js,
  js IS JSON OBJECT "object?",
  js IS JSON ARRAY "array?",
  js IS JSON ARRAY WITH UNIQUE KEYS "array w. UK?",
  js IS JSON ARRAY WITHOUT UNIQUE KEYS "array w/o UK?"
FROM (VALUES ('[{"a":"1"},
 {"b":"2","b":"3"}]')) foo(js);`,
		`SELECT js,
  js IS JSON OBJECT "object?"
  FROM (VALUES ('[{"a":"1"},
 {"b":"2","b":"3"}]')) foo(js); `,
		`CREATE MATERIALIZED VIEW public.test_jsonb_view AS
SELECT 
    id,
    data->>'name' AS name,
    JSON_VALUE(data, '$.age' RETURNING INTEGER) AS age,
    JSON_EXISTS(data, '$.skills[*] ? (@ == "JSON")') AS knows_json,
    jt.skill
FROM public.test_jsonb,
JSON_TABLE(data, '$.skills[*]' 
    COLUMNS (
        skill TEXT PATH '$'
    )
) AS jt;`,
		`SELECT JSON_ARRAY($1, 12, TRUE, $2) AS json_array;`,
		`CREATE TABLE sales.json_data (
    id int PRIMARY KEY,
    array_column TEXT CHECK (array_column IS JSON ARRAY),
    unique_keys_column TEXT CHECK (unique_keys_column IS JSON WITH UNIQUE KEYS)
);`,
	}
	sqlsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[0], []string{JSON_ARRAYAGG}),
		},
		sqls[1]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[1], []string{JSON_OBJECT}),
		},
		sqls[2]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[2], []string{JSON_OBJECTAGG}),
		},
		sqls[3]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[3], []string{JSON_OBJECT}),
		},
		sqls[4]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[4], []string{JSON_ARRAYAGG}),
		},
		sqls[5]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[5], []string{JSON_OBJECT}),
		},
		sqls[6]: []QueryIssue{
			NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[6], []string{JSON_QUERY}),
		},
		sqls[7]: []QueryIssue{
			NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[7], []string{JSON_TABLE}),
		},
		// sqls[8]: []QueryIssue{
		// 	NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[8]),
		//NOT REPORTED YET because of PARSER failing if JSON_TABLE has a parameterized values $1, $2 ...
		//https://github.com/pganalyze/pg_query_go/issues/127
		// },
		sqls[9]: []QueryIssue{
			NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[9], []string{JSON_EXISTS}),
		},
		sqls[10]: []QueryIssue{
			NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[10], []string{JSON_QUERY}),
		},
		sqls[11]: []QueryIssue{
			NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[11], []string{JSON_VALUE}),
		},
		sqls[12]: []QueryIssue{
			NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[12], []string{JSON_VALUE, JSON_EXISTS}),
		},
		sqls[13]: []QueryIssue{
			NewJsonPredicateIssue(DML_QUERY_OBJECT_TYPE, "", sqls[13]),
		},
		sqls[14]: []QueryIssue{
			NewJsonPredicateIssue(DML_QUERY_OBJECT_TYPE, "", sqls[14]),
		},
		sqls[15]: []QueryIssue{
			NewJsonPredicateIssue(DML_QUERY_OBJECT_TYPE, "", sqls[15]),
		},
		sqls[16]: []QueryIssue{
			NewJsonQueryFunctionIssue("MVIEW", "public.test_jsonb_view", sqls[16], []string{JSON_VALUE, JSON_EXISTS, JSON_TABLE}),
		},
		sqls[17]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[17], []string{JSON_ARRAY}),
		},
		sqls[18]: []QueryIssue{
			NewJsonPredicateIssue("TABLE", "sales.json_data", sqls[18]),
		},
	}
	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range sqlsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)
		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestJsonbSubscriptingIssue(t *testing.T) {
	ddlSqls := []string{
		`CREATE TABLE test_jsonb1 (                                                                                                                                                                         
    id SERIAL PRIMARY KEY,
    data JSONB
);`,
		`CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT,
    email TEXT,
    active BOOLEAN
);`,
		`CREATE TABLE test_json_chk (
    id int,
    data1 jsonb,
    CHECK (data1['key']<>'')
);`,
		`CREATE OR REPLACE FUNCTION get_user_info(user_id INT)
RETURNS JSONB AS $$
BEGIN
    RETURN (
        SELECT jsonb_build_object(
            'id', id,
            'name', name,
            'email', email,
            'active', active
        )
        FROM users
        WHERE id = user_id
    );
END;
$$ LANGUAGE plpgsql;`,
	}
	sqls := []string{

		`CREATE TABLE test_json_chk (
    id int,
    data1 jsonb,
    CHECK (data1['key']<>'')
);`,
		`SELECT 
    data->>'name' AS name, 
    data['scores'][1] AS second_score
FROM test_jsonb1;`,
		`SELECT ('[{"key": "value1"}, {"key": "value2"}]'::jsonb)[1]['key'] AS object_in_array; `,
		`SELECT (JSON_OBJECT(
  'movie' VALUE JSON_OBJECT('code' VALUE 'P123', 'title' VALUE 'Jaws'),
  'director' VALUE 'Steven Spielberg'
)::JSONB)['movie'] AS nested_json_object;`,
		`SELECT (jsonb_build_object('name', 'PostgreSQL', 'version', 14, 'open_source', TRUE))['name'] AS json_obj;`,
		`SELECT ('{"key": "value1"}'::jsonb || '{"key": "value2"}'::jsonb)['key'] AS object_in_array;`,
		`SELECT ('{"key": "value1"}'::jsonb || '{"key": "value2"}')['key'] AS object_in_array;`,
		`SELECT (data || '{"new_key": "new_value"}' )['name'] FROM test_jsonb;`,
		`SELECT (jsonb_build_object('name', 'PostgreSQL', 'version', 14, 'open_source', TRUE))['name'] AS json_obj;`,
		`SELECT (jsonb_build_object('name', 'PostgreSQL', 'version', 14, 'open_source', TRUE) || '{"key": "value2"}')['name'] AS json_obj;`,
		`SELECT (ROW('Alice', 'Smith', 25))['0'] ;`,
		`SELECT (get_user_info(2))['name'] AS user_info;`,
	}

	stmtsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewJsonbSubscriptingIssue(TABLE_OBJECT_TYPE, "test_json_chk", sqls[0]),
		},
		sqls[1]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[1]),
		},
		sqls[2]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[2]),
		},
		sqls[3]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[3]),
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[3], []string{JSON_OBJECT}),
		},
		sqls[4]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[4]),
		},
		sqls[5]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[5]),
		},
		sqls[6]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[6]),
		},
		sqls[7]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[7]),
		},
		sqls[8]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[8]),
		},
		sqls[9]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[9]),
		},
		sqls[10]: []QueryIssue{},
		sqls[11]: []QueryIssue{
			NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", sqls[11]),
		},
	}

	parserIssueDetector := NewParserIssueDetector()
	for _, stmt := range ddlSqls {
		err := parserIssueDetector.ParseAndProcessDDL(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}

	// Finalize column metadata after processing all DDLs
	parserIssueDetector.FinalizeColumnMetadata()

	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)
		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}
func TestAggregateFunctions(t *testing.T) {
	sqls := []string{
		`SELECT
		department,
		any_value(employee_name) AS any_employee
	FROM employees
	GROUP BY department;`,
		`SELECT range_intersect_agg(multi_event_range) AS intersection_of_multiranges
FROM multiranges;`,
		`SELECT range_agg(multi_event_range) AS union_of_multiranges
FROM multiranges;`,
		`CREATE OR REPLACE FUNCTION aggregate_ranges()
RETURNS INT4MULTIRANGE AS $$
DECLARE
    aggregated_range INT4MULTIRANGE;
BEGIN
    SELECT range_agg(range_value) INTO aggregated_range FROM ranges;
	SELECT
		department,
		any_value(employee_name) AS any_employee
	FROM employees
	GROUP BY department;
    RETURN aggregated_range;
END;
$$ LANGUAGE plpgsql;`,
	}
	aggregateSqls := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewAnyValueAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[0]),
		},
		sqls[1]: []QueryIssue{
			NewRangeAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[1], []string{"range_intersect_agg"}),
		},
		sqls[2]: []QueryIssue{
			NewRangeAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[2], []string{"range_agg"}),
		},
		sqls[3]: []QueryIssue{
			NewRangeAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", "SELECT range_agg(range_value)                       FROM ranges;", []string{"range_agg"}),
			NewAnyValueAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[0]),
		},
	}
	aggregateSqls[sqls[3]] = modifiedIssuesforPLPGSQL(aggregateSqls[sqls[3]], "FUNCTION", "aggregate_ranges")

	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range aggregateSqls {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)
		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestRegexFunctionsIssue(t *testing.T) {
	dmlStmts := []string{
		`SELECT regexp_count('This is an example. Another example. Example is a common word.', 'example')`,
		`SELECT regexp_instr('This is an example. Another example. Example is a common word.', 'example')`,
		`SELECT regexp_like('This is an example. Another example. Example is a common word.', 'example')`,
		`SELECT regexp_count('abc','abc'), regexp_instr('abc','abc'), regexp_like('abc','abc')`,
	}

	ddlStmts := []string{
		`CREATE TABLE x (id INT PRIMARY KEY, id2 INT DEFAULT regexp_count('This is an example. Another example. Example is a common word.', 'example'))`,
	}

	parserIssueDetector := NewParserIssueDetector()

	for _, stmt := range dmlStmts {
		issues, err := parserIssueDetector.getDMLIssues(stmt)
		testutils.FatalIfError(t, err)
		assert.Equal(t, 1, len(issues))
		assert.Equal(t, NewRegexFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", stmt), issues[0])
	}

	for _, stmt := range ddlStmts {
		issues, err := parserIssueDetector.getDDLIssues(stmt)
		testutils.FatalIfError(t, err)
		assert.Equal(t, 1, len(issues))
		assert.Equal(t, NewRegexFunctionsIssue(TABLE_OBJECT_TYPE, "x", stmt), issues[0])
	}

}

func TestFetchWithTiesInSelect(t *testing.T) {

	stmt1 := `
	SELECT * FROM employees
		ORDER BY salary DESC
		FETCH FIRST 2 ROWS WITH TIES;`

	// subquery
	stmt2 := `SELECT *
	FROM (
		SELECT * FROM employees
		ORDER BY salary DESC
		FETCH FIRST 2 ROWS WITH TIES
	) AS top_employees;`

	stmt3 := `CREATE VIEW top_employees_view AS
		SELECT *
		FROM (
			SELECT * FROM employees
			ORDER BY salary DESC
			FETCH FIRST 2 ROWS WITH TIES
		) AS top_employees;`

	expectedIssues := map[string][]QueryIssue{
		stmt1: []QueryIssue{NewFetchWithTiesIssue("DML_QUERY", "", stmt1)},
		stmt2: []QueryIssue{NewFetchWithTiesIssue("DML_QUERY", "", stmt2)},
	}
	expectedDDLIssues := map[string][]QueryIssue{
		stmt3: []QueryIssue{NewFetchWithTiesIssue("VIEW", "top_employees_view", stmt3)},
	}

	parserIssueDetector := NewParserIssueDetector()

	for stmt, expectedIssues := range expectedIssues {
		issues, err := parserIssueDetector.GetDMLIssues(stmt, ybversion.LatestStable)

		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}

	for stmt, expectedIssues := range expectedDDLIssues {
		issues, err := parserIssueDetector.GetDDLIssues(stmt, ybversion.LatestStable)

		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestCopyUnsupportedConstructIssuesDetected(t *testing.T) {
	expectedIssues := map[string][]QueryIssue{
		`COPY my_table FROM '/path/to/data.csv' WHERE col1 > 100;`:                {NewCopyFromWhereIssue("DML_QUERY", "", `COPY my_table FROM '/path/to/data.csv' WHERE col1 > 100;`)},
		`COPY my_table(col1, col2) FROM '/path/to/data.csv' WHERE col2 = 'test';`: {NewCopyFromWhereIssue("DML_QUERY", "", `COPY my_table(col1, col2) FROM '/path/to/data.csv' WHERE col2 = 'test';`)},
		`COPY my_table FROM '/path/to/data.csv' WHERE TRUE;`:                      {NewCopyFromWhereIssue("DML_QUERY", "", `COPY my_table FROM '/path/to/data.csv' WHERE TRUE;`)},
		`COPY employees (id, name, age)
		FROM STDIN WITH (FORMAT csv)
		WHERE age > 30;`: {NewCopyFromWhereIssue("DML_QUERY", "", `COPY employees (id, name, age)
		FROM STDIN WITH (FORMAT csv)
		WHERE age > 30;`)},

		`COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE);`: {NewCopyOnErrorIssue("DML_QUERY", "", `COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE);`)},
		`COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR STOP);`:   {NewCopyOnErrorIssue("DML_QUERY", "", `COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR STOP);`)},

		`COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE) WHERE age > 18;`:     {NewCopyFromWhereIssue("DML_QUERY", "", `COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE) WHERE age > 18;`), NewCopyOnErrorIssue("DML_QUERY", "", `COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE) WHERE age > 18;`)},
		`COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR STOP) WHERE name = 'Alice';`: {NewCopyFromWhereIssue("DML_QUERY", "", `COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR STOP) WHERE name = 'Alice';`), NewCopyOnErrorIssue("DML_QUERY", "", `COPY table_name (name, age) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR STOP) WHERE name = 'Alice';`)},

		`COPY my_table FROM '/path/to/data.csv' WITH (FORMAT csv);`:                          {},
		`COPY my_table FROM '/path/to/data.csv' WITH (FORMAT text);`:                         {},
		`COPY my_table FROM '/path/to/data.csv';`:                                            {},
		`COPY my_table FROM '/path/to/data.csv' WITH (DELIMITER ',');`:                       {},
		`COPY my_table(col1, col2) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true);`: {},
	}

	parserIssueDetector := NewParserIssueDetector()

	for stmt, expectedIssues := range expectedIssues {
		issues, err := parserIssueDetector.getDMLIssues(stmt)
		testutils.FatalIfError(t, err)
		assert.Equal(t, len(expectedIssues), len(issues))

		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestForeignKeyReferencesPartitionedTableIssues(t *testing.T) {
	requiredDDLs := []string{
		`CREATE TABLE abc1(id int PRIMARY KEY, val text) PARTITION BY RANGE (id);`,
		`CREATE TABLE schema1.abc(id int PRIMARY KEY, val text) PARTITION BY RANGE (id);`,
	}
	stmt1 := `CREATE TABLE abc_fk(id int PRIMARY KEY, abc_id INT REFERENCES abc1(id), val text) ;`
	stmt2 := `ALTER TABLE schema1.abc_fk1
ADD CONSTRAINT fk FOREIGN KEY (abc1_id)
REFERENCES schema1.abc (id);
`
	stmt3 := `CREATE TABLE abc_fk (
    id INT PRIMARY KEY,
    abc_id INT,
    val TEXT,
    CONSTRAINT fk_abc FOREIGN KEY (abc_id) REFERENCES abc1(id)
);
`

	stmt4 := `CREATE TABLE schema1.abc_fk(id int PRIMARY KEY, abc_id INT, val text, FOREIGN KEY (abc_id) REFERENCES schema1.abc(id));`

	ddlStmtsWithIssues := map[string][]QueryIssue{
		stmt1: []QueryIssue{
			NewForeignKeyReferencesPartitionedTableIssue(TABLE_OBJECT_TYPE, "abc_fk", stmt1, "abc_fk_abc_id_fkey"),
		},
		stmt2: []QueryIssue{
			NewForeignKeyReferencesPartitionedTableIssue(TABLE_OBJECT_TYPE, "schema1.abc_fk1", stmt2, "fk"),
		},
		stmt3: []QueryIssue{
			NewForeignKeyReferencesPartitionedTableIssue(TABLE_OBJECT_TYPE, "abc_fk", stmt3, "fk_abc"),
		},
		stmt4: []QueryIssue{
			NewForeignKeyReferencesPartitionedTableIssue(TABLE_OBJECT_TYPE, "schema1.abc_fk", stmt4, "abc_fk_abc_id_fkey"),
		},
	}
	parserIssueDetector := NewParserIssueDetector()
	for _, stmt := range requiredDDLs {
		err := parserIssueDetector.ParseAndProcessDDL(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}

	parserIssueDetector.FinalizeColumnMetadata()

	for stmt, expectedIssues := range ddlStmtsWithIssues {
		issues, err := parserIssueDetector.GetDDLIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestNonDecimalIntegerLiteralsIssues(t *testing.T) {
	sql1 := `SELECT 5678901234, 0x1527D27F2 as hex;`
	sql2 := `SELECT 5678901234, 0o52237223762 as octal;`
	sql3 := `SELECT 5678901234, 0b101010010011111010010011111110010 as binary;`
	sql4 := `CREATE VIEW zz AS
    SELECT
        5678901234 AS DEC,
        0x1527D27F2 AS hex,
        0o52237223762 AS oct,
        0b10101001001111101001001111111`
	sql5 := `SELECT 5678901234, 0O52237223762 as octal;` // captial "0O" case
	sqls := map[string]QueryIssue{
		sql1: NewNonDecimalIntegerLiteralIssue("DML_QUERY", "", sql1),
		sql2: NewNonDecimalIntegerLiteralIssue("DML_QUERY", "", sql2),
		sql3: NewNonDecimalIntegerLiteralIssue("DML_QUERY", "", sql3),
		sql4: NewNonDecimalIntegerLiteralIssue("VIEW", "zz", sql4),
		sql5: NewNonDecimalIntegerLiteralIssue("DML_QUERY", "", sql5),
	}
	parserIssueDetector := NewParserIssueDetector()
	for sql, expectedIssue := range sqls {
		issues, err := parserIssueDetector.GetAllIssues(sql, ybversion.LatestStable)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(issues))
		cmp.Equal(issues[0], expectedIssue)
	}
	sqlsWithoutIssues := []string{
		`SELECT 1234, '0x4D2';`,    //string constant starting with 0x
		`SELECT $1, $2 as binary;`, // parameterised strings for constant data
		//DEFAULT and check constraints are not reported because parse tree doesn't the info of non-decimal integer literal usage as it converts it to decimal
		`CREATE TABLE bitwise_example (
    id SERIAL PRIMARY KEY,
    flags INT DEFAULT 0x0F CHECK (flags & 0x01 = 0x01) -- Hexadecimal bitwise check
);`,
		`CREATE TABLE bin_default(id int, bin_int int DEFAULT 0b1010010101 CHECK (bin_int<>0b1000010101));`,
		/*
		   similarly this insert is also can't be reported as parser changes them to decimal integers while giving parseTree
		   insert_stmt:{relation:{relname:"non_decimal_table" inh:true relpersistence:"p" location:12} cols:{res_target:{name:"binary_value" ...
		   select_stmt:{select_stmt:{values_lists:{list:{items:{a_const:{ival:{ival:10} location:81}} items:{a_const:{ival:{ival:10} location:89}} items:{a_const:{ival:{ival:10} location:96}}}}
		*/
		`INSERT INTO non_decimal_table (binary_value, octal_value, hex_value)
    VALUES (0b1010, 0o012, 0xA);`,
	}
	for _, sql := range sqlsWithoutIssues {
		issues, err := parserIssueDetector.GetAllIssues(sql, ybversion.LatestStable)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(issues))
	}
}
func TestCTEIssues(t *testing.T) {
	sqls := []string{
		`WITH w AS (
    SELECT key, very_expensive_function(val) as f FROM some_table
)
SELECT * FROM w AS w1 JOIN w AS w2 ON w1.f = w2.f;`,
		`WITH w AS NOT MATERIALIZED (
    SELECT * FROM big_table
)
SELECT * FROM w AS w1 JOIN w AS w2 ON w1.key = w2.ref
WHERE w2.key = 123;`,
		`WITH moved_rows AS MATERIALIZED (
    DELETE FROM products
    WHERE
        "date" >= '2010-10-01' AND
        "date" < '2010-11-01'
    RETURNING *
)
INSERT INTO products_log
SELECT * FROM moved_rows;`,
		`CREATE VIEW view1 AS
WITH data_cte AS NOT MATERIALIZED (
    SELECT 
        generate_series(1, 5) AS id,
        'Name ' || generate_series(1, 5) AS name
)
SELECT * FROM data_cte;`,
		`CREATE VIEW view2 AS
WITH data_cte AS MATERIALIZED (
    SELECT 
        generate_series(1, 5) AS id,
        'Name ' || generate_series(1, 5) AS name
)
SELECT * FROM data_cte;`,
		`CREATE VIEW view3 AS
WITH data_cte AS (
    SELECT 
        generate_series(1, 5) AS id,
        'Name ' || generate_series(1, 5) AS name
)
SELECT * FROM data_cte;`,
	}

	stmtsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{},
		sqls[1]: []QueryIssue{
			NewCTEWithMaterializedIssue(DML_QUERY_OBJECT_TYPE, "", sqls[1]),
		},
		sqls[2]: []QueryIssue{
			NewCTEWithMaterializedIssue(DML_QUERY_OBJECT_TYPE, "", sqls[2]),
		},
		sqls[3]: []QueryIssue{
			NewCTEWithMaterializedIssue("VIEW", "view1", sqls[3]),
		},
		sqls[4]: []QueryIssue{
			NewCTEWithMaterializedIssue("VIEW", "view2", sqls[4]),
		},
		sqls[5]: []QueryIssue{},
	}

	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}

}

func TestSQLBodyIssues(t *testing.T) {
	sqls := []string{
		`CREATE OR REPLACE FUNCTION asterisks(n int)
  RETURNS text
  LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
RETURN repeat('*', n);`,
		`CREATE OR REPLACE FUNCTION asterisks(n int)
  RETURNS SETOF text
  LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
BEGIN ATOMIC
SELECT repeat('*', g) FROM generate_series (1, n) g;
END;`,
		`CREATE OR REPLACE FUNCTION asterisks(n int)
  RETURNS SETOF text
  LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS
$func$
SELECT repeat('*', g) FROM generate_series (1, n) g;
$func$;`,
	}

	stmtsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewSqlBodyInFunctionIssue("FUNCTION", "asterisks", sqls[0]),
		},
		sqls[1]: []QueryIssue{
			NewSqlBodyInFunctionIssue("FUNCTION", "asterisks", sqls[1]),
		},
		sqls[2]: []QueryIssue{},
	}
	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetDDLIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestDatabaseOptions(t *testing.T) {
	sqls := []string{
		` CREATE DATABASE locale_example
    WITH LOCALE = 'en_US.UTF-8'
         TEMPLATE = template0;`,
		`CREATE DATABASE locale_provider_example
    WITH ICU_LOCALE = 'en_US'
         LOCALE_PROVIDER = 'icu'
         TEMPLATE = template0;`,
		`CREATE DATABASE oid_example
    WITH OID = 123456;`,
		`CREATE DATABASE collation_version_example
    WITH COLLATION_VERSION = '153.128';`,
		`CREATE DATABASE icu_rules_example
    WITH ICU_RULES = '&a < b < c';`,
		`CREATE DATABASE builtin_locale_example
    WITH BUILTIN_LOCALE = 'C';`,
		`CREATE DATABASE strategy_example
    WITH STRATEGY = 'wal_log';`,
	}
	stmtsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewDatabaseOptionsPG15Issue("DATABASE", "locale_example", sqls[0], []string{"locale"}),
		},
		sqls[1]: []QueryIssue{
			NewDatabaseOptionsPG15Issue("DATABASE", "locale_provider_example", sqls[1], []string{"icu_locale", "locale_provider"}),
		},
		sqls[2]: []QueryIssue{
			NewDatabaseOptionsPG15Issue("DATABASE", "oid_example", sqls[2], []string{"oid"}),
		},
		sqls[3]: []QueryIssue{
			NewDatabaseOptionsPG15Issue("DATABASE", "collation_version_example", sqls[3], []string{"collation_version"}),
		},
		sqls[4]: []QueryIssue{
			NewDatabaseOptionsPG17Issue("DATABASE", "icu_rules_example", sqls[4], []string{"icu_rules"}),
		},
		sqls[5]: []QueryIssue{
			NewDatabaseOptionsPG17Issue("DATABASE", "builtin_locale_example", sqls[5], []string{"builtin_locale"}),
		},
		sqls[6]: []QueryIssue{
			NewDatabaseOptionsPG15Issue("DATABASE", "strategy_example", sqls[6], []string{"strategy"}),
		},
	}
	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}
func TestListenNotifyIssues(t *testing.T) {
	sqls := []string{
		`LISTEN my_table_changes;`,
		`NOTIFY my_table_changes, 'Row inserted: id=1, name=Alice';`,
		`UNLISTEN my_notification;`,
		`SELECT pg_notify('my_notification', 'Payload from pg_notify');`,
		`CREATE OR REPLACE FUNCTION notify_on_insert()
RETURNS TRIGGER AS $$
BEGIN
  PERFORM pg_notify('row_inserted', 'New row added with id: ' || NEW.id);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;`,
		`CREATE OR REPLACE FUNCTION notify_and_insert()
RETURNS VOID AS $$
BEGIN
	LISTEN my_table_changes;
    INSERT INTO my_table (name) VALUES ('Charlie');
	NOTIFY my_table_changes, 'New row added with name: Charlie';
    PERFORM pg_notify('my_table_changes', 'New row added with name: Charlie');
	UNLISTEN my_table_changes;
END;
$$ LANGUAGE plpgsql;`,
	}

	stmtsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewListenNotifyIssue(DML_QUERY_OBJECT_TYPE, "", sqls[0]),
		},
		sqls[1]: []QueryIssue{
			NewListenNotifyIssue(DML_QUERY_OBJECT_TYPE, "", sqls[1]),
		},
		sqls[2]: []QueryIssue{
			NewListenNotifyIssue(DML_QUERY_OBJECT_TYPE, "", sqls[2]),
		},
		sqls[3]: []QueryIssue{
			NewListenNotifyIssue(DML_QUERY_OBJECT_TYPE, "", sqls[3]),
		},
		sqls[4]: []QueryIssue{
			NewListenNotifyIssue(FUNCTION_OBJECT_TYPE, "notify_on_insert", "SELECT pg_notify('row_inserted', 'New row added with id: ' || NEW.id);"),
		},
		sqls[5]: []QueryIssue{
			NewListenNotifyIssue(FUNCTION_OBJECT_TYPE, "notify_and_insert", "LISTEN my_table_changes;"),
			NewListenNotifyIssue(FUNCTION_OBJECT_TYPE, "notify_and_insert", "NOTIFY my_table_changes, 'New row added with name: Charlie';"),
			NewListenNotifyIssue(FUNCTION_OBJECT_TYPE, "notify_and_insert", "SELECT pg_notify('my_table_changes', 'New row added with name: Charlie');"),
			NewListenNotifyIssue(FUNCTION_OBJECT_TYPE, "notify_and_insert", "UNLISTEN my_table_changes;"),
		},
	}
	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}

func TestTwoPhaseCommit(t *testing.T) {
	sqls := []string{
		`PREPARE TRANSACTION 'tx1';`,
		`CREATE OR REPLACE PROCEDURE transfer_money(sender_id INT, receiver_id INT, amount NUMERIC)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Deduct amount from sender in db1
    UPDATE accounts SET balance = balance - amount WHERE id = sender_id;
    PREPARE TRANSACTION 'txn_db1';

    -- Insert transaction record in db2
    PERFORM dblink_exec('dbname=db2 user=postgres password=your_password',
        'INSERT INTO transactions (sender_id, receiver_id, amount) 
         VALUES (' || sender_id || ', ' || receiver_id || ', ' || amount || ');
         PREPARE TRANSACTION ''txn_db2'';');

    -- Commit both transactions
    EXECUTE 'COMMIT PREPARED ''txn_db1''';

    PERFORM dblink_exec('dbname=db2 user=postgres password=your_password', 
        'COMMIT PREPARED ''txn_db2'';');

    RAISE NOTICE 'Transaction committed successfully';

EXCEPTION
    WHEN OTHERS THEN
        -- Rollback in case of failure
        EXECUTE 'ROLLBACK PREPARED ''txn_db1''';
        PERFORM dblink_exec('dbname=db2 user=postgres password=your_password', 
            'ROLLBACK PREPARED ''txn_db2'';');
        RAISE EXCEPTION 'Transaction failed: %', SQLERRM;
END;
$$;`,
	}

	stmtsWithExpectedIssues := map[string][]QueryIssue{
		sqls[0]: []QueryIssue{
			NewTwoPhaseCommitIssue(DML_QUERY_OBJECT_TYPE, "", sqls[0]),
		},
		sqls[1]: []QueryIssue{
			NewTwoPhaseCommitIssue("PROCEDURE", "transfer_money", "PREPARE TRANSACTION 'txn_db1';"),
			NewTwoPhaseCommitIssue("PROCEDURE", "transfer_money", "COMMIT PREPARED 'txn_db1';"),
			NewTwoPhaseCommitIssue("PROCEDURE", "transfer_money", "ROLLBACK PREPARED 'txn_db1';"),
		},
	}
	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}

	_, err := parserIssueDetector.GetAllIssues(`PREPARE TRANSACTION $1`, ybversion.LatestStable)
	assert.Error(t, err, `syntax error at or near "$1"`)

}

func TestCompressionClause(t *testing.T) {
	stmts := []string{
		`CREATE TABLE tbl_comp1(id int, v text COMPRESSION pglz);`,
		`ALTER TABLE ONLY public.tbl_comp ALTER COLUMN v SET COMPRESSION pglz;`,
	}
	sqlsWithExpectedIssues := map[string][]QueryIssue{
		stmts[0]: []QueryIssue{
			NewCompressionClauseForToasting("TABLE", "tbl_comp1", stmts[0]),
		},
		stmts[1]: []QueryIssue{
			NewCompressionClauseForToasting("TABLE", "public.tbl_comp", stmts[1]),
		},
	}
	parserIssueDetector := NewParserIssueDetector()
	for stmt, expectedIssues := range sqlsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)
		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}

}

func TestTimestampOrDateHotspotsIssues(t *testing.T) {
	stmts := []string{
		`CREATE TABLE test(id int PRIMARY KEY, created_at timestamp with time zone, val text, ordered_date date);`,
		`CREATE INDEX idx_val on test(val); `,
		`CREATE INDEX idx_id_created_at on test(id, created_at);`,
		`CREATE INDEX idx_created_at on test(created_at);`,
		`CREATE INDEX idx_id_created_at1 on test(id, created_at ASC);`,
		`CREATE INDEX idx_created_at_ordered_date on test(created_at, ordered_date);`,
		`CREATE INDEX idx_id_created_at_ordered_date on test(id, created_at, ordered_date);`,
		`CREATE TABLE test_pk_timestamp(id int, created_at timestamp PRIMARY KEY);`,
		`CREATE TABLE test_pk_second_col_timestamp(id int, created_at timestamp with time zone, PRIMARY KEY(id, created_at));`,
		`CREATE TABLE test_pk_timestamp_alter(id int, createdat date);`,
		`ALTER TABLE test_pk_timestamp_alter ADD CONSTRAINT pk1 PRIMARY KEY(createdat, id);`,
		`ALTER TABLE test_pk_timestamp_alter ADD CONSTRAINT pk2 PRIMARY KEY(id, createdat);`,
		`CREATE TABLE schema2.test_uk_timestamp(id int, created_at timestamp UNIQUE);`,
		`CREATE TABLE test_uk_second_col_timestamp(id int, created_at timestamp with time zone, UNIQUE(id, created_at));`,
		`CREATE TABLE "Test_uk_timestamp_alter"(id int, createdat date);`,
		`ALTER TABLE "Test_uk_timestamp_alter" ADD CONSTRAINT pk1 UNIQUE(createdat, id);`,
		`ALTER TABLE "Test_uk_timestamp_alter" ADD CONSTRAINT pk2 UNIQUE(id, createdat);`,
	}
	sqlsWithExpectedIssues := map[string][]QueryIssue{
		stmts[1]: []QueryIssue{},
		stmts[2]: []QueryIssue{},
		stmts[3]: []QueryIssue{
			NewHotspotOnTimestampIndexIssue(INDEX_OBJECT_TYPE, "idx_created_at ON test", stmts[3], "created_at"),
		},
		stmts[4]: []QueryIssue{},
		stmts[5]: []QueryIssue{
			NewHotspotOnTimestampIndexIssue(INDEX_OBJECT_TYPE, "idx_created_at_ordered_date ON test", stmts[5], "created_at"),
		},
		stmts[6]: []QueryIssue{},
		stmts[7]: []QueryIssue{
			NewHotspotOnTimestampPKOrUKIssue(TABLE_OBJECT_TYPE, "test_pk_timestamp", stmts[7], "created_at"),
		},
		stmts[8]: []QueryIssue{},
		stmts[10]: []QueryIssue{
			NewHotspotOnDatePKOrUKIssue(TABLE_OBJECT_TYPE, "test_pk_timestamp_alter", stmts[10], "createdat"),
		},
		stmts[11]: []QueryIssue{},
		stmts[12]: []QueryIssue{
			NewHotspotOnTimestampPKOrUKIssue(TABLE_OBJECT_TYPE, "schema2.test_uk_timestamp", stmts[12], "created_at"),
		},
		stmts[13]: []QueryIssue{},
		stmts[15]: []QueryIssue{
			NewHotspotOnDatePKOrUKIssue(TABLE_OBJECT_TYPE, "Test_uk_timestamp_alter", stmts[15], "createdat"),
		},
		stmts[16]: []QueryIssue{},
	}
	parserIssueDetector := NewParserIssueDetector()
	statementsToParseFirst := []string{
		stmts[0],
		stmts[7],
		stmts[8],
		stmts[9],
		stmts[12],
		stmts[14],
	}
	for _, stmt := range statementsToParseFirst {
		err := parserIssueDetector.ParseAndProcessDDL(stmt)
		assert.Nil(t, err)
	}

	parserIssueDetector.FinalizeColumnMetadata()

	for stmt, expectedIssues := range sqlsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)
		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(queryIssue QueryIssue) bool {
				return cmp.Equal(expectedIssue, queryIssue)
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}
}
