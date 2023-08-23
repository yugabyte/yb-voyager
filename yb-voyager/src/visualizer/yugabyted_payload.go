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

type VisualizerDBPayload struct {
	MigrationUUID       uuid.UUID `json:"migration_uuid"`
	MigrationPhase      int       `json:"migration_phase"`
	InvocationSequence  int       `json:"invocation_sequence"`
	MigrationDirectory  string    `json:"migration_dir"`
	DatabaseName        string    `json:"database_name"`
	SchemaName          string    `json:"schema_name"`
	Payload             string    `json:"payload"`
	Status              string    `json:"status"`
	InvocationTimestamp string    `json:"invocation_timestamp"`
}

var MIGRATION_PHASE_MAP = map[string]int{
	"EXPORT SCHEMA":  0,
	"ANALYZE SCHEMA": 1,
	"EXPORT DATA":    2,
	"IMPORT SCHEMA":  3,
	"IMPORT DATA":    4,
}

// Create a visualisation metadata struct
func CreateVisualzerDBPayload(mUUID uuid.UUID,
	phase int,
	invocationSequence int,
	dbName string,
	schema string,
	visualizerPayload string,
	status string,
	timestamp string) VisualizerDBPayload {

	return VisualizerDBPayload{
		MigrationUUID:       mUUID,
		MigrationPhase:      phase,
		InvocationSequence:  invocationSequence,
		DatabaseName:        dbName,
		SchemaName:          schema,
		Payload:             visualizerPayload,
		Status:              status,
		InvocationTimestamp: timestamp,
	}
}
