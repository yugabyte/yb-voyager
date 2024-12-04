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

type OracleContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (ora *OracleContainer) Start(ctx context.Context) (err error) {
	// refer: https://hub.docker.com/r/gvenzl/oracle-xe
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("gvenzl/oracle-xe:%s", ora.DBVersion),
		ExposedPorts: []string{"1521/tcp"},
		Env: map[string]string{
			"ORACLE_PASSWORD":   ora.Password, // for SYS user
			"ORACLE_DATABASE":   ora.DBName,
			"APP_USER":          ora.User,
			"APP_USER_PASSWORD": ora.Password,
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

	ora.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	return err
}

func (ora *OracleContainer) Terminate(ctx context.Context) {
	err := ora.container.Terminate(ctx)
	if err != nil {
		log.Errorf("faile to terminate oracle container: %v", err)
	}
}

func (ora *OracleContainer) GetHostPort() (string, int, error) {
	ctx := context.Background()

	host, err := ora.container.Host(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch host for oracle container: %w", err)
	}

	port, err := ora.container.MappedPort(ctx, nat.Port(DEFAULT_ORACLE_PORT))
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch mapped port for oracle container: %w", err)
	}

	return host, port.Int(), nil
}
