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
package tgtdb

import (
	"context"
	"os"
	"testing"

	_ "github.com/godror/godror"
	_ "github.com/jackc/pgx/v5/stdlib"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	postgresTestDB *TestDB
	oracleTestDB   *TestDB
	yugabyteTestDB *TestDB
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	// setting target db type, version and defaults in tconf
	pgTargetConf := &TargetConf{
		TargetDBType: "postgresql",
		DBVersion:    "11",
		User:         "ybvoyager",
		Password:     "postgres",
		SSLMode:      "disable",
		Port:         5432,
		Schema:       "public",
		DBName:       "postgres",
	}

	oracleTargetConf := &TargetConf{
		TargetDBType: "oracle",
		DBVersion:    "21",
		User:         "ybvoyager",
		Password:     "password",
		Port:         1521,
		DBName:       "DMS",
		Schema:       "YBVOYAGER",
	}

	ybTargetConf := &TargetConf{
		TargetDBType: "yugabytedb",
		DBVersion:    "2.20.7.1-b10",
		User:         "yugabyte",
		Password:     "password",
		SSLMode:      "disable",
		Port:         5433,
		Schema:       "public",
	}

	postgresTestDB, err = StartTestDB(ctx, pgTargetConf)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, postgresTestDB)

	oracleTestDB, err = StartTestDB(ctx, oracleTargetConf)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, oracleTestDB)

	yugabyteTestDB, err = StartTestDB(ctx, ybTargetConf)
	if err != nil {
		utils.ErrExit("Failed to start testDB: %v", err)
	}
	defer StopTestDB(ctx, yugabyteTestDB)

	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()
	os.Exit(exitCode)
}
