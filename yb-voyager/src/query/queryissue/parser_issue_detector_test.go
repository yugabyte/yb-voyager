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
			NewSystemColumnsIssue("DML_QUERY", "", "SELECT * FROM employees e WHERE e.xmax = (SELECT MAX(xmax) FROM employees WHERE department = e.department);"),
		},
		stmt2: []QueryIssue{
			NewPercentTypeSyntaxIssue("FUNCTION", "process_order", "orders.id%TYPE"),
			NewStorageParameterIssue("TABLE", "public.example", "ALTER TABLE ONLY public.example ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor=70);"),
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
			NewDeferrableConstraintIssue("TABLE", "abc", stmt5, "cnstr_id"),
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
			NewExclusionConstraintIssue("TABLE", "Test", stmt11, "Test_room_id_time_range_excl"),
			NewExclusionConstraintIssue("TABLE", "Test", stmt11, "no_time_overlap_constr"),
		},
		stmt13: []QueryIssue{
			NewIndexOnComplexDatatypesIssue("INDEX", "idx_on_daterange ON test_dt", stmt13, "daterange"),
		},
	}

	//Should modify it in similar way we do it actual code as the particular DDL issue in plpgsql can have different Details map on the basis of objectType
	stmtsWithExpectedIssues[stmt1] = modifiedIssuesforPLPGSQL(stmtsWithExpectedIssues[stmt1], "FUNCTION", "list_high_earners")

	stmtsWithExpectedIssues[stmt2] = modifiedIssuesforPLPGSQL(stmtsWithExpectedIssues[stmt2], "FUNCTION", "process_order")

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
			NewPrimaryOrUniqueConsOnUnsupportedIndexTypesIssue("TABLE", "public.xml_data_example", stmt16, "daterange", "xml_data_example_d_key"),
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
		stmt19: []QueryIssue{
			NewLODatatypeIssue("TABLE", "test_lo_default", stmt19, "raster"),
			NewLOFuntionsIssue("TABLE", "test_lo_default", stmt19, []string{"lo_import"}),
		},
		stmt20: []QueryIssue{
			NewSecurityInvokerViewIssue("VIEW", "public.view_explicit_security_invoker", stmt20),
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

func TestUnloggedTableIssueReportedInOlderVersion(t *testing.T) {
	stmt := "CREATE UNLOGGED TABLE tbl_unlog (id int, val text);"
	parserIssueDetector := NewParserIssueDetector()

	// Not reported by default
	issues, err := parserIssueDetector.GetDDLIssues(stmt, ybversion.LatestStable)
	fatalIfError(t, err)
	assert.Equal(t, 0, len(issues))

	// older version should report the issue
	issues, err = parserIssueDetector.GetDDLIssues(stmt, ybversion.V2024_1_0_0)
	fatalIfError(t, err)
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
			NewLODatatypeIssue("TABLE", "test_large_objects", "CREATE TABLE IF NOT EXISTS test_large_objects(id INT, raster lo DEFAULT lo_import(3242));", "raster"),
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
	fatalIfError(t, err)
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
			NewJsonQueryFunctionIssue("MVIEW", "public.test_jsonb_view", sqls[13], []string{JSON_VALUE, JSON_EXISTS, JSON_TABLE}),
		},
		sqls[14]: []QueryIssue{
			NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", sqls[14], []string{JSON_ARRAY}),
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
		fatalIfError(t, err)
		assert.Equal(t, 1, len(issues))
		assert.Equal(t, NewRegexFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", stmt), issues[0])
	}

	for _, stmt := range ddlStmts {
		issues, err := parserIssueDetector.getDDLIssues(stmt)
		fatalIfError(t, err)
		assert.Equal(t, 1, len(issues))
		assert.Equal(t, NewRegexFunctionsIssue(TABLE_OBJECT_TYPE, "x", stmt), issues[0])
	}

}
