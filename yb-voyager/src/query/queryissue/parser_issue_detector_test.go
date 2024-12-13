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
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
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
)

func modifyiedIssuesforPLPGSQL(issues []QueryIssue, objType string, objName string) []QueryIssue {
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
			NewSystemColumnsIssue("DML_QUERY", "", "SELECT * FROM employees e WHERE e.xmax = (SELECT MAX(xmax) FROM employees WHERE department = e.department);"),
		},
		stmt2: []QueryIssue{
			NewPercentTypeSyntaxIssue("FUNCTION", "process_order", "orders.id%TYPE"),
			NewStorageParameterIssue("TABLE", "public.example", "ALTER TABLE ONLY public.example ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor=70);"),
			NewUnloggedTableIssue("TABLE", "tbl_unlog", "CREATE UNLOGGED TABLE tbl_unlog (id int, val text);"),
			NewMultiColumnGinIndexIssue("INDEX", "idx_example ON example_table", "CREATE INDEX idx_example ON example_table USING gin(name, name1);"),
			NewUnsupportedIndexMethodIssue("INDEX", "idx_example ON schema1.example_table", "CREATE INDEX idx_example ON schema1.example_table USING gist(name);", "gist"),
			NewAdvisoryLocksIssue("DML_QUERY", "", "SELECT pg_advisory_unlock(orderid);"),
		},
		stmt3: []QueryIssue{
			NewStorageParameterIssue("INDEX", "abc ON public.example", stmt3),
		},
		stmt4: []QueryIssue{
			NewDisableRuleIssue("TABLE", "public.example", stmt4, "example_rule"),
		},
		stmt5: []QueryIssue{
			NewDeferrableConstraintIssue("TABLE", "abc, constraint: (cnstr_id)", stmt5),
		},
		stmt6: []QueryIssue{
			NewAdvisoryLocksIssue("DML_QUERY", "", stmt6),
		},
		stmt7: []QueryIssue{
			NewSystemColumnsIssue("DML_QUERY", "", stmt7),
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
			NewExclusionConstraintIssue("TABLE", "Test, constraint: (Test_room_id_time_range_excl)", stmt11),
			NewExclusionConstraintIssue("TABLE", "Test, constraint: (no_time_overlap_constr)", stmt11),
		},
		stmt13: []QueryIssue{
			NewIndexOnComplexDatatypesIssue("INDEX", "idx_on_daterange ON test_dt", stmt13, "daterange"),
		},
	}

	//Should modify it in similar way we do it actual code as the particular DDL issue in plpgsql can have different Details map on the basis of objectType
	stmtsWithExpectedIssues[stmt1] = modifyiedIssuesforPLPGSQL(stmtsWithExpectedIssues[stmt1], "FUNCTION", "list_high_earners")

	stmtsWithExpectedIssues[stmt2] = modifyiedIssuesforPLPGSQL(stmtsWithExpectedIssues[stmt2], "FUNCTION", "process_order")

	for _, stmt := range requiredDDLs {
		err := parserIssueDetector.ParseRequiredDDLs(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}
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
	requiredDDLs := []string{stmt16}
	parserIssueDetector := NewParserIssueDetector()
	stmtsWithExpectedIssues := map[string][]QueryIssue{
		stmt14: []QueryIssue{
			NewAdvisoryLocksIssue("MVIEW", "public.sample_data_view", stmt14),
			NewSystemColumnsIssue("MVIEW", "public.sample_data_view", stmt14),
			NewXmlFunctionsIssue("MVIEW", "public.sample_data_view", stmt14),
		},
		stmt15: []QueryIssue{
			NewAdvisoryLocksIssue("VIEW", "public.orders_view", stmt15),
			NewSystemColumnsIssue("VIEW", "public.orders_view", stmt15),
			NewXmlFunctionsIssue("VIEW", "public.orders_view", stmt15),
			//TODO: Add CHECK OPTION issue when we move it from regex to parser logic
		},
		stmt16: []QueryIssue{
			NewXmlFunctionsIssue("TABLE", "public.xml_data_example", stmt16),
			NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue("TABLE", "public.xml_data_example, constraint: (xml_data_example_d_key)", stmt16, "daterange", true),
			NewMultiColumnListPartition("TABLE", "public.xml_data_example", stmt16),
			NewInsufficientColumnInPKForPartition("TABLE", "public.xml_data_example", stmt16, []string{"name"}),
			NewXMLDatatypeIssue("TABLE", "public.xml_data_example", stmt16, "description"),
		},
		stmt17: []QueryIssue{
			NewXmlFunctionsIssue("TABLE", "invoices", stmt17),
		},
		stmt18: []QueryIssue{
			NewXmlFunctionsIssue("INDEX", "idx_invoices ON invoices", stmt18),
		},
	}
	for _, stmt := range requiredDDLs {
		err := parserIssueDetector.ParseRequiredDDLs(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}
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
