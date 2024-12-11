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

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
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
)

func TestAllDDLIssues(t *testing.T) {
	requiredDDLs := []string{stmt12}
	parserIssueDetector := NewParserIssueDetector()
	stmtsWithExpectedIssues := map[string][]issue.IssueInstance{
		stmt1: []issue.IssueInstance{
			issue.NewPercentTypeSyntaxIssue("FUNCTION", "list_high_earners", "public.emp1.salary%TYPE"),
			issue.NewPercentTypeSyntaxIssue("FUNCTION", "list_high_earners", "employees.name%TYPE"),
			issue.NewPercentTypeSyntaxIssue("FUNCTION", "list_high_earners", "employees.salary%TYPE"),
			issue.NewClusterONIssue("FUNCTION", "list_high_earners", "ALTER TABLE employees CLUSTER ON idx;"),
			issue.NewAdvisoryLocksIssue("FUNCTION", "list_high_earners", "SELECT pg_advisory_unlock(sender_id);"),
			issue.NewAdvisoryLocksIssue("FUNCTION", "list_high_earners", "SELECT pg_advisory_unlock(receiver_id);"),
			issue.NewXmlFunctionsIssue("FUNCTION", "list_high_earners", "SELECT id, xpath('/person/name/text()', data) AS name FROM test_xml_type;"),
			issue.NewSystemColumnsIssue("FUNCTION", "list_high_earners", "SELECT * FROM employees e WHERE e.xmax = (SELECT MAX(xmax) FROM employees WHERE department = e.department);"),
		},
		stmt2: []issue.IssueInstance{
			issue.NewPercentTypeSyntaxIssue("FUNCTION", "process_order", "orders.id%TYPE"),
			issue.NewStorageParameterIssue("FUNCTION", "process_order", "ALTER TABLE ONLY public.example ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor=70);"),
			issue.NewUnloggedTableIssue("FUNCTION", "process_order", "CREATE UNLOGGED TABLE tbl_unlog (id int, val text);"),
			issue.NewMultiColumnGinIndexIssue("FUNCTION", "process_order", "CREATE INDEX idx_example ON example_table USING gin(name, name1);"),
			issue.NewUnsupportedIndexMethodIssue("FUNCTION", "process_order", "CREATE INDEX idx_example ON schema1.example_table USING gist(name);", "gist"),
			issue.NewAdvisoryLocksIssue("FUNCTION", "process_order", "SELECT pg_advisory_unlock(orderid);"),
		},
		stmt3: []issue.IssueInstance{
			issue.NewStorageParameterIssue("INDEX", "abc ON public.example", stmt3),
		},
		stmt4: []issue.IssueInstance{
			issue.NewDisableRuleIssue("TABLE", "public.example", stmt4, "example_rule"),
		},
		stmt5: []issue.IssueInstance{
			issue.NewDeferrableConstraintIssue("TABLE", "abc, constraint: (cnstr_id)", stmt5),
		},
		stmt6: []issue.IssueInstance{
			issue.NewAdvisoryLocksIssue("DML_QUERY", "", stmt6),
		},
		stmt7: []issue.IssueInstance{
			issue.NewSystemColumnsIssue("DML_QUERY", "", stmt7),
		},
		stmt8: []issue.IssueInstance{
			issue.NewXmlFunctionsIssue("DML_QUERY", "", stmt8),
		},
		stmt9: []issue.IssueInstance{
			issue.NewGeneratedColumnsIssue("TABLE", "order_details", stmt9, []string{"amount"}),
		},
		stmt10: []issue.IssueInstance{
			issue.NewMultiColumnListPartition("TABLE", "test_non_pk_multi_column_list", stmt10),
			issue.NewInsufficientColumnInPKForPartition("TABLE", "test_non_pk_multi_column_list", stmt10, []string{"country_code", "record_type"}),
		},
		stmt11: []issue.IssueInstance{
			issue.NewExclusionConstraintIssue("TABLE", "Test, constraint: (Test_room_id_time_range_excl)", stmt11),
			issue.NewExclusionConstraintIssue("TABLE", "Test, constraint: (no_time_overlap_constr)", stmt11),
		},
		stmt13: []issue.IssueInstance{
			issue.NewIndexOnComplexDatatypesIssue("INDEX", "idx_on_daterange ON test_dt", stmt13, "daterange"),
		},
	}

	for _, stmt := range requiredDDLs {
		err := parserIssueDetector.ParseRequiredDDLs(stmt)
		assert.NoError(t, err, "Error parsing required ddl: %s", stmt)
	}
	for stmt, expectedIssues := range stmtsWithExpectedIssues {
		issues, err := parserIssueDetector.GetAllIssues(stmt, ybversion.LatestStable)
		assert.NoError(t, err, "Error detecting issues for statement: %s", stmt)

		assert.Equal(t, len(expectedIssues), len(issues), "Mismatch in issue count for statement: %s", stmt)
		for _, expectedIssue := range expectedIssues {
			found := slices.ContainsFunc(issues, func(issueInstance issue.IssueInstance) bool {
				typeNameMatches := issueInstance.TypeName == expectedIssue.TypeName
				queryMatches := issueInstance.SqlStatement == expectedIssue.SqlStatement
				objectNameMatches := issueInstance.ObjectName == expectedIssue.ObjectName
				objectTypeMatches := issueInstance.ObjectType == expectedIssue.ObjectType
				return typeNameMatches && queryMatches && objectNameMatches && objectTypeMatches
			})
			assert.True(t, found, "Expected issue not found: %v in statement: %s", expectedIssue, stmt)
		}
	}

}
