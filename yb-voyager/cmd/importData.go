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
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/adaptiveparallelism"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/monitor"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var metaInfoDirName = META_INFO_DIR_NAME
var batchSizeInNumRows = int64(0)
var batchImportPool *pool.Pool
var colocatedBatchImportPool *pool.Pool
var colocatedBatchImportQueue chan func()

var tablesProgressMetadata map[string]*utils.TableProgressMetadata
var importerRole string
var identityColumnsMetaDBKey string
var importPhase string

// stores the data files description in a struct
var dataFileDescriptor *datafile.Descriptor
var truncateSplits utils.BoolStr                                             // to truncate *.D splits after import
var TableToColumnNames = utils.NewStructMap[sqlname.NameTuple, []string]()   // map of table name to columnNames
var TableToIdentityColumnNames *utils.StructMap[sqlname.NameTuple, []string] // map of table name to generated always as identity column's names
var valueConverter dbzm.ValueConverter

var TableNameToSchema *utils.StructMap[sqlname.NameTuple, map[string]map[string]string]
var conflictDetectionCache *ConflictDetectionCache
var targetDBDetails *callhome.TargetDBDetails
var skipReplicationChecks utils.BoolStr
var skipNodeHealthChecks utils.BoolStr
var skipDiskUsageHealthChecks utils.BoolStr
var progressReporter *ImportDataProgressReporter

// Error policy
var errorPolicySnapshotFlag importdata.ErrorPolicy = importdata.AbortErrorPolicy

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

		err := retrieveMigrationUUID()
		if err != nil {
			utils.ErrExit("failed to get migration UUID: %w", err)
		}
		sourceDBType = GetSourceDBTypeFromMSR()
		err = validateImportFlags(cmd, importerRole)
		if err != nil {
			utils.ErrExit("Error validating import flags: %s", err.Error())
		}

		err = validateImportDataFlags()
		if err != nil {
			utils.ErrExit("Error validating import data flags: %s", err.Error())
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

func importDataCommandFn(cmd *cobra.Command, args []string) {
	importPhase = dbzm.MODE_SNAPSHOT
	ExitIfAlreadyCutover(importerRole)
	reportProgressInBytes = false
	tconf.ImportMode = true
	checkExportDataDoneFlag()

	/*
		Before this point MSR won't not be initialised in case of importDataFileCmd
		In case of importDataCmd, MSR would be initialised already by previous commands
	*/
	saveOnPrimaryKeyConflictActionInMSR()

	sourceDBType = GetSourceDBTypeFromMSR()
	sqlname.SourceDBType = sourceDBType

	if tconf.TargetDBType == YUGABYTEDB {
		tconf.Schema = strings.ToLower(tconf.Schema)
	} else if tconf.TargetDBType == ORACLE && !utils.IsQuotedString(tconf.Schema) {
		tconf.Schema = strings.ToUpper(tconf.Schema)
	}
	tdb = tgtdb.NewTargetDB(&tconf)
	err := tdb.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB: %s", err)
	}
	// Check if target DB has the required permissions
	if tconf.RunGuardrailsChecks {
		checkImportDataPermissions()
	}

	targetDBDetails = tdb.GetCallhomeTargetDBInfo()

	// we don't want to re-register in case import data to source/source-replica
	reregisterYBNames := importerRole == TARGET_DB_IMPORTER_ROLE && bool(startClean)
	err = InitNameRegistry(exportDir, importerRole, nil, nil, &tconf, tdb, reregisterYBNames)
	if err != nil {
		utils.ErrExit("initialize name registry: %v", err)
	}

	dataStore = datastore.NewDataStore(filepath.Join(exportDir, "data"))
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	log.Infof("Parsed DataFileDescriptor: %v", spew.Sdump(dataFileDescriptor))
	// TODO: handle case-sensitive in table names with oracle ff-db
	// quoteTableNameIfRequired()
	importFileTasks := discoverFilesToImport()
	log.Debugf("Discovered import file tasks: %v", importFileTasks)

	err = setImportTypeAndIdentityColumnMetaDBKeyForImporterRole(importerRole)
	if err != nil {
		utils.ErrExit("error while setting import type or identity column metadb key: %v", err)
	}

	if changeStreamingIsEnabled(importType) && (tconf.TableList != "" || tconf.ExcludeTableList != "") {
		utils.ErrExit("--table-list and --exclude-table-list are not supported for live migration. Re-run the command without these flags.")
	} else {
		importFileTasks = applyTableListFilter(importFileTasks)
	}

	importData(importFileTasks, errorPolicySnapshotFlag)
	tdb.Finalize()
	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		importDataCompletedEvent := createSnapshotImportCompletedEvent()
		controlPlane.SnapshotImportCompleted(&importDataCompletedEvent)
		packAndSendImportDataToTargetPayload(COMPLETE, nil)
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		packAndSendImportDataToSrcReplicaPayload(COMPLETE, nil)
	case SOURCE_DB_IMPORTER_ROLE:
		packAndSendImportDataToSourcePayload(COMPLETE, nil)
	}

	if changeStreamingIsEnabled(importType) {
		startExportDataFromTargetIfRequired()
	}
}

func setImportTypeAndIdentityColumnMetaDBKeyForImporterRole(importerRole string) error {

	record, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("Failed to get migration status record: %s", err)
	}

	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		importType = record.ExportType
		identityColumnsMetaDBKey = metadb.TARGET_DB_IDENTITY_COLUMNS_KEY
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		if record.FallbackEnabled {
			return fmt.Errorf("cannot import data to source-replica. Fall-back workflow is already enabled.")
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
			utils.ErrExit("Failed to check if triggers and FKs are enabled: %s", err)
		}
		if len(enabledTriggers) > 0 || len(enabledFks) > 0 {
			if len(enabledTriggers) > 0 {
				utils.PrintAndLog("%s [%s]", color.RedString("\nEnabled Triggers:"), strings.Join(enabledTriggers, ", "))
			}
			if len(enabledFks) > 0 {
				utils.PrintAndLog("%s [%s]", color.RedString("\nEnabled Foreign Keys:"), strings.Join(enabledFks, ", "))
			}
			fmt.Printf("\n%s", color.RedString("Disable the above triggers and FKs before importing data.\n"))
			fkAndTriggersCheckFailed = true
			fmt.Println("\nCheck the documentation to disable triggers and FKs:", color.BlueString("https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-back/#cutover-to-the-target"))
		}
	}

	missingPermissions, err := tdb.GetMissingImportDataPermissions(importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE)
	if err != nil {
		utils.ErrExit("Failed to get missing import data permissions: %s", err)
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
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		return
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch MigrationStatusRecord: %w", err)
	}
	if !msr.FallForwardEnabled && !msr.FallbackEnabled {
		utils.PrintAndLog("No fall-forward/back enabled. Exiting.")
		return
	}
	tableListExportedFromSource := msr.TableListExportedFromSource
	importTableList, err := getImportTableList(tableListExportedFromSource)
	if err != nil {
		utils.ErrExit("failed to generate table list : %v", err)
	}
	importTableNames := lo.Map(importTableList, func(tableName sqlname.NameTuple, _ int) string {
		return tableName.ForUserQuery()
	})

	lockFile.Unlock() // unlock export dir from import data cmd before switching current process to ff/fb sync cmd

	cmd := []string{"yb-voyager", "export", "data", "from", "target",
		"--export-dir", exportDir,
		fmt.Sprintf("--send-diagnostics=%t", callhome.SendDiagnostics),
		"--log-level", config.LogLevel,
	}

	passExportDataFromTargetSpecificCLIFlags := map[string]bool{
		"disable-pb":           true,
		"table-list":           true,
		"transaction-ordering": true,
	}

	// Check whether the command specifc flags have been set in the config file
	keysSetInConfig, err := readConfigFileAndGetExportDataFromTargetKeys()
	var displayCmdAndExit bool
	var configFileErr error
	if err != nil {
		displayCmdAndExit = true
		configFileErr = err
	} else {
		for key := range passExportDataFromTargetSpecificCLIFlags {
			if slices.Contains(keysSetInConfig, key) {
				passExportDataFromTargetSpecificCLIFlags[key] = false
			}
		}
	}

	// Log which command specific flags are to be passed to the command
	log.Infof("Command specific flags to be passed to export data from target: %v", passExportDataFromTargetSpecificCLIFlags)

	// Command specific flags
	if bool(disablePb) && passExportDataFromTargetSpecificCLIFlags["disable-pb"] {
		cmd = append(cmd, "--disable-pb=true")
	}
	if passExportDataFromTargetSpecificCLIFlags["transaction-ordering"] {
		cmd = append(cmd, fmt.Sprintf("--transaction-ordering=%t", transactionOrdering))
	}
	if passExportDataFromTargetSpecificCLIFlags["table-list"] {
		cmd = append(cmd, "--table-list", strings.Join(importTableNames, ","))
	}

	if msr.UseYBgRPCConnector {
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

	cmdStr := "TARGET_DB_PASSWORD=*** " + strings.Join(cmd, " ")

	utils.PrintAndLog("Starting export data from target with command:\n %s", color.GreenString(cmdStr))

	// If error had occurred while reading the config file, display the command and exit
	if displayCmdAndExit {
		// We are delaying this error message to be displayed here so that we can display the command
		// after it has been constructed
		utils.ErrExit("failed to read config file: %s\nPlease check the config file and re-run the command with only the required flags", configFileErr)
	}

	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager: %w", lookErr)
	}
	env := os.Environ()
	env = slices.Insert(env, 0, "TARGET_DB_PASSWORD="+tconf.Password)

	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager export data from target: %w\n Please re-run with command :\n%s", execErr, cmdStr)
	}
}

type ImportFileTask struct {
	ID           int
	FilePath     string
	TableNameTup sqlname.NameTuple
	RowCount     int64
	FileSize     int64
}

func (task *ImportFileTask) String() string {
	return fmt.Sprintf("{ID: %d, FilePath: %s, TableName: %s, RowCount: %d, FileSize: %d}", task.ID, task.FilePath, task.TableNameTup.ForOutput(), task.RowCount, task.FileSize)
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

func discoverFilesToImport() []*ImportFileTask {
	result := []*ImportFileTask{}
	if dataFileDescriptor.DataFileList == nil {
		utils.ErrExit("It looks like the data is exported using older version of Voyager. Please use matching version to import the data.")
	}

	for i, fileEntry := range dataFileDescriptor.DataFileList {
		if fileEntry.RowCount == 0 {
			// In case of PG Live migration  pg_dump and dbzm both are used and we don't skip empty tables
			// but pb hangs for empty so skipping empty tables in snapshot import
			continue
		}
		tableName, err := namereg.NameReg.LookupTableName(fileEntry.TableName)
		if err != nil {
			utils.ErrExit("lookup table name from name registry: %v", err)
		}
		task := &ImportFileTask{
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

func applyTableListFilter(importFileTasks []*ImportFileTask) []*ImportFileTask {
	result := []*ImportFileTask{}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch migration status record: %w", err)
	}
	source = *msr.SourceDBConf
	_, noDefaultSchema := getDefaultSourceSchemaName()

	allTables := lo.Uniq(lo.Map(importFileTasks, func(task *ImportFileTask, _ int) sqlname.NameTuple {
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
				utils.ErrExit("Invalid table name pattern: %q: %s", pattern, err)
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
		utils.PrintAndLog("Unknown table names in the table-list: %v", allUnknown)
		utils.PrintAndLog("Valid table names are: %v", lo.Map(allTables, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		}))
		utils.ErrExit("Please fix the table names in table-list and retry.")
	}

	for _, task := range importFileTasks {
		if len(includeList) > 0 && !slices.Contains(includeList, task.TableNameTup) {
			log.Infof("Skipping table %q (fileName: %s) as it is not in the include list", task.TableNameTup, task.FilePath)
			continue
		}
		if len(excludeList) > 0 && slices.Contains(excludeList, task.TableNameTup) {
			log.Infof("Skipping table %q (fileName: %s) as it is in the exclude list", task.TableNameTup, task.FilePath)
			continue
		}
		result = append(result, task)
	}
	return result
}

func updateTargetConfInMigrationStatus() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch importerRole {
		case TARGET_DB_IMPORTER_ROLE, IMPORT_FILE_ROLE:
			record.TargetDBConf = tconf.Clone()
			record.TargetDBConf.Password = ""
			record.TargetDBConf.Uri = ""
		case SOURCE_REPLICA_DB_IMPORTER_ROLE:
			record.SourceReplicaDBConf = tconf.Clone()
			record.SourceReplicaDBConf.Password = ""
			record.SourceReplicaDBConf.Uri = ""
		case SOURCE_DB_IMPORTER_ROLE:
			record.SourceDBAsTargetConf = tconf.Clone()
			record.SourceDBAsTargetConf.Password = ""
			record.SourceDBAsTargetConf.Uri = ""
		default:
			panic(fmt.Sprintf("unsupported importer role: %s", importerRole))
		}
	})
	if err != nil {
		utils.ErrExit("Failed to update target conf in migration status record: %s", err)
	}
}

func importData(importFileTasks []*ImportFileTask, errorPolicy importdata.ErrorPolicy) {

	if (importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE) && (tconf.EnableUpsert) {
		if !utils.AskPrompt(color.RedString("WARNING: Ensure that tables on target YugabyteDB do not have secondary indexes. " +
			"If a table has secondary indexes, setting --enable-upsert to true may lead to corruption of the indexes. Are you sure you want to proceed?")) {
			utils.ErrExit("Aborting import.")
		}
	}

	exportDirDataDir := filepath.Join(exportDir, "data")
	errorHandler, err := importdata.GetImportDataErrorHandler(errorPolicy, exportDirDataDir)
	if err != nil {
		utils.ErrExit("Failed to initialize error handler: %s", err)
	}

	if importerRole == TARGET_DB_IMPORTER_ROLE {
		importDataStartEvent := createSnapshotImportStartedEvent()
		controlPlane.SnapshotImportStarted(&importDataStartEvent)
	}
	updateTargetConfInMigrationStatus()
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %s", err)
	}

	if msr.IsSnapshotExportedViaDebezium() {
		valueConverter, err = dbzm.NewValueConverter(exportDir, tdb, tconf, importerRole, msr.SourceDBConf.DBType)
	} else {
		valueConverter, err = dbzm.NewNoOpValueConverter()
	}
	if err != nil {
		utils.ErrExit("create value converter: %s", err)
	}

	TableNameToSchema, err = valueConverter.GetTableNameToSchema()
	if err != nil {
		utils.ErrExit("getting table name to schema: %s", err)
	}

	err = tdb.InitConnPool()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB connection pool: %s", err)
	}

	var adaptiveParallelismStarted bool
	if tconf.EnableYBAdaptiveParallelism {
		adaptiveParallelismStarted, err = startAdaptiveParallelism()
		if err != nil {
			utils.ErrExit("Failed to start adaptive parallelism: %s", err)
		}
	}
	progressReporter = NewImportDataProgressReporter(bool(disablePb))
	err = startMonitoringTargetYBHealth()
	if err != nil {
		utils.ErrExit("Failed to start monitoring health: %s", err)
	}
	if adaptiveParallelismStarted {
		utils.PrintAndLog("Using 1-%d parallel jobs (adaptive)", tconf.MaxParallelism)
	} else {
		utils.PrintAndLog("Using %d parallel jobs.", tconf.Parallelism)
	}

	targetDBVersion := tdb.GetVersion()
	fmt.Printf("%s version: %s\n", tconf.TargetDBType, targetDBVersion)

	err = tdb.CreateVoyagerSchema()
	if err != nil {
		utils.ErrExit("Failed to create voyager metadata schema on target DB: %s", err)
	}

	utils.PrintAndLog("\nimport of data in %q database started", tconf.DBName)
	var pendingTasks, completedTasks []*ImportFileTask
	state := NewImportDataState(exportDir)
	if startClean {
		err := cleanMSRForImportDataStartClean()
		if err != nil {
			utils.ErrExit("Failed to clean MigrationStatusRecord for import data start clean: %s", err)
		}
		cleanImportState(state, importFileTasks)
		err = cleanStoredErrors(errorHandler, importFileTasks)
		if err != nil {
			utils.ErrExit("Failed to clean stored errors: %s", err)
		}
		pendingTasks = importFileTasks
	} else {
		pendingTasks, completedTasks, err = classifyTasksForImport(state, importFileTasks)
		if err != nil {
			utils.ErrExit("Failed to classify tasks: %s", err)
		}
	}
	log.Infof("pending tasks: %v", pendingTasks)
	log.Infof("completed tasks: %v", completedTasks)

	err = runPKConflictModeGuardrails(state, importFileTasks)
	if err != nil {
		utils.ErrExit("Error checking PK conflict mode on fresh start: %s", err)
	}

	//TODO: BUG: we are applying table-list filter on importFileTasks, but here we are considering all tables as per
	// export-data table-list. Should be fine because we are only disabling and re-enabling, but this is still not ideal.
	sourceTableList := msr.TableListExportedFromSource
	if msr.SourceDBConf != nil {
		source = *msr.SourceDBConf
	}
	importTableList, err := getImportTableList(sourceTableList)
	if err != nil {
		utils.ErrExit("Error generating table list to import: %v", err)
	}

	err = fetchAndStoreGeneratedAlwaysIdentityColumnsInMetadb(importTableList)
	if err != nil {
		utils.ErrExit("error fetching or storing the generated always identity columns: %v", err)
	}
	err = disableGeneratedAlwaysAsIdentityColumns()
	if err != nil {
		utils.ErrExit("error disabling generated always identity columns: %v", err)
	}
	// Import snapshots
	if importerRole != SOURCE_DB_IMPORTER_ROLE {
		utils.PrintAndLog("Already imported tables: %v", importFileTasksToTableNames(completedTasks))
		if len(pendingTasks) == 0 {
			utils.PrintAndLog("All the tables are already imported, nothing left to import\n")
		} else {
			utils.PrintAndLog("Tables to import: %v", importFileTasksToTableNames(pendingTasks))
			prepareTableToColumns(pendingTasks) //prepare the tableToColumns map
			maxParallelConns, err := getMaxParallelConnections()
			if err != nil {
				utils.ErrExit("Failed to get max parallel connections: %s", err)
			}
			if importerRole == TARGET_DB_IMPORTER_ROLE {
				importDataAllTableMetrics := createInitialImportDataTableMetrics(pendingTasks)
				controlPlane.UpdateImportedRowCount(importDataAllTableMetrics)
			}

			useTaskPicker := utils.GetEnvAsBool("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT", true)
			if useTaskPicker {
				maxColocatedBatchesInProgress := utils.GetEnvAsInt("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS", 3)
				err := importTasksViaTaskPicker(pendingTasks, state, progressReporter,
					maxParallelConns, maxParallelConns, maxColocatedBatchesInProgress,
					errorHandler)
				if err != nil {
					utils.ErrExit("Failed to import tasks via task picker. %s", err)
				}
			} else {
				poolSize := maxParallelConns * 2
				for _, task := range pendingTasks {
					// The code can produce `poolSize` number of batches at a time. But, it can consume only
					// `parallelism` number of batches at a time.
					batchImportPool = pool.New().WithMaxGoroutines(poolSize)
					log.Infof("created batch import pool of size: %d", poolSize)

					taskImporter, err := NewFileTaskImporter(task, state, batchImportPool, progressReporter, nil, false, errorHandler)
					if err != nil {
						utils.ErrExit("Failed to create file task importer: %s", err)
					}

					for !taskImporter.AllBatchesSubmitted() {
						err := taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
						if err != nil {
							utils.ErrExit("Failed to submit next batch: task:%v err: %s", task, err)
						}
					}

					batchImportPool.Wait() // wait for file import to finish
					taskImporter.PostProcess()
				}
				time.Sleep(time.Second * 2)
			}
		}
		utils.PrintAndLog("snapshot data import complete\n\n")
	}

	if changeStreamingIsEnabled(importType) {
		if importerRole != SOURCE_DB_IMPORTER_ROLE {
			displayImportedRowCountSnapshot(state, importFileTasks, errorHandler)
		}

		waitForDebeziumStartIfRequired()
		importPhase = dbzm.MODE_STREAMING
		color.Blue("streaming changes to %s...", tconf.TargetDBType)

		if err != nil {
			utils.ErrExit("failed to get table unique key columns map: %s", err)
		}
		valueConverter, err = dbzm.NewValueConverter(exportDir, tdb, tconf, importerRole, source.DBType)
		if err != nil {
			utils.ErrExit("Failed to create value converter: %s", err)
		}
		err = streamChanges(state, importTableList)
		if err != nil {
			utils.ErrExit("Failed to stream changes to %s: %s", tconf.TargetDBType, err)
		}

		status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
		if err != nil {
			utils.ErrExit("failed to read export status for restore sequences: %s", err)
		}
		// in case of live migration sequences are restored after cutover
		err = tdb.RestoreSequences(status.Sequences)
		if err != nil {
			utils.ErrExit("failed to restore sequences: %s", err)
		}
		err = restoreGeneratedIdentityColumns(importTableList)
		if err != nil {
			utils.ErrExit("failed to restore generated columns: %s", err)
		}
		utils.PrintAndLog("Completed streaming all relevant changes to %s", tconf.TargetDBType)
		err = markCutoverProcessed(importerRole)
		if err != nil {
			utils.ErrExit("failed to mark cutover as processed: %s", err)
		}
		utils.PrintAndLog("\nRun the following command to get the current report of the migration:\n" +
			color.CyanString("yb-voyager get data-migration-report --export-dir %q", exportDir))
	} else {
		// offline migration; either using dbzm or pg_dump/ora2pg
		if !msr.IsSnapshotExportedViaDebezium() {
			errImport := executePostSnapshotImportSqls()
			if errImport != nil {
				utils.ErrExit("Error in importing finalize-schema-post-data-import sql: %v", err)
			}
		} else {
			status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
			if err != nil {
				utils.ErrExit("failed to read export status for restore sequences: %s", err)
			}
			err = tdb.RestoreSequences(status.Sequences)
			if err != nil {
				utils.ErrExit("failed to restore sequences: %s", err)
			}
		}
		err = restoreGeneratedIdentityColumns(importTableList)
		if err != nil {
			utils.ErrExit("failed to restore generated columns: %s", err)
		}
		displayImportedRowCountSnapshot(state, importFileTasks, errorHandler)
	}
	fmt.Printf("\nImport data complete.\n")
}

// For a fresh start but non empty tables in tableList && OnPrimaryKeyConflict is set to IGNORE -> notify user
func runPKConflictModeGuardrails(state *ImportDataState, allTasks []*ImportFileTask) error {
	// in case of ERROR mode, no need to check for non-empty tables
	// but for IGNORE or UPDATE(in future), we need to prompt user
	if tconf.OnPrimaryKeyConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR {
		return nil
	}

	if !isTargetDBImporter(importerRole) {
		return nil
	}

	if !isFreshStart(state, allTasks) {
		log.Info("Not a fresh start, skipping primary key conflict mode check.")
		return nil
	}

	pendingTasks := getPendingTasks(state, allTasks)
	pendingTablesList := importFileTasksToTableNameTuples(pendingTasks)
	nonEmptyTables := tdb.GetNonEmptyTables(pendingTablesList)
	if len(nonEmptyTables) == 0 {
		log.Info("No non-empty tables found in the target DB, skipping primary key conflict mode check.")
		return nil
	}

	var nonEmptyTablesWithPK []sqlname.NameTuple
	for _, table := range nonEmptyTables {
		colList, err := tdb.GetPrimaryKeyColumns(table)
		if err != nil {
			return fmt.Errorf("failed to get primary key columns for table %s: %w", table.ForOutput(), err)
		}
		if len(colList) > 0 { // table has PK
			nonEmptyTablesWithPK = append(nonEmptyTablesWithPK, table)
		}
	}

	// all nonEmptyTables have no primary key columns
	if len(nonEmptyTablesWithPK) == 0 {
		log.Infof("No non-empty tables with primary key found in %v, skipping primary key conflict mode check.",
			sqlname.NameTupleListToStrings(nonEmptyTables))
		return nil
	}

	utils.PrintAndLog(
		"\nTarget tables with pre-existing data: %v\n"+
			"Note that because of the config on-primary-key-conflict as 'IGNORE', rows that have a primary key conflict "+
			"with an existing row in the above set of tables will be silently ignored.\n",
		sqlname.NameTupleListToStrings(nonEmptyTablesWithPK),
	)
	if !utils.AskPrompt("Please confirm whether to proceed") {
		utils.ErrExit("Aborting import.")
	}

	return nil
}

// A fresh start is when all tasks are pending(non-zero), no started or completed tasks.
func isFreshStart(state *ImportDataState, allTasks []*ImportFileTask) bool {
	inProgressTasks := getInProgressTasks(state, allTasks)
	notStartedTasks := getNotStartedTasks(state, allTasks)
	completedTasks := getCompletedTasks(state, allTasks)

	return len(inProgressTasks) == 0 && len(completedTasks) == 0 && len(notStartedTasks) == len(allTasks)
}

func restoreGeneratedIdentityColumns(importTableList []sqlname.NameTuple) error {
	if importTableList == nil {
		return nil
	}
	// restore value for IDENTITY BY DEFAULT columns once IDENTITY ALWAYS columns are enabled back
	err := enableGeneratedAlwaysAsIdentityColumns()
	if err != nil {
		return err
	}

	//To restore the sequence value of the BY DEFAULT columns using the ALTER TABLE .. ALTER COLUMN .. SET GENERATED BY DEFAULT which updates the sequence value as well
	//TODO: these sequences of all identity columns are also covered as part of the RestoreSequences as well so we don't need this ALTER
	//see if we should remove it.
	err = restoreGeneratedByDefaultAsIdentityColumns(importTableList)
	if err != nil {
		return err
	}

	return nil
}

func getMaxParallelConnections() (int, error) {
	maxParallelConns := tconf.Parallelism
	if tconf.EnableYBAdaptiveParallelism {
		// in case of adaptive parallelism, we need to use maxParalllelism * 2
		yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
		if !ok {
			return 0, fmt.Errorf("adaptive parallelism is only supported if target DB is YugabyteDB")
		}
		maxParallelConns = yb.GetNumMaxConnectionsInPool()
	}
	return maxParallelConns, nil
}

/*
1. Initialize a worker pool. In case of TARGET_DB_IMPORTER_ROLE  or IMPORT_FILE_ROLE, also create a colocated batch import pool and a corresponding queue.
2. Create a task picker which helps the importer choose which task to process in each iteration.
3. Loop until all tasks are done:
  - Pick a task from the task picker.
  - If the task is not already being processed, create a new FileTaskImporter for the task.
  - For the task that is picked, produce the next batch and submit it to the worker pool. Worker will asynchronously import the batch.
  - If task is done, mark it as done in the task picker.
*/
func importTasksViaTaskPicker(pendingTasks []*ImportFileTask, state *ImportDataState, progressReporter *ImportDataProgressReporter, maxParallelConns int,
	maxShardedTasksInProgress int, maxColocatedBatchesInProgress int, errorHandler importdata.ImportDataErrorHandler) error {

	var err error

	setupWorkerPoolAndQueue(maxParallelConns, maxColocatedBatchesInProgress)
	taskImporters := map[int]*FileTaskImporter{}
	tableTypes, err := getTableTypes(pendingTasks)
	if err != nil {
		return fmt.Errorf("get table types: %w", err)
	}

	var taskPicker FileTaskPicker
	var yb *tgtdb.TargetYugabyteDB
	var ok bool
	if importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE {
		yb, ok = tdb.(*tgtdb.TargetYugabyteDB)
		if !ok {
			return fmt.Errorf("expected tdb to be of type TargetYugabyteDB, got: %T", tdb)
		}
		taskPicker, err = NewColocatedCappedRandomTaskPicker(maxShardedTasksInProgress, maxColocatedBatchesInProgress, pendingTasks, state, yb, colocatedBatchImportQueue, tableTypes)
		if err != nil {
			return fmt.Errorf("create colocated aware randmo task picker: %w", err)
		}
	} else {
		taskPicker, err = NewSequentialTaskPicker(pendingTasks, state)
		if err != nil {
			return fmt.Errorf("create sequential task picker: %w", err)
		}
	}

	for taskPicker.HasMoreTasks() {
		task, err := taskPicker.Pick()
		if err != nil {
			return fmt.Errorf("get next task: %w", err)
		}
		log.Infof("Picked task for import: %s", task)
		var taskImporter *FileTaskImporter
		var ok bool
		taskImporter, ok = taskImporters[task.ID]
		if !ok {
			taskImporter, err = createFileTaskImporter(task, state, batchImportPool, progressReporter, colocatedBatchImportQueue, tableTypes, errorHandler)
			if err != nil {
				return fmt.Errorf("create file task importer: %w", err)
			}
			log.Infof("created file task importer for table: %s, task: %v", task.TableNameTup.ForOutput(), task)
			taskImporters[task.ID] = taskImporter
		}

		if taskImporter.AllBatchesSubmitted() {
			// All batches for this task have been submitted.
			// task could have been completed (all batches imported) OR still in progress
			// in case task is done, we should inform task picker so that we stop picking that task.
			log.Infof("All batches submitted for task: %s", task)
			taskDone, err := state.AllBatchesImported(task.FilePath, task.TableNameTup)
			if err != nil {
				return fmt.Errorf("check if all batches are imported: task: %v err :%w", task, err)
			}
			if taskDone {
				taskImporter.PostProcess()
				err = taskPicker.MarkTaskAsDone(task)
				if err != nil {
					return fmt.Errorf("mark task as done: task: %v, err: %w", task, err)
				}
				state.UnregisterFileTaskImporter(taskImporter)
				log.Infof("Import of task done: %s", task)
				continue
			} else {
				// some batches are still in progress, wait for them to complete as decided by the picker.
				// don't want to busy-wait, so in case of sequentialTaskPicker, we sleep.
				err := taskPicker.WaitForTasksBatchesTobeImported()
				if err != nil {
					return fmt.Errorf("wait for tasks batches to be imported: %w", err)
				}
				continue
			}

		}
		err = taskImporter.ProduceAndSubmitNextBatchToWorkerPool()
		if err != nil {
			return fmt.Errorf("submit next batch: task:%v err: %s", task, err)
		}
	}
	return nil
}

func setupWorkerPoolAndQueue(maxParallelConns int, maxColocatedBatchesInProgress int) {
	shardedPoolSize := maxParallelConns * 2
	batchImportPool = pool.New().WithMaxGoroutines(shardedPoolSize)
	log.Infof("created batch import pool of size: %d", shardedPoolSize)

	if importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE {
		colocatedBatchImportPool = pool.New().WithMaxGoroutines(maxColocatedBatchesInProgress)
		log.Infof("created colocated batch import pool of size: %d", maxColocatedBatchesInProgress)

		colocatedBatchImportQueue = make(chan func(), maxColocatedBatchesInProgress*2)

		colocatedBatchImportQueueConsumer := func() {
			// just read from channel and submit to the worker pool.
			// worker pool has a max size of maxColocatedBatchesInProgress, so it will block if all workers are busy.
			for {
				select {
				case f := <-colocatedBatchImportQueue:
					colocatedBatchImportPool.Go(f)
				}
			}
		}
		go colocatedBatchImportQueueConsumer()
	}
}

// getTableTypes returns a map of table name to table type (sharded/colocated) for all tables in the tasks.
func getTableTypes(tasks []*ImportFileTask) (*utils.StructMap[sqlname.NameTuple, string], error) {
	if !slices.Contains([]string{TARGET_DB_IMPORTER_ROLE, IMPORT_FILE_ROLE}, importerRole) {
		return nil, nil
	}

	tableTypes := utils.NewStructMap[sqlname.NameTuple, string]()
	yb, ok := tdb.(YbTargetDBColocatedChecker)
	if !ok {
		return nil, fmt.Errorf("expected tdb to be of type TargetYugabyteDB, got: %T", tdb)
	}
	isDBColocated, err := yb.IsDBColocated()
	if err != nil {
		return nil, fmt.Errorf("checking if db is colocated: %w", err)
	}
	for _, task := range tasks {
		if tableType, ok := tableTypes.Get(task.TableNameTup); !ok {
			if !isDBColocated {
				tableType = SHARDED
			} else {
				isColocated, err := yb.IsTableColocated(task.TableNameTup)
				if err != nil {
					return nil, fmt.Errorf("checking if table is colocated: table: %v: %w", task.TableNameTup.ForOutput(), err)
				}
				tableType = lo.Ternary(isColocated, COLOCATED, SHARDED)
			}
			tableTypes.Put(task.TableNameTup, tableType)
		}
	}
	return tableTypes, nil

}

/*
when TARGET_DB_IMPORTER_ROLE or IMPORT_FILE_ROLE, we pass on
the batchImportPool and the colocatedBatchImportQueue to the FileTaskImporter
so that it can submit sharded table batches to the batchImportPool,
and colocated table batches to the colocatedBatchImportQueue.

Otherwise, we simply pass the batchImportPool to the FileTaskImporter.
*/
func createFileTaskImporter(task *ImportFileTask, state *ImportDataState, batchImportPool *pool.Pool, progressReporter *ImportDataProgressReporter, colocatedBatchImportQueue chan func(),
	tableTypes *utils.StructMap[sqlname.NameTuple, string], errorHandler importdata.ImportDataErrorHandler) (*FileTaskImporter, error) {
	var taskImporter *FileTaskImporter
	var err error
	if importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE {
		tableType, ok := tableTypes.Get(task.TableNameTup)
		if !ok {
			return nil, fmt.Errorf("table type not found for table: %s", task.TableNameTup.ForOutput())
		}

		taskImporter, err = NewFileTaskImporter(task, state, batchImportPool, progressReporter, colocatedBatchImportQueue, tableType == COLOCATED, errorHandler)
		if err != nil {
			return nil, fmt.Errorf("create file task importer: %w", err)
		}
	} else {
		taskImporter, err = NewFileTaskImporter(task, state, batchImportPool, progressReporter, nil, false, errorHandler)
		if err != nil {
			return nil, fmt.Errorf("create file task importer: %w", err)
		}
	}
	return taskImporter, nil
}

func startMonitoringTargetYBHealth() error {
	if !slices.Contains([]string{TARGET_DB_IMPORTER_ROLE, IMPORT_FILE_ROLE}, importerRole) {
		return nil
	}
	if skipNodeHealthChecks && skipDiskUsageHealthChecks && skipReplicationChecks {
		return nil
	}
	yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return fmt.Errorf("monitoring health is only supported if target DB is YugabyteDB")
	}
	go func() {
		//for now not sending any other parameters as not required for monitor usage
		ybClient := dbzm.NewYugabyteDBCDCClient(exportDir, "", tconf.SSLRootCert, tconf.DBName, "", nil)
		err := ybClient.Init()
		if err != nil {
			log.Errorf("error intialising the yb client : %v", err)
		}
		monitorTDBHealth := monitor.NewMonitorTargetYBHealth(yb, bool(skipDiskUsageHealthChecks), bool(skipReplicationChecks), bool(skipNodeHealthChecks), ybClient, func(info string) {
			displayMonitoringInformationOnTheConsole(info)
		})
		err = monitorTDBHealth.StartMonitoring()
		if err != nil {
			log.Errorf("error monitoring the target health: %v", err)
		}
	}()
	return nil
}

func displayMonitoringInformationOnTheConsole(info string) {
	if info == "" {
		return
	}
	if disablePb {
		utils.PrintAndLog(info)
	} else {
		log.Warnf("monitoring: %v", info)
		if importPhase == dbzm.MODE_SNAPSHOT || importerRole == IMPORT_FILE_ROLE {
			progressReporter.DisplayInformation(info)
		} else {
			statsReporter.DisplayInformation(info)

		}
	}
}

func startAdaptiveParallelism() (bool, error) {
	yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return false, fmt.Errorf("adaptive parallelism is only supported if target DB is YugabyteDB")
	}
	if !yb.IsAdaptiveParallelismSupported() {
		utils.PrintAndLog(color.YellowString("Note: Continuing without adaptive parallelism as it is not supported in this version of YugabyteDB."))
		return false, nil
	}

	go func() {
		err := adaptiveparallelism.AdaptParallelism(yb)
		if err != nil {
			log.Errorf("adaptive parallelism error: %v", err)
		}
	}()
	return true, nil
}

func waitForDebeziumStartIfRequired() error {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("failed to get migration status record: %w", err)
	}
	if msr.SnapshotMechanism == "debezium" {
		// we already wait for snapshot to have completed by debezium
		// so no need to wait again here.
		return nil
	}

	// in case pg_dump was used to export snapshot data,
	// we need to wait for export-data to have started debezium in cdc phase
	// in order to avoid any race conditions.
	fmt.Println("Initializing streaming phase...")
	log.Infof("waiting for export-data to have started debezium in cdc phase")
	for {
		msr, err = metaDB.GetMigrationStatusRecord()
		if err != nil {
			return fmt.Errorf("failed to get migration status record: %w", err)
		}
		if lo.Contains([]string{TARGET_DB_IMPORTER_ROLE, SOURCE_REPLICA_DB_IMPORTER_ROLE}, importerRole) &&
			msr.ExportDataSourceDebeziumStarted {
			break
		}
		if importerRole == SOURCE_DB_IMPORTER_ROLE && msr.ExportDataTargetDebeziumStarted {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return nil
}

func packAndSendImportDataToTargetPayload(status string, errorMsg error) {

	if !shouldSendCallhome() {
		return
	}
	//basic payload details
	payload := createCallhomePayload()
	switch importType {
	case SNAPSHOT_ONLY:
		payload.MigrationType = OFFLINE
	case SNAPSHOT_AND_CHANGES:
		payload.MigrationType = LIVE_MIGRATION
	}
	payload.TargetDBDetails = callhome.MarshalledJsonString(targetDBDetails)
	payload.MigrationPhase = IMPORT_DATA_PHASE
	importDataPayload := callhome.ImportDataPhasePayload{
		PayloadVersion:              callhome.IMPORT_DATA_CALLHOME_PAYLOAD_VERSION,
		ParallelJobs:                int64(tconf.Parallelism),
		StartClean:                  bool(startClean),
		EnableUpsert:                bool(tconf.EnableUpsert),
		Error:                       callhome.SanitizeErrorMsg(errorMsg),
		ControlPlaneType:            getControlPlaneType(),
		BatchSize:                   batchSizeInNumRows,
		OnPrimaryKeyConflictAction:  tconf.OnPrimaryKeyConflictAction,
		EnableYBAdaptiveParallelism: bool(tconf.EnableYBAdaptiveParallelism),
		AdaptiveParallelismMax:      int64(tconf.MaxParallelism),
		ErrorPolicySnapshot:         errorPolicySnapshotFlag.String(),
	}

	importDataPayload.YBClusterMetrics = BuildCallhomeYBClusterMetrics()

	//Getting the imported snapshot details
	importRowsMap, err := getImportedSnapshotRowsMap("target")
	if err != nil {
		log.Infof("callhome: error in getting the import data: %v", err)
	} else {
		importRowsMap.IterKV(func(key sqlname.NameTuple, value RowCountPair) (bool, error) {
			importDataPayload.TotalRows += value.Imported
			if value.Imported > importDataPayload.LargestTableRows {
				importDataPayload.LargestTableRows = value.Imported
			}
			return true, nil
		})
	}

	importDataPayload.Phase = importPhase

	if importPhase != dbzm.MODE_SNAPSHOT && statsReporter != nil {
		importDataPayload.EventsImportRate = statsReporter.EventsImportRateLast3Min
		importDataPayload.TotalImportedEvents = statsReporter.TotalEventsImported
	}

	payload.PhasePayload = callhome.MarshalledJsonString(importDataPayload)
	payload.Status = status

	err = callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func fetchAndStoreGeneratedAlwaysIdentityColumnsInMetadb(tables []sqlname.NameTuple) error {
	tableKeyToIdentityColumnNames := make(map[string][]string)

	//Fetching the table to identity columns information from metadb if present
	found, err := metaDB.GetJsonObject(nil, identityColumnsMetaDBKey, &tableKeyToIdentityColumnNames)
	if err != nil {
		return fmt.Errorf("failed to get identity columns from meta db: %s", err)
	}
	if found {
		//IF present in metadb retrieve a map of table NameTuple Key -> columns
		//convert it to the struct map of NameTuple -> columns in global variable TableToIdentityColumns
		TableToIdentityColumnNames = utils.NewStructMap[sqlname.NameTuple, []string]()
		for key, columns := range tableKeyToIdentityColumnNames {
			nameTuple, err := namereg.NameReg.LookupTableName(key)
			if err != nil {
				return fmt.Errorf("lookup for table name in name reg: %v with: %v", key, err)
			}
			TableToIdentityColumnNames.Put(nameTuple, columns)
		}
		return nil
	}
	//If not found Fetch it from db in struct map of NameTuple -> columns
	TableToIdentityColumnNames = getIdentityColumnsForTables(tables, "ALWAYS")
	//also store it as map of table NameTuple key -> columns and then store it in metadb
	TableToIdentityColumnNames.IterKV(func(key sqlname.NameTuple, value []string) (bool, error) {
		tableKeyToIdentityColumnNames[key.ForKey()] = value
		return true, nil
	})
	// saving in metadb for handling restarts
	err = metaDB.InsertJsonObject(nil, identityColumnsMetaDBKey, tableKeyToIdentityColumnNames)
	if err != nil {
		return fmt.Errorf("failed to insert into the key '%s': %v", identityColumnsMetaDBKey, err)
	}
	return nil
}
func disableGeneratedAlwaysAsIdentityColumns() error {
	err := tdb.DisableGeneratedAlwaysAsIdentityColumns(TableToIdentityColumnNames)
	if err != nil {
		return fmt.Errorf("failed to disable generated always as identity columns: %s", err)
	}
	return nil
}

func enableGeneratedAlwaysAsIdentityColumns() error {
	err := tdb.EnableGeneratedAlwaysAsIdentityColumns(TableToIdentityColumnNames)
	if err != nil {
		return fmt.Errorf("failed to enable generated always as identity columns: %s", err)
	}
	return nil
}

func restoreGeneratedByDefaultAsIdentityColumns(tables []sqlname.NameTuple) error {
	log.Infof("restoring generated by default as identity columns for tables: %v", tables)
	tablesToIdentityColumnNames := getIdentityColumnsForTables(tables, "BY DEFAULT")
	err := tdb.EnableGeneratedByDefaultAsIdentityColumns(tablesToIdentityColumnNames)
	if err != nil {
		return fmt.Errorf("failed to enable generated by default as identity columns: %s", err)
	}
	return nil
}

func getIdentityColumnsForTables(tables []sqlname.NameTuple, identityType string) *utils.StructMap[sqlname.NameTuple, []string] {
	var result = utils.NewStructMap[sqlname.NameTuple, []string]()
	log.Infof("getting identity(%s) columns for tables: %v", identityType, tables)
	for _, table := range tables {
		identityColumns, err := tdb.GetIdentityColumnNamesForTable(table, identityType)
		if err != nil {
			utils.ErrExit("error in getting identity(%s) columns for table: %s: %w", identityType, table, err)
		}
		if len(identityColumns) > 0 {
			log.Infof("identity(%s) columns for table %s: %v", identityType, table, identityColumns)
			result.Put(table, identityColumns)
		}
	}
	return result
}

func importFileTasksToTableNames(tasks []*ImportFileTask) []string {
	tableNames := []string{}
	for _, t := range tasks {
		tableNames = append(tableNames, t.TableNameTup.ForKey())
	}
	return lo.Uniq(tableNames)
}

func importFileTasksToTableNameTuples(tasks []*ImportFileTask) []sqlname.NameTuple {
	tableNames := []sqlname.NameTuple{}
	for _, t := range tasks {
		tableNames = append(tableNames, t.TableNameTup)
	}
	return lo.UniqBy(tableNames, func(t sqlname.NameTuple) string {
		return t.ForKey()
	})
}

func classifyTasksForImport(state *ImportDataState, tasks []*ImportFileTask) (pendingTasks, completedTasks []*ImportFileTask, err error) {
	inProgressTasks := []*ImportFileTask{}
	notStartedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			return nil, nil, fmt.Errorf("get table import state: %w", err)
		}
		switch fileImportState {
		case FILE_IMPORT_COMPLETED, FILE_IMPORT_COMPLETED_WITH_ERRORS:
			completedTasks = append(completedTasks, task)
		case FILE_IMPORT_IN_PROGRESS:
			inProgressTasks = append(inProgressTasks, task)
		case FILE_IMPORT_NOT_STARTED:
			notStartedTasks = append(notStartedTasks, task)
		default:
			return nil, nil, fmt.Errorf("invalid table import state: %s", fileImportState)
		}
	}
	// Start with in-progress tasks, followed by not-started tasks.
	return append(inProgressTasks, notStartedTasks...), completedTasks, nil
}

func getNotStartedTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	notStartedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("get table import state: %s: %s", task.TableNameTup, err)
		}
		if fileImportState == FILE_IMPORT_NOT_STARTED {
			notStartedTasks = append(notStartedTasks, task)
		}
	}
	return notStartedTasks
}

func getInProgressTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	inProgressTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("get table import state: %s: %s", task.TableNameTup, err)
		}
		if fileImportState == FILE_IMPORT_IN_PROGRESS {
			inProgressTasks = append(inProgressTasks, task)
		}
	}
	return inProgressTasks
}

func getPendingTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	return append(getInProgressTasks(state, tasks), getNotStartedTasks(state, tasks)...)
}

func getCompletedTasks(state *ImportDataState, tasks []*ImportFileTask) []*ImportFileTask {
	completedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("get table import state: %s: %s", task.TableNameTup, err)
		}
		if fileImportState == FILE_IMPORT_COMPLETED || fileImportState == FILE_IMPORT_COMPLETED_WITH_ERRORS {
			completedTasks = append(completedTasks, task)
		}
	}
	return completedTasks
}

func cleanImportState(state *ImportDataState, tasks []*ImportFileTask) {
	tableNames := importFileTasksToTableNameTuples(tasks)
	nonEmptyNts := tdb.GetNonEmptyTables(tableNames)
	if len(nonEmptyNts) > 0 {
		nonEmptyTableNames := lo.Map(nonEmptyNts, func(nt sqlname.NameTuple, _ int) string {
			return nt.ForOutput()
		})
		if truncateTables {
			// truncate tables only supported for import-data-to-target.
			utils.PrintAndLog("Truncating non-empty tables on DB: %v", nonEmptyTableNames)
			err := tdb.TruncateTables(nonEmptyNts)
			if err != nil {
				utils.ErrExit("failed to truncate tables: %s", err)
			}
		} else {
			utils.PrintAndLog("Non-Empty tables: [%s]", strings.Join(nonEmptyTableNames, ", "))
			utils.PrintAndLog("The above list of tables on DB are not empty.")
			utils.PrintAndLog("If you wish to truncate them, re-run the import command with --truncate-tables true")
			yes := utils.AskPrompt("Do you want to start afresh without truncating tables")
			if !yes {
				utils.ErrExit("Aborting import.")
			}
		}

	}

	for _, task := range tasks {
		err := state.Clean(task.FilePath, task.TableNameTup)
		if err != nil {
			utils.ErrExit("failed to clean import data state for table: %q: %s", task.TableNameTup, err)
		}
	}

	sqlldrDir := filepath.Join(exportDir, "sqlldr")
	if utils.FileOrFolderExists(sqlldrDir) {
		err := os.RemoveAll(sqlldrDir)
		if err != nil {
			utils.ErrExit("failed to remove sqlldr directory: %q: %s", sqlldrDir, err)
		}
	}

	if changeStreamingIsEnabled(importType) {
		// clearing state from metaDB based on importerRole
		err := metaDB.ResetQueueSegmentMeta(importerRole)
		if err != nil {
			utils.ErrExit("failed to reset queue segment meta: %s", err)
		}
		err = metaDB.DeleteJsonObject(identityColumnsMetaDBKey)
		if err != nil {
			utils.ErrExit("failed to reset identity columns meta: %s", err)
		}
	}
}

func executePostSnapshotImportSqls() error {
	sequenceFilePath := filepath.Join(exportDir, "data", "postdata.sql")
	if utils.FileOrFolderExists(sequenceFilePath) {
		fmt.Printf("setting resume value for sequences %10s\n", "")
		err := executeSqlFile(sequenceFilePath, "SEQUENCE", func(_, _ string) bool { return false })
		if err != nil {
			return err
		}
	}
	return nil
}

func getIndexName(sqlQuery string, indexName string) (string, error) {
	// Return the index name itself if it is aleady qualified with schema name
	if len(strings.Split(indexName, ".")) == 2 {
		return indexName, nil
	}

	parts := strings.FieldsFunc(sqlQuery, func(c rune) bool { return unicode.IsSpace(c) || c == '(' || c == ')' })
	for index, part := range parts {
		if strings.EqualFold(part, "ON") {
			tableName := parts[index+1]
			schemaName := getTargetSchemaName(tableName)
			return fmt.Sprintf("%s.%s", schemaName, indexName), nil
		}
	}
	return "", fmt.Errorf("could not find `ON` keyword in the CREATE INDEX statement")
}

// TODO: This function is a duplicate of the one in tgtdb/yb.go. Consolidate the two.
func getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	if tconf.TargetDBType == POSTGRESQL {
		defaultSchema, noDefaultSchema := GetDefaultPGSchema(tconf.Schema, ",")
		if noDefaultSchema {
			utils.ErrExit("no default schema for table: %q ", tableName)
		}
		return defaultSchema
	}
	return tconf.Schema // default set to "public"
}

func prepareTableToColumns(tasks []*ImportFileTask) {
	for _, task := range tasks {
		var columns []string
		dfdTableToExportedColumns := getDfdTableNameToExportedColumns(dataFileDescriptor)
		if dfdTableToExportedColumns != nil {
			columns, _ = dfdTableToExportedColumns.Get(task.TableNameTup)
		} else if dataFileDescriptor.HasHeader {
			// File is either exported from debezium OR this is `import data file` case.
			reader, err := dataStore.Open(task.FilePath)
			if err != nil {
				utils.ErrExit("datastore.Open: %q: %v", task.FilePath, err)
			}
			df, err := datafile.NewDataFile(task.FilePath, reader, dataFileDescriptor)
			if err != nil {
				utils.ErrExit("opening datafile: %q: %v", task.FilePath, err)
			}
			header := df.GetHeader()
			columns = strings.Split(header, dataFileDescriptor.Delimiter)
			log.Infof("read header from file %q: %s", task.FilePath, header)
			log.Infof("header row split using delimiter %q: %v\n", dataFileDescriptor.Delimiter, columns)
			df.Close()
		}
		TableToColumnNames.Put(task.TableNameTup, columns)
	}
}

func getDfdTableNameToExportedColumns(dataFileDescriptor *datafile.Descriptor) *utils.StructMap[sqlname.NameTuple, []string] {
	if dataFileDescriptor.TableNameToExportedColumns == nil {
		return nil
	}

	result := utils.NewStructMap[sqlname.NameTuple, []string]()
	for tableNameRaw, columnList := range dataFileDescriptor.TableNameToExportedColumns {
		nt, err := namereg.NameReg.LookupTableName(tableNameRaw)
		if err != nil {
			utils.ErrExit("lookup table in name registry: %q: %v", tableNameRaw, err)
		}
		result.Put(nt, columnList)
	}
	return result
}

func checkExportDataDoneFlag() {
	metaInfoDir := filepath.Join(exportDir, metaInfoDirName)
	_, err := os.Stat(metaInfoDir)
	if err != nil {
		utils.ErrExit("metainfo dir is missing. Exiting.")
	}

	if dataIsExported() {
		return
	}

	utils.PrintAndLog("Waiting for snapshot data export to complete...")
	for !dataIsExported() {
		time.Sleep(time.Second * 2)
	}
	utils.PrintAndLog("Snapshot data export is complete.")
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
	importDataCmd.Flags().MarkHidden("continue-on-error")
	importDataToTargetCmd.Flags().MarkHidden("continue-on-error")
	registerTargetDBConnFlags(importDataCmd)
	registerTargetDBConnFlags(importDataToTargetCmd)
	registerImportDataCommonFlags(importDataCmd)
	registerImportDataCommonFlags(importDataToTargetCmd)
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

func createInitialImportDataTableMetrics(tasks []*ImportFileTask) []*cp.UpdateImportedRowCountEvent {
	result := []*cp.UpdateImportedRowCountEvent{}
	for _, task := range tasks {
		var schemaName, tableName string
		schemaName, tableName = cp.SplitTableNameForPG(task.TableNameTup.ForKey())
		tableMetrics := cp.UpdateImportedRowCountEvent{
			BaseUpdateRowCountEvent: cp.BaseUpdateRowCountEvent{
				BaseEvent: cp.BaseEvent{
					EventType:     "IMPORT DATA",
					MigrationUUID: migrationUUID,
					SchemaNames:   []string{schemaName},
				},
				TableName:         tableName,
				Status:            cp.EXPORT_OR_IMPORT_DATA_STATUS_INT_TO_STR[ROW_UPDATE_STATUS_NOT_STARTED],
				TotalRowCount:     getTotalProgressAmount(task),
				CompletedRowCount: 0,
			},
		}
		result = append(result, &tableMetrics)
	}

	return result
}

func saveOnPrimaryKeyConflictActionInMSR() {
	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.OnPrimaryKeyConflictAction = tconf.OnPrimaryKeyConflictAction
	})
}

func cleanMSRForImportDataStartClean() error {
	if !startClean {
		log.Infof("skipping cleaning migration status record for import data command start clean")
		return nil
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("failed to get migration status record: %s", err)
	}

	if msr == nil {
		return fmt.Errorf("migration status record not found.")
	} else {
		metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			msr.OnPrimaryKeyConflictAction = ""
		})
	}
	return nil
}

func cleanStoredErrors(errorHandler importdata.ImportDataErrorHandler, tasks []*ImportFileTask) error {
	// clean stored errors for all tasks
	for _, task := range tasks {
		err := errorHandler.CleanUpStoredErrors(task.TableNameTup, task.FilePath)
		if err != nil {
			return fmt.Errorf("failed to clean up stored errors for task %s: %w", task.TableNameTup.ForOutput(), err)
		}
	}
	return nil
}

func isTargetDBImporter(importerRole string) bool {
	return importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE
}

func BuildCallhomeYBClusterMetrics() callhome.YBClusterMetrics {
	yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		utils.ErrExit("importData: expected tdb to be of type TargetYugabyteDB, got: %T", tdb)
	}

	clusterMetrics, err := yb.GetClusterMetrics()
	if err != nil {
		log.Errorf("callhome: error in getting cluster metrics: %v", err)
		return callhome.YBClusterMetrics{}
	}

	now := time.Now().UTC()
	nodes := make([]callhome.NodeMetric, 0)
	var totalCpuPct, totalMemPct float64
	for _, nodeMetrics := range clusterMetrics {
		if nodeMetrics.Status != "OK" {
			log.Warnf("callhome: node %s has error: %s", nodeMetrics.UUID, nodeMetrics.Error)
		}

		cpuPct, err := nodeMetrics.CPUPercent()
		if err != nil {
			log.Warnf("callhome: error getting CPU percent for node %s: %v", nodeMetrics.UUID, err)
		}
		memPct, err := nodeMetrics.MemPercent()
		if err != nil {
			log.Warnf("callhome: error getting Mem percent for node %s: %v", nodeMetrics.UUID, err)
		}

		nodes = append(nodes, callhome.NodeMetric{
			UUID:   nodeMetrics.UUID,
			CPUPct: cpuPct,
			MemPct: memPct,
			Status: nodeMetrics.Status,
			Error:  nodeMetrics.Error,
		})

		totalCpuPct += cpuPct
		totalMemPct += memPct
	}

	if len(nodes) == 0 {
		log.Error("callhome: no nodes found in cluster metrics")
		return callhome.YBClusterMetrics{}
	}

	avgCpuPct := totalCpuPct / float64(len(nodes))
	avgMemPct := totalMemPct / float64(len(nodes))
	return callhome.YBClusterMetrics{
		Timestamp: now,
		CPUAvgPct: avgCpuPct,
		MemAvgPct: avgMemPct,
		Nodes:     nodes,
	}
}
