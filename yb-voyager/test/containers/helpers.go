package testcontainers

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/testcontainers/testcontainers-go"
)

const (
	DEFAULT_PG_PORT     = "5432"
	DEFAULT_YB_PORT     = "5433"
	DEFAULT_ORACLE_PORT = "1521"
	DEFAULT_MYSQL_PORT  = "3306"

	POSTGRESQL = "postgresql"
	YUGABYTEDB = "yugabytedb"
	ORACLE     = "oracle"
	MYSQL      = "mysql"
)

//go:embed test_schemas/postgresql_schema.sql
var postgresInitSchemaFile []byte

//go:embed test_schemas/oracle_schema.sql
var oracleInitSchemaFile []byte

//go:embed test_schemas/mysql_schema.sql
var mysqlInitSchemaFile []byte

//go:embed test_schemas/yugabytedb_schema.sql
var yugabytedbInitSchemaFile []byte

func printContainerLogs(container testcontainers.Container) {
	if container == nil {
		log.Printf("Cannot fetch logs: container is nil")
		return
	}

	containerID := container.GetContainerID()
	logs, err := container.Logs(context.Background())
	if err != nil {
		log.Printf("Error fetching logs for container %s: %v", containerID, err)
		return
	}
	defer logs.Close()

	// Read the logs
	logData, err := io.ReadAll(logs)
	if err != nil {
		log.Printf("Error reading logs for container %s: %v", containerID, err)
		return
	}

	fmt.Printf("=== Logs for container %s ===\n%s\n=== End of Logs for container %s ===\n", containerID, string(logData), containerID)
}

// pingDatabase tries to connect to the database using the driver and connection string.
// It retries for a few times with a delay before giving up.
func pingDatabase(driver string, connStr string) error {
	var err error
	maxRetries := 3
	retryDelay := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		db, openErr := sql.Open(driver, connStr)
		if openErr != nil {
			err = openErr
		} else {
			pingErr := db.Ping()
			closeErr := db.Close()
			if pingErr == nil && closeErr == nil {
				return nil // success
			}

			if pingErr != nil {
				err = pingErr
			} else {
				err = closeErr
			}
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("pingDatabase failed even after '%d' retries: %w", maxRetries, err)
}
