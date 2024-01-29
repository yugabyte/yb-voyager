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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var exporterRole string = SOURCE_DB_EXPORTER_ROLE

var exportDataCmd = &cobra.Command{
	Use: "data",
	Short: "Export tables' data (either snapshot-only or snapshot-and-changes) from source database to export-dir. \nNote: For Oracle and MySQL, there is a beta feature to speed up the data export of snapshot, set the environment variable BETA_FAST_DATA_EXPORT=1 to try it out. You can refer to YB Voyager Documentation (https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#accelerate-data-export-for-mysql-and-oracle) for more details on this feature.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/data-migration/export-data/\n" +
		"Also export data(changes) from target Yugabyte DB in the fall-back/fall-forward workflows.",
	Long: ``,
	Args: cobra.NoArgs,

	PreRun: exportDataCommandPreRun,

	Run: exportDataCommandFn,
}

var exportDataFromCmd = &cobra.Command{
	Use:   "from",
	Short: `Export data from various databases`,
	Long:  ``,
}

var exportDataFromSrcCmd = &cobra.Command{
	Use:   "source",
	Short: exportDataCmd.Short,
	Long:  exportDataCmd.Long,
	Args:  exportDataCmd.Args,

	PreRun: exportDataCmd.PreRun,

	Run: exportDataCmd.Run,
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
	exportDataCmd.AddCommand(exportDataFromCmd)
	exportDataFromCmd.AddCommand(exportDataFromSrcCmd)

	registerCommonGlobalFlags(exportDataCmd)
	registerCommonGlobalFlags(exportDataFromSrcCmd)
	registerCommonExportFlags(exportDataCmd)
	registerCommonExportFlags(exportDataFromSrcCmd)
	registerSourceDBConnFlags(exportDataCmd, true)
	registerSourceDBConnFlags(exportDataFromSrcCmd, true)
	registerExportDataFlags(exportDataCmd)
	registerExportDataFlags(exportDataFromSrcCmd)
}

func exportDataCommandPreRun(cmd *cobra.Command, args []string) {
	setExportFlagsDefaults()
	err := validateExportFlags(cmd, exporterRole)
	if err != nil {
		utils.ErrExit("Error: %s", err.Error())
	}
	validateExportTypeFlag()
	markFlagsRequired(cmd)
	if changeStreamingIsEnabled(exportType) {
		useDebezium = true
	}
}

func exportDataCommandFn(cmd *cobra.Command, args []string) {
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	ExitIfAlreadyCutover(exporterRole)
	checkDataDirs()
	if useDebezium && !changeStreamingIsEnabled(exportType) {
		utils.PrintAndLog("Note: Beta feature to accelerate data export is enabled by setting BETA_FAST_DATA_EXPORT environment variable")
	}
	if changeStreamingIsEnabled(exportType) {
		utils.PrintAndLog(color.YellowString(`Note: Live migration is a TECH PREVIEW feature.`))
	}
	utils.PrintAndLog("export of data for source type as '%s'", source.DBType)
	sqlname.SourceDBType = source.DBType

	err := retrieveMigrationUUID()
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
		color.Green("Export of data complete \u2705")
		log.Info("Export of data completed.")
		startFallBackSetupIfRequired()
	} else if ProcessShutdownRequested {
		log.Info("Shutting down as SIGINT/SIGTERM received.")
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
	saveSourceDBConfInMSR()
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
		os.Exit(0)
	}

	if changeStreamingIsEnabled(exportType) || useDebezium {
		config, tableNametoApproxRowCountMap, err := prepareDebeziumConfig(finalTableList, tablesColumnList)
		if err != nil {
			log.Errorf("Failed to prepare dbzm config: %v", err)
			return false
		}
		if source.DBType == POSTGRESQL && changeStreamingIsEnabled(exportType) {
			// pg live migration. Steps are as follows:
			// 1. create replication slot.
			// 2. export snapshot corresponding to replication slot by passing it to pg_dump
			// 3. start debezium with configration to read changes from the created replication slot.

			if !dataIsExported() { // if snapshot is not already done...
				err = exportPGSnapshotWithPGdump(ctx, cancel, finalTableList, tablesColumnList)
				if err != nil {
					log.Errorf("export snapshot failed: %v", err)
					return false
				}

				// updating the MSR with PGPublicationName
				err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
					PGPublicationName := "voyager_dbz_publication_" + strings.ReplaceAll(migrationUUID.String(), "-", "_")
					record.PGPublicationName = PGPublicationName
				})
				if err != nil {
					log.Errorf("update PGPublicationName: update migration status record: %v", err)
					return false
				}
			}

			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				utils.ErrExit("get migration status record: %v", err)
			}

			// Setting up sequence values for debezium to start tracking from..
			sequenceValueMap, err := getPGDumpSequencesAndValues()
			if err != nil {
				utils.ErrExit("get pg dump sequence values: %v", err)
			}

			var sequenceInitValues strings.Builder
			for seqName, seqValue := range sequenceValueMap {
				sequenceInitValues.WriteString(fmt.Sprintf("%s:%d,", seqName.Qualified.Quoted, seqValue))
			}

			config.SnapshotMode = "never"
			config.ReplicationSlotName = msr.PGReplicationSlotName
			config.PublicationName = msr.PGPublicationName
			config.InitSequenceMaxMapping = sequenceInitValues.String()
		}

		err = debeziumExportData(ctx, config, tableNametoApproxRowCountMap)
		if err != nil {
			log.Errorf("Export Data using debezium failed: %v", err)
			return false
		}

		if changeStreamingIsEnabled(exportType) {
			log.Infof("live migration complete, proceeding to cutover")
			if isTargetDBExporter(exporterRole) {
				err = ybCDCClient.DeleteStreamID()
				if err != nil {
					utils.ErrExit("failed to delete stream id after data export: %v", err)
				}
			}
			if exporterRole == SOURCE_DB_EXPORTER_ROLE {
				msr, err := metaDB.GetMigrationStatusRecord()
				if err != nil {
					utils.ErrExit("get migration status record: %v", err)
				}
				deletePGReplicationSlot(msr, &source)
				deletePGPublication(msr, &source)
			}

			// mark cutover processed only after cleanup like deleting replication slot and yb cdc stream id
			err = markCutoverProcessed(exporterRole)
			if err != nil {
				utils.ErrExit("failed to create trigger file after data export: %v", err)
			}
			utils.PrintAndLog("\nRun the following command to get the current report of the migration:\n" +
				color.CyanString("yb-voyager get data-migration-report --export-dir %q\n", exportDir))
		}
		return true
	} else {
		minQuotedTableList := lo.Map(finalTableList, func(table *sqlname.SourceName, _ int) string {
			return table.Qualified.MinQuoted //Case sensitivity
		})
		err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.TableListExportedFromSource = minQuotedTableList
		})
		if err != nil {
			utils.ErrExit("update table list exported from source: update migration status record: %s", err)
		}
		fmt.Printf("num tables to export: %d\n", len(finalTableList))
		utils.PrintAndLog("table list for data export: %v", finalTableList)
		err = exportDataOffline(ctx, cancel, finalTableList, tablesColumnList, "")
		if err != nil {
			log.Errorf("Export Data failed: %v", err)
			return false
		}
		return true
	}

}

func exportPGSnapshotWithPGdump(ctx context.Context, cancel context.CancelFunc, finalTableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string) error {
	// create replication slot
	pgDB := source.DB().(*srcdb.PostgreSQL)
	replicationConn, err := pgDB.GetReplicationConnection()
	if err != nil {
		return fmt.Errorf("export snapshot: failed to create replication connection: %v", err)
	}
	// need to keep the replication connection open until snapshot is complete.
	defer func() {
		err := replicationConn.Close(context.Background())
		if err != nil {
			log.Errorf("close replication connection: %v", err)
		}
	}()
	res, err := pgDB.CreateLogicalReplicationSlot(replicationConn, migrationUUID, true)
	if err != nil {
		return fmt.Errorf("export snapshot: failed to create replication slot: %v", err)
	}
	// pg_dump
	err = exportDataOffline(ctx, cancel, finalTableList, tablesColumnList, res.SnapshotName)
	if err != nil {
		log.Errorf("Export Data failed: %v", err)
		return err
	}

	// save replication slot in MSR
	err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.PGReplicationSlotName = res.SlotName
		record.SnapshotMechanism = "pg_dump"
	})
	if err != nil {
		utils.ErrExit("update PGReplicationSlotName: update migration status record: %s", err)
	}
	setDataIsExported()
	return nil
}

func getPGDumpSequencesAndValues() (map[*sqlname.SourceName]int64, error) {
	result := map[*sqlname.SourceName]int64{}
	path := filepath.Join(exportDir, "data", "postdata.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		utils.ErrExit("read file %q: %v", path, err)
	}

	lines := strings.Split(string(data), "\n")
	// Sample line: SELECT pg_catalog.setval('public.actor_actor_id_seq', 200, true);
	setvalRegex := regexp.MustCompile(`(?i)SELECT.*setval\((?P<args>.*)\)`)

	for _, line := range lines {
		matches := setvalRegex.FindStringSubmatch(line)
		if len(matches) == 0 {
			continue
		}
		argsIdx := setvalRegex.SubexpIndex("args")
		if argsIdx > len(matches) {
			return nil, fmt.Errorf("invalid index %d for matches - %s for line %s", argsIdx, matches, line)
		}
		args := strings.Split(matches[argsIdx], ",")

		seqNameRaw := args[0][1 : len(args[0])-1]
		seqName := sqlname.NewSourceNameFromQualifiedName(seqNameRaw)

		seqVal, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %s to int in line - %s: %v", args[1], line, err)
		}

		result[seqName] = seqVal
	}
	return result, nil
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

func exportDataOffline(ctx context.Context, cancel context.CancelFunc, finalTableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string, snapshotName string) error {
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

	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch source.DBType {
		case POSTGRESQL:
			record.SnapshotMechanism = "pg_dump"
		case ORACLE, MYSQL:
			record.SnapshotMechanism = "ora2pg"
		}
	})
	if err != nil {
		utils.ErrExit("update PGReplicationSlotName: update migration status record: %s", err)
	}

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
	go source.DB().ExportData(ctx, exportDir, finalTableList, quitChan, exportDataStart, exportSuccessChan, tablesColumnList, snapshotName)
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
	displayExportedRowCountSnapshot(false)
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
	exportSnapshotStatusFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportSnapshotStatusFile := jsonfile.NewJsonFile[ExportSnapshotStatus](exportSnapshotStatusFilePath)
	dfdFilePath := exportDir + datafile.DESCRIPTOR_PATH
	if startClean {
		utils.CleanDir(exportDataDir)
		utils.CleanDir(sslDir)
		clearDataIsExported()
		err := os.Remove(dfdFilePath)
		if err != nil && !os.IsNotExist(err) {
			utils.ErrExit("Failed to remove data file descriptor: %s", err)
		}
		err = os.Remove(propertiesFilePath)
		if err != nil && !os.IsNotExist(err) {
			utils.ErrExit("Failed to remove properties file: %s", err)
		}
		err = exportSnapshotStatusFile.Delete()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			utils.ErrExit("Failed to remove export snapshot status file: %s", err)
		}

		err = metadb.TruncateTablesInMetaDb(exportDir, []string{metadb.QUEUE_SEGMENT_META_TABLE_NAME, metadb.EXPORTED_EVENTS_STATS_TABLE_NAME, metadb.EXPORTED_EVENTS_STATS_PER_TABLE_TABLE_NAME})
		if err != nil {
			utils.ErrExit("Failed to truncate tables in metadb: %s", err)
		}
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
		return getDefaultPGSchema(source.Schema)
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
	return lo.UniqBy(result, func(tableName *sqlname.SourceName) string {
		return tableName.Qualified.MinQuoted
	})
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
		return
	}

	lockFile.Unlock() // unlock export dir from export data cmd before switching current process to fall-back setup cmd
	cmd := []string{"yb-voyager", "import", "data", "to", "source",
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

	utils.PrintAndLog("Starting import data to target with command:\n %s", color.GreenString(cmdStr))
	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager - %w", err)
	}
	env := os.Environ()
	found := false
	for i, e := range env {
		if strings.HasPrefix(e, "SOURCE_DB_PASSWORD=") {
			env[i] = "SOURCE_DB_PASSWORD=" + source.Password
			found = true
			break
		}
	}
	if !found {
		env = append(env, fmt.Sprintf("SOURCE_DB_PASSWORD=%s", source.Password))
	}
	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager import data to target - %w\n Please re-run with command :\n%s", err, cmdStr)
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

func saveSourceDBConfInMSR() {
	if exporterRole != SOURCE_DB_EXPORTER_ROLE {
		return
	}
	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		// overriding the current value of SourceDBConf
		record.SourceDBConf = source.Clone()
		record.SourceDBConf.Password = ""
		record.SourceDBConf.Uri = ""
	})
}
