package callhome

import (
	"reflect"
	"testing"

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
				Host      string `json:"host"`
				DBType    string `json:"db_type"`
				DBVersion string `json:"db_version"`
				DBSize    int64  `json:"total_db_size_bytes"`
				Role      string `json:"role,omitempty"`
			}{},
		},
		{
			name:       "Validate TargetDBDetails Struct Definition",
			actualType: reflect.TypeOf(TargetDBDetails{}),
			expectedType: struct {
				Host      string `json:"host"`
				DBVersion string `json:"db_version"`
				NodeCount int    `json:"node_count"`
				Cores     int    `json:"total_cores"`
			}{},
		},
		{
			name:       "Validate UnsupportedFeature Struct Definition",
			actualType: reflect.TypeOf(UnsupportedFeature{}),
			expectedType: struct {
				FeatureName      string   `json:"FeatureName"`
				Objects          []string `json:"Objects,omitempty"`
				ObjectCount      int      `json:"ObjectCount"`
				TotalOccurrences int      `json:"TotalOccurrences"`
			}{},
		},
		{
			name:       "Validate AssessMigrationPhasePayload Struct Definition",
			actualType: reflect.TypeOf(AssessMigrationPhasePayload{}),
			expectedType: struct {
				TargetDBVersion            *ybversion.YBVersion `json:"target_db_version"`
				MigrationComplexity        string               `json:"migration_complexity"`
				UnsupportedFeatures        string               `json:"unsupported_features"`
				UnsupportedDatatypes       string               `json:"unsupported_datatypes"`
				UnsupportedQueryConstructs string               `json:"unsupported_query_constructs"`
				MigrationCaveats           string               `json:"migration_caveats"`
				UnsupportedPlPgSqlObjects  string               `json:"unsupported_plpgsql_objects"`
				Error                      string               `json:"error,omitempty"`
				TableSizingStats           string               `json:"table_sizing_stats"`
				IndexSizingStats           string               `json:"index_sizing_stats"`
				SchemaSummary              string               `json:"schema_summary"`
				SourceConnectivity         bool                 `json:"source_connectivity"`
				IopsInterval               int64                `json:"iops_interval"`
			}{},
		},
		{
			name:       "Validate AssessMigrationBulkPhasePayload Struct Definition",
			actualType: reflect.TypeOf(AssessMigrationBulkPhasePayload{}),
			expectedType: struct {
				FleetConfigCount int `json:"fleet_config_count"`
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
				StartClean             bool `json:"start_clean"`
				AppliedRecommendations bool `json:"applied_recommendations"`
				UseOrafce              bool `json:"use_orafce"`
				CommentsOnObjects      bool `json:"comments_on_objects"`
			}{},
		},
		{
			name:       "Validate AnalyzePhasePayload Struct Definition",
			actualType: reflect.TypeOf(AnalyzePhasePayload{}),
			expectedType: struct {
				TargetDBVersion *ybversion.YBVersion `json:"target_db_version"`
				Issues          string               `json:"issues"`
				DatabaseObjects string               `json:"database_objects"`
			}{},
		},
		{
			name:       "Validate ExportDataPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ExportDataPhasePayload{}),
			expectedType: struct {
				ParallelJobs            int64  `json:"parallel_jobs"`
				TotalRows               int64  `json:"total_rows_exported"`
				LargestTableRows        int64  `json:"largest_table_rows_exported"`
				StartClean              bool   `json:"start_clean"`
				ExportSnapshotMechanism string `json:"export_snapshot_mechanism,omitempty"`
				Phase                   string `json:"phase,omitempty"`
				TotalExportedEvents     int64  `json:"total_exported_events,omitempty"`
				EventsExportRate        int64  `json:"events_export_rate_3m,omitempty"`
				LiveWorkflowType        string `json:"live_workflow_type,omitempty"`
			}{},
		},
		{
			name:       "Validate ImportSchemaPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ImportSchemaPhasePayload{}),
			expectedType: struct {
				ContinueOnError    bool `json:"continue_on_error"`
				EnableOrafce       bool `json:"enable_orafce"`
				IgnoreExist        bool `json:"ignore_exist"`
				RefreshMviews      bool `json:"refresh_mviews"`
				ErrorCount         int  `json:"errors"`
				PostSnapshotImport bool `json:"post_snapshot_import"`
				StartClean         bool `json:"start_clean"`
			}{},
		},
		{
			name:       "Validate ImportDataPhasePayload Struct Definition",
			actualType: reflect.TypeOf(ImportDataPhasePayload{}),
			expectedType: struct {
				ParallelJobs        int64  `json:"parallel_jobs"`
				TotalRows           int64  `json:"total_rows_imported"`
				LargestTableRows    int64  `json:"largest_table_rows_imported"`
				StartClean          bool   `json:"start_clean"`
				Phase               string `json:"phase,omitempty"`
				TotalImportedEvents int64  `json:"total_imported_events,omitempty"`
				EventsImportRate    int64  `json:"events_import_rate_3m,omitempty"`
				LiveWorkflowType    string `json:"live_workflow_type,omitempty"`
				EnableUpsert        bool   `json:"enable_upsert"`
			}{},
		},
		{
			name:       "Validate ImportDataFilePhasePayload Struct Definition",
			actualType: reflect.TypeOf(ImportDataFilePhasePayload{}),
			expectedType: struct {
				ParallelJobs       int64  `json:"parallel_jobs"`
				TotalSize          int64  `json:"total_size_imported"`
				LargestTableSize   int64  `json:"largest_table_size_imported"`
				FileStorageType    string `json:"file_storage_type"`
				StartClean         bool   `json:"start_clean"`
				DataFileParameters string `json:"data_file_parameters"`
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
			name:       "Validate EndMigrationPhasePayload Struct Definition",
			actualType: reflect.TypeOf(EndMigrationPhasePayload{}),
			expectedType: struct {
				BackupDataFiles      bool `json:"backup_data_files"`
				BackupLogFiles       bool `json:"backup_log_files"`
				BackupSchemaFiles    bool `json:"backup_schema_files"`
				SaveMigrationReports bool `json:"save_migration_reports"`
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CompareStructs(t, tt.actualType, reflect.TypeOf(tt.expectedType), tt.name)
		})
	}
}
