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
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var dataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show data export, import, and migration status",
	Long:  `Show the status of data export, import, and live migration. Combines export status, import status, and the live migration report into a single view.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateReportOutputFormat(migrationReportFormats, reportOrStatusCmdOutputFormat)
	},

	Run: func(cmd *cobra.Command, args []string) {
		streamChanges, err := checkStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}

		if streamChanges {
			runLiveMigrationReport(cmd)
			return
		}

		showedSomething := false

		if exportDataAvailable() {
			showExportStatus()
			showedSomething = true
		}

		if dataIsExported() {
			if showedSomething {
				fmt.Println()
			}
			showImportStatus()
			showedSomething = true
		}

		if !showedSomething {
			fmt.Println("No data export or import activity found yet.")
		}
	},
}

func init() {
	dataCmd.AddCommand(dataStatusCmd)
	registerExportDirFlag(dataStatusCmd)
	registerConfigFileFlag(dataStatusCmd)

	dataStatusCmd.PersistentFlags().StringVarP(&config.LogLevel, "log-level", "l", "info",
		"log level for yb-voyager. Accepted values: (trace, debug, info, warn, error, fatal, panic)")

	dataStatusCmd.Flags().StringVar(&reportOrStatusCmdOutputFormat, "output-format", "table",
		"format in which report will be generated: (table, json) (default: table)")

	dataStatusCmd.Flags().StringVar(&targetDbPassword, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server. Alternatively, you can also specify the password by setting the environment variable TARGET_DB_PASSWORD.")

	dataStatusCmd.Flags().StringVar(&sourceReplicaDbPassword, "source-replica-db-password", "",
		"password with which to connect to the source-replica DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_REPLICA_DB_PASSWORD.")

	dataStatusCmd.Flags().StringVar(&sourceDbPassword, "source-db-password", "",
		"password with which to connect to the source DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD.")
}

func exportDataAvailable() bool {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil || msr == nil {
		return false
	}
	return msr.SourceDBConf != nil
}

func showExportStatus() {
	err := InitNameRegistry(exportDir, namereg.SOURCE_DB_EXPORTER_STATUS_ROLE, nil, nil, nil, nil, false)
	if err != nil {
		utils.ErrExit("initializing name registry: %v", err)
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %s", err)
	}
	if msr == nil {
		color.Cyan(exportDataStatusMsg)
		return
	}
	useDebezium = msr.IsSnapshotExportedViaDebezium()

	if msr.SourceDBConf == nil {
		return
	}
	source = *msr.SourceDBConf
	sqlname.SourceDBType = source.DBType
	leafPartitions := getLeafPartitionsFromRootTable()

	var rows []*exportTableMigStatusOutputRow
	if useDebezium {
		rows, err = runExportDataStatusCmdDbzm(false, leafPartitions, msr)
	} else {
		rows, err = runExportDataStatusCmd(msr, leafPartitions)
	}
	if err != nil {
		utils.ErrExit("error: %s\n", err)
	}
	if reportOrStatusCmdOutputFormat == "json" {
		reportFilePath := filepath.Join(exportDir, "reports", "export-data-status-report.json")
		reportFile := jsonfile.NewJsonFile[[]*exportTableMigStatusOutputRow](reportFilePath)
		err := reportFile.Create(&rows)
		if err != nil {
			utils.ErrExit("creating json file: %s: %v", reportFilePath, err)
		}
		fmt.Print(color.GreenString("Export data status report written to %s\n", reportFilePath))
		return
	}
	displayExportDataStatus(rows)
}

func showImportStatus() {
	dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	if utils.FileOrFolderExists(dataFileDescriptorPath) {
		importerRole = TARGET_DB_IMPORTER_ROLE
	} else {
		importerRole = IMPORT_FILE_ROLE
	}

	err := InitNameRegistry(exportDir, importerRole, nil, nil, nil, nil, false)
	if err != nil {
		utils.ErrExit("initialize name registry: %w", err)
	}
	err = runImportDataStatusCmd()
	if err != nil {
		utils.ErrExit("error running import data status: %w\n", err)
	}
}

func runLiveMigrationReport(cmd *cobra.Command) {
	migrationStatus, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error while getting migration status: %w\n", err)
	}
	migrationUUID, err = uuid.Parse(migrationStatus.MigrationUUID)
	if err != nil {
		utils.ErrExit("error while parsing migration UUID: %w\n", err)
	}
	if migrationStatus.TargetDBConf != nil {
		getTargetPassword(cmd)
		migrationStatus.TargetDBConf.Password = tconf.Password
	}
	if migrationStatus.FallForwardEnabled {
		getSourceReplicaDBPassword(cmd)
		migrationStatus.SourceReplicaDBConf.Password = tconf.Password
	}
	if migrationStatus.FallbackEnabled {
		getSourceDBPassword(cmd)
		migrationStatus.SourceDBAsTargetConf.Password = tconf.Password
	}
	err = InitNameRegistry(exportDir, "", nil, nil, nil, nil, false)
	if err != nil {
		utils.ErrExit("initializing name registry: %w", err)
	}
	color.Yellow("Generating data migration report for migration UUID: %s...\n", migrationStatus.MigrationUUID)
	getDataMigrationReportCmdFn(migrationStatus)
}
