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
package queryparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Test ObjectCollector with default predicate(AllObjectsPredicate)
func TestObjectCollector(t *testing.T) {
	tests := []struct {
		Sql             string
		ExpectedObjects []string
		ExpectedSchemas []string
	}{
		{
			Sql:             `SELECT * from public.employees`,
			ExpectedObjects: []string{"public.employees"},
			ExpectedSchemas: []string{"public"},
		},
		{
			Sql:             `SELECT * from employees`,
			ExpectedObjects: []string{"employees"},
			ExpectedSchemas: []string{""}, // empty schemaname indicates unqualified objectname
		},
		{
			Sql:             `SELECT * from s1.employees`,
			ExpectedObjects: []string{"s1.employees"},
			ExpectedSchemas: []string{"s1"},
		},
		{
			Sql:             `SELECT * from s2.employees where salary > (Select salary from s3.employees)`,
			ExpectedObjects: []string{"s3.employees", "s2.employees"},
			ExpectedSchemas: []string{"s3", "s2"},
		},
		{
			Sql:             `SELECT c.name, SUM(o.amount) AS total_spent FROM sales.customers c JOIN finance.orders o ON c.id = o.customer_id GROUP BY c.name HAVING SUM(o.amount) > 1000`,
			ExpectedObjects: []string{"sales.customers", "finance.orders", "sum"},
			ExpectedSchemas: []string{"sales", "finance", ""},
		},
		{
			Sql:             `SELECT name FROM hr.employees WHERE department_id IN (SELECT id FROM public.departments WHERE location_id IN (SELECT id FROM eng.locations WHERE country = 'USA'))`,
			ExpectedObjects: []string{"hr.employees", "public.departments", "eng.locations"},
			ExpectedSchemas: []string{"hr", "public", "eng"},
		},
		{
			Sql:             `SELECT name FROM sales.customers UNION SELECT name FROM marketing.customers;`,
			ExpectedObjects: []string{"sales.customers", "marketing.customers"},
			ExpectedSchemas: []string{"sales", "marketing"},
		},
		{
			Sql: `CREATE VIEW analytics.top_customers AS
					SELECT user_id, COUNT(*) as order_count	FROM public.orders GROUP BY user_id HAVING COUNT(*) > 10;`,
			ExpectedObjects: []string{"analytics.top_customers", "public.orders", "count"},
			ExpectedSchemas: []string{"analytics", "public", ""}, // "" -> unknown schemaName
		},
		{
			Sql: `WITH user_orders AS (
					SELECT u.id, o.id as order_id
					FROM users u
					JOIN orders o ON u.id = o.user_id
					WHERE o.amount > 100
				), order_items AS (
					SELECT o.order_id, i.product_id
					FROM order_items i
					JOIN user_orders o ON i.order_id = o.order_id
				)
				SELECT p.name, COUNT(oi.product_id) FROM products p
				JOIN order_items oi ON p.id = oi.product_id GROUP BY p.name;`,
			ExpectedObjects: []string{"users", "orders", "order_items", "user_orders", "products", "count"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql: `UPDATE finance.accounts
						SET balance = balance + 1000
						WHERE account_id IN (
							SELECT account_id FROM public.users WHERE active = true
					);`,
			ExpectedObjects: []string{"finance.accounts", "public.users"},
			ExpectedSchemas: []string{"finance", "public"},
		},
		{
			Sql:             `SELECT classid, objid, refobjid FROM pg_depend WHERE refclassid = $1::regclass AND deptype = $2 ORDER BY 3`,
			ExpectedObjects: []string{"pg_depend"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql:             `SELECT pg_advisory_unlock_shared(100);`,
			ExpectedObjects: []string{"pg_advisory_unlock_shared"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql:             `SELECT xpath_exists('/employee/name', '<employee><name>John</name></employee>'::xml)`,
			ExpectedObjects: []string{"xpath_exists"},
			ExpectedSchemas: []string{""},
		},
	}

	for _, tc := range tests {
		parseResult, err := Parse(tc.Sql)
		assert.NoError(t, err)

		objectsCollector := NewObjectCollector(nil)
		processor := func(msg protoreflect.Message) error {
			objectsCollector.Collect(msg)
			return nil
		}

		visited := make(map[protoreflect.Message]bool)
		parseTreeMsg := GetProtoMessageFromParseTree(parseResult)
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		collectedObjects := objectsCollector.GetObjects()
		collectedSchemas := objectsCollector.GetSchemaList()

		assert.ElementsMatch(t, tc.ExpectedObjects, collectedObjects,
			"Objects list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedObjects, len(tc.ExpectedObjects), collectedObjects, len(collectedObjects))
		assert.ElementsMatch(t, tc.ExpectedSchemas, collectedSchemas,
			"Schema list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedSchemas, len(tc.ExpectedSchemas), collectedSchemas, len(collectedSchemas))
	}
}

// Test ObjectCollector with default predicate(AllObjectsPredicate)
// test focussed on collecting function names from DMLs
func TestObjectCollector2(t *testing.T) {
	tests := []struct {
		Sql             string
		ExpectedObjects []string
		ExpectedSchemas []string
	}{
		{
			Sql:             `SELECT finance.calculate_tax(amount) AS tax, name FROM sales.transactions;`,
			ExpectedObjects: []string{"finance.calculate_tax", "sales.transactions"},
			ExpectedSchemas: []string{"finance", "sales"},
		},
		{
			Sql:             `SELECT hr.get_employee_details(e.id) FROM hr.employees e;`,
			ExpectedObjects: []string{"hr.get_employee_details", "hr.employees"},
			ExpectedSchemas: []string{"hr"},
		},
		{
			Sql:             `Select now();`,
			ExpectedObjects: []string{"now"},
			ExpectedSchemas: []string{""},
		},
		{ // nested functions
			Sql:             `SELECT finance.calculate_bonus(sum(salary)) FROM hr.employees;`,
			ExpectedObjects: []string{"finance.calculate_bonus", "sum", "hr.employees"},
			ExpectedSchemas: []string{"finance", "", "hr"},
		},
		{ // functions as arguments in expressions
			Sql: `SELECT e.name, CASE
					WHEN e.salary > finance.calculate_bonus(e.salary) THEN 'High'
					ELSE 'Low'
				END AS salary_grade
				FROM hr.employees e;`,
			ExpectedObjects: []string{"finance.calculate_bonus", "hr.employees"},
			ExpectedSchemas: []string{"finance", "hr"},
		},
		{
			Sql:             `CREATE SEQUENCE finance.invoice_seq START 1000;`,
			ExpectedObjects: []string{"finance.invoice_seq"},
			ExpectedSchemas: []string{"finance"},
		},
		{
			Sql: `SELECT department,
						MAX(CASE WHEN month = 'January' THEN sales ELSE 0 END) AS January_Sales,
						MAX(CASE WHEN month = 'February' THEN sales ELSE 0 END) AS February_Sales
					FROM sales_data
					GROUP BY department;`,
			ExpectedObjects: []string{"max", "sales_data"},
			ExpectedSchemas: []string{""},
		},
		{ // quoted mixed case
			Sql:             `SELECT * FROM "SALES_DATA"."Order_Details";`,
			ExpectedObjects: []string{"SALES_DATA.Order_Details"},
			ExpectedSchemas: []string{"SALES_DATA"},
		},
		{
			Sql:             `SELECT * FROM myfunc(analytics.calculate_metrics(2024)) AS cm(metrics);`,
			ExpectedObjects: []string{"myfunc", "analytics.calculate_metrics"},
			ExpectedSchemas: []string{"", "analytics"},
		},
		{
			Sql: `COPY (SELECT id, xmlagg(xmlparse(document xml_column)) AS combined_xml
					FROM my_table
						GROUP BY id)
					TO '/path/to/file.csv' WITH CSV;`,
			ExpectedObjects: []string{"xmlagg", "my_table"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql: `COPY (SELECT ctid, xmin, id, data FROM schema_name.my_table) 
							TO '/path/to/file_with_system_columns.csv' WITH CSV;`,
			ExpectedObjects: []string{"schema_name.my_table"},
			ExpectedSchemas: []string{"schema_name"},
		},
	}

	for _, tc := range tests {
		parseResult, err := Parse(tc.Sql)
		assert.NoError(t, err)

		objectsCollector := NewObjectCollector(nil)
		processor := func(msg protoreflect.Message) error {
			objectsCollector.Collect(msg)
			return nil
		}

		visited := make(map[protoreflect.Message]bool)
		parseTreeMsg := GetProtoMessageFromParseTree(parseResult)
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		collectedObjects := objectsCollector.GetObjects()
		collectedSchemas := objectsCollector.GetSchemaList()

		assert.ElementsMatch(t, tc.ExpectedObjects, collectedObjects,
			"Objects list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedObjects, len(tc.ExpectedObjects), collectedObjects, len(collectedObjects))
		assert.ElementsMatch(t, tc.ExpectedSchemas, collectedSchemas,
			"Schema list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedSchemas, len(tc.ExpectedSchemas), collectedSchemas, len(collectedSchemas))
	}
}

// Test ObjectCollector with TablesOnlyPredicate
func TestTableObjectCollector(t *testing.T) {
	tests := []struct {
		Sql             string
		ExpectedObjects []string
		ExpectedSchemas []string
	}{
		{
			Sql:             `SELECT * from public.employees`,
			ExpectedObjects: []string{"public.employees"},
			ExpectedSchemas: []string{"public"},
		},
		{
			Sql:             `SELECT * from employees`,
			ExpectedObjects: []string{"employees"},
			ExpectedSchemas: []string{""}, // empty schemaname indicates unqualified objectname
		},
		{
			Sql:             `SELECT * from s1.employees`,
			ExpectedObjects: []string{"s1.employees"},
			ExpectedSchemas: []string{"s1"},
		},
		{
			Sql:             `SELECT * from s2.employees where salary > (Select salary from s3.employees)`,
			ExpectedObjects: []string{"s3.employees", "s2.employees"},
			ExpectedSchemas: []string{"s3", "s2"},
		},
		{
			Sql:             `SELECT c.name, SUM(o.amount) AS total_spent FROM sales.customers c JOIN finance.orders o ON c.id = o.customer_id GROUP BY c.name HAVING SUM(o.amount) > 1000`,
			ExpectedObjects: []string{"sales.customers", "finance.orders"},
			ExpectedSchemas: []string{"sales", "finance"},
		},
		{
			Sql:             `SELECT name FROM hr.employees WHERE department_id IN (SELECT id FROM public.departments WHERE location_id IN (SELECT id FROM eng.locations WHERE country = 'USA'))`,
			ExpectedObjects: []string{"hr.employees", "public.departments", "eng.locations"},
			ExpectedSchemas: []string{"hr", "public", "eng"},
		},
		{
			Sql:             `SELECT name FROM sales.customers UNION SELECT name FROM marketing.customers;`,
			ExpectedObjects: []string{"sales.customers", "marketing.customers"},
			ExpectedSchemas: []string{"sales", "marketing"},
		},
		{
			Sql: `CREATE VIEW analytics.top_customers AS
					SELECT user_id, COUNT(*) as order_count	FROM public.orders GROUP BY user_id HAVING COUNT(*) > 10;`,
			ExpectedObjects: []string{"analytics.top_customers", "public.orders"},
			ExpectedSchemas: []string{"analytics", "public"},
		},
		{
			Sql: `WITH user_orders AS (
					SELECT u.id, o.id as order_id
					FROM users u
					JOIN orders o ON u.id = o.user_id
					WHERE o.amount > 100
				), order_items AS (
					SELECT o.order_id, i.product_id
					FROM order_items i
					JOIN user_orders o ON i.order_id = o.order_id
				)
				SELECT p.name, COUNT(oi.product_id) FROM products p
				JOIN order_items oi ON p.id = oi.product_id GROUP BY p.name;`,
			ExpectedObjects: []string{"users", "orders", "order_items", "user_orders", "products"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql: `UPDATE finance.accounts
						SET balance = balance + 1000
						WHERE account_id IN (
							SELECT account_id FROM public.users WHERE active = true
					);`,
			ExpectedObjects: []string{"finance.accounts", "public.users"},
			ExpectedSchemas: []string{"finance", "public"},
		},
		{
			Sql:             `SELECT classid, objid, refobjid FROM pg_depend WHERE refclassid = $1::regclass AND deptype = $2 ORDER BY 3`,
			ExpectedObjects: []string{"pg_depend"},
			ExpectedSchemas: []string{""},
		},
		{
			Sql:             `SELECT pg_advisory_unlock_shared(100);`,
			ExpectedObjects: []string{}, // empty slice represents no object collected
			ExpectedSchemas: []string{},
		},
		{
			Sql:             `SELECT xpath_exists('/employee/name', '<employee><name>John</name></employee>'::xml)`,
			ExpectedObjects: []string{},
			ExpectedSchemas: []string{},
		},
		{
			Sql: `SELECT t.id,
					xpath('/root/node', xmlparse(document t.xml_column)) AS extracted_nodes,
					s2.some_function()
				FROM s1.some_table t
				WHERE t.id IN (SELECT id FROM s2.some_function())
				AND pg_advisory_lock(t.id);`,
			ExpectedObjects: []string{"s1.some_table"},
			ExpectedSchemas: []string{"s1"},
		},
	}

	for _, tc := range tests {
		parseResult, err := Parse(tc.Sql)
		assert.NoError(t, err)

		objectsCollector := NewObjectCollector(TablesOnlyPredicate)
		processor := func(msg protoreflect.Message) error {
			objectsCollector.Collect(msg)
			return nil
		}

		visited := make(map[protoreflect.Message]bool)
		parseTreeMsg := GetProtoMessageFromParseTree(parseResult)
		err = TraverseParseTree(parseTreeMsg, visited, processor)
		assert.NoError(t, err)

		collectedObjects := objectsCollector.GetObjects()
		collectedSchemas := objectsCollector.GetSchemaList()

		assert.ElementsMatch(t, tc.ExpectedObjects, collectedObjects,
			"Objects list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedObjects, len(tc.ExpectedObjects), collectedObjects, len(collectedObjects))
		assert.ElementsMatch(t, tc.ExpectedSchemas, collectedSchemas,
			"Schema list mismatch for sql [%s]. Expected: %v(len=%d), Actual: %v(len=%d)", tc.Sql, tc.ExpectedSchemas, len(tc.ExpectedSchemas), collectedSchemas, len(collectedSchemas))
	}
}
