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
package testutils

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

// TestTargetDB bundles a YugabyteDB test container with the tgtdb.TargetDB
// handle opened against it, for integration tests that import into a target.
type TestTargetDB struct {
	Tconf tgtdb.TargetConf
	testcontainers.TestContainer
	tgtdb.TargetDB
}

// SetupYugabyteTestDb starts a YugabyteDB test container, opens a TargetDB
// against it, creates the voyager metadata schema and initializes the
// connection pool. The caller owns Finalize().
func SetupYugabyteTestDb(t *testing.T) *TestTargetDB {
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := yugabytedbContainer.Start(context.Background())
	FatalIfError(t, err)
	host, port, err := yugabytedbContainer.GetHostPort()
	FatalIfError(t, err)
	target := &TestTargetDB{
		TestContainer: yugabytedbContainer,
		TargetDB: tgtdb.NewTargetDB(&tgtdb.TargetConf{
			TargetDBType: "yugabytedb",
			DBVersion:    yugabytedbContainer.GetConfig().DBVersion,
			User:         yugabytedbContainer.GetConfig().User,
			Password:     yugabytedbContainer.GetConfig().Password,
			Schemas:      []sqlname.Identifier{sqlname.NewIdentifier(constants.YUGABYTEDB, yugabytedbContainer.GetConfig().Schema)},
			DBName:       yugabytedbContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
		}),
	}

	err = target.TargetDB.Init()
	FatalIfError(t, err)
	err = target.TargetDB.CreateVoyagerSchema()
	FatalIfError(t, err)
	err = target.TargetDB.InitConnPool()
	FatalIfError(t, err)
	return target
}

// AssertIdentityColumnIsAlways asserts that schema.table.column is a
// GENERATED ALWAYS AS IDENTITY column on the target.
func AssertIdentityColumnIsAlways(t *testing.T, conn *sql.DB, schema, table, column string) {
	t.Helper()
	var identityGeneration string
	err := conn.QueryRow(
		`SELECT identity_generation FROM information_schema.columns
		 WHERE table_schema = $1 AND table_name = $2 AND column_name = $3`,
		schema, table, column,
	).Scan(&identityGeneration)
	assert.NoError(t, err, "querying identity_generation for %s.%s.%s", schema, table, column)
	assert.Equal(t, "ALWAYS", identityGeneration,
		"expected identity_generation=ALWAYS for %s.%s.%s, got %q", schema, table, column, identityGeneration)
}
