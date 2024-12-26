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
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
)

func getDetectorIssues(t *testing.T, detector UnsupportedConstructDetector, sql string) []QueryIssue {
	parseResult, err := queryparser.Parse(sql)
	assert.NoError(t, err, "Failed to parse SQL: %s", sql)

	visited := make(map[protoreflect.Message]bool)

	processor := func(msg protoreflect.Message) error {
		err := detector.Detect(msg)
		if err != nil {
			return err
		}
		return nil
	}

	parseTreeMsg := queryparser.GetProtoMessageFromParseTree(parseResult)
	err = queryparser.TraverseParseTree(parseTreeMsg, visited, processor)
	assert.NoError(t, err)
	return detector.GetIssues()
}

func TestFuncCallDetector(t *testing.T) {
	advisoryLockSqls := []string{
		`SELECT pg_advisory_lock(100), COUNT(*) FROM cars;`,
		`SELECT pg_advisory_lock_shared(100), COUNT(*) FROM cars;`,
		`SELECT pg_advisory_unlock_shared(100);`,
		`SELECT * FROM (SELECT pg_advisory_xact_lock(200)) AS lock_acquired;`,
		`SELECT * FROM (SELECT pg_advisory_xact_lock_shared(200)) AS lock_acquired;`,
		`SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(300) IS TRUE;`,
		`SELECT id, first_name FROM employees WHERE salary > 400 AND EXISTS (SELECT 1 FROM pg_advisory_lock(500));`,
		`SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(600) IS TRUE AND salary > 700;`,
		`SELECT pg_try_advisory_lock_shared(1234, 100);`,
		`SELECT pg_try_advisory_xact_lock_shared(1,2);`,
		`WITH lock_cte AS (
            SELECT pg_advisory_lock(1000) AS lock_acquired
        )
        SELECT e.id, e.name
        FROM employees e
        JOIN lock_cte ON TRUE
        WHERE e.department = 'Engineering';`,
		`SELECT e.id, e.name
        FROM employees e
        WHERE EXISTS (
            SELECT 1
            FROM projects p
            WHERE p.manager_id = e.id
            AND pg_try_advisory_lock_shared(p.project_id)
        );`,
		`SELECT e.id,
            CASE
                WHEN e.salary > 100000 THEN pg_advisory_lock(e.id)
                ELSE pg_advisory_unlock(e.id)
            END AS lock_status
        FROM employees e;`,
		`SELECT e.id, l.lock_status
        FROM employees e
        JOIN LATERAL (
            SELECT pg_try_advisory_lock(e.id) AS lock_status
        ) l ON TRUE
        WHERE e.status = 'active';`,
		`WITH lock_cte AS (
            SELECT 1
        )
        SELECT e.id, e.name, pg_try_advisory_lock(600)
        FROM employees e
        JOIN lock_cte ON TRUE
        WHERE pg_advisory_unlock(500) = TRUE;`,
		`SELECT pg_advisory_unlock_all();`,
	}

	anyValAggSqls := []string{
		`SELECT
		department,
		any_value(employee_name) AS any_employee
	FROM employees
	GROUP BY department;`,
	}
	loFunctionSqls := []string{
		`UPDATE documents
SET content_oid = lo_import('/path/to/new/file.pdf')
WHERE title = 'Sample Document';`,
		`INSERT INTO documents (title, content_oid)
VALUES ('Sample Document', lo_import('/path/to/your/file.pdf'));`,
		`SELECT lo_export(content_oid, '/path/to/exported_design_document.pdf')
FROM documents
WHERE title = 'Design Document';`,
		`SELECT lo_create('32142');`,
		`SELECT  lo_unlink(loid);`,
		`SELECT lo_unlink((SELECT content_oid FROM documents WHERE title = 'Sample Document'));`,
		`create table test_lo_default (id int, raster lo DEFAULT lo_import('3242'));`,
	}
	for _, sql := range advisoryLockSqls {

		issues := getDetectorIssues(t, NewFuncCallDetector(sql), sql)
		assert.Equal(t, 1, len(issues), "Expected 1 issue for SQL: %s", sql)
		assert.Equal(t, ADVISORY_LOCKS, issues[0].Type, "Expected Advisory Locks issue for SQL: %s", sql)
	}

	for _, sql := range loFunctionSqls {
		issues := getDetectorIssues(t, NewFuncCallDetector(sql), sql)
		assert.Equal(t, len(issues), 1)
		assert.Equal(t, issues[0].Type, LARGE_OBJECT_FUNCTIONS, "Large Objects not detected in SQL: %s", sql)
	}

	for _, sql := range anyValAggSqls {
		issues := getDetectorIssues(t, NewFuncCallDetector(sql), sql)
		assert.Equal(t, 1, len(issues), "Expected 1 issue for SQL: %s", sql)
		assert.Equal(t, AGGREGATE_FUNCTION, issues[0].Type, "Expected Advisory Locks issue for SQL: %s", sql)
	}
}

func TestColumnRefDetector(t *testing.T) {
	systemColumnSqls := []string{
		`SELECT xmin, xmax FROM employees;`,
		`SELECT * FROM (SELECT * FROM employees WHERE xmin = 100) AS version_info;`,
		`SELECT * FROM (SELECT xmin, xmax FROM employees) AS version_info;`,
		`SELECT * FROM employees WHERE xmin = 200;`,
		`SELECT * FROM employees WHERE 1 = 1 AND xmax = 300;`,
		`SELECT cmin
		FROM employees;`,
		`SELECT cmax
		FROM employees;`,
		`SELECT ctid, tableoid, xmin, xmax, cmin, cmax
		FROM employees;`,
		`WITH versioned_employees AS (
            SELECT *, xmin, xmax
            FROM employees
        )
        SELECT ve1.id, ve2.id
        FROM versioned_employees ve1
        JOIN versioned_employees ve2 ON ve1.xmin = ve2.xmax
        WHERE ve1.id <> ve2.id;`,
		`SELECT e.id, e.name,
            ROW_NUMBER() OVER (ORDER BY e.ctid) AS row_num
        FROM employees e;`,
		`SELECT *
        FROM employees e
        WHERE e.xmax = (
            SELECT MAX(xmax)
            FROM employees
            WHERE department = e.department
        );`,
		`UPDATE employees
        SET salary = salary * 1.05
        WHERE department = 'Sales'
        RETURNING id, xmax;`,
		`SELECT xmin, COUNT(*)
        FROM employees
        GROUP BY xmin
        HAVING COUNT(*) > 1;`,
	}

	for _, sql := range systemColumnSqls {
		issues := getDetectorIssues(t, NewColumnRefDetector(sql), sql)

		assert.Equal(t, 1, len(issues), "Expected 1 issue for SQL: %s", sql)
		assert.Equal(t, SYSTEM_COLUMNS, issues[0].Type, "Expected System Columns issue for SQL: %s", sql)
	}
}

func TestRangeTableFuncDetector(t *testing.T) {
	xmlTableSqls := []string{
		// Test Case 1: Simple XMLTABLE usage with basic columns
		`SELECT
		    p.id,
		    x.product_id,
		    x.product_name,
		    x.price
		FROM
		    products_basic p,
		    XMLTABLE(
		        '//Product'
		        PASSING p.data
		        COLUMNS
		            product_id TEXT PATH 'ID',
		            product_name TEXT PATH 'Name',
		            price NUMERIC PATH 'Price'
		    ) AS x;`,

		// Test Case 2: XMLTABLE with CROSS JOIN LATERAL
		`SELECT
		    o.order_id,
		    items.product,
		    items.quantity::INT
		FROM
		    orders_lateral o
		    CROSS JOIN LATERAL XMLTABLE(
		        '/order/item'
		        PASSING o.order_details
		        COLUMNS
		            product TEXT PATH 'product',
		            quantity TEXT PATH 'quantity'
		    ) AS items;`,

		// Test Case 3: XMLTABLE within a Common Table Expression (CTE)
		`WITH xml_data AS (
		    SELECT id, xml_column FROM xml_documents_cte
		)
		SELECT
		    xd.id,
		    e.emp_id,
		    e.name,
		    e.department
		FROM
		    xml_data xd,
		    XMLTABLE(
		        '//Employee'
		        PASSING xd.xml_column
		        COLUMNS
		            emp_id INT PATH 'ID',
		            name TEXT PATH 'Name',
		            department TEXT PATH 'Department'
		    ) AS e;`,

		// Test Case 4: Nested XMLTABLEs for handling hierarchical XML structures
		`SELECT
		    s.section_name,
		    b.title,
		    b.author
		FROM
		    library_nested l,
		    XMLTABLE(
		        '/library/section'
		        PASSING l.lib_data
		        COLUMNS
		            section_name TEXT PATH '@name',
		            books XML PATH 'book'
		    ) AS s,
		    XMLTABLE(
		        '/book'
		        PASSING s.books
		        COLUMNS
		            title TEXT PATH 'title',
		            author TEXT PATH 'author'
		    ) AS b;`,

		// Test Case 5: XMLTABLE with XML namespaces
		`SELECT
		    x.emp_name,
		    x.position,
		    x.city,
		    x.country
		FROM
		    employees_ns,
		    XMLTABLE(
		        XMLNAMESPACES (
		            'http://example.com/emp' AS emp,
		            'http://example.com/address' AS addr
		        ),
		        '/emp:Employee'  -- Using the emp namespace prefix
		        PASSING employees_ns.emp_data
		        COLUMNS
		            emp_name TEXT PATH 'emp:Name',        -- Using emp prefix
		            position TEXT PATH 'emp:Position',    -- Using emp prefix
		            city TEXT PATH 'addr:Address/addr:City',
		            country TEXT PATH 'addr:Address/addr:Country'
		    ) AS x;`,

		// Test Case 6: XMLTABLE used within a VIEW creation
		`CREATE VIEW order_items_view AS
		SELECT
		    o.order_id,
		    o.customer_name,
		    items.product,
		    items.quantity::INT
		FROM
		    orders_view o,
		    XMLTABLE(
		        '/order/item'
		        PASSING o.order_details
		        COLUMNS
		            product TEXT PATH 'product',
		            quantity TEXT PATH 'quantity'
		    ) AS items;`,
		`CREATE VIEW public.order_items_view AS
		SELECT o.order_id,
			o.customer_name,
			items.product,
			(items.quantity)::integer AS quantity
		FROM public.orders_view o,
			LATERAL XMLTABLE(('/order/item'::text) PASSING (o.order_details) COLUMNS product text PATH ('product'::text), quantity text PATH ('quantity'::text)) items;`,

		// Test Case 7: XMLTABLE with aggregation functions
		`SELECT
		    s.report_id,
		    SUM(t.amount::NUMERIC) AS total_sales
		FROM
		    sales_reports_nested s,
		    XMLTABLE(
		        '/sales/transaction'
		        PASSING s.sales_data
		        COLUMNS
		            amount TEXT PATH 'amount'
		    ) AS t
		GROUP BY
		    s.report_id;`,

		// Test Case 8: Nested XMLTABLE() with subqueries
		`SELECT
		s.report_id,
		SUM(t.amount::NUMERIC) AS total_sales
	FROM
		sales_reports_complex s,
		XMLTABLE(
			'/sales/transaction'
			PASSING s.sales_data
			COLUMNS
				transaction_id INT PATH '@id',
				transaction_details XML PATH 'details'
		) AS t,
		XMLTABLE(
			'/transaction/detail'
			PASSING t.transaction_details
			COLUMNS
				amount TEXT PATH 'amount'
		) AS detail
	GROUP BY
		s.report_id;`,
	}

	for _, sql := range xmlTableSqls {
		issues := getDetectorIssues(t, NewRangeTableFuncDetector(sql), sql)

		assert.Equal(t, 1, len(issues), "Expected 1 issue for SQL: %s", sql)
		assert.Equal(t, XML_FUNCTIONS, issues[0].Type, "Expected XML Functions issue for SQL: %s", sql)
	}
}

// TestXmlExprDetector tests the XML Function Detection - FuncCallDetector, XMLExprDetector, RangeTableFuncDetector.
func TestXMLFunctionsDetectors(t *testing.T) {
	xmlFunctionSqls := []string{
		`SELECT id, xmlelement(name "employee", name) AS employee_data FROM employees;`,
		`SELECT id, xpath('/person/name/text()', data) AS name FROM xml_example;`,
		`SELECT id FROM employees WHERE xmlexists('/id' PASSING BY VALUE xmlcolumn);`,
		`SELECT e.id, x.employee_xml
		        FROM employees e
		        JOIN (
		            SELECT xmlelement(name "employee", xmlattributes(e.id AS "id"), e.name) AS employee_xml
		            FROM employees e
		        ) x ON x.employee_xml IS NOT NULL
		        WHERE xmlexists('//employee[name="John Doe"]' PASSING BY REF x.employee_xml);`,
		`WITH xml_data AS (
		            SELECT
		                id,
		                xml_column,
		                xpath('/root/element/@attribute', xml_column) as xpath_result
		            FROM xml_documents
		        )
		        SELECT
		            x.id,
		            (xt.value).text as value
		        FROM
		            xml_data x
		            CROSS JOIN LATERAL unnest(x.xpath_result) as xt(value);`,
		`SELECT e.id, e.name
		        FROM employees e
		        WHERE CASE
		            WHEN e.department = 'IT' THEN xmlexists('//access[@level="high"]' PASSING e.permissions)
		            ELSE FALSE
		        END;`,
		`SELECT xmlserialize(
		            content xmlelement(name "employees",
		                xmlagg(
		                    xmlelement(name "employee",
		                        xmlattributes(e.id AS "id"),
		                        e.name
		                    )
		                )
		            ) AS CLOB
		        ) AS employees_xml
		        FROM employees e
		        WHERE e.status = 'active';`,
		`CREATE VIEW employee_xml_view AS
		        SELECT e.id,
		            xmlelement(name "employee",
		                xmlattributes(e.id AS "id"),
		                e.name,
		                e.department
		            ) AS employee_xml
		        FROM employees e;`,
		`SELECT  xmltext('<inventory><item>Widget</item></inventory>') AS inventory_text
		FROM inventory
		WHERE id = 5;`,
		`SELECT xmlforest(name, department) AS employee_info
		FROM employees
		WHERE id = 4;`,
		`SELECT xmltable.*
				FROM xmldata,
				    XMLTABLE('//ROWS/ROW'
				            PASSING data
				            COLUMNS id int PATH '@id',
				                ordinality FOR ORDINALITY,
				                "COUNTRY_NAME" text,
				                country_id text PATH 'COUNTRY_ID',
				                size_sq_km float PATH 'SIZE[@unit = "sq_km"]',
				                size_other text PATH
				                'concat(SIZE[@unit!="sq_km"], " ", SIZE[@unit!="sq_km"]/@unit)',
				                 premier_name text PATH 'PREMIER_NAME' DEFAULT 'not specified');`,
		`SELECT xmltable.*
				FROM XMLTABLE(XMLNAMESPACES('http://example.com/myns' AS x,
				                            'http://example.com/b' AS "B"),
				             '/x:example/x:item'
				                PASSING (SELECT data FROM xmldata)
				                COLUMNS foo int PATH '@foo',
				                  bar int PATH '@B:bar');`,
		`SELECT xml_is_well_formed_content('<project>Alpha</project>') AS is_well_formed_content
		FROM projects
		WHERE project_id = 10;`,
		`SELECT xml_is_well_formed_document(xmlforest(name, department)) AS is_well_formed_document
		FROM employees
		WHERE id = 2;`,
		`SELECT xml_is_well_formed(xmltext('<employee><name>Jane Doe</name></employee>')) AS is_well_formed
		FROM employees
		WHERE id = 1;`,
		`SELECT xmlparse(DOCUMENT '<employee><name>John</name></employee>');`,
		`SELECT xpath_exists('/employee/name', '<employee><name>John</name></employee>'::xml)`,
		`SELECT table_to_xml('employees', TRUE, FALSE, '');`,
		`SELECT query_to_xml('SELECT * FROM employees', TRUE, FALSE, '');`,
		`SELECT schema_to_xml('public', TRUE, FALSE, '');`,
		`SELECT database_to_xml(TRUE, TRUE, '');`,
		`SELECT query_to_xmlschema('SELECT * FROM employees', TRUE, FALSE, '');`,
		`SELECT table_to_xmlschema('employees', TRUE, FALSE, '');`,
		`SELECT xmlconcat('<root><tag1>value1</tag1></root>'::xml, '<tag2>value2</tag2>'::xml);`,
		`SELECT xmlcomment('Sample XML comment');`,
		`SELECT xmlpi(name php, 'echo "hello world";');`,
		`SELECT xmlroot('<root><child>content</child></root>', VERSION '1.0');`,
		`SELECT xmlagg('<item>content</item>');`,
		`SELECT xmlexists('//some/path' PASSING BY REF '<root><some><path/></some></root>');`,
		`SELECT table_to_xml_and_xmlschema('public', 'employees', true, false, '');`,
		`SELECT * FROM cursor_to_xmlschema('foo_cursor', false, true,'');`,
		`SELECT * FROM cursor_to_xml('foo_cursor', 1, false, false,'');`,
		`SELECT query_to_xml_and_xmlschema('SELECT * FROM employees', true, false, '');`,
		`SELECT schema_to_xmlschema('public', true, false, '');`,
		`SELECT schema_to_xml_and_xmlschema('public', true, false, '');`,
		`SELECT database_to_xmlschema(true, false, '');`,
		`SELECT database_to_xml_and_xmlschema(true, false, '');`,
		`SELECT xmlconcat2('<element>Content</element>', '<additional>More Content</additional>');`,
		`SELECT xmlvalidate('<root><valid>content</valid></root>');`,
		`SELECT xml_in('<root><data>input</data></root>');`,
		`SELECT xml_out('<root><data>output</data></root>');`,
		`SELECT xml_recv('');`,
		`SELECT xml_send('<root><data>send</data></root>');`,
	}

	for _, sql := range xmlFunctionSqls {
		detectors := []UnsupportedConstructDetector{
			NewXmlExprDetector(sql),
			NewRangeTableFuncDetector(sql),
			NewFuncCallDetector(sql),
		}

		parseResult, err := queryparser.Parse(sql)
		assert.NoError(t, err)

		visited := make(map[protoreflect.Message]bool)

		processor := func(msg protoreflect.Message) error {
			for _, detector := range detectors {
				log.Debugf("running detector %T", detector)
				err := detector.Detect(msg)
				if err != nil {
					log.Debugf("error in detector %T: %v", detector, err)
					return fmt.Errorf("error in detectors %T: %w", detector, err)
				}
			}
			return nil
		}

		parseTreeMsg := queryparser.GetProtoMessageFromParseTree(parseResult)
		err = queryparser.TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		var allIssues []QueryIssue
		for _, detector := range detectors {
			allIssues = append(allIssues, detector.GetIssues()...)
		}

		xmlIssueDetected := false
		for _, issue := range allIssues {
			if issue.Type == XML_FUNCTIONS {
				xmlIssueDetected = true
				break
			}
		}

		assert.True(t, xmlIssueDetected, "Expected XML Functions issue for SQL: %s", sql)
	}
}

// Combination of: FuncCallDetector, ColumnRefDetector, XmlExprDetector
func TestCombinationOfDetectors1(t *testing.T) {
	combinationSqls := []string{
		`WITH LockedEmployees AS (
    SELECT *, pg_advisory_lock(xmin) AS lock_acquired
    FROM employees
    WHERE pg_try_advisory_lock(xmin) IS TRUE
)
SELECT xmlelement(name "EmployeeData", xmlagg(
    xmlelement(name "Employee", xmlattributes(id AS "ID"),
    xmlforest(name AS "Name", xmin AS "TransactionID", xmax AS "ModifiedID"))))
FROM LockedEmployees
WHERE xmax IS NOT NULL;`,
		`WITH Data AS (
    SELECT id, name, xmin, xmax,
           pg_try_advisory_lock(id) AS lock_status,
           xmlelement(name "info", xmlforest(name as "name", xmin as "transaction_start", xmax as "transaction_end")) as xml_info
    FROM projects
    WHERE xmin > 100 AND xmax < 500
)
SELECT x.id, x.xml_info
FROM Data x
WHERE x.lock_status IS TRUE;`,
		`UPDATE employees
SET salary = salary * 1.1
WHERE pg_try_advisory_xact_lock(ctid) IS TRUE AND department = 'Engineering'
RETURNING id,
          xmlelement(name "UpdatedEmployee",
                     xmlattributes(id AS "ID"),
                     xmlforest(name AS "Name", salary AS "NewSalary", xmin AS "TransactionStartID", xmax AS "TransactionEndID"));`,
	}
	expectedIssueTypes := mapset.NewThreadUnsafeSet[string]([]string{ADVISORY_LOCKS, SYSTEM_COLUMNS, XML_FUNCTIONS}...)

	for _, sql := range combinationSqls {
		detectors := []UnsupportedConstructDetector{
			NewFuncCallDetector(sql),
			NewColumnRefDetector(sql),
			NewXmlExprDetector(sql),
		}
		parseResult, err := queryparser.Parse(sql)
		assert.NoError(t, err)

		visited := make(map[protoreflect.Message]bool)

		processor := func(msg protoreflect.Message) error {
			for _, detector := range detectors {
				log.Debugf("running detector %T", detector)
				err := detector.Detect(msg)
				if err != nil {
					log.Debugf("error in detector %T: %v", detector, err)
					return fmt.Errorf("error in detectors %T: %w", detector, err)
				}
			}
			return nil
		}

		parseTreeMsg := queryparser.GetProtoMessageFromParseTree(parseResult)
		err = queryparser.TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		var allIssues []QueryIssue
		for _, detector := range detectors {
			allIssues = append(allIssues, detector.GetIssues()...)
		}
		issueTypesDetected := mapset.NewThreadUnsafeSet[string]()
		for _, issue := range allIssues {
			issueTypesDetected.Add(issue.Type)
		}

		assert.True(t, expectedIssueTypes.Equal(issueTypesDetected), "Expected issue types do not match the detected issue types. Expected: %v, Actual: %v", expectedIssueTypes, issueTypesDetected)
	}
}

func TestCombinationOfDetectors1WithObjectCollector(t *testing.T) {
	tests := []struct {
		Sql             string
		ExpectedObjects []string
		ExpectedSchemas []string
	}{
		{
			Sql: `WITH LockedEmployees AS (
					SELECT *, pg_advisory_lock(xmin) AS lock_acquired
					FROM public.employees
					WHERE pg_try_advisory_lock(xmin) IS TRUE
				)
				SELECT xmlelement(name "EmployeeData", xmlagg(
					xmlelement(name "Employee", xmlattributes(id AS "ID"),
					xmlforest(name AS "Name", xmin AS "TransactionID", xmax AS "ModifiedID"))))
				FROM LockedEmployees
				WHERE xmax IS NOT NULL;`,
			/*
				Limitation: limited coverage provided by objectCollector.Collect() right now. Might not detect some cases.
				xmlelement, xmlforest etc are present under xml_expr node in parse tree not funccall node.
			*/
			ExpectedObjects: []string{"pg_advisory_lock", "public.employees", "pg_try_advisory_lock", "xmlagg", "lockedemployees"},
			ExpectedSchemas: []string{"public", ""},
		},
		{
			Sql: `WITH Data AS (
					SELECT id, name, xmin, xmax,
						pg_try_advisory_lock(id) AS lock_status,
						xmlelement(name "info", xmlforest(name as "name", xmin as "transaction_start", xmax as "transaction_end")) as xml_info
					FROM projects
					WHERE xmin > 100 AND xmax < 500
				)
				SELECT x.id, x.xml_info
				FROM Data x
				WHERE x.lock_status IS TRUE;`,
			ExpectedObjects: []string{"pg_try_advisory_lock", "projects", "data"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql: `UPDATE s1.employees
					SET salary = salary * 1.1
					WHERE pg_try_advisory_xact_lock(ctid) IS TRUE AND department = 'Engineering'
					RETURNING id, xmlelement(name "UpdatedEmployee", xmlattributes(id AS "ID"),
						xmlforest(name AS "Name", salary AS "NewSalary", xmin AS "TransactionStartID", xmax AS "TransactionEndID"));`,
			ExpectedObjects: []string{"s1.employees", "pg_try_advisory_xact_lock"},
			ExpectedSchemas: []string{"s1", ""},
		},
	}

	expectedIssueTypes := mapset.NewThreadUnsafeSet[string]([]string{ADVISORY_LOCKS, SYSTEM_COLUMNS, XML_FUNCTIONS}...)

	for _, tc := range tests {
		detectors := []UnsupportedConstructDetector{
			NewFuncCallDetector(tc.Sql),
			NewColumnRefDetector(tc.Sql),
			NewXmlExprDetector(tc.Sql),
		}
		parseResult, err := queryparser.Parse(tc.Sql)
		assert.NoError(t, err)

		visited := make(map[protoreflect.Message]bool)

		objectCollector := queryparser.NewObjectCollector(nil)
		processor := func(msg protoreflect.Message) error {
			for _, detector := range detectors {
				log.Debugf("running detector %T", detector)
				err := detector.Detect(msg)
				if err != nil {
					log.Debugf("error in detector %T: %v", detector, err)
					return fmt.Errorf("error in detectors %T: %w", detector, err)
				}
			}
			objectCollector.Collect(msg)
			return nil
		}

		parseTreeMsg := queryparser.GetProtoMessageFromParseTree(parseResult)
		err = queryparser.TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		var allIssues []QueryIssue
		for _, detector := range detectors {
			allIssues = append(allIssues, detector.GetIssues()...)
		}
		issueTypesDetected := mapset.NewThreadUnsafeSet[string]()
		for _, issue := range allIssues {
			issueTypesDetected.Add(issue.Type)
		}

		assert.True(t, expectedIssueTypes.Equal(issueTypesDetected), "Expected issue types do not match the detected issue types. Expected: %v, Actual: %v", expectedIssueTypes, issueTypesDetected)

		collectedObjects := objectCollector.GetObjects()
		collectedSchemas := objectCollector.GetSchemaList()

		assert.ElementsMatch(t, tc.ExpectedObjects, collectedObjects,
			"Objects list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedObjects, len(tc.ExpectedObjects), collectedObjects, len(collectedObjects))
		assert.ElementsMatch(t, tc.ExpectedSchemas, collectedSchemas,
			"Schema list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedSchemas, len(tc.ExpectedSchemas), collectedSchemas, len(collectedSchemas))
	}
}

func TestJsonConstructorDetector(t *testing.T) {
	sql := `SELECT JSON_ARRAY('PostgreSQL', 12, TRUE, NULL) AS json_array;`

	issues := getDetectorIssues(t, NewJsonConstructorFuncDetector(sql), sql)
	assert.Equal(t, 1, len(issues), "Expected 1 issue for SQL: %s", sql)
	assert.Equal(t, JSON_CONSTRUCTOR_FUNCTION, issues[0].Type, "Expected Advisory Locks issue for SQL: %s", sql)

}

func TestJsonQueryFunctionDetector(t *testing.T) {
	sql := `SELECT id, JSON_VALUE(details, '$.title') AS title
FROM books
WHERE JSON_EXISTS(details, '$.price ? (@ > $price)' PASSING 30 AS price);`

	issues := getDetectorIssues(t, NewJsonQueryFunctionDetector(sql), sql)
	assert.Equal(t, 1, len(issues), "Expected 1 issue for SQL: %s", sql)
	assert.Equal(t, JSON_QUERY_FUNCTION, issues[0].Type, "Expected Advisory Locks issue for SQL: %s", sql)

}
