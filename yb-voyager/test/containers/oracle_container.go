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

type OracleContainer struct {
	mutex sync.Mutex
	ContainerConfig
	container testcontainers.Container
}

func (ora *OracleContainer) Start(ctx context.Context) (err error) {
	ora.mutex.Lock()
	defer ora.mutex.Unlock()

	if ora.container != nil {
		if ora.container.IsRunning() {
			utils.PrintAndLog("Oracle-%s container already running", ora.DBVersion)
			return nil
		}

		// but if it’s stopped, so start it back up in place
		utils.PrintAndLog("Restarting Oracle-%s container", ora.DBVersion)
		if err := ora.container.Start(ctx); err != nil {
			return fmt.Errorf("failed to restart oracle container: %w", err)
		}
		// Wait for it to accept connections again
		return pingDatabase("godror", ora.GetConnectionString())
	}

	// since these Start() can be called from anywhere so need a way to ensure that correct files(without needing abs path) are picked from project directories
	tmpFile, err := os.CreateTemp(os.TempDir(), "oracle_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp schema file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(oracleInitSchemaFile); err != nil {
		return fmt.Errorf("failed to write to temp schema file: %w", err)
	}

	// refer: https://hub.docker.com/r/gvenzl/oracle-xe
	req := testcontainers.ContainerRequest{
		// TODO: verify the docker images being used are the correct/certified ones (No license issue)
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
				HostFilePath:      tmpFile.Name(),
				ContainerFilePath: "docker-entrypoint-initdb.d/oracle_schema.sql",
				FileMode:          0755,
			},
		},
	}

	ora.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	printContainerLogs(ora.container)
	if err != nil {
		return fmt.Errorf("failed to start oracle container: %w", err)
	}

	err = pingDatabase("godror", ora.GetConnectionString())
	if err != nil {
		return fmt.Errorf("failed to ping oracle container: %w", err)
	}
	return nil
}

// Stop simulates a database outage by stopping (but not removing) the Docker container.
// The underlying data directory remains intact, so you can call Start() later
// and the DB will pick up with exactly the same contents.
func (ora *OracleContainer) Stop(ctx context.Context) error {
	ora.mutex.Lock()
	defer ora.mutex.Unlock()

	if ora.container == nil {
		return nil
	} else if !ora.container.IsRunning() {
		utils.PrintAndLog("Oracle-%s container already stopped", ora.DBVersion)
		return nil
	}

	timeout := 10 * time.Second
	// Stop with a 10s timeout—this sends SIGTERM and waits, but does NOT remove the container.
	if err := ora.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop postgres container: %w", err)
	}
	return nil
}

func (ora *OracleContainer) Terminate(ctx context.Context) {
	ora.mutex.Lock()
	defer ora.mutex.Unlock()

	if ora == nil {
		return
	}

	err := ora.container.Terminate(ctx)
	if err != nil {
		log.Errorf("failed to terminate oracle container: %v", err)
	}
}

func (ora *OracleContainer) GetHostPort() (string, int, error) {
	if ora.container == nil {
		return "", -1, fmt.Errorf("oracle container is not started: nil")
	}

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

func (ora *OracleContainer) GetConfig() ContainerConfig {
	return ora.ContainerConfig
}

func (ora *OracleContainer) GetConnectionString() string {
	config := ora.GetConfig()
	host, port, err := ora.GetHostPort()
	if err != nil {
		utils.ErrExit("failed to get host port for oracle connection string: %v", err)
	}

	connectString := fmt.Sprintf(`(DESCRIPTION = (ADDRESS = (PROTOCOL = TCP)(HOST = %s)(PORT = %d))(CONNECT_DATA = (SERVICE_NAME = %s)))`,
		host, port, config.DBName)
	return fmt.Sprintf(`user="%s" password="%s" connectString="%s"`, config.User, config.Password, connectString)
}

func (ora *OracleContainer) GetConnection() (*sql.DB, error) {
	if ora.container == nil {
		utils.ErrExit("oracle container is not started: nil")
	}

	conn, err := sql.Open("godror", ora.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to oracle container: %w", err)
	}
	return conn, nil
}

func (ora *OracleContainer) GetVersion() (string, error) {
	if ora == nil {
		return "", fmt.Errorf("oracle container is not started: nil")
	}

	conn, err := ora.GetConnection()
	if err != nil {
		return "", fmt.Errorf("failed to get connection to oracle container: %w", err)
	}
	defer conn.Close()

	var version string
	err = conn.QueryRow("SELECT BANNER FROM V$VERSION").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get oracle version: %w", err)
	}
	return version, nil
}

func (ora *OracleContainer) ExecuteSqls(sqls ...string) {
	if ora == nil {
		utils.ErrExit("oracle container is not started: nil")
	}

	conn, err := ora.GetConnection()
	if err != nil {
		utils.ErrExit("failed to connect to oracle for executing sqls: %w", err)
	}
	defer conn.Close()

	for _, sqlStmt := range sqls {
		_, err := conn.Exec(sqlStmt)
		if err != nil {
			utils.ErrExit("failed to execute sql '%s': %w", sqlStmt, err)
		}
	}
}

func (ora *OracleContainer) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	conn, err := ora.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for query: %w", err)
	}
	defer conn.Close()

	rows, err := conn.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return rows, nil
}
