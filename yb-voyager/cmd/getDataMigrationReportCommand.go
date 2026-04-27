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
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var sourceReplicaDbPassword string
var sourceDbPassword string
var nameRegistryForSourceReplicaRole *namereg.NameRegistry

var includeDetailedIterationsStats utils.BoolStr

var getDataMigrationReportCmd = &cobra.Command{
	Use:   "data-migration-report",
	Short: "Print the consolidated report of migration of data.",
	Long:  `Print the consolidated report of migration of data among different DBs (source / target / source-replica) when export-type 'snapshot-and-changes' is enabled.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		validateReportOutputFormat(migrationReportFormats, reportOrStatusCmdOutputFormat)
	},
	Run: func(cmd *cobra.Command, args []string) {
		migrationStatus, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error while getting migration status: %w\n", err)
		}
		if migrationStatus.LatestIterationNumber == 0 {
			if includeDetailedIterationsStats {
				utils.ErrExit("Error: Detailed report is only applicable for multiple iterations of Live migration with fallback workflow")
			}
		}
		if migrationStatus.FallForwardEnabled {
			if includeDetailedIterationsStats {
				utils.ErrExit("Error: Detailed report is only applicable for multiple iterations of Live migration with fallback workflow")
			}
		}
		streamChanges, err := checkStreamingMode()
		if err != nil {
			utils.ErrExit("error while checking streaming mode: %w\n", err)
		}
		migrationUUID, err = uuid.Parse(migrationStatus.MigrationUUID)
		if err != nil {
			utils.ErrExit("error while parsing migration UUID: %w\n", err)
		}
		if streamChanges {
			if migrationStatus.TargetDBConf != nil {
				getTargetPassword(cmd)
				migrationStatus.TargetDBConf.Password = tconf.Password
				targetDBPassword = tconf.Password
			}
			if migrationStatus.FallForwardEnabled {
				getSourceReplicaDBPassword(cmd)
				migrationStatus.SourceReplicaDBConf.Password = tconf.Password
			}
			if migrationStatus.FallbackEnabled {
				getSourceDBPassword(cmd)
				migrationStatus.SourceDBAsTargetConf.Password = tconf.Password
				sourceDbPassword = tconf.Password
			}
			err = InitNameRegistry(exportDir, "", nil, nil, nil, nil, false)
			if err != nil {
				utils.ErrExit("initializing name registry: %w", err)
			}
			color.Yellow("Generating data migration report for migration UUID: %s...\n", migrationStatus.MigrationUUID)
			getDataMigrationReportCmdFn(migrationStatus, false, true)
		} else {
			utils.ErrExit("Error: Data migration report is only applicable when export-type is 'snapshot-and-changes'(live migration)\nPlease run export data status/import data status commands.")
		}
	},
}

type RowData struct {
	TableName                   string `json:"table_name"`
	DBType                      string `json:"db_type"`
	IterationNumber             int    `json:"iteration_number,omitempty"`
	ExportedSnapshotRows        int64  `json:"exported_snapshot_rows"`
	ImportedSnapshotRows        int64  `json:"imported_snapshot_rows"`
	ErroredImportedSnapshotRows int64  `json:"errored_imported_snapshot_rows"`
	ImportedInserts             int64  `json:"imported_inserts"`
	ImportedUpdates             int64  `json:"imported_updates"`
	ImportedDeletes             int64  `json:"imported_deletes"`
	ExportedInserts             int64  `json:"exported_inserts"`
	ExportedUpdates             int64  `json:"exported_updates"`
	ExportedDeletes             int64  `json:"exported_deletes"`
	FinalRowCount               int64  `json:"final_row_count"`
}

var fBEnabled, fFEnabled bool
var firstHeader = []string{"TABLE", "DB_TYPE", "EXPORTED", "IMPORTED", "ERRORED-IMPORTED", "EXPORTED", "EXPORTED", "EXPORTED", "IMPORTED", "IMPORTED", "IMPORTED", "FINAL_ROW_COUNT"}
var secondHeader = []string{"", "", "SNAPSHOT_ROWS", "SNAPSHOT_ROWS", "SNAPSHOT_ROWS", "INSERTS", "UPDATES", "DELETES", "INSERTS", "UPDATES", "DELETES", ""}

var firstHeaderForDetailedReport = []string{"TABLE", "DB_TYPE", "ITERATION", "EXPORTED", "IMPORTED", "ERRORED-IMPORTED", "EXPORTED", "EXPORTED", "EXPORTED", "IMPORTED", "IMPORTED", "IMPORTED", "CUMULATIVE"}
var secondHeaderForDetailedReport = []string{"", "", "NUMBER", "SNAPSHOT_ROWS", "SNAPSHOT_ROWS", "SNAPSHOT_ROWS", "INSERTS", "UPDATES", "DELETES", "INSERTS", "UPDATES", "DELETES", "FINAL_ROW_COUNT"}

var firstHeaderForIteration = []string{"TABLE", "DB_TYPE", "EXPORTED", "EXPORTED", "EXPORTED", "IMPORTED", "IMPORTED", "IMPORTED"}
var secondHeaderForIteration = []string{"", "", "INSERTS", "UPDATES", "DELETES", "INSERTS", "UPDATES", "DELETES"}

type RowDataForIteration struct {
	TableName       string `json:"table_name"`
	DBType          string `json:"db_type"`
	ExportedInserts int64  `json:"exported_inserts"`
	ExportedUpdates int64  `json:"exported_updates"`
	ExportedDeletes int64  `json:"exported_deletes"`
	ImportedInserts int64  `json:"imported_inserts"`
	ImportedUpdates int64  `json:"imported_updates"`
	ImportedDeletes int64  `json:"imported_deletes"`
}

func getDataMigrationReportCmdFn(msr *metadb.MigrationStatusRecord, donotPrint bool, aggregateIterationsStats bool) {

	var reportData []*RowData
	fBEnabled = msr.FallbackEnabled
	fFEnabled = msr.FallForwardEnabled
	tableList := msr.TableListExportedFromSource
	tableNameTups, err := getInitialImportTableListForLive(tableList)
	if err != nil {
		utils.ErrExit("getting name tuples from table list: %w", err)
	}
	params := namereg.NameRegistryParams{
		FilePath: fmt.Sprintf("%s/metainfo/name_registry.json", exportDir),
		Role:     SOURCE_REPLICA_DB_IMPORTER_ROLE,
		SDB:      nil,
		YBDB:     nil,
	}
	nameRegistryForSourceReplicaRole = namereg.NewNameRegistry(params)
	err = nameRegistryForSourceReplicaRole.Init()
	if err != nil {
		utils.ErrExit("initializing name registry for source replica: %w", err)
	}
	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	dbzmStatus, err := dbzm.ReadExportStatus(exportStatusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file: %s: %w", exportStatusFilePath, err)
	}
	if dbzmStatus == nil {
		utils.ErrExit("Export data has not started yet. Try running after export has started.")
	}
	dbzmNameTupToRowCount := utils.NewStructMap[sqlname.NameTuple, int64]()

	exportSnapshotStatusFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportSnapshotStatusFile = jsonfile.NewJsonFile[ExportSnapshotStatus](exportSnapshotStatusFilePath)
	var exportSnapshotStatus *ExportSnapshotStatus

	sqlname.SourceDBType = source.DBType
	var exportedPGSnapshotRowsMap *utils.StructMap[sqlname.NameTuple, int64]

	source = *msr.SourceDBConf
	if msr.ExportTypeFromSource == SNAPSHOT_AND_CHANGES {
		if source.DBType == POSTGRESQL {
			exportSnapshotStatus, err = exportSnapshotStatusFile.Read()
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					utils.ErrExit("Export data has not started yet. Try running after export has started.")
				}
				utils.ErrExit("Failed to read export status file: %s: %w", exportSnapshotStatusFilePath, err)
			}
			exportedPGSnapshotRowsMap, _, err = getExportedSnapshotRowsMap(exportSnapshotStatus)
			if err != nil {
				utils.ErrExit("error while getting exported snapshot rows: %w\n", err)
			}
		} else {
			//for ORACLE case to fetch dbzm status file
			for _, tableExportStatus := range dbzmStatus.Tables {
				tableName := fmt.Sprintf("%s.%s", tableExportStatus.SchemaName, tableExportStatus.TableName)
				nt, err := namereg.NameReg.LookupTableName(tableName)
				if err != nil {
					utils.ErrExit("lookup in name registry: %s: %w", tableName, err)
				}
				dbzmNameTupToRowCount.Put(nt, tableExportStatus.ExportedRowCountSnapshot)
			}
		}
	}

	var sourceExportedEventsMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]
	var targetExportedEventsMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]
	sourceExportedEventsMap, err = metaDB.GetExportedEventsStatsForExporterRole(SOURCE_DB_EXPORTER_ROLE)
	if err != nil {
		utils.ErrExit("getting exported events from source stats: %w", err)
	}
	if fFEnabled {
		targetExportedEventsMap, err = metaDB.GetExportedEventsStatsForExporterRole(TARGET_DB_EXPORTER_FF_ROLE)
		if err != nil {
			utils.ErrExit("getting exported events from target stats: %w", err)
		}
	}
	if fBEnabled {
		targetExportedEventsMap, err = metaDB.GetExportedEventsStatsForExporterRole(TARGET_DB_EXPORTER_FB_ROLE)
		if err != nil {
			utils.ErrExit("getting exported events from target stats: %w", err)
		}
	}

	var targetImportedSnapshotRowsMap *utils.StructMap[sqlname.NameTuple, RowCountPair]
	var targetEventsImportedMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]
	if msr.TargetDBConf != nil {
		errorHandler, err := getImportDataErrorHandlerUsed()
		if err != nil {
			utils.ErrExit("error while getting import data error handler: %w\n", err)
		}
		targetImportedSnapshotRowsMap, err = getImportedSnapshotRowsMap("target", tableNameTups, errorHandler)
		if err != nil {
			utils.ErrExit("error while getting imported snapshot rows for target DB: %w\n", err)
		}
		targetEventsImportedMap, err = getImportedEventsMap("target", tableNameTups, msr.TargetDBConf)
		if err != nil {
			utils.ErrExit("error while getting imported events counts for target DB: %w\n", err)
		}
	}

	var replicaImportedSnapshotRowsMap *utils.StructMap[sqlname.NameTuple, RowCountPair]
	var replicaEventsImportedMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]
	if fFEnabled {
		// In this case we need to lookup in a namereg where role is SOURCE_REPLICA_DB_IMPORTER_ROLE so that
		// look up happens properly for source_replica names as here reg is the map of target->source-replica tablename
		oldNameReg := namereg.NameReg
		namereg.NameReg = *nameRegistryForSourceReplicaRole
		replicaImportedSnapshotRowsMap, err = getImportedSnapshotRowsMap("source-replica", tableNameTups, nil)

		if err != nil {
			utils.ErrExit("error while getting imported snapshot rows for source-replica DB: %w\n", err)
		}
		sourceReplicaTups := make([]sqlname.NameTuple, len(tableNameTups))
		for i, ntup := range tableNameTups {
			sourceReplicaTups[i], err = namereg.NameReg.LookupTableName(ntup.ForKey())
			if err != nil {
				utils.ErrExit("lookup table %s: %v", ntup.ForKey(), err)
			}
		}
		replicaEventsImportedMap, err = getImportedEventsMap("source-replica", sourceReplicaTups, msr.SourceReplicaDBConf)
		if err != nil {
			utils.ErrExit("error while getting imported events counts for source-replica DB: %w\n", err)
		}
		namereg.NameReg = oldNameReg
	}

	var sourceEventsImportedMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]
	if fBEnabled {
		sourceEventsImportedMap, err = getImportedEventsMap("source", tableNameTups, msr.SourceDBAsTargetConf)
		if err != nil {
			utils.ErrExit("error while getting imported events counts for source DB in case of fall-back: %w\n", err)
		}
	}

	for _, nameTup := range tableNameTups {

		row := RowData{}
		updateExportedSnapshotRowsInTheRow(msr, &row, nameTup, dbzmNameTupToRowCount, exportedPGSnapshotRowsMap)
		row.ImportedSnapshotRows = 0
		row.ErroredImportedSnapshotRows = 0
		row.TableName = nameTup.ForKey()
		row.DBType = "source"
		err := updateExportedEventsCountsInTheRow(&row, nameTup, sourceExportedEventsMap, targetExportedEventsMap) //source OUT counts
		if err != nil {
			utils.ErrExit("error while getting exported events counts for source DB: %w\n", err)
		}
		if fBEnabled {
			err = updateImportedEventsCountsInTheRow(&row, nameTup, nil, sourceEventsImportedMap, msr) //fall back IN counts
			if err != nil {
				utils.ErrExit("error while getting imported events for source DB in case of fall-back: %w\n", err)
			}
		}
		addRowInTheReport(&reportData, row, nameTup.ForKey())
		row = RowData{}
		row.TableName = ""
		row.DBType = "target"
		row.ExportedSnapshotRows = 0
		if msr.TargetDBConf != nil { // In case import is not started yet, target DB conf will be nil
			err = updateImportedEventsCountsInTheRow(&row, nameTup, targetImportedSnapshotRowsMap, targetEventsImportedMap, msr) //target IN counts
			if err != nil {
				utils.ErrExit("error while getting imported events for target DB: %w\n", err)
			}
		}
		if fFEnabled || fBEnabled {
			err = updateExportedEventsCountsInTheRow(&row, nameTup, sourceExportedEventsMap, targetExportedEventsMap) // target OUT counts
			if err != nil {
				utils.ErrExit("error while getting exported events for target DB: %w\n", err)
			}
		}
		addRowInTheReport(&reportData, row, nameTup.ForKey())
		if fFEnabled {
			row = RowData{}
			row.TableName = ""
			row.DBType = "source-replica"
			row.ExportedSnapshotRows = 0
			err = updateImportedEventsCountsInTheRow(&row, nameTup, replicaImportedSnapshotRowsMap, replicaEventsImportedMap, msr) //fall forward IN counts
			if err != nil {
				utils.ErrExit("error while getting imported events for DB %s: %w\n", row.DBType, err)
			}
			addRowInTheReport(&reportData, row, nameTup.ForKey())
		}
	}

	if aggregateIterationsStats {
		reportData, err = aggregateDataWithIterationsIfRequired(reportData, msr)
		if err != nil {
			utils.ErrExit("error while aggregating data with iterations: %w", err)
		}
	}

	isIteration := msr.IterationNo > 0

	if reportOrStatusCmdOutputFormat == "json" {
		err = generateReportInJsonFormat(reportData, msr, isIteration, donotPrint)
		if err != nil {
			utils.ErrExit("error while generating report in json format: %w", err)
		}
	} else {
		statsPerTable := parseRowDataToStatsPerTable(reportData)
		printReport(isIteration, statsPerTable, msr)
	}

}

func generateReportInJsonFormat(reportData []*RowData, msr *metadb.MigrationStatusRecord, isIteration bool, donotPrint bool) error {
	var err error
	// Print the report in json format.
	reportFilePath := filepath.Join(exportDir, "reports", "data-migration-report.json")
	if isIteration {
		reportFile := jsonfile.NewJsonFile[[]*RowDataForIteration](reportFilePath)
		reportDataForIteration := convertRowDataToRowDataForIteration(reportData)
		err = reportFile.Create(&reportDataForIteration)
		if err != nil {
			return fmt.Errorf("creating into json file: %s: %w", reportFilePath, err)
		}
	} else {
		reportFile := jsonfile.NewJsonFile[[]*RowData](reportFilePath)
		err = reportFile.Create(&reportData)
		if err != nil {
			return fmt.Errorf("creating into json file: %s: %w", reportFilePath, err)
		}
	}
	if !isIteration && msr.LatestIterationNumber > 0 && !donotPrint {
		if includeDetailedIterationsStats {
			utils.PrintAndLogfPhase("\nDetailed data migration report for all iterations:")
		} else {
			utils.PrintAndLogfPhase("\nAggregated Data migration report for the overall migration:")
		}
	}
	if !donotPrint {
		fmt.Print(color.GreenString("Data migration report is written to %s\n", reportFilePath))
	}
	return nil
}

func parseRowDataToStatsPerTable(reportData []*RowData) map[string][]*RowData {
	statsPerTable := make(map[string][]*RowData)
	for _, row := range reportData {
		statsPerTable[row.TableName] = append(statsPerTable[row.TableName], row)
	}
	return statsPerTable
}

func printReport(forIteration bool, statsPerTable map[string][]*RowData, msr *metadb.MigrationStatusRecord) {
	tableNames := lo.Keys(statsPerTable)
	slices.Sort(tableNames)

	uitbl := uitable.New()
	printHeader(uitbl, forIteration)

	if !forIteration && msr.LatestIterationNumber > 0 {
		if includeDetailedIterationsStats {
			utils.PrintAndLogfPhase("\nDetailed data migration report for all iterations:")
		} else {
			utils.PrintAndLogfPhase("\nAggregated Data migration report for the overall migration:")
		}
	}

	rowsInCurrUITable := 0
	maxRowsInOnePage := 30
	for _, tableName := range tableNames {
		rowsForCurrTable := len(statsPerTable[tableName])
		// If adding this table would exceed the page limit
		// AND we already have at least one table on the page, flush.
		if rowsInCurrUITable > 0 && rowsInCurrUITable+rowsForCurrTable > maxRowsInOnePage {
			//print the current table and print the header for the next set of tables
			fmt.Print("\n")
			fmt.Println(uitbl)
			fmt.Print("\n")
			uitbl = uitable.New()
			rowsInCurrUITable = 0
			printHeader(uitbl, forIteration)
		}
		uitbl.AddRow() // blank row
		lastIterationNumber := 0
		for i, row := range statsPerTable[tableName] {
			if i > 0 {
				row.TableName = ""
			}

			if includeDetailedIterationsStats && row.IterationNumber != lastIterationNumber {
				uitbl.AddRow()
			}
			addRowToTable(uitbl, row, forIteration)
			lastIterationNumber = row.IterationNumber
		}
		rowsInCurrUITable += rowsForCurrTable
	}
	fmt.Print("\n")
	fmt.Println(uitbl)
	fmt.Print("\n")

	if !bool(includeDetailedIterationsStats) && msr.LatestIterationNumber > 0 {
		//If detailed report is not enabled, and there are iterations, print the info to see the detailed report
		utils.PrintAndLogfInfo("To see the detailed report with all the iterations, run the command with the --include-detailed-iterations-stats true flag.\n\n")
	}

}

func printHeader(uitbl *uitable.Table, forIteration bool) {
	uitbl.MaxColWidth = 50
	uitbl.Wrap = true
	uitbl.Separator = " | "
	if forIteration {
		addHeader(uitbl, firstHeaderForIteration...)
		addHeader(uitbl, secondHeaderForIteration...)
	} else if includeDetailedIterationsStats {
		addHeader(uitbl, firstHeaderForDetailedReport...)
		addHeader(uitbl, secondHeaderForDetailedReport...)
	} else {
		addHeader(uitbl, firstHeader...)
		addHeader(uitbl, secondHeader...)
	}
}

func addRowToTable(uitbl *uitable.Table, row *RowData, forIteration bool) {
	if forIteration {
		uitbl.AddRow(row.TableName, row.DBType, row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes, row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes)
	} else if includeDetailedIterationsStats {
		if row.IterationNumber == 0 {
			uitbl.AddRow(row.TableName, row.DBType, row.IterationNumber, row.ExportedSnapshotRows, row.ImportedSnapshotRows, row.ErroredImportedSnapshotRows,
				row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes, row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes, row.FinalRowCount)
		} else { //iteration with changes only have no snapshot rows
			uitbl.AddRow(row.TableName, row.DBType, row.IterationNumber, "-", "-", "-",
				row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes, row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes, row.FinalRowCount)
		}
	} else {
		uitbl.AddRow(row.TableName, row.DBType, row.ExportedSnapshotRows, row.ImportedSnapshotRows, row.ErroredImportedSnapshotRows,
			row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes, row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes, row.FinalRowCount)
	}

}

func addRowInTheReport(reportData *[]*RowData, row RowData, tableName string) {
	row.TableName = tableName
	row.FinalRowCount = getFinalRowCount(row)
	*reportData = append(*reportData, &row)
}

func aggregateDataWithIterationsIfRequired(reportData []*RowData, msr *metadb.MigrationStatusRecord) ([]*RowData, error) {
	if msr.LatestIterationNumber == 0 {
		return reportData, nil
	}

	if includeDetailedIterationsStats {
		for _, row := range reportData {
			row.IterationNumber = 0 //set the iteration number to 0 for the main migration data
		}
	}

	iterationsDir := msr.GetIterationsDir(exportDir)
	//get all the iteration export dirs
	for i := 1; i <= msr.LatestIterationNumber; i++ {
		iterationExportDir := GetIterationExportDir(iterationsDir, i)
		//run get data migration report command for each iteration
		iterationReportData, err := getIterationDataMigrationReport(iterationExportDir)
		if err != nil {
			return nil, fmt.Errorf("error while getting iteration data migration report: %w", err)
		}
		if includeDetailedIterationsStats {

			for _, row := range iterationReportData {
				row.IterationNumber = i
			}

			//For detailed one, just append the iteration report data to the main report data
			reportData = append(reportData, iterationReportData...)
		} else {
			//aggregate the iteration report data with the main report data
			reportData = aggregateReportData(reportData, iterationReportData)
		}
	}

	if includeDetailedIterationsStats {
		//group the report data by table name and calcutate cumulative row count for iteration
		reportData = groupByTableNameAndCalculateCumulativeRowCount(reportData)
	}
	return reportData, nil
}

func groupByTableNameAndCalculateCumulativeRowCount(reportData []*RowData) []*RowData {
	//group the report data by table name
	reportDataMap := make(map[string][]*RowData)
	for _, row := range reportData {
		reportDataMap[row.TableName] = append(reportDataMap[row.TableName], row)
	}
	tableNames := lo.Keys(reportDataMap)
	slices.Sort(tableNames)

	reportData = make([]*RowData, 0)
	for _, tableName := range tableNames {
		var finalRowCountSrc int64 = 0
		var finalRowCountTgt int64 = 0
		rows := reportDataMap[tableName]
		for i, row := range rows {
			if row.IterationNumber == 0 {
				if row.DBType == "source" {
					finalRowCountSrc = row.FinalRowCount
				} else {
					finalRowCountTgt = row.FinalRowCount
				}
			}
			if row.IterationNumber > 0 {
				if row.DBType == "source" {
					finalRowCountSrc += row.ExportedInserts - row.ExportedDeletes + row.ImportedInserts - row.ImportedDeletes
					rows[i].FinalRowCount = finalRowCountSrc
				} else {
					finalRowCountTgt += row.ImportedInserts - row.ImportedDeletes + row.ExportedInserts - row.ExportedDeletes
					rows[i].FinalRowCount = finalRowCountTgt
				}

			}
			reportData = append(reportData, row)
		}
	}

	return reportData
}

func getIterationDataMigrationReport(iterationExportDir string) ([]*RowData, error) {
	currExportDir := exportDir
	currMetaDB := metaDB
	currReportFormat := reportOrStatusCmdOutputFormat
	currMigrationUUID := migrationUUID

	defer func() {
		exportDir = currExportDir
		metaDB = currMetaDB
		reportOrStatusCmdOutputFormat = currReportFormat
		migrationUUID = currMigrationUUID
	}()

	exportDir = iterationExportDir
	metaDB = initMetaDB(iterationExportDir)
	reportOrStatusCmdOutputFormat = "json"

	iterationMsr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("error while getting iteration migration status record: %w", err)
	}
	if iterationMsr.TargetDBConf != nil {
		iterationMsr.TargetDBConf.Password = targetDBPassword
	}
	if iterationMsr.FallbackEnabled && iterationMsr.SourceDBAsTargetConf != nil {
		iterationMsr.SourceDBAsTargetConf.Password = sourceDbPassword
	}
	migrationUUID = uuid.MustParse(iterationMsr.MigrationUUID)

	getDataMigrationReportCmdFn(iterationMsr, true, false)

	iterationReportFile := filepath.Join(iterationExportDir, "reports", "data-migration-report.json")
	iterationReportJsonFile := jsonfile.NewJsonFile[[]*RowData](iterationReportFile)
	iterationReportData, err := iterationReportJsonFile.Read()
	if err != nil {
		return nil, fmt.Errorf("error while reading iteration data migration report %s: %w", iterationReportFile, err)
	}

	return *iterationReportData, nil
}

func aggregateReportData(reportData []*RowData, iterationReportData []*RowData) []*RowData {
	reportDataMap := make(map[string]*RowData)
	for _, row := range reportData {
		reportDataMap[fmt.Sprintf("%s-%s", row.TableName, row.DBType)] = row
	}
	for _, row := range iterationReportData {
		key := fmt.Sprintf("%s-%s", row.TableName, row.DBType)
		if reportDataMap[key] == nil {
			//Not possible, iteration report data should have set of tables ad dbtype of the tables
			utils.ErrExit("Error: iteration report data should have set of tables ad dbtype of the tables, key: %s", key)
		}
		reportDataMap[key].ExportedInserts += row.ExportedInserts
		reportDataMap[key].ExportedUpdates += row.ExportedUpdates
		reportDataMap[key].ExportedDeletes += row.ExportedDeletes
		reportDataMap[key].ImportedInserts += row.ImportedInserts
		reportDataMap[key].ImportedUpdates += row.ImportedUpdates
		reportDataMap[key].ImportedDeletes += row.ImportedDeletes
		reportDataMap[key].FinalRowCount = getFinalRowCount(*reportDataMap[key])
	}
	return reportData
}

func convertRowDataToRowDataForIteration(reportData []*RowData) []*RowDataForIteration {
	reportDataForIteration := make([]*RowDataForIteration, len(reportData))
	for i, row := range reportData {
		reportDataForIteration[i] = &RowDataForIteration{
			TableName:       row.TableName,
			DBType:          row.DBType,
			ExportedInserts: row.ExportedInserts,
			ExportedUpdates: row.ExportedUpdates,
			ExportedDeletes: row.ExportedDeletes,
			ImportedInserts: row.ImportedInserts,
			ImportedUpdates: row.ImportedUpdates,
			ImportedDeletes: row.ImportedDeletes,
		}
	}
	return reportDataForIteration
}

func updateExportedSnapshotRowsInTheRow(msr *metadb.MigrationStatusRecord, row *RowData, nameTup sqlname.NameTuple, dbzmSnapshotRowCount *utils.StructMap[sqlname.NameTuple, int64], exportedSnapshotPGRowsMap *utils.StructMap[sqlname.NameTuple, int64]) error {
	if msr.ExportTypeFromSource == CHANGES_ONLY {
		//TODO: handle it better if it is iterative cutove
		row.ExportedSnapshotRows = 0
		return nil
	}
	// TODO: read only from one place(data file descriptor). Right now, data file descriptor does not store schema names.
	if msr.IsSnapshotExportedViaDebezium() {
		row.ExportedSnapshotRows, _ = dbzmSnapshotRowCount.Get(nameTup)
	} else {
		row.ExportedSnapshotRows, _ = exportedSnapshotPGRowsMap.Get(nameTup)
	}
	return nil
}

func getImportedEventsMap(dbType string, tableNameTups []sqlname.NameTuple, targetConf *tgtdb.TargetConf) (*utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter], error) {
	switch dbType {
	case "target":
		importerRole = TARGET_DB_IMPORTER_ROLE
	case "source-replica":
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
	case "source":
		importerRole = SOURCE_DB_IMPORTER_ROLE
	}
	//reinitialise targetDB
	tconf = *targetConf
	tdb = tgtdb.NewTargetDB(&tconf)
	err := tdb.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the target DB: %w", err)
	}
	defer tdb.Finalize()
	state := NewImportDataState(exportDir)
	tableNameTupToEventsCounter, err := state.GetImportedEventsStatsForTableList(tableNameTups, migrationUUID)
	if err != nil {
		return nil, fmt.Errorf("imported events stats for tableList: %w", err)
	}
	return tableNameTupToEventsCounter, nil
}

func updateImportedEventsCountsInTheRow(row *RowData, tableNameTup sqlname.NameTuple, snapshotImportedRowsMap *utils.StructMap[sqlname.NameTuple, RowCountPair],
	eventsImportedMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter], msr *metadb.MigrationStatusRecord) error {
	switch row.DBType {
	case "target":
		importerRole = TARGET_DB_IMPORTER_ROLE
	case "source-replica":
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
	case "source":
		importerRole = SOURCE_DB_IMPORTER_ROLE
	}
	if importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE {
		var err error
		tblName := tableNameTup.ForKey()
		// In case of source-replica role namereg is the map of target->source-replica name
		// and hence ForKey() returns source-replica name so we need to get that from reg
		tableNameTup, err = nameRegistryForSourceReplicaRole.LookupTableName(tblName)
		if err != nil {
			return fmt.Errorf("lookup %s in source replica name registry: %w", tblName, err)
		}
	}

	if importerRole != SOURCE_DB_IMPORTER_ROLE && msr.ExportTypeFromSource == SNAPSHOT_AND_CHANGES {
		rowCountPair, _ := snapshotImportedRowsMap.Get(tableNameTup)
		row.ImportedSnapshotRows = rowCountPair.Imported
		row.ErroredImportedSnapshotRows = rowCountPair.Errored
	}

	eventCounter, _ := eventsImportedMap.Get(tableNameTup)
	row.ImportedInserts = eventCounter.NumInserts
	row.ImportedUpdates = eventCounter.NumUpdates
	row.ImportedDeletes = eventCounter.NumDeletes
	return nil
}

func updateExportedEventsCountsInTheRow(row *RowData, tableNameTup sqlname.NameTuple, sourceExportedEventsMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter], targetExportedEventsMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]) error {
	var exportedEventsMap *utils.StructMap[sqlname.NameTuple, *tgtdb.EventCounter]
	switch row.DBType {
	case "source":
		exportedEventsMap = sourceExportedEventsMap
	case "target":
		exportedEventsMap = targetExportedEventsMap
	}

	eventCounter, _ := exportedEventsMap.Get(tableNameTup)
	if eventCounter != nil {
		row.ExportedInserts = eventCounter.NumInserts
		row.ExportedUpdates = eventCounter.NumUpdates
		row.ExportedDeletes = eventCounter.NumDeletes
	}

	return nil
}

func getFinalRowCount(row RowData) int64 {
	if row.DBType == "source" {
		return row.ExportedSnapshotRows + row.ExportedInserts + row.ImportedInserts - row.ExportedDeletes - row.ImportedDeletes
	}
	return row.ImportedSnapshotRows + row.ImportedInserts + row.ExportedInserts - row.ImportedDeletes - row.ExportedDeletes
}

func getFinalRowCountWithSnapshotRows(row RowData, exportSnapshotRows int64, importSnapshotRows int64) int64 {
	if row.DBType == "source" {
		return exportSnapshotRows + row.ExportedInserts + row.ImportedInserts - row.ExportedDeletes - row.ImportedDeletes
	}
	return importSnapshotRows + row.ImportedInserts + row.ExportedInserts - row.ImportedDeletes - row.ExportedDeletes
}

func init() {
	getCommand.AddCommand(getDataMigrationReportCmd)
	registerExportDirFlag(getDataMigrationReportCmd)
	registerConfigFileFlag(getDataMigrationReportCmd)
	getDataMigrationReportCmd.PersistentFlags().StringVarP(&config.LogLevel, "log-level", "l", "info",
		"log level for yb-voyager. Accepted values: (trace, debug, info, warn, error, fatal, panic)")
	getDataMigrationReportCmd.Flags().StringVar(&reportOrStatusCmdOutputFormat, "output-format", "table",
		"format in which report will be generated: (table, json) (default: table)")

	BoolVar(getDataMigrationReportCmd.Flags(), &includeDetailedIterationsStats, "include-detailed-iterations-stats", false,
		"include the detailed report with all the iterations stats in the report.")

	getDataMigrationReportCmd.Flags().StringVar(&sourceReplicaDbPassword, "source-replica-db-password", "",
		"password with which to connect to the target Source-Replica DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_REPLICA_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	getDataMigrationReportCmd.Flags().StringVar(&sourceDbPassword, "source-db-password", "",
		"password with which to connect to the target source DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes")

	getDataMigrationReportCmd.Flags().StringVar(&targetDBPassword, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server. Alternatively, you can also specify the password by setting the environment variable TARGET_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")
}
