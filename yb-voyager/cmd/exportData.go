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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/gosuri/uilive"
	"github.com/magiconair/properties"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var exportDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This command is used to export table's data from source database to *.sql files \nNote: For Oracle and MySQL, there is a beta feature to speed up the data export, set the environment variable BETA_FAST_DATA_EXPORT=1 to try it out. You can refer to YB Voyager Documentation (https://docs.yugabyte.com/preview/migrate/migrate-steps/#export-data) for more details on this feature.",
	Long:  ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		setExportFlagsDefaults()
		validateExportFlags(cmd)
		validateExportTypeFlag()
		markFlagsRequired(cmd)
		if changeStreamingIsEnabled(exportType) {
			useDebezium = true
		}
		err := visualizerDB.CreateYugabytedTableMetricsTable()
		if err != nil {
			log.Warnf("Failed to create table metrics table for visualization. %s", err)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		checkDataDirs()
		exportData()
	},
}

func init() {
	exportCmd.AddCommand(exportDataCmd)
	registerCommonGlobalFlags(exportDataCmd)
	registerCommonExportFlags(exportDataCmd)
	exportDataCmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - disable progress bar during data export(default false)")

	exportDataCmd.Flags().StringVar(&source.ExcludeTableList, "exclude-table-list", "",
		"list of tables to exclude while exporting data (ignored if --table-list is used)")

	exportDataCmd.Flags().StringVar(&source.TableList, "table-list", "",
		"list of the tables to export data")

	exportDataCmd.Flags().IntVar(&source.NumConnections, "parallel-jobs", 4,
		"number of Parallel Jobs to extract data from source database")

	exportDataCmd.Flags().StringVar(&exportType, "export-type", SNAPSHOT_ONLY,
		fmt.Sprintf("export type: %s, %s, %s", SNAPSHOT_ONLY, CHANGES_ONLY, SNAPSHOT_AND_CHANGES))
}

func exportData() {
	if useDebezium {
		utils.PrintAndLog("Note: Beta feature to accelerate data export is enabled by setting BETA_FAST_DATA_EXPORT environment variable")
	}
	utils.PrintAndLog("export of data for source type as '%s'", source.DBType)
	sqlname.SourceDBType = source.DBType
	err := retrieveMigrationUUID(exportDir)
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}

	// Send 'IN PROGRESS' metadata for `EXPORT DATA` step
	utils.WaitGroup.Add(1)
	go createAndSendVisualizerPayload("EXPORT DATA", "IN PROGRESS", "")

	success := exportDataOffline()
	if success {
		tableRowCount := map[string]int64{}
		for _, fileEntry := range datafile.OpenDescriptor(exportDir).DataFileList {
			tableRowCount[fileEntry.TableName] += fileEntry.RowCount
		}
		printExportedRowCount(tableRowCount, useDebezium)
		callhome.GetPayload(exportDir, migrationUUID)
		callhome.UpdateDataStats(exportDir, tableRowCount)
		callhome.PackAndSendPayload(exportDir)

		createExportDataDoneFlag()
		color.Green("Export of data complete \u2705")
		log.Info("Export of data completed.")
	} else {
		color.Red("Export of data failed! Check %s/logs for more details. \u274C", exportDir)
		log.Error("Export of data failed.")
		atexit.Exit(1)
	}

	// Send 'COMPLETED' metadata for `EXPORT DATA` step
	utils.WaitGroup.Add(1)
	go createAndSendVisualizerPayload("EXPORT DATA", "COMPLETED", "")

	// Wait till the visualisation metadata is sent
	utils.WaitGroup.Wait()
}

func exportDataOffline() bool {
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the source db: %s", err)
	}
	defer source.DB().Disconnect()
	checkSourceDBCharset()
	source.DB().CheckRequiredToolsAreInstalled()

	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	metaDB, err = NewMetaDB(exportDir)
	if err != nil {
		utils.ErrExit("Failed to initialize meta db: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var tableList []*sqlname.SourceName
	// store table list after filtering unsupported or unnecessary tables
	var finalTableList, skippedTableList []*sqlname.SourceName
	excludeTableList := extractTableListFromString(source.ExcludeTableList)
	if source.TableList != "" {
		finalTableList = extractTableListFromString(source.TableList)
	} else {
		tableList = source.DB().GetAllTableNames()
		finalTableList = sqlname.SetDifference(tableList, excludeTableList)
		log.Infof("initial all tables table list for data export: %v", tableList)

		finalTableList, skippedTableList = source.DB().FilterEmptyTables(finalTableList)
		if len(skippedTableList) != 0 {
			utils.PrintAndLog("skipping empty tables: %v", skippedTableList)
		}

		finalTableList, skippedTableList = source.DB().FilterUnsupportedTables(finalTableList, useDebezium)
		if len(skippedTableList) != 0 {
			utils.PrintAndLog("skipping unsupported tables: %v", skippedTableList)
		}
	}

	tablesColumnList, unsupportedColumnNames := source.DB().GetColumnsWithSupportedTypes(finalTableList, useDebezium)
	if len(unsupportedColumnNames) > 0 {
		log.Infof("preparing column list for the data export without unsupported datatype columns: %v", unsupportedColumnNames)
		if !utils.AskPrompt("\nThe following columns data export is unsupported:\n" + strings.Join(unsupportedColumnNames, "\n") +
			"\nDo you want to ignore just these columns' data and continue with export") {
			utils.ErrExit("Exiting at user's request. Use `--exclude-table-list` flag to continue without these tables")
		}
		finalTableList = filterTableWithEmptySupportedColumnList(finalTableList, tablesColumnList)
	}

	if len(finalTableList) == 0 {
		fmt.Println("no tables present to export, exiting...")
		createExportDataDoneFlag()
		dfd := datafile.Descriptor{
			ExportDir:    exportDir,
			DataFileList: make([]*datafile.FileEntry, 0),
		}
		dfd.Save()
		os.Exit(0)
	}

	if changeStreamingIsEnabled(exportType) || useDebezium {
		finalTableList = filterTablePartitions(finalTableList)
		fmt.Printf("num tables to export: %d\n", len(finalTableList))
		utils.PrintAndLog("table list for data export: %v", finalTableList)
		err := debeziumExportData(ctx, finalTableList, tablesColumnList)
		if err != nil {
			log.Errorf("Export Data using debezium failed: %v", err)
			return false
		}
		return true
	}

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
				"Check %s/logs/yb-voyager.log for more details.", exportDir)
		}
	}()

	initializeExportTableMetadata(finalTableList)

	log.Infof("Export table metadata: %s", spew.Sdump(tablesProgressMetadata))
	UpdateTableApproxRowCount(&source, exportDir, tablesProgressMetadata)

	if source.DBType == POSTGRESQL {
		//need to export setval() calls to resume sequence value generation
		sequenceList := source.DB().GetAllSequences()
		for _, seq := range sequenceList {
			name := sqlname.NewSourceNameFromMaybeQualifiedName(seq, "public")
			finalTableList = append(finalTableList, name)
		}
	}
	fmt.Printf("Initiating data export.\n")
	utils.WaitGroup.Add(1)
	go source.DB().ExportData(ctx, exportDir, finalTableList, quitChan, exportDataStart, exportSuccessChan, tablesColumnList)
	// Wait for the export data to start.
	<-exportDataStart

	// Create initial entry for each table in the table metrics table
	utils.WaitGroup.Add(1)
	go createAndSendVisualizerExportTableMetrics(utils.GetSortedKeys(tablesProgressMetadata))

	updateFilePaths(&source, exportDir, tablesProgressMetadata)
	utils.WaitGroup.Add(1)
	exportDataStatus(ctx, tablesProgressMetadata, quitChan, exportSuccessChan, disablePb)

	utils.WaitGroup.Wait() // waiting for the dump and progress bars to complete
	if ctx.Err() != nil {
		fmt.Printf("ctx error(exportData.go): %v\n", ctx.Err())
		return false
	}

	source.DB().ExportDataPostProcessing(exportDir, tablesProgressMetadata)
	return true
}

func debeziumExportData(ctx context.Context, tableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string) error {
	runId = time.Now().String()
	absExportDir, err := filepath.Abs(exportDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for export dir: %v", err)
	}

	var snapshotMode string

	switch exportType {
	case SNAPSHOT_AND_CHANGES:
		snapshotMode = "initial"
	case CHANGES_ONLY:
		snapshotMode = "never"
	case SNAPSHOT_ONLY:
		snapshotMode = "initial_only"
	default:
		return fmt.Errorf("invalid export type %s", exportType)
	}

	var dbzmTableList, dbzmColumnList []string
	for _, table := range tableList {
		dbzmTableList = append(dbzmTableList, table.Qualified.Unquoted)
	}

	for tableName, columns := range tablesColumnList {
		for _, column := range columns {
			columnName := fmt.Sprintf("%s.%s", tableName.Qualified.Unquoted, column)
			if column == "*" {
				dbzmColumnList = append(dbzmColumnList, columnName) //for all columns <schema>.<table>.*
				break
			}
			dbzmColumnList = append(dbzmColumnList, columnName) // if column is PK, then data for it will come from debezium
		}
	}

	var columnSequenceMap []string
	colToSeqMap := source.DB().GetColumnToSequenceMap(tableList)
	for column, sequence := range colToSeqMap {
		columnSequenceMap = append(columnSequenceMap, fmt.Sprintf("%s:%s", column, sequence))
	}
	err = source.PrepareSSLParamsForDebezium(absExportDir)
	if err != nil {
		return fmt.Errorf("failed to prepare ssl params for debezium: %w", err)
	}

	config := &dbzm.Config{
		RunId:          runId,
		SourceDBType:   source.DBType,
		ExportDir:      absExportDir,
		MetadataDBPath: getMetaDBPath(absExportDir),
		Host:           source.Host,
		Port:           source.Port,
		Username:       source.User,
		Password:       source.Password,

		DatabaseName:      source.DBName,
		SchemaNames:       source.Schema,
		TableList:         dbzmTableList,
		ColumnList:        dbzmColumnList,
		ColumnSequenceMap: columnSequenceMap,

		SSLMode:               source.SSLMode,
		SSLCertPath:           source.SSLCertPath,
		SSLKey:                source.SSLKey,
		SSLRootCert:           source.SSLRootCert,
		SSLKeyStore:           source.SSLKeyStore,
		SSLKeyStorePassword:   source.SSLKeyStorePassword,
		SSLTrustStore:         source.SSLTrustStore,
		SSLTrustStorePassword: source.SSLTrustStorePassword,
		SnapshotMode:          snapshotMode,
	}
	if source.DBType == "oracle" {
		jdbcConnectionStringPrefix := "jdbc:oracle:thin:@"
		if source.IsOracleCDBSetup() {
			// uri = cdb uri
			connectionString := srcdb.GetOracleConnectionString(source.Host, source.Port, source.CDBName, source.CDBSid, source.CDBTNSAlias)
			config.Uri = fmt.Sprintf("%s%s", jdbcConnectionStringPrefix, connectionString)
			config.PDBName = source.DBName
		} else {
			connectionString := srcdb.GetOracleConnectionString(source.Host, source.Port, source.DBName, source.DBSid, source.TNSAlias)
			config.Uri = fmt.Sprintf("%s%s", jdbcConnectionStringPrefix, connectionString)
		}

		config.TNSAdmin, err = getTNSAdmin(source)
		if err != nil {
			return fmt.Errorf("failed to get tns admin: %w", err)
		}
		config.OracleJDBCWalletLocationSet, err = isOracleJDBCWalletLocationSet(source)
		if err != nil {
			return fmt.Errorf("failed to determine if Oracle JDBC wallet location is set: %v", err)
		}
	} else if source.DBType == "yugabytedb" {
		if exportType == CHANGES_ONLY {
			ybServers := source.DB().GetServers()
			ybCDCClient := dbzm.NewYugabyteDBCDCClient(exportDir, ybServers, config.SSLRootCert, config.DatabaseName, config.TableList[0])
			err := ybCDCClient.Init()
			if err != nil {
				return fmt.Errorf("failed to initialize YugabyteDB CDC client: %w", err)
			}
			config.YBMasterNodes, err = ybCDCClient.ListMastersNodes()
			if err != nil {
				return fmt.Errorf("failed to list master nodes: %w", err)
			}
			if startClean {
				err = ybCDCClient.DeleteStreamID()
				if err != nil {
					return fmt.Errorf("failed to delete stream id: %w", err)
				}
				config.YBStreamID, err = ybCDCClient.GenerateAndStoreStreamID()
				if err != nil {
					return fmt.Errorf("failed to generate stream id: %w", err)
				}
				utils.PrintAndLog("Generated new YugabyteDB CDC stream-id: %s", config.YBStreamID)
			} else {
				config.YBStreamID, err = ybCDCClient.GetStreamID()
				if err != nil {
					return fmt.Errorf("failed to get stream id: %w", err)
				}
			}
		}
	}

	tableNameToApproxRowCountMap := getTableNameToApproxRowCountMap(tableList)
	progressTracker := NewProgressTracker(tableNameToApproxRowCountMap)
	debezium := dbzm.NewDebezium(config)
	err = debezium.Start()
	if err != nil {
		return fmt.Errorf("failed to start debezium: %w", err)
	}

	var status *dbzm.ExportStatus
	snapshotComplete := false
	for debezium.IsRunning() {
		status, err = debezium.GetExportStatus()
		if err != nil {
			return fmt.Errorf("failed to read export status: %w", err)
		}
		if status == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		progressTracker.UpdateProgress(status)
		if !snapshotComplete {
			snapshotComplete, err = checkAndHandleSnapshotComplete(status, progressTracker)
			if err != nil {
				return fmt.Errorf("failed to check if snapshot is complete: %w", err)
			}
		}
		time.Sleep(time.Millisecond * 500)
	}
	if err := debezium.Error(); err != nil {
		return fmt.Errorf("debezium failed with error: %w", err)
	}
	// handle case where debezium finished before snapshot completion
	// was handled in above loop
	if !snapshotComplete {
		status, err = debezium.GetExportStatus()
		if err != nil {
			return fmt.Errorf("failed to read export status: %w", err)
		}
		snapshotComplete, err = checkAndHandleSnapshotComplete(status, progressTracker)
		if !snapshotComplete || err != nil {
			return fmt.Errorf("snapshot was not completed: %w", err)
		}
	}

	log.Info("Debezium exited normally.")
	return nil
}

func reportStreamingProgress() {
	tableWriter := uilive.New()
	headerWriter := tableWriter.Newline()
	separatorWriter := tableWriter.Newline()
	row1Writer := tableWriter.Newline()
	row2Writer := tableWriter.Newline()
	row3Writer := tableWriter.Newline()
	row4Writer := tableWriter.Newline()
	footerWriter := tableWriter.Newline()
	tableWriter.Start()
	for {
		totalEventCount, totalEventCountRun, err := metaDB.GetTotalExportedEvents(runId)
		if err != nil {
			utils.ErrExit("failed to get total exported count from metadb: %w", err)
		}
		throughputInLast3Min, err := metaDB.GetExportedEventsRateInLastNMinutes(runId, 3)
		if err != nil {
			utils.ErrExit("failed to get export rate from metadb: %w", err)
		}
		throughputInLast10Min, err := metaDB.GetExportedEventsRateInLastNMinutes(runId, 10)
		if err != nil {
			utils.ErrExit("failed to get export rate from metadb: %w", err)
		}
		fmt.Fprint(tableWriter, color.GreenString("| %-40s | %30s |\n", "---------------------------------------", "-----------------------------"))
		fmt.Fprint(headerWriter, color.GreenString("| %-40s | %30s |\n", "Metric", "Value"))
		fmt.Fprint(separatorWriter, color.GreenString("| %-40s | %30s |\n", "---------------------------------------", "-----------------------------"))
		fmt.Fprint(row1Writer, color.GreenString("| %-40s | %30s |\n", "Total Exported Events", strconv.FormatInt(totalEventCount, 10)))
		fmt.Fprint(row2Writer, color.GreenString("| %-40s | %30s |\n", "Total Exported Events (Current Run)", strconv.FormatInt(totalEventCountRun, 10)))
		fmt.Fprint(row3Writer, color.GreenString("| %-40s | %30s |\n", "Export Rate(Last 3 min)", strconv.FormatInt(throughputInLast3Min, 10)+"/sec"))
		fmt.Fprint(row4Writer, color.GreenString("| %-40s | %30s |\n", "Export Rate(Last 10 min)", strconv.FormatInt(throughputInLast10Min, 10)+"/sec"))
		fmt.Fprint(footerWriter, color.GreenString("| %-40s | %30s |\n", "---------------------------------------", "-----------------------------"))
		tableWriter.Flush()
		time.Sleep(30 * time.Second)
	}
}

// oracle wallet location can be optionally set in $TNS_ADMIN/ojdbc.properties as
// oracle.net.wallet_location=<>
func isOracleJDBCWalletLocationSet(s srcdb.Source) (bool, error) {
	if s.DBType != "oracle" {
		return false, fmt.Errorf("invalid source db type %s for checking jdbc wallet location", s.DBType)
	}
	tnsAdmin, err := getTNSAdmin(s)
	if err != nil {
		return false, fmt.Errorf("failed to get tns admin: %w", err)
	}
	ojdbcPropertiesFilePath := filepath.Join(tnsAdmin, "ojdbc.properties")
	if _, err := os.Stat(ojdbcPropertiesFilePath); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		return false, nil
	}
	ojdbcProperties := properties.MustLoadFile(ojdbcPropertiesFilePath, properties.UTF8)
	walletLocationKey := "oracle.net.wallet_location"
	_, present := ojdbcProperties.Get(walletLocationKey)
	return present, nil
}

// https://www.orafaq.com/wiki/TNS_ADMIN
// default is $ORACLE_HOME/network/admin
func getTNSAdmin(s srcdb.Source) (string, error) {
	if s.DBType != "oracle" {
		return "", fmt.Errorf("invalid source db type %s for getting TNS_ADMIN", s.DBType)
	}
	tnsAdminEnvVar, present := os.LookupEnv("TNS_ADMIN")
	if present {
		return tnsAdminEnvVar, nil
	} else {
		return filepath.Join(s.GetOracleHome(), "network", "admin"), nil
	}
}

func filterTableWithEmptySupportedColumnList(finalTableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string) []*sqlname.SourceName {
	var filteredTableList []*sqlname.SourceName
	for _, table := range finalTableList {
		if len(tablesColumnList[table]) == 0 {
			continue
		}
		filteredTableList = append(filteredTableList, table)
	}
	return filteredTableList
}

func checkAndHandleSnapshotComplete(status *dbzm.ExportStatus, progressTracker *ProgressTracker) (bool, error) {
	if !status.SnapshotExportIsComplete() {
		return false, nil
	}
	progressTracker.Done(status)
	createExportDataDoneFlag()
	err := writeDataFileDescriptor(exportDir, status)
	if err != nil {
		return false, fmt.Errorf("failed to write data file descriptor: %w", err)
	}
	log.Infof("snapshot export is complete.")
	err = renameDbzmExportedDataFiles()
	if err != nil {
		return false, fmt.Errorf("failed to rename dbzm exported data files: %v", err)
	}
	if changeStreamingIsEnabled(exportType) {
		color.Blue("streaming changes to a local queue file...")
		if !disablePb {
			go reportStreamingProgress()
		}
	}
	return true, nil
}
func getTableNameToApproxRowCountMap(tableList []*sqlname.SourceName) map[string]int64 {
	tableNameToApproxRowCountMap := make(map[string]int64)
	for _, table := range tableList {
		tableNameToApproxRowCountMap[table.Qualified.Unquoted] = source.DB().GetTableApproxRowCount(table)
	}
	return tableNameToApproxRowCountMap
}

// required only for postgresql since GetAllTables() returns all tables and partitions
func filterTablePartitions(tableList []*sqlname.SourceName) []*sqlname.SourceName {
	if source.DBType != POSTGRESQL || source.TableList != "" {
		return tableList
	}

	filteredTableList := []*sqlname.SourceName{}
	for _, table := range tableList {
		if !source.DB().IsTablePartition(table) {
			filteredTableList = append(filteredTableList, table)
		}
	}
	return filteredTableList
}

func writeDataFileDescriptor(exportDir string, status *dbzm.ExportStatus) error {
	dataFileList := make([]*datafile.FileEntry, 0)
	for _, table := range status.Tables {
		// TODO: TableName and FilePath must be quoted by debezium plugin.
		tableName := quoteIdentifierIfRequired(table.TableName)
		if source.DBType == POSTGRESQL && table.SchemaName != "public" {
			tableName = fmt.Sprintf("%s.%s", table.SchemaName, tableName)
		}
		fileEntry := &datafile.FileEntry{
			TableName: tableName,
			FilePath:  fmt.Sprintf("%s_data.sql", tableName),
			RowCount:  table.ExportedRowCountSnapshot,
			FileSize:  -1, // Not available.
		}
		dataFileList = append(dataFileList, fileEntry)
	}
	dfd := datafile.Descriptor{
		FileFormat:   datafile.TEXT,
		Delimiter:    "\t",
		HasHeader:    true,
		ExportDir:    exportDir,
		DataFileList: dataFileList,
	}
	dfd.Save()
	return nil
}

// handle renaming for tables having case sensitivity and reserved keywords
func renameDbzmExportedDataFiles() error {
	status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
	if err != nil {
		return fmt.Errorf("failed to read export status during renaming exported data files: %w", err)
	}
	if status == nil {
		return fmt.Errorf("export status is empty during renaming exported data files")
	}

	for i := 0; i < len(status.Tables); i++ {
		tableName := status.Tables[i].TableName
		// either case sensitive(postgresql) or reserved keyword(any source db)
		if (!sqlname.IsAllLowercase(tableName) && source.DBType == POSTGRESQL) ||
			sqlname.IsReservedKeywordPG(tableName) {
			tableName = fmt.Sprintf("\"%s\"", status.Tables[i].TableName)
		}

		oldFilePath := filepath.Join(exportDir, "data", status.Tables[i].FileName)
		newFilePath := filepath.Join(exportDir, "data", tableName+"_data.sql")
		if status.Tables[i].SchemaName != "public" && source.DBType == POSTGRESQL {
			newFilePath = filepath.Join(exportDir, "data", status.Tables[i].SchemaName+"."+tableName+"_data.sql")
		}

		log.Infof("Renaming %s to %s", oldFilePath, newFilePath)
		err = os.Rename(oldFilePath, newFilePath)
		if err != nil {
			return fmt.Errorf("failed to rename dbzm exported data file: %w", err)
		}

		//rename table schema file as well
		oldTableSchemaFilePath := filepath.Join(exportDir, "data", "schemas", strings.Replace(status.Tables[i].FileName, "_data.sql", "_schema.json", 1))
		newTableSchemaFilePath := filepath.Join(exportDir, "data", "schemas", tableName+"_schema.json")
		if status.Tables[i].SchemaName != "public" && source.DBType == POSTGRESQL {
			newTableSchemaFilePath = filepath.Join(exportDir, "data", "schemas", status.Tables[i].SchemaName+"."+tableName+"_schema.json")
		}
		log.Infof("Renaming %s to %s", oldTableSchemaFilePath, newTableSchemaFilePath)
		err = os.Rename(oldTableSchemaFilePath, newTableSchemaFilePath)
		if err != nil {
			return fmt.Errorf("failed to rename dbzm exported table schema file: %w", err)
		}
	}
	return nil
}

// flagName can be "exclude-table-list" or "table-list"
func validateTableListFlag(tableListString string, flagName string) {
	if tableListString == "" {
		return
	}
	tableList := utils.CsvStringToSlice(tableListString)
	// TODO: update regexp once table name with double quotes are allowed/supported
	tableNameRegex := regexp.MustCompile("[a-zA-Z0-9_.]+")
	for _, table := range tableList {
		if !tableNameRegex.MatchString(table) {
			utils.ErrExit("Error: Invalid table name '%v' provided wtih --%s flag", table, flagName)
		}
	}
}

func checkDataDirs() {
	exportDataDir := filepath.Join(exportDir, "data")
	flagFilePath := filepath.Join(exportDir, "metainfo", "flags", "exportDataDone")
	propertiesFilePath := filepath.Join(exportDir, "metainfo", "conf", "application.properties")
	sslDir := filepath.Join(exportDir, "metainfo", "ssl")
	dfdFilePath := exportDir + datafile.DESCRIPTOR_PATH
	if startClean {
		utils.CleanDir(exportDataDir)
		utils.CleanDir(sslDir)
		os.Remove(flagFilePath)
		os.Remove(dfdFilePath)
		os.Remove(propertiesFilePath)
		truncateTablesInMetaDb(exportDir, []string{QUEUE_SEGMENT_META_TABLE_NAME, EXPORTED_EVENTS_STATS_TABLE_NAME, EXPORTED_EVENTS_STATS_PER_TABLE_TABLE_NAME})
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

func getDefaultSourceSchemaName() string {
	switch source.DBType {
	case MYSQL:
		return source.DBName
	case POSTGRESQL:
		return "public"
	case ORACLE:
		return source.Schema
	default:
		panic("invalid db type")
	}
}

func extractTableListFromString(flagTableList string) []*sqlname.SourceName {
	result := []*sqlname.SourceName{}
	if flagTableList == "" {
		return result
	}
	tableList := utils.CsvStringToSlice(flagTableList)
	defaultSourceSchemaName := getDefaultSourceSchemaName()
	for _, table := range tableList {
		result = append(result, sqlname.NewSourceNameFromMaybeQualifiedName(table, defaultSourceSchemaName))
	}
	return result
}

func createExportDataDoneFlag() {
	exportDoneFlagPath := filepath.Join(exportDir, "metainfo", "flags", "exportDataDone")
	_, err := os.Create(exportDoneFlagPath)
	if err != nil {
		utils.ErrExit("creating exportDataDone flag: %v", err)
	}
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
