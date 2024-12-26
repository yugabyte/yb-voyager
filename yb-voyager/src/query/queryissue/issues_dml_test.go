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

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/yugabytedb"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

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

func testCopyOnErrorIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	// In case the COPY ... ON_ERROR construct gets supported in the future, this test will fail with a different error message-something related to the data.csv file not being found.
	_, err = conn.Exec(ctx, `COPY pg_largeobject (loid, pageno, data) FROM '/path/to/data.csv' WITH (FORMAT csv, HEADER true, ON_ERROR IGNORE);`)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "ERROR: option \"on_error\" not recognized (SQLSTATE 42601)", copyOnErrorIssue)

}

func testCopyFromWhereIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	// In case the COPY FROM ...  WHERE construct gets supported in the future, this test will fail with a different error message-something related to the data.csv file not being found.
	_, err = conn.Exec(ctx, `COPY pg_largeobject (loid, pageno, data) FROM '/path/to/data.csv' WHERE loid = 1 WITH (FORMAT csv, HEADER true);`)
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

func TestDMLIssuesInYBVersion(t *testing.T) {
	var err error
	ybVersion := os.Getenv("YB_VERSION")
	if ybVersion == "" {
		panic("YB_VERSION env variable is not set. Set YB_VERSION=2024.1.3.0-b105 for example")
	}

	ybVersionWithoutBuild := strings.Split(ybVersion, "-")[0]
	testYbVersion, err = ybversion.NewYBVersion(ybVersionWithoutBuild)
	fatalIfError(t, err)

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
	success := t.Run(fmt.Sprintf("%s-%s", "lo functions", ybVersion), testLOFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "regex functions", ybVersion), testRegexFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "copy on error", ybVersion), testCopyOnErrorIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "copy from where", ybVersion), testCopyFromWhereIssue)
	assert.True(t, success)
	success = t.Run(fmt.Sprintf("%s-%s", "json constructor functions", ybVersion), testJsonConstructorFunctions)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "json query functions", ybVersion), testJsonQueryFunctions)
	assert.True(t, success)

}
