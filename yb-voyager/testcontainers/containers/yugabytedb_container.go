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

type YugabyteDBContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (yb *YugabyteDBContainer) Start(ctx context.Context) (err error) {
	// this will create a 1 Node RF-1 cluster
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("yugabytedb/yugabyte:%s", yb.DBVersion),
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

	yb.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	return err
}

func (yb *YugabyteDBContainer) Terminate(ctx context.Context) {
	err := yb.container.Terminate(ctx)
	if err != nil {
		log.Errorf("faile to terminate postgres container: %v", err)
	}
}

func (yb *YugabyteDBContainer) GetHostPort() (string, int, error) {
	ctx := context.Background()

	host, err := yb.container.Host(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch host for postgres container: %w", err)
	}

	port, err := yb.container.MappedPort(ctx, nat.Port(DEFAULT_YB_PORT))
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch mapped port for postgres container: %w", err)
	}

	return host, port.Int(), nil
}
