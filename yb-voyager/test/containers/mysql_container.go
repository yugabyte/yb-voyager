package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MysqlContainer struct {
	mutex sync.Mutex
	ContainerConfig
	container testcontainers.Container
}

func (ms *MysqlContainer) SetConfig(config ContainerConfig) {
	ms.ContainerConfig = config
}

func (ms *MysqlContainer) Start(ctx context.Context) (err error) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if ms.container != nil {
		// already running, do nothing.
		if ms.container.IsRunning() {
			utils.PrintAndLogf("MySQL-%s container already running", ms.DBVersion)
			return nil
		}
		// but if it’s stopped, so start it back up in place
		utils.PrintAndLogf("Restarting MySQL-%s container", ms.DBVersion)
		if err := ms.container.Start(ctx); err != nil {
			return fmt.Errorf("failed to restart mysql container: %w", err)
		}

		// Wait for it to accept connections again
		return pingDatabase("mysql", ms.GetConnectionString())
	}

	// since these Start() can be called from anywhere so need a way to ensure that correct files(without needing abs path) are picked from project directories
	tmpFile, err := os.CreateTemp(os.TempDir(), "mysql_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp schema file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(mysqlInitSchemaFile); err != nil {
		return fmt.Errorf("failed to write to temp schema file: %w", err)
	}

	req := testcontainers.ContainerRequest{
		// TODO: verify the docker images being used are the correct/certified ones
		Image:        fmt.Sprintf("mysql:%s", ms.DBVersion),
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": ms.Password,
			"MYSQL_USER":          ms.User,
			"MYSQL_PASSWORD":      ms.Password,
			"MYSQL_DATABASE":      ms.DBName,
		},
		WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(2 * time.Minute).WithPollInterval(5 * time.Second),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      tmpFile.Name(),
				ContainerFilePath: "docker-entrypoint-initdb.d/mysql_schema.sql",
				FileMode:          0755,
			},
		},
	}

	ms.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	printContainerLogs(ms.container)

	if err != nil {
		return fmt.Errorf("failed to start mysql container: %w", err)
	}

	err = pingDatabase("mysql", ms.GetConnectionString())
	if err != nil {
		return fmt.Errorf("failed to ping mysql container: %w", err)
	}
	return nil
}

// Stop simulates a database outage by stopping (but not removing) the Docker container.
// The underlying data directory remains intact, so you can call Start() later
// and the DB will pick up with exactly the same contents.
func (ms *MysqlContainer) Stop(ctx context.Context) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if ms.container == nil {
		return nil
	} else if !ms.container.IsRunning() {
		utils.PrintAndLogf("MySQL-%s container already stopped", ms.DBVersion)
		return nil
	}

	timeout := 10 * time.Second
	// Stop with a 10s timeout—this sends SIGTERM and waits, but does NOT remove the container.
	if err := ms.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop postgres container: %w", err)
	}
	return nil
}

func (ms *MysqlContainer) Terminate(ctx context.Context) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if ms == nil {
		return
	}

	err := ms.container.Terminate(ctx)
	if err != nil {
		log.Errorf("failed to terminate mysql container: %v", err)
	}
}

func (ms *MysqlContainer) GetHostPort() (string, int, error) {
	if ms.container == nil {
		return "", -1, fmt.Errorf("mysql container is not started: nil")
	}

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

func (ms *MysqlContainer) GetConfig() ContainerConfig {
	return ms.ContainerConfig
}

func (ms *MysqlContainer) GetConnectionString() string {
	host, port, err := ms.GetHostPort()
	if err != nil {
		utils.ErrExit("failed to get host port for mysql connection string: %v", err)
	}

	// DSN format: user:password@tcp(host:port)/dbname
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		ms.User, ms.Password, host, port, ms.DBName)
}

func (ms *MysqlContainer) GetConnection() (*sql.DB, error) {
	if ms.container == nil {
		utils.ErrExit("mysql container is not started: nil")
	}

	db, err := sql.Open("mysql", ms.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}

	return db, nil
}

func (ms *MysqlContainer) GetVersion() (string, error) {
	if ms == nil {
		return "", fmt.Errorf("mysql container is not started: nil")
	}

	db, err := ms.GetConnection()
	if err != nil {
		return "", fmt.Errorf("failed to get connection for mysql version: %w", err)
	}
	defer db.Close()

	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to fetch mysql version: %w", err)
	}

	return version, nil
}

func (ms *MysqlContainer) ExecuteSqls(sqls ...string) {
	if ms == nil {
		utils.ErrExit("mysql container is not started: nil")
	}

	db, err := sql.Open("mysql", ms.GetConnectionString())
	if err != nil {
		utils.ErrExit("failed to connect to mysql for executing sqls: %w", err)
	}

	for _, sqlStmt := range sqls {
		_, err := db.Exec(sqlStmt)
		if err != nil {
			utils.ErrExit("failed to execute sql '%s': %w", sqlStmt, err)
		}
	}
}

func (ms *MysqlContainer) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	db, err := ms.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for query: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return rows, nil
}
