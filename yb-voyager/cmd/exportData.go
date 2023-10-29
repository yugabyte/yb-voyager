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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var exporterRole string

var exportDataCmd = &cobra.Command{
	Use: "data",
	Short: "Export tables' data (either snapshot-only or snapshot-and-changes) from source database to export-dir. \nNote: For Oracle and MySQL, there is a beta feature to speed up the data export of snapshot, set the environment variable BETA_FAST_DATA_EXPORT=1 to try it out. You can refer to YB Voyager Documentation (https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#accelerate-data-export-for-mysql-and-oracle) for more details on this feature.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/data-migration/export-data/",
	Long: ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		setExportFlagsDefaults()
		if exporterRole == "" {
			exporterRole = SOURCE_DB_EXPORTER_ROLE
		}
		err := validateExportFlags(cmd, exporterRole)
		if err != nil {
			utils.ErrExit("Error: %s", err.Error())
		}
		validateExportTypeFlag()
		markFlagsRequired(cmd)
		if changeStreamingIsEnabled(exportType) {
			useDebezium = true
		}
	},

	Run: exportDataCommandFn,
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
	registerCommonGlobalFlags(exportDataCmd)
	registerCommonExportFlags(exportDataCmd)
	registerSourceDBConnFlags(exportDataCmd, true)
	registerExportDataFlags(exportDataCmd)
}

func exportDataCommandFn(cmd *cobra.Command, args []string) {
	triggerName, err := getTriggerName(exporterRole)
	if err != nil {
		utils.ErrExit("failed to get trigger name for checking if DB is switched over: %v", err)
	}
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	exitIfDBSwitchedOver(triggerName)
	checkDataDirs()
	if useDebezium && !changeStreamingIsEnabled(exportType) {
		utils.PrintAndLog("Note: Beta feature to accelerate data export is enabled by setting BETA_FAST_DATA_EXPORT environment variable")
	}
	if changeStreamingIsEnabled(exportType) {
		utils.PrintAndLog(color.YellowString(`Note: Live migration is a TECH PREVIEW feature.`))
	}
	utils.PrintAndLog("export of data for source type as '%s'", source.DBType)
	sqlname.SourceDBType = source.DBType

	err = retrieveMigrationUUID()
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}
	success := exportData()
	if success {
		tableRowCount := getExportedRowCountSnapshot(exportDir)
		callhome.GetPayload(exportDir, migrationUUID)
		callhome.UpdateDataStats(exportDir, tableRowCount)
		callhome.PackAndSendPayload(exportDir)

		setDataIsExported()
		updateSourceDBConfInMSR()
		color.Green("Export of data complete \u2705")
		log.Info("Export of data completed.")
		startFallBackSetupIfRequired()
	} else {
		color.Red("Export of data failed! Check %s/logs for more details. \u274C", exportDir)
		log.Error("Export of data failed.")
		atexit.Exit(1)
	}
}

func exportData() bool {

	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the source db: %s", err)
	}
	defer source.DB().Disconnect()
	checkSourceDBCharset()
	source.DB().CheckRequiredToolsAreInstalled()
	updateSourceDBConfInMSR()
	saveExportTypeInMetaDB()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finalTableList, tablesColumnList := getFinalTableColumnList()

	if len(finalTableList) == 0 {
		utils.PrintAndLog("no tables present to export, exiting...")
		setDataIsExported()
		dfd := datafile.Descriptor{
			ExportDir:    exportDir,
			DataFileList: make([]*datafile.FileEntry, 0),
		}
		dfd.Save()
		updateSourceDBConfInMSR()
		os.Exit(0)
	}

	if changeStreamingIsEnabled(exportType) || useDebezium {
		config, tableNametoApproxRowCountMap, err := prepareDebeziumConfig(finalTableList, tablesColumnList)
		if err != nil {
			log.Errorf("Failed to prepare dbzm config: %v", err)
			return false
		}
		err = debeziumExportData(ctx, config, tableNametoApproxRowCountMap)
		if err != nil {
			log.Errorf("Export Data using debezium failed: %v", err)
			return false
		}

		if changeStreamingIsEnabled(exportType) {
			log.Infof("live migration complete, proceeding to cutover")
			triggerName, err := getTriggerName(exporterRole)
			if err != nil {
				utils.ErrExit("failed to get trigger name after data export: %v", err)
			}
			err = createTriggerIfNotExists(triggerName)
			if err != nil {
				utils.ErrExit("failed to create trigger file after data export: %v", err)
			}
			displayExportedRowCountSnapshotAndChanges()
		}
		return true
	} else {
		err = exportDataOffline(ctx, cancel, finalTableList, tablesColumnList)
		if err != nil {
			log.Errorf("Export Data failed: %v", err)
			return false
		}
		return true
	}

}

func getFinalTableColumnList() ([]*sqlname.SourceName, map[*sqlname.SourceName][]string) {
	var tableList []*sqlname.SourceName
	// store table list after filtering unsupported or unnecessary tables
	var finalTableList, skippedTableList []*sqlname.SourceName
	fullTableList := source.DB().GetAllTableNames()
	excludeTableList := extractTableListFromString(fullTableList, source.ExcludeTableList, "exclude")
	if source.TableList != "" {
		tableList = extractTableListFromString(fullTableList, source.TableList, "include")
	} else {
		tableList = fullTableList
	}
	finalTableList = sqlname.SetDifference(tableList, excludeTableList)
	log.Infof("initial all tables table list for data export: %v", tableList)

	if !changeStreamingIsEnabled(exportType) {
		finalTableList, skippedTableList = source.DB().FilterEmptyTables(finalTableList)
		if len(skippedTableList) != 0 {
			utils.PrintAndLog("skipping empty tables: %v", skippedTableList)
		}
	}

	finalTableList, skippedTableList = source.DB().FilterUnsupportedTables(finalTableList, useDebezium)
	if len(skippedTableList) != 0 {
		utils.PrintAndLog("skipping unsupported tables: %v", skippedTableList)
	}

	tablesColumnList, unsupportedColumnNames := source.DB().GetColumnsWithSupportedTypes(finalTableList, useDebezium, changeStreamingIsEnabled(exportType))
	if len(unsupportedColumnNames) > 0 {
		log.Infof("preparing column list for the data export without unsupported datatype columns: %v", unsupportedColumnNames)
		if !utils.AskPrompt("\nThe following columns data export is unsupported:\n" + strings.Join(unsupportedColumnNames, "\n") +
			"\nDo you want to ignore just these columns' data and continue with export") {
			utils.ErrExit("Exiting at user's request. Use `--exclude-table-list` flag to continue without these tables")
		}
		finalTableList = filterTableWithEmptySupportedColumnList(finalTableList, tablesColumnList)
	}
	return finalTableList, tablesColumnList
}

func exportDataOffline(ctx context.Context, cancel context.CancelFunc, finalTableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string) error {
	fmt.Printf("num tables to export: %d\n", len(finalTableList))
	utils.PrintAndLog("table list for data export: %v", finalTableList)
	exportDataStart := make(chan bool)
	quitChan := make(chan bool)             //for checking failure/errors of the parallel goroutines
	exportSuccessChan := make(chan bool, 1) //Check if underlying tool has exited successfully.
	go func() {
		q := <-quitChan
		if q {
			log.Infoln("Cancel() being called, within exportDataOffline()")
			cancel()                    //will cancel/stop both dump tool and progress bar
			time.Sleep(time.Second * 5) //give sometime for the cancel to complete before this function returns
			utils.ErrExit("yb-voyager encountered internal error. "+
				"Check %s/logs/yb-voyager-export-data.log for more details.", exportDir)
		}
	}()

	initializeExportTableMetadata(finalTableList)

	log.Infof("Export table metadata: %s", spew.Sdump(tablesProgressMetadata))
	UpdateTableApproxRowCount(&source, exportDir, tablesProgressMetadata)

	if source.DBType == POSTGRESQL {
		//need to export setval() calls to resume sequence value generation
		sequenceList := source.DB().GetAllSequences()
		for _, seq := range sequenceList {
			name := sqlname.NewSourceNameFromQualifiedName(seq)
			finalTableList = append(finalTableList, name)
		}
	}
	fmt.Printf("Initiating data export.\n")
	utils.WaitGroup.Add(1)
	go source.DB().ExportData(ctx, exportDir, finalTableList, quitChan, exportDataStart, exportSuccessChan, tablesColumnList)
	// Wait for the export data to start.
	<-exportDataStart

	updateFilePaths(&source, exportDir, tablesProgressMetadata)
	utils.WaitGroup.Add(1)
	exportDataStatus(ctx, tablesProgressMetadata, quitChan, exportSuccessChan, bool(disablePb))

	utils.WaitGroup.Wait() // waiting for the dump and progress bars to complete
	if ctx.Err() != nil {
		fmt.Printf("ctx error(exportData.go): %v\n", ctx.Err())
		return fmt.Errorf("ctx error(exportData.go): %w", ctx.Err())
	}

	source.DB().ExportDataPostProcessing(exportDir, tablesProgressMetadata)
	displayExportedRowCountSnapshot()
	return nil
}

// flagName can be "exclude-table-list" or "table-list"
func validateTableListFlag(tableListValue string, flagName string) {
	if tableListValue == "" {
		return
	}
	tableList := utils.CsvStringToSlice(tableListValue)

	tableNameRegex := regexp.MustCompile(`[a-zA-Z0-9_."]+`)
	for _, table := range tableList {
		if !tableNameRegex.MatchString(table) {
			utils.ErrExit("Error: Invalid table name '%v' provided with --%s flag", table, flagName)
		}
	}
}

// flagName can be "exclude-table-list-file-path" or "table-list-file-path"
func validateAndExtractTableNamesFromFile(filePath string, flagName string) (string, error) {
	if filePath == "" {
		return "", nil
	}
	if !utils.FileOrFolderExists(filePath) {
		return "", fmt.Errorf("path %q does not exist", filePath)
	}
	tableList, err := utils.ReadTableNameListFromFile(filePath)
	if err != nil {
		return "", fmt.Errorf("reading table list from file: %w", err)
	}

	tableNameRegex := regexp.MustCompile(`[a-zA-Z0-9_."]+`)
	for _, table := range tableList {
		if !tableNameRegex.MatchString(table) {
			return "", fmt.Errorf("invalid table name '%v' provided in file %s with --%s flag", table, filePath, flagName)
		}
	}
	return strings.Join(tableList, ","), nil
}

func checkDataDirs() {
	exportDataDir := filepath.Join(exportDir, "data")
	propertiesFilePath := filepath.Join(exportDir, "metainfo", "conf", "application.properties")
	sslDir := filepath.Join(exportDir, "metainfo", "ssl")
	dfdFilePath := exportDir + datafile.DESCRIPTOR_PATH
	if startClean {
		utils.CleanDir(exportDataDir)
		utils.CleanDir(sslDir)
		clearDataIsExported()
		os.Remove(dfdFilePath)
		os.Remove(propertiesFilePath)
		metadb.TruncateTablesInMetaDb(exportDir, []string{metadb.QUEUE_SEGMENT_META_TABLE_NAME, metadb.EXPORTED_EVENTS_STATS_TABLE_NAME, metadb.EXPORTED_EVENTS_STATS_PER_TABLE_TABLE_NAME})
	} else {
		if !utils.IsDirectoryEmpty(exportDataDir) {
			if (changeStreamingIsEnabled(exportType)) &&
				dbzm.IsMigrationInStreamingMode(exportDir) {
				utils.PrintAndLog("Continuing streaming from where we left off...")
			} else {
				utils.ErrExit("%s/data directory is not empty, use --start-clean flag to clean the directories and start", exportDir)
			}
		}
	}
}

func getDefaultSourceSchemaName() (string, bool) {
	switch source.DBType {
	case MYSQL:
		return source.DBName, false
	case POSTGRESQL, YUGABYTEDB:
		schemas := strings.Split(source.Schema, "|")
		if len(schemas) == 1 {
			return source.Schema, false
		} else if slices.Contains(schemas, "public") {
			return "public", false
		} else {
			return "", true
		}
	case ORACLE:
		return source.Schema, false
	default:
		panic("invalid db type")
	}
}

func extractTableListFromString(fullTableList []*sqlname.SourceName, flagTableList string, listName string) []*sqlname.SourceName {
	result := []*sqlname.SourceName{}
	if flagTableList == "" {
		return result
	}
	findPatternMatchingTables := func(pattern string, defaultSourceSchema string) []*sqlname.SourceName {
		result := lo.Filter(fullTableList, func(tableName *sqlname.SourceName, _ int) bool {
			table := tableName.Qualified.MinQuoted
			sqlNamePattern := sqlname.NewSourceNameFromMaybeQualifiedName(pattern, defaultSourceSchema)
			pattern = sqlNamePattern.Qualified.MinQuoted
			matched, err := filepath.Match(pattern, table)
			if err != nil {
				utils.ErrExit("Invalid table name pattern %q: %s", pattern, err)
			}
			return matched
		})
		return result
	}
	tableList := utils.CsvStringToSlice(flagTableList)
	var unqualifiedTables []string
	defaultSourceSchema, noDefaultSchema := getDefaultSourceSchemaName()
	var unknownTableNames []string
	for _, pattern := range tableList {
		if noDefaultSchema && len(strings.Split(pattern, ".")) == 1 {
			unqualifiedTables = append(unqualifiedTables, pattern)
			continue
		}
		tables := findPatternMatchingTables(pattern, defaultSourceSchema)
		if len(tables) == 0 {
			unknownTableNames = append(unknownTableNames, pattern)
		}
		result = append(result, tables...)
	}
	if len(unqualifiedTables) > 0 {
		utils.ErrExit("Qualify following table names %v in the %s list with schema name", unqualifiedTables, listName)
	}
	if len(unknownTableNames) > 0 {
		utils.PrintAndLog("Unknown table names %v in the %s list", unknownTableNames, listName)
		utils.ErrExit("Valid table names are %v", fullTableList)
	}
	return lo.Uniq(result)
}

func checkSourceDBCharset() {
	// If source db does not use unicode character set, ask for confirmation before
	// proceeding for export.
	charset, err := source.DB().GetCharset()
	if err != nil {
		utils.PrintAndLog("[WARNING] Failed to find character set of the source db: %s", err)
		return
	}
	log.Infof("Source database charset: %q", charset)
	if !strings.Contains(strings.ToLower(charset), "utf") {
		utils.PrintAndLog("voyager supports only unicode character set for source database. "+
			"But the source database is using '%s' character set. ", charset)
		if !utils.AskPrompt("Are you sure you want to proceed with export? ") {
			utils.ErrExit("Export aborted.")
		}
	}
}

func changeStreamingIsEnabled(s string) bool {
	return (s == CHANGES_ONLY || s == SNAPSHOT_AND_CHANGES)
}

func getTableNameToApproxRowCountMap(tableList []*sqlname.SourceName) map[string]int64 {
	tableNameToApproxRowCountMap := make(map[string]int64)
	for _, table := range tableList {
		tableNameToApproxRowCountMap[table.Qualified.Unquoted] = source.DB().GetTableApproxRowCount(table)
	}
	return tableNameToApproxRowCountMap
}

func filterTableWithEmptySupportedColumnList(finalTableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string) []*sqlname.SourceName {
	filteredTableList := lo.Reject(finalTableList, func(tableName *sqlname.SourceName, _ int) bool {
		return len(tablesColumnList[tableName]) == 0
	})
	return filteredTableList
}

func startFallBackSetupIfRequired() {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE {
		return
	}
	if !changeStreamingIsEnabled(exportType) {
		return
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch MigrationstatusRecord: %w", err)
	}
	if !msr.FallbackEnabled {
		utils.PrintAndLog("No fall-back enabled. Exiting.")
		return
	}

	cmd := []string{"yb-voyager", "fall-back", "setup",
		"--export-dir", exportDir,
		fmt.Sprintf("--send-diagnostics=%t", callhome.SendDiagnostics),
	}
	if utils.DoNotPrompt {
		cmd = append(cmd, "--yes")
	}
	if disablePb {
		cmd = append(cmd, "--disable-pb=true")
	}
	cmdStr := "SOURCE_DB_PASSWORD=*** " + strings.Join(cmd, " ")

	utils.PrintAndLog("Starting fall-back setup with command:\n %s", color.GreenString(cmdStr))
	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager - %w", err)
	}
	env := os.Environ()
	env = append(env, fmt.Sprintf("SOURCE_DB_PASSWORD=%s", source.Password))
	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager fall-back setup - %w\n Please re-run with command :\n%s", err, cmdStr)
	}
}

func dataIsExported() bool {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("check if schema is exported: load migration status record: %s", err)
	}

	return msr.ExportDataDone
}

func setDataIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportDataDone = true
	})
	if err != nil {
		utils.ErrExit("set data is exported: update migration status record: %s", err)
	}
}

func clearDataIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportDataDone = false
	})
	if err != nil {
		utils.ErrExit("clear data is exported: update migration status record: %s", err)
	}
}

func updateSourceDBConfInMSR() {
	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		if record.SourceDBConf == nil {
			record.SourceDBConf = source.Clone()
			record.SourceDBConf.Password = ""
		} else {
			// currently db type is only required for import data commands
			record.SourceDBConf.DBType = source.DBType
		}
	})
}
