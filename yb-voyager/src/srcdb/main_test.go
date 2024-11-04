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
package srcdb

import (
	"context"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"

	_ "github.com/godror/godror"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	postgresTestDB *TestDB
	oracleTestDB   *TestDB
	mysqlTestDB    *TestDB
	yugabyteTestDB *TestDB
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	// setting source db type, version and defaults
	pgSource := &Source{
		DBType:    "postgresql",
		DBVersion: "11",
		User:      "ybvoyager",
		Password:  "postgres",
		SSLMode:   "disable",
		Port:      5432,
		Schema:    "public",
	}
	oracleSource := &Source{
		DBType:    "oracle",
		DBVersion: "21",
		User:      "ybvoyager",
		Password:  "password",
		Port:      1521,
		DBName:    "DMS",
		Schema:    "YBVOYAGER",
	}
	mysqlSource := &Source{
		DBType:    "mysql",
		DBVersion: "8.4",
		User:      "ybvoyager",
		Password:  "password",
		Port:      3306,
		DBName:    "dms",
		SSLMode:   "disable",
	}
	ybSource := &Source{
		DBType:    "yugabytedb",
		DBVersion: "2.20.7.1-b10",
		User:      "yugabyte",
		Password:  "password",
		SSLMode:   "disable",
		Port:      5433,
		Schema:    "public",
	}

	postgresTestDB, err = StartTestDB(ctx, pgSource)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, postgresTestDB)

	oracleTestDB, err = StartTestDB(ctx, oracleSource)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, oracleTestDB)

	mysqlTestDB, err = StartTestDB(ctx, mysqlSource)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, mysqlTestDB)

	yugabyteTestDB, err = StartTestDB(ctx, ybSource)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, yugabyteTestDB)

	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()
	os.Exit(exitCode)
}
