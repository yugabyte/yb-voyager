package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var allowedGlobalConfigKeys = map[string]bool{
	"export-dir":            true,
	"log-level":             true,
	"send-diagnostics":      true,
	"run-guardrails-checks": true,
}

var allowedSourceConfigKeys = map[string]bool{
	"name":                 true,
	"db-type":              true,
	"db-host":              true,
	"db-port":              true,
	"db-user":              true,
	"db-name":              true,
	"db-password":          true,
	"db-schema":            true,
	"ssl-cert":             true,
	"ssl-mode":             true,
	"ssl-key":              true,
	"ssl-ca":               true,
	"ssl-root-cert":        true,
	"ssl-crl":              true,
	"oracle-db-sid":        true,
	"oracle-home":          true,
	"oracle-tns-alias":     true,
	"oracle-cdb-name":      true,
	"oracle-cdb-sid":       true,
	"oracle-cdb-tns-alias": true,
}

var allowedTargetConfigKeys = map[string]bool{
	"name":          true,
	"db-host":       true,
	"db-port":       true,
	"db-user":       true,
	"db-password":   true,
	"db-name":       true,
	"db-schema":     true,
	"ssl-cert":      true,
	"ssl-mode":      true,
	"ssl-key":       true,
	"ssl-root-cert": true,
	"ssl-crl":       true,
}

var allowedAssessMigrationConfigKeys = map[string]bool{
	"iops-capture-interval":   true,
	"target-db-version":       true,
	"assessment-metadata-dir": true,
}

var allowedAnalyzeSchemaConfigKeys = map[string]bool{
	"output-format":     true,
	"target-db-version": true,
}

var allowedExportSchemaConfigKeys = map[string]bool{
	"use-orafce":               true,
	"comments-on-objects":      true,
	"object-type-list":         true,
	"exclude-object-type-list": true,
	"skip-recommendations":     true,
	"assessment-report-path":   true,
}

var allowedExportDataConfigKeys = map[string]bool{
	"disable-pb":                   true,
	"exclude-table-list":           true,
	"table-list":                   true,
	"exclude-table-list-file-path": true,
	"table-list-file-path":         true,
	"parallel-jobs":                true,
	"export-type":                  true,
}

var allowedExportDataFromTargetConfigKeys = map[string]bool{
	"disable-pb":                   true,
	"exclude-table-list":           true,
	"table-list":                   true,
	"exclude-table-list-file-path": true,
	"table-list-file-path":         true,
}

var allowedImportSchemaConfigKeys = map[string]bool{
	"continue-on-error":        true,
	"object-type-list":         true,
	"exclude-object-type-list": true,
	"straight-order":           true,
	"post-snapshot-import":     true,
	"ignore-exists":            true,
	"enable-orafce":            true,
}

var allowedImportDataConfigKeys = map[string]bool{
	"batch-size":                   true,
	"parallel-jobs":                true,
	"enable-adaptive-parallelism":  true,
	"adaptive-parallelism-max":     true,
	"disable-pb":                   true,
	"max-retries":                  true,
	"exclude-table-list":           true,
	"table-list":                   true,
	"exclude-table-list-file-path": true,
	"table-list-file-path":         true,
	"enable-upsert":                true,
	"use-public-ip":                true,
	"target-endpoints":             true,
	"truncate-tables":              true,
}

var allowedImportDataToSourceConfigKeys = map[string]bool{
	"parallel-jobs": true,
	"disable-pb":    true,
	"max-retries":   true,
}

var allowedImportDataToSourceReplicaConfigKeys = map[string]bool{
	"batch-size":      true,
	"parallel-jobs":   true,
	"truncate-tables": true,
	"disable-pb":      true,
	"max-retries":     true,
}

var allowedImportDataFileConfigKeys = map[string]bool{
	"disable-pb":                  true,
	"max-retries":                 true,
	"enable-upsert":               true,
	"use-public-ip":               true,
	"target-endpoints":            true,
	"batch-size":                  true,
	"parallel-jobs":               true,
	"enable-adaptive-parallelism": true,
	"adaptive-parallelism-max":    true,
	"format":                      true,
	"delimiter":                   true,
	"data-dir":                    true,
	"file-table-map":              true,
	"has-header":                  true,
	"escape-char":                 true,
	"quote-char":                  true,
	"file-opts":                   true,
	"null-string":                 true,
	"truncate-tables":             true,
}

var allowedInitCutoverToTargetConfigKeys = map[string]bool{
	"prepare-for-fall-back": true,
	"use-yb-grpc-connector": true,
}

var allowedArchiveChangesConfigKeys = map[string]bool{
	"delete-changes-without-archiving": true,
	"fs-utilization-threshold":         true,
	"move-to":                          true,
}

var allowedEndMigrationConfigKeys = map[string]bool{
	"backup-schema-files":    true,
	"backup-data-files":      true,
	"save-migration-reports": true,
	"backup-log-files":       true,
	"backup-dir":             true,
}

// Define allowed nested sections
var allowedConfigSections = map[string]map[string]bool{
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

/*
initConfig initializes the configuration for the given Cobra command.

	It performs the following steps:
	 1. Creates a new Viper instance to isolate config handling for the command.
	 2. Loads the config file if explicitly provided via --config, or defaults to ~/.yb-voyager.yaml.
	 3. Reads and validates the configuration file for allowed global keys, sections, and section keys.
	 4. Binds Viper config values to Cobra flags, giving priority to command-line flags over config values.
	 5. Returns an error if config file reading, validation, or flag binding fails.

	This setup ensures CLI > Config precedence
*/
func initConfig(cmd *cobra.Command) error {
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
			return err
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
			return err
		}
	}

	// Validate the config file for allowed keys and sections
	err := validateConfigFile(v)
	if err != nil {
		return err
	}

	// Bind the config values to the Cobra command flags
	err = bindCobraFlagsToViper(cmd, v)
	if err != nil {
		return fmt.Errorf("failed to bind cobra flags to viper: %w", err)
	}

	return nil
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
	var (
		invalidGlobalKeys   []string
		invalidSectionKeys  = make(map[string][]string)
		invalidSections     []string
		conflictingSections = [][]string{}
		presentSections     = make(map[string]bool)
	)

	for _, key := range v.AllKeys() {
		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			// Check global level keys
			if !allowedGlobalConfigKeys[key] {
				invalidGlobalKeys = append(invalidGlobalKeys, key)
			}
		} else {
			// Validate section-based keys
			// The section is the first part of the key, the rest of the parts combined using "." are the nested key
			// For example: "a.b.c" -> section: "a", nestedKey: "b.c"
			section := parts[0]
			nestedKey := strings.Join(parts[1:], ".")
			presentSections[section] = true
			if allowedConfigSections[section] == nil {
				// Unknown section
				if !utils.ContainsAnyStringFromSlice(invalidSections, section) {
					invalidSections = append(invalidSections, section)
				}
			} else if !allowedConfigSections[section][nestedKey] {
				// Invalid key inside a known section
				invalidSectionKeys[section] = append(invalidSectionKeys[section], nestedKey)
			}
		}
	}

	// Check for mutually exclusive section usage
	for _, group := range aliasCommandsPrefixes {
		var used []string
		for _, sec := range group {
			if presentSections[sec] {
				used = append(used, sec)
			}
		}
		if len(used) > 1 {
			conflictingSections = append(conflictingSections, used)
		}
	}

	// If invalid configurations exist, print them
	if len(invalidGlobalKeys) > 0 || len(invalidSectionKeys) > 0 || len(invalidSections) > 0 || len(conflictingSections) > 0 {
		if len(invalidGlobalKeys) > 0 {
			fmt.Printf("%s %v\n", color.RedString("Invalid global config keys:"), invalidGlobalKeys)
		}
		for section, keys := range invalidSectionKeys {
			printStatement := fmt.Sprintf("Invalid keys in section '%s':", section)
			fmt.Printf("%s %v\n", color.RedString(printStatement), keys)
		}
		if len(invalidSections) > 0 {
			fmt.Printf("%s %v\n", color.RedString("Invalid sections:"), invalidSections)
		}
		if len(conflictingSections) > 0 {
			for _, conflict := range conflictingSections {
				fmt.Printf("%s %v\n", color.RedString("Only one of the following sections can be used:"), conflict)
			}
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

	This function allows users to configure flags through the config file or environment variables,
	while still letting command-line input take precedence.
*/
func bindCobraFlagsToViper(cmd *cobra.Command, v *viper.Viper) error {
	var bindErr error

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
		} else if v.IsSet(f.Name) {
			// Bind the global flag from viper to cmd
			val := v.GetString(f.Name)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
		} else if strings.HasPrefix(f.Name, "source-") && v.IsSet("source."+strings.TrimPrefix(f.Name, "source-")) {
			// Handle source db type flags
			val := v.GetString("source." + strings.TrimPrefix(f.Name, "source-"))
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
		} else if strings.HasPrefix(f.Name, "oracle-") && v.IsSet("source."+f.Name) {
			// Handle oracle db type flags, since they are also prefixed with source but are special cases
			val := v.GetString("source." + f.Name)
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
		} else if strings.HasPrefix(f.Name, "target-") && v.IsSet("target."+strings.TrimPrefix(f.Name, "target-")) {
			// Handle target db type flags
			val := v.GetString("target." + strings.TrimPrefix(f.Name, "target-"))
			err := cmd.Flags().Set(f.Name, val)
			if err != nil {
				bindErr = err
				return
			}
			// fmt.Printf("binding %s from viper to cmd flag: %s=%s\n", "target."+strings.TrimPrefix(f.Name, "target-"), f.Name, val)
		}
		// If the flag is not set in viper, do nothing and leave it as is
		// This allows the flag to retain its default value or the value set by the user in the command line
	})

	if bindErr != nil {
		return bindErr
	}

	return nil
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
