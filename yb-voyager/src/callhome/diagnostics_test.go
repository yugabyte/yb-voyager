package callhome

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestCallhomeStructs(t *testing.T) {
	// Define the expected structure for Payload
	expectedPayload := struct {
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
	}{}

	t.Run("Validate Payload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(Payload{}), reflect.TypeOf(expectedPayload), "Payload")
	})

	// Define the expected structure for SourceDBDetails
	expectedSourceDBDetails := struct {
		Host      string `json:"host"`
		DBType    string `json:"db_type"`
		DBVersion string `json:"db_version"`
		DBSize    int64  `json:"total_db_size_bytes"`
		Role      string `json:"role,omitempty"`
	}{}

	t.Run("Validate SourceDBDetails Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(SourceDBDetails{}), reflect.TypeOf(expectedSourceDBDetails), "SourceDBDetails")
	})

	// Define the expected structure for TargetDBDetails
	expectedTargetDBDetails := struct {
		Host      string `json:"host"`
		DBVersion string `json:"db_version"`
		NodeCount int    `json:"node_count"`
		Cores     int    `json:"total_cores"`
	}{}

	t.Run("Validate TargetDBDetails Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(TargetDBDetails{}), reflect.TypeOf(expectedTargetDBDetails), "TargetDBDetails")
	})

	// Define the expected structure for UnsupportedFeature
	expectedUnsupportedFeature := struct {
		FeatureName      string   `json:"FeatureName"`
		Objects          []string `json:"Objects,omitempty"`
		ObjectCount      int      `json:"ObjectCount"`
		TotalOccurrences int      `json:"TotalOccurrences"`
	}{}

	t.Run("Validate UnsupportedFeature Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(UnsupportedFeature{}), reflect.TypeOf(expectedUnsupportedFeature), "UnsupportedFeature")
	})

	// Define the expected structure for AssessMigrationPhasePayload
	expectedAssessMigrationPhasePayload := struct {
		MigrationComplexity        string `json:"migration_complexity"`
		UnsupportedFeatures        string `json:"unsupported_features"`
		UnsupportedDatatypes       string `json:"unsupported_datatypes"`
		UnsupportedQueryConstructs string `json:"unsupported_query_constructs"`
		MigrationCaveats           string `json:"migration_caveats"`
		UnsupportedPlPgSqlObjects  string `json:"unsupported_plpgsql_objects"`
		Error                      string `json:"error,omitempty"`
		TableSizingStats           string `json:"table_sizing_stats"`
		IndexSizingStats           string `json:"index_sizing_stats"`
		SchemaSummary              string `json:"schema_summary"`
		SourceConnectivity         bool   `json:"source_connectivity"`
		IopsInterval               int64  `json:"iops_interval"`
	}{}

	t.Run("Validate AssessMigrationPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(AssessMigrationPhasePayload{}), reflect.TypeOf(expectedAssessMigrationPhasePayload), "AssessMigrationPhasePayload")
	})

	// Define the expected structure for AssessMigrationBulkPhasePayload
	expectedAssessMigrationBulkPhasePayload := struct {
		FleetConfigCount int `json:"fleet_config_count"`
	}{}

	t.Run("Validate AssessMigrationBulkPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(AssessMigrationBulkPhasePayload{}), reflect.TypeOf(expectedAssessMigrationBulkPhasePayload), "AssessMigrationBulkPhasePayload")
	})

	// Define the expected structure for ObjectSizingStats
	expectedObjectSizingStats := struct {
		SchemaName      string `json:"schema_name,omitempty"`
		ObjectName      string `json:"object_name"`
		ReadsPerSecond  int64  `json:"reads_per_second"`
		WritesPerSecond int64  `json:"writes_per_second"`
		SizeInBytes     int64  `json:"size_in_bytes"`
	}{}

	t.Run("Validate ObjectSizingStats Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ObjectSizingStats{}), reflect.TypeOf(expectedObjectSizingStats), "ObjectSizingStats")
	})

	// Define the expected structure for ExportSchemaPhasePayload
	expectedExportSchemaPhasePayload := struct {
		StartClean             bool `json:"start_clean"`
		AppliedRecommendations bool `json:"applied_recommendations"`
		UseOrafce              bool `json:"use_orafce"`
		CommentsOnObjects      bool `json:"comments_on_objects"`
	}{}

	t.Run("Validate ExportSchemaPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ExportSchemaPhasePayload{}), reflect.TypeOf(expectedExportSchemaPhasePayload), "ExportSchemaPhasePayload")
	})

	// Define the expected structure for AnalyzePhasePayload
	expectedAnalyzePhasePayload := struct {
		Issues          string `json:"issues"`
		DatabaseObjects string `json:"database_objects"`
	}{}

	t.Run("Validate AnalyzePhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(AnalyzePhasePayload{}), reflect.TypeOf(expectedAnalyzePhasePayload), "AnalyzePhasePayload")
	})

	// Define the expected structure for ExportDataPhasePayload
	expectedExportDataPhasePayload := struct {
		ParallelJobs            int64  `json:"parallel_jobs"`
		TotalRows               int64  `json:"total_rows_exported"`
		LargestTableRows        int64  `json:"largest_table_rows_exported"`
		StartClean              bool   `json:"start_clean"`
		ExportSnapshotMechanism string `json:"export_snapshot_mechanism,omitempty"`
		Phase                   string `json:"phase,omitempty"`
		TotalExportedEvents     int64  `json:"total_exported_events,omitempty"`
		EventsExportRate        int64  `json:"events_export_rate_3m,omitempty"`
		LiveWorkflowType        string `json:"live_workflow_type,omitempty"`
	}{}

	t.Run("Validate ExportDataPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ExportDataPhasePayload{}), reflect.TypeOf(expectedExportDataPhasePayload), "ExportDataPhasePayload")
	})

	// Define the expected structure for ImportSchemaPhasePayload
	expectedImportSchemaPhasePayload := struct {
		ContinueOnError    bool `json:"continue_on_error"`
		EnableOrafce       bool `json:"enable_orafce"`
		IgnoreExist        bool `json:"ignore_exist"`
		RefreshMviews      bool `json:"refresh_mviews"`
		ErrorCount         int  `json:"errors"`
		PostSnapshotImport bool `json:"post_snapshot_import"`
		StartClean         bool `json:"start_clean"`
	}{}

	t.Run("Validate ImportSchemaPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ImportSchemaPhasePayload{}), reflect.TypeOf(expectedImportSchemaPhasePayload), "ImportSchemaPhasePayload")
	})

	// Define the expected structure for ImportDataPhasePayload
	expectedImportDataPhasePayload := struct {
		ParallelJobs        int64  `json:"parallel_jobs"`
		TotalRows           int64  `json:"total_rows_imported"`
		LargestTableRows    int64  `json:"largest_table_rows_imported"`
		StartClean          bool   `json:"start_clean"`
		Phase               string `json:"phase,omitempty"`
		TotalImportedEvents int64  `json:"total_imported_events,omitempty"`
		EventsImportRate    int64  `json:"events_import_rate_3m,omitempty"`
		LiveWorkflowType    string `json:"live_workflow_type,omitempty"`
		EnableUpsert        bool   `json:"enable_upsert"`
	}{}

	t.Run("Validate ImportDataPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ImportDataPhasePayload{}), reflect.TypeOf(expectedImportDataPhasePayload), "ImportDataPhasePayload")
	})

	// Define the expected structure for ImportDataFilePhasePayload
	expectedImportDataFilePhasePayload := struct {
		ParallelJobs       int64  `json:"parallel_jobs"`
		TotalSize          int64  `json:"total_size_imported"`
		LargestTableSize   int64  `json:"largest_table_size_imported"`
		FileStorageType    string `json:"file_storage_type"`
		StartClean         bool   `json:"start_clean"`
		DataFileParameters string `json:"data_file_parameters"`
	}{}

	t.Run("Validate ImportDataFilePhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(ImportDataFilePhasePayload{}), reflect.TypeOf(expectedImportDataFilePhasePayload), "ImportDataFilePhasePayload")
	})

	// Define the expected structure for DataFileParameters
	expectedDataFileParameters := struct {
		FileFormat string `json:"FileFormat"`
		Delimiter  string `json:"Delimiter"`
		HasHeader  bool   `json:"HasHeader"`
		QuoteChar  string `json:"QuoteChar,omitempty"`
		EscapeChar string `json:"EscapeChar,omitempty"`
		NullString string `json:"NullString,omitempty"`
	}{}

	t.Run("Validate DataFileParameters Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(DataFileParameters{}), reflect.TypeOf(expectedDataFileParameters), "DataFileParameters")
	})

	// Define the expected structure for EndMigrationPhasePayload
	expectedEndMigrationPhasePayload := struct {
		BackupDataFiles      bool `json:"backup_data_files"`
		BackupLogFiles       bool `json:"backup_log_files"`
		BackupSchemaFiles    bool `json:"backup_schema_files"`
		SaveMigrationReports bool `json:"save_migration_reports"`
	}{}

	t.Run("Validate EndMigrationPhasePayload Struct Definition", func(t *testing.T) {
		utils.CompareStructs(t, reflect.TypeOf(EndMigrationPhasePayload{}), reflect.TypeOf(expectedEndMigrationPhasePayload), "EndMigrationPhasePayload")
	})
}
