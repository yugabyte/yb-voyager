package queryparser

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestFuncCallDetector tests the Advisory Lock Detector.
func TestFuncCallDetector(t *testing.T) {
	advisoryLockSqls := []string{
		`SELECT pg_advisory_lock(100), COUNT(*) FROM cars;`,
		`SELECT * FROM (SELECT pg_advisory_lock(200)) AS lock_acquired;`,
		`SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(300) IS TRUE;`,
		`SELECT id, first_name FROM employees WHERE salary > 400 AND EXISTS (SELECT 1 FROM pg_advisory_lock(500));`,
		`SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(600) IS TRUE AND salary > 700;`,
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
            AND pg_try_advisory_lock(p.project_id)
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
	}

	detector := NewFuncCallDetector()

	for _, sql := range advisoryLockSqls {
		parseResult, err := pg_query.Parse(sql)
		assert.NoError(t, err, "Failed to parse SQL: %s", sql)

		visited := make(map[protoreflect.Message]bool)
		constructs := []string{}

		processor := func(msg protoreflect.Message) error {
			detected, err := detector.Detect(msg)
			if err != nil {
				return err
			}
			constructs = append(constructs, detected...)
			return nil
		}

		for _, stmt := range parseResult.Stmts {
			parseTreeMsg := stmt.Stmt.ProtoReflect()
			err = TraverseParseTree(parseTreeMsg, visited, processor)
			assert.NoError(t, err, "Error during traversal for SQL: %s", sql)
		}
		// The detector should detect Advisory Locks in these queries
		assert.Contains(t, constructs, ADVISORY_LOCKS, "Advisory Locks not detected in SQL: %s", sql)
	}
}

// TestColumnRefDetector tests the System Column Detector.
func TestColumnRefDetector(t *testing.T) {
	systemColumnSqls := []string{
		`SELECT xmin, xmax FROM employees;`,
		`SELECT * FROM (SELECT * FROM employees WHERE xmin = 100) AS version_info;`,
		`SELECT * FROM (SELECT xmin, xmax FROM employees) AS version_info;`,
		`SELECT * FROM employees WHERE xmin = 200;`,
		`SELECT * FROM employees WHERE 1 = 1 AND xmax = 300;`,
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

	detector := NewColumnRefDetector()

	for _, sql := range systemColumnSqls {
		parseResult, err := pg_query.Parse(sql)
		assert.NoError(t, err, "Failed to parse SQL: %s", sql)

		visited := make(map[protoreflect.Message]bool)
		constructs := []string{}

		processor := func(msg protoreflect.Message) error {
			detected, err := detector.Detect(msg)
			if err != nil {
				return err
			}
			constructs = append(constructs, detected...)
			return nil
		}

		for _, stmt := range parseResult.Stmts {
			parseTreeMsg := stmt.Stmt.ProtoReflect()
			err = TraverseParseTree(parseTreeMsg, visited, processor)
			assert.NoError(t, err)
		}
		// The detector should detect System Columns in these queries
		assert.Contains(t, constructs, SYSTEM_COLUMNS, "System Columns not detected in SQL: %s", sql)
	}
}

// TestXmlExprDetector tests the XML Function Detection.
func TestXmlExprDetectorAndFuncCallDetector(t *testing.T) {
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
	}

	detectors := []UnsupportedConstructDetector{
		NewXmlExprDetector(),
		NewFuncCallDetector(),
	}
	compositeDetector := &CompositeDetector{detectors: detectors}

	for _, sql := range xmlFunctionSqls {
		parseResult, err := pg_query.Parse(sql)
		assert.NoError(t, err)

		visited := make(map[protoreflect.Message]bool)
		constructs := []string{}

		processor := func(msg protoreflect.Message) error {
			detected, err := compositeDetector.Detect(msg)
			if err != nil {
				return err
			}
			constructs = append(constructs, detected...)
			return nil
		}

		for _, stmt := range parseResult.Stmts {
			parseTreeMsg := stmt.Stmt.ProtoReflect()
			err = TraverseParseTree(parseTreeMsg, visited, processor)
			assert.NoError(t, err)
		}
		// The detector should detect XML Functions in these queries
		assert.Contains(t, constructs, XML_FUNCTIONS, "XML Functions not detected in SQL: %s", sql)
	}
}
