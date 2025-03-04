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
	testPostgresSource = &TestDB{
		TestContainer: postgresContainer,
		Source: &Source{
			DBType:    "postgresql",
			DBVersion: postgresContainer.GetConfig().DBVersion,
			User:      postgresContainer.GetConfig().User,
			Password:  postgresContainer.GetConfig().Password,
			Schema:    postgresContainer.GetConfig().Schema,
			DBName:    postgresContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   "disable",
		},
	}
	err = testPostgresSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresSource.DB().Disconnect()

	oracleContainer := testcontainers.NewTestContainer("oracle", nil)
	err = oracleContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start oracle container: %v", err)
	}
	host, port, err = oracleContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}

	testOracleSource = &TestDB{
		TestContainer: oracleContainer,
		Source: &Source{
			DBType:    "oracle",
			DBVersion: oracleContainer.GetConfig().DBVersion,
			User:      oracleContainer.GetConfig().User,
			Password:  oracleContainer.GetConfig().Password,
			Schema:    oracleContainer.GetConfig().Schema,
			DBName:    oracleContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
		},
	}

	err = testOracleSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to oracle database: %w", err)
	}
	defer testOracleSource.DB().Disconnect()

	mysqlContainer := testcontainers.NewTestContainer("mysql", nil)
	err = mysqlContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start mysql container: %v", err)
	}
	host, port, err = mysqlContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testMySQLSource = &TestDB{
		TestContainer: mysqlContainer,
		Source: &Source{
			DBType:    "mysql",
			DBVersion: mysqlContainer.GetConfig().DBVersion,
			User:      mysqlContainer.GetConfig().User,
			Password:  mysqlContainer.GetConfig().Password,
			Schema:    mysqlContainer.GetConfig().Schema,
			DBName:    mysqlContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   "disable",
		},
	}

	err = testMySQLSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to mysql database: %w", err)
	}
	defer testMySQLSource.DB().Disconnect()

	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb container: %v", err)
	}
	host, port, err = yugabytedbContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testYugabyteDBSource = &TestDB{
		TestContainer: yugabytedbContainer,
		Source: &Source{
			DBType:    "yugabytedb",
			DBVersion: yugabytedbContainer.GetConfig().DBVersion,
			User:      yugabytedbContainer.GetConfig().User,
			Password:  yugabytedbContainer.GetConfig().Password,
			Schema:    yugabytedbContainer.GetConfig().Schema,
			DBName:    yugabytedbContainer.GetConfig().DBName,
			Host:      host,
			Port:      port,
			SSLMode:   "disable",
		},
	}

	err = testYugabyteDBSource.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to yugabytedb database: %w", err)
	}
	defer testYugabyteDBSource.DB().Disconnect()

	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	// cleanig up all the running containers
	testcontainers.TerminateAllContainers()

	os.Exit(exitCode)
}
