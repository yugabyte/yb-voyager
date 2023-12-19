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

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var analyzeSchemaCmd = &cobra.Command{
	Use: "analyze-schema",
	Short: "Analyze converted source database schema and generate a report about YB incompatible constructs.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/analyze-schema/",
	Long: ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		validateReportOutputFormat()
	},

	Run: func(cmd *cobra.Command, args []string) {
		analyzeSchema()
	},
}

var exportSchemaCmd = &cobra.Command{
	Use:   "export-schema",
	Short: formatCommandDescriptionString(EXPORT_SCHEMA_SHORT_DESC),
	Long:  ``,

	PreRun: exportSchemaCmdPreRun,
	Run:    exportSchemaCmdRun,
}

var exportDataCmd = &cobra.Command{
	Use:   "export-data",
	Short: formatCommandDescriptionString(EXPORT_DATA_SHORT_DESC),
	Long:  ``,
	Args:  cobra.NoArgs,

	PreRun: exportDataCmdPreRun,
	Run:    exportDataCmdRunFn,
}

var exportDataFromSrcCmd = &cobra.Command{
	Use:   "export-data-from-source",
	Short: exportDataCmd.Short,
	Long:  exportDataCmd.Long,
	Args:  exportDataCmd.Args,

	PreRun: exportDataCmd.PreRun,
	Run:    exportDataCmd.Run,
}

var exportDataFromTargetCmd = &cobra.Command{
	Use:   "export-data-from-target",
	Short: "Export data from target YugabyteDB in the fall-back/fall-forward workflows.",
	Long:  ``,

	Run: exportDataFromTargetCmdRun,
}

var exportDataStatusCmd = &cobra.Command{
	Use:   "export-data-status",
	Short: "Print status of an ongoing/completed data export.",

	Run: exportDataStatusCmdRun,
}

var getDataMigrationReportCmd = &cobra.Command{
	Use:   "get-data-migration-report",
	Short: "Print the consolidated report of migration of data.",
	Long:  `Print the consolidated report of migration of data among different DBs (source / target / source-replica) when export-type 'snapshot-and-changes' is enabled.`,

	Run: getDataMigrationReportCmdRun,
}

var importSchemaCmd = &cobra.Command{
	Use: "import-schema",
	Short: "Import schema into the target YugabyteDB database\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/import-schema/",

	PreRun: importSchemaPreRunFn,
	Run:    importSchemaRunFn,
}

var importDataCmd = &cobra.Command{
	Use: "import-data",
	Short: "Import data from compatible source database to target database.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/data-migration/import-data/",
	Long:   `Import the data exported from the source database into the target database. Also import data(snapshot + changes from target) into source-replica/source in case of live migration with fall-back/fall-forward worflows.`,
	Args:   cobra.NoArgs,
	PreRun: importDataCmdPreRun,
	Run:    importDataCommandFn,
}

var importDataToSourceCmd = &cobra.Command{
	Use: "import-data-to-source",
	Short: "Import data into the source DB to prepare for fall-back.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-back/",
	Long: ``,

	Run: importDataToSourceCmdRun,
}

var importDataToSourceReplicaCmd = &cobra.Command{
	Use: "import-data-to-source-replica",
	Short: "Import data into source-replica database to prepare for fall-forward.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-forward/",
	Long: ``,

	Run: importDataToSourceReplicaCmdRun,
}

var importDataToTargetCmd = &cobra.Command{
	Use:   "import-data-to-target",
	Short: importDataCmd.Short,
	Long:  importDataCmd.Long,
	Args:  importDataCmd.Args,

	PreRun: importDataCmd.PreRun,
	Run:    importDataCmd.Run,
}

var importDataFileCmd = &cobra.Command{
	Use: "import-data-file",
	Short: "This command imports data from given files into YugabyteDB database. The files can be present either in local directories or cloud storages like AWS S3, GCS buckets and Azure blob storage. Incremental data load is also supported.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/bulk-data-load/",

	PreRun: importDataFileCmdPreRun,

	Run: importDataFileCmdRun,
}

var importDataStatusCmd = &cobra.Command{
	Use:   "import-data-status",
	Short: "Print status of an ongoing/completed import data.",

	Run: importDataStatusCmdRun,
}

var endMigrationCmd = &cobra.Command{
	Use:   "end-migration",
	Short: "End the current migration and cleanup all metadata stored in databases(Target, Source-Replica and Source) and export-dir",
	Long:  "End the current migration and cleanup all metadata stored in databases(Target, Source-Replica and Source) and export-dir",

	PreRun: endMigrationCmdPreRun,
	Run:    endMigrationCommandFn,
}

var archiveChangesCmd = &cobra.Command{
	Use: "archive-changes",
	Short: "Delete the already imported changes and optionally archive them before deleting.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/cutover-archive/archive-changes/",
	Long: `This command limits the disk space used by the locally queued CDC events. Once the changes from the local queue are applied on the target DB (and source-replica DB), they are eligible for deletion. The command gives an option to archive the changes before deleting by moving them to some other directory.

Note that: even if some changes are applied to the target databases, they are deleted only after the disk space utilisation exceeds 70%.
	`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateCommonArchiveFlags()
	},

	Run: archiveChangesCommandFn,
}

var cutoverStatusCmd = &cobra.Command{
	Use:   "cutover-status",
	Short: "Prints status of the cutover to YugabyteDB, To initiate cutover, refer to `yb-voyager initiate-cutover-to-*` commands.",
	Long:  "Prints status of the cutover to YugabyteDB, To initiate cutover, refer to `yb-voyager initiate-cutover-to-*` commands.",

	Run: func(cmd *cobra.Command, args []string) {
		checkAndReportCutoverStatus()
	},
}

var cutoverToSourceCmd = &cobra.Command{
	Use:   "initiate-cutover-to-source",
	Short: "Initiate cutover to source DB",
	Long:  `Initiate cutover to source DB`,

	Run: func(cmd *cobra.Command, args []string) {
		err := InitiatePrimarySwitch("fallback")
		if err != nil {
			utils.ErrExit("failed to initiate fallback: %v", err)
		}
	},
}

var cutoverToSourceReplicaCmd = &cobra.Command{
	Use:   "initiate-cutover-to-source-replica",
	Short: "Initiate cutover to source-replica DB",
	Long:  `Initiate cutover to source-replica DB`,

	Run: func(cmd *cobra.Command, args []string) {
		err := InitiatePrimarySwitch("fallforward")
		if err != nil {
			utils.ErrExit("failed to initiate fallforward: %v", err)
		}
	},
}

var cutoverToTargetCmd = &cobra.Command{
	Use:   "initiate-cutover-to-target",
	Short: "Initiate cutover to target DB",
	Long:  `Initiate cutover to target DB`,

	Run: cutoverToTargetCmdRun,
}
