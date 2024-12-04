package testcontainers

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (pg *PostgresContainer) Start(ctx context.Context) (err error) {
	// TODO: verify the docker images being used are the correct certified ones
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("postgres:%s", pg.DBVersion),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     pg.User,
			"POSTGRES_PASSWORD": pg.Password,
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

	pg.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		reader, err := pg.container.Logs(ctx)
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
	}

	return err
}

func (pg *PostgresContainer) Terminate(ctx context.Context) {
	err := pg.container.Terminate(ctx)
	if err != nil {
		log.Errorf("faile to terminate postgres container: %v", err)
	}
}

func (pg *PostgresContainer) GetHostPort() (string, int, error) {
	ctx := context.Background()

	host, err := pg.container.Host(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch host for postgres container: %w", err)
	}

	port, err := pg.container.MappedPort(ctx, nat.Port(DEFAULT_PG_PORT))
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch mapped port for postgres container: %w", err)
	}

	return host, port.Int(), nil
}
