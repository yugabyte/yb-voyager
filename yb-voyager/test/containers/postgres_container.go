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

type PostgresContainer struct {
	ContainerConfig
	container testcontainers.Container
	db        *sql.DB
}

func (pg *PostgresContainer) Start(ctx context.Context) (err error) {
	if pg.container != nil {
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
		return err
	}

	dsn := pg.GetConnectionString()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		pg.container.Terminate(ctx)
		return fmt.Errorf("failed to ping postgres after connection: %w", err)
	}

	// Store the DB connection for reuse
	pg.db = db
	return nil
}

func (pg *PostgresContainer) Terminate(ctx context.Context) {
	if pg == nil {
		return
	}

	// Close the DB connection if it exists
	if pg.db != nil {
		if err := pg.db.Close(); err != nil {
			log.Errorf("failed to close postgres db connection: %v", err)
		}
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

	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", config.User, config.Password, host, port, config.DBName)
}

func (pg *PostgresContainer) ExecuteSqls(sqls ...string) {
	if pg.db == nil {
		utils.ErrExit("db connection not initialized for postgres container")
	}

	for _, sqlStmt := range sqls {
		_, err := pg.db.Exec(sqlStmt)
		if err != nil {
			utils.ErrExit("failed to execute sql '%s': %w", sqlStmt, err)
		}
	}
}
