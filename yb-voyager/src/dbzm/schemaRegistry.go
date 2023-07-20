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
package dbzm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type DbzSchema struct {
	Type string `json:"type"`
	Name string `json:"name"`
	// Not decoding the rest of the fields for now.
}

type Schema struct {
	ColName      string    `json:"name"`
	Index        int64     `json:"index"`
	ColDbzSchema DbzSchema `json:"schema"`
}

type TableSchema struct {
	Columns []Schema `json:"columns"`
}

func GetTableSchema(tableName string, exportDir string) TableSchema {
	schemaFileName := fmt.Sprintf("%s_schema.json", tableName)
	schemaFilePath := filepath.Join(exportDir, "data", "schemas", schemaFileName)
	schemaFile, err := os.Open(schemaFilePath)
	if err != nil {
		utils.ErrExit("Failed to open schema file %s: %v", schemaFilePath, err)
	}
	defer schemaFile.Close()
	var tableSchema TableSchema
	err = json.NewDecoder(schemaFile).Decode(&tableSchema)
	if err != nil {
		utils.ErrExit("Failed to decode schema file %s: %v", schemaFilePath, err)
	}
	return tableSchema

}

func (ts *TableSchema) GetColumnSchema(columnName string) Schema {
	for _, colSchema := range ts.Columns {
		if colSchema.ColName == columnName {
			return colSchema
		}
	}
	return Schema{}
}
