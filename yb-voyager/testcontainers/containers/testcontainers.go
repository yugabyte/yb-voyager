package testcontainers

import (
	"context"
	"fmt"
)

type TestContainer interface {
	Start(ctx context.Context) error
	Terminate(ctx context.Context)
	GetHostPort() (string, int, error)

	/*
		TODOs
			// Function to run sql script for a specific test case
			SetupSqlScript(scriptName string, dbName string) error
	*/
}

type ContainerConfig struct {
	DBVersion string
	User      string
	Password  string
	DBName    string
	Schema    string
}

func NewTestContainer(dbType string, containerConfig *ContainerConfig) TestContainer {
	switch dbType {
	case POSTGRESQL:
		return &PostgresContainer{
			ContainerConfig: *containerConfig,
		}
	case YUGABYTEDB:
		return &YugabyteDBContainer{
			ContainerConfig: *containerConfig,
		}
	case ORACLE:
		return &OracleContainer{
			ContainerConfig: *containerConfig,
		}
	case MYSQL:
		return &MysqlContainer{
			ContainerConfig: *containerConfig,
		}
	default:
		panic(fmt.Sprintf("unsupported db type '%q' for creating test container\n", dbType))
	}
}
