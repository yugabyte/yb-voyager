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
package noopcp

import "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"

type NoopControlPlane struct {
}

func New() *NoopControlPlane {
	return &NoopControlPlane{}
}

func (cp *NoopControlPlane) Init() error {
	return nil
}

func (cp *NoopControlPlane) Finalize() {
}

func (cp *NoopControlPlane) MigrationAssessmentStarted(_ *cp.MigrationAssessmentStartedEvent) {
}

func (cp *NoopControlPlane) MigrationAssessmentCompleted(_ *cp.MigrationAssessmentCompletedEvent) {
}

func (cp *NoopControlPlane) ExportSchemaStarted(exportSchemaEvent *cp.ExportSchemaStartedEvent) {
}

func (cp *NoopControlPlane) ExportSchemaCompleted(exportSchemaEvent *cp.ExportSchemaCompletedEvent) {
}

func (cp *NoopControlPlane) SchemaAnalysisStarted(schemaAnalysisEvent *cp.SchemaAnalysisStartedEvent) {
}

func (cp *NoopControlPlane) SchemaAnalysisIterationCompleted(schemaAnalysisReport *cp.SchemaAnalysisIterationCompletedEvent) {
}

func (cp *NoopControlPlane) SnapshotExportStarted(snapshotExportEvent *cp.SnapshotExportStartedEvent) {
}

func (cp *NoopControlPlane) UpdateExportedRowCount(
	snapshotExportTablesMetrics []*cp.UpdateExportedRowCountEvent) {
}

func (cp *NoopControlPlane) SnapshotExportCompleted(snapshotExportEvent *cp.SnapshotExportCompletedEvent) {
}

func (cp *NoopControlPlane) ImportSchemaStarted(importSchemaEvent *cp.ImportSchemaStartedEvent) {
}

func (cp *NoopControlPlane) ImportSchemaCompleted(importSchemaEvent *cp.ImportSchemaCompletedEvent) {
}

func (cp *NoopControlPlane) SnapshotImportStarted(snapshotImportEvent *cp.SnapshotImportStartedEvent) {
}

func (cp *NoopControlPlane) UpdateImportedRowCount(
	snapshotImportTableMetrics []*cp.UpdateImportedRowCountEvent) {
}

func (cp *NoopControlPlane) SnapshotImportCompleted(snapshotImportEvent *cp.SnapshotImportCompletedEvent) {
}

func (cp *NoopControlPlane) MigrationEnded(migrationEndedEvent *cp.MigrationEndedEvent) {
}
