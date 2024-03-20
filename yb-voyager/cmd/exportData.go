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
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
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
	saveExportTypeInMSR()
	err = namereg.InitNameRegistry(exportDir, exporterRole, &source, source.DB(), nil, nil)
	if err != nil {
		utils.ErrExit("initialize name registry: %v", err)
	}

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
	var partitionsToRootTableMap map[string]string
	partitionsToRootTableMap, finalTableList, err = addLeafPartitionsInTableList(finalTableList)
	if err != nil {
		utils.ErrExit("failed to add the leaf partitions in table list: %w", err)
	}

	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch source.DBType {
		case POSTGRESQL:
			record.SourceRenameTablesMap = partitionsToRootTableMap
		case YUGABYTEDB:
			//Need to keep this as separate field as we have to rename tables in case of partitions for export from target as we do for source but
			//partitions can change on target during migration so need to handle that case
			record.TargetRenameTablesMap = partitionsToRootTableMap
		}
	})

	leafPartitions := make(map[string][]string)
	tableListToDisplay := lo.Uniq(lo.Map(finalTableList, func(table *sqlname.SourceName, _ int) string {
		renamedTable, isRenamed := renameTableIfRequired(table.Qualified.MinQuoted)
		if isRenamed {
			//TODO: it is ok to do this right now but will fix everything with Name Registry
			table := strings.TrimPrefix(table.Qualified.MinQuoted, "public.")
			leafPartitions[renamedTable] = append(leafPartitions[renamedTable], table)
			if len(strings.Split(renamedTable, ".")) == 1 {
				//TODD: just for now, hack to display unique table list as we are already
				// adding qualified root table in addLeafParititonsInTableList func
				return fmt.Sprintf("public.%s", renamedTable)
			}
		}
		return renamedTable
	}))
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}

	//handle case of display in case user is filtering few partitions in table-list
	tableListToDisplay = lo.Map(tableListToDisplay, func(table string, _ int) string {
		if source.DBType == POSTGRESQL && leafPartitions[table] != nil && msr.IsExportTableListSet {
			partitions := strings.Join(leafPartitions[table], ", ")
			return fmt.Sprintf("%s (%s)", table, partitions)
		}
		return table
	})
	fmt.Printf("num tables to export: %d\n", len(tableListToDisplay))
	utils.PrintAndLog("table list for data export: %v", tableListToDisplay)

	//finalTableList is with leaf partitions and root tables after this in the whole export flow to make all the catalog queries work fine

	if changeStreamingIsEnabled(exportType) || useDebezium {
		config, tableNametoApproxRowCountMap, err := prepareDebeziumConfig(partitionsToRootTableMap, finalTableList, tablesColumnList, leafPartitions)
		if err != nil {
			log.Errorf("Failed to prepare dbzm config: %v", err)
			return false
		}
		if source.DBType == POSTGRESQL && changeStreamingIsEnabled(exportType) {
			// pg live migration. Steps are as follows:
			// 1. create publication, replication slot.
			// 2. export snapshot corresponding to replication slot by passing it to pg_dump
			// 3. start debezium with configration to read changes from the created replication slot, publication.

			err := source.DB().ValidateTablesReadyForLiveMigration(finalTableList)
			if err != nil {
				utils.ErrExit("error: validate if tables are ready for live migration: %v", err)
			}
			if !dataIsExported() { // if snapshot is not already done...
				err = exportPGSnapshotWithPGdump(ctx, cancel, finalTableList, tablesColumnList)
				if err != nil {
					log.Errorf("export snapshot failed: %v", err)
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
		saveTableToUniqueKeyColumnsMapInMetaDB(finalTableList)
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
		err = storeTableListInMSR(finalTableList)
		if err != nil {
			utils.ErrExit("store table list in MSR: %v", err)
		}
		err = exportDataOffline(ctx, cancel, finalTableList, tablesColumnList, "")
		if err != nil {
			log.Errorf("Export Data failed: %v", err)
			return false
		}
		return true
	}
}

// required only for postgresql/yugabytedb since GetAllTables() returns all tables and partitions
func addLeafPartitionsInTableList(tableList []*sqlname.SourceName) (map[string]string, []*sqlname.SourceName, error) {
	requiredForSource := source.DBType == "postgresql" || source.DBType == "yugabytedb"
	if !requiredForSource {
		return nil, tableList, nil
	}
	// here we are adding leaf partitions in the list only in yb or pg because events in the
	// these dbs are referred with leaf partitions only but in oracle root table is main point of reference
	// for partitions and in Oracle fall-forward/fall-back case we are doing renaming using the config `ybexporter.tables.rename` in dbzm
	// when event is coming from YB for leaf partitions it is getting renamed to root_table
	// ex - customers -> cust_other, cust_part11, cust_part12, cust_part22, cust_part21
	// events from oracle - will have customers for all of these partitions
	// events from yb,pg will have `cust_other, cust_part11, cust_part12, cust_part22, cust_part21` out of these leaf partitions only
	// using the dbzm we are renaming these events coming from yb to root_table for oracle.
	// not required for pg to rename them.

	modifiedTableList := []*sqlname.SourceName{}
	var partitionsToRootTableMap = make(map[string]string)

	//TODO: optimisation to avoid multiple calls to DB with one call in the starting to fetch TablePartitionTree map.

	//TODO: test when we upgrade to PG13+ as partitions are handled with root table
	//Refer- https://debezium.zulipchat.com/#narrow/stream/302529-community-general/topic/Connector.20not.20working.20with.20partitions
	for _, table := range tableList {
		rootTable, err := GetRootTableOfPartition(table)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get root table of partition %s: %v", table.Qualified.MinQuoted, err)
		}
		allLeafPartitions := GetAllLeafPartitions(table)
		switch true {
		case len(allLeafPartitions) == 0 && rootTable != table: //leaf partition
			partitionsToRootTableMap[table.Qualified.Unquoted] = rootTable.Qualified.MinQuoted // Unquoted->MinQuoted map as debezium uses Unquoted table name
			modifiedTableList = append(modifiedTableList, table)
		case len(allLeafPartitions) == 0 && rootTable == table: //normal table
			modifiedTableList = append(modifiedTableList, table)
		case len(allLeafPartitions) > 0 && source.TableList != "": // table with partitions in table list
			for _, leafPartition := range allLeafPartitions {
				modifiedTableList = append(modifiedTableList, leafPartition)
				partitionsToRootTableMap[leafPartition.Qualified.Unquoted] = rootTable.Qualified.MinQuoted
			}
		}
		// will be keeping root in the list as it might be required by some of the catalog queries
		modifiedTableList = append(modifiedTableList, rootTable)
	}
	return partitionsToRootTableMap, lo.UniqBy(modifiedTableList, func(table *sqlname.SourceName) string {
		return table.Qualified.MinQuoted
	}), nil
}

func GetRootTableOfPartition(table *sqlname.SourceName) (*sqlname.SourceName, error) {
	parentTable := source.DB().ParentTableOfPartition(table)
	if parentTable == "" {
		return table, nil
	}
	defaultSourceSchema, noDefaultSchema := getDefaultSourceSchemaName()
	if noDefaultSchema {
		return nil, fmt.Errorf("default schema not found")
	}
	return GetRootTableOfPartition(sqlname.NewSourceNameFromMaybeQualifiedName(parentTable, defaultSourceSchema))
}

func GetAllLeafPartitions(table *sqlname.SourceName) []*sqlname.SourceName {
	allLeafPartitions := []*sqlname.SourceName{}
	childPartitions := source.DB().GetPartitions(table)
	for _, childPartition := range childPartitions {
		leafPartitions := GetAllLeafPartitions(childPartition)
		if len(leafPartitions) == 0 {
			allLeafPartitions = append(allLeafPartitions, childPartition)
		} else {
			allLeafPartitions = append(allLeafPartitions, leafPartitions...)
		}
	}
	return allLeafPartitions
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
	// Note: publication object needs to be created before replication slot
	// https://www.postgresql.org/message-id/flat/e0885261-5723-7bab-f541-e6a260f50328%402ndquadrant.com#a5f257b667575719ad98c59281f3e191
	publicationName := "voyager_dbz_publication_" + strings.ReplaceAll(migrationUUID.String(), "-", "_")
	err = pgDB.CreatePublication(replicationConn, publicationName, finalTableList, true)
	if err != nil {
		return fmt.Errorf("create publication: %v", err)
	}
	replicationSlotName := fmt.Sprintf("voyager_%s", strings.ReplaceAll(migrationUUID.String(), "-", "_"))
	res, err := pgDB.CreateLogicalReplicationSlot(replicationConn, replicationSlotName, true)
	if err != nil {
		return fmt.Errorf("export snapshot: failed to create replication slot: %v", err)
	}
	yellowBold := color.New(color.FgYellow, color.Bold)
	utils.PrintAndLog(yellowBold.Sprintf("Created replication slot '%s' on source PG database. "+
		"Be sure to run 'initiate cutover to target'/'end migration' command after completing/aborting this migration to drop the replication slot. "+
		"This is important to avoid filling up disk space.", replicationSlotName))

	// save replication slot, publication name in MSR
	err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.PGReplicationSlotName = res.SlotName
		record.SnapshotMechanism = "pg_dump"
		record.PGPublicationName = publicationName
	})
	if err != nil {
		utils.ErrExit("update PGReplicationSlotName: update migration status record: %s", err)
	}

	// pg_dump
	err = exportDataOffline(ctx, cancel, finalTableList, tablesColumnList, res.SnapshotName)
	if err != nil {
		log.Errorf("Export Data failed: %v", err)
		return err
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
	// last arg is `is_called``. if is_called=true, next_val= 201, if is_called=false, next_val=200
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

		isCalled := strings.TrimSpace(args[2])
		if isCalled == "false" {
			// we always restore sequences with is_called=true, therefore, we need to minus 1.
			seqVal--
		}

		result[seqName] = seqVal
	}
	return result, nil
}

func reportUnsupportedTables(finalTableList []*sqlname.SourceName) {
	//report Partitions or case sensitive tables
	var caseSensitiveTables []string
	var partitionedTables []string
	allNonPKTables, err := source.DB().GetNonPKTables()
	if err != nil {
		utils.ErrExit("get non-pk tables: %v", err)
	}
	var nonPKTables []string
	for _, table := range finalTableList {
		if source.DBType == POSTGRESQL {
			if table.ObjectName.MinQuoted != table.ObjectName.Unquoted {
				caseSensitiveTables = append(caseSensitiveTables, table.Qualified.MinQuoted)
			}
		}
		if lo.Contains(allNonPKTables, table.Qualified.MinQuoted) {
			nonPKTables = append(nonPKTables, table.Qualified.MinQuoted)
		}
	}
	if len(caseSensitiveTables) == 0 && len(partitionedTables) == 0 && len(nonPKTables) == 0 {
		return
	}
	if len(caseSensitiveTables) > 0 {
		utils.PrintAndLog("Case sensitive table names: %s", caseSensitiveTables)
	}
	if len(nonPKTables) > 0 {
		utils.PrintAndLog("Table names without a Primary key: %s", nonPKTables)
	}
	utils.ErrExit("This voyager release does not support live-migration for tables without a primary key and tables with case sensitive name.\n" +
		"You can exclude these tables using the --exclude-table-list argument.")
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
	isTableListModified := len(sqlname.SetDifference(fullTableList, finalTableList)) != 0
	if exporterRole == SOURCE_DB_EXPORTER_ROLE {
		metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			if isTableListModified {
				record.IsExportTableListSet = true
			} else {
				record.IsExportTableListSet = false
			}
		})
	}
	if changeStreamingIsEnabled(exportType) {
		reportUnsupportedTables(finalTableList)
	}
	log.Infof("initial all tables table list for data export: %v", tableList)

	if !changeStreamingIsEnabled(exportType) {
		finalTableList, skippedTableList = source.DB().FilterEmptyTables(finalTableList)
		if len(skippedTableList) != 0 {
			utils.PrintAndLog("skipping empty tables: %v", skippedTableList)
		}
	}

	finalTableList, skippedTableList = source.DB().FilterUnsupportedTables(migrationUUID, finalTableList, useDebezium)
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
	if exporterRole == SOURCE_DB_EXPORTER_ROLE {
		exportDataStartEvent := createSnapshotExportStartedEvent()
		controlPlane.SnapshotExportStarted(&exportDataStartEvent)
	}

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

	if exporterRole == SOURCE_DB_EXPORTER_ROLE {
		exportDataTableMetrics := createUpdateExportedRowCountEventList(
			utils.GetSortedKeys(tablesProgressMetadata))
		controlPlane.UpdateExportedRowCount(exportDataTableMetrics)
	}

	updateFilePaths(&source, exportDir, tablesProgressMetadata)
	utils.WaitGroup.Add(1)
	exportDataStatus(ctx, tablesProgressMetadata, quitChan, exportSuccessChan, bool(disablePb))

	utils.WaitGroup.Wait() // waiting for the dump and progress bars to complete
	if ctx.Err() != nil {
		fmt.Printf("ctx error(exportData.go): %v\n", ctx.Err())
		return fmt.Errorf("ctx error(exportData.go): %w", ctx.Err())
	}

	source.DB().ExportDataPostProcessing(exportDir, tablesProgressMetadata)
	if source.DBType == POSTGRESQL {
		//Make leaf partitions data files entry under the name of root table
		renameDatafileDescriptor(exportDir)
		//Similarly for the export snapshot status file
		err = renameExportSnapshotStatus(exportSnapshotStatusFile)
		if err != nil {
			return fmt.Errorf("rename export snapshot status: %w", err)
		}
	}
	displayExportedRowCountSnapshot(false)

	if exporterRole == SOURCE_DB_EXPORTER_ROLE {
		exportDataCompleteEvent := createSnapshotExportCompletedEvent()
		controlPlane.SnapshotExportCompleted(&exportDataCompleteEvent)
	}

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
		return GetDefaultPGSchema(source.Schema, "|")
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

	utils.PrintAndLog("Starting import data to source with command:\n %s", color.GreenString(cmdStr))
	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager - %w", err)
	}
	env := os.Environ()
	env = slices.Insert(env, 0, "SOURCE_DB_PASSWORD="+source.Password)
	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager import data to source - %w\n Please re-run with command :\n%s", err, cmdStr)
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

func createSnapshotExportStartedEvent() cp.SnapshotExportStartedEvent {
	result := cp.SnapshotExportStartedEvent{}
	initBaseSourceEvent(&result.BaseEvent, "EXPORT DATA")
	return result
}

func createSnapshotExportCompletedEvent() cp.SnapshotExportCompletedEvent {
	result := cp.SnapshotExportCompletedEvent{}
	initBaseSourceEvent(&result.BaseEvent, "EXPORT DATA")
	return result
}

func createUpdateExportedRowCountEventList(tableNames []string) []*cp.UpdateExportedRowCountEvent {

	result := []*cp.UpdateExportedRowCountEvent{}
	var schemaName, tableName2 string

	for _, tableName := range tableNames {
		tableMetadata := tablesProgressMetadata[tableName]
		if source.DBType == "postgresql" && strings.Count(tableName, ".") == 1 {
			schemaName, tableName2 = cp.SplitTableNameForPG(tableName)
		} else {
			schemaName, tableName2 = source.Schema, tableName
		}
		tableMetrics := cp.UpdateExportedRowCountEvent{
			BaseUpdateRowCountEvent: cp.BaseUpdateRowCountEvent{
				BaseEvent: cp.BaseEvent{
					EventType:     "EXPORT DATA",
					MigrationUUID: migrationUUID,
					SchemaNames:   []string{schemaName},
				},
				TableName:         tableName2,
				Status:            cp.EXPORT_OR_IMPORT_DATA_STATUS_INT_TO_STR[tableMetadata.Status],
				TotalRowCount:     tableMetadata.CountTotalRows,
				CompletedRowCount: tableMetadata.CountLiveRows,
			},
		}
		result = append(result, &tableMetrics)
	}

	return result
}

func saveTableToUniqueKeyColumnsMapInMetaDB(tableList []*sqlname.SourceName) {
	res, err := source.DB().GetTableToUniqueKeyColumnsMap(tableList)
	if err != nil {
		utils.ErrExit("get table to unique key columns map: %v", err)
	}

	log.Infof("updating metaDB with table to unique key columns map: %v", res)
	key := fmt.Sprintf("%s_%s", metadb.TABLE_TO_UNIQUE_KEY_COLUMNS_KEY, exporterRole)
	err = metadb.UpdateJsonObjectInMetaDB(metaDB, key, func(record *map[string][]string) {
		*record = res
	})
	if err != nil {
		utils.ErrExit("insert table to unique key columns map: %v", err)
	}
}
