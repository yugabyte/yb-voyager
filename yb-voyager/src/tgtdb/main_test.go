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
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

type TestDB struct {
	testcontainers.TestContainer
	TargetDB
}

var (
	testPostgresTarget   *TestDB
	testOracleTarget     *TestDB
	testYugabyteDBTarget *TestDB
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	host, port, err := postgresContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testPostgresTarget = &TestDB{
		TestContainer: postgresContainer,
		TargetDB: NewTargetDB(&TargetConf{
			TargetDBType: "postgresql",
			DBVersion:    postgresContainer.GetConfig().DBVersion,
			User:         postgresContainer.GetConfig().User,
			Password:     postgresContainer.GetConfig().Password,
			Schema:       postgresContainer.GetConfig().Schema,
			DBName:       postgresContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
			SSLMode:      "disable",
		}),
	}

	err = testPostgresTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresTarget.Finalize()

	// oracleContainer := testcontainers.NewTestContainer("oracle", nil)
	// _ = oracleContainer.Start(ctx)
	// host, port, err = oracleContainer.GetHostPort()
	// if err != nil {
	// 	utils.ErrExit("%v", err)
	// }
	// testOracleTarget = &TestDB2{
	// 	Container: oracleContainer,
	// 	TargetDB: NewTargetDB(&TargetConf{
	// 		TargetDBType: "oracle",
	// 		DBVersion:    oracleContainer.GetConfig().DBVersion,
	// 		User:         oracleContainer.GetConfig().User,
	// 		Password:     oracleContainer.GetConfig().Password,
	// 		Schema:       oracleContainer.GetConfig().Schema,
	// 		DBName:       oracleContainer.GetConfig().DBName,
	// 		Host:         host,
	// 		Port:         port,
	// 	}),
	// }

	// err = testOracleTarget.Init()
	// if err != nil {
	// 	utils.ErrExit("Failed to connect to oracle database: %w", err)
	// }
	// defer testOracleTarget.Finalize()

	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb container: %v", err)
	}
	host, port, err = yugabytedbContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testYugabyteDBTarget = &TestDB{
		TestContainer: yugabytedbContainer,
		TargetDB: NewTargetDB(&TargetConf{
			TargetDBType: "yugabytedb",
			DBVersion:    yugabytedbContainer.GetConfig().DBVersion,
			User:         yugabytedbContainer.GetConfig().User,
			Password:     yugabytedbContainer.GetConfig().Password,
			Schema:       yugabytedbContainer.GetConfig().Schema,
			DBName:       yugabytedbContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
		}),
	}

	err = testYugabyteDBTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to yugabytedb database: %w", err)
	}
	defer testYugabyteDBTarget.Finalize()

	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	// cleaning up all the running containers
	testcontainers.TerminateAllContainers()

	os.Exit(exitCode)
}
