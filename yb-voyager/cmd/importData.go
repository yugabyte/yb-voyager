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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	reporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var metaInfoDirName = META_INFO_DIR_NAME
var batchSizeInNumRows = int64(0)
var importerRole string
var identityColumnsMetaDBKey string

// importPhase is the phase reported before the import engine exists (see
// currentImportPhase); after that the engine owns the phase.
var importPhase string

// stores the data files description in a struct
var dataFileDescriptor *datafile.Descriptor
var truncateSplits utils.BoolStr // to truncate *.D splits after import
var targetDBDetails *callhome.TargetDBDetails
var skipReplicationChecks utils.BoolStr
var skipNodeHealthChecks utils.BoolStr
var skipDiskUsageHealthChecks utils.BoolStr
var callhomeMetricsCollector *callhome.ImportDataMetricsCollector

// ShutdownImportProgressBars stops the mpb progress container so that its
// rendering goroutine no longer writes to stdout. Must be called before
// printing any final messages on signal receipt to avoid the bars overwriting them.
func ShutdownImportProgressBars() {
	if importer != nil {
		importer.ShutdownProgressBars()
	}
}

// importer is the single handle cmd keeps on the running import engine
// (src/importdata). The callhome payload builders read its phase and streaming
// stats through currentImportPhase / currentStatsReporter.
var importer *importdata.Importer

// currentImportPhase returns the engine's phase once the engine exists, and the
// command-level importPhase before that (set to snapshot at the start of the
// import data command, empty during PreRun), exactly as the former global did.
func currentImportPhase() string {
	if importer == nil {
		return importPhase
	}
	return importer.ImportPhase()
}

func currentStatsReporter() *reporter.StreamImportStatsReporter {
	if importer == nil {
		return nil
	}
	return importer.StatsReporter()
}

var importTableList []sqlname.NameTuple

// Error policy
var errorPolicySnapshotFlag importdata.ErrorPolicy = importdata.AbortErrorPolicy

// snapshot batch production
var enableRandomBatchProduction utils.BoolStr
var maxConcurrentBatchProductionsConfig int = 10

// live migration
var cdcPartitionKey string
var cdcPartitionKeyOverrides string

var importDataCmd = &cobra.Command{
	Use: "data",
	Short: "Import data from compatible source database to target database.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/data-migration/import-data/",
	Long: `Import the data exported from the source database into the target database. Also import data(snapshot + changes from target) into source-replica/source in case of live migration with fall-back/fall-forward worflows.`,
	Args: cobra.NoArgs,
	PreRun: func(cmd *cobra.Command, args []string) {
		if tconf.TargetDBType == "" {
			tconf.TargetDBType = YUGABYTEDB
		}
		if importerRole == "" {
			importerRole = TARGET_DB_IMPORTER_ROLE
		}
		validateTargetDBTypeFlag()

		// Adaptive-parallelism default is per-target (Balanced for YugabyteDB,
		// Disabled for yugabytedb-amp, which has no YB cluster control API so
		// adaptive parallelism cannot work there). The explicit-flag guardrails
		// for amp live in validateParallelismFlags (invoked via validateImportFlags).
		if !cmd.Flags().Changed("adaptive-parallelism") {
			tconf.AdaptiveParallelismMode = defaultAdaptiveParallelismMode(tconf.TargetDBType)
		}

		err := retrieveMigrationUUID()
		if err != nil {
			utils.ErrExit("failed to get migration UUID: %w", err)
		}
		sourceDBType = GetSourceDBTypeFromMSR()
		// validateImportFlags runs the parallelism conflict check (validateParallelismFlags)
		// and the amp source-compat check; the adaptive-parallelism default above must be
		// resolved before this point.
		err = validateImportFlags(cmd, importerRole)
		if err != nil {
			utils.ErrExit("Error validating import flags: %s", err.Error())
		}

		err = validateImportDataFlags()
		if err != nil {
			utils.ErrExit("Error validating import data flags: %s", err.Error())
		}

		// Reject the import-data flags that are not applicable for a yugabytedb-amp target.
		// Run after validateImportDataFlags so --on-primary-key-conflict has been validated
		// (the generic validity check) before we report it as not applicable for amp.
		validateAmpUnsupportedFlags(cmd)

		err = validateImportUsePartitionRootFlag()
		if err != nil {
			utils.ErrExit("Error validating --use-partition-root flag: %s", err.Error())
		}
	},
	Run: importDataCommandFn,
}

var importDataToCmd = &cobra.Command{
	Use:   "to",
	Short: "Import data into various databases",
	Long:  `Import data into various databases`,
}

var importDataToTargetCmd = &cobra.Command{
	Use:   "target",
	Short: importDataCmd.Short,
	Long:  importDataCmd.Long,
	Args:  importDataCmd.Args,

	PreRun: importDataCmd.PreRun,

	Run: importDataCmd.Run,
}

func handleCutoverAlreadyProcessedForImportData() {
	//If cutover is already processed by this command and the cutover for the respective flow is completed then exit
	cutoverAlreadyProcessed := isCutoverAlreadyProcessed(importerRole)
	if !cutoverAlreadyProcessed {
		return
	}
	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		if GetCutoverStatus(metaDB) == COMPLETED {
			utils.ErrExit("cutover to target already processed, exiting...")
		}
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		if getCutoverToSourceReplicaStatus(metaDB) == COMPLETED {
			utils.ErrExit("cutover to source-replica already processed, exiting...")
		}
	case SOURCE_DB_IMPORTER_ROLE:
		if GetCutoverToSourceStatus(exportDir, metaDB) == COMPLETED {
			utils.ErrExit("cutover to source already processed, exiting...")
		}
	}
	//If cutover is not completed then start further commands after current import data
	startFurtherCommandsAfterCurrentImportData()
}

func importDataCommandFn(cmd *cobra.Command, args []string) {
	importPhase = dbzm.MODE_SNAPSHOT

	reportProgressInBytes = false
	tconf.ImportMode = true

	if err := setupImportDataObservability(); err != nil {
		utils.ErrExit("Failed to setup import data observability: %w", err)
	}

	err := setImportTypeAndIdentityColumnMetaDBKeyForImporterRole(importerRole)
	if err != nil {
		utils.ErrExit("error while setting import type or identity column metadb key: %w", err)
	}
	checkExportDataDoneOrStartedFlag()

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("failed to get migration status record: %w", err)
	}
	if importerRole == TARGET_DB_IMPORTER_ROLE && !msr.IsParentMigration() {
		//It this is not the parent migration, then use the target db conf stored in the migration
		password := tconf.Password
		tconf = *msr.TargetDBConf
		tconf.Password = password
	}
	/*
		Before this point MSR won't not be initialised in case of importDataFileCmd
		In case of importDataCmd, MSR would be initialised already by previous commands
	*/
	saveOnPrimaryKeyConflictActionInMSR()

	sourceDBType = GetSourceDBTypeFromMSR()
	sqlname.SourceDBType = sourceDBType

	//Schema validation is done in the Init step as of now
	// TODO: will handle it later with other task of completely supporting case sensitive schemas in target db schema for MysqL/oracle sources
	//TODO: also for the source-replica ORACLE case to validate the schemas on source-replica
	tconf.Schemas = sqlname.ParseIdentifiersFromString(tconf.TargetDBType, tconf.SchemaConfig, ",")
	tdb = tgtdb.NewTargetDB(&tconf)
	err = tdb.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB: %w", err)
	}
	// Check if target DB has the required permissions
	if tconf.RunGuardrailsChecks {
		checkImportDataPermissions()
	}

	targetDBDetails = tdb.GetCallhomeTargetDBInfo()

	// we don't want to re-register in case import data to source/source-replica
	reregisterYBNames := shouldReregisterYBNames()
	err = InitNameRegistry(exportDir, importerRole, nil, nil, &tconf, tdb, reregisterYBNames)
	if err != nil {
		utils.ErrExit("initialize name registry: %w", err)
	}

	var importFileTasks []*importdata.ImportFileTask
	if importdata.ImportSnapshotRequired(importerRole, importType) {

		dataStore = datastore.NewDataStore(filepath.Join(exportDir, "data"))
		dataFileDescriptor = datafile.OpenDescriptor(exportDir)
		log.Infof("Parsed DataFileDescriptor: %v", spew.Sdump(dataFileDescriptor))
		// TODO: handle case-sensitive in table names with oracle ff-db
		// quoteTableNameIfRequired()
		importFileTasks = discoverFilesToImport()
		log.Debugf("Discovered import file tasks: %v", importFileTasks)
	}

	err = validateCdcPartitionKeyFlags(cmd)
	if err != nil {
		utils.ErrExit("error validating cdc partition key flags: %w", err)
	}

	msr, err = metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch MigrationStatusRecord: %w", err)
	}

	//Starting table list
	importFileTasks, importTableList, err = initialiseImportTableList(importFileTasks, msr)
	if err != nil {
		utils.ErrExit("Failed to initialize import table list: %w", err)
	}

	if importerRole == TARGET_DB_IMPORTER_ROLE && tconf.EnableUpsert {
		if !utils.AskPrompt(color.RedString("WARNING: Ensure that tables on target YugabyteDB do not have secondary indexes. " +
			"If a table has secondary indexes, setting --enable-upsert to true may lead to corruption of the indexes. Are you sure you want to proceed?")) {
			utils.ErrExit("Aborting import.")
		}
	}

	handleCutoverAlreadyProcessedForImportData()

	runImportDataEngine(importFileTasks)
	tdb.Finalize()

	if furtherCommandsRequired() {
		startFurtherCommandsAfterCurrentImportData()
	} else {
		switch importerRole {
		case TARGET_DB_IMPORTER_ROLE:
			sendImportDataPayloadToCallhomeAndControlPlane()
		case SOURCE_REPLICA_DB_IMPORTER_ROLE:
			packAndSendImportDataToSrcReplicaPayload(COMPLETE, nil)
		case SOURCE_DB_IMPORTER_ROLE:
			packAndSendImportDataToSourcePayload(COMPLETE, nil)
		}
	}

}

func sendImportDataPayloadToCallhomeAndControlPlane() {
	//send callhome / control plane payload before starting export data from target
	importDataCompletedEvent := createSnapshotImportCompletedEvent()
	controlPlane.SnapshotImportCompleted(&importDataCompletedEvent)
	packAndSendImportDataToTargetPayload(COMPLETE, nil)
}

func furtherCommandsRequired() bool {
	return isFallbackEnabledOrFallForwardEnabled() || isNextIterationRequired()
}
func startFurtherCommandsAfterCurrentImportData() {
	//Fallback export data from target commands
	startExportDataFromTargetIfRequired()

	//Start next iterations's export data from source
	startExportDataFromSourceOnNextIteration()
}

func isFallbackEnabledOrFallForwardEnabled() bool {
	if !changeStreamingIsEnabled(importType) {
		return false
	}
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		return false
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch MigrationStatusRecord: %w", err)
	}
	if !msr.FallForwardEnabled && !msr.FallbackEnabled {
		utils.PrintAndLogf("No fall-forward/back enabled. Exiting.")
		return false
	}
	return true
}
func isNextIterationRequired() bool {
	if importerRole != SOURCE_DB_IMPORTER_ROLE {
		return false
	}
	currentMsr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("failed to get migration status record: %w", err)
	}
	return currentMsr.RestartDataMigrationSourceTargetNextIteration

}
func startExportDataFromSourceOnNextIteration() {
	if !isNextIterationRequired() {
		return
	}

	injectBeforeInitializeNextIteration()

	err := initializeNextIteration()
	if err != nil {
		utils.ErrExit("failed to initialize next iteration: %w", err)
	}

	injectAfterInitializeNextIteration()

	currentMsr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("failed to get migration status record: %w", err)
	}

	//Start export from source on next iteration

	lockFile.Unlock() // unlock export dir from import data cmd before switching current process to ff/fb sync cmd
	cmd := []string{"yb-voyager", "export", "data", "from", "source"}
	if cfgFile != "" {
		//If there are cli overrides for the command, pass them as cli overrides to the import data to source command
		//Only disable-pb and log-level are the common flags of both the commands
		for _, override := range resolvedConfig.fromCLI {
			if override.FlagName == "disable-pb" {
				//only for disable-pb flag common export/import flag, if it is overidden then pass it as CLI override also to this command
				cmd = append(cmd, "--"+override.FlagName, override.Value)
				continue
			}
			if override.FlagName == "export-dir" {
				//Do not do anything for export-dir as it should always be the current iteration export dir
				continue
			}
			if override.FlagName == "config-file" {
				//For config file, always pass the current config file
				cmd = append(cmd, "--"+override.FlagName, cfgFile)
				continue
			}
			if !slices.Contains(globalFlags, override.FlagName) {
				//if its not a global flag then skip passing it to the command as it will be command specific flag
				continue
			}
			cmd = append(cmd, "--"+override.FlagName, override.Value)
		}

	} else {
		cmd = append(cmd, "--export-dir", lo.Ternary(currentMsr.IsParentMigration(), exportDir, currentMsr.ParentExportDir))
		if bool(disablePb) {
			cmd = append(cmd, "--disable-pb=true")
		}
		cmd = append(cmd, fmt.Sprintf("--send-diagnostics=%t", callhome.SendDiagnostics))
		cmd = append(cmd, "--log-level", config.LogLevel)
		cmd = append(cmd, "--export-type", CHANGES_ONLY)
		//TODO: see if we can do better, but these params are required for import data to target cmd
		cmd = append(cmd, "--source-db-type", currentMsr.SourceDBConf.DBType)
		cmd = append(cmd, "--source-db-name", currentMsr.SourceDBConf.DBName)
		cmd = append(cmd, "--source-db-user", currentMsr.SourceDBConf.User)
		cmd = append(cmd, "--source-db-schema", currentMsr.SourceDBConf.SchemaConfig)
	}

	//TODO: somehow figure out that whether table list is overidden by CLI or not and then only pass it
	if currentMsr.SourceDBConf.TableList != "" {
		//If these are overridden by CLI/Config file then pass it to the command always
		cmd = append(cmd, "--table-list", currentMsr.SourceDBConf.TableList)
	}
	if currentMsr.SourceDBConf.ExcludeTableList != "" {
		//If these are overridden by CLI/Config file then pass it to the command always
		cmd = append(cmd, "--exclude-table-list", currentMsr.SourceDBConf.ExcludeTableList)
	}

	iterationExportDir := GetIterationExportDir(currentMsr.GetIterationsDir(exportDir), currentMsr.IterationNo+1)
	utils.PrintAndLogfPhase("\nStarting export data from source on iteration %d at %s.", currentMsr.IterationNo+1, iterationExportDir)
	fmt.Println()

	cmdStr := "SOURCE_DB_PASSWORD=*** " + strings.Join(cmd, " ")

	utils.PrintAndLogf("Starting export data from source with command:\n %s", color.GreenString(cmdStr))

	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager: %w", lookErr)
	}
	env := os.Environ()
	env = slices.Insert(env, 0, "SOURCE_DB_PASSWORD="+tconf.Password)

	packAndSendImportDataToSourcePayload(COMPLETE, nil)

	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager export data from source: %w\n Please re-run with command :\n%s", execErr, cmdStr)
	}

}

func checkTablesPresentInTarget(tablesToImport []sqlname.NameTuple) {
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		return
	}
	tablesNotPresentInTarget := []sqlname.NameTuple{}
	for _, tableName := range tablesToImport {
		if !tableName.TargetTableAvailable() {
			tablesNotPresentInTarget = append(tablesNotPresentInTarget, tableName)
		}
	}
	if len(tablesNotPresentInTarget) > 0 {
		utils.PrintAndLogfInfo("\nFollowing source tables are not present in the target database:\n%v", strings.Join(lo.Map(tablesNotPresentInTarget, func(t sqlname.NameTuple, _ int) string {
			return t.ForKey()
		}), ", "))
		utils.ErrExit("Create these tables in the target database to continue with the import.")
	}
}

// checkPartitionConsistency verifies that partitions are the same between source and target
// when '--use-partition-root false' is used during import. This is required because CDC events
// will contain partition table names that must exist on the target.
func checkPartitionConsistency(msr *metadb.MigrationStatusRecord, importTableList []sqlname.NameTuple) {
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		//TODO to have similar consistency check in source also later
		return
	}
	if msr.SourceRenameTablesMap == nil {
		// No partitions to check
		return
	}

	log.Infof("Checking partition consistency between source and target ('--use-partition-root false')")

	// Get list of partitions from MSR (source partitions)
	rootToLeafPartitions := utils.NewStructMap[sqlname.NameTuple, []string]()
	for leaf, root := range msr.SourceRenameTablesMap {
		rootTup, err := namereg.NameReg.LookupTableName(root)
		if err != nil {
			utils.ErrExit("failed to lookup root table %s: %w", root, err)
		}
		leaves, ok := rootToLeafPartitions.Get(rootTup)
		if !ok {
			leaves = []string{}
		}
		leaves = append(leaves, leaf)
		rootToLeafPartitions.Put(rootTup, leaves)
	}

	checkIfTableExistsOnTarget := func(table string) bool {
		// Try to lookup the partition in name registry
		tableTup, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(table)
		if err != nil {
			log.Warnf("Partition %s from source not found in name registry: %v", table, err)
			return false
		}
		return tableTup.TargetTableAvailable()
	}
	// Check each source partition exists on target
	missingRootToLeafPartitions := utils.NewStructMap[sqlname.NameTuple, []string]()
	// the callback never returns an error
	_ = rootToLeafPartitions.IterKV(func(root sqlname.NameTuple, leaves []string) (bool, error) {
		if !lo.ContainsBy(importTableList, func(t sqlname.NameTuple) bool {
			return t.Equals(root)
		}) {
			//if the root table is not in the import table list, then skip the check
			//since its not being exported from source and this is really possible as we don't allow changing table-list in the middle of the migration
			log.Infof("Root table %s is not in the import table list, skipping check", root)
			return true, nil
		}
		for _, leaf := range leaves {
			if !checkIfTableExistsOnTarget(leaf) {
				leaves, ok := missingRootToLeafPartitions.Get(root)
				if !ok {
					leaves = []string{}
				}
				leaves = append(leaves, leaf)
				missingRootToLeafPartitions.Put(root, leaves)
			}
		}
		return true, nil
	})

	if len(missingRootToLeafPartitions.Keys()) > 0 {
		utils.PrintAndLogfInfo("\nWhen using '--use-partition-root false', CDC events will contain partition table names.")
		utils.PrintAndLogfInfo("The following root table partitions are not present on the target database:")
		printMissingRootToLeafPartitions(missingRootToLeafPartitions)
		utils.PrintAndLogfWarning("\nEnsure that all partitions from the source exist on the target, or use --use-partition-root true (default).")
		if !utils.AskPrompt("\nDo you want to continue anyway") {
			//ideally we should just exit but for now since this is a new feature, we will just give a prompt in case if we miss something
			utils.ErrExit("Aborting.")
		}
	}
	log.Infof("Partition consistency check passed: %v root-to-leaf partitions verified", rootToLeafPartitions)
}

func printMissingRootToLeafPartitions(missingRootToLeafPartitions *utils.StructMap[sqlname.NameTuple, []string]) {
	sortFn := func(a, b sqlname.NameTuple) bool { return a.AsQualifiedCatalogName() < b.AsQualifiedCatalogName() }
	// display-only iteration; the callback never returns an error
	_ = missingRootToLeafPartitions.IterKVSorted(sortFn, func(root sqlname.NameTuple, leaves []string) (bool, error) {
		utils.PrintAndLogfInfo("  - %s:", root.ForOutput())
		sort.Slice(leaves, func(i, j int) bool { return leaves[i] < leaves[j] })
		utils.PrintAndLogfInfo("    - %s", strings.Join(leaves, ", "))
		return true, nil
	})
}
func shouldReregisterYBNames() bool {
	actualDataImportStarted := false
	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		statusRecord, err := metaDB.GetImportDataStatusRecord()
		if err != nil {
			utils.ErrExit("failed to get import data status record: %w", err)
		}
		actualDataImportStarted = statusRecord.ImportDataStarted
	case IMPORT_FILE_ROLE:
		statusRecord, err := metaDB.GetImportDataFileStatusRecord()
		if err != nil {
			utils.ErrExit("failed to get import data status record: %w", err)
		}
		actualDataImportStarted = statusRecord.ImportDataStarted
	default:
		//for other importers we shouldn't re-register as this is for YB names and other importers are source-replica / source
		return false

	}
	return (bool(startClean) || !actualDataImportStarted)
}

func setImportTypeAndIdentityColumnMetaDBKeyForImporterRole(importerRole string) error {

	record, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return goerrors.Errorf("Failed to get migration status record: %w", err)
	}

	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		importType = record.ExportTypeFromSource
		identityColumnsMetaDBKey = metadb.TARGET_DB_IDENTITY_COLUMNS_KEY
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		if record.FallbackEnabled {
			return goerrors.Errorf("cannot import data to source-replica. Fall-back workflow is already enabled.")
		}
		if record.ExportTypeFromSource == CHANGES_ONLY {
			return goerrors.Errorf("cannot import data to source-replica. Export type is changes-only.")
		}
		updateFallForwardEnabledInMetaDB()
		identityColumnsMetaDBKey = metadb.FF_DB_IDENTITY_COLUMNS_KEY
	case SOURCE_DB_IMPORTER_ROLE:
		identityColumnsMetaDBKey = metadb.SOURCE_DB_IDENTITY_COLUMNS_KEY

	}
	return nil
}

func checkImportDataPermissions() {
	// If import to source on PG, check if triggers and FKs are disabled
	fkAndTriggersCheckFailed := false
	if importerRole == SOURCE_DB_IMPORTER_ROLE {
		enabledTriggers, enabledFks, err := tdb.GetEnabledTriggersAndFks()
		if err != nil {
			utils.ErrExit("Failed to check if triggers and FKs are enabled: %w", err)
		}
		if len(enabledTriggers) > 0 || len(enabledFks) > 0 {
			if len(enabledTriggers) > 0 {
				utils.PrintAndLogf("%s [%s]", color.RedString("\nEnabled Triggers:"), strings.Join(enabledTriggers, ", "))
			}
			if len(enabledFks) > 0 {
				utils.PrintAndLogf("%s [%s]", color.RedString("\nEnabled Foreign Keys:"), strings.Join(enabledFks, ", "))
			}
			fmt.Printf("\n%s", color.RedString("Disable the above triggers and FKs before importing data.\n"))
			fkAndTriggersCheckFailed = true
			fmt.Println("\nCheck the documentation to disable triggers and FKs:", color.BlueString("https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-back/#cutover-to-the-target"))
		}
	}

	missingPermissions, err := tdb.GetMissingImportDataPermissions(importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE)
	if err != nil {
		utils.ErrExit("Failed to get missing import data permissions: %w", err)
	}
	if len(missingPermissions) > 0 {
		// Not printing the target db is missing permissions message for YB
		// In YB we only check whether he user is a superuser and hence print only in the case where target db is not YB
		// In case of fall forward too we only run superuser checks and hence print only in the case where fallback is enabled
		if tconf.TargetDBType != YUGABYTEDB && !(importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE) {
			utils.PrintAndLog(color.RedString("\nPermissions and configurations missing in the target database for importing data:"))
		}
		output := strings.Join(missingPermissions, "\n")
		utils.PrintAndLog(output)

		var link string
		switch importerRole {
		case SOURCE_REPLICA_DB_IMPORTER_ROLE:
			link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-forward/#prepare-source-replica-database"
		case SOURCE_DB_IMPORTER_ROLE:
			link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-back/#prepare-the-source-database"
		default:
			if changeStreamingIsEnabled(importType) {
				link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-migrate/#prepare-the-target-database"
			} else {
				link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-target-database"
			}
		}
		fmt.Println("\nCheck the documentation to prepare the database for migration:", color.BlueString(link))

		// Prompt user to continue if missing permissions only if fk and triggers check did not fail
		if fkAndTriggersCheckFailed {
			utils.ErrExit("Please grant the required permissions and retry the import.")
		} else if !utils.AskPrompt("\nDo you want to continue anyway") {
			utils.ErrExit("Please grant the required permissions and retry the import.")
		}
	} else {
		// If only fk and triggers check failed just simply error out
		if fkAndTriggersCheckFailed {
			utils.ErrExit("")
		} else {
			log.Info("The target database has the required permissions for importing data.")
		}
	}
}

func startExportDataFromTargetIfRequired() {
	if !isFallbackEnabledOrFallForwardEnabled() {
		return
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch MigrationStatusRecord: %w", err)
	}

	lockFile.Unlock() // unlock export dir from import data cmd before switching current process to ff/fb sync cmd

	cmd := generateExportDataFromTargetCommand(msr)

	cmdStr := "TARGET_DB_PASSWORD=*** " + strings.Join(cmd, " ")

	msg := "Starting fallback flow from target to source"
	if msr.IterationNo > 0 {
		msg += fmt.Sprintf(" on iteration %d", msr.IterationNo)
	}
	utils.PrintfInfo("\n%s\n", msg)

	utils.PrintAndLogf("Starting export data from target with command:\n %s", color.GreenString(cmdStr))

	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager: %w", lookErr)
	}
	env := os.Environ()
	env = slices.Insert(env, 0, "TARGET_DB_PASSWORD="+tconf.Password)

	sendImportDataPayloadToCallhomeAndControlPlane()

	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager export data from target: %w\n Please re-run with command :\n%s", execErr, cmdStr)
	}
}

func generateExportDataFromTargetCommand(msr *metadb.MigrationStatusRecord) []string {
	cmd := []string{"yb-voyager", "export", "data", "from", "target"}

	arguments := generateGlobalExportImportArguments()
	cmd = append(cmd, arguments...)

	if msr.UseYBgRPCConnector {
		//if using gRPC connector, set some overrides for the command like target ssl related
		if tconf.SSLMode == "prefer" || tconf.SSLMode == "allow" {
			utils.PrintAndLog(color.RedString("Warning: SSL mode '%s' is not supported for 'export data from target' yet. Downgrading it to 'disable'.\nIf you don't want these settings you can restart the 'export data from target' with a different value for --target-ssl-mode and --target-ssl-root-cert flag.", source.SSLMode))
			tconf.SSLMode = "disable"
		}
		cmd = append(cmd, "--target-ssl-mode", tconf.SSLMode)
		if tconf.SSLRootCert != "" {
			cmd = append(cmd, "--target-ssl-root-cert", tconf.SSLRootCert)
		}
	}
	if utils.DoNotPrompt {
		cmd = append(cmd, "--yes")
	}
	return cmd
}

// func quoteTableNameIfRequired() {
// 	if tconf.TargetDBType != ORACLE {
// 		return
// 	}
// 	for _, fileEntry := range dataFileDescriptor.DataFileList {
// 		if sqlname.IsQuoted(fileEntry.TableName) {
// 			continue
// 		}
// 		if sqlname.IsReservedKeywordOracle(fileEntry.TableName) ||
// 			(sqlname.IsCaseSensitive(fileEntry.TableName, ORACLE)) {
// 			newTableName := fmt.Sprintf(`"%s"`, fileEntry.TableName)
// 			if dataFileDescriptor.TableNameToExportedColumns != nil {
// 				dataFileDescriptor.TableNameToExportedColumns[newTableName] = dataFileDescriptor.TableNameToExportedColumns[fileEntry.TableName]
// 				delete(dataFileDescriptor.TableNameToExportedColumns, fileEntry.TableName)
// 			}
// 			fileEntry.TableName = newTableName
// 		}
// 	}
// }

func discoverFilesToImport() []*importdata.ImportFileTask {
	result := []*importdata.ImportFileTask{}
	if dataFileDescriptor.DataFileList == nil {
		utils.ErrExit("It looks like the data is exported using older version of Voyager. Please use matching version to import the data.")
	}

	for i, fileEntry := range dataFileDescriptor.DataFileList {
		if fileEntry.RowCount == 0 {
			// In case of PG Live migration  pg_dump and dbzm both are used and we don't skip empty tables
			// but pb hangs for empty so skipping empty tables in snapshot import
			continue
		}

		//using the LookupTableNameAndIgnoreIfTargetNotFound if there are tables in descriptor which are not present in the target
		//for such tables we will not get target table hence we will ask users to exclude them in table-list flags
		tableName, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(fileEntry.TableName)
		if err != nil {
			utils.ErrExit("lookup table name from name registry: %w", err)
		}
		task := &importdata.ImportFileTask{
			ID:           i,
			FilePath:     fileEntry.FilePath,
			TableNameTup: tableName,
			RowCount:     fileEntry.RowCount,
			FileSize:     fileEntry.FileSize,
		}
		result = append(result, task)
	}
	return result
}

func applyTableListFilter(importFileTasks []*importdata.ImportFileTask) []*importdata.ImportFileTask {
	result := []*importdata.ImportFileTask{}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch migration status record: %w", err)
	}
	source = *msr.SourceDBConf
	_, noDefaultSchema := getDefaultSourceSchemaName()

	allTables := lo.Uniq(lo.Map(importFileTasks, func(task *importdata.ImportFileTask, _ int) sqlname.NameTuple {
		return task.TableNameTup
	}))
	slices.SortFunc(allTables, func(a, b sqlname.NameTuple) int {
		if a.ForKey() < b.ForKey() {
			return -1 // a is less than b
		} else if a.ForKey() > b.ForKey() {
			return 1 // a is greater than b
		}
		return 0 // a is equal to b
	})
	log.Infof("allTables: %v", allTables)

	findPatternMatchingTables := func(pattern string) []sqlname.NameTuple {
		result := lo.Filter(allTables, func(tableNameTup sqlname.NameTuple, _ int) bool {
			matched, err := tableNameTup.MatchesPattern(pattern)
			if err != nil {
				utils.ErrExit("Invalid table name pattern: %q: %w", pattern, err)
			}
			return matched
		})
		return result
	}

	extractTableList := func(flagTableList, listName string) ([]sqlname.NameTuple, []string) {
		tableList := utils.CsvStringToSlice(flagTableList)
		var result []sqlname.NameTuple
		var unqualifiedTables []string
		var unknownTables []string
		for _, table := range tableList {
			if noDefaultSchema && len(strings.Split(table, ".")) == 1 {
				unqualifiedTables = append(unqualifiedTables, table)
				continue
			}

			matchingTables := findPatternMatchingTables(table)
			if len(matchingTables) == 0 {
				unknownTables = append(unknownTables, table) //so that unknown check can be done later
			} else {
				result = append(result, matchingTables...)
			}
		}
		if len(unqualifiedTables) > 0 {
			utils.ErrExit("Qualify following table names in the %s list with schema-name: %v", listName, unqualifiedTables)
		}
		log.Infof("%s tableList: %v", listName, result)
		return result, unknownTables
	}

	includeList, unknownInclude := extractTableList(tconf.TableList, "include")
	excludeList, unknownExclude := extractTableList(tconf.ExcludeTableList, "exclude")
	allUnknown := append(unknownInclude, unknownExclude...)
	if len(allUnknown) > 0 {
		utils.PrintAndLogf("Unknown table names in the table-list: %v", allUnknown)
		tablesPresentInTarget := lo.Filter(allTables, func(t sqlname.NameTuple, _ int) bool {
			return t.TargetTableAvailable()
		})
		utils.PrintAndLogf("Valid table names are: %v", lo.Map(tablesPresentInTarget, func(t sqlname.NameTuple, _ int) string {
			//For the tables that are present in target, we will display the current table name (i.e. as per target table name) properly
			return t.ForOutput()
		}))
		utils.ErrExit("Please fix the table names in table-list and retry.")
	}

	tablesNotPresentInTarget := []sqlname.NameTuple{}

	for _, task := range importFileTasks {
		if len(includeList) > 0 && !slices.Contains(includeList, task.TableNameTup) {
			log.Infof("Skipping table %q (fileName: %s) as it is not in the include list", task.TableNameTup, task.FilePath)
			continue
		}
		if len(excludeList) > 0 && slices.Contains(excludeList, task.TableNameTup) {
			log.Infof("Skipping table %q (fileName: %s) as it is in the exclude list", task.TableNameTup, task.FilePath)
			continue
		}
		if !task.TableNameTup.TargetTableAvailable() && !changeStreamingIsEnabled(importType) {
			//If table not exclude and not present in target then we will ask users to exclude them in table-list flags
			//for offline migration only as for live we don't support table-list flags so this doesn't matter for that
			//and we can assume that all export side tables should be present in target
			tablesNotPresentInTarget = append(tablesNotPresentInTarget, task.TableNameTup)
			continue
		}
		result = append(result, task)
	}
	if len(tablesNotPresentInTarget) > 0 {
		utils.PrintAndLogfInfo("\nFollowing source tables are not present in the target database:\n%v", strings.Join(lo.Map(tablesNotPresentInTarget, func(t sqlname.NameTuple, _ int) string {
			return t.ForKey()
		}), ","))
		utils.ErrExit("Create these tables in the target database or exclude the tables in table-list flags if you don't want to import them.")
	}
	return result
}

func setupImportDataObservability() error {
	if err := startMetricsServer(importerRole, migrationUUID); err != nil {
		return goerrors.Errorf("Failed to start metrics server: %w", err)
	}
	if callhome.SendDiagnostics {
		callhomeMetricsCollector = callhome.NewImportDataMetricsCollector()
	}
	return nil
}

func initialiseImportTableList(importFileTasks []*importdata.ImportFileTask, msr *metadb.MigrationStatusRecord) ([]*importdata.ImportFileTask, []sqlname.NameTuple, error) {
	var err error
	if changeStreamingIsEnabled(importType) {
		if tconf.TableList != "" || tconf.ExcludeTableList != "" {
			utils.ErrExit("--table-list and --exclude-table-list are not supported for live migration. Re-run the command without these flags.")
		}
		//For live migration we need to use the source table list to get the import table list
		//as we don't suport filtering tables in import data for live migration so it might be okay to use source side list
		//and one more reason of using that is empty tables which are not present in datafile descriptor but we need to have them in
		// import list for live migration as streaming changes will be done for them
		importTableList, err = getInitialImportTableListForLive(msr.TableListExportedFromSource)
		if err != nil {
			return nil, nil, goerrors.Errorf("Failed to get import table list: %w", err)
		}
		checkTablesPresentInTarget(importTableList) //to check whether tables exist or not we should use importTableList in live migration case as it includes all the tables being migration e.e.g mepty tables etc..

		// When '--use-partition-root false', verify that partitions are consistent between source and target
		if !importUsePartitionRoot {
			checkPartitionConsistency(msr, importTableList)
		}
		return importFileTasks, importTableList, nil
	}
	//for offline migration we need to use the import file tasks to get the import table list
	//as that is the one where we filter the tables via table-list flags
	//we don't need empty tables in offline case, data migration doesn't matter for them
	//Table list after applying table list filter
	importFileTasks = applyTableListFilter(importFileTasks)
	importTableList = importdata.ImportFileTasksToTableNameTuples(importFileTasks)

	return importFileTasks, importTableList, nil
}

// buildImportDataConfig copies the cmd package-level variables the import engine
// used to read directly into importdata.Config. Values only; no callbacks.
func buildImportDataConfig() importdata.Config {
	cfg := importdata.Config{
		ExportDir:                exportDir,
		MetaDB:                   metaDB,
		MigrationUUID:            migrationUUID,
		ImporterRole:             importerRole,
		ImportType:               importType,
		IdentityColumnsMetaDBKey: identityColumnsMetaDBKey,
		SourceDBType:             sourceDBType,
		Source:                   source,
		Tconf:                    tconf,
		Tdb:                      tdb,
		ControlPlane:             controlPlane,

		StartClean:                    startClean,
		TruncateTables:                truncateTables,
		TruncateSplits:                truncateSplits,
		DisablePb:                     disablePb,
		BatchSizeInNumRows:            batchSizeInNumRows,
		ErrorPolicySnapshot:           errorPolicySnapshotFlag,
		EnableRandomBatchProduction:   enableRandomBatchProduction,
		MaxConcurrentBatchProductions: maxConcurrentBatchProductionsConfig,
		SkipReplicationChecks:         skipReplicationChecks,
		SkipNodeHealthChecks:          skipNodeHealthChecks,
		SkipDiskUsageHealthChecks:     skipDiskUsageHealthChecks,
		CdcPartitionKey:               cdcPartitionKey,
		CdcPartitionKeyOverrides:      cdcPartitionKeyOverrides,
		ImportUsePartitionRoot:        importUsePartitionRoot,
		EventBatchMaxRetryCount:       EVENT_BATCH_MAX_RETRY_COUNT,
		ReportProgressInBytes:         reportProgressInBytes,

		DataFileDescriptor:       dataFileDescriptor,
		DataStore:                dataStore,
		ImportTableList:          importTableList,
		CallhomeMetricsCollector: callhomeMetricsCollector,
	}
	// validateCdcPartitionKeyFlags already parsed this string during flag validation;
	// parse it again here so the engine never re-parses the raw flag at runtime.
	parsed, err := parseCdcPartitionKeyOverrides(cdcPartitionKeyOverrides)
	if err != nil {
		utils.ErrExit("%w", err)
	}
	cfg.CdcPartitionKeyOverridesParsed = parsed
	if importerRole == TARGET_DB_IMPORTER_ROLE {
		cfg.SnapshotImportStartedEvent = createSnapshotImportStartedEvent()
	}
	return cfg
}

// runImportDataEngine runs the import engine for the given tasks and then the
// post-processing that used to follow inside importData().
func runImportDataEngine(importFileTasks []*importdata.ImportFileTask) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %w", err)
	}
	if msr.SourceDBConf != nil {
		source = *msr.SourceDBConf
	}
	importer = importdata.NewImporter(buildImportDataConfig())
	importer.ImportData(importFileTasks)
	importDataPostProcessing(msr, importFileTasks)
}

// importDataPostProcessing is the tail of the former importData(): sequence and
// identity-column restoration, cutover processing and the snapshot import report.
func importDataPostProcessing(msr *metadb.MigrationStatusRecord, importFileTasks []*importdata.ImportFileTask) {
	var err error
	if changeStreamingIsEnabled(importType) {
		err = postCutoverProcessing(importTableList)
		if err != nil {
			utils.ErrExit("failed to post cutover processing: %w", err)
		}
		utils.PrintAndLog("\nRun the following command to get the current report of the migration:\n" +
			color.CyanString("yb-voyager get data-migration-report --export-dir %q", exportDir))
	} else {
		err = postSnapshotImportProcessing(msr, importTableList)
		if err != nil {
			utils.ErrExit("failed to post snapshot import processing: %w", err)
		}
		importdata.DisplayImportedRowCountSnapshot(importDataStateConfigFromGlobals(), importer.State(), importFileTasks, importer.ErrorHandler())
	}
	fmt.Printf("\nImport data complete.\n")
}

func postSnapshotImportProcessing(msr *metadb.MigrationStatusRecord, importTableList []sqlname.NameTuple) error {
	err := restoreSequencesInOfflineMigration(msr, importTableList)
	if err != nil {
		return goerrors.Errorf("failed to restore sequences: %w", err)
	}
	return nil
}

func postCutoverProcessing(importTableList []sqlname.NameTuple) error {
	utils.PrintAndLogfInfo("Processing cutover initiate request...\n")
	status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
	if err != nil {
		return goerrors.Errorf("failed to read export status for restore sequences: %w", err)
	}

	// in case of live migration sequences are restored after cutover
	err = restoreSequencesInLiveMigration(status.Sequences)
	if err != nil {
		return goerrors.Errorf("failed to restore sequences: %w", err)
	}

	err = importer.RestoreGeneratedIdentityColumns(importTableList)
	if err != nil {
		return goerrors.Errorf("failed to restore generated columns: %w", err)
	}

	utils.PrintAndLogf("Completed streaming all relevant changes to %s", tconf.TargetDBType)

	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		injectCutoverToTargetImporterPreMarkProcessed()
	case SOURCE_DB_IMPORTER_ROLE:
		injectCutoverToSourceImporterPreMarkProcessed()
	}

	err = markCutoverProcessed(importerRole)
	if err != nil {
		return goerrors.Errorf("failed to mark cutover as processed: %w", err)
	}

	if importerRole == SOURCE_DB_IMPORTER_ROLE {
		injectCutoverToSourceImporterPostMarkProcessed()
	}
	if importerRole == TARGET_DB_IMPORTER_ROLE {
		injectCutoverToTargetImporterPostMarkProcessed()
	}

	//Waiting for the cutover of the export data from target to mark the cutover as processed for the importer role
	//so that we continue the anything required after post cutover processing only when the cutover for both the commands are completed
	//next step will be the initialise the next iteration and then mark the latest iteration number in the MSR. so this is required now to wait first
	err = waitUntilCutoverProcessedByCorrespondingExporterForImporter(importerRole)
	if err != nil {
		return goerrors.Errorf("failed to wait until cutover processed by exporter: %w", err)
	}
	return nil
}

func waitUntilCutoverProcessedByCorrespondingExporterForImporter(importerRole string) error {
	timeout := 2 * time.Minute
	startTime := time.Now()
	if importerRole == TARGET_DB_IMPORTER_ROLE {
		utils.PrintAndLogfInfo("\nWaiting for export data from source to complete...")
	} else {
		utils.PrintAndLogfInfo("\nWaiting for export data from target to complete...")
	}
	for {
		if time.Since(startTime) > timeout {
			if importerRole == TARGET_DB_IMPORTER_ROLE {
				return goerrors.Errorf("timeout waiting for cutover export data from source to complete. Ensure 'export data from source' is running, then re-run this command.")
			} else {
				return goerrors.Errorf("timeout waiting for cutover export data from target to complete. Ensure 'export data from target' is running, then re-run this command.")
			}
		}
		record, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			return fmt.Errorf("failed to get migration status record: %w", err)
		}
		switch importerRole {
		case SOURCE_DB_IMPORTER_ROLE:
			if record.CutoverToSourceProcessedByTargetExporter {
				return nil
			}
		case TARGET_DB_IMPORTER_ROLE:
			if record.CutoverProcessedBySourceExporter {
				return nil
			}
		case SOURCE_REPLICA_DB_IMPORTER_ROLE:
			if record.CutoverToSourceReplicaProcessedByTargetExporter {
				return nil
			}
		default:
			return goerrors.Errorf("invalid importer role: %s", importerRole)
		}
		time.Sleep(2 * time.Second)
	}
}

// getTableTypes returns a map of table name to table type (sharded/colocated) for all tables in the tasks.
// The *FromGlobals helpers copy the cmd package-level variables the importdata
// components used to read directly into their config structs. They are a bridge
// until the import engine itself moves out of cmd (PR C), after which
// importdata.Config supplies these values.
func importDataStateConfigFromGlobals() importdata.ImportDataStateConfig {
	return importdata.ImportDataStateConfig{
		ExportDir:          exportDir,
		ImporterRole:       importerRole,
		Tdb:                tdb,
		Tconf:              tconf,
		MigrationUUID:      migrationUUID,
		TruncateSplits:     truncateSplits,
		DataFileDescriptor: dataFileDescriptor,
	}
}

func newImportDataStateFromGlobals() *importdata.ImportDataState {
	return importdata.NewImportDataState(importDataStateConfigFromGlobals())
}

func packAndSendImportDataToTargetPayload(status string, errorMsg error) {

	if !shouldSendCallhome() {
		return
	}

	//basic payload details
	payload := createCallhomePayload(migrationUUID)
	switch importType {
	case SNAPSHOT_ONLY:
		payload.MigrationType = OFFLINE
	case SNAPSHOT_AND_CHANGES:
		payload.MigrationType = LIVE_MIGRATION
	}
	payload.TargetDBDetails = callhome.MarshalledJsonString(targetDBDetails)
	payload.MigrationPhase = IMPORT_DATA_PHASE

	dataMetrics := callhome.ImportDataMetrics{}
	if callhomeMetricsCollector != nil {
		dataMetrics.SnapshotTotalRows = callhomeMetricsCollector.GetSnapshotTotalRows()
		dataMetrics.SnapshotTotalBytes = callhomeMetricsCollector.GetSnapshotTotalBytes()
		dataMetrics.CurrentParallelConnections = callhomeMetricsCollector.GetCurrentParallelConnections()
	}

	// Get phase-related metrics from existing logic
	// TODO: fix: pass proper error handler here.
	importRowsMap, err := importdata.GetImportedSnapshotRowsMap("target", importTableList, nil, importDataStateConfigFromGlobals())
	if err != nil {
		log.Infof("callhome: error in getting the import data: %v", err)
	} else {
		// callhome payload assembly; the callback never returns an error
		_ = importRowsMap.IterKV(func(key sqlname.NameTuple, value importdata.RowCountPair) (bool, error) {
			dataMetrics.MigrationSnapshotTotalRows += value.Imported
			if value.Imported > dataMetrics.MigrationSnapshotLargestTableRows {
				dataMetrics.MigrationSnapshotLargestTableRows = value.Imported
			}
			return true, nil
		})
	}

	// Set live migration metrics if applicable
	if currentImportPhase() != dbzm.MODE_SNAPSHOT && currentStatsReporter() != nil {
		dataMetrics.MigrationCdcTotalImportedEvents = currentStatsReporter().TotalEventsImported
		dataMetrics.CdcEventsImportRate3min = currentStatsReporter().EventsImportRateLast3Min
	}

	// Set table list count
	dataMetrics.TableListCount = len(importTableList)

	importDataPayload := callhome.ImportDataPhasePayload{
		PayloadVersion:             callhome.IMPORT_DATA_CALLHOME_PAYLOAD_VERSION,
		ParallelJobs:               int64(tconf.Parallelism),
		StartClean:                 bool(startClean),
		EnableUpsert:               bool(tconf.EnableUpsert),
		Error:                      callhome.SanitizeErrorMsg(errorMsg, anonymizer),
		ControlPlaneType:           getControlPlaneType(),
		BatchSize:                  batchSizeInNumRows,
		OnPrimaryKeyConflictAction: tconf.OnPrimaryKeyConflictAction,
		// TODO: store the mode properly
		EnableYBAdaptiveParallelism: tconf.AdaptiveParallelismMode.IsEnabled(),
		AdaptiveParallelismMax:      int64(tconf.MaxParallelism),
		ErrorPolicySnapshot:         errorPolicySnapshotFlag.String(),
		DataMetrics:                 dataMetrics,
		Phase:                       currentImportPhase(),
	}

	var err2 error
	importDataPayload.YBClusterMetrics, err2 = BuildCallhomeYBClusterMetrics()
	if err2 != nil {
		log.Infof("callhome: error in getting the YB cluster metrics: %v", err2)
	}

	// Below adds cutover timings if applicable
	msr, err := metaDB.GetMigrationStatusRecord()
	if err == nil {
		importDataPayload.CutoverTimings = CalculateCutoverTimingsForTarget(msr)
	} else {
		log.Infof("callhome: error getting MSR for cutover timings: %v", err)
	}

	payload.PhasePayload = callhome.MarshalledJsonString(importDataPayload)
	payload.Status = status

	err = callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func checkExportDataDoneOrStartedFlag() {
	metaInfoDir := filepath.Join(exportDir, metaInfoDirName)
	_, err := os.Stat(metaInfoDir)
	if err != nil {
		utils.ErrExit("metainfo dir is missing. Exiting.")
	}

	if dataIsExported() {
		return
	}

	if importType == CHANGES_ONLY {
		//For changes only the data exported is marked done once the slot is created
		msg := lo.Ternary(importerRole == TARGET_DB_IMPORTER_ROLE, "Waiting for export data from source to start...", "Waiting for export data from target to start...")
		utils.PrintAndLog(msg)
	} else {
		//for snapshot it is marked done once the snapshot is complete
		utils.PrintAndLogf("Waiting for snapshot data export to complete...")

	}
	for !dataIsExported() {
		time.Sleep(time.Second * 2)
	}
	if importType == CHANGES_ONLY {
		utils.PrintAndLogf("Export data from source is started.")
	} else {
		utils.PrintAndLogf("Snapshot data export is complete.")
	}
}

func init() {
	// adding child commands to parent import commands
	importCmd.AddCommand(importDataCmd)
	importDataCmd.AddCommand(importDataToCmd)
	importDataToCmd.AddCommand(importDataToTargetCmd)

	// adding flags to the `import data` and `import data to target` commands
	registerFlagsForTarget(importDataCmd)
	registerFlagsForTarget(importDataToTargetCmd)
	registerCommonGlobalFlags(importDataCmd)
	registerCommonGlobalFlags(importDataToTargetCmd)
	registerCommonImportFlags(importDataCmd)
	registerCommonImportFlags(importDataToTargetCmd)
	mustMarkFlagHidden(importDataCmd, "continue-on-error")
	mustMarkFlagHidden(importDataToTargetCmd, "continue-on-error")
	registerTargetDBConnFlags(importDataCmd)
	registerTargetDBConnFlags(importDataToTargetCmd)
	registerTargetDBTypeFlag(importDataCmd)
	registerTargetDBTypeFlag(importDataToTargetCmd)
	registerImportDataCommonFlags(importDataCmd)
	registerImportDataCommonFlags(importDataToTargetCmd)
	registerImportUsePartitionRootFlagToTarget(importDataCmd)
	registerImportUsePartitionRootFlagToTarget(importDataToTargetCmd)
	registerImportDataToTargetFlags(importDataCmd)
	registerImportDataToTargetFlags(importDataToTargetCmd)
}

func createSnapshotImportStartedEvent() cp.SnapshotImportStartedEvent {
	result := cp.SnapshotImportStartedEvent{}
	initBaseTargetEvent(&result.BaseEvent, "IMPORT DATA")
	return result
}

func createSnapshotImportCompletedEvent() cp.SnapshotImportCompletedEvent {
	result := cp.SnapshotImportCompletedEvent{}
	initBaseTargetEvent(&result.BaseEvent, "IMPORT DATA")
	return result
}

func saveOnPrimaryKeyConflictActionInMSR() {
	if !isPrimaryKeyConflictModeValid() {
		return
	}

	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.OnPrimaryKeyConflictAction = tconf.OnPrimaryKeyConflictAction
	})
	if err != nil {
		utils.ErrExit("failed to save on-primary-key-conflict action in migration status record: %w", err)
	}
}

func isPrimaryKeyConflictModeValid() bool {
	return importerRole == IMPORT_FILE_ROLE || importerRole == TARGET_DB_IMPORTER_ROLE
}

func BuildCallhomeYBClusterMetrics() (callhome.YBClusterMetrics, error) {
	// YB cluster metrics only exist for a real YugabyteDB target. Non-YB targets
	// (yb-amp's single-node PG-compatible compute, PG fall-back/forward roles)
	// have no such API — return empty rather than a misleading type-assertion error.
	if tconf.TargetDBType != YUGABYTEDB {
		return callhome.YBClusterMetrics{}, nil
	}
	yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return callhome.YBClusterMetrics{}, goerrors.Errorf("importData: expected tdb to be of type TargetYugabyteDB, got: %T", tdb)
	}

	clusterMetrics, err := yb.GetClusterMetrics()
	if err != nil {
		return callhome.YBClusterMetrics{}, err
	}

	now := time.Now().UTC()
	nodes := make([]callhome.NodeMetric, 0)
	var totalCpuPct, maxCpuPct float64
	for _, nodeMetrics := range clusterMetrics {
		// in case of err value will be -1
		cpuPct, err := nodeMetrics.GetCPUPercent()
		if err != nil {
			// ignore and not error out - getter function can fail for a node but we would still like to collect metrics for other nodes
			log.Warnf("callhome: error getting CPU percent for node %s: %v", nodeMetrics.UUID, err)
		}
		memPct, err := nodeMetrics.GetMemPercent()
		if err != nil {
			log.Warnf("callhome: error getting Mem percent for node %s: %v", nodeMetrics.UUID, err)
		}

		// in case of err value will be -1
		memoryFree, err := nodeMetrics.GetMemoryFree()
		if err != nil {
			log.Warnf("callhome: error getting Memory Free for node %s: %v", nodeMetrics.UUID, err)
		}
		memoryAvailable, err := nodeMetrics.GetMemoryAvailable()
		if err != nil {
			log.Warnf("callhome: error getting Memory Available for node %s: %v", nodeMetrics.UUID, err)
		}
		memoryTotal, err := nodeMetrics.GetMemoryTotal()
		if err != nil {
			log.Warnf("callhome: error getting Memory Total for node %s: %v", nodeMetrics.UUID, err)
		}

		nodes = append(nodes, callhome.NodeMetric{
			UUID:                   nodeMetrics.UUID,
			TotalCPUPct:            cpuPct,
			TserverMemSoftLimitPct: memPct,
			MemoryFree:             memoryFree,
			MemoryAvailable:        memoryAvailable,
			MemoryTotal:            memoryTotal,
			Status:                 nodeMetrics.Status,
			Error:                  nodeMetrics.Error,
		})

		totalCpuPct += cpuPct
		if cpuPct > maxCpuPct {
			maxCpuPct = cpuPct
		}
	}

	if len(nodes) == 0 {
		return callhome.YBClusterMetrics{}, goerrors.Errorf("no nodes found in cluster metrics")
	}

	avgCpuPct := totalCpuPct / float64(len(nodes))
	return callhome.YBClusterMetrics{
		Timestamp: now,
		AvgCpuPct: avgCpuPct,
		MaxCpuPct: maxCpuPct,
		Nodes:     nodes,
	}, nil
}
