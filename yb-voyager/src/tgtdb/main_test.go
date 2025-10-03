//go:build integration

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

func createTestDBTarget(ctx context.Context, config *testcontainers.ContainerConfig) *TestDB {
	container := testcontainers.NewTestContainer(config.DBType, config)
	err := container.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start %s container: %v", config.DBType, err)
	}

	host, port, err := container.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}

	sslMode := ""
	if config.DBType == "postgresql" {
		sslMode = "disable"
	}

	return &TestDB{
		TestContainer: container,
		TargetDB: NewTargetDB(&TargetConf{
			TargetDBType: config.DBType,
			DBVersion:    container.GetConfig().DBVersion,
			User:         container.GetConfig().User,
			Password:     container.GetConfig().Password,
			Schema:       container.GetConfig().Schema,
			DBName:       container.GetConfig().DBName,
			Host:         host,
			Port:         port,
			SSLMode:      sslMode,
		}),
	}
}

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// PostgreSQL setup
	postgresConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.POSTGRESQL,
	}
	testPostgresTarget = createTestDBTarget(ctx, postgresConfig)
	err := testPostgresTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresTarget.Finalize()

	// Oracle setup
	oracleConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.ORACLE,
	}
	testOracleTarget = createTestDBTarget(ctx, oracleConfig)
	err = testOracleTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to oracle database: %w", err)
	}
	defer testOracleTarget.Finalize()

	// YugabyteDB setup
	yugabytedbConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.YUGABYTEDB,
	}
	testYugabyteDBTarget = createTestDBTarget(ctx, yugabytedbConfig)
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
