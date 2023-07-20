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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func streamChanges() error {
	queueFilePath := filepath.Join(exportDir, "data", "queue.json")
	log.Infof("Streaming changes from %s", queueFilePath)
	file, err := os.OpenFile(queueFilePath, os.O_CREATE, 0640)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", queueFilePath, err)
	}
	defer file.Close()
    valueConverterSuite = tdb.GetDebeziumValueConverterSuite(true)
	r := utils.NewTailReader(file)
	dec := json.NewDecoder(r)
	log.Infof("Waiting for changes in %s", queueFilePath)
	// TODO: Batch the changes.
	for dec.More() {
		dec.UseNumber()
		var event tgtdb.Event
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

func handleEvent(event *tgtdb.Event) error {
	log.Debugf("Handling event: %v", event)
	tableName := event.TableName
	if sourceDBType == "postgresql" && event.SchemaName != "public" {
		tableName = event.SchemaName + "." + event.TableName
	}
	// preparing value converters for the streaming mode
	err := transformEventKeyFields(event, tableName)
	if err != nil {
		return fmt.Errorf("error transforming event key fields: %v", err)
	}
	batch := []*tgtdb.Event{event}
	err = tdb.ExecuteBatch(batch)
	if err != nil {
		return fmt.Errorf("error executing batch: %v", err)
	}
	return nil
}

func transformEventKeyFields(event *tgtdb.Event, tableName string) error {
	tableSchema := dbzm.GetTableSchema(tableName, exportDir)
	columnToSchema := make(map[string]dbzm.Schema)
	for _, column := range tableSchema.Columns {
		columnToSchema[column.ColName] = column
	}
	for column, value := range event.Fields {
		var columnValue string
		if value == nil {
			columnValue = "NULL"
		} else {
			var err error
			columnValue = fmt.Sprintf("%v", value)
			columnValue, err = dbzm.TransformValue(columnValue, columnToSchema[column], valueConverterSuite)
			if err != nil {
				return fmt.Errorf("error transforming value: %v", err)
			}
		}
		event.Fields[column] = columnValue
	}
	for column, value := range event.Key {
		var columnValue string
		if value == nil {
			columnValue = "NULL"
		} else {
			var err error
			columnValue = fmt.Sprintf("%v", value)
			columnValue, err = dbzm.TransformValue(columnValue, columnToSchema[column], valueConverterSuite)
			if err != nil {
				return fmt.Errorf("error transforming value: %v", err)
			}
		}
		event.Key[column] = columnValue
	}
	return nil
}
