package queryparser

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test function that uses real SQL input to check detection of Advisory Locks
func TestContainsAdvisoryLocks(t *testing.T) {
	testCases := []struct {
		name     string
		SQL      string
		expected bool
	}{
		{
			name:     "Advisory Lock in Target List",
			SQL:      `SELECT pg_advisory_lock(100), COUNT(*) FROM cars`,
			expected: true,
		},
		{
			name:     "Advisory Lock in FROM clause",
			SQL:      `SELECT * FROM (SELECT pg_advisory_lock(200)) AS lock_acquired;`,
			expected: true,
		},
		{
			name:     "Advisory Lock in WHERE clause 1",
			SQL:      `SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(300) IS TRUE;`,
			expected: true,
		},
		{
			name:     "Advisory Lock in WHERE clause 2",
			SQL:      `SELECT id, first_name FROM employees WHERE salary > 400 AND EXISTS (SELECT 1 FROM pg_advisory_lock(500))`,
			expected: true,
		},
		{
			name:     "Advisory Lock in WHERE clause 3",
			SQL:      `SELECT id, first_name FROM employees WHERE pg_try_advisory_lock(600) IS TRUE AND salary > 700`,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("assertion check for test %s\n", tc.name)
			qp := New(tc.SQL)
			err := qp.Parse()
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			// Check for Advisory Locks based on generated parse tree
			result := qp.containsAdvisoryLocks()
			assert.Equal(t, tc.expected, result, "Expected result does not match actual result.")
		})
	}
}

// Test function that uses real SQL input to check detection of System Columns
func TestContainsSystemColumns(t *testing.T) {
	testCases := []struct {
		name     string
		SQL      string
		expected bool
	}{
		{
			name:     "System Columns in Target List",
			SQL:      `SELECT xmin, xmax FROM employees`,
			expected: true,
		},
		{
			name:     "System Columns in FROM clause 1",
			SQL:      `SELECT * FROM (SELECT * FROM employees WHERE xmin = 100) AS version_info`,
			expected: true,
		},
		{
			name:     "System Columns in FROM clause 2",
			SQL:      `SELECT * FROM (SELECT xmin, xmax FROM employees) AS version_info`,
			expected: true,
		},
		{
			name:     "System Columns in WHERE clause 1",
			SQL:      `SELECT * FROM employees WHERE xmin = 200`,
			expected: true,
		},
		{
			name:     "System Columns in WHERE clause 2",
			SQL:      `SELECT * FROM employees WHERE 1 = 1 AND xmax = 300`,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("assertion check for test %s\n", tc.name)
			qp := New(tc.SQL)
			err := qp.Parse()
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			// Check for System Columns based on generated parse tree
			result := qp.containsSystemColumns()
			assert.Equal(t, tc.expected, result, "Expected result does not match actual result.")
		})
	}
}

// Test function that uses real SQL input to check detection of XML functions
func TestContainsXmlFunctions(t *testing.T) {
	testCases := []struct {
		name     string
		SQL      string
		expected bool
	}{
		{
			name:     "No XML function",
			SQL:      `SELECT id, name FROM employees`,
			expected: false,
		},
		{
			name:     "With XML function in target list as XMLExpr Node",
			SQL:      `SELECT id, xmlelement(name "employee", name) AS employee_data FROM employees`,
			expected: true,
		},
		{
			name:     "With XML function in target list as Func Call Node",
			SQL:      `SELECT id, xpath('/person/name/text()', data) AS name from xml_example;`,
			expected: true,
		},
		{
			name:     "With XML function in WHERE clause",
			SQL:      `SELECT id FROM employees WHERE xmlexists('/id' PASSING BY VALUE xmlcolumn)`,
			expected: true,
		},
		{
			name: "XML function in FROM clause",
			SQL: `SELECT name 
					FROM xmltable(
						'//item' PASSING BY VALUE 
						xmlparse(document '<items><item><name>Example</name></item></items>')
						COLUMNS 
						name TEXT PATH './name'
					) AS xt;`,
			expected: true,
		},
		// TODO: future
		// 		{
		// 			name: "Nesed XML function in WHERE clause",
		// 			SQL: `SELECT * FROM employees
		// WHERE LENGTH(xmlelement(name "emp", xmlforest(name as "name"))::text) > 100;`,
		// 			expected: true,
		// 		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("assertion check for test %s\n", tc.name)
			qp := New(tc.SQL)
			err := qp.Parse()
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			// Check for XML functions based on generated parse tree
			result := qp.containsXmlFunctions()
			assert.Equal(t, tc.expected, result, "Expected result does not match actual result.")
		})
	}
}
