package queryparser

import (
	"fmt"
	"sort"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestFuncCallDetector tests the Advisory Lock Detector.
func TestFuncCallDetector(t *testing.T) {
	advisoryLockSqls := []string{
		`SELECT pg_advisory_lock(100), COUNT(*) FROM cars;`,
		`SELECT pg_advisory_lock_shared(100), COUNT(*) FROM cars;`,
		`SELECT * FROM (SELECT pg_advisory_xact_lock(200)) AS lock_acquired;`,
		`SELECT * FROM (SELECT pg_advisory_xact_lock_shared(200)) AS lock_acquired;`,
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
	}

	detector := NewFuncCallDetector()
	for _, sql := range advisoryLockSqls {
		parseResult, err := pg_query.Parse(sql)
		assert.NoError(t, err, "Failed to parse SQL: %s", sql)

		visited := make(map[protoreflect.Message]bool)
		unsupportedConstructs := []string{}

		processor := func(msg protoreflect.Message) error {
			constructs, err := detector.Detect(msg)
			if err != nil {
				return err
			}
			unsupportedConstructs = append(unsupportedConstructs, constructs...)
			return nil
		}

		parseTreeMsg := parseResult.Stmts[0].Stmt.ProtoReflect()
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)
		// The detector should detect Advisory Locks in these queries
		assert.Contains(t, unsupportedConstructs, ADVISORY_LOCKS, "Advisory Locks not detected in SQL: %s", sql)
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
		unsupportedConstructs := []string{}

		processor := func(msg protoreflect.Message) error {
			constructs, err := detector.Detect(msg)
			if err != nil {
				return err
			}
			unsupportedConstructs = append(unsupportedConstructs, constructs...)
			return nil
		}

		parseTreeMsg := parseResult.Stmts[0].Stmt.ProtoReflect()
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)
		// The detector should detect System Columns in these queries
		assert.Contains(t, unsupportedConstructs, SYSTEM_COLUMNS, "System Columns not detected in SQL: %s", sql)
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
		`SELECT  xmltext('<inventory><item>Widget</item></inventory>') AS inventory_text
FROM inventory
WHERE id = 5;`,
		`SELECT xmlforest(name, department) AS employee_info
FROM employees
WHERE id = 4;`,
		// TODO: future -
		// 		`SELECT xmltable.*
		// FROM xmldata,
		//     XMLTABLE('//ROWS/ROW'
		//             PASSING data
		//             COLUMNS id int PATH '@id',
		//                 ordinality FOR ORDINALITY,
		//                 "COUNTRY_NAME" text,
		//                 country_id text PATH 'COUNTRY_ID',
		//                 size_sq_km float PATH 'SIZE[@unit = "sq_km"]',
		//                 size_other text PATH
		//                 'concat(SIZE[@unit!="sq_km"], " ", SIZE[@unit!="sq_km"]/@unit)',
		//                  premier_name text PATH 'PREMIER_NAME' DEFAULT 'not specified');`,
		// 		`SELECT xmltable.*
		// FROM XMLTABLE(XMLNAMESPACES('http://example.com/myns' AS x,
		//                             'http://example.com/b' AS "B"),
		//              '/x:example/x:item'
		//                 PASSING (SELECT data FROM xmldata)
		//                 COLUMNS foo int PATH '@foo',
		//                   bar int PATH '@B:bar');`,
		`SELECT xml_is_well_formed_content('<project>Alpha</project>') AS is_well_formed_content
FROM projects
WHERE project_id = 10;`,
		`SELECT xml_is_well_formed_document(xmlforest(name, department)) AS is_well_formed_document
FROM employees
WHERE id = 2;`,
		`SELECT xml_is_well_formed(xmltext('<employee><name>Jane Doe</name></employee>')) AS is_well_formed
FROM employees
WHERE id = 1;`,
	}

	detectors := []UnsupportedConstructDetector{
		NewXmlExprDetector(),
		NewFuncCallDetector(),
	}

	for _, sql := range xmlFunctionSqls {
		parseResult, err := pg_query.Parse(sql)
		assert.NoError(t, err)

		visited := make(map[protoreflect.Message]bool)
		unsupportedConstructs := []string{}

		processor := func(msg protoreflect.Message) error {
			for _, detector := range detectors {
				log.Debugf("running detector %T", detector)
				constructs, err := detector.Detect(msg)
				if err != nil {
					log.Debugf("error in detector %T: %v", detector, err)
					return fmt.Errorf("error in detectors %T: %w", detector, err)
				}
				unsupportedConstructs = lo.Union(unsupportedConstructs, constructs)
			}
			return nil
		}

		parseTreeMsg := parseResult.Stmts[0].Stmt.ProtoReflect()
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)
		// The detector should detect XML Functions in these queries
		assert.Contains(t, unsupportedConstructs, XML_FUNCTIONS, "XML Functions not detected in SQL: %s", sql)
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
	expectedConstructs := []string{ADVISORY_LOCKS, SYSTEM_COLUMNS, XML_FUNCTIONS}

	detectors := []UnsupportedConstructDetector{
		NewFuncCallDetector(),
		NewColumnRefDetector(),
		NewXmlExprDetector(),
	}
	for _, sql := range combinationSqls {
		parseResult, err := pg_query.Parse(sql)
		assert.NoError(t, err)

		visited := make(map[protoreflect.Message]bool)
		unsupportedConstructs := []string{}

		processor := func(msg protoreflect.Message) error {
			for _, detector := range detectors {
				log.Debugf("running detector %T", detector)
				constructs, err := detector.Detect(msg)
				if err != nil {
					log.Debugf("error in detector %T: %v", detector, err)
					return fmt.Errorf("error in detectors %T: %w", detector, err)
				}
				unsupportedConstructs = lo.Union(unsupportedConstructs, constructs)
			}
			return nil
		}

		parseTreeMsg := parseResult.Stmts[0].Stmt.ProtoReflect()
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		sort.Strings(unsupportedConstructs)
		sort.Strings(expectedConstructs)
		assert.Equal(t, expectedConstructs, unsupportedConstructs, "Detected constructs do not exactly match the expected constructs")
	}
}
