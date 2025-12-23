//go:build unit

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
package callhome

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestCallhomeStructs(t *testing.T) {
	tests := []struct {
		name         string
		actualType   reflect.Type
		expectedType interface{}
	}{
		{
			name:       "Validate Payload Struct Definition",
			actualType: reflect.TypeOf(Payload{}),
			expectedType: struct {
				MigrationUUID    uuid.UUID `json:"migration_uuid"`
				PhaseStartTime   string    `json:"phase_start_time"`
				CollectedAt      string    `json:"collected_at"`
				SourceDBDetails  string    `json:"source_db_details"`
				TargetDBDetails  string    `json:"target_db_details"`
				YBVoyagerVersion string    `json:"yb_voyager_version"`
				MigrationPhase   string    `json:"migration_phase"`
				PhasePayload     string    `json:"phase_payload"`
				MigrationType    string    `json:"migration_type"`
				TimeTakenSec     int       `json:"time_taken_sec"`
				Status           string    `json:"status"`
			}{},
		},
		{
			name:       "Validate SourceDBDetails Struct Definition",
			actualType: reflect.TypeOf(SourceDBDetails{}),
			expectedType: struct {
				PayloadVersion     string   `json:"payload_version"`
				Host               string   `json:"host"`
				DBType             string   `json:"db_type"`
				DBVersion          string   `json:"db_version"`
				DBSize             int64    `json:"total_db_size_bytes"`
				Role               string   `json:"role,omitempty"`
				DBSystemIdentifier int64    `json:"db_system_identifier,omitempty"`
				DBName             string   `json:"db_name,omitempty"`
				SchemaNames        []string `json:"schema_names,omitempty"`
			}{},
		},
		{
			name:       "Validate TargetDBDetails Struct Definition",
			actualType: reflect.TypeOf(TargetDBDetails{}),
			expectedType: struct {
				Host               string `json:"host"`
				DBVersion          string `json:"db_version"`
				NodeCount          int    `json:"node_count"`
				Cores              int    `json:"total_cores"`
				DBSystemIdentifier string `json:"db_system_identifier,omitempty"`
			}{},
		},
		{
			name:       "Validate AssessMigrationPhasePayload Struct Definition",
			actualType: reflect.TypeOf(AssessMigrationPhasePayload{}),
			expectedType: struct {
				PayloadVersion                 string                     `json:"payload_version"`
				TargetDBVersion                *ybversion.YBVersion       `json:"target_db_version"`
				Sizing                         *SizingCallhome            `json:"sizing"`
				MigrationComplexity            string                     `json:"migration_complexity"`
				MigrationComplexityExplanation string                     `json:"migration_complexity_explanation"`
				SchemaSummary                  string                     `json:"schema_summary"`
				Issues                         []AssessmentIssueCallhome  `json:"assessment_issues"`
				Error                          string                     `json:"error"`
				TableSizingStats               string                     `json:"table_sizing_stats"`
				IndexSizingStats               string                     `json:"index_sizing_stats"`
				SourceConnectivity             bool                       `json:"source_connectivity"`
				IopsInterval                   int64                      `json:"iops_interval"`
				ControlPlaneType               string                     `json:"control_plane_type"`
				AnonymizedDDLs                 []string                   `json:"anonymized_ddls"`
				ReplicaAssessmentTopology      *ReplicaAssessmentTopology `json:"replica_assessment_topology,omitempty"`
			}{},
		},
		{
			name:       "Validate ReplicaAssessmentTopology Struct Definition",
			actualType: reflect.TypeOf(ReplicaAssessmentTopology{}),
			expectedType: struct {
				ReplicasDiscovered int `json:"replicas_discovered"`
				ReplicasProvided   int `json:"replicas_provided"`
				ReplicasUsed       int `json:"replicas_used"`
			}{},
		},
		{
			name:       "Validate AssessmentIssueCallhome Struct Definition",
			actualType: reflect.TypeOf(AssessmentIssueCallhome{}),
			expectedType: struct {
				Category            string                 `json:"category"`
				CategoryDescription string                 `json:"category_description"`
				Type                string                 `json:"type"`
				Name                string                 `json:"name"`
				Impact              string                 `json:"impact"`
				ObjectType          string                 `json:"object_type"`
				ObjectName          string                 `json:"object_name"`
				ObjectUsage         string                 `json:"object_usage,omitempty"`
				SqlStatement        string                 `json:"sql_statement,omitempty"`
				Details             map[string]interface{} `json:"details,omitempty"`
			}{},
		},
		{
			name:       "Validate SizingCallhome Struct Definition",
			actualType: reflect.TypeOf(SizingCallhome{}),
			expectedType: struct {
				ColocatedTables                                    []string `json:"colocated_tables"`
				ColocatedReasoning                                 string   `json:"colocated_reasoning"`
				ShardedTables                                      []string `json:"sharded_tables"`
				NumNodes                                           float64  `json:"num_nodes"`
				VCPUsPerInstance                                   int      `json:"vcpus_per_instance"`
				MemoryPerInstance                                  int      `json:"memory_per_instance"`
				OptimalSelectConnectionsPerNode                    int64    `json:"optimal_select_connections_per_node"`
				OptimalInsertConnectionsPerNode                    int64    `json:"optimal_insert_connections_per_node"`
				EstimatedTimeInMinForImport                        float64  `json:"estimated_time_in_min_for_import"`
				EstimatedTimeInMinForImportWithoutRedundantIndexes float64  `json:"estimated_time_in_min_for_import_without_redundant_indexes"`
			}{},
		},
		{
			name:       "Validate AssessMigrationBulkPhasePayload Struct Definition",
			actualType: reflect.TypeOf(AssessMigrationBulkPhasePayload{}),
			expectedType: struct {
				FleetConfigCount int    `json:"fleet_config_count"`
				Error            string `json:"error"`
				ControlPlaneType string `json:"control_plane_type"`
			}{},
		},
		{
			name:       "Validate ObjectSizingStats Struct Definition",
			actualType: reflect.TypeOf(ObjectSizingStats{}),
			expectedType: struct {
				SchemaName      string `json:"schema_name,omitempty"`
				ObjectName      string `json:"object_name"`
				ReadsPerSecond  int64  `json:"reads_per_second"`
				WritesPerSecond int64  `json:"writes_per_second"`
				SizeInBytes     int64  `json:"size_in_bytes"`
			}{},
		},
		{
			name:       "Validate ExportSchemaPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ExportSchemaPhasePayload{}),
			expectedType: struct {
				PayloadVersion            string                     `json:"payload_version"`
				StartClean                bool                       `json:"start_clean"`
				AppliedRecommendations    bool                       `json:"applied_recommendations"`
				UseOrafce                 bool                       `json:"use_orafce"`
				CommentsOnObjects         bool                       `json:"comments_on_objects"`
				SkipRecommendations       bool                       `json:"skip_recommendations"`
				AssessRunInExportSchema   bool                       `json:"assess_run_in_export_schema"`
				SkipPerfOptimizations     bool                       `json:"skip_performance_optimizations"`
				Error                     string                     `json:"error"`
				ControlPlaneType          string                     `json:"control_plane_type"`
				SchemaOptimizationChanges []SchemaOptimizationChange `json:"schema_optimization_changes"`
			}{},
		},
		{
			name:       "Validate AnalyzePhasePayload Struct Definition",
			actualType: reflect.TypeOf(AnalyzePhasePayload{}),
			expectedType: struct {
				PayloadVersion   string                 `json:"payload_version"`
				TargetDBVersion  *ybversion.YBVersion   `json:"target_db_version"`
				Issues           []AnalyzeIssueCallhome `json:"issues"`
				DatabaseObjects  string                 `json:"database_objects"`
				Error            string                 `json:"error"`
				ControlPlaneType string                 `json:"control_plane_type"`
			}{},
		},
		{
			name:       "Validate AnalyzeIssueCallhome Struct Definition",
			actualType: reflect.TypeOf(AnalyzeIssueCallhome{}),
			expectedType: struct {
				Category   string `json:"category"`
				Type       string `json:"type"`
				Name       string `json:"name"`
				Impact     string `json:"impact"`
				ObjectType string `json:"object_type"`
				ObjectName string `json:"object_name"`
			}{},
		},
		{
			name:       "Validate ExportDataPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ExportDataPhasePayload{}),
			expectedType: struct {
				PayloadVersion            string          `json:"payload_version"`
				ParallelJobs              int64           `json:"parallel_jobs"`
				TotalRows                 int64           `json:"total_rows_exported"`
				LargestTableRows          int64           `json:"largest_table_rows_exported"`
				StartClean                bool            `json:"start_clean"`
				ExportSnapshotMechanism   string          `json:"export_snapshot_mechanism,omitempty"`
				Phase                     string          `json:"phase,omitempty"`
				TotalExportedEvents       int64           `json:"total_exported_events,omitempty"`
				EventsExportRate          int64           `json:"events_export_rate_3m,omitempty"`
				LiveWorkflowType          string          `json:"live_workflow_type,omitempty"`
				Error                     string          `json:"error"`
				ControlPlaneType          string          `json:"control_plane_type"`
				AllowOracleClobDataExport bool            `json:"allow_oracle_clob_data_export"`
				CutoverTimings            *CutoverTimings `json:"cutover_timings,omitempty"`
			}{},
		},
		{
			name:       "Validate ImportSchemaPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ImportSchemaPhasePayload{}),
			expectedType: struct {
				ContinueOnError    bool   `json:"continue_on_error"`
				EnableOrafce       bool   `json:"enable_orafce"`
				IgnoreExist        bool   `json:"ignore_exist"`
				RefreshMviews      bool   `json:"refresh_mviews"`
				ErrorCount         int    `json:"errors"`
				PostSnapshotImport bool   `json:"post_snapshot_import"`
				StartClean         bool   `json:"start_clean"`
				Error              string `json:"error"`
				ControlPlaneType   string `json:"control_plane_type"`
			}{},
		},
		{
			name:       "Validate ImportDataPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ImportDataPhasePayload{}),
			expectedType: struct {
				PayloadVersion              string            `json:"payload_version"`
				BatchSize                   int64             `json:"batch_size"`
				ParallelJobs                int64             `json:"parallel_jobs"`
				OnPrimaryKeyConflictAction  string            `json:"on_primary_key_conflict_action"`
				EnableYBAdaptiveParallelism bool              `json:"enable_yb_adaptive_parallelism"`
				AdaptiveParallelismMax      int64             `json:"adaptive_parallelism_max"`
				ErrorPolicySnapshot         string            `json:"error_policy_snapshot"`
				StartClean                  bool              `json:"start_clean"`
				YBClusterMetrics            YBClusterMetrics  `json:"yb_cluster_metrics"`
				DataMetrics                 ImportDataMetrics `json:"data_metrics"`
				Phase                       string            `json:"phase,omitempty"`
				LiveWorkflowType            string            `json:"live_workflow_type,omitempty"`
				EnableUpsert                bool              `json:"enable_upsert"`
				Error                       string            `json:"error"`
				ControlPlaneType            string            `json:"control_plane_type"`
				CutoverTimings              *CutoverTimings   `json:"cutover_timings,omitempty"`
			}{},
		},
		{
			name:       "Validate CutoverTimings Struct Definition",
			actualType: reflect.TypeOf(CutoverTimings{}),
			expectedType: struct {
				TotalCutoverTimeSec int64  `json:"total_cutover_time_sec"`
				CutoverType         string `json:"cutover_type"`
			}{},
		},
		{
			name:       "Validate YBClusterMetrics Struct Definition",
			actualType: reflect.TypeOf(YBClusterMetrics{}),
			expectedType: struct {
				Timestamp time.Time    `json:"timestamp"`
				AvgCpuPct float64      `json:"avg_cpu_pct"`
				MaxCpuPct float64      `json:"max_cpu_pct"`
				Nodes     []NodeMetric `json:"nodes"`
			}{},
		},
		{
			name:       "Validate NodeMetric Struct Definition",
			actualType: reflect.TypeOf(NodeMetric{}),
			expectedType: struct {
				UUID                   string  `json:"uuid"`
				TotalCPUPct            float64 `json:"total_cpu_pct"`
				TserverMemSoftLimitPct float64 `json:"tserver_mem_soft_limit_pct"`
				MemoryFree             int64   `json:"memory_free"`
				MemoryAvailable        int64   `json:"memory_available"`
				MemoryTotal            int64   `json:"memory_total"`
				Status                 string  `json:"status"`
				Error                  string  `json:"error"`
			}{},
		},
		{
			name:       "Validate ImportDataFilePhasePayload Struct Definition",
			actualType: reflect.TypeOf(ImportDataFilePhasePayload{}),
			expectedType: struct {
				ParallelJobs       int64                 `json:"parallel_jobs"`
				FileStorageType    string                `json:"file_storage_type"`
				StartClean         bool                  `json:"start_clean"`
				DataFileParameters string                `json:"data_file_parameters"`
				DataMetrics        ImportDataFileMetrics `json:"data_metrics"`
				Error              string                `json:"error"`
				ControlPlaneType   string                `json:"control_plane_type"`
			}{},
		},
		{
			name:       "Validate DataFileParameters Struct Definition",
			actualType: reflect.TypeOf(DataFileParameters{}),
			expectedType: struct {
				FileFormat string `json:"FileFormat"`
				Delimiter  string `json:"Delimiter"`
				HasHeader  bool   `json:"HasHeader"`
				QuoteChar  string `json:"QuoteChar,omitempty"`
				EscapeChar string `json:"EscapeChar,omitempty"`
				NullString string `json:"NullString,omitempty"`
			}{},
		},
		{
			name:       "Validate ImportDataMetrics Struct Definition",
			actualType: reflect.TypeOf(ImportDataMetrics{}),
			expectedType: struct {
				CurrentParallelConnections        int   `json:"current_parallel_connections"`
				MigrationSnapshotTotalRows        int64 `json:"migration_snapshot_total_rows"`
				MigrationSnapshotLargestTableRows int64 `json:"migration_snapshot_largest_table_rows"`
				MigrationCdcTotalImportedEvents   int64 `json:"migration_cdc_total_imported_events"`
				SnapshotTotalRows                 int64 `json:"snapshot_total_rows"`
				SnapshotTotalBytes                int64 `json:"snapshot_total_bytes"`
				CdcEventsImportRate3min           int64 `json:"cdc_events_import_rate_3min"`
			}{},
		},
		{
			name:       "Validate ImportDataFileMetrics Struct Definition",
			actualType: reflect.TypeOf(ImportDataFileMetrics{}),
			expectedType: struct {
				CurrentParallelConnections         int   `json:"current_parallel_connections"`
				MigrationSnapshotTotalBytes        int64 `json:"migration_snapshot_total_bytes"`
				MigrationSnapshotLargestTableBytes int64 `json:"migration_snapshot_largest_table_bytes"`
				SnapshotTotalRows                  int64 `json:"snapshot_total_rows"`
				SnapshotTotalBytes                 int64 `json:"snapshot_total_bytes"`
			}{},
		},
		{
			name:       "Validate EndMigrationPhasePayload Struct Definition",
			actualType: reflect.TypeOf(EndMigrationPhasePayload{}),
			expectedType: struct {
				BackupDataFiles      bool   `json:"backup_data_files"`
				BackupLogFiles       bool   `json:"backup_log_files"`
				BackupSchemaFiles    bool   `json:"backup_schema_files"`
				SaveMigrationReports bool   `json:"save_migration_reports"`
				Error                string `json:"error"`
				ControlPlaneType     string `json:"control_plane_type"`
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CompareStructs(t, tt.actualType, reflect.TypeOf(tt.expectedType), tt.name)
		})
	}
}
