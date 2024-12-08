package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	POSTGRESQL = "postgresql"
	YUGABYTEDB = "yugabytedb"
)

func StartDBContainer(ctx context.Context, dbType string) (testcontainers.Container, string, nat.Port, error) {
	switch dbType {
	case POSTGRESQL:
		return startPostgresContainer(ctx)
	case YUGABYTEDB:
		return startYugabyteDBContainer(ctx)
	default:
		return nil, "", "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func startPostgresContainer(ctx context.Context) (container testcontainers.Container, host string, port nat.Port, err error) {
	// Create a PostgreSQL TestContainer
	req := testcontainers.ContainerRequest{
		Image:        "postgres:latest", // Use the latest PostgreSQL image
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",     // Set PostgreSQL username
			"POSTGRES_PASSWORD": "testpassword", // Set PostgreSQL password
			"POSTGRES_DB":       "testdb",       // Set PostgreSQL database name
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * 1e9), // Wait for PostgreSQL to be ready
	}

	// Start the container
	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, "", "", err
	}

	// Get the container's host and port
	host, err = pgContainer.Host(ctx)
	if err != nil {
		return nil, "", "", err
	}

	port, err = pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, "", "", err
	}

	return pgContainer, host, port, nil
}

func startYugabyteDBContainer(ctx context.Context) (container testcontainers.Container, host string, port nat.Port, err error) {
	// Create a YugabyteDB TestContainer
	req := testcontainers.ContainerRequest{
		Image:        "yugabytedb/yugabyte:latest",
		ExposedPorts: []string{"5433/tcp"},
		WaitingFor:   wait.ForListeningPort("5433/tcp"),
		Cmd:          []string{"bin/yugabyted", "start", "--daemon=false"},
	}

	// Start the container
	ybContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", "", err
	}

	// Get the container's host and port
	host, err = ybContainer.Host(ctx)
	if err != nil {
		return nil, "", "", err
	}

	port, err = ybContainer.MappedPort(ctx, "5433")
	if err != nil {
		return nil, "", "", err
	}

	return ybContainer, host, port, nil
}

// waitForDBConnection waits until the database is ready for connections.
func WaitForDBToBeReady(db *sql.DB) error {
	for i := 0; i < 12; i++ {
		if err := db.Ping(); err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("database did not become ready in time")
}
