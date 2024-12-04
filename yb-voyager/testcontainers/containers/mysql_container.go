package testcontainers

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type MysqlContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (ms *MysqlContainer) Start(ctx context.Context) (err error) {
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("mysql:%s", ms.DBVersion),
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": ms.Password,
			"MYSQL_USER":          ms.User,
			"MYSQL_PASSWORD":      ms.Password,
			"MYSQL_DATABASE":      ms.DBName,
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

	ms.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	return err
}

func (ms *MysqlContainer) Terminate(ctx context.Context) {
	err := ms.container.Terminate(ctx)
	if err != nil {
		log.Errorf("faile to terminate mysql container: %v", err)
	}
}

func (ms *MysqlContainer) GetHostPort() (string, int, error) {
	ctx := context.Background()

	host, err := ms.container.Host(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch host for mysql container: %w", err)
	}

	port, err := ms.container.MappedPort(ctx, nat.Port(DEFAULT_MYSQL_PORT))
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch mapped port for mysql container: %w", err)
	}

	return host, port.Int(), nil
}
