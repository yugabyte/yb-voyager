package libmig

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// TODO: Unify with tgtdb.TargetDB.

type TargetDB struct {
	connPool *ConnectionPool
}

func NewTargetDB(connPool *ConnectionPool) *TargetDB {
	return &TargetDB{connPool: connPool}
}

func (tdb *TargetDB) TruncateTable(ctx context.Context, tableID *TableID) error {
	cmd := fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", tableID.QualifiedName())
	log.Infof("Running SQL command: %s", cmd)
	return tdb.connPool.WithConn(func(conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, cmd)
		return err
	})
}

func (tdb *TargetDB) TableHasPK(ctx context.Context, tableID *TableID) (bool, error) {
	var tableName string
	if utils.IsQuotedString(tableID.TableName) {
		tableName = strings.Trim(tableID.TableName, `"`)
	} else {
		tableName = strings.ToLower(tableID.TableName)
	}
	query := fmt.Sprintf(`SELECT * FROM information_schema.table_constraints
	WHERE constraint_type = 'PRIMARY KEY' AND table_name = '%s' AND table_schema = '%s';`, tableName, tableID.SchemaName)

	var rows pgx.Rows
	var err error
	err = tdb.connPool.WithConn(func(conn *pgx.Conn) error {
		log.Infof("Running query on target DB: %s", query)
		rows, err = conn.Query(ctx, query)
		return err
	})
	if err != nil {
		return false, fmt.Errorf("query target db to check if PK exists: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		log.Infof("Table %q has a PK", tableID)
		return true, nil
	} else {
		log.Infof("Table %q does not have a PK", tableID)
		return false, nil
	}
}

func (tdb *TargetDB) Copy(ctx context.Context, copyCommand string, batch *Batch) (int64, error) {
	var rowsAffected int64
	var err error
	var retry bool

	sleepIntervalSec := 0
	for attempt := 1; attempt <= 10; attempt++ {
		rowsAffected, retry, err = tdb.copy(ctx, copyCommand, batch)
		if err == nil { // SUCCESS.
			log.Infof("%q => rowsAffected: %v", copyCommand, rowsAffected)
			return rowsAffected, nil
		}
		if !retry {
			break
		}
		sleepIntervalSec += 10
		if sleepIntervalSec > 60 {
			sleepIntervalSec = 60
		}
		log.Warnf("attempt %d to %q failed (retry after %d sec): %s",
			attempt, copyCommand, sleepIntervalSec, err)
		time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
	}
	return rowsAffected, fmt.Errorf("copy batch %d: %w", batch.BatchNumber, err)
}

func (tdb *TargetDB) copy(ctx context.Context, copyCommand string, batch *Batch) (int64, bool, error) {
	r, err := batch.Reader()
	if err != nil {
		return 0, false, fmt.Errorf("create reader for batch %d: %w", batch.BatchNumber, err)
	}
	defer r.Close()

	var rowsAffected int64
	err = tdb.connPool.WithConn(func(c *pgx.Conn) error {
		res, err2 := c.PgConn().CopyFrom(context.Background(), r, copyCommand)
		rowsAffected = res.RowsAffected()
		return err2
	})
	if err == nil {
		// All is well.
		return rowsAffected, false, nil
	}
	if strings.Contains(err.Error(), "invalid input syntax") {
		// Retrying is not going to help when the batch data has some syntax error.
		return rowsAffected, false, fmt.Errorf("COPY: %w", err)
	}
	// See: https://github.com/yugabyte/yb-voyager/issues/240
	if strings.Contains(err.Error(), "Sending too long RPC message") {
		// This error doesn't go away even if we retry. Request the user to reduce batch size.
		utils.PrintAndLog("Target server reported error: %s", err)
		utils.PrintAndLog("\nRetry after reducing the --batch-size.\n")
		return rowsAffected, false, fmt.Errorf("COPY: %w", err)
	}
	if !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
		// COPY failed with unknown error. Request for retry.
		return rowsAffected, true, err
	}
	// "duplicate key value..." error implies:
	// - UPSERT mode is not being used.
	// - Transactional COPY is in effect.
	// - The batch was successfully imported earlier, but retried by yb-voyager.
	log.Infof("Received unique constraint violation error batch %d. Counting rows from batch data.", batch.BatchNumber)
	rowsAffected, err = countRowsInBatch(batch)
	if err != nil {
		err = fmt.Errorf("count rows from batch %d: %w", batch.BatchNumber, err)
	}
	return rowsAffected, false, err
}

func countRowsInBatch(batch *Batch) (int64, error) {
	var count int64

	r, err := batch.Reader()
	if err != nil {
		return 0, fmt.Errorf("create reader for batch %d: %w", batch.BatchNumber, err)
	}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" && line != `\.` {
			count++
		}
	}
	if count > 0 && batch.Desc.HasHeader {
		count--
	}
	return count, scanner.Err()
}
