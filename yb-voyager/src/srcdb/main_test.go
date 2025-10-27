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
package srcdb

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
	*Source
}

var (
	testPostgresSource   *TestDB
	testOracleSource     *TestDB
	testMySQLSource      *TestDB
	testYugabyteDBSource *TestDB
)

func createTestDBSource(ctx context.Context, config *testcontainers.ContainerConfig) *TestDB {
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
	if config.DBType == "postgresql" || config.DBType == "mysql" || config.DBType == "yugabytedb" {
		sslMode = "disable"
	}

	testDB := &TestDB{
		TestContainer: container,
		Source: &Source{
			DBType:    config.DBType,
			DBVersion: container.GetConfig().DBVersion,
			User:      container.GetConfig().User,
			Password:  container.GetConfig().Password,
			Schema:    container.GetConfig().Schema,
			DBName:    container.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   sslMode,
		},
	}

	err = testDB.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to %s database: %v", config.DBType, err)
	}

	return testDB
}

func destroyTestDBSource(ctx context.Context, testDB *TestDB) {
	testDB.DB().Disconnect()
	testDB.TestContainer.Terminate(ctx)
}

// TestMain setup test database containers for all db types
// and run the tests
// and clean up the containers after the tests are run
func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// PostgreSQL setup
	postgresConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.POSTGRESQL,
	}
	testPostgresSource = createTestDBSource(ctx, postgresConfig)

	// Oracle setup
	oracleConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.ORACLE,
	}
	testOracleSource = createTestDBSource(ctx, oracleConfig)

	// MySQL setup
	mysqlConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.MYSQL,
	}
	testMySQLSource = createTestDBSource(ctx, mysqlConfig)

	// YugabyteDB setup
	yugabytedbConfig := &testcontainers.ContainerConfig{
		DBType: testcontainers.YUGABYTEDB,
	}
	testYugabyteDBSource = createTestDBSource(ctx, yugabytedbConfig)

	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	// cleanup after the tests
	destroyTestDBSource(ctx, testPostgresSource)
	destroyTestDBSource(ctx, testOracleSource)
	destroyTestDBSource(ctx, testMySQLSource)
	destroyTestDBSource(ctx, testYugabyteDBSource)
	testcontainers.TerminateAllContainers() // safety net in case any of them still left

	os.Exit(exitCode)
}
