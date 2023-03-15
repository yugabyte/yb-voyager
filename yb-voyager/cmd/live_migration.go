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
	Op            string            `json:"op"`
	SchemaName    string            `json:"schema_name"`
	TableName     string            `json:"table_name"`
	QualifiedName string            `json:"qualified_name"`
	Key           map[string]string `json:"key"`
	Fields        map[string]string `json:"fields"`
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
		panic("unknown op: " + e.Op)
	}
}

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s);"
const updateTemplate = "UPDATE %s SET %s WHERE %s;"
const deleteTemplate = "DELETE FROM %s WHERE %s;"

func (event *Event) getInsertStmt() string {
	columnList := make([]string, 0, len(event.Fields))
	valueList := make([]string, 0, len(event.Fields))
	for column, value := range event.Fields {
		columnList = append(columnList, column)
		valueList = append(valueList, value)
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	stmt := fmt.Sprintf(insertTemplate, event.QualifiedName, columns, values)
	return stmt
}

func (event *Event) getUpdateStmt() string {
	var setClauses []string
	for column, value := range event.Fields {
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", column, value))
	}
	setClause := strings.Join(setClauses, ", ")
	var whereClauses []string
	for column, value := range event.Key {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(updateTemplate, event.QualifiedName, setClause, whereClause)
}

func (event *Event) getDeleteStmt() string {
	var whereClauses []string
	for column, value := range event.Key {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, event.QualifiedName, whereClause)
}

//==================================================================================

func streamChanges(connPool *tgtdb.ConnectionPool, sourceDBType string, targetSchema string) error {
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

		event.QualifiedName = event.SchemaName + "." + event.TableName
		if sourceDBType == ORACLE || sourceDBType == MYSQL {
			if targetSchema == YUGABYTEDB_DEFAULT_SCHEMA {
				event.QualifiedName = event.TableName
			} else {
				event.SchemaName = targetSchema
				event.QualifiedName = targetSchema + "." + event.TableName
			}
		}

		err = handleEvent(connPool, &event)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
	}
	return nil
}

func handleEvent(connPool *tgtdb.ConnectionPool, event *Event) error {
	log.Debugf("Handling event: %v", event)
	stmt := event.GetSQLStmt()
	log.Debug(stmt)
	err := connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		tag, err := conn.Exec(context.Background(), stmt)
		if err != nil {
			log.Errorf("Error executing stmt: %v", err)
		}
		log.Debugf("Executed stmt [ %s ]: rows affected => %v", stmt, tag.RowsAffected())
		return false, err
	})
	// Idempotency considerations:
	// Note: Assuming PK column value is not changed via UPDATEs
	// INSERT: The connPool sets `yb_enable_upsert_mode to true`. Hece the insert will be
	// successful even if the row already exists.
	// DELETE does NOT fail if the row does not exist. Rows affected will be 0.
	// UPDATE statement does not fail if the row does not exist. Rows affected will be 0.

	return err
}
