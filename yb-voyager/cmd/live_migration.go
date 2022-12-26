package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Event struct {
	Op         string            `json:"op"`
	SchemaName string            `json:"schema_name"`
	TableName  string            `json:"table_name"`
	Key        map[string]string `json:"key"`
	After      map[string]string `json:"after"`
}

func streamChanges() error {
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
		err = handleEvent(&event)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
	}
	return nil
}

func handleEvent(event *Event) error {
	log.Infof("Handling event: %v", event)
	switch event.Op {
	case "c":
		return handleCreateEvent(event)
	case "u":
		return handleUpdateEvent(event)
	case "d":
		return handleDeleteEvent(event)
	default:
		return fmt.Errorf("unknown event op: %s", event.Op)
	}
}

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s);"

func handleCreateEvent(event *Event) error {
	tableName := event.SchemaName + "." + event.TableName
	columnList := make([]string, 0, len(event.After))
	valueList := make([]string, 0, len(event.After))
	for column, value := range event.After {
		columnList = append(columnList, column)
		valueList = append(valueList, value)
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	query := fmt.Sprintf(insertTemplate, tableName, columns, values)
	fmt.Println(query)
	return nil
}

const updateTemplate = "UPDATE %s SET %s WHERE %s;"

func handleUpdateEvent(event *Event) error {
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
	query := fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
	fmt.Println(query)
	return nil
}

const deleteTemplate = "DELETE FROM %s WHERE %s;"

func handleDeleteEvent(event *Event) error {
	tableName := event.SchemaName + "." + event.TableName
	var whereClauses []string
	for column, value := range event.Key {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	query := fmt.Sprintf(deleteTemplate, tableName, whereClause)
	fmt.Println(query)
	return nil
}
