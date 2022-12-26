package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Event struct {
	Op         string            `json:"op"`
	SchemaName string            `json:"schema_name"`
	TableName  string            `json:"table_name"`
	Key        map[string]string `json:"key"`
	After      map[string]string `json:"after"`
}

func (e *Event) GetSQLStmt() string {
	switch e.Op {
	case "c":
		return e.getInsertStmt()
	case "u":
		return e.getUpdateStmt()
	case "d":
		return e.getDeleteStmt()
	default:
		return ""
	}
}

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s);"
const updateTemplate = "UPDATE %s SET %s WHERE %s;"
const deleteTemplate = "DELETE FROM %s WHERE %s;"

func (event *Event) getInsertStmt() string {
	tableName := event.SchemaName + "." + event.TableName
	columnList := make([]string, 0, len(event.After))
	valueList := make([]string, 0, len(event.After))
	for column, value := range event.After {
		columnList = append(columnList, column)
		valueList = append(valueList, value)
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	stmt := fmt.Sprintf(insertTemplate, tableName, columns, values)
	return stmt
}

func (event *Event) getUpdateStmt() string {
	tableName := event.SchemaName + "." + event.TableName
	var setClauses []string
	for column, value := range event.After {
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", column, value))
	}
	setClause := strings.Join(setClauses, ", ")
	var whereClauses []string
	for column, value := range event.Key {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
}

func (event *Event) getDeleteStmt() string {
	tableName := event.SchemaName + "." + event.TableName
	var whereClauses []string
	for column, value := range event.Key {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, tableName, whereClause)
}

//==================================================================================

func streamChanges(connPool *tgtdb.ConnectionPool) error {
	queueFilePath := filepath.Join(exportDir, "data", "queue.json")
	log.Infof("Streaming changes from %s", queueFilePath)
	file, err := os.OpenFile(queueFilePath, os.O_CREATE, 0640)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", queueFilePath, err)
	}
	defer file.Close()

	r := utils.NewTailReader(file)
	dec := json.NewDecoder(r)
	log.Infof("Waiting for changes in %s", queueFilePath)
	// TODO: Batch the changes.
	for dec.More() {
		var event Event
		err := dec.Decode(&event)
		if err != nil {
			return fmt.Errorf("error decoding change: %v", err)
		}
		err = handleEvent(connPool, &event)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
	}
	return nil
}

func handleEvent(connPool *tgtdb.ConnectionPool, event *Event) error {
	log.Infof("Handling event: %v", event)
	stmt := event.GetSQLStmt()
	utils.PrintAndLog(stmt)
	err := connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		tag, err := conn.Exec(context.Background(), stmt)
		if err != nil {
			log.Errorf("Error executing stmt: %v", err)
		}
		log.Infof("Executed stmt [ %s ]: rows affected => %v", stmt, tag.RowsAffected())
		return false, err
	})
	// Idempotency considerations:
	// INSERT: The connPool sets `yb_enable_upsert_mode to true`. Hece the insert will be
	// successful even if the row already exists.
	// DELETE does NOT fail if the row does not exist. Rows affected will be 0.
	// UPDATE statement does not fail if the row does not exist. Rows affected will be 0.

	return err
}
