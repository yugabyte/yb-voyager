package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
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
	mutex sync.Mutex
	ContainerConfig
	container testcontainers.Container
}

func (yb *YugabyteDBContainer) Start(ctx context.Context) (err error) {
	yb.mutex.Lock()
	defer yb.mutex.Unlock()

	if yb.container != nil {
		if yb.container.IsRunning() {
			utils.PrintAndLog("YugabyteDB-%s container already running", yb.DBVersion)
			return nil
		}

		// but if it’s stopped, so start it back up in place
		utils.PrintAndLog("Restarting YugabyteDB-%s container", yb.DBVersion)
		if err := yb.container.Start(ctx); err != nil {
			return fmt.Errorf("failed to restart yugabytedb container: %w", err)
		}

		if err := yugabyteWait().WaitUntilReady(ctx, yb.container); err != nil {
			return err
		}
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
	// TODO: Ideally we should test with 3 Node RF-3 cluster
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
		WaitingFor: yugabyteWait(),
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
	if err != nil {
		return fmt.Errorf("failed to start yugabytedb container: %w", err)
	}

	return nil
}

// Stop simulates a database outage by stopping (but not removing) the Docker container.
// The underlying data directory remains intact, so you can call Start() later
// and the DB will pick up with exactly the same contents.
func (yb *YugabyteDBContainer) Stop(ctx context.Context) error {
	yb.mutex.Lock()
	defer yb.mutex.Unlock()

	if yb.container == nil {
		return nil
	} else if !yb.container.IsRunning() {
		utils.PrintAndLog("YugabyteDB-%s container already stopped", yb.DBVersion)
		return nil
	}

	timeout := 10 * time.Second
	// Stop with a 10s timeout—this sends SIGTERM and waits, but does NOT remove the container.
	if err := yb.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop postgres container: %w", err)
	}

	utils.PrintAndLog("🛑 YugabyteDB-%s container stopped", yb.DBVersion)
	return nil
}

func (yb *YugabyteDBContainer) Terminate(ctx context.Context) {
	yb.mutex.Lock()
	defer yb.mutex.Unlock()

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

func (yb *YugabyteDBContainer) GetConnection() (*sql.DB, error) {
	if yb.container == nil {
		utils.ErrExit("yugabytedb container is not started: nil")
	}

	connStr := yb.GetConnectionString()
	conn, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to yugabytedb: %w", err)
	}
	return conn, nil
}

func (yb *YugabyteDBContainer) GetVersion() (string, error) {
	if yb == nil {
		return "", fmt.Errorf("postgres container is not started: nil")
	}

	conn, err := yb.GetConnection()
	if err != nil {
		return "", fmt.Errorf("failed to get connection for postgres version: %w", err)
	}
	defer conn.Close()

	var version string
	err = conn.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to query postgres version: %w", err)
	}

	return version, nil
}

func (yb *YugabyteDBContainer) ExecuteSqls(sqls ...string) {
	if yb == nil {
		utils.ErrExit("yugabytedb container is not started: nil")
	}

	connStr := yb.GetConnectionString()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		utils.ErrExit("failed to connect to yugabytedb for executing sqls: %w", err)
	}
	defer conn.Close(context.Background())

	retryCount := 3
	retryErrors := []string{
		"Restart read required",
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

func (yb *YugabyteDBContainer) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	if yb == nil {
		utils.ErrExit("yugabytedb container is not started: nil")
	}

	conn, err := yb.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for yugabytedb query: %w", err)
	}
	defer conn.Close()
	rows, err := conn.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query '%s': %w", sql, err)
	}

	return rows, nil
}

// yugabyteWait returns a wait strategy for YugabyteDB node(s)
func yugabyteWait() wait.Strategy {
	return wait.ForSQL(nat.Port("5433/tcp"), "pgx",
		func(host string, port nat.Port) string {
			return fmt.Sprintf(
				"postgres://yugabyte:password@%s:%s/yugabyte?sslmode=disable",
				host, port.Port())
		},
	).WithQuery("SELECT 1").
		WithStartupTimeout(3 * time.Minute)
}
