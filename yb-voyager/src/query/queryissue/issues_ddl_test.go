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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/yugabytedb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

var (
	testYugabytedbContainer *yugabytedb.Container
	testYugabytedbConnStr   string
	testYbVersion           *ybversion.YBVersion
)

func getConn() (*pgx.Conn, error) {
	ctx := context.Background()
	var connStr string
	var err error
	if testYugabytedbConnStr != "" {
		connStr = testYugabytedbConnStr
	} else {
		connStr, err = testYugabytedbContainer.YSQLConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return nil, err
		}
	}

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func fatalIfError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func assertErrorCorrectlyThrownForIssueForYBVersion(t *testing.T, execErr error, expectedError string, issue issue.Issue) {
	isFixed, err := issue.IsFixedIn(testYbVersion)
	fatalIfError(t, err)

	if isFixed {
		assert.NoError(t, execErr)
	} else {
		assert.ErrorContains(t, execErr, expectedError)
	}
}

func getConnWithNoticeHandler(noticeHandler func(*pgconn.PgConn, *pgconn.Notice)) (*pgx.Conn, error) {
	ctx := context.Background()
	var connStr string
	var err error
	if testYugabytedbConnStr != "" {
		connStr = testYugabytedbConnStr
	} else {
		connStr, err = testYugabytedbContainer.YSQLConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return nil, err
		}
	}

	conf, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	conf.OnNotice = noticeHandler
	conn, err := pgx.ConnectConfig(ctx, conf)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func testXMLFunctionIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "SELECT xmlconcat('<abc/>', '<bar>foo</bar>')")
	assert.ErrorContains(t, err, "unsupported XML feature")
}

func testStoredGeneratedFunctionsIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
		CREATE TABLE rectangles (
		id SERIAL PRIMARY KEY,
		length NUMERIC NOT NULL,
		width NUMERIC NOT NULL,
		area NUMERIC GENERATED ALWAYS AS (length * width) STORED
	)`)
	assert.ErrorContains(t, err, "syntax error")
}

func testUnloggedTableIssue(t *testing.T) {
	noticeFound := false
	noticeHandler := func(conn *pgconn.PgConn, notice *pgconn.Notice) {
		if notice != nil && notice.Message != "" {
			assert.Equal(t, "unlogged option is currently ignored in YugabyteDB, all non-temp tables will be logged", notice.Message)
			noticeFound = true
		}
	}
	ctx := context.Background()
	conn, err := getConnWithNoticeHandler(noticeHandler)
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "CREATE UNLOGGED TABLE unlogged_table (a int)")
	// in 2024.2, UNLOGGED no longer throws an error, just a notice
	if noticeFound {
		return
	} else {
		assert.ErrorContains(t, err, "UNLOGGED database object not supported yet")
	}

}

func testAlterTableAddPKOnPartitionIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE orders2 (
	order_id bigint NOT NULL,
	order_date timestamp
	) PARTITION BY RANGE (order_date);
	ALTER TABLE orders2 ADD PRIMARY KEY (order_id,order_date)`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "changing primary key of a partitioned table is not yet implemented", alterTableAddPKOnPartitionIssue)
}

func TestDDLIssuesInYBVersion(t *testing.T) {
	var err error
	ybVersion := os.Getenv("YB_VERSION")
	if ybVersion == "" {
		panic("YB_VERSION env variable is not set. Set YB_VERSIONS=2024.1.3.0-b105 for example")
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
	var success bool
	success = t.Run(fmt.Sprintf("%s-%s", "xml functions", ybVersion), testXMLFunctionIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "stored generated functions", ybVersion), testStoredGeneratedFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "unlogged table", ybVersion), testUnloggedTableIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "alter table add PK on partition", ybVersion), testAlterTableAddPKOnPartitionIssue)
	assert.True(t, success)

}
