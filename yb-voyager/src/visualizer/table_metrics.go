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
package visualizer

import "github.com/google/uuid"

type VisualizerTableMetrics struct {
	MigrationUUID       uuid.UUID `json:"migration_uuid"`
	TableName           string    `json:"table_name"`
	Schema              string    `json:"schema_name"`
	MigrationPhase      int       `json:"migration_phase"`
	Status              int       `json:"status"` //(0: NOT-STARTED, 1: IN-PROGRESS, 2: DONE, 3: COMPLETED)
	CountLiveRows       int64     `json:"count_live_rows"`
	CountTotalRows      int64     `json:"count_total_rows"`
	InvocationTimestamp string    `json:"invocation_timestamp"`
}

var TABLE_STATUS_MAP = map[string]int{
	"NOT STARTED": 0,
	"IN PROGRESS": 1,
	"DONE":        2,
	"COMPLETED":   3,
}

// Create a table metrics struct
func CreateVisualzerDBTableMetrics(mUUID uuid.UUID,
	tableName string,
	schema string,
	phase int,
	status int,
	completedRows int64,
	totalRows int64,
	timestamp string) VisualizerTableMetrics {

	return VisualizerTableMetrics{
		MigrationUUID:       mUUID,
		TableName:           tableName,
		Schema:              schema,
		MigrationPhase:      phase,
		Status:              status,
		CountLiveRows:       completedRows,
		CountTotalRows:      totalRows,
		InvocationTimestamp: timestamp,
	}
}
