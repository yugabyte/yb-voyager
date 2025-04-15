package cmd

import (
	"fmt"
	"os"
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
	SourceDBFlagPrefix = "source-"
	TargetDBFlagPrefix = "target-"
	OracleDBFlagPrefix = "oracle-"

	// Config key prefixes (used in config file keys)
	SourceDBConfigPrefix = "source."
	TargetDBConfigPrefix = "target."
)

var allowedGlobalConfigKeys = mapset.NewThreadUnsafeSet[string](
	"export-dir", "log-level", "send-diagnostics", "run-guardrails-checks",
)

var allowedSourceConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-type", "db-host", "db-port", "db-user", "db-name", "db-password",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-ca", "ssl-root-cert",
	"ssl-crl", "oracle-db-sid", "oracle-home", "oracle-tns-alias", "oracle-cdb-name",
	"oracle-cdb-sid", "oracle-cdb-tns-alias",
)

var allowedTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"name", "db-host", "db-port", "db-user", "db-password", "db-name",
	"db-schema", "ssl-cert", "ssl-mode", "ssl-key", "ssl-root-cert", "ssl-crl",
)

var allowedAssessMigrationConfigKeys = mapset.NewThreadUnsafeSet[string](
	"iops-capture-interval", "target-db-version", "assessment-metadata-dir",
)

var allowedAnalyzeSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"output-format", "target-db-version",
)

var allowedExportSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"use-orafce", "comments-on-objects", "object-type-list", "exclude-object-type-list",
	"skip-recommendations", "assessment-report-path",
)

var allowedExportDataConfigKeys = mapset.NewThreadUnsafeSet[string](
	"disable-pb", "exclude-table-list", "table-list", "exclude-table-list-file-path",
	"table-list-file-path", "parallel-jobs", "export-type",
)

var allowedExportDataFromTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"disable-pb", "exclude-table-list", "table-list", "exclude-table-list-file-path",
	"table-list-file-path",
)

var allowedImportSchemaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"continue-on-error", "object-type-list", "exclude-object-type-list", "straight-order",
	"post-snapshot-import", "ignore-exists", "enable-orafce",
)

var allowedImportDataConfigKeys = mapset.NewThreadUnsafeSet[string](
	"batch-size", "parallel-jobs", "enable-adaptive-parallelism", "adaptive-parallelism-max",
	"disable-pb", "max-retries", "exclude-table-list", "table-list",
	"exclude-table-list-file-path", "table-list-file-path", "enable-upsert", "use-public-ip",
	"target-endpoints", "truncate-tables",
)

var allowedImportDataToSourceConfigKeys = mapset.NewThreadUnsafeSet[string](
	"parallel-jobs", "disable-pb", "max-retries",
)

var allowedImportDataToSourceReplicaConfigKeys = mapset.NewThreadUnsafeSet[string](
	"batch-size", "parallel-jobs", "truncate-tables", "disable-pb", "max-retries",
)

var allowedImportDataFileConfigKeys = mapset.NewThreadUnsafeSet[string](
	"disable-pb", "max-retries", "enable-upsert", "use-public-ip", "target-endpoints",
	"batch-size", "parallel-jobs", "enable-adaptive-parallelism", "adaptive-parallelism-max",
	"format", "delimiter", "data-dir", "file-table-map", "has-header", "escape-char",
	"quote-char", "file-opts", "null-string", "truncate-tables",
)

var allowedInitCutoverToTargetConfigKeys = mapset.NewThreadUnsafeSet[string](
	"prepare-for-fall-back", "use-yb-grpc-connector",
)

var allowedArchiveChangesConfigKeys = mapset.NewThreadUnsafeSet[string](
	"delete-changes-without-archiving", "fs-utilization-threshold", "move-to",
)

var allowedEndMigrationConfigKeys = mapset.NewThreadUnsafeSet[string](
	"backup-schema-files", "backup-data-files", "save-migration-reports", "backup-log-files",
	"backup-dir",
)

// Define allowed nested sections
var allowedConfigSections = map[string]mapset.Set[string]{
	"source":                        allowedSourceConfigKeys,
	"target":                        allowedTargetConfigKeys,
	"assess-migration":              allowedAssessMigrationConfigKeys,
	"analyze-schema":                allowedAnalyzeSchemaConfigKeys,
	"export-schema":                 allowedExportSchemaConfigKeys,
	"export-data":                   allowedExportDataConfigKeys,
	"export-data-from-source":       allowedExportDataConfigKeys,
	"export-data-from-target":       allowedExportDataFromTargetConfigKeys,
	"import-schema":                 allowedImportSchemaConfigKeys,
	"import-data":                   allowedImportDataConfigKeys,
	"import-data-to-target":         allowedImportDataConfigKeys,
	"import-data-to-source":         allowedImportDataToSourceConfigKeys,
	"import-data-to-source-replica": allowedImportDataToSourceReplicaConfigKeys,
	"import-data-file":              allowedImportDataFileConfigKeys,
	"initiate-cutover-to-target":    allowedInitCutoverToTargetConfigKeys,
	"archive-changes":               allowedArchiveChangesConfigKeys,
	"end-migration":                 allowedEndMigrationConfigKeys,
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
func initConfig(cmd *cobra.Command) ([]ConfigFlagOverride, error) {
	v := viper.New()

	// Precedence of which config file to use:
	// CLI Flag > ENV Variable > Default config file in home directory
	if cfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(cfgFile)
	} else if os.Getenv("YB_VOYAGER_CONFIG_FILE") != "" {
		// passed as an ENV variable by the name YB_VOYAGER_CONFIG_FILE
		v.SetConfigFile(os.Getenv("YB_VOYAGER_CONFIG_FILE"))
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}

		// Search config in home directory with name "yb-voyager-config" (without extension).
		v.AddConfigPath(home)
		v.SetConfigName("yb-voyager-config")
		v.SetConfigType("yaml")
	}

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", v.ConfigFileUsed())
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Validate the config file for allowed keys and sections
	err := validateConfigFile(v)
	if err != nil {
		return nil, err
	}

	// Bind the config values to the Cobra command flags
	overrides, err := bindCobraFlagsToViper(cmd, v)
	if err != nil {
		return nil, fmt.Errorf("failed to bind cobra flags to viper: %w", err)
	}

	return overrides, nil
}

/*
validateConfigFile checks the loaded configuration for correctness.

	It performs the following validations:
	1. Ensures that all global-level keys (non-nested) are in the allowed list.
	2. Ensures that all section names (e.g., export-schema) are known and valid.
	3. Ensures that all nested keys inside each section are valid for that section.
	4. Ensures that only one section from each group of mutually exclusive aliases is used (e.g.,
	"export-data" vs "export-data-from-source").

	Any invalid global keys, unknown sections, or invalid section keys are collected and printed.
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

	// If invalid configurations exist, print them
	if invalidGlobalKeys.Cardinality() > 0 || len(invalidSectionKeys) > 0 || invalidSections.Cardinality() > 0 || len(conflictingSections) > 0 {
		if invalidGlobalKeys.Cardinality() > 0 {
			fmt.Printf("%s [%s]\n", color.RedString("Invalid global config keys:"), strings.Join(invalidGlobalKeys.ToSlice(), ", "))
		}
		for section, keys := range invalidSectionKeys {
			fmt.Printf("%s [%s]\n", color.RedString(fmt.Sprintf("Invalid keys in section '%s':", section)), strings.Join(keys.ToSlice(), ", "))
		}
		if invalidSections.Cardinality() > 0 {
			fmt.Printf("%s [%s]\n", color.RedString("Invalid sections:"), strings.Join(invalidSections.ToSlice(), ", "))
		}
		for _, conflict := range conflictingSections {
			fmt.Printf("%s [%s]\n", color.RedString("Only one of the following sections can be used:"), strings.Join(conflict, ", "))
		}

		// Return a general error message
		return fmt.Errorf("found invalid configurations in config file: %s", v.ConfigFileUsed())
		// TODO: Add a link to a sample config file in the error message
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
		if v.IsSet(configKeyPrefix + "." + f.Name) {
			val := v.GetString(configKeyPrefix + "." + f.Name)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				// In case of an error while setting the flag from viper, return the error
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: configKeyPrefix + "." + f.Name,
				Value:     val,
			})
		} else if v.IsSet(f.Name) {
			// Bind the global flag from viper to cmd
			val := v.GetString(f.Name)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: f.Name,
				Value:     val,
			})
		} else if strings.HasPrefix(f.Name, SourceDBFlagPrefix) && v.IsSet(SourceDBConfigPrefix+strings.TrimPrefix(f.Name, SourceDBFlagPrefix)) {
			// Handle source db type flags
			val := v.GetString(SourceDBConfigPrefix + strings.TrimPrefix(f.Name, SourceDBFlagPrefix))
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: SourceDBConfigPrefix + strings.TrimPrefix(f.Name, SourceDBFlagPrefix),
				Value:     val,
			})
		} else if strings.HasPrefix(f.Name, OracleDBFlagPrefix) && v.IsSet(SourceDBConfigPrefix+f.Name) {
			// Handle oracle db type flags, since they are also prefixed with source but are special cases
			val := v.GetString(SourceDBConfigPrefix + f.Name)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: SourceDBConfigPrefix + f.Name,
				Value:     val,
			})
		} else if strings.HasPrefix(f.Name, TargetDBFlagPrefix) && v.IsSet(TargetDBConfigPrefix+strings.TrimPrefix(f.Name, TargetDBFlagPrefix)) {
			// Handle target db type flags
			val := v.GetString(TargetDBConfigPrefix + strings.TrimPrefix(f.Name, TargetDBFlagPrefix))
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			overrides = append(overrides, ConfigFlagOverride{
				FlagName:  f.Name,
				ConfigKey: TargetDBConfigPrefix + strings.TrimPrefix(f.Name, TargetDBFlagPrefix),
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
