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
	"fmt"
	"io"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainersself "github.com/yugabyte/yb-voyager/yb-voyager/testcontainers/containers"
	"gotest.tools/assert"
)

type TestDB struct {
	Container testcontainers.Container
	*Source
}

type TestDB2 struct {
	testcontainersself.TestContainer
	*Source
}

func (tdb *TestDB) GetContainerHostPort(ctx context.Context) (string, int) {
	host, err := tdb.Container.Host(ctx)
	if err != nil {
		utils.ErrExit("failed to fetch host for container %T: %v", tdb.Container, err)
	}

	portStr := strconv.Itoa(tdb.Source.Port)
	port, err := tdb.Container.MappedPort(ctx, nat.Port(portStr))
	if err != nil {
		utils.ErrExit("failed to fetch mapped port for %s container: %v", tdb.Source.DBType, err)
	}

	return host, port.Int()
}

func StartTestDB(ctx context.Context, source *Source) (*TestDB, error) {
	testDB := &TestDB{
		Source: source,
	}

	var err error
	switch source.DBType {
	case "postgresql":
		testDB.Container, err = startPostgreSQLContainer(ctx, source)
	case "oracle":
		testDB.Container, err = startOracleContainer(ctx, source)
	case "mysql":
		testDB.Container, err = startMysqlContainer(ctx, source)
	case "yugabytedb":
		testDB.Container, err = startYugabyteDBContainer(ctx, source)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", source.DBType)
	}

	if err != nil {
		reader, err := testDB.Container.Logs(ctx)
		if err != nil {
			fmt.Printf("failed to get container logs: %v", err)
		} else {
			fmt.Println("=== Container Logs ===")
			data, err := io.ReadAll(reader)
			if err != nil {
				panic(err)
			}
			fmt.Printf("%s\n", string(data))
			fmt.Println("=== End of Logs ===")
		}
		return nil, fmt.Errorf("failed to start '%s' test container: %w", source.DBType, err)
	}

	source.Host, source.Port = testDB.GetContainerHostPort(ctx)
	log.Infof("fetched container host=%s, port=%d", source.Host, source.Port)
	err = source.DB().Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return testDB, nil
}

// StopTestDB cleans up resources after tests are complete
func StopTestDB(ctx context.Context, testDB *TestDB) {
	testDB.Source.DB().Disconnect()
	if err := testDB.Container.Terminate(ctx); err != nil {
		utils.ErrExit("Failed to terminate container: %v", err)
	}
}

func startPostgreSQLContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	// TODO: verify the docker images being used are the correct certified ones
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("postgres:%s", source.DBVersion),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     source.User,
			"POSTGRES_PASSWORD": source.Password,
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(1 * time.Minute),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "./test_schemas/postgresql_schema.sql",
				ContainerFilePath: "docker-entrypoint-initdb.d/postgresql_schema.sql",
				FileMode:          0755,
			},
		},
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startMysqlContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("mysql:%s", source.DBVersion),
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": source.Password,
			"MYSQL_USER":          source.User,
			"MYSQL_PASSWORD":      source.Password,
			"MYSQL_DATABASE":      source.DBName,
		},
		WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(1 * time.Minute).WithPollInterval(1 * time.Second),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "./test_schemas/mysql_schema.sql",
				ContainerFilePath: "docker-entrypoint-initdb.d/mysql_schema.sql",
				FileMode:          0755,
			},
		},
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startOracleContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	// refer: https://hub.docker.com/r/gvenzl/oracle-xe
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("gvenzl/oracle-xe:%s", source.DBVersion),
		ExposedPorts: []string{"1521/tcp"},
		Env: map[string]string{
			"ORACLE_PASSWORD":   source.Password, // for SYS user
			"ORACLE_DATABASE":   source.DBName,
			"APP_USER":          source.User,
			"APP_USER_PASSWORD": source.Password,
		},
		WaitingFor: wait.ForLog("DATABASE IS READY TO USE").WithStartupTimeout(2 * time.Minute).WithPollInterval(5 * time.Second),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "./test_schemas/oracle_schema.sql",
				ContainerFilePath: "docker-entrypoint-initdb.d/oracle_schema.sql",
				FileMode:          0755,
			},
		},
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// this will create a 1 Node RF-1 cluster
func startYugabyteDBContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("yugabytedb/yugabyte:%s", source.DBVersion),
		ExposedPorts: []string{"5433/tcp", "15433/tcp", "7000/tcp", "9000/tcp", "9042/tcp"},
		Cmd: []string{
			"bin/yugabyted",
			"start",
			"--daemon=false",
			"--ui=false",
			"--initial_scripts_dir=/home/yugabyte/initial-scripts",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5433/tcp").WithStartupTimeout(2*time.Minute).WithPollInterval(1*time.Second),
			wait.ForLog("Data placement constraint successfully verified").WithStartupTimeout(3*time.Minute).WithPollInterval(1*time.Second),
		),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "./test_schemas/yugabytedb_schema.sql",
				ContainerFilePath: "/home/yugabyte/initial-scripts/yugabytedb_schema.sql",
				FileMode:          0755,
			},
		},
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// === assertion helper functions

func assertEqualStringSlices(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("Mismatch in slice length. Expected: %v, Actual: %v", expected, actual)
	}

	sort.Strings(expected)
	sort.Strings(actual)
	assert.DeepEqual(t, expected, actual)
}

func assertEqualSourceNameSlices(t *testing.T, expected, actual []*sqlname.SourceName) {
	sortSourceNames(expected)
	sortSourceNames(actual)
	assert.DeepEqual(t, expected, actual)
}

func sortSourceNames(tables []*sqlname.SourceName) {
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Qualified.MinQuoted < tables[j].Qualified.MinQuoted
	})
}
