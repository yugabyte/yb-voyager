package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type PostgresContainer struct {
	mutex sync.Mutex
	ContainerConfig
	forLive   bool
	container testcontainers.Container
}

func (pg *PostgresContainer) Start(ctx context.Context) (err error) {
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	// if container is not nil, it means it was already started
	if pg.container != nil {
		// already running, do nothing.
		if pg.container.IsRunning() {
			utils.PrintAndLog("Postgres-%s container already running", pg.DBVersion)
			return nil
		}
		// but if it’s stopped, so start it back up in place
		utils.PrintAndLog("Restarting Postgres-%s container", pg.DBVersion)
		if err := pg.container.Start(ctx); err != nil {
			return fmt.Errorf("failed to restart postgres container: %w", err)
		}

		// Wait for it to accept connections again
		return pingDatabase("pgx", pg.GetConnectionString())
	}

	// since these Start() can be called from anywhere so need a way to ensure that correct files(without needing abs path) are picked from project directories
	tmpFile, err := os.CreateTemp(os.TempDir(), "postgresql_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp schema file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(postgresInitSchemaFile); err != nil {
		return fmt.Errorf("failed to write to temp schema file: %w", err)
	}

	req := testcontainers.ContainerRequest{
		// TODO: verify the docker images being used are the correct/certified ones
		Image:        fmt.Sprintf("postgres:%s", pg.DBVersion),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     pg.User,
			"POSTGRES_PASSWORD": pg.Password,
			"POSTGRES_DB":       pg.DBName, // NOTE: PG image makes the database with same name as user if not specific
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(2*time.Minute).WithPollInterval(5*time.Second),
			wait.ForLog("database system is ready to accept connections").WithStartupTimeout(3*time.Minute).WithPollInterval(5*time.Second),
		),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      tmpFile.Name(),
				ContainerFilePath: "docker-entrypoint-initdb.d/postgresql_schema.sql",
				FileMode:          0755,
			},
		},
	}

	if pg.forLive {
		req.Cmd = []string{
			"postgres",
			"-c", "wal_level=logical", // <-- set wal_level,
			"-c", "max_wal_senders=10", // optional for logical replication
		}
	}

	pg.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	printContainerLogs(pg.container)
	if err != nil {
		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	err = pingDatabase("pgx", pg.GetConnectionString())
	if err != nil {
		return fmt.Errorf("failed to ping postgres container: %w", err)
	}

	return nil
}

// Stop simulates a database outage by stopping (but not removing) the Docker container.
// The underlying data directory remains intact, so you can call Start() later
// and the DB will pick up with exactly the same contents.
func (pg *PostgresContainer) Stop(ctx context.Context) error {
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	if pg.container == nil {
		return nil
	} else if !pg.container.IsRunning() {
		utils.PrintAndLog("Postgres-%s container already stopped", pg.DBVersion)
		return nil
	}

	timeout := 10 * time.Second
	// Stop with a 10s timeout—this sends SIGTERM and waits, but does NOT remove the container.
	if err := pg.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop postgres container: %w", err)
	}
	return nil
}

func (pg *PostgresContainer) Terminate(ctx context.Context) {
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	if pg == nil {
		return
	}

	err := pg.container.Terminate(ctx)
	if err != nil {
		log.Errorf("failed to terminate postgres container: %v", err)
	}
}

func (pg *PostgresContainer) GetHostPort() (string, int, error) {
	if pg.container == nil {
		return "", -1, fmt.Errorf("postgres container is not started: nil")
	}

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

func (pg *PostgresContainer) GetConfig() ContainerConfig {
	return pg.ContainerConfig
}

func (pg *PostgresContainer) GetConnectionString() string {
	config := pg.GetConfig()
	host, port, err := pg.GetHostPort()
	if err != nil {
		utils.ErrExit("failed to get host port for postgres connection string: %v", err)
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", config.User, config.Password, host, port, config.DBName)
}

func (pg *PostgresContainer) GetConnection() (*sql.DB, error) {
	if pg == nil {
		utils.ErrExit("postgres container is not started: nil")
	}

	connStr := pg.GetConnectionString()

	conn, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to postgres: %w", err)
	}

	return conn, nil
}

func (pg *PostgresContainer) GetVersion() (string, error) {
	if pg == nil {
		return "", fmt.Errorf("postgres container is not started: nil")
	}

	conn, err := pg.GetConnection()
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

func (pg *PostgresContainer) ExecuteSqls(sqls ...string) {
	if pg == nil {
		utils.ErrExit("postgres container is not started: nil")
	}

	connStr := pg.GetConnectionString()
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		utils.ErrExit("failed to connect to postgres for executing sqls: %w", err)
	}
	defer conn.Close(context.Background())

	for _, sqlStmt := range sqls {
		_, err := conn.Exec(context.Background(), sqlStmt)
		if err != nil {
			utils.ErrExit("failed to execute sql '%s': %w", sqlStmt, err)
		}
	}
}

func (pg *PostgresContainer) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	if pg == nil {
		utils.ErrExit("postgres container is not started: nil")
	}

	conn, err := pg.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for postgres query: %w", err)
	}

	rows, err := conn.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query '%s': %w", sql, err)
	}

	return rows, nil
}
