package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type MysqlContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (ms *MysqlContainer) Start(ctx context.Context) (err error) {
	if ms.container != nil && ms.container.IsRunning() {
		utils.PrintAndLog("Mysql-%s container already running", ms.DBVersion)
		return nil
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

func (ms *MysqlContainer) Terminate(ctx context.Context) {
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
