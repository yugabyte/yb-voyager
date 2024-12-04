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
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/yugabytedb"
)

var (
	yugabytedbContainer *yugabytedb.Container
	versions            = []string{}
)

func testXMLFunctionIssue(t *testing.T) {
	ctx := context.Background()
	connStr, err := yugabytedbContainer.YSQLConnectionString(ctx, "sslmode=disable")
	assert.NoError(t, err)
	conn, err := pgx.Connect(ctx, connStr)
	assert.NoError(t, err)
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "SELECT xmlconcat('<abc/>', '<bar>foo</bar>')")
	assert.ErrorContains(t, err, "unsupported XML feature")
}

func TestDDLIssuesInAllVersions(t *testing.T) {
	for _, version := range versions {
		var err error
		ctx := context.Background()
		yugabytedbContainer, err = yugabytedb.Run(
			ctx,
			"yugabytedb/yugabyte:"+version,
		)
		assert.NoError(t, err)
		defer yugabytedbContainer.Terminate(context.Background())
		success := t.Run(fmt.Sprintf("%s-%s", "xml functions", version), func(t *testing.T) {
			testXMLFunctionIssue(t)
		})
		assert.True(t, success)

		yugabytedbContainer.Terminate(context.Background())
	}
}

func init() {
	versionsToTest := os.Getenv("YB_VERSIONS")
	if versionsToTest == "" {
		panic("YB_VERSIONS env variable is not set. Set YB_VERSIONS=2024.1.3.0-b105,2.23.1.0-b220 for example")
	}
	versions = strings.Split(versionsToTest, ",")
}
