package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type PostgresContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (pg *PostgresContainer) Start(ctx context.Context) (err error) {
	if pg.container != nil && pg.container.IsRunning() {
		utils.PrintAndLog("Postgres-%s container already running", pg.DBVersion)
		return nil
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

func (pg *PostgresContainer) Terminate(ctx context.Context) {
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
