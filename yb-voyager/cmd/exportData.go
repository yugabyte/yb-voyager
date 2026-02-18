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
	"database/sql"
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
	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var exporterRole string = SOURCE_DB_EXPORTER_ROLE
var exportPhase string

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
	Use:   "export-from-source",
	Short: exportDataCmd.Short,
	Long:  exportDataCmd.Long,
	Args:  exportDataCmd.Args,

	PreRun: exportDataCmd.PreRun,

	Run: exportDataCmd.Run,
}

func init() {
	dataCmd.AddCommand(exportDataFromSrcCmd)

	registerCommonGlobalFlags(exportDataCmd)
	registerCommonGlobalFlags(exportDataFromSrcCmd)
	registerCommonExportFlags(exportDataCmd)
	registerCommonExportFlags(exportDataFromSrcCmd)
	registerSourceDBConnFlags(exportDataCmd, true, true)
	registerSourceDBConnFlags(exportDataFromSrcCmd, true, true)
	registerExportDataFlags(exportDataCmd)
	registerExportDataFlags(exportDataFromSrcCmd)
}

func exportDataCommandPreRun(cmd *cobra.Command, args []string) {
	setExportFlagsDefaults()
	err := validateExportFlags(cmd, exporterRole)
	if err != nil {
		utils.ErrExit("failed to validate export flags: %w", err)
	}
	validateExportTypeFlag()
	markFlagsRequired(cmd)
	if changeStreamingIsEnabled(exportType) {
		useDebezium = true
	}

	if bool(source.AllowOracleClobDataExport) {
		if source.DBType != ORACLE {
			utils.ErrExit(color.RedString("allow-oracle-clob-data-export is only valid with source db type oracle. Remove this flag and retry."))
		} else if changeStreamingIsEnabled(exportType) {
			utils.ErrExit(color.RedString("allow-oracle-clob-data-export is not supported for Live Migration. Remove this flag and retry."))
		} else if useDebezium {
			utils.ErrExit(color.RedString("allow-oracle-clob-data-export is not supported for BETA_FAST_DATA_EXPORT export path. Remove this flag and retry."))
		} else {
			utils.PrintAndLog(color.YellowString("Note: Experimental CLOB export is enabled for Oracle offline export."))
		}
	}
}

func exportDataCommandFn(cmd *cobra.Command, args []string) {
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	err := retrieveMigrationUUID()
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}

	ExitIfAlreadyCutover(exporterRole)
	if useDebezium && !changeStreamingIsEnabled(exportType) {
		utils.PrintAndLogf("Note: Beta feature to accelerate data export is enabled by setting BETA_FAST_DATA_EXPORT environment variable")
	}
	printLiveMigrationLimitations()
	utils.PrintAndLogf("export of data for source type as '%s'", source.DBType)
	sqlname.SourceDBType = source.DBType

	success := exportData()
	if success {
		sendPayloadAsPerExporterRole(COMPLETE, nil)

		setDataIsExported()
		log.Info("Export of data completed.")

		if !changeStreamingIsEnabled(exportType) {
			printExportDataFooter()
		} else {
			color.Green("Export of data complete")
		}
		startFallBackSetupIfRequired()
	} else if ProcessShutdownRequested {
		log.Info("Shutting down as SIGINT/SIGTERM received.")
	} else {
		color.Red("Export of data failed! Check %s/logs for more details.", exportDir)
		log.Error("Export of data failed.")
		sendPayloadAsPerExporterRole(ERROR, nil)
		atexit.Exit(1)
	}
}

func printExportDataFooter() {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		log.Warnf("failed to get MSR for footer: %v", err)
		return
	}

	dataDir := displayPath(filepath.Join(exportDir, "data"))
	artifacts := []string{dataDir}

	wf := resolveWorkflow(msr)
	phases := computePhaseStatuses(wf, msr, StepExportData)

	configFlag := fmt.Sprintf("--config-file %s", displayPath(cfgFile))

	footer := CommandFooter{
		SectionTitle: "Export Data Summary",
		Title:        "Data export completed successfully.",
		Artifacts:    artifacts,
		NextStepDesc: []string{"Import data into your target YugabyteDB:"},
		NextStepCmd:  fmt.Sprintf("yb-voyager data import-to-target %s", configFlag),
		Phases:       phases,
	}
	printCommandFooter(footer)
}

func printLiveMigrationLimitations() {
	if !changeStreamingIsEnabled(exportType) {
		return
	}
	switch source.DBType {
	case ORACLE:
		utils.PrintAndLogfWarning("Note: Live migration is a TECH PREVIEW feature.")
	default:
		if exporterRole == SOURCE_DB_EXPORTER_ROLE {
			utils.PrintAndLogfWarning("\nImportant: The following limitations apply to live migration:\n")
			utils.PrintAndLogfInfo("  1. Schema modifications(for example, adding/droping columns, creating/deleting tables, adding/deleting partitions etc) on the source and target databases are not supported during live migration.\n")
			utils.PrintAndLogfInfo("  2. Primary Key or Unique Key columns should be identical between source and target databases.\n")
			utils.PrintAndLogfInfo("  3. TRUNCATE operations on source database tables are not automatically replicated to the target database.\n")
			utils.PrintAndLogfInfo("  4. Sequences that are not associated with any column or are attached to columns of non-integer types are not supported for automatic value generation resumption. These sequences must be manually resumed during the cutover phase.\n")
			utils.PrintAndLogfInfo("  5. Tables without a Primary Key are not supported for live migration.\n\n")
		} else {
			workflow := lo.Ternary(exporterRole == TARGET_DB_EXPORTER_FF_ROLE, "fall forward", "fall back")
			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				utils.ErrExit("error getting migration status record: %w", err)
			}
			if msr.UseYBgRPCConnector {
				utils.PrintAndLogfWarning("Note: Live migration with %s using YugabyteDB gRPC connector is a TECH PREVIEW feature.", workflow)
			} else {
				utils.PrintAndLogfWarning("\nImportant: The following limitation applies to live migration with %s:\n\n", workflow)
				utils.PrintAndLogfInfo("  1. SAVEPOINT statements within transactions on the target database are not supported during live migration with %s enabled. Transactions rolling back to some SAVEPOINT may cause data inconsistency between the databases.\n", workflow)
				utils.PrintAndLogfInfo("  2. Rows larger than 4MB in target database can cause consistency issues during live migration with %s enabled. Refer to this tech advisory for more information %s\n", workflow, utils.Path.Sprint("https://docs.yugabyte.com/stable/releases/techadvisories/ta-29060/"))
				utils.PrintAndLogfInfo("  3. Workloads with read-committed isolation level are not fully supported. It is recommended to use repeatable-read or serializable isolation levels for the duration of the migration.\n\n")
			}
		}

	}
}
func sendPayloadAsPerExporterRole(status string, errorMsg error) {
	if !callhome.SendDiagnostics {
		return
	}
	switch exporterRole {
	case SOURCE_DB_EXPORTER_ROLE:
		packAndSendExportDataPayload(status, errorMsg)
	case TARGET_DB_EXPORTER_FB_ROLE, TARGET_DB_EXPORTER_FF_ROLE:
		packAndSendExportDataFromTargetPayload(status, errorMsg)
	}
}

func packAndSendExportDataPayload(status string, errorMsg error) {

	if !shouldSendCallhome() {
		return
	}
	payload := createCallhomePayload()

	switch exportType {
	case SNAPSHOT_ONLY:
		payload.MigrationType = OFFLINE
	case SNAPSHOT_AND_CHANGES:
		payload.MigrationType = LIVE_MIGRATION
	}
	sourceDBDetails := anonymizeSourceDBDetails(&source)

	payload.SourceDBDetails = callhome.MarshalledJsonString(sourceDBDetails)

	payload.MigrationPhase = EXPORT_DATA_PHASE
	exportDataPayload := callhome.ExportDataPhasePayload{
		PayloadVersion:            callhome.EXPORT_DATA_CALLHOME_PAYLOAD_VERSION,
		ParallelJobs:              int64(source.NumConnections),
		StartClean:                bool(startClean),
		Error:                     callhome.SanitizeErrorMsg(errorMsg, anonymizer),
		ControlPlaneType:          getControlPlaneType(),
		AllowOracleClobDataExport: bool(source.AllowOracleClobDataExport),
	}

	updateExportSnapshotDataStatsInPayload(&exportDataPayload)

	if exportDataPayload.ExportSnapshotMechanism == "debezium" {
		exportDataPayload.ParallelJobs = 1 //In case of debezium parallel-jobs is not used as such
	}

	exportDataPayload.Phase = exportPhase
	if exportPhase != dbzm.MODE_SNAPSHOT {
		exportDataPayload.TotalExportedEvents = totalEventCount
		exportDataPayload.EventsExportRate = throughputInLast3Min
	}

	payload.PhasePayload = callhome.MarshalledJsonString(exportDataPayload)
	payload.Status = status

	err := callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func exportData() bool {
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the source db: %w", err)
	}
	defer source.DB().Disconnect()

	if source.RunGuardrailsChecks {
		err = source.DB().CheckSourceDBVersion(exportType)
		if err != nil {
			utils.ErrExit("Source DB version check failed: %w", err)
		}

		binaryCheckIssues, err := checkDependenciesForExport()
		if err != nil {
			utils.ErrExit("check dependencies for export: %w", err)
		} else if len(binaryCheckIssues) > 0 {
			headerStmt := color.RedString("Missing dependencies for export data:")
			utils.PrintAndLogf("\n%s\n%s", headerStmt, strings.Join(binaryCheckIssues, "\n"))
			utils.ErrExit("")
		}
	}

	allSchemas, err := source.DB().GetAllSchemaNamesIdentifiers()
	if err != nil {
		utils.ErrExit("get all schema names identifiers: %w", err)
	}
	//TODO: handle non-start-clean cases for guadrails for not allowing the schema to be changed and use the one stored in MSR
	source.Schemas, err = namereg.SchemaNameMatcher(source.DBType, allSchemas, source.SchemaConfig)
	if err != nil {
		utils.ErrExit("schema name matcher: %w", err)
	}
	
	source.DBVersion = source.DB().GetVersion()
	source.DBSize, err = source.DB().GetDatabaseSize()
	if err != nil {
		log.Errorf("error getting database size: %v", err) //can just log as this is used for call-home only
	}

	// Get PostgreSQL system identifier while still connected
	source.FetchDBSystemIdentifier()

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %w", err)
	}

	if source.DBType == YUGABYTEDB {
		source.IsYBGrpcConnector = msr.UseYBgRPCConnector
	}

	clearMigrationStateIfRequired()

	err = InitNameRegistry(exportDir, exporterRole, &source, source.DB(), nil, nil, false)
	if err != nil {
		utils.ErrExit("initialize name registry: %w", err)
	}

	if source.RunGuardrailsChecks {
		checkIfSchemasHaveUsagePermissions()
	}

	checkSourceDBCharset()
	saveSourceDBConfInMSR()
	saveExportTypeInMSR()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var partitionsToRootTableMap map[string]string
	// get initial table list
	partitionsToRootTableMap, finalTableList, err := getInitialTableList()
	if err != nil {
		var exportErr *errs.ExportDataError
		if errors.As(err, &exportErr) {
			utils.ErrExit(err.Error())
		}
		utils.ErrExit("error in get initial table list: %w", err)
	}

	// Check if source DB has required permissions for export data
	if source.RunGuardrailsChecks {
		checkExportDataPermissions(finalTableList)
	}

	// finalizing table list and column list to be exported based on the datatypes supported by the source DB
	finalTableList, tablesColumnList := finalizeTableAndColumnList(finalTableList)
	handleEmptyTableListForExport(finalTableList)

	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch source.DBType {
		case POSTGRESQL:
			record.SourceRenameTablesMap = partitionsToRootTableMap
		case YUGABYTEDB:
			//Need to keep this as separate field as we have to rename tables in case of partitions for export from target as we do for source but
			//partitions can change on target during migration so need to handle that case
			record.TargetRenameTablesMap = partitionsToRootTableMap
			record.TargetExportedTableListWithLeafPartitions = lo.Map(finalTableList, func(t sqlname.NameTuple, _ int) string {
				return t.ForOutput()
			})
		}
	})

	msr, err = metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %w", err)
	}

	leafPartitions := utils.NewStructMap[sqlname.NameTuple, []sqlname.NameTuple]()
	tableListTuplesToDisplay := lo.Map(finalTableList, func(table sqlname.NameTuple, _ int) sqlname.NameTuple {
		renamedTable, isRenamed := renameTableIfRequired(table.ForOutput())
		if isRenamed {
			//Fine to lookup directly as this will root table in case of partitions
			tuple, err := namereg.NameReg.LookupTableName(renamedTable)
			if err != nil {
				utils.ErrExit("lookup table name: %s: %w", renamedTable, err)
			}
			currPartitions, ok := leafPartitions.Get(tuple)
			if !ok {
				var partitions []sqlname.NameTuple
				partitions = append(partitions, table)
				leafPartitions.Put(tuple, partitions)
			} else {
				currPartitions = append(currPartitions, table)
				leafPartitions.Put(tuple, currPartitions)
			}
			return tuple
		}
		return table
	})
	tableListTuplesToDisplay = lo.UniqBy(tableListTuplesToDisplay, func(table sqlname.NameTuple) string {
		return table.ForKey()
	})

	//handle case of display in case user is filtering few partitions in table-list
	tableListToDisplay := lo.Map(tableListTuplesToDisplay, func(table sqlname.NameTuple, _ int) string {
		partitions, ok := leafPartitions.Get(table)
		if slices.Contains([]string{POSTGRESQL, YUGABYTEDB}, source.DBType) && ok && msr.IsExportTableListSet {
			partitions := strings.Join(lo.Map(partitions, func(partition sqlname.NameTuple, _ int) string {
				return partition.ForOutput()
			}), ", ")
			return fmt.Sprintf("%s (%s)", table.ForOutput(), partitions)
		}
		return table.ForOutput()
	})

	fmt.Printf("num tables to export: %d\n", len(tableListToDisplay))
	utils.PrintAndLogf("table list for data export: %v", tableListToDisplay)

	if source.DBType == POSTGRESQL && !changeStreamingIsEnabled(exportType) {
		utils.PrintAndLogf("Only the sequences that are attached to the above exported tables will be restored during the migration.")
	}

	//finalTableList is with leaf partitions and root tables after this in the whole export flow to make all the catalog queries work fine

	if changeStreamingIsEnabled(exportType) || useDebezium {
		exportPhase = dbzm.MODE_SNAPSHOT
		config, tableNametoApproxRowCountMap, err := prepareDebeziumConfig(partitionsToRootTableMap, finalTableList, tablesColumnList, leafPartitions)
		if err != nil {
			log.Errorf("Failed to prepare dbzm config: %v", err)
			return false
		}
		saveTableToUniqueKeyColumnsMapInMetaDB(finalTableList, leafPartitions)
		if source.DBType == POSTGRESQL && changeStreamingIsEnabled(exportType) {
			// pg live migration. Steps are as follows:
			// 1. create publication, replication slot.
			// 2. export snapshot corresponding to replication slot by passing it to pg_dump
			// 3. start debezium with configration to read changes from the created replication slot, publication.

			if !dataIsExported() { // if snapshot is not already done...
				err = exportPGSnapshotWithPGdump(ctx, cancel, finalTableList, tablesColumnList, leafPartitions)
				if err != nil {
					log.Errorf("export snapshot failed: %v", err)
					return false
				}
			}

			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				utils.ErrExit("get migration status record: %v", err)
			}

			isActive, err := checkIfReplicationSlotIsActive(msr.PGReplicationSlotName)
			if err != nil {
				utils.ErrExit("error checking if replication slot is active: %w", err)
			}
			if isActive {
				errorMsg := fmt.Sprintf("Replication slot '%s' is active", msr.PGReplicationSlotName)
				pid, err := dbzm.GetPIDOfDebeziumOnExportDir(exportDir, exporterRole)
				if err == nil {
					// we have the PID of the process running debezium export
					// so we can suggest the user to terminate it
					errorMsg = fmt.Sprintf("%s. Terminate the process on the pid %s, and re-run the command.", errorMsg, pid)
				} else {
					log.Errorf("error getting debezium PID: %v", err)
				}
				utils.ErrExit(color.RedString("\n%s", errorMsg))
			}

			// Setting up sequence values for debezium to start tracking from..
			sequenceValueMap, err := getPGDumpSequencesAndValues()
			if err != nil {
				utils.ErrExit("get pg dump sequence values: %w", err)
			}

			var sequenceInitValues strings.Builder
			sequenceValueMap.IterKV(func(seqName sqlname.NameTuple, seqValue int64) (bool, error) {
				sequenceInitValues.WriteString(fmt.Sprintf("%s:%d,", seqName.ForKey(), seqValue))
				return true, nil
			})

			config.SnapshotMode = "never"
			config.ReplicationSlotName = msr.PGReplicationSlotName
			config.PublicationName = msr.PGPublicationName
			config.InitSequenceMaxMapping = sequenceInitValues.String()
		}

		// if source.DBType == YUGABYTEDB && !msr.UseYBgRPCConnector {
		// Not having this check right now for the YB as this is not available in all YB versions, TODO: add it later
		// 	msr, err := metaDB.GetMigrationStatusRecord()
		// 	if err != nil {
		// 		utils.ErrExit("get migration status record: %v", err)
		// 	}

		// 	isActive, err := checkIfReplicationSlotIsActive(msr.YBReplicationSlotName)
		// 	if err != nil {
		// 		utils.ErrExit("error checking if replication slot is active: %v", err)
		// 	}
		// 	if isActive {
		// 		utils.ErrExit("Replication slot '%s' is active. Check and terminate if there is any internal voyager process running.", msr.YBReplicationSlotName)
		// 	}
		// }
		err = debeziumExportData(ctx, config, tableNametoApproxRowCountMap)
		if err != nil {
			log.Errorf("Export Data using debezium failed: %v", err)
			return false
		}

		if changeStreamingIsEnabled(exportType) {
			log.Infof("live migration complete, proceeding to cutover")
			msr, err := metaDB.GetMigrationStatusRecord()
			if err != nil {
				utils.ErrExit("get migration status record: %w", err)
			}
			if isTargetDBExporter(exporterRole) {
				if msr.UseYBgRPCConnector {
					err = ybCDCClient.DeleteStreamID()
					if err != nil {
						utils.ErrExit("failed to delete stream id after data export: %w", err)
					}
				} else {
					fmt.Println("Deleting YB replication slot and publication")
					err = deleteYBReplicationSlotAndPublication(msr.YBReplicationSlotName, msr.YBPublicationName, source)
					if err != nil {
						utils.ErrExit("failed to delete replication slot and publication after data export: %w", err)
					}
				}
			}
			if exporterRole == SOURCE_DB_EXPORTER_ROLE {
				if err != nil {
					utils.ErrExit("get migration status record: %w", err)
				}
				fmt.Println("Deleting PG replication slot and publication")
				deletePGReplicationSlot(msr, &source)
				deletePGPublication(msr, &source)
			}

			// mark cutover processed only after cleanup like deleting replication slot and yb cdc stream id
			err = markCutoverProcessed(exporterRole)
			if err != nil {
				utils.ErrExit("failed to create trigger file after data export: %w", err)
			}

			updateCallhomeExportPhase()

			utils.PrintAndLog("\nRun the following command to get the current report of the migration:\n" +
				color.CyanString("yb-voyager get data-migration-report --export-dir %q\n", exportDir))
		}
		return true
	} else {
		exportPhase = dbzm.MODE_SNAPSHOT
		err = storeTableListInMSR(finalTableList)
		if err != nil {
			utils.ErrExit("store table list in MSR: %w", err)
		}
		err = exportDataOffline(ctx, cancel, finalTableList, tablesColumnList, "")
		if err != nil {
			log.Errorf("Export Data failed: %v", err)
			return false
		}
		return true
	}
}

func checkExportDataPermissions(finalTableList []sqlname.NameTuple) {
	// If source is PostgreSQL or YB, check if the number of existing replicaton slots is less than the max allowed
	if (source.DBType == POSTGRESQL && changeStreamingIsEnabled(exportType)) ||
		(source.DBType == YUGABYTEDB && !bool(useYBgRPCConnector)) {
		// The queries used to check replication slots on YB don't throw an error even if logical replication is not supported
		// in that YB version. They are inbuilt PG queries and will return the default values i.e. max allowed slots = 10 and
		// current slots = 0
		// Hence it is safe to check this in YB irrespective of the version.
		isAvailable, usedCount, maxCount, err := source.DB().CheckIfReplicationSlotsAreAvailable()
		if err != nil {
			utils.ErrExit("check replication slots: %w", err)
		}
		if !isAvailable {
			utils.PrintAndLogf("\n%s Current replication slots: %d; Max allowed replication slots: %d\n", color.RedString("ERROR:"), usedCount, maxCount)
			utils.ErrExit("")
		}
	}

	missingPermissions, hasMissingPermissions, err := source.DB().GetMissingExportDataPermissions(exportType, finalTableList)
	if err != nil {
		utils.ErrExit("get missing export data permissions: %w", err)
	}
	if hasMissingPermissions {
		color.Red("\nPermissions and configurations missing in the source database for export data:\n")
		output := strings.Join(missingPermissions, "\n")
		utils.PrintAndLogf("%s\n", output)

		var link string
		if changeStreamingIsEnabled(exportType) {
			link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-migrate/#prepare-the-source-database"
		} else {
			link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
		}
		fmt.Println("\nCheck the documentation to prepare the database for migration:", color.BlueString(link))

		// All missing permissions are critical - no override allowed
		utils.ErrExit("Migration cannot proceed without the required permissions and configurations.")
	} else {
		// TODO: Print this message on the console too once the code is stable
		log.Info("All required permissions are present for the source database.")
	}
}

func checkIfSchemasHaveUsagePermissions() {
	schemasMissingUsage, err := source.DB().GetSchemasMissingUsagePermissions()
	if err != nil {
		utils.ErrExit("get schemas missing usage permissions: %w", err)
	}
	if len(schemasMissingUsage) > 0 {
		utils.PrintAndLogf("\n%s[%s]", color.RedString(fmt.Sprintf("Missing USAGE permission for user %s on Schemas: ", source.User)), strings.Join(schemasMissingUsage, ", "))

		var link string
		if changeStreamingIsEnabled(exportType) {
			link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-migrate/#prepare-the-source-database"
		} else {
			link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
		}
		utils.ErrExit("\nCheck the documentation to prepare the database for migration: %s", color.BlueString(link))
	}
}

func updateCallhomeExportPhase() {
	if !callhome.SendDiagnostics {
		return
	}
	switch exporterRole {
	case SOURCE_DB_EXPORTER_ROLE:
		exportPhase = CUTOVER_TO_TARGET
	case TARGET_DB_EXPORTER_FF_ROLE:
		exportPhase = CUTOVER_TO_SOURCE_REPLICA
	case TARGET_DB_EXPORTER_FB_ROLE:
		exportPhase = CUTOVER_TO_SOURCE
	}

}

// required only for postgresql/yugabytedb since GetAllTables() returns all tables and partitions
// addAllLeafPartitions - this flag helps this function understand if it needs add leaf partitions or the list already consist it
func addLeafPartitionsInTableList(tableList []sqlname.NameTuple, addAllLeafPartitions bool) (map[string]string, []sqlname.NameTuple, error) {
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

	modifiedTableList := []sqlname.NameTuple{}
	var partitionsToRootTableMap = make(map[string]string)

	//TODO: optimisation to avoid multiple calls to DB with one call in the starting to fetch TablePartitionTree map.

	//TODO: test when we upgrade to PG13+ as partitions are handled with root table
	//Refer- https://debezium.zulipchat.com/#narrow/stream/302529-community-general/topic/Connector.20not.20working.20with.20partitions
	for _, table := range tableList {
		qualifiedCatalogName := table.AsQualifiedCatalogName()
		rootTable, err := GetRootTableOfPartition(table)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get root table of partition %s: %w", table.ForKey(), err)
		}
		allLeafPartitions := GetAllLeafPartitions(table)
		prevLengthOfList := len(modifiedTableList)
		switch true {
		case len(allLeafPartitions) == 0 && !rootTable.Equals(table): //leaf partition
			partitionsToRootTableMap[qualifiedCatalogName] = rootTable.AsQualifiedCatalogName() // Unquoted->Unquoted map as debezium uses Unquoted table name
			modifiedTableList = append(modifiedTableList, table)
		case len(allLeafPartitions) == 0 && rootTable.Equals(table): //normal table
			modifiedTableList = append(modifiedTableList, table)
		case len(allLeafPartitions) > 0 && addAllLeafPartitions: // table with partitions in table list
			for _, leafPartition := range allLeafPartitions {
				modifiedTableList = append(modifiedTableList, leafPartition)
				partitionsToRootTableMap[leafPartition.AsQualifiedCatalogName()] = rootTable.AsQualifiedCatalogName()
			}
		}
		if prevLengthOfList < len(modifiedTableList) {
			// will be keeping root in the list if leaf partitions are added for this table as it might be required by some of the catalog queries
			modifiedTableList = append(modifiedTableList, rootTable)
		}
	}
	return partitionsToRootTableMap, lo.UniqBy(modifiedTableList, func(table sqlname.NameTuple) string {
		return table.ForKey()
	}), nil
}

func GetRootTableOfPartition(table sqlname.NameTuple) (sqlname.NameTuple, error) {
	parentTable := source.DB().ParentTableOfPartition(table)
	if parentTable == "" {
		// The original tuples are hand-crafted ones without target names, so for the root,
		//we shouldn't use these but instead get a proper one from the name registry.
		tuple, err := namereg.NameReg.LookupTableName(table.ForKey())
		if err != nil {
			//TODO: fix back to returning error once we remove any DB calls in subsequent even for leaf partitions etc..
			return table, nil
		}
		return tuple, nil
	}

	// non-root table
	tuple := getNameTupleForNonRoot(parentTable)
	return GetRootTableOfPartition(tuple)
}

// For partitions case there is no defined mapping and hence lookup will fail for need to create nametuple for non-root table by hand
func getNameTupleForNonRoot(table string) sqlname.NameTuple {
	parts := strings.Split(table, ".")
	defaultSchemaName, _ := getDefaultSourceSchemaName()
	schema := defaultSchemaName
	tableName := parts[0]
	if len(parts) > 1 {
		schema = parts[0]
		tableName = parts[1]
	}
	//remove quotes if present to pass raw table name to objecName
	schema = strings.Trim(schema, "\"")
	tableName = strings.Trim(tableName, "\"")
	obj := sqlname.NewObjectName(source.DBType, defaultSchemaName, schema, tableName)
	return sqlname.NameTuple{
		SourceName:  obj,
		CurrentName: obj,
	}
}

func GetAllLeafPartitions(table sqlname.NameTuple) []sqlname.NameTuple {
	allLeafPartitions := []sqlname.NameTuple{}
	childPartitions := source.DB().GetPartitions(table)
	for _, childPartition := range childPartitions {
		parititon := getNameTupleForNonRoot(childPartition)
		leafPartitions := GetAllLeafPartitions(parititon)
		if len(leafPartitions) == 0 {
			allLeafPartitions = append(allLeafPartitions, parititon)
		} else {
			allLeafPartitions = append(allLeafPartitions, leafPartitions...)
		}
	}
	return allLeafPartitions
}

func checkIfReplicationSlotIsActive(replicationSlot string) (bool, error) {
	if source.DBType != POSTGRESQL {
		return false, nil
	}
	var isActive bool
	var activePID sql.NullString
	stmt := fmt.Sprintf("select active, active_pid from pg_replication_slots where slot_name='%s'", replicationSlot)
	err := source.DB().QueryRow(stmt).Scan(&isActive, &activePID)
	if err != nil {
		return false, fmt.Errorf("error checking if replication slot is active: %w", err)
	}
	return isActive && (activePID.String != ""), nil
}

func exportPGSnapshotWithPGdump(ctx context.Context, cancel context.CancelFunc, finalTableList []sqlname.NameTuple, tablesColumnList *utils.StructMap[sqlname.NameTuple, []string], leafPartitions *utils.StructMap[sqlname.NameTuple, []sqlname.NameTuple]) error {
	// create replication slot
	pgDB := source.DB().(*srcdb.PostgreSQL)
	replicationConn, err := pgDB.GetReplicationConnection()
	if err != nil {
		return fmt.Errorf("export snapshot: failed to create replication connection: %w", err)
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
	err = pgDB.CreatePublication(replicationConn, publicationName, finalTableList, true, leafPartitions)
	if err != nil {
		return fmt.Errorf("create publication: %w", err)
	}
	replicationSlotName := fmt.Sprintf("voyager_%s", strings.ReplaceAll(migrationUUID.String(), "-", "_"))
	res, err := pgDB.CreateLogicalReplicationSlot(replicationConn, replicationSlotName, true)
	if err != nil {
		return fmt.Errorf("export snapshot: failed to create replication slot: %w", err)
	}
	yellowBold := color.New(color.FgYellow, color.Bold)
	utils.PrintAndLog(yellowBold.Sprintf("Created replication slot '%s' on source PG database. "+
		"Be sure to run 'initiate cutover to target' or 'end migration' command after completing/aborting this migration to drop the replication slot. "+
		"This is important to avoid filling up disk space.", replicationSlotName))

	// save replication slot, publication name in MSR
	err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.PGReplicationSlotName = res.SlotName
		record.SnapshotMechanism = "pg_dump"
		record.PGPublicationName = publicationName
	})
	if err != nil {
		utils.ErrExit("update PGReplicationSlotName: update migration status record: %w", err)
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

func getPGDumpSequencesAndValues() (*utils.StructMap[sqlname.NameTuple, int64], error) {
	result := utils.NewStructMap[sqlname.NameTuple, int64]()
	path := filepath.Join(exportDir, "data", "postdata.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
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
			return nil, goerrors.Errorf("invalid index %d for matches - %s for line %s", argsIdx, matches, line)
		}
		args := strings.Split(matches[argsIdx], ",")

		seqNameRaw := args[0][1 : len(args[0])-1]
		seqName, err := namereg.NameReg.LookupTableName(seqNameRaw)
		if err != nil {
			return nil, fmt.Errorf("lookup for sequence name %s: %w", seqNameRaw, err)
		}

		seqVal, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %s to int in line - %s: %w", args[1], line, err)
		}

		isCalled := strings.TrimSpace(args[2])
		if isCalled == "false" {
			// we always restore sequences with is_called=true, therefore, we need to minus 1.
			seqVal--
		}

		result.Put(seqName, seqVal)
	}
	return result, nil
}

// Return the nameTuple of the qualfieidObj string
func getNameTupleFromQualifiedObject(qualifiedObjectStr string, qualifiedObjectName *sqlname.ObjectName, goToNameRegDirectly bool) (sqlname.NameTuple, error) {
	sourceTypeHandlesPartitionAsSeparateTable := func(dbType string) bool {
		return dbType == POSTGRESQL || dbType == YUGABYTEDB
	}
	if !sourceTypeHandlesPartitionAsSeparateTable(source.DBType) || goToNameRegDirectly {
		//ORACLE and MySQL no need to care about leaf partitions
		tuple, err := namereg.NameReg.LookupTableName(qualifiedObjectStr)
		if err != nil {
			return sqlname.NameTuple{}, fmt.Errorf("lookup for table name failed err: %s: %w", qualifiedObjectStr, err)
		}
		return tuple, nil
	}
	//Now for PG/YB create a ObjectName and a NameTuple by hand and then check if that is a partition table or not
	var obj *sqlname.ObjectName
	if qualifiedObjectName != nil {
		//if passed in parameter then take it else create one
		obj = qualifiedObjectName
	} else {
		//create the ObjectName by hand
		defaultSchemaName, _ := getDefaultSourceSchemaName()
		obj = sqlname.NewObjectNameWithQualifiedName(source.DBType, defaultSchemaName, qualifiedObjectStr)
	}
	//get the name tupe case its leaf partition return a handcrafted NameTuple else return  from nameReg lookup
	var err error
	tuple := sqlname.NameTuple{
		SourceName:  obj,
		CurrentName: obj,
	}
	parent := source.DB().ParentTableOfPartition(tuple)

	if parent == "" {
		tuple, err = namereg.NameReg.LookupTableName(fmt.Sprintf("%s.%s", obj.SchemaName.Unquoted, obj.Unqualified.Unquoted))
		if err != nil {
			return sqlname.NameTuple{}, fmt.Errorf("lookup for table name failed err: %s: %w", obj.Unqualified, err)
		}
	}
	return tuple, nil
}

func applyTableListFlagsOnFullListAndAddLeafPartitions(fullTableList []sqlname.NameTuple, tableListViaFlag string, excludeTableListViaFlag string) ([]sqlname.NameTuple, []sqlname.NameTuple, error) {
	var err error
	var includeTableList, excludeTableList []sqlname.NameTuple

	applyFilterAndAddLeafTable := func(flagList string, flagName string) ([]sqlname.NameTuple, error) {
		log.Infof("applying filter for %s list of tables and adding leaf tables - %s", flagName, flagList)
		flagTableList, err := extractTableListFromString(fullTableList, flagList, flagName)
		if err != nil {
			return nil, errs.NewExportDataError(errs.APPLY_TABLE_LIST_FLAGS_ON_FULL_LIST, fmt.Sprintf("extract_table_list_from_%s_list", flagName), err)
		}
		_, flagTableList, err = addLeafPartitionsInTableList(flagTableList, true)
		if err != nil {
			return nil, errs.NewExportDataError(errs.APPLY_TABLE_LIST_FLAGS_ON_FULL_LIST, fmt.Sprintf("add_leaf_partitions_in_%s_list", flagName), err)
		}
		log.Infof("filtered %s list of tables and added leaf tables - %v", flagName, lo.Map(flagTableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		}))
		return flagTableList, nil
	}

	if excludeTableListViaFlag != "" {
		//Apply exclude table list filter if present
		excludeTableList, err = applyFilterAndAddLeafTable(excludeTableListViaFlag, "exclude")
		if err != nil {
			return nil, nil, err
		}
	}

	includeTableList = fullTableList
	if tableListViaFlag != "" {
		//Apply include table list filter if present
		includeTableList, err = applyFilterAndAddLeafTable(tableListViaFlag, "include")
		if err != nil {
			return nil, nil, err
		}
	} else {
		//this is only for removing  the mid level partitioned table from fullTableList
		//by passing false in `addAllLeafPartitions` boolean flag which  means this function won't the leaf parittions again it will just filter the list based on type
		//i.e. only add if its a normal, root, or leaf table.
		log.Infof("keeping only leaf and root tables in full table list - %v", lo.Map(includeTableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		}))
		_, includeTableList, err = addLeafPartitionsInTableList(includeTableList, false)
		if err != nil {
			return nil, nil, errs.NewExportDataError(errs.APPLY_TABLE_LIST_FLAGS_ON_FULL_LIST, "add_leaf_partitions", err)
		}
		log.Infof("filtered full table list and added leaf tables - %v", lo.Map(includeTableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		}))
	}
	//return the include and exclude table list generated from the command flags  table-list (include list) / exclude-table-list in this

	return includeTableList, excludeTableList, nil
}

func fetchTablesNamesFromSourceAndFilterTableList() (map[string]string, []sqlname.NameTuple, error) {
	var tableListInFirstRun []sqlname.NameTuple
	var nameTupleTableListFromDB []sqlname.NameTuple
	var err error
	tableListFromDB := source.DB().GetAllTableNames()
	for _, t := range tableListFromDB {
		tuple, err := getNameTupleFromQualifiedObject(t.Qualified.Quoted, nil, false)
		if err != nil {
			return nil, nil, errs.NewExportDataError(errs.FETCH_TABLES_NAMES_FROM_SOURCE, "get_name_tuple_from_qualified_object", err)
		}
		nameTupleTableListFromDB = append(nameTupleTableListFromDB, tuple)
	}
	log.Infof("fetched tables from source - %v", lo.Map(nameTupleTableListFromDB, func(t sqlname.NameTuple, _ int) string {
		return t.ForOutput()
	}))

	//apply table list flags filter on the nameTupleTableListFromDB
	includeTableList, excludeTableList, err := applyTableListFlagsOnFullListAndAddLeafPartitions(nameTupleTableListFromDB, source.TableList, source.ExcludeTableList)
	if err != nil {
		return nil, nil, propagateIfExportDataError(err, errs.FETCH_TABLES_NAMES_FROM_SOURCE)
	}
	tableListInFirstRun = sqlname.SetDifferenceNameTuples(includeTableList, excludeTableList)

	isTableListModified := false
	if source.TableList != "" || source.ExcludeTableList != "" {
		isTableListModified = len(sqlname.SetDifferenceNameTuples(nameTupleTableListFromDB, tableListInFirstRun)) != 0
	}
	if exporterRole == SOURCE_DB_EXPORTER_ROLE {
		metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			if isTableListModified {
				record.IsExportTableListSet = true
			} else {
				record.IsExportTableListSet = false
			}
		})
	}

	var partitionsToRootTableMap map[string]string
	//Just populating the partitionsToRootTableMap from the finalTableList which is filtered from flags if required and has leaf and roots both
	partitionsToRootTableMap, tableListInFirstRun, err = addLeafPartitionsInTableList(tableListInFirstRun, false)
	if err != nil {
		return nil, nil, errs.NewExportDataError(errs.FETCH_TABLES_NAMES_FROM_SOURCE, "add_leaf_partitions", err)
	}
	log.Infof("filtered table list in first run - %v", lo.Map(tableListInFirstRun, func(t sqlname.NameTuple, _ int) string {
		return t.ForOutput()
	}))
	log.Infof("partitions of all root table being exported: %v", partitionsToRootTableMap)
	return partitionsToRootTableMap, tableListInFirstRun, nil
}

func retrieveFirstRunListAndPartitionsRootMap(msr *metadb.MigrationStatusRecord) ([]sqlname.NameTuple, map[string]string, error) {
	var firstRunTableWithLeafsAndRoots []sqlname.NameTuple
	var partitionsToRootTableMap map[string]string
	var err error
	storedTableList := make([]string, 0)
	isFirstRunOfTargetExporter := false
	fetchNameTupleFromNameRegDirectly := true
	switch source.DBType {
	case ORACLE, MYSQL:
		storedTableList = msr.TableListExportedFromSource
	case POSTGRESQL:
		storedTableList = msr.SourceExportedTableListWithLeafPartitions
		partitionsToRootTableMap = msr.SourceRenameTablesMap
		//In case of PG we need to check the leaf condititon for the tables in table list first and then return the nametuple
		//via namereg or handcrafted one
		fetchNameTupleFromNameRegDirectly = false
	case YUGABYTEDB:

		//In case of YB we need to check the leaf condititon for the tables in table list first and then return the nametuple
		//via namereg or handcrafted one
		fetchNameTupleFromNameRegDirectly = false
		// On subsequent run after the first we will use the stored table with leaf partitions
		storedTableList = msr.TargetExportedTableListWithLeafPartitions
		partitionsToRootTableMap = msr.TargetRenameTablesMap
		isFirstRunOfTargetExporter = len(msr.TargetExportedTableListWithLeafPartitions) == 0 || msr.TargetRenameTablesMap == nil
		//For the first run of export data from target we use the TableListExportedFromSource (which has only root tables)
		if isFirstRunOfTargetExporter {
			storedTableList = msr.TableListExportedFromSource
			if msr.SourceDBConf.DBType != POSTGRESQL {
				//but in case we are using this source list and its source is not PG then we can directly use the namereg
				fetchNameTupleFromNameRegDirectly = true
			}

		}

	}

	log.Infof("stored table list from meta db - %v", storedTableList)

	for _, table := range storedTableList {
		tuple, err := getNameTupleFromQualifiedObject(table, nil, fetchNameTupleFromNameRegDirectly)
		if err != nil {
			return nil, nil, errs.NewExportDataError(errs.RETRIEVE_FIRST_RUN_TABLE_LIST_OPERATION, "get_name_tuple_from_qualified_object", err)
		}
		firstRunTableWithLeafsAndRoots = append(firstRunTableWithLeafsAndRoots, tuple)
	}

	if source.DBType == YUGABYTEDB {
		//For the first run of export data from target we will fetch the leaf partitions from target and store them
		if isFirstRunOfTargetExporter {
			//Now add the leaf partitions to the stored table-list for the first run of export data from target
			// as it will only have root table names and get the partitionsToRootTableMap
			partitionsToRootTableMap, firstRunTableWithLeafsAndRoots, err = addLeafPartitionsInTableList(firstRunTableWithLeafsAndRoots, true)
			if err != nil {
				return nil, nil, errs.NewExportDataError(errs.RETRIEVE_FIRST_RUN_TABLE_LIST_OPERATION, "add_leaf_partitions", err)
			}
		}
	}
	return firstRunTableWithLeafsAndRoots, partitionsToRootTableMap, nil
}

func getInitialTableList() (map[string]string, []sqlname.NameTuple, error) {
	// store table list after filtering unsupported or unnecessary tables
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error fetching migration status record: %w", err)
	}
	storedTableListNotAvailable := func() bool {
		//if any of the list in DB is empty
		return len(msr.TableListExportedFromSource) == 0 || len(msr.SourceExportedTableListWithLeafPartitions) == 0
	}

	if bool(startClean) || storedTableListNotAvailable() {
		log.Infof("fetching tables names from source and filtering table list")
		// fresh start case or the first run where we don't have a table list stored in msr
		partitionsToRootTableMap, finalTableList, err := fetchTablesNamesFromSourceAndFilterTableList()
		if err != nil {
			return nil, nil, propagateIfExportDataError(err, errs.GET_INITIAL_TABLE_LIST_OPERATION)
		}
		return partitionsToRootTableMap, finalTableList, nil
	}

	//sunsequent run case where we will use a table-list stored in msr
	var firstRunTableWithLeafsAndRoots []sqlname.NameTuple
	var partitionsToRootTableMap map[string]string
	log.Infof("retrieving first run list and partitions root map")

	firstRunTableWithLeafsAndRoots, partitionsToRootTableMap, err = retrieveFirstRunListAndPartitionsRootMap(msr)
	if err != nil {
		return nil, nil, propagateIfExportDataError(err, errs.GET_INITIAL_TABLE_LIST_OPERATION)
	}

	//guardrails around the table-list in case of re-run

	registeredList, err := getRegisteredNameRegList()
	if err != nil {
		return nil, nil, errs.NewExportDataError(errs.GET_INITIAL_TABLE_LIST_OPERATION, "get_registered_name_registry_list", err)
	}

	rootTables := make([]sqlname.NameTuple, 0)
	for _, v := range partitionsToRootTableMap {
		tuple, err := namereg.NameReg.LookupTableName(v)
		if err != nil {
			utils.ErrExit("look up failed for the table name: %s: %w", v, err)
		}
		rootTables = append(rootTables, tuple)
	}
	rootTables = lo.UniqBy(rootTables, func(t sqlname.NameTuple) string {
		return t.ForKey()
	})

	// Finding all the partitions of all root tables part of migration, and report if there any new partitions added
	rootToNewLeafTablesMap, err := detectAndReportNewLeafPartitionsOnPartitionedTables(rootTables, registeredList)
	if err != nil {
		return nil, nil, propagateIfExportDataError(err, errs.GET_INITIAL_TABLE_LIST_OPERATION)
	}

	firstRunTableWithLeafParititons, currentRunTableListWithLeafPartitions, err := applyTableListFlagsOnCurrentAndRemoveRootsFromBothLists(registeredList, source.TableList, source.ExcludeTableList, rootToNewLeafTablesMap, rootTables, firstRunTableWithLeafsAndRoots)
	if err != nil {
		return nil, nil, propagateIfExportDataError(err, errs.GET_INITIAL_TABLE_LIST_OPERATION)
	}
	//Reporting the guardrail msgs only on leaf tables to be consistent so filtering the root table from both the list
	_, _, err = guardrailsAroundFirstRunAndCurrentRunTableList(firstRunTableWithLeafParititons, currentRunTableListWithLeafPartitions)
	if err != nil {
		//Directly erroring out here as we want to fail if guardrails checks fail
		utils.ErrExit(err.Error())
	}

	return partitionsToRootTableMap, firstRunTableWithLeafsAndRoots, nil

}

func applyTableListFlagsOnCurrentAndRemoveRootsFromBothLists(
	registeredList []sqlname.NameTuple,
	tableListViaFlag string,
	excludeTableListViaFlag string,
	rootToNewLeafTablesMap map[string][]string,
	rootTables []sqlname.NameTuple,
	firstRunTableWithLeafsAndRoots []sqlname.NameTuple) ([]sqlname.NameTuple, []sqlname.NameTuple, error) {

	//apply include/exclude flags and if a new table is passed (which is not present in name registry), then error out Unknown table
	currentRunIncludeTableList, currentRunExlcudeTableList, err := applyTableListFlagsOnFullListAndAddLeafPartitions(registeredList, tableListViaFlag, excludeTableListViaFlag)
	if err != nil {
		return nil, nil, propagateIfExportDataError(err, errs.APPLY_TABLE_LIST_FLAGS_ON_SUBSEQUENT_RUN)
	}
	//Filtering the include and exclude list here using the ForKey() because we are using the LookupTableNameAndIgnoreOtherSideMappingIfNotFound for the Registered list
	//Which will populate the NameTuple for all the tables with both sides in case available (including the partitions) and if not available then only one side.
	//but the addLeafPartitions function still adds the leafs with only Source side populated so in case such cases the String comparision won't help so we need to do the Key based Differences
	currentRunTableListFilteredViaFlags := sqlname.SetDifferenceNameTuplesWithKey(currentRunIncludeTableList, currentRunExlcudeTableList)
	//checks if a given table is new leaf table or not
	isNewLeafTable := func(t sqlname.NameTuple) bool {
		for _, leafs := range rootToNewLeafTablesMap {
			if slices.Contains(leafs, t.AsQualifiedCatalogName()) {
				return true
			}
		}
		return false
	}

	var currentRunTableListWithLeafPartitions []sqlname.NameTuple
	for _, t := range currentRunTableListFilteredViaFlags {
		if lo.ContainsBy(rootTables, func(r sqlname.NameTuple) bool {
			return t.Equals(r)
		}) {

			//Remove root table
			continue
		}

		//TODO: Not go to db for leaf partitions in subsequent for applying table-list  re-use the PartitionsToRootMap (by populating it with exhaustive list)
		//of either by handling it in with nameregistry or something.
		//Filtering the new leaf tables if present in filteredListWithoutRootTable as we have reported above
		if isNewLeafTable(t) {
			//remove the new leaf table as reported above already
			continue
		}

		//Add the tuple if its not root table or new Leaf Table
		currentRunTableListWithLeafPartitions = append(currentRunTableListWithLeafPartitions, t)
	}

	firstRunTableWithLeafParititons := lo.Filter(firstRunTableWithLeafsAndRoots, func(t sqlname.NameTuple, _ int) bool {
		return !lo.ContainsBy(rootTables, func(root sqlname.NameTuple) bool {
			return t.Equals(root)
		})
	})

	return firstRunTableWithLeafParititons, currentRunTableListWithLeafPartitions, nil
}

func getRegisteredNameRegList() ([]sqlname.NameTuple, error) {
	registeredList, err := namereg.NameReg.GetRegisteredTableList(true)
	if err != nil {
		utils.ErrExit("error getting registered list in name registry: %w", err)
	}

	return registeredList, nil
}

func guardrailsAroundFirstRunAndCurrentRunTableList(firstRunTableListWithLeafPartitions, currentRunTableListWithLeafPartitions []sqlname.NameTuple) ([]sqlname.NameTuple, []sqlname.NameTuple, error) {
	missingTables := sqlname.SetDifferenceNameTuplesWithKey(firstRunTableListWithLeafPartitions, currentRunTableListWithLeafPartitions)
	extraTables := sqlname.SetDifferenceNameTuplesWithKey(currentRunTableListWithLeafPartitions, firstRunTableListWithLeafPartitions)
	displayList := func(list []sqlname.NameTuple) string {
		return strings.Join(lo.Map(list, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		}), ",")
	}

	if len(missingTables) > 0 || len(extraTables) > 0 {
		changeListMsg := color.RedString("Changing the table list during live-migration is not allowed.")
		missingMsgFinal := ""
		extraMsgFinal := ""
		if len(missingTables) > 0 {
			missingMsg := color.YellowString("Missing tables in the current run compared to the initial list")
			missingMsgFinal = fmt.Sprintf("\n%s: [%v]", missingMsg, displayList(missingTables))
		}
		if len(extraTables) > 0 {
			extraMsg := color.YellowString("Extra tables in the current run compared to the initial list")
			extraMsgFinal = fmt.Sprintf("\n%s: [%v]", extraMsg, displayList(extraTables))
		}
		tableListInFirstRunMsg := fmt.Sprintf("\n%s: [%v]", color.YellowString("Table list passed in the initial run of migration"), displayList(firstRunTableListWithLeafPartitions))
		reRunMsg := color.YellowString("Re-run the command with the table list passed in the initial run of migration or start a fresh migration.")
		//The following error msg is in format -
		/*
			Changing the table list during live-migration is not allowed.


			Missing tables in the current run compared to the initial list: [<list>]
			Extra tables in the current run compared to the initial list: [<list>]
			Table list passed in the initial run of migration: - [<list>]

			Re-run the command with the table list passed in the initial run of migration or start a fresh migration.
		*/
		return missingTables, extraTables, goerrors.Errorf("\n%s\n\n%s%s%s\n\n%s", changeListMsg, missingMsgFinal, extraMsgFinal, tableListInFirstRunMsg, reRunMsg)
	}

	return nil, nil, nil

}

func detectAndReportNewLeafPartitionsOnPartitionedTables(rootTables []sqlname.NameTuple, registeredList []sqlname.NameTuple) (map[string][]string, error) {
	updatedPartitionsToRootTableMap, _, err := addLeafPartitionsInTableList(rootTables, true)
	if err != nil {
		return nil, errs.NewExportDataError(errs.DETECT_AND_REPORT_NEW_LEAF_PARTITIONS_ON_PARTITIONED_TABLES, "add_leaf_partitions", err)
	}

	rootToNewLeafTablesMap := make(map[string][]string)
	for leaf, rootTable := range updatedPartitionsToRootTableMap {
		if !lo.ContainsBy(registeredList, func(tbl sqlname.NameTuple) bool { //see if
			return tbl.AsQualifiedCatalogName() == leaf
		}) {
			//If this leaf table is not registered in name registry then it is a newly added leaf tables
			rootToNewLeafTablesMap[rootTable] = append(rootToNewLeafTablesMap[rootTable], leaf)
		}
	}

	//TODO: also detect this during ongoing command as well - ticket to track https://github.com/yugabyte/yb-voyager/issues/2356
	if len(lo.Keys(rootToNewLeafTablesMap)) > 0 {
		utils.PrintAndLogf("Detected new partition tables for the following partitioned tables. These will not be considered during migration:")
		listToPrint := ""
		for k, leafs := range rootToNewLeafTablesMap {
			listToPrint += fmt.Sprintf("Root table: %s, new leaf partitions: %s\n", k, strings.Join(leafs, ", "))
		}
		utils.PrintAndLog(listToPrint)
		msg := "Do you want to continue?"
		if !utils.AskPrompt(msg) {
			utils.ErrExit("Aborting, Start a fresh migration if required...")
		}
	}
	return rootToNewLeafTablesMap, nil
}

func propagateIfExportDataError(err error, currentFlow string) error {
	var exportDataErr *errs.ExportDataError
	if errors.As(err, &exportDataErr) {
		exportDataErr.AddCall(exportDataErr.CurrentFlow())
		return errs.NewExportDataErrorWithCompletedCalls(currentFlow, exportDataErr.CallExecutionHistory(), exportDataErr.FailedStep(), exportDataErr.Unwrap())
	}
	return fmt.Errorf("error in %s: %w", currentFlow, err)

}

func exportDataOffline(ctx context.Context, cancel context.CancelFunc, finalTableList []sqlname.NameTuple, tablesColumnList *utils.StructMap[sqlname.NameTuple, []string], snapshotName string) error {
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
			utils.ErrExit("yb-voyager encountered internal error: "+
				"Check: %s/logs/yb-voyager-export-data.log for more details.", exportDir)
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
		utils.ErrExit("update PGReplicationSlotName: update migration status record: %w", err)
	}

	log.Infof("Export table metadata: %s", spew.Sdump(tablesProgressMetadata))
	UpdateTableApproxRowCount(&source, exportDir, tablesProgressMetadata)

	if source.DBType == POSTGRESQL {
		//need to export setval() calls to resume sequence value generation
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error getting migration status record: %w", err)
		}
		colToSeqMap, err := fetchOrRetrieveColToSeqMap(msr, finalTableList)
		if err != nil {
			utils.ErrExit("error fetching the column to sequence mapping: %w", err)
		}
		for _, seq := range colToSeqMap {
			seqTuple, err := namereg.NameReg.LookupTableName(seq)
			if err != nil {
				utils.ErrExit("lookup for sequence failed: %s: err: %w", seq, err)
			}
			finalTableList = append(finalTableList, seqTuple)
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
	}
	displayExportedRowCountSnapshot(false)

	// YDB control plane is only implemented for offline migration as of now
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
			utils.ErrExit("Error invalid table name: '%s' provided with --%s flag", table, flagName)
		}
	}
}

// flagName can be "exclude-table-list-file-path" or "table-list-file-path"
func validateAndExtractTableNamesFromFile(filePath string, flagName string) (string, error) {
	if filePath == "" {
		return "", nil
	}
	if !utils.FileOrFolderExists(filePath) {
		return "", goerrors.Errorf("path %q does not exist", filePath)
	}
	tableList, err := utils.ReadTableNameListFromFile(filePath)
	if err != nil {
		return "", fmt.Errorf("reading table list from file: %w", err)
	}

	tableNameRegex := regexp.MustCompile(`[a-zA-Z0-9_."]+`)
	for _, table := range tableList {
		if !tableNameRegex.MatchString(table) {
			return "", goerrors.Errorf("invalid table name '%s' provided in file %s with --%s flag", table, filePath, flagName)
		}
	}
	return strings.Join(tableList, ","), nil
}

func clearMigrationStateIfRequired() {
	log.Infof("clearing migration state if required: %t", startClean)
	exportDataDir := filepath.Join(exportDir, "data")
	propertiesFilePath := filepath.Join(exportDir, "metainfo", "conf", "application.properties")
	sslDir := filepath.Join(exportDir, "metainfo", "ssl")
	exportSnapshotStatusFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportSnapshotStatusFile := jsonfile.NewJsonFile[ExportSnapshotStatus](exportSnapshotStatusFilePath)
	dfdFilePath := exportDir + datafile.DESCRIPTOR_PATH
	if startClean {
		if dataIsExported() {
			if !utils.AskPrompt("Data is already exported. Are you sure you want to clean the data directory and start afresh") {
				utils.ErrExit("Export aborted.")
			}
		}
		utils.CleanDir(exportDataDir)
		utils.CleanDir(sslDir)
		clearDataIsExported()
		err := os.Remove(dfdFilePath)
		if err != nil && !os.IsNotExist(err) {
			utils.ErrExit("Failed to remove data file descriptor: %w", err)
		}
		err = os.Remove(propertiesFilePath)
		if err != nil && !os.IsNotExist(err) {
			utils.ErrExit("Failed to remove properties file: %w", err)
		}
		err = exportSnapshotStatusFile.Delete()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			utils.ErrExit("Failed to remove export snapshot status file: %w", err)
		}
		nameregFile := filepath.Join(exportDir, "metainfo", "name_registry.json")
		err = os.Remove(nameregFile)
		if err != nil && !os.IsNotExist(err) {
			utils.ErrExit("Failed to remove name registry file: %w", err)
		}

		err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.TableListExportedFromSource = nil
			record.SourceExportedTableListWithLeafPartitions = nil
			record.SourceRenameTablesMap = nil
			record.SourceColumnToSequenceMapping = nil
			record.TargetExportedTableListWithLeafPartitions = nil
			record.TargetColumnToSequenceMapping = nil
			record.TargetRenameTablesMap = nil
		})

		err = metadb.TruncateTablesInMetaDb(exportDir, []string{metadb.QUEUE_SEGMENT_META_TABLE_NAME, metadb.EXPORTED_EVENTS_STATS_TABLE_NAME, metadb.EXPORTED_EVENTS_STATS_PER_TABLE_TABLE_NAME})
		if err != nil {
			utils.ErrExit("Failed to truncate tables in metadb: %w", err)
		}
	} else {
		if !utils.IsDirectoryEmpty(exportDataDir) {
			if (changeStreamingIsEnabled(exportType)) &&
				dbzm.IsMigrationInStreamingMode(exportDir) {
				utils.PrintAndLogf("Continuing streaming from where we left off...")
			} else {
				utils.ErrExit("data directory is not empty, use --start-clean flag to clean the directories and start: %s", exportDir)
			}
		}
	}
}

func getDefaultSourceSchemaName() (string, bool) {
	switch source.DBType {
	case MYSQL:
		return source.DBName, false
	case POSTGRESQL, YUGABYTEDB:
		return GetDefaultPGSchema(source.Schemas)
	case ORACLE:
		return source.Schemas[0].Unquoted, false
	default:
		panic("invalid db type")
	}
}

func extractTableListFromString(fullTableList []sqlname.NameTuple, flagTableList string, listName string) ([]sqlname.NameTuple, error) {
	result := []sqlname.NameTuple{}
	if flagTableList == "" {
		return result, nil
	}
	findPatternMatchingTables := func(pattern string) []sqlname.NameTuple {
		result := lo.Filter(fullTableList, func(tableName sqlname.NameTuple, _ int) bool {
			ok, err := tableName.MatchesPattern(pattern)
			if err != nil {
				utils.ErrExit("Invalid table name pattern: %q: %w", pattern, err)
			}
			return ok
		})
		return result
	}
	tableList := utils.CsvStringToSlice(flagTableList)
	var unknownTableNames []string
	for _, pattern := range tableList {
		tables := findPatternMatchingTables(pattern)
		if len(tables) == 0 {
			unknownTableNames = append(unknownTableNames, pattern)
		}
		result = append(result, tables...)
	}
	if len(unknownTableNames) > 0 {
		return nil, errs.NewUnknownTableErr(listName, unknownTableNames, lo.Map(fullTableList, func(tableName sqlname.NameTuple, _ int) string {
			return tableName.ForOutput()
		}))
	}
	return lo.UniqBy(result, func(tableName sqlname.NameTuple) string {
		return tableName.ForKey()
	}), nil
}

func checkSourceDBCharset() {
	// If source db does not use unicode character set, ask for confirmation before
	// proceeding for export.
	charset, err := source.DB().GetCharset()
	if err != nil {
		utils.PrintAndLogf("[WARNING] Failed to find character set of the source db: %s", err)
		return
	}
	log.Infof("Source database charset: %q", charset)
	if !strings.Contains(strings.ToLower(charset), "utf") {
		utils.PrintAndLogf("voyager supports only unicode character set for source database. "+
			"But the source database is using '%s' character set. ", charset)
		if !utils.AskPrompt("Are you sure you want to proceed with export? ") {
			utils.ErrExit("Export aborted.")
		}
	}
}

func changeStreamingIsEnabled(s string) bool {
	return (s == CHANGES_ONLY || s == SNAPSHOT_AND_CHANGES)
}

func getTableNameToApproxRowCountMap(tableList []sqlname.NameTuple) map[string]int64 {
	tableNameToApproxRowCountMap := make(map[string]int64)
	for _, table := range tableList {
		tableNameToApproxRowCountMap[table.ForKey()] = source.DB().GetTableApproxRowCount(table)
	}
	return tableNameToApproxRowCountMap
}

func filterTableWithEmptySupportedColumnList(finalTableList []sqlname.NameTuple, tablesColumnList *utils.StructMap[sqlname.NameTuple, []string]) []sqlname.NameTuple {
	filteredTableList := lo.Reject(finalTableList, func(tableName sqlname.NameTuple, _ int) bool {
		column, ok := tablesColumnList.Get(tableName)
		return len(column) == 0 || !ok
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
		"--log-level", config.LogLevel,
	}
	if utils.DoNotPrompt {
		cmd = append(cmd, "--yes")
	}

	passImportDataToSourceSpecificCLIFlags := map[string]bool{
		"disable-pb": true,
	}

	// Check whether the command specifc flags have been set in the config file
	keysSetInConfig, err := readConfigFileAndGetImportDataToSourceKeys()
	var displayCmdAndExit bool
	var configFileErr error
	if err != nil {
		displayCmdAndExit = true
		configFileErr = err
	} else {
		for key := range passImportDataToSourceSpecificCLIFlags {
			if slices.Contains(keysSetInConfig, key) {
				passImportDataToSourceSpecificCLIFlags[key] = false
			}
		}
	}

	// Log which command specific flags are to be passed to the command
	log.Infof("Command specific flags to be passed to import data to source: %v", passImportDataToSourceSpecificCLIFlags)

	// Command specific flags
	if bool(disablePb) && passImportDataToSourceSpecificCLIFlags["disable-pb"] {
		cmd = append(cmd, "--disable-pb=true")
	}

	cmdStr := "SOURCE_DB_PASSWORD=*** " + strings.Join(cmd, " ")

	utils.PrintAndLogf("Starting import data to source with command:\n %s", color.GreenString(cmdStr))

	// If error had occurred while reading the config file, display the command and exit
	if displayCmdAndExit {
		// We are delaying this error message to be displayed here so that we can display the command
		// after it has been constructed
		utils.ErrExit("failed to read config file: %w\nPlease check the config file and re-run the command with only the required flags", configFileErr)
	}

	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager: %w", err)
	}
	env := os.Environ()
	env = slices.Insert(env, 0, "SOURCE_DB_PASSWORD="+source.Password)

	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager import data to source: %w\n Please re-run with command :\n%s", err, cmdStr)
	}
}

// ================================ Export Data table list filtering ================================

// Finalize table and column lists for export, based on migration phase (offline/live) and DB type.
func finalizeTableAndColumnList(finalTableList []sqlname.NameTuple) ([]sqlname.NameTuple, *utils.StructMap[sqlname.NameTuple, []string]) {
	reportUnsupportedTablesForLiveMigration(finalTableList)
	log.Infof("initial all tables table list for data export: %v", lo.Map(finalTableList, func(t sqlname.NameTuple, _ int) string {
		return t.ForOutput()
	}))

	log.Info("finalizing table list by filtering empty tables and unsupported tables/columns")

	// 1. empty tables check for offline migration only; live can have events for empty tables
	if !changeStreamingIsEnabled(exportType) {
		var emptyTableList []sqlname.NameTuple
		finalTableList, emptyTableList = source.DB().FilterEmptyTables(finalTableList)
		if len(emptyTableList) != 0 {
			utils.PrintAndLogf("skipping empty tables: %v", lo.Map(emptyTableList, func(table sqlname.NameTuple, _ int) string {
				return table.ForOutput()
			}))
		}
	}

	// 2. filtering unsupported tables(mostly oracle specific)
	var unsupportedTableList []sqlname.NameTuple
	finalTableList, unsupportedTableList = source.DB().FilterUnsupportedTables(migrationUUID, finalTableList, useDebezium)
	if len(unsupportedTableList) != 0 {
		utils.PrintAndLogf("skipping unsupported tables: %v", lo.Map(unsupportedTableList, func(table sqlname.NameTuple, _ int) string {
			return table.ForOutput()
		}))
	}

	// 3. filtering unsupported columns
	tablesColumnList, unsupportedTableColumnsMap, err := source.DB().GetColumnsWithSupportedTypes(finalTableList, useDebezium, changeStreamingIsEnabled(exportType))
	if err != nil {
		utils.ErrExit("get columns with supported types: %w", err)
	}
	// If any of the keys of unsupportedTableColumnsMap contains values in the string array then do this check
	if len(unsupportedTableColumnsMap.Keys()) > 0 {
		handleUnsupportedColumnsInExportData(unsupportedTableColumnsMap)
		finalTableList = filterTableWithEmptySupportedColumnList(finalTableList, tablesColumnList)
	}

	log.Infof("final table list after filtering empty tables and unsupported tables/columns - %v", lo.Map(finalTableList, func(t sqlname.NameTuple, _ int) string {
		return t.ForOutput()
	}))
	return finalTableList, tablesColumnList
}

func reportUnsupportedTablesForLiveMigration(finalTableList []sqlname.NameTuple) {
	if !changeStreamingIsEnabled(exportType) {
		return
	}

	//report non-pk tables
	allNonPKTables, err := source.DB().GetNonPKTables()
	if err != nil {
		utils.ErrExit("get non-pk tables: %w", err)
	}
	var nonPKTables []string
	for _, table := range finalTableList {
		if lo.Contains(allNonPKTables, table.ForKey()) {
			nonPKTables = append(nonPKTables, table.ForOutput())
		}
	}
	if len(nonPKTables) > 0 {
		utils.PrintAndLogf("Table names without a Primary key: %s", nonPKTables)
		utils.ErrExit("Currently voyager does not support live-migration for tables without a primary key.\n" +
			"You can exclude these tables using the --exclude-table-list argument.")
	}
}

func handleUnsupportedColumnsInExportData(unsupportedTableColumnsMap *utils.StructMap[sqlname.NameTuple, []string]) {
	log.Infof("preparing column list for the data export without unsupported datatype columns: %v", unsupportedTableColumnsMap)

	var unsupportedColsMsg strings.Builder
	unsupportedColsMsg.WriteString("The following columns data export is unsupported:\n")
	unsupportedTableColumnsMap.IterKV(func(k sqlname.NameTuple, v []string) (bool, error) {
		if len(v) != 0 {
			unsupportedColsMsg.WriteString(fmt.Sprintf("%s: %s\n", k.ForOutput(), v))
		}
		return true, nil
	})
	utils.PrintAndLog(unsupportedColsMsg.String())
	if !utils.AskPrompt("\nDo you want to continue with the export by ignoring just these columns' data") {
		utils.ErrExit("Exiting at user's request. Use `--exclude-table-list` flag to continue without these tables")
	} else {
		utils.PrintAndLog(color.YellowString("Continuing with the export by ignoring just these columns' data.\n"+
			"Please make sure to remove any null constraints on these columns in the %s database.", getImportingDatabaseString()))
	}
}

func getImportingDatabaseString() string {
	if exporterRole == SOURCE_DB_EXPORTER_ROLE {
		return "target"
	} else if exporterRole == TARGET_DB_EXPORTER_FF_ROLE {
		return "source-replica"
	} else if exporterRole == TARGET_DB_EXPORTER_FB_ROLE {
		return "source"
	}
	return "UNKNOWN"
}

func handleEmptyTableListForExport(finalTableList []sqlname.NameTuple) {
	if len(finalTableList) != 0 {
		return
	}

	utils.PrintAndLogf("no tables present to export, exiting...")
	setDataIsExported()
	dfd := datafile.Descriptor{
		ExportDir:    exportDir,
		DataFileList: make([]*datafile.FileEntry, 0),
	}
	dfd.Save()
	os.Exit(0)
}

// ================================ MSR or YugabyteD specific functions ================================

func dataIsExported() bool {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record for checking ExportDataDone: %w", err)
	}

	return msr.ExportDataDone
}

func setDataIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportDataDone = true
	})
	if err != nil {
		utils.ErrExit("failed to set data is exported in migration status record: %w", err)
	}
}

func clearDataIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportDataDone = false
	})
	if err != nil {
		utils.ErrExit("failed to clear export data done flag in migration status record: %w", err)
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
			schema := ""
			if len(source.Schemas) > 0 {
				schema = source.Schemas[0].Unquoted
			}
			schemaName, tableName2 = schema, tableName
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

func saveTableToUniqueKeyColumnsMapInMetaDB(tableList []sqlname.NameTuple, leafPartitions *utils.StructMap[sqlname.NameTuple, []sqlname.NameTuple]) {
	res, err := source.DB().GetTableToUniqueKeyColumnsMap(tableList)
	if err != nil {
		utils.ErrExit("get table to unique key columns map: %w", err)
	}

	if res == nil {
		log.Infof("no table to unique key columns map found, saving nil to metaDB")
		key := fmt.Sprintf("%s_%s", metadb.TABLE_TO_UNIQUE_KEY_COLUMNS_KEY, exporterRole)
		err = metadb.UpdateJsonObjectInMetaDB(metaDB, key, func(record *map[string][]string) {
			*record = nil
		})
		if err != nil {
			utils.ErrExit("insert table to unique key columns map: %w", err)
		}
		return
	}

	//Adding all the leaf partitions unique key columns to the root table unique key columns since in the importer all the events only have the root table name
	leafPartitions.IterKV(func(rootTable sqlname.NameTuple, value []sqlname.NameTuple) (bool, error) {
		for _, leafTable := range value {
			leafUniqueColumns, ok := res.Get(leafTable)
			if !ok {
				continue
			}
			//Do not add leaf table key in the map since this config will be read by importer
			res.Delete(leafTable)
			rootUniqueColumns, ok := res.Get(rootTable)
			if !ok {
				rootUniqueColumns = []string{}
			}
			rootUniqueColumns = append(rootUniqueColumns, leafUniqueColumns...)
			res.Put(rootTable, lo.Uniq(rootUniqueColumns))
		}
		return true, nil
	})

	metaDbData := make(map[string][]string)
	res.IterKV(func(k sqlname.NameTuple, v []string) (bool, error) {
		metaDbData[k.AsQualifiedCatalogName()] = v
		return true, nil
	})

	log.Infof("updating metaDB with table to unique key columns map: %v", res)
	key := fmt.Sprintf("%s_%s", metadb.TABLE_TO_UNIQUE_KEY_COLUMNS_KEY, exporterRole)
	err = metadb.UpdateJsonObjectInMetaDB(metaDB, key, func(record *map[string][]string) {
		*record = metaDbData
	})
	if err != nil {
		utils.ErrExit("insert table to unique key columns map: %w", err)
	}
}
