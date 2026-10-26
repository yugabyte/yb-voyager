package testcontainers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type PostgresContainer struct {
	mutex sync.Mutex
	ContainerConfig
	container testcontainers.Container
}

func (pg *PostgresContainer) SetConfig(config ContainerConfig) {
	pg.ContainerConfig = config
}

func (pg *PostgresContainer) Start(ctx context.Context) (err error) {
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	// if container is not nil, it means it was already started
	if pg.container != nil {
		// already running, do nothing.
		if pg.container.IsRunning() {
			utils.PrintAndLogf("Postgres-%s container already running", pg.DBVersion)
			return nil
		}
		// but if it’s stopped, so start it back up in place
		utils.PrintAndLogf("Restarting Postgres-%s container", pg.DBVersion)
		if err := pg.container.Start(ctx); err != nil {
			return fmt.Errorf("failed to restart postgres container: %w", err)
		}

		// Wait for it to accept connections again
		return pingDatabase("pgx", pg.GetConnectionString())
	}

	if pg.MTLS != nil {
		return pg.startWithMTLS(ctx)
	}

	// since these Start() can be called from anywhere so need a way to ensure that correct files(without needing abs path) are picked from project directories
	tmpFile, err := os.CreateTemp(os.TempDir(), "postgresql_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp schema file: %w", err)
	}
	defer utils.CloseAndLogOnError(tmpFile.Name(), tmpFile)

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

	if pg.ContainerConfig.ForLive {
		req.Cmd = []string{
			"postgres",
			"-c", "wal_level=logical", // <-- set wal_level for logical replication
			"-c", "max_replication_slots=20", // <-- increase max replication slots for live migration tests
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
		utils.PrintAndLogf("Postgres-%s container already stopped", pg.DBVersion)
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
	if pg == nil {
		return
	}

	pg.mutex.Lock()
	defer pg.mutex.Unlock()

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
		utils.ErrExit("failed to get host port for postgres connection string: %w", err)
	}

	return withConnQuery(fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", config.User, config.Password, host, port, config.DBName),
		config.harnessConnQuery("sslmode=disable"))
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

func (pg *PostgresContainer) getConnWithDefaultDB() (*pgx.Conn, error) {
	host, port, err := pg.GetHostPort()
	if err != nil {
		return nil, fmt.Errorf("failed to get host port for postgres connection string: %w", err)
	}
	connStr := withConnQuery(fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", pg.User, pg.Password, host, port, "postgres"),
		pg.harnessConnQuery("sslmode=disable"))
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	return conn, nil
}
func (pg *PostgresContainer) CreateDatabase(dbName string) error {
	conn, err := pg.getConnWithDefaultDB()
	if err != nil {
		return fmt.Errorf("failed to get connection with default database: %w", err)
	}
	defer conn.Close(context.Background())

	//check if database already exists
	existsQuery := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = '%s')", dbName)
	var exists bool
	err = conn.QueryRow(context.Background(), existsQuery).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}
	if exists {
		return nil
	}

	_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		return fmt.Errorf("failed to create database '%s': %w", dbName, err)
	}
	return nil
}

func (pg *PostgresContainer) DropDatabase(dbName string) error {
	conn, err := pg.getConnWithDefaultDB()
	if err != nil {
		return fmt.Errorf("failed to get connection with default database: %w", err)
	}
	defer conn.Close(context.Background())

	// First, terminate all active connections to the database
	terminateQuery := `
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid();
	`

	_, err = conn.Exec(context.Background(), terminateQuery, dbName)
	if err != nil {
		return fmt.Errorf("failed to terminate some connections to database '%s': %w", dbName, err)
	}

	for i := 0; i < 5; i++ {
		_, err = conn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE %s", dbName))
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to drop database '%s': %w", dbName, err)
	}
	return nil
}

func (pg *PostgresContainer) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	if pg == nil {
		utils.ErrExit("postgres container is not started: nil")
	}

	conn, err := pg.GetConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for postgres query: %w", err)
	}
	defer conn.Close()
	rows, err := conn.Query(sql, args...) //nolint:sqlclosecheck // rows are returned to and closed by the caller
	if err != nil {
		return nil, fmt.Errorf("failed to execute query '%s': %w", sql, err)
	}

	return rows, nil
}

func (pg *PostgresContainer) GetConnectionWithDB(dbName string) (*sql.DB, error) {
	config := pg.GetConfig()
	host, port, err := pg.GetHostPort()
	if err != nil {
		return nil, fmt.Errorf("failed to get host port for postgres connection string: %w", err)
	}
	connStr := withConnQuery(fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", config.User, config.Password, host, port, dbName),
		config.harnessConnQuery(""))
	return sql.Open("pgx", connStr)
}

func (pg *PostgresContainer) QueryOnDB(dbName string, sql string, args ...interface{}) (*sql.Rows, error) {
	if pg == nil {
		utils.ErrExit("postgres container is not started: nil")
	}
	conn, err := pg.GetConnectionWithDB(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for postgres query on db '%s': %w", dbName, err)
	}
	defer conn.Close()
	rows, err := conn.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query '%s': %w", sql, err)
	}
	return rows, nil
}

func (pg *PostgresContainer) ExecuteSqlsOnDB(dbName string, sqls ...string) {
	if pg == nil {
		utils.ErrExit("postgres container is not started: nil")
	}
	conn, err := pg.GetConnectionWithDB(dbName)
	if err != nil {
		utils.ErrExit("failed to get connection for postgres executing sqls on db: %w", err)
	}
	defer conn.Close()
	for _, sql := range sqls {
		_, err := conn.Exec(sql)
		if err != nil {
			utils.ErrExit("failed to execute sql '%s': %w", sql, err)
		}
	}
}

const (
	pgMTLSStagingDir = "/tmp/mtls"
	pgMTLSServingDir = "/var/lib/postgresql/mtls"
)

// The `local` line is the only one that does not require TLS: docker-entrypoint initializes the
// database over the Unix socket, which is not reachable from outside the container.
const pgMTLSHBA = `local    all          all       trust
hostssl  all          all  all  cert
hostssl  replication  all  all  cert
`

// startWithMTLS starts a server whose pg_hba.conf admits network clients only over TLS with
// `cert` authentication, i.e. a client certificate signed by the CA whose CN is the role.
func (pg *PostgresContainer) startWithMTLS(ctx context.Context) (err error) {
	// Postgres refuses a key file that is not owned by its OS user, and copied files are
	// root-owned, so the entrypoint installs them with the right owner before starting postgres.
	entrypointScript := fmt.Sprintf(
		"install -d -o postgres -g postgres -m 700 %[2]s && "+
			"install -o postgres -g postgres -m 644 %[1]s/ca.crt %[1]s/server.crt %[1]s/pg_hba.conf %[2]s/ && "+
			"install -o postgres -g postgres -m 600 %[1]s/server.key %[2]s/ && "+
			`exec docker-entrypoint.sh "$@"`,
		pgMTLSStagingDir, pgMTLSServingDir)

	cmd := []string{
		"postgres",
		"-c", "ssl=on",
		"-c", "ssl_ca_file=" + pgMTLSServingDir + "/ca.crt",
		"-c", "ssl_cert_file=" + pgMTLSServingDir + "/server.crt",
		"-c", "ssl_key_file=" + pgMTLSServingDir + "/server.key",
		"-c", "hba_file=" + pgMTLSServingDir + "/pg_hba.conf",
	}
	if pg.ForLive {
		cmd = append(cmd, "-c", "wal_level=logical", "-c", "max_replication_slots=20")
	}

	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("postgres:%s", pg.DBVersion),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     pg.User,
			"POSTGRES_PASSWORD": pg.Password,
			"POSTGRES_DB":       pg.DBName,
		},
		Entrypoint: []string{"sh", "-c", entrypointScript, "sh"},
		Cmd:        cmd,
		Files: []testcontainers.ContainerFile{
			{Reader: bytes.NewReader(pg.MTLS.CACertPEM), ContainerFilePath: pgMTLSStagingDir + "/ca.crt", FileMode: 0o644},
			{Reader: bytes.NewReader(pg.MTLS.ServerCertPEM), ContainerFilePath: pgMTLSStagingDir + "/server.crt", FileMode: 0o644},
			{Reader: bytes.NewReader(pg.MTLS.ServerKeyPEM), ContainerFilePath: pgMTLSStagingDir + "/server.key", FileMode: 0o600},
			{Reader: strings.NewReader(pgMTLSHBA), ContainerFilePath: pgMTLSStagingDir + "/pg_hba.conf", FileMode: 0o644},
		},
		// The first occurrence is the entrypoint's temporary, socket-only initdb server.
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(3 * time.Minute),
	}

	pg.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		printContainerLogs(pg.container)
		return fmt.Errorf("failed to start mTLS postgres container: %w", err)
	}
	err = pingDatabase("pgx", pg.GetConnectionString())
	if err != nil {
		return fmt.Errorf("failed to ping mTLS postgres container: %w", err)
	}
	return nil
}

const (
	pgTLSStagingDir = "/tmp/tls"
	// Outside PGDATA so the files are not part of the data directory.
	pgTLSServingDir         = "/var/lib/postgresql"
	pgTLSCertSwapTimeout    = 30 * time.Second
	pgTLSCertSwapPollPeriod = 200 * time.Millisecond
)

// EnableTLS turns on TLS with the given PEM certificate and key, and returns once the server
// presents that certificate. Calling it again swaps the certificate. Undo with DisableTLS.
func (pg *PostgresContainer) EnableTLS(ctx context.Context, certPEM []byte, keyPEM []byte) error {
	if pg == nil || pg.container == nil {
		return fmt.Errorf("postgres container is not started")
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("decode server certificate PEM")
	}
	wantCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse server certificate: %w", err)
	}

	err = pg.container.CopyToContainer(ctx, certPEM, pgTLSStagingDir+"/server.crt", 0o644)
	if err != nil {
		return fmt.Errorf("copy server certificate into postgres container: %w", err)
	}
	err = pg.container.CopyToContainer(ctx, keyPEM, pgTLSStagingDir+"/server.key", 0o600)
	if err != nil {
		return fmt.Errorf("copy server key into postgres container: %w", err)
	}
	// Postgres refuses a key file that is not owned by its OS user or is group/world readable,
	// and copied files are root-owned.
	installCmd := fmt.Sprintf(
		"install -o postgres -g postgres -m 644 %[1]s/server.crt %[2]s/server.crt && "+
			"install -o postgres -g postgres -m 600 %[1]s/server.key %[2]s/server.key",
		pgTLSStagingDir, pgTLSServingDir)
	exitCode, output, err := pg.container.Exec(ctx, []string{"sh", "-c", installCmd}, tcexec.Multiplexed())
	if err != nil {
		return fmt.Errorf("install server certificate in postgres container: %w", err)
	}
	if exitCode != 0 {
		out, _ := io.ReadAll(output)
		return fmt.Errorf("install server certificate in postgres container: exit code %d: %s", exitCode, out)
	}

	err = pg.executeSqlsWithDefaultDB(ctx,
		fmt.Sprintf("ALTER SYSTEM SET ssl_cert_file = '%s/server.crt'", pgTLSServingDir),
		fmt.Sprintf("ALTER SYSTEM SET ssl_key_file = '%s/server.key'", pgTLSServingDir),
		"ALTER SYSTEM SET ssl = on",
		"SELECT pg_reload_conf()",
	)
	if err != nil {
		return fmt.Errorf("enable ssl in postgres container: %w", err)
	}
	return pg.waitForServedCertificate(ctx, wantCert)
}

// DisableTLS reverts EnableTLS, returning the server to the image's default (ssl=off).
func (pg *PostgresContainer) DisableTLS(ctx context.Context) error {
	if pg == nil || pg.container == nil {
		return fmt.Errorf("postgres container is not started")
	}
	err := pg.executeSqlsWithDefaultDB(ctx,
		"ALTER SYSTEM RESET ssl",
		"ALTER SYSTEM RESET ssl_cert_file",
		"ALTER SYSTEM RESET ssl_key_file",
		"SELECT pg_reload_conf()",
	)
	if err != nil {
		return fmt.Errorf("disable ssl in postgres container: %w", err)
	}
	return nil
}

func (pg *PostgresContainer) executeSqlsWithDefaultDB(ctx context.Context, sqls ...string) error {
	conn, err := pg.getConnWithDefaultDB()
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	for _, sqlStmt := range sqls {
		_, err := conn.Exec(ctx, sqlStmt)
		if err != nil {
			return fmt.Errorf("execute sql %q: %w", sqlStmt, err)
		}
	}
	return nil
}

// pg_reload_conf() only signals the postmaster, so a connection opened right after it can
// still be served the previous certificate.
func (pg *PostgresContainer) waitForServedCertificate(ctx context.Context, wantCert *x509.Certificate) error {
	host, port, err := pg.GetHostPort()
	if err != nil {
		return err
	}
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/postgres?sslmode=require", pg.User, pg.Password, host, port)

	deadline := time.Now().Add(pgTLSCertSwapTimeout)
	for {
		servedCert, err := fetchServedCertificate(ctx, connStr)
		if err == nil && servedCert.Equal(wantCert) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres did not present certificate CN=%q within %s (last error: %v)",
				wantCert.Subject.CommonName, pgTLSCertSwapTimeout, err)
		}
		time.Sleep(pgTLSCertSwapPollPeriod)
	}
}

func fetchServedCertificate(ctx context.Context, connStr string) (*x509.Certificate, error) {
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connect with sslmode=require: %w", err)
	}
	defer conn.Close(ctx)
	tlsConn, ok := conn.PgConn().Conn().(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("connection is not using TLS")
	}
	peerCerts := tlsConn.ConnectionState().PeerCertificates
	if len(peerCerts) == 0 {
		return nil, fmt.Errorf("server presented no certificate")
	}
	return peerCerts[0], nil
}
