/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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
	Fields     map[string]string `json:"fields"`
}

func (e *Event) GetSQLStmt(targetSchema string) string {
	switch e.Op {
	case "c":
		return e.getInsertStmt(targetSchema)
	case "u":
		return e.getUpdateStmt(targetSchema)
	case "d":
		return e.getDeleteStmt(targetSchema)
	default:
		panic("unknown op: " + e.Op)
	}
}

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s);"
const updateTemplate = "UPDATE %s SET %s WHERE %s;"
const deleteTemplate = "DELETE FROM %s WHERE %s;"

func (event *Event) getInsertStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	columnList := make([]string, 0, len(event.Fields))
	valueList := make([]string, 0, len(event.Fields))
	for column, value := range event.Fields {
		columnList = append(columnList, column)
		valueList = append(valueList, value)
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	stmt := fmt.Sprintf(insertTemplate, tableName, columns, values)
	return stmt
}

func (event *Event) getUpdateStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
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
	return fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
}

func (event *Event) getDeleteStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	var whereClauses []string
	for column, value := range event.Key {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, tableName, whereClause)
}

//==================================================================================

func streamChanges(connPool *tgtdb.ConnectionPool, targetSchema string) error {
	cdcDirPath := filepath.Join(exportDir, "data", "cdc")
	archiveDirPath := filepath.Join(exportDir, "data", "cdc", "archive")
	err := os.MkdirAll(archiveDirPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating archive dir: %v", err)
	}

	// keep looking for segments continuously
	// and stream changes from segments one by one in sequence
	for {
		dirEntries, err := os.ReadDir(cdcDirPath)
		if err != nil {
			return fmt.Errorf("error reading cdc dir: %v", err)
		}

		var queueSegmentFiles []string
		for _, dirEntry := range dirEntries {
			if !dirEntry.IsDir() && strings.HasSuffix(dirEntry.Name(), ".ndjson") {
				queueSegmentFiles = append(queueSegmentFiles, dirEntry.Name())
			}
		}

		// sort files by segment number
		sort.Slice(queueSegmentFiles, func(i, j int) bool {
			segNumIdx := len("queue.")
			len1 := len(queueSegmentFiles[i])
			len2 := len(queueSegmentFiles[j])

			segNum1, err := strconv.ParseInt(queueSegmentFiles[i][segNumIdx:len1-7], 10, 64)
			if err != nil {
				utils.ErrExit("error parsing segment file name for segNum1: %v", err)
			}

			segNum2, err := strconv.ParseInt(queueSegmentFiles[j][segNumIdx:len2-7], 10, 64)
			if err != nil {
				utils.ErrExit("error parsing segment file name for segNum2: %v", err)
			}

			return segNum1 < segNum2
		})

		// walk through the segment files in cdc dir
		for _, queueSegmentFile := range queueSegmentFiles {
			queueFilePath := filepath.Join(cdcDirPath, queueSegmentFile)
			log.Infof("streaming changes from %s", queueFilePath)
			err = streamChangesFromFile(connPool, queueFilePath, targetSchema)
			if err != nil {
				return fmt.Errorf("error streaming changes from %s: %v", queueSegmentFile, err)
			}

			// move queue file to archive directory once streamed
			archivePath := filepath.Join(archiveDirPath, filepath.Base(queueSegmentFile))
			err = os.Rename(queueFilePath, archivePath)
			if err != nil {
				return fmt.Errorf("error archiving file %s: %v", queueFilePath, err)
			}
		}
		time.Sleep(time.Second * 2)
	}
}

func streamChangesFromFile(connPool *tgtdb.ConnectionPool, path string, targetSchema string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0640)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", path, err)
	}
	defer file.Close()

	eventsCount := int64(0)
	r := utils.NewTailReader(file)
	for {
		line, err := r.ReadLine()
		if string(line) == `\.` && err == io.EOF {
			log.Infof("reached end of file %s", path)
			break
		} else if err != nil {
			return fmt.Errorf("error reading line from file %s: %v", path, err)
		}

		var event Event
		err = json.Unmarshal(line, &event)
		if err != nil {
			return fmt.Errorf("error decoding change: %v", err)
		}

		err = handleEvent(connPool, &event, targetSchema)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
		eventsCount++
	}
	// TODO: remove this line once we have status commands
	utils.PrintAndLog("Processed %d events from %s", eventsCount, filepath.Base(path))

	return nil
}

func handleEvent(connPool *tgtdb.ConnectionPool, event *Event, targetSchema string) error {
	log.Debugf("Handling event: %v", event)
	stmt := event.GetSQLStmt(targetSchema)
	log.Debug(stmt)
	err := connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
		tag, err := conn.Exec(context.Background(), stmt)
		if err != nil {
			log.Errorf("Error executing stmt: %v", err)
		}
		log.Debugf("Executed stmt [ %s ]: rows affected => %v", stmt, tag.RowsAffected())
		return false, err
	})
	// Idempotency considerations(considering tables with PK):
	// Note: Assuming PK column value is not changed via UPDATEs
	// INSERT: The connPool sets `yb_enable_upsert_mode to true`. Hence the insert will be
	// successful even if the row already exists.
	// DELETE does NOT fail if the row does not exist. Rows affected will be 0.
	// UPDATE statement does not fail if the row does not exist. Rows affected will be 0.

	return err
}
