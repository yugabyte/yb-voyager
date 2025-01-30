package testcontainers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type YugabyteDBContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (yb *YugabyteDBContainer) Start(ctx context.Context) (err error) {
	if yb.container != nil {
		utils.PrintAndLog("YugabyteDB-%s container already running", yb.DBVersion)
		return nil
	}

	// since these Start() can be called from anywhere so need a way to ensure that correct files(without needing abs path) are picked from project directories
	tmpFile, err := os.CreateTemp(os.TempDir(), "yugabytedb_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp schema file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(yugabytedbInitSchemaFile); err != nil {
		return fmt.Errorf("failed to write to temp schema file: %w", err)
	}

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
			wait.ForListeningPort("5433/tcp").WithStartupTimeout(2*time.Minute).WithPollInterval(5*time.Second),
			wait.ForLog("Data placement constraint successfully verified").WithStartupTimeout(3*time.Minute).WithPollInterval(1*time.Second),
		),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      tmpFile.Name(),
				ContainerFilePath: "/home/yugabyte/initial-scripts/yugabytedb_schema.sql",
				FileMode:          0755,
			},
		},
	}

	yb.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	printContainerLogs(yb.container)
	return err
}

func (yb *YugabyteDBContainer) Terminate(ctx context.Context) {
	if yb == nil {
		return
	}

	err := yb.container.Terminate(ctx)
	if err != nil {
		log.Errorf("failed to terminate yugabytedb container: %v", err)
	}
}

func (yb *YugabyteDBContainer) GetHostPort() (string, int, error) {
	if yb.container == nil {
		return "", -1, fmt.Errorf("yugabytedb container is not started: nil")
	}

	ctx := context.Background()
	host, err := yb.container.Host(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch host for yugabytedb container: %w", err)
	}

	port, err := yb.container.MappedPort(ctx, nat.Port(DEFAULT_YB_PORT))
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch mapped port for yugabytedb container: %w", err)
	}

	return host, port.Int(), nil
}

func (yb *YugabyteDBContainer) GetConfig() ContainerConfig {
	return yb.ContainerConfig
}

func (yb *YugabyteDBContainer) GetConnectionString() string {
	config := yb.GetConfig()
	host, port, err := yb.GetHostPort()
	if err != nil {
		utils.ErrExit("failed to get host port for yugabytedb connection string: %v", err)
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", config.User, config.Password, host, port, config.DBName)
}

func (yb *YugabyteDBContainer) ExecuteSqls(sqls ...string) {
	connStr := yb.GetConnectionString()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		utils.ErrExit("failed to connect postgres for executing sqls: %w", err)
	}
	defer conn.Close(context.Background())

	retryCount := 3
	retryErrors := []string{
		"Restart read required at",
	}
	for _, sql := range sqls {
		var err error
		for i := 0; i < retryCount; i++ {
			_, err = conn.Exec(context.Background(), sql)
			if err == nil {
				break
			}
			if !lo.ContainsBy(retryErrors, func(r string) bool {
				return strings.Contains(err.Error(), r)
			}) {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			utils.ErrExit("failed to execute sql '%s': %w", sql, err)
		}
	}
}
