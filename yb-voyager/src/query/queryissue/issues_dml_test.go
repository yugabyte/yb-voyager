//go:build issues_integration

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
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/yugabytedb"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func testXMLFunctionIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "SELECT xmlconcat('<abc/>', '<bar>foo</bar>')")
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "unsupported XML feature", xmlFunctionsIssue)
}

func testAdvisoryLocks(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "SELECT pg_advisory_unlock_shared(100);")
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "advisory locks are not yet implemented", advisoryLocksIssue)
}

func testSystemColumns(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE system_col_test(id int, val text);
	`)
	assert.NoError(t, err)
	for _, col := range unsupportedSysCols.ToSlice() {
		query := fmt.Sprintf("select %s from system_col_test", col)
		_, err = conn.Exec(ctx, query)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, fmt.Sprintf(`System column "%s" is not supported yet`, col), systemColumnsIssue)
	}
}

func testLOFunctionsIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE EXTENSION lo;
	SELECT lo_create('2342');`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "Transaction for catalog table write operation 'pg_largeobject_metadata' not found", loDatatypeIssue)
}

func testJsonbSubscriptingIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `SELECT ('{"a": {"b": {"c": 1}}}'::jsonb)['a']['b']['c'];`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "cannot subscript type jsonb because it is not an array", jsonbSubscriptingIssue)
}

func testRegexFunctionsIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	stmts := []string{
		`SELECT regexp_count('This is an example. Another example. Example is a common word.', 'example')`,
		`SELECT regexp_instr('This is an example. Another example. Example is a common word.', 'example')`,
		`SELECT regexp_like('This is an example. Another example. Example is a common word.', 'example')`,
	}

	for _, stmt := range stmts {
		_, err = conn.Exec(ctx, stmt)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", regexFunctionsIssue)
	}
}

func testFetchWithTiesIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())

	stmts := []string{
		`CREATE TABLE fetch_table(id int, salary bigint);
		SELECT * FROM fetch_table
		ORDER BY salary DESC
		FETCH FIRST 2 ROWS WITH TIES;`,
	}

	for _, stmt := range stmts {
		_, err = conn.Exec(ctx, stmt)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `syntax error at or near "WITH"`, regexFunctionsIssue)
	}
}

func testCopyOnErrorIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	// Write the csv content to the file
	//4th row has error
	csvData := `id,name,value
1,Item1,10
2,Item2,20
3,Item3,30
4|Item4,40
5,Item5,50
6,Item6,60
7,Item7,70
8,Item8,80
9,Item9,90
10,Item10,100`

	fileName, err := testutils.CreateTempFile("/tmp", csvData, "csv")
	assert.NoError(t, err)

	defer conn.Close(context.Background())

	// In case the COPY ... ON_ERROR construct gets supported in the future, this test will fail with a different error message-something related to the data.csv file not being found.
	_, err = conn.Exec(ctx, fmt.Sprintf(`	CREATE TABLE my_table_copy_error (
		id INT,
		name TEXT,
		value INT
	);
	
		COPY my_table_copy_error (id, name, value) 
	FROM '%s'
	WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE)`, fileName))
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "ERROR: option \"on_error\" not recognized (SQLSTATE 42601)", copyOnErrorIssue)
}

func testCopyFromWhereIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	// Write the csv content to the file
	csvData := `id,name,value
1,Item1,10
2,Item2,20
3,Item3,30
4,Item4,40
5,Item5,50
6,Item6,60
7,Item7,70
8,Item8,80
9,Item9,90
10,Item10,100`

	fileName, err := testutils.CreateTempFile("/tmp", csvData, "csv")
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, fmt.Sprintf(`
	CREATE TABLE my_table_copy_where (
    id INT,
    name TEXT,
    value INT
);

	COPY my_table_copy_where (id, name, value) 
FROM '%s'
WITH (FORMAT csv, HEADER true)
WHERE id <=5;`, fileName))
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "ERROR: syntax error at or near \"WHERE\" (SQLSTATE 42601)", copyFromWhereIssue)
}

func testJsonConstructorFunctions(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)
	sqls := map[string]string{
		`select json_object('code' VALUE 'P123', 'title': 'Jaws');`: `syntax error at or near "VALUE"`,
		`select JSON_ARRAYAGG('[1, "2", null]');`:                   `does not exist`,
		`SELECT json_objectagg(k VALUE v) AS json_result
	FROM (VALUES ('a', 1), ('b', 2), ('c', 3)) AS t(k, v);`: `syntax error at or near "VALUE"`,
		`SELECT JSON_ARRAY('PostgreSQL', 12, TRUE, NULL) AS json_array;`: `does not exist`,
	}
	for sql, expectedErr := range sqls {
		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)

		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, expectedErr, jsonConstructorFunctionsIssue)
	}
}

func testJsonPredicateIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `SELECT js, js IS JSON "json?" FROM (VALUES ('123'), ('"abc"'), ('{"a": "b"}'), ('[1,2]'),('abc')) foo(js);`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `syntax error at or near "JSON"`, jsonConstructorFunctionsIssue)
}

func testJsonQueryFunctions(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)
	sqls := []string{
		`SELECT id, JSON_QUERY(details, '$.author') AS author
FROM books;`,
		`SELECT 
    id, 
    JSON_VALUE(details, '$.title') AS title,
    JSON_VALUE(details, '$.price')::NUMERIC AS price
FROM books;`,
		`SELECT id, details
FROM books
WHERE JSON_EXISTS(details, '$.author');`,
	}
	for _, sql := range sqls {
		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)

		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `does not exist`, jsonConstructorFunctionsIssue)
	}

	jsonTableSQL := `SELECT * FROM json_table(
			'[{"a":10,"b":20},{"a":30,"b":40}]'::jsonb,
			'$[*]'
			COLUMNS (
				column_a int4 path '$.a',
				column_b int4 path '$.b'
			)
		);`
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, jsonTableSQL)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `syntax error at or near "COLUMNS"`, jsonConstructorFunctionsIssue)
}

func testMergeStmtIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)
	sqls := []string{`
	MERGE INTO customer_account ca
USING recent_transactions t
ON t.customer_id = ca.customer_id
WHEN MATCHED THEN
  UPDATE SET balance = balance + transaction_value
WHEN NOT MATCHED THEN
  INSERT (customer_id, balance)
  VALUES (t.customer_id, t.transaction_value);
`,
		`
  MERGE INTO wines w
USING wine_stock_changes s
ON s.winename = w.winename
WHEN NOT MATCHED AND s.stock_delta > 0 THEN
  INSERT VALUES(s.winename, s.stock_delta)
WHEN MATCHED AND w.stock + s.stock_delta > 0 THEN
  UPDATE SET stock = w.stock + s.stock_delta
WHEN MATCHED THEN
  DELETE
RETURNING merge_action(), w.*;
	`, // MERGE ... RETURNING statement >PG15 feature
	}

	for _, sql := range sqls {
		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		var errMsg string
		switch {
		case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0):
			errMsg = "This statement not supported yet"
		default:
			errMsg = `syntax error at or near "MERGE"`
		}
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, mergeStatementIssue)
	}

}

func testRangeAggFunctionsIssue(t *testing.T) {
	sqls := []string{
		`CREATE TABLE events (
    id SERIAL PRIMARY KEY,
    event_range daterange
);

INSERT INTO events (event_range) VALUES
    ('[2024-01-01, 2024-01-10]'::daterange),
    ('[2024-01-05, 2024-01-15]'::daterange),
    ('[2024-01-20, 2024-01-25]'::daterange);

SELECT range_agg(event_range) AS union_of_ranges
FROM events;`,

		`SELECT range_intersect_agg(event_range) AS intersection_of_ranges
FROM events;`,
	}

	for _, sql := range sqls {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `does not exist`, rangeAggregateFunctionIssue)
	}
}

func testAnyValueAggFunctions(t *testing.T) {
	sql := `CREATE TABLE any_value_ex (
    department TEXT,
    employee_name TEXT,
    salary NUMERIC
);

INSERT INTO any_value_ex VALUES
('HR', 'Alice', 50000),
('HR', 'Bob', 55000),
('IT', 'Charlie', 60000),
('IT', 'Diana', 62000);

SELECT
    department,
    any_value(employee_name) AS any_employee
FROM any_value_ex
GROUP BY department;`

	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, sql)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `does not exist`, anyValueAggregateFunction)

}

func testNonDecimalIntegerLiteralIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)
	sqls := []string{
		`CREATE VIEW zz AS
    SELECT
        5678901234 as DEC,
        0x1527D27F2 as hex,
        0o52237223762 as oct,
        0b101010010011111010010011111110010 as bin;`,
		`SELECT 5678901234, 0b101010010011111010010011111110010 as binary;`,
	}
	for _, sql := range sqls {
		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		var errMsg string
		switch {
		case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0):
			errMsg = `trailing junk after numeric literal at or near`
		default:
			errMsg = `syntax error at or near "as"`
		}
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, nonDecimalIntegerLiteralIssue)
	}
}
func testCTEWithMaterializedIssue(t *testing.T) {
	sqls := map[string]string{`
	CREATE TABLE big_table(key text, ref text, c1 int, c2 int);
	WITH w AS NOT MATERIALIZED (
		SELECT * FROM big_table
	)
	SELECT * FROM w AS w1 JOIN w AS w2 ON w1.key = w2.ref
	WHERE w2.key = '123';`: `syntax error at or near "NOT"`,
		`CREATE TABLE big_table1(key text, ref text, c1 int, c2 int);
		WITH w AS MATERIALIZED (
		SELECT * FROM big_table1
	)
	SELECT * FROM w AS w1 JOIN w AS w2 ON w1.key = w2.ref
	WHERE w2.key = '123';`: `syntax error at or near "MATERIALIZED"`,
	}
	for sql, errMsg := range sqls {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)

		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, cteWithMaterializedIssue)
	}
}

func testEventsListenNotifyIssue(t *testing.T) {
	sqls := map[string]string{
		`LISTEN my_table_changes;`:                                       `LISTEN not supported yet and will be ignored`,
		`NOTIFY my_table_changes, 'Row inserted: id=1, name=Alice';`:     `NOTIFY not supported yet and will be ignored`,
		`UNLISTEN my_notification;`:                                      `UNLISTEN not supported yet and will be ignored`,
		`SELECT pg_notify('my_notification', 'Payload from pg_notify');`: `NOTIFY not supported yet and will be ignored`,
	}
	for sql, warnMsg := range sqls {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		connConfig := conn.Config()
		connConfig.OnNotice = func(conn *pgconn.PgConn, n *pgconn.Notice) {
			if n != nil {
				assert.Contains(t, n.Message, warnMsg)
			}
		}

		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		assert.NoError(t, err)

		assertErrorCorrectlyThrownForIssueForYBVersion(t, fmt.Errorf(""), "", listenNotifyIssue)
	}
}

func TestDMLIssuesInYBVersion(t *testing.T) {
	var err error
	ybVersion := os.Getenv("YB_VERSION")
	if ybVersion == "" {
		panic("YB_VERSION env variable is not set. Set YB_VERSION=2024.1.3.0-b105 for example")
	}

	ybVersionWithoutBuild := strings.Split(ybVersion, "-")[0]
	testYbVersion, err = ybversion.NewYBVersion(ybVersionWithoutBuild)
	testutils.FatalIfError(t, err)

	testYugabytedbConnStr = os.Getenv("YB_CONN_STR")
	if testYugabytedbConnStr == "" {
		// spawn yugabytedb container
		var err error
		ctx := context.Background()
		testYugabytedbContainer, err = yugabytedb.Run(
			ctx,
			"yugabytedb/yugabyte:"+ybVersion,
		)
		assert.NoError(t, err)
		defer testYugabytedbContainer.Terminate(context.Background())
	}

	// run tests

	success := t.Run(fmt.Sprintf("%s-%s", "xml functions", ybVersion), testXMLFunctionIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "advisory locks", ybVersion), testAdvisoryLocks)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "system columns", ybVersion), 	testSystemColumns)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "lo functions", ybVersion), testLOFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "regex functions", ybVersion), testRegexFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "fetch with ties", ybVersion), testFetchWithTiesIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "copy on error", ybVersion), testCopyOnErrorIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "copy from where", ybVersion), testCopyFromWhereIssue)
	assert.True(t, success)
	success = t.Run(fmt.Sprintf("%s-%s", "json constructor functions", ybVersion), testJsonConstructorFunctions)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "json query functions", ybVersion), testJsonQueryFunctions)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "merge statement", ybVersion), testMergeStmtIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "json subscripting", ybVersion), testJsonbSubscriptingIssue)
	assert.True(t, success)
	success = t.Run(fmt.Sprintf("%s-%s", "any-value aggregate functions", ybVersion), testAnyValueAggFunctions)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "range aggregate functions", ybVersion), testRangeAggFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "json type predicate", ybVersion), testJsonPredicateIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "Non-decimal integer literal", ybVersion), testNonDecimalIntegerLiteralIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "cte with materialized cluase", ybVersion), testCTEWithMaterializedIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "events listen / notify", ybVersion), testEventsListenNotifyIssue)
	assert.True(t, success)

}
