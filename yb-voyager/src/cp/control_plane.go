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
	"fmt"
	"runtime"
	"strings"
	"syscall"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// Status mappings for data export/import
var EXPORT_OR_IMPORT_DATA_STATUS_INT_TO_STR = map[int]string{
	0: "NOT STARTED",
	1: "IN PROGRESS",
	2: "DONE",
	3: "COMPLETED",
}

// Inverse mapping: status strings to integers
var UPDATE_ROW_COUNT_STATUS_STR_TO_INT = map[string]int{
	"NOT STARTED": 0,
	"IN PROGRESS": 1,
	"DONE":        2,
	"COMPLETED":   3,
}

// Migration phase mappings
var MIGRATION_PHASE_MAP = map[string]int{
	"ASSESS MIGRATION": 1,
	"EXPORT SCHEMA":    2,
	"ANALYZE SCHEMA":   3,
	"EXPORT DATA":      4,
	"IMPORT SCHEMA":    5,
	"IMPORT DATA":      6,
}

// IsExportPhase checks if the event type is an export phase (phases 1-4)
func IsExportPhase(eventType string) bool {
	return MIGRATION_PHASE_MAP[eventType] <= 4
}

// IsImportPhase checks if the event type is an import phase (phases 5-6)
func IsImportPhase(eventType string) bool {
	return MIGRATION_PHASE_MAP[eventType] > 4
}

type ControlPlane interface {
	Init() error
	Finalize()

	MigrationAssessmentStarted(*MigrationAssessmentStartedEvent)
	MigrationAssessmentCompleted(*MigrationAssessmentCompletedEvent)

	ExportSchemaStarted(*ExportSchemaStartedEvent)
	ExportSchemaCompleted(*ExportSchemaCompletedEvent) // Only success is reported.

	SchemaAnalysisStarted(*SchemaAnalysisStartedEvent)
	SchemaAnalysisIterationCompleted(*SchemaAnalysisIterationCompletedEvent)

	SnapshotExportStarted(*SnapshotExportStartedEvent)
	UpdateExportedRowCount([]*UpdateExportedRowCountEvent)
	SnapshotExportCompleted(*SnapshotExportCompletedEvent)

	ImportSchemaStarted(*ImportSchemaStartedEvent)
	ImportSchemaCompleted(*ImportSchemaCompletedEvent)

	SnapshotImportStarted(*SnapshotImportStartedEvent)
	UpdateImportedRowCount([]*UpdateImportedRowCountEvent)
	SnapshotImportCompleted(*SnapshotImportCompletedEvent)

	MigrationEnded(*MigrationEndedEvent)
}

func GetSchemaList(schema string) []string {
	return strings.Split(schema, "|")
}

func SplitTableNameForPG(tableName string) (string, string) {
	splitTableName := strings.Split(tableName, ".")
	return splitTableName[0], splitTableName[1]
}

type BaseEvent struct {
	EventType     string
	MigrationUUID uuid.UUID
	DBType        string
	DatabaseName  string
	SchemaNames   []string
	DBIP          []string
	Port          int
	DBVersion     string
}

type BaseUpdateRowCountEvent struct {
	BaseEvent
	TableName         string
	Status            string
	TotalRowCount     int64
	CompletedRowCount int64
}

type VoyagerInstance struct {
	IP                 string
	OperatingSystem    string
	DiskSpaceAvailable uint64 // Available disk space in bytes
	ExportDirectory    string
}

// TableMetrics represents table-level metrics for both export and import phases
// This is used by both YBM and Yugabyted control plane implementations
type TableMetrics struct {
	MigrationUUID       uuid.UUID `json:"migration_uuid"`
	TableName           string    `json:"table_name"`
	SchemaName          string    `json:"schema_name"`
	MigrationPhase      int       `json:"migration_phase"`
	Status              int       `json:"status"` // 0: NOT-STARTED, 1: IN-PROGRESS, 2: DONE, 3: COMPLETED
	CountLiveRows       int64     `json:"count_live_rows"`
	CountTotalRows      int64     `json:"count_total_rows"`
	InvocationTimestamp string    `json:"invocation_timestamp"`
}

// PrepareVoyagerInstance gathers information about the Voyager instance
func PrepareVoyagerInstance(exportDir string) *VoyagerInstance {
	ip, err := utils.GetLocalIP()
	log.Infof("voyager machine ip: %s\n", ip)
	if err != nil {
		log.Warnf("failed to obtain local IP address: %v", err)
	}

	//TODO: for import data cmd, the START and COMPLETE readings of available disk space can be very different
	diskSpace, err := getAvailableDiskSpace(exportDir)
	log.Infof("voyager disk space available: %d\n", diskSpace)
	if err != nil {
		log.Warnf("failed to determine available disk space: %v", err)
	}

	return &VoyagerInstance{
		IP:                 ip,
		OperatingSystem:    runtime.GOOS,
		DiskSpaceAvailable: diskSpace,
		ExportDirectory:    exportDir,
	}
}

// getAvailableDiskSpace returns available disk space for a given directory path
func getAvailableDiskSpace(dirPath string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dirPath, &stat); err != nil {
		return 0, fmt.Errorf("error getting disk space for directory %q: %w", dirPath, err)
	}
	// Available blocks * size per block to get available space in bytes
	return stat.Bavail * uint64(stat.Bsize), nil
}

type ExportSchemaStartedEvent struct {
	BaseEvent
}

type ExportSchemaCompletedEvent struct {
	BaseEvent
}

type MigrationAssessmentStartedEvent struct {
	BaseEvent
}

type MigrationAssessmentCompletedEvent struct {
	BaseEvent
	// Report field allows control plane decoupling:
	// - Yugabyted: receives a JSON string (pre-marshaled in yugabyted_event_builder.go)
	// - YBM: receives AssessMigrationPayloadYBM struct (marshaled once when sending to API)
	// This enables each control plane to optimize its data flow independently
	Report interface{}
}

type SchemaAnalysisStartedEvent struct {
	BaseEvent
}

type SchemaAnalysisIterationCompletedEvent struct {
	BaseEvent
	// AnalysisReport is a common struct (utils.SchemaReport) used by both control planes
	// - Yugabyted: marshals it to JSON string before storing in database
	// - YBM: passes struct directly, marshaled once when sending to API
	// Both achieve single marshal, different patterns based on storage mechanism
	AnalysisReport utils.SchemaReport
}

type SnapshotExportStartedEvent struct {
	BaseEvent
}

type UpdateExportedRowCountEvent struct {
	BaseUpdateRowCountEvent
}

type SnapshotExportCompletedEvent struct {
	BaseEvent
}

type ImportSchemaStartedEvent struct {
	BaseEvent
}

type ImportSchemaCompletedEvent struct {
	BaseEvent
}

type SnapshotImportStartedEvent struct {
	BaseEvent
}

type UpdateImportedRowCountEvent struct {
	BaseUpdateRowCountEvent
}

type SnapshotImportCompletedEvent struct {
	BaseEvent
}

type MigrationEndedEvent struct {
	BaseEvent
}
