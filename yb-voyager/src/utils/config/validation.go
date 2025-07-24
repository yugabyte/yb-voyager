package config

import (
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/viper"
)

// ConfigValidationError holds all the invalid configurations detected
type ConfigValidationError struct {
	InvalidGlobalKeys   mapset.Set[string]
	InvalidSectionKeys  map[string]mapset.Set[string]
	InvalidSections     mapset.Set[string]
	ConflictingSections [][]string
}

// Error implements the error interface for ValidationError
func (e *ConfigValidationError) Error() string {
	var sb strings.Builder

	sb.WriteString("\nConfig file validation failed:\n")

	if e.InvalidGlobalKeys.Cardinality() > 0 {
		sb.WriteString(fmt.Sprintf("Invalid global config keys: [%s]\n", strings.Join(e.InvalidGlobalKeys.ToSlice(), ", ")))
	}

	for section, keys := range e.InvalidSectionKeys {
		sb.WriteString(fmt.Sprintf("Invalid keys in section '%s': [%s]\n", section, strings.Join(keys.ToSlice(), ", ")))
	}

	if e.InvalidSections.Cardinality() > 0 {
		sb.WriteString(fmt.Sprintf("Invalid sections: [%s]\n", strings.Join(e.InvalidSections.ToSlice(), ", ")))
	}

	for _, conflict := range e.ConflictingSections {
		sb.WriteString(fmt.Sprintf("Only one of the following sections can be used: [%s]\n", strings.Join(conflict, ", ")))
	}

	return sb.String()
}

// Allowed global config keys
var AllowedGlobalConfigKeys = mapset.NewThreadUnsafeSet[string](
	"export-dir", "log-level", "send-diagnostics",
	"profile",
	// environment variables keys
	"control-plane-type", "yugabyted-db-conn-string", "java-home",
	"local-call-home-service-host", "local-call-home-service-port",
	"yb-tserver-port", "tns-admin",
)

// Allowed source config keys
var allowedSourceConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-type", "db-host", "db-port", "db-user", "db-name", "db-password",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert",
	"ssl-crl", "oracle-db-sid", "oracle-home", "oracle-tns-alias", "oracle-cdb-name",
	"oracle-cdb-sid", "oracle-cdb-tns-alias",
)

// Allowed source replica config keys
var allowedSourceReplicaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-host", "db-port", "db-user", "db-name", "db-password",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert",
	"ssl-crl", "db-sid",
	"oracle-home", "oracle-tns-alias",
)

// Allowed target config keys
var allowedTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-host", "db-port", "db-user", "db-password", "db-name",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert", "ssl-crl",
)

// Allowed assess migration config keys
var allowedAssessMigrationConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"iops-capture-interval", "target-db-version", "assessment-metadata-dir",
	"invoked-by-export-schema",
	// environment variables keys
	"report-unsupported-query-constructs", "report-unsupported-plpgsql-objects",
)

// Allowed analyze schema config keys
var allowedAnalyzeSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"output-format", "target-db-version",
	// environment variables keys
	"report-unsupported-plpgsql-objects",
)

// Allowed export schema config keys
var allowedExportSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"use-orafce", "comments-on-objects", "object-type-list", "exclude-object-type-list",
	"skip-recommendations", "assessment-report-path",
	"assess-schema-before-export",
	// environment variables keys
	"ybvoyager-skip-merge-constraints-transformation",
)

// Allowed export data config keys
var allowedExportDataConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"disable-pb", "exclude-table-list", "table-list", "exclude-table-list-file-path",
	"table-list-file-path", "parallel-jobs", "export-type",
	// environment variables keys
	"queue-segment-max-bytes", "debezium-dist-dir", "beta-fast-data-export",
)

// Allowed export data from target config keys
var allowedExportDataFromTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"disable-pb", "exclude-table-list", "table-list", "exclude-table-list-file-path",
	"table-list-file-path", "transaction-ordering",
	// environment variables keys
	"yb-master-port", "queue-segment-max-bytes", "debezium-dist-dir",
)

// Allowed import schema config keys
var allowedImportSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"continue-on-error", "object-type-list", "exclude-object-type-list", "straight-order",
	"ignore-exist", "enable-orafce",
)

// Allowed finalize schema post data import config keys
var allowedFinalizeSchemaPostDataImportConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"continue-on-error", "ignore-exist", "refresh-mviews",
)

// Allowed import data config keys
var allowedImportDataConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"batch-size", "parallel-jobs", "enable-adaptive-parallelism", "adaptive-parallelism-max",
	"skip-replication-checks",
	"disable-pb", "max-retries", "exclude-table-list", "table-list",
	"exclude-table-list-file-path", "table-list-file-path", "enable-upsert", "use-public-ip",
	"target-endpoints", "truncate-tables", "error-policy-snapshot",
	"skip-node-health-checks", "skip-disk-usage-health-checks",
	"on-primary-key-conflict", "disable-transactional-writes",
	"truncate-splits",

	// environment variables keys
	"csv-reader-max-buffer-size-bytes", "ybvoyager-max-colocated-batches-in-progress", "num-event-channels", "event-channel-size",
	"max-events-per-batch", "max-interval-between-batches", "max-cpu-threshold",
	"adaptive-parallelism-frequency-seconds", "min-available-memory-threshold", "max-batch-size-bytes",
	"ybvoyager-use-task-picker-for-import",
)

// Allowed import data to source config keys
var allowedImportDataToSourceConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"parallel-jobs", "disable-pb",
	// environment variables keys
	"num-event-channels", "event-channel-size", "max-events-per-batch",
	"max-interval-between-batches", "max-batch-size-bytes",
)

// Allowed import data to source replica config keys
var allowedImportDataToSourceReplicaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"batch-size", "parallel-jobs", "truncate-tables", "disable-pb", "max-retries",
	// environment variables keys
	"ybvoyager-max-colocated-batches-in-progress", "num-event-channels",
	"event-channel-size", "max-events-per-batch", "max-interval-between-batches",
	"max-batch-size-bytes",
)

// Allowed import data file config keys
var allowedImportDataFileConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"disable-pb", "max-retries", "enable-upsert", "use-public-ip", "target-endpoints",
	"batch-size", "parallel-jobs", "enable-adaptive-parallelism", "adaptive-parallelism-max",
	"format", "delimiter", "data-dir", "file-table-map", "has-header", "escape-char",
	"quote-char", "file-opts", "null-string", "truncate-tables", "error-policy",
	"disable-transactional-writes", "truncate-splits", "skip-replication-checks",
	"skip-node-health-checks", "skip-disk-usage-health-checks", "on-primary-key-conflict",
	// environment variables keys
	"csv-reader-max-buffer-size-bytes", "ybvoyager-max-colocated-batches-in-progress",
	"max-cpu-threshold", "adaptive-parallelism-frequency-seconds",
	"min-available-memory-threshold", "max-batch-size-bytes", "ybvoyager-use-task-picker-for-import",
)

// Allowed initiate cutover to target config keys
var allowedInitCutoverToTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"prepare-for-fall-back", "use-yb-grpc-connector",
)

// Allowed archive changes config keys
var allowedArchiveChangesConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"delete-changes-without-archiving", "fs-utilization-threshold", "move-to",
)

// Allowed end migration config keys
var allowedEndMigrationConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"backup-schema-files", "backup-data-files", "save-migration-reports", "backup-log-files",
	"backup-dir",
)

// Define allowed nested sections
var AllowedConfigSections = map[string]mapset.Set[string]{
	"source":                           allowedSourceConfigKeys,
	"source-replica":                   allowedSourceReplicaConfigKeys,
	"target":                           allowedTargetConfigKeys,
	"assess-migration":                 allowedAssessMigrationConfigKeys,
	"analyze-schema":                   allowedAnalyzeSchemaConfigKeys,
	"export-schema":                    allowedExportSchemaConfigKeys,
	"export-data":                      allowedExportDataConfigKeys,
	"export-data-from-source":          allowedExportDataConfigKeys,
	"export-data-from-target":          allowedExportDataFromTargetConfigKeys,
	"import-schema":                    allowedImportSchemaConfigKeys,
	"finalize-schema-post-data-import": allowedFinalizeSchemaPostDataImportConfigKeys,
	"import-data":                      allowedImportDataConfigKeys,
	"import-data-to-target":            allowedImportDataConfigKeys,
	"import-data-to-source":            allowedImportDataToSourceConfigKeys,
	"import-data-to-source-replica":    allowedImportDataToSourceReplicaConfigKeys,
	"import-data-file":                 allowedImportDataFileConfigKeys,
	"initiate-cutover-to-target":       allowedInitCutoverToTargetConfigKeys,
	"archive-changes":                  allowedArchiveChangesConfigKeys,
	"end-migration":                    allowedEndMigrationConfigKeys,
}

// Define mutually exclusive section groups
var AliasCommandsPrefixes = [][]string{
	{"export-data", "export-data-from-source"},
	{"import-data", "import-data-to-target"},
}

// ValidateConfigFile validates the config file using comprehensive YB Voyager validation rules
func ValidateConfigFile(v *viper.Viper) error {
	invalidGlobalKeys := mapset.NewThreadUnsafeSet[string]()
	invalidSectionKeys := make(map[string]mapset.Set[string])
	invalidSections := mapset.NewThreadUnsafeSet[string]()
	conflictingSections := [][]string{}
	presentSections := mapset.NewThreadUnsafeSet[string]()

	for _, key := range v.AllKeys() {
		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			// Check global level keys
			if !AllowedGlobalConfigKeys.Contains(key) {
				invalidGlobalKeys.Add(key)
			}
		} else {
			// Validate section-based keys
			// The section is the first part of the key, the rest of the parts combined using "." are the nested key
			// For example: "a.b.c" -> section: "a", nestedKey: "b.c"
			section := parts[0]
			nestedKey := strings.Join(parts[1:], ".")
			presentSections.Add(section)

			allowedKeys, ok := AllowedConfigSections[section]
			if !ok {
				// Unknown section
				invalidSections.Add(section)
				continue
			}

			if !allowedKeys.Contains(nestedKey) {
				// Invalid key inside a known section
				if _, exists := invalidSectionKeys[section]; !exists {
					invalidSectionKeys[section] = mapset.NewThreadUnsafeSet[string]()
				}
				invalidSectionKeys[section].Add(nestedKey)
			}
		}
	}

	// Check for mutually exclusive section usage
	for _, group := range AliasCommandsPrefixes {
		var used []string
		for _, sec := range group {
			if presentSections.Contains(sec) {
				used = append(used, sec)
			}
		}
		if len(used) > 1 {
			conflictingSections = append(conflictingSections, used)
		}
	}

	// If invalid configurations exist, return a ValidationError
	if invalidGlobalKeys.Cardinality() > 0 || len(invalidSectionKeys) > 0 || invalidSections.Cardinality() > 0 || len(conflictingSections) > 0 {
		return &ConfigValidationError{
			InvalidGlobalKeys:   invalidGlobalKeys,
			InvalidSectionKeys:  invalidSectionKeys,
			InvalidSections:     invalidSections,
			ConflictingSections: conflictingSections,
		}
	}

	return nil
}
