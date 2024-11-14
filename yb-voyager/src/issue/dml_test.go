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

package issue

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/yugabytedb"
)

func TestXMLFunctionIssue(t *testing.T) {
	ctx := context.Background()
	yugabytedbContainer, err := yugabytedb.Run(
		ctx,
		"yugabytedb/yugabyte:2024.1.3.0-b105",
		// "yugabytedb/yugabyte:2024.1.3",
	)
	assert.NoError(t, err)
	defer yugabytedbContainer.Terminate(context.Background())

	connStr, err := yugabytedbContainer.YSQLConnectionString(ctx, "sslmode=disable")
	assert.NoError(t, err)
	conn, err := pgx.Connect(ctx, connStr)
	assert.NoError(t, err)
	defer conn.Close(context.Background())

	_, err = conn.Exec(ctx, "SELECT xmlconcat('<abc/>', '<bar>foo</bar>')")
	assert.ErrorContains(t, err, "unsupported XML feature")

}
