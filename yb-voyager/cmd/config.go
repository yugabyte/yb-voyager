package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	// Flag name prefixes (used in CLI flags)
	SourceDBFlagPrefix        = "source-"
	TargetDBFlagPrefix        = "target-"
	OracleDBFlagPrefix        = "oracle-"
	SourceReplicaDBFlagPrefix = "source-replica-"

	// Config key prefixes (used in config file keys)
	SourceDBConfigPrefix        = "source."
	TargetDBConfigPrefix        = "target."
	SourceReplicaDBConfigPrefix = "source-replica."
)

var allowedGlobalConfigKeys = mapset.NewThreadUnsafeSet[string](
	"export-dir", "log-level", "send-diagnostics",
	"profile",
	// environment variables keys
	"control-plane-type", "yugabyted-db-conn-string", "java-home",
	"local-call-home-service-host", "local-call-home-service-port",
	"yb-tserver-port", "tns-admin",
)

var allowedSourceConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-type", "db-host", "db-port", "db-user", "db-name", "db-password",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert",
	"ssl-crl", "oracle-db-sid", "oracle-home", "oracle-tns-alias", "oracle-cdb-name",
	"oracle-cdb-sid", "oracle-cdb-tns-alias",
)

var allowedSourceReplicaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-host", "db-port", "db-user", "db-name", "db-password",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert",
	"ssl-crl", "db-sid",
	"oracle-home", "oracle-tns-alias",
)

var allowedTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-host", "db-port", "db-user", "db-password", "db-name",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert", "ssl-crl",
)

var allowedAssessMigrationConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"iops-capture-interval", "target-db-version", "assessment-metadata-dir",
	"invoked-by-export-schema",
	// environment variables keys
	"report-unsupported-query-constructs", "report-unsupported-plpgsql-objects",
)

var allowedAnalyzeSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"output-format", "target-db-version",
	// environment variables keys
	"report-unsupported-plpgsql-objects",
)

var allowedExportSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"use-orafce", "comments-on-objects", "object-type-list", "exclude-object-type-list",
	"skip-recommendations", "assessment-report-path",
	"assess-schema-before-export",
	// environment variables keys
	"ybvoyager-skip-merge-constraints-transformation",
)

var allowedExportDataConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"disable-pb", "exclude-table-list", "table-list", "exclude-table-list-file-path",
	"table-list-file-path", "parallel-jobs", "export-type",
	// environment variables keys
	"queue-segment-max-bytes", "debezium-dist-dir", "beta-fast-data-export",
)

var allowedExportDataFromTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"disable-pb", "exclude-table-list", "table-list", "exclude-table-list-file-path",
	"table-list-file-path", "transaction-ordering",
	// environment variables keys
	"yb-master-port", "queue-segment-max-bytes", "debezium-dist-dir",
)

var allowedImportSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"continue-on-error", "object-type-list", "exclude-object-type-list", "straight-order",
	"ignore-exist", "enable-orafce",
)

var allowedFinalizeSchemaPostDataImportConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"continue-on-error", "ignore-exist", "refresh-mviews",
)

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

var allowedImportDataToSourceConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"parallel-jobs", "disable-pb",
	// environment variables keys
	"num-event-channels", "event-channel-size", "max-events-per-batch",
	"max-interval-between-batches", "max-batch-size-bytes",
)

var allowedImportDataToSourceReplicaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level", "run-guardrails-checks",
	"batch-size", "parallel-jobs", "truncate-tables", "disable-pb", "max-retries",
	// environment variables keys
	"ybvoyager-max-colocated-batches-in-progress", "num-event-channels",
	"event-channel-size", "max-events-per-batch", "max-interval-between-batches",
	"max-batch-size-bytes",
)

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

var allowedInitCutoverToTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"prepare-for-fall-back", "use-yb-grpc-connector",
)

var allowedArchiveChangesConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"delete-changes-without-archiving", "fs-utilization-threshold", "move-to",
)

var allowedEndMigrationConfigKeys = mapset.NewThreadUnsafeSet[string](
	"log-level",
	"backup-schema-files", "backup-data-files", "save-migration-reports", "backup-log-files",
	"backup-dir",
)

// Define allowed nested sections
var allowedConfigSections = map[string]mapset.Set[string]{
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
var aliasCommandsPrefixes = [][]string{
	{"export-data", "export-data-from-source"},
	{"import-data", "import-data-to-target"},
}

// ConfigFlagOverride represents a CLI flag whose value was set from the config file.
// It captures the flag name, the corresponding config key that supplied the value,
// and the final value that was applied. This is useful for logging and debugging
// which flags were influenced by configuration during command execution.
type ConfigFlagOverride struct {
	FlagName  string
	ConfigKey string
	Value     string
}

type EnvVarSetViaConfig struct {
	EnvVar    string
	ConfigKey string
	Value     string
}

/*
initConfig initializes the configuration for the given Cobra command.

	It performs the following steps:
	 1. Creates a new Viper instance to isolate config handling for the command.
	 2. Loads the config file if explicitly provided via --config, or defaults to ~/.yb-voyager.yaml.
	 3. Reads and validates the configuration file for allowed global keys, sections, and section keys.
	 4. Binds Viper config values to Cobra flags, giving priority to command-line flags over config values.
	 5. Returns a slice of ConfigFlagOverride structs, which represent the flags that were set from the config file.
	 6. If any error occurs during the process, it returns the error.

	This setup ensures CLI > Config precedence
*/
func initConfig(cmd *cobra.Command) ([]ConfigFlagOverride, []EnvVarSetViaConfig, map[string]string, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if cfgFile != "" {
		// Use config file from the flag.
		if !utils.FileOrFolderExists(cfgFile) {
			return nil, nil, nil, fmt.Errorf("config file does not exist: %s", cfgFile)
		}

		cfgFile, err := filepath.Abs(cfgFile)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get absolute path for config file: %s: %w", cfgFile, err)
		}
		cfgFile = filepath.Clean(cfgFile)

		v.SetConfigFile(cfgFile)
	}

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err == nil {
		cfgFile = v.ConfigFileUsed()
		fmt.Println("Using config file:", color.BlueString(v.ConfigFileUsed()))
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, nil, nil, err
		}
	}

	// Validate the config file for allowed keys and sections
	err := validateConfigFile(v)
	if err != nil {
		return nil, nil, nil, err
	}

	// Bind the config values to the Cobra command flags
	overrides, err := bindCobraFlagsToViper(cmd, v)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to bind cobra flags to viper: %w", err)
	}

	envVarsSetViaConfig, envVarsAlreadyExported, err := bindEnvVarsToViper(cmd, v)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to bind environment variables to viper: %w", err)
	}

	return overrides, envVarsSetViaConfig, envVarsAlreadyExported, nil
}

// map of string environment variable names to their config keys
var confParamEnvVarPairs = map[string]string{
	"control-plane-type":           "CONTROL_PLANE_TYPE",
	"yugabyted-db-conn-string":     "YUGABYTED_DB_CONN_STRING",
	"java-home":                    "JAVA_HOME",
	"local-call-home-service-host": "LOCAL_CALL_HOME_SERVICE_HOST",
	"local-call-home-service-port": "LOCAL_CALL_HOME_SERVICE_PORT",
	"yb-tserver-port":              "YB_TSERVER_PORT",
	"tns-admin":                    "TNS_ADMIN",
	"send-diagnostics":             "YB_VOYAGER_SEND_DIAGNOSTICS",

	"source.db-password": "SOURCE_DB_PASSWORD",

	"target.db-password": "TARGET_DB_PASSWORD",

	"source-replica.db-password": "SOURCE_REPLICA_DB_PASSWORD",

	"assess-migration.report-unsupported-query-constructs": "REPORT_UNSUPPORTED_QUERY_CONSTRUCTS",
	"assess-migration.report-unsupported-plpgsql-objects":  "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS",

	"analyze-schema.report-unsupported-plpgsql-objects": "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS",

	"export-schema.ybvoyager-skip-merge-constraints-transformation": "YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS",

	"export-data.queue-segment-max-bytes": "QUEUE_SEGMENT_MAX_BYTES",
	"export-data.debezium-dist-dir":       "DEBEZIUM_DIST_DIR",
	"export-data.beta-fast-data-export":   "BETA_FAST_DATA_EXPORT",

	"export-data-from-source.queue-segment-max-bytes": "QUEUE_SEGMENT_MAX_BYTES",
	"export-data-from-source.debezium-dist-dir":       "DEBEZIUM_DIST_DIR",
	"export-data-from-source.beta-fast-data-export":   "BETA_FAST_DATA_EXPORT",

	"export-data-from-target.yb-master-port":          "YB_MASTER_PORT",
	"export-data-from-target.queue-segment-max-bytes": "QUEUE_SEGMENT_MAX_BYTES",
	"export-data-from-target.debezium-dist-dir":       "DEBEZIUM_DIST_DIR",

	"import-data-file.csv-reader-max-buffer-size-bytes":            "CSV_READER_MAX_BUFFER_SIZE_BYTES",
	"import-data-file.ybvoyager-max-colocated-batches-in-progress": "YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS",
	"import-data-file.max-cpu-threshold":                           "MAX_CPU_THRESHOLD",
	"import-data-file.adaptive-parallelism-frequency-seconds":      "ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS",
	"import-data-file.min-available-memory-threshold":              "MIN_AVAILABLE_MEMORY_THRESHOLD",
	"import-data-file.max-batch-size-bytes":                        "MAX_BATCH_SIZE_BYTES",
	"import-data-file.ybvoyager-use-task-picker-for-import":        "YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT",

	"import-data.csv-reader-max-buffer-size-bytes":            "CSV_READER_MAX_BUFFER_SIZE_BYTES",
	"import-data.ybvoyager-max-colocated-batches-in-progress": "YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS",
	"import-data.num-event-channels":                          "NUM_EVENT_CHANNELS",
	"import-data.event-channel-size":                          "EVENT_CHANNEL_SIZE",
	"import-data.max-events-per-batch":                        "MAX_EVENTS_PER_BATCH",
	"import-data.max-interval-between-batches":                "MAX_INTERVAL_BETWEEN_BATCHES",
	"import-data.max-cpu-threshold":                           "MAX_CPU_THRESHOLD",
	"import-data.adaptive-parallelism-frequency-seconds":      "ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS",
	"import-data.min-available-memory-threshold":              "MIN_AVAILABLE_MEMORY_THRESHOLD",
	"import-data.max-batch-size-bytes":                        "MAX_BATCH_SIZE_BYTES",
	"import-data.ybvoyager-use-task-picker-for-import":        "YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT",

	"import-data-to-target.csv-reader-max-buffer-size-bytes":            "CSV_READER_MAX_BUFFER_SIZE_BYTES",
	"import-data-to-target.ybvoyager-max-colocated-batches-in-progress": "YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS",
	"import-data-to-target.num-event-channels":                          "NUM_EVENT_CHANNELS",
	"import-data-to-target.event-channel-size":                          "EVENT_CHANNEL_SIZE",
	"import-data-to-target.max-events-per-batch":                        "MAX_EVENTS_PER_BATCH",
	"import-data-to-target.max-interval-between-batches":                "MAX_INTERVAL_BETWEEN_BATCHES",
	"import-data-to-target.max-cpu-threshold":                           "MAX_CPU_THRESHOLD",
	"import-data-to-target.adaptive-parallelism-frequency-seconds":      "ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS",
	"import-data-to-target.min-available-memory-threshold":              "MIN_AVAILABLE_MEMORY_THRESHOLD",
	"import-data-to-target.max-batch-size-bytes":                        "MAX_BATCH_SIZE_BYTES",
	"import-data-to-target.ybvoyager-use-task-picker-for-import":        "YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT",

	"import-data-to-source-replica.ybvoyager-max-colocated-batches-in-progress": "YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS",
	"import-data-to-source-replica.num-event-channels":                          "NUM_EVENT_CHANNELS",
	"import-data-to-source-replica.event-channel-size":                          "EVENT_CHANNEL_SIZE",
	"import-data-to-source-replica.max-events-per-batch":                        "MAX_EVENTS_PER_BATCH",
	"import-data-to-source-replica.max-interval-between-batches":                "MAX_INTERVAL_BETWEEN_BATCHES",
	"import-data-to-source-replica.max-batch-size-bytes":                        "MAX_BATCH_SIZE_BYTES",

	"import-data-to-source.num-event-channels":           "NUM_EVENT_CHANNELS",
	"import-data-to-source.event-channel-size":           "EVENT_CHANNEL_SIZE",
	"import-data-to-source.max-events-per-batch":         "MAX_EVENTS_PER_BATCH",
	"import-data-to-source.max-interval-between-batches": "MAX_INTERVAL_BETWEEN_BATCHES",
	"import-data-to-source.max-batch-size-bytes":         "MAX_BATCH_SIZE_BYTES",
}

/*
bindEnvVarsToViper sets up environment variables based on the resolved Viper config for a given Cobra command.

It identifies config keys relevant to the command being executed, checks whether they are already exported as
environment variables, and sets them only if not already present. This enables legacy parts of the codebase
that use os.Getenv to still benefit from values defined in the config file.

Resolution precedence:
  - If the environment variable is already set in the shell, it is respected and not overridden.
  - If the environment variable is unset but the config file defines the key, it is set using os.Setenv.
  - If neither is set, the environment variable is not set at all.

Config key matching logic:
  - For command-scoped keys: the config key must be prefixed with <command-path> (e.g., "export-data.table-list").
  - For global keys (like "export-dir"), config keys without a dot are always allowed.
  - Keys scoped under sections like "source.", "source-replica.", or "target." are also accepted globally.

Alias support:
  - If a command like "export data from source" is executed, it will internally map to a config prefix
    like "export-data" or "export-data-from-source", depending on what exists in the config file.
	The config params will be used according to the resolved prefix by this logic.

Returns:
  - A map of environment variables that were set via os.Setenv, useful for diagnostics or logging.

Note:
  - These environment variables are scoped to the Go process only and are not visible to the parent shell.
  - They can be used freely within os.Getenv calls, but cannot be accessed via `echo $VAR` after CLI exits.
*/

func bindEnvVarsToViper(cmd *cobra.Command, v *viper.Viper) ([]EnvVarSetViaConfig, map[string]string, error) {
	subCmdPath := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name())
	subCmdPath = strings.TrimSpace(subCmdPath) // remove leading space if any
	// Replace spaces with hyphens
	configKeyPrefix := strings.ReplaceAll(subCmdPath, " ", "-")
	configKeyPrefix = setToAliasPrefixIfSet(configKeyPrefix, v)

	var envVarsSetViaConfig []EnvVarSetViaConfig
	envVarsAlreadyExported := make(map[string]string)

	// Iterate over known config-to-env-var mappings and set env vars
	// only if:
	// - the config key is relevant to this command (matches configKeyPrefix or known sections like source./target.)
	// - the env var is not already exported in the shell
	// - the config key is explicitly set in the config file (non-empty value)
	// This ensures the correct config values are surfaced to os.Getenv consumers,
	// without overriding environment variables that the user has already exported.
	for confKey, envVar := range confParamEnvVarPairs {
		// Proceed if key is relevant to this command (has correct prefix) or is global
		if strings.HasPrefix(confKey, configKeyPrefix+".") ||
			strings.HasPrefix(confKey, "source.") ||
			strings.HasPrefix(confKey, "source-replica.") ||
			strings.HasPrefix(confKey, "target.") ||
			len(strings.Split(confKey, ".")) == 1 {

			// Skip if env var already exported
			if existingVal, exists := os.LookupEnv(envVar); exists {
				envVarsAlreadyExported[envVar] = existingVal
				continue
			}

			val := v.GetString(confKey)
			// Set the env var only if Viper has a non-empty value
			if val != "" {
				if err := os.Setenv(envVar, val); err != nil {
					return nil, nil, fmt.Errorf("failed to set environment variable %s: %w", envVar, err)
				}
				envVarsSetViaConfig = append(envVarsSetViaConfig, EnvVarSetViaConfig{
					EnvVar:    envVar,
					ConfigKey: confKey,
					Value:     val,
				})
			}
		}
	}

	return envVarsSetViaConfig, envVarsAlreadyExported, nil
}

// ValidationError holds all the invalid configurations detected
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
		sb.WriteString(fmt.Sprintf("%s [%s]\n", color.RedString("Invalid global config keys:"), strings.Join(e.InvalidGlobalKeys.ToSlice(), ", ")))
	}

	for section, keys := range e.InvalidSectionKeys {
		sb.WriteString(fmt.Sprintf("%s [%s]\n", color.RedString(fmt.Sprintf("Invalid keys in section '%s':", section)), strings.Join(keys.ToSlice(), ", ")))
	}

	if e.InvalidSections.Cardinality() > 0 {
		sb.WriteString(fmt.Sprintf("%s [%s]\n", color.RedString("Invalid sections:"), strings.Join(e.InvalidSections.ToSlice(), ", ")))
	}

	for _, conflict := range e.ConflictingSections {
		sb.WriteString(fmt.Sprintf("%s [%s]\n", color.RedString("Only one of the following sections can be used:"), strings.Join(conflict, ", ")))
	}

	return sb.String()
}

/*
validateConfigFile checks the loaded configuration for correctness.

	It performs the following validations:
	1. Ensures that all global-level keys (non-nested) are in the allowed list.
	2. Ensures that all section names (e.g., export-schema) are known and valid.
	3. Ensures that all nested keys inside each section are valid for that section.
	4. Ensures that only one section from each group of mutually exclusive aliases is used (e.g.,
	"export-data" vs "export-data-from-source").

	Any invalid global keys, unknown sections, or invalid section keys are collected and returned
	Conflicts between mutually exclusive sections are also reported.
	If any validation error is found, an error is returned.

	This helps catch misconfigurations early and provides comprehensive feedback to the user.
*/
func validateConfigFile(v *viper.Viper) error {

	invalidGlobalKeys := mapset.NewThreadUnsafeSet[string]()
	invalidSectionKeys := make(map[string]mapset.Set[string])
	invalidSections := mapset.NewThreadUnsafeSet[string]()
	conflictingSections := [][]string{}
	presentSections := mapset.NewThreadUnsafeSet[string]()

	for _, key := range v.AllKeys() {
		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			// Check global level keys
			if !allowedGlobalConfigKeys.Contains(key) {
				invalidGlobalKeys.Add(key)
			}
		} else {
			// Validate section-based keys
			// The section is the first part of the key, the rest of the parts combined using "." are the nested key
			// For example: "a.b.c" -> section: "a", nestedKey: "b.c"
			section := parts[0]
			nestedKey := strings.Join(parts[1:], ".")
			presentSections.Add(section)

			allowedKeys, ok := allowedConfigSections[section]
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
	for _, group := range aliasCommandsPrefixes {
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

/*
bindCobraFlagsToViper binds configuration values from a Viper instance to the flags of a given Cobra command.

	It performs the following actions:
	 1. Derives a config key prefix based on the command path (replacing spaces with hyphens).
	 2. If the prefix matches an alias group (e.g., "export-data-from-source"), it replaces it with the canonical alias prefix.
	 3. For each flag in the command:
	    - If the flag is not already set by the user:
	    - Checks for a matching value in Viper using the full prefixed key.
	    - Falls back to checking the global (non-prefixed) key.
	    - Additionally, handles special prefixed keys for source/target DB configs:
	    - Flags starting with "source-" → looks under "source.<flag-suffix>"
	    - Flags starting with "oracle-" → looks under "source.<oracle-flag-name>"
	    - Flags starting with "target-" → looks under "target.<flag-suffix>"
	 4. If a value is found in Viper, the corresponding flag is set with that value.
	 5. If any error occurs during binding, it stops further processing and returns the error.
	 6. Also returns a slice of ConfigFlagOverride structs, which represent the flags that were set from the config file. Should only be used if there are no errors.

	This function allows users to configure flags through the config file or environment variables,
	while still letting command-line input take precedence.
*/
func bindCobraFlagsToViper(cmd *cobra.Command, v *viper.Viper) ([]ConfigFlagOverride, error) {
	var bindErr error
	var overrides []ConfigFlagOverride

	subCmdPath := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name())
	subCmdPath = strings.TrimSpace(subCmdPath) // remove leading space if any
	// Replace spaces with hyphens
	configKeyPrefix := strings.ReplaceAll(subCmdPath, " ", "-")
	configKeyPrefix = setToAliasPrefixIfSet(configKeyPrefix, v)

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if bindErr != nil || f.Changed {
			return // Skip already-set flags or if an error occurred
		}

		// Check for <command_path>.<flagname>
		var configKey string
		if configKey = configKeyPrefix + "." + f.Name; v.IsSet(configKey) {
			val := v.GetString(configKey)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				// In case of an error while setting the flag from viper, return the error
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKey,
				Value:     val,
			})
		} else if configKey = f.Name; v.IsSet(configKey) {
			// Bind the global flag from viper to cmd
			val := v.GetString(configKey)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKey,
				Value:     val,
			})
		} else if configKey = SourceDBConfigPrefix + strings.TrimPrefix(f.Name, SourceDBFlagPrefix); strings.HasPrefix(f.Name, SourceDBFlagPrefix) && v.IsSet(configKey) {
			// Handle source db type flags
			val := v.GetString(configKey)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKey,
				Value:     val,
			})
		} else if configKey = SourceDBConfigPrefix + f.Name; strings.HasPrefix(f.Name, OracleDBFlagPrefix) && v.IsSet(configKey) {
			// Handle oracle db type flags, since they are also prefixed with source but are special cases
			val := v.GetString(configKey)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKey,
				Value:     val,
			})
		} else if configKey = TargetDBConfigPrefix + strings.TrimPrefix(f.Name, TargetDBFlagPrefix); strings.HasPrefix(f.Name, TargetDBFlagPrefix) && v.IsSet(configKey) {
			// Handle target db type flags
			val := v.GetString(configKey)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKey,
				Value:     val,
			})
		} else if configKey = SourceReplicaDBConfigPrefix + strings.TrimPrefix(f.Name, SourceReplicaDBFlagPrefix); strings.HasPrefix(f.Name, SourceReplicaDBFlagPrefix) && v.IsSet(configKey) {
			// Handle source-replica db type flags
			val := v.GetString(configKey)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKey,
				Value:     val,
			})
		}
		// If the flag is not set in viper, do nothing and leave it as is
		// This allows the flag to retain its default value or the value set by the user in the command line
	})

	return overrides, bindErr
}

/*
setToAliasPrefixIfSet checks whether the given configKeyPrefix or any of its alias variants (as defined in aliasCommandsPrefixes) are set in the config file.

	If the current prefix is part of an alias group (e.g., {"export-data", "export-data-from-source"}),
	the function returns the first prefix in the group that has a section set in the config file (`v.IsSet(sec)`).

	This is useful when a command has multiple aliases, and we want to bind to whichever variant
	is actually used in the config.

	If none of the aliases are set, the original configKeyPrefix is returned unchanged.

Example:

- cmd path: "yb-voyager export data" → configKeyPrefix: "export-data"

- if "export-data-from-source" is set in the config, it will be returned as the effective prefix
*/
func setToAliasPrefixIfSet(configKeyPrefix string, v *viper.Viper) string {
	for _, group := range aliasCommandsPrefixes {
		if utils.ContainsAnyStringFromSlice(group, configKeyPrefix) {
			for _, sec := range group {
				if v.IsSet(sec) {
					configKeyPrefix = sec
					return configKeyPrefix
				}
			}
		}
	}
	return configKeyPrefix
}

/*
readAndValidateConfigFile reads the config file and validates it.
This functions is called only by readConfigFileAndGetExportDataFromTargetKeys and readConfigFileAndGetImportDataToSourceKeys functions.
It returns a Viper instance if the config file is set and found, or an error if there are issues with the file.
It does the following:
1. If the config file is set, it reads the file using Viper.
2. If the file is read successfully, it validates the config file for allowed keys and sections.
3. If the cfgFile variable is empty or an error occurs while reading the file, it returns nil for the Viper instance.
4. If the config file is read and validated successfully, it returns the Viper instance.
*/
func readAndValidateConfigFile() (*viper.Viper, error) {
	if cfgFile == "" {
		return nil, nil
	}

	v := viper.New()
	v.SetConfigFile(cfgFile)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Validate the config file for allowed keys and sections
	err := validateConfigFile(v)
	if err != nil {
		return nil, err
	}

	return v, nil
}

/*
readConfigFileAndGetExportDataFromTargetKeys reads the config file and returns the export-data-from-target keys.
It returns a slice of strings containing the keys that are set in the config file.
It does the following:
1. It calls readAndValidateConfigFile to read the config file and validate it.
2. If and error occurs in this, it returns an error and a nil slice.
3. If viper instance is nil, it returns an empty slice.
4. If the config file is read and validated successfully, it returns a slice of strings containing the keys that are set in the config file.
*/
func readConfigFileAndGetExportDataFromTargetKeys() ([]string, error) {
	v, err := readAndValidateConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to read and validate config file: %w", err)
	}
	if v == nil {
		return []string{}, nil
	}

	// Get the export-data-from-target keys that are set in the config file
	exportDataFromTargetKeys := []string{}
	const keyPrefix = "export-data-from-target."
	for _, key := range v.AllKeys() {
		if strings.HasPrefix(key, keyPrefix) && v.IsSet(key) {
			// Extract the key name after the prefix
			key = strings.TrimPrefix(key, keyPrefix)
			key = strings.TrimSpace(key)
			// Add the key to the list
			exportDataFromTargetKeys = append(exportDataFromTargetKeys, key)
		}
	}

	return exportDataFromTargetKeys, nil
}

/*
readConfigFileAndGetImportDataToSourceKeys reads the config file and returns the import-data-to-source keys.
It returns a slice of strings containing the keys that are set in the config file.
It does the following:
1. It calls readAndValidateConfigFile to read the config file and validate it.
2. If and error occurs in this, it returns an error and a nil slice.
3. If viper instance is nil, it returns an empty slice.
4. If the config file is read and validated successfully, it returns a slice of strings containing the keys that are set in the config file.
*/
func readConfigFileAndGetImportDataToSourceKeys() ([]string, error) {
	v, err := readAndValidateConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to read and validate config file: %w", err)
	}
	if v == nil {
		return []string{}, nil
	}

	// Get the import-data-to-source keys that are set in the config file
	importDataToSourceKeys := []string{}
	const keyPrefix = "import-data-to-source."
	for _, key := range v.AllKeys() {
		if strings.HasPrefix(key, keyPrefix) && v.IsSet(key) {
			// Extract the key name after the prefix
			key = strings.TrimPrefix(key, keyPrefix)
			key = strings.TrimSpace(key)
			// Add the key to the list
			importDataToSourceKeys = append(importDataToSourceKeys, key)
		}
	}

	return importDataToSourceKeys, nil
}
