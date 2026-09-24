//go:build integration_live_migration || failpoint_export || failpoint_import || failpoint_cutover

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
package testlivemigration

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// This inserts some rows in target table having sequence and validates if the ids ingested are correct or not
func assertSequenceValues(t *testing.T, startID int, endId int, ybConn *sql.DB, tableName string) error {
	_, err := ybConn.Exec(fmt.Sprintf(`INSERT INTO %s (name, email, description)
SELECT
	md5(random()::text),                                      -- name
	md5(random()::text) || '@example.com',                    -- email
	repeat(md5(random()::text), 10)                           -- description (~320 chars)
FROM generate_series(%d, %d);`, tableName, startID, endId))
	if err != nil {
		return fmt.Errorf("failed to insert into target: %w", err)
	}

	ids := []int{}
	for i := startID; i <= endId; i++ {
		ids = append(ids, i)
	}
	query := fmt.Sprintf("SELECT id from %s where id IN (%s) ORDER BY id;", tableName, strings.Join(lo.Map(ids, func(id int, _ int) string {
		return strconv.Itoa(id)
	}), ", "))
	rows, err := ybConn.Query(query)
	testutils.FatalIfError(t, err, "failed to read data")
	var resIds []int
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		testutils.FatalIfError(t, err, "error scanning rows")
		resIds = append(resIds, id)
	}
	if !assert.Equal(t, ids, resIds) {
		return fmt.Errorf("ids do not match %v != %v", ids, resIds)
	}
	return nil
}
