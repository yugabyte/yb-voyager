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
	"path/filepath"
	"strconv"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TestDB struct {
	Container testcontainers.Container
	Source    *Source
}

var testDB *TestDB

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
		testDB.Container, err = startPostgresContainer(ctx, source)
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
		return nil, fmt.Errorf("failed to start '%s' test container: %w", source.DBType, err)
	}

	source.Host, source.Port = testDB.GetContainerHostPort(ctx)
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

func startPostgresContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	schemaFileName := fmt.Sprintf("%s_schema.sql", source.DBType)
	schemaFilePath := filepath.Join(".", "test_schemas", schemaFileName)
	mountPath := filepath.Join("docker-entrypoint-initdb.d", schemaFileName)

	// TODO: verify the docker images being used are the correct ones
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("postgres:%s", source.DBVersion),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     source.User,
			"POSTGRES_PASSWORD": source.Password,
			"POSTGRES_DB":       source.DBName,
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      schemaFilePath,
				ContainerFilePath: mountPath,
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
			"MYSQL_USER":     source.User,
			"MYSQL_PASSWORD": source.Password,
			"MYSQL_DATABASE": source.DBName,
		},
		WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(60 * time.Second),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startOracleContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	return nil, nil
}

func startYugabyteDBContainer(ctx context.Context, source *Source) (testcontainers.Container, error) {
	return nil, nil
}
