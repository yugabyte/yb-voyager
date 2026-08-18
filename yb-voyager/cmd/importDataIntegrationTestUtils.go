//go:build integration || integration_voyager_command

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
package cmd

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type TestTargetDB struct {
	Tconf tgtdb.TargetConf
	testcontainers.TestContainer
	tgtdb.TargetDB
}

var testYugabyteDBTarget *TestTargetDB

func setupYugabyteTestDb(t *testing.T) {
	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := yugabytedbContainer.Start(context.Background())
	testutils.FatalIfError(t, err)
	host, port, err := yugabytedbContainer.GetHostPort()
	testutils.FatalIfError(t, err)
	testYugabyteDBTarget = &TestTargetDB{
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

	tdb = testYugabyteDBTarget.TargetDB
	err = tdb.Init()
	testutils.FatalIfError(t, err)
	err = tdb.CreateVoyagerSchema()
	testutils.FatalIfError(t, err)
	err = tdb.InitConnPool()
	testutils.FatalIfError(t, err)
}

func assertIdentityColumnIsAlways(t *testing.T, conn *sql.DB, schema, table, column string) {
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
