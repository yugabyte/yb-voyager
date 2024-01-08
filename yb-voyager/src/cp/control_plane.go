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
package cp

import (
	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ControlPlane interface {
	Init() error
	ExportSchemaStarted(*ExportSchemaEvent)
	ExportSchemaCompleted(*ExportSchemaEvent) // Only success is reported.

	// SubmitMigrationAssessmentReport(*MigrationAssessmentReport)

	SchemaAnalysisStarted(*SchemaAnalysisEvent)
	SchemaAnalysisIterationCompleted(*SchemaAnalysisReport)

	SnapshotExportStarted(*SnapshotExportEvent)
	UpdateExportedRowCount([]*SnapshotExportTableMetrics)
	SnapshotExportCompleted(*SnapshotExportEvent)

	// ChangeStreamExportStarted()
	// UpdateExportedChangesCount(*ExportedChangesCounter)

	ImportSchemaStarted(*ImportSchemaEvent)
	ImportSchemaCompleted(*ImportSchemaEvent)

	SnapshotImportStarted(*SnapshotImportEvent)
	UpdateImportedRowCount([]*SnapshotImportTableMetrics)
	SnapshotImportCompleted(*SnapshotImportEvent)

	Finalize()

	// ChangeStreamImportStarted()
	// UpdateImportedChangesCount(*ImportedChangesCounter)

	// CutoverInitiated()
	// CutoverCompleted()
}

type ExportSchemaEvent struct {
	MigrationUUID uuid.UUID
	DatabaseName  string
	SchemaName    string
	DBType        string
}

type SchemaAnalysisEvent struct {
	MigrationUUID uuid.UUID
	DatabaseName  string
	SchemaName    string
	DBType        string
}

type SchemaAnalysisReport struct {
	MigrationUUID  uuid.UUID
	DatabaseName   string
	SchemaName     string
	DBType         string
	AnalysisReport utils.Report
}

type SnapshotExportEvent struct {
	MigrationUUID uuid.UUID
	DatabaseName  string
	SchemaName    string
	DBType        string
}

type SnapshotExportTableMetrics struct {
	MigrationUUID  uuid.UUID
	SchemaName     string
	TableName      string
	Status         int //(0: NOT-STARTED, 1: IN-PROGRESS, 2: DONE, 3: COMPLETED)
	CountLiveRows  int64
	CountTotalRows int64
}

type ImportSchemaEvent struct {
	MigrationUUID uuid.UUID
	DatabaseName  string
	SchemaName    string
}

type SnapshotImportEvent struct {
	MigrationUUID uuid.UUID
	DatabaseName  string
	SchemaName    string
}

type SnapshotImportTableMetrics struct {
	MigrationUUID  uuid.UUID
	SchemaName     string
	TableName      string
	Status         int //(0: NOT-STARTED, 1: IN-PROGRESS, 2: DONE, 3: COMPLETED)
	CountLiveRows  int64
	CountTotalRows int64
}
