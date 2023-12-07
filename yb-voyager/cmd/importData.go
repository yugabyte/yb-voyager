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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/jackc/pgx/v4"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

var metaInfoDirName = META_INFO_DIR_NAME
var batchSize = int64(0)
var batchImportPool *pool.Pool
var tablesProgressMetadata map[string]*utils.TableProgressMetadata
var importerRole string
var identityColumnsMetaDBKey string

// stores the data files description in a struct
var dataFileDescriptor *datafile.Descriptor
var truncateSplits utils.BoolStr                           // to truncate *.D splits after import
var TableToColumnNames = make(map[string][]string)         // map of table name to columnNames
var TableToIdentityColumnNames = make(map[string][]string) // map of table name to generated always as identity column's names
var valueConverter dbzm.ValueConverter
var TableNameToSchema map[string]map[string]map[string]string

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
		err := validateImportFlags(cmd, importerRole)
		if err != nil {
			utils.ErrExit("Error: %s", err.Error())
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
	triggerName, err := getTriggerName(importerRole)
	if err != nil {
		utils.ErrExit("failed to get trigger name for checking if DB is switched over: %v", err)
	}
	exitIfDBSwitchedOver(triggerName)
	reportProgressInBytes = false
	tconf.ImportMode = true
	checkExportDataDoneFlag()
	sourceDBType = GetSourceDBTypeFromMSR()
	sqlname.SourceDBType = sourceDBType
	dataStore = datastore.NewDataStore(filepath.Join(exportDir, "data"))
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	// TODO: handle case-sensitive in table names with oracle ff-db
	// quoteTableNameIfRequired()
	importFileTasks := discoverFilesToImport()
	record, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %s", err)
	}
	if importerRole == TARGET_DB_IMPORTER_ROLE {

		importType = record.ExportType
		identityColumnsMetaDBKey = metadb.TARGET_DB_IDENTITY_COLUMNS_KEY
	}

	if importerRole == FF_DB_IMPORTER_ROLE {
		if record.FallbackEnabled {
			utils.ErrExit("cannot import data to source-replica. Fall-back workflow is already enabled.")
		}
		updateFallForwarDBExistsInMetaDB()
		identityColumnsMetaDBKey = metadb.FF_DB_IDENTITY_COLUMNS_KEY
	}

	if changeStreamingIsEnabled(importType) && (tconf.TableList != "" || tconf.ExcludeTableList != "") {
		utils.ErrExit("--table-list and --exclude-table-list are not supported for live migration. Re-run the command without these flags.")
	} else {
		importFileTasks = applyTableListFilter(importFileTasks)
	}

	importData(importFileTasks)
	if changeStreamingIsEnabled(importType) {
		startFallforwardSynchronizeIfRequired()
	}
}

func startFallforwardSynchronizeIfRequired() {
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
	var unqualifiedTableList []string
	for _, qualifiedTableName := range tableListExportedFromSource {
		// TODO: handle case sensitivity?
		unqualifiedTableName := sqlname.NewSourceNameFromQualifiedName(qualifiedTableName).ObjectName.Unquoted
		unqualifiedTableList = append(unqualifiedTableList, unqualifiedTableName)
	}

	lockFile.Unlock() // unlock export dir from import data cmd before switching current process to ff/fb sync cmd
	cmd := []string{"yb-voyager", "export", "data", "from", "target",
		"--export-dir", exportDir,
		"--table-list", strings.Join(unqualifiedTableList, ","),
		fmt.Sprintf("--send-diagnostics=%t", callhome.SendDiagnostics),
	}
	if utils.DoNotPrompt {
		cmd = append(cmd, "--yes")
	}
	if disablePb {
		cmd = append(cmd, "--disable-pb=true")
	}
	cmdStr := "TARGET_DB_PASSWORD=*** " + strings.Join(cmd, " ")

	utils.PrintAndLog("Starting export data from target with command:\n %s", color.GreenString(cmdStr))
	binary, lookErr := exec.LookPath(os.Args[0])
	if lookErr != nil {
		utils.ErrExit("could not find yb-voyager - %w", err)
	}
	env := os.Environ()
	env = append(env, fmt.Sprintf("TARGET_DB_PASSWORD=%s", tconf.Password))
	execErr := syscall.Exec(binary, cmd, env)
	if execErr != nil {
		utils.ErrExit("failed to run yb-voyager export data from target - %w\n Please re-run with command :\n%s", err, cmdStr)
	}
}

type ImportFileTask struct {
	ID        int
	FilePath  string
	TableName string
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
		task := &ImportFileTask{
			ID:        i,
			FilePath:  fileEntry.FilePath,
			TableName: fileEntry.TableName,
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
	defaultSourceSchema, noDefaultSchema := getDefaultSourceSchemaName()

	//TODO: handle with case sensitivity later
	standardizeCaseInsensitiveTableNames := func(tableName string, defaultSourceSchema string) string {
		parts := strings.Split(tableName, ".")
		tableName = parts[len(parts)-1]
		if !utils.IsQuotedString(tableName) {
			tableName = strings.ToLower(tableName)
		}
		if len(parts) > 1 {
			if parts[0] == defaultSourceSchema {
				return tableName
			}
			return fmt.Sprintf(`%s.%s`, parts[0], tableName)
		}
		return tableName
	}

	allTables := lo.Map(importFileTasks, func(task *ImportFileTask, _ int) string {
		return standardizeCaseInsensitiveTableNames(task.TableName, defaultSourceSchema)
	})
	slices.Sort(allTables)
	log.Infof("allTables: %v", allTables)

	findPatternMatchingTables := func(pattern string) []string {
		result := lo.Filter(allTables, func(tableName string, _ int) bool {
			matched, err := filepath.Match(pattern, tableName)
			if err != nil {
				utils.ErrExit("Invalid table name pattern %q: %s", pattern, err)
			}
			return matched
		})
		return result
	}

	extractTableList := func(flagTableList, listName string) []string {
		tableList := utils.CsvStringToSlice(flagTableList)
		var result []string
		var unqualifiedTables []string
		for _, table := range tableList {
			if noDefaultSchema && len(strings.Split(table, ".")) == 1 {
				unqualifiedTables = append(unqualifiedTables, table)
				continue
			}
			table = standardizeCaseInsensitiveTableNames(table, defaultSourceSchema)
			matchingTables := findPatternMatchingTables(table)
			if len(matchingTables) == 0 {
				result = append(result, table) //so that unknown check can be done later
			} else {
				result = append(result, matchingTables...)
			}
		}
		if len(unqualifiedTables) > 0 {
			utils.ErrExit("Qualify following table names %v in the %s list with schema-name.", unqualifiedTables, listName)
		}
		log.Infof("%s tableList: %v", listName, result)
		return result
	}

	includeList := extractTableList(tconf.TableList, "include")
	excludeList := extractTableList(tconf.ExcludeTableList, "exclude")

	checkUnknownTableNames := func(tableNames []string, listName string) {
		unknownTableNames := make([]string, 0)
		for _, tableName := range tableNames {
			if !slices.Contains(allTables, tableName) {
				unknownTableNames = append(unknownTableNames, tableName)
			}
		}
		if len(unknownTableNames) > 0 {
			utils.PrintAndLog("Unknown table names in the %s list: %v", listName, unknownTableNames)
			utils.PrintAndLog("Valid table names are: %v", allTables)
			utils.ErrExit("Please fix the table names in the %s list and retry.", listName)
		}
	}
	checkUnknownTableNames(includeList, "include")
	checkUnknownTableNames(excludeList, "exclude")

	for _, task := range importFileTasks {
		table := standardizeCaseInsensitiveTableNames(task.TableName, defaultSourceSchema)
		if len(includeList) > 0 && !slices.Contains(includeList, table) {
			log.Infof("Skipping table %q (fileName: %s) as it is not in the include list", task.TableName, task.FilePath)
			continue
		}
		if len(excludeList) > 0 && slices.Contains(excludeList, table) {
			log.Infof("Skipping table %q (fileName: %s) as it is in the exclude list", task.TableName, task.FilePath)
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
		case FF_DB_IMPORTER_ROLE:
			record.FallForwardDBConf = tconf.Clone()
			record.FallForwardDBConf.Password = ""
			record.FallForwardDBConf.Uri = ""
		case FB_DB_IMPORTER_ROLE:
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

func importData(importFileTasks []*ImportFileTask) {
	if tconf.TargetDBType == YUGABYTEDB {
		tconf.Schema = strings.ToLower(tconf.Schema)
	} else if tconf.TargetDBType == ORACLE && !utils.IsQuotedString(tconf.Schema) {
		tconf.Schema = strings.ToUpper(tconf.Schema)
	}
	err := retrieveMigrationUUID()
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}
	payload := callhome.GetPayload(exportDir, migrationUUID)
	updateTargetConfInMigrationStatus()
	tdb = tgtdb.NewTargetDB(&tconf)
	err = tdb.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB: %s", err)
	}
	defer tdb.Finalize()

	valueConverter, err = dbzm.NewValueConverter(exportDir, tdb, tconf, importerRole)
	TableNameToSchema = valueConverter.GetTableNameToSchema()
	if err != nil {
		utils.ErrExit("Failed to create value converter: %s", err)
	}
	err = tdb.InitConnPool()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB connection pool: %s", err)
	}
	utils.PrintAndLog("Using %d parallel jobs.", tconf.Parallelism)

	targetDBVersion := tdb.GetVersion()

	fmt.Printf("%s version: %s\n", tconf.TargetDBType, targetDBVersion)

	payload.TargetDBVersion = targetDBVersion
	//payload.NodeCount = len(tconfs) // TODO: Figure out way to populate NodeCount.

	err = tdb.CreateVoyagerSchema()
	if err != nil {
		utils.ErrExit("Failed to create voyager metadata schema on target DB: %s", err)
	}

	utils.PrintAndLog("\nimport of data in %q database started", tconf.DBName)
	var pendingTasks, completedTasks []*ImportFileTask
	state := NewImportDataState(exportDir)
	if startClean {
		cleanImportState(state, importFileTasks)
		pendingTasks = importFileTasks
	} else {
		pendingTasks, completedTasks, err = classifyTasks(state, importFileTasks)
		if err != nil {
			utils.ErrExit("Failed to classify tasks: %s", err)
		}
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %s", err)
	}
	sourceTableList := msr.TableListExportedFromSource
	if msr.SourceDBConf != nil {
		source = *msr.SourceDBConf
	}
	importTableList := getImportTableList(sourceTableList)

	disableGeneratedAlwaysAsIdentityColumns(importTableList)
	// restore value for IDENTITY BY DEFAULT columns once IDENTITY ALWAYS columns are enabled back
	defer restoreGeneratedByDefaultAsIdentityColumns(importTableList)
	defer enableGeneratedAlwaysAsIdentityColumns()

	// Import snapshots
	if importerRole != FB_DB_IMPORTER_ROLE {
		utils.PrintAndLog("Already imported tables: %v", importFileTasksToTableNames(completedTasks))
		if len(pendingTasks) == 0 {
			utils.PrintAndLog("All the tables are already imported, nothing left to import\n")
		} else {
			utils.PrintAndLog("Tables to import: %v", importFileTasksToTableNames(pendingTasks))
			prepareTableToColumns(pendingTasks) //prepare the tableToColumns map
			poolSize := tconf.Parallelism * 2
			progressReporter := NewImportDataProgressReporter(bool(disablePb))
			for _, task := range pendingTasks {
				// The code can produce `poolSize` number of batches at a time. But, it can consume only
				// `parallelism` number of batches at a time.
				batchImportPool = pool.New().WithMaxGoroutines(poolSize)

				totalProgressAmount := getTotalProgressAmount(task)
				progressReporter.ImportFileStarted(task, totalProgressAmount)
				importedProgressAmount := getImportedProgressAmount(task, state)
				progressReporter.AddProgressAmount(task, importedProgressAmount)
				updateProgressFn := func(progressAmount int64) {
					progressReporter.AddProgressAmount(task, progressAmount)
				}
				importFile(state, task, updateProgressFn)
				batchImportPool.Wait()                // Wait for the file import to finish.
				progressReporter.FileImportDone(task) // Remove the progress-bar for the file.
			}
			time.Sleep(time.Second * 2)
		}
		utils.PrintAndLog("snapshot data import complete\n\n")
		callhome.PackAndSendPayload(exportDir)
	}

	if !dbzm.IsDebeziumForDataExport(exportDir) {
		executePostImportDataSqls()
		displayImportedRowCountSnapshot(state, importFileTasks)
	} else {
		if changeStreamingIsEnabled(importType) {
			if importerRole != FB_DB_IMPORTER_ROLE {
				displayImportedRowCountSnapshot(state, importFileTasks)
			}
			color.Blue("streaming changes to %s...", tconf.TargetDBType)
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

			utils.PrintAndLog("Completed streaming all relevant changes to %s", tconf.TargetDBType)
			triggerName, err := getTriggerName(importerRole)
			if err != nil {
				utils.ErrExit("failed to get trigger name after streaming changes: %s", err)
			}
			createTriggerIfNotExists(triggerName)
			utils.PrintAndLog("\nRun the following command to get the current report of the migration:\n" +
				color.CyanString("yb-voyager get data-migration-report --export-dir %q", exportDir))
		} else {
			status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
			if err != nil {
				utils.ErrExit("failed to read export status for restore sequences: %s", err)
			}
			err = tdb.RestoreSequences(status.Sequences)
			if err != nil {
				utils.ErrExit("failed to restore sequences: %s", err)
			}
			displayImportedRowCountSnapshot(state, importFileTasks)
		}
	}

	fmt.Printf("\nImport data complete.\n")
}

func disableGeneratedAlwaysAsIdentityColumns(tables []string) {
	found, err := metaDB.GetJsonObject(nil, identityColumnsMetaDBKey, &TableToIdentityColumnNames)
	if err != nil {
		utils.ErrExit("failed to get identity columns from meta db: %s", err)
	}
	if !found {
		TableToIdentityColumnNames = getIdentityColumnsForTables(tables, "ALWAYS")
		// saving in metadb for handling restarts
		metaDB.InsertJsonObject(nil, identityColumnsMetaDBKey, TableToIdentityColumnNames)
	}

	err = tdb.DisableGeneratedAlwaysAsIdentityColumns(TableToIdentityColumnNames)
	if err != nil {
		utils.ErrExit("failed to disable generated always as identity columns: %s", err)
	}
}

func enableGeneratedAlwaysAsIdentityColumns() {
	err := tdb.EnableGeneratedAlwaysAsIdentityColumns(TableToIdentityColumnNames)
	if err != nil {
		utils.ErrExit("failed to enable generated always as identity columns: %s", err)
	}
}

func restoreGeneratedByDefaultAsIdentityColumns(tables []string) {
	log.Infof("restoring generated by default as identity columns for tables: %v", tables)
	tablesToIdentityColumnNames := getIdentityColumnsForTables(tables, "BY DEFAULT")
	err := tdb.EnableGeneratedByDefaultAsIdentityColumns(tablesToIdentityColumnNames)
	if err != nil {
		utils.ErrExit("failed to enable generated by default as identity columns: %s", err)
	}
}

func getIdentityColumnsForTables(tables []string, identityType string) map[string][]string {
	var result = make(map[string][]string)
	log.Infof("getting identity(%s) columns for tables: %v", identityType, tables)
	for _, table := range tables {
		identityColumns, err := tdb.GetIdentityColumnNamesForTable(table, identityType)
		if err != nil {
			utils.ErrExit("error in getting identity(%s) columns for table %s: %w", identityType, table, err)
		}
		if len(identityColumns) > 0 {
			log.Infof("identity(%s) columns for table %s: %v", identityType, table, identityColumns)
			result[table] = identityColumns
		}
	}
	return result
}

func getTotalProgressAmount(task *ImportFileTask) int64 {
	fileEntry := dataFileDescriptor.GetFileEntry(task.FilePath, task.TableName)
	if fileEntry == nil {
		utils.ErrExit("entry not found for file %q and table %s", task.FilePath, task.TableName)
	}
	if reportProgressInBytes {
		return fileEntry.FileSize
	} else {
		return fileEntry.RowCount
	}
}

func getImportedProgressAmount(task *ImportFileTask, state *ImportDataState) int64 {
	if reportProgressInBytes {
		byteCount, err := state.GetImportedByteCount(task.FilePath, task.TableName)
		if err != nil {
			utils.ErrExit("Failed to get imported byte count for table %s: %s", task.TableName, err)
		}
		return byteCount
	} else {
		rowCount, err := state.GetImportedRowCount(task.FilePath, task.TableName)
		if err != nil {
			utils.ErrExit("Failed to get imported row count for table %s: %s", task.TableName, err)
		}
		return rowCount
	}
}

func importFileTasksToTableNames(tasks []*ImportFileTask) []string {
	tableNames := []string{}
	for _, t := range tasks {
		tableNames = append(tableNames, t.TableName)
	}
	return lo.Uniq(tableNames)
}

func classifyTasks(state *ImportDataState, tasks []*ImportFileTask) (pendingTasks, completedTasks []*ImportFileTask, err error) {
	inProgressTasks := []*ImportFileTask{}
	notStartedTasks := []*ImportFileTask{}
	for _, task := range tasks {
		fileImportState, err := state.GetFileImportState(task.FilePath, task.TableName)
		if err != nil {
			return nil, nil, fmt.Errorf("get table import state: %w", err)
		}
		switch fileImportState {
		case FILE_IMPORT_COMPLETED:
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

func cleanImportState(state *ImportDataState, tasks []*ImportFileTask) {
	tableNames := importFileTasksToTableNames(tasks)
	nonEmptyTableNames := tdb.GetNonEmptyTables(tableNames)
	if len(nonEmptyTableNames) > 0 {
		utils.PrintAndLog("Following tables are not empty. "+
			"TRUNCATE them before importing data with --start-clean.\n%s",
			strings.Join(nonEmptyTableNames, ", "))
		yes := utils.AskPrompt("Do you want to continue without truncating these tables?")
		if !yes {
			utils.ErrExit("Aborting import.")
		}
	}

	for _, task := range tasks {
		err := state.Clean(task.FilePath, task.TableName)
		if err != nil {
			utils.ErrExit("failed to clean import data state for table %q: %s", task.TableName, err)
		}
	}

	sqlldrDir := filepath.Join(exportDir, "sqlldr")
	if utils.FileOrFolderExists(sqlldrDir) {
		err := os.RemoveAll(sqlldrDir)
		if err != nil {
			utils.ErrExit("failed to remove sqlldr directory %q: %s", sqlldrDir, err)
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

func getImportBatchArgsProto(tableName, filePath string) *tgtdb.ImportBatchArgs {
	columns := TableToColumnNames[tableName]
	columns, err := tdb.IfRequiredQuoteColumnNames(tableName, columns)
	if err != nil {
		utils.ErrExit("if required quote column names: %s", err)
	}
	// If `columns` is unset at this point, no attribute list is passed in the COPY command.
	fileFormat := dataFileDescriptor.FileFormat
	if fileFormat == datafile.SQL {
		fileFormat = datafile.TEXT
	}
	importBatchArgsProto := &tgtdb.ImportBatchArgs{
		TableName:  tableName,
		Columns:    columns,
		FileFormat: fileFormat,
		Delimiter:  dataFileDescriptor.Delimiter,
		HasHeader:  dataFileDescriptor.HasHeader && fileFormat == datafile.CSV,
		QuoteChar:  dataFileDescriptor.QuoteChar,
		EscapeChar: dataFileDescriptor.EscapeChar,
		NullString: dataFileDescriptor.NullString,
	}
	log.Infof("ImportBatchArgs: %v", spew.Sdump(importBatchArgsProto))
	return importBatchArgsProto
}

func importFile(state *ImportDataState, task *ImportFileTask, updateProgressFn func(int64)) {

	origDataFile := task.FilePath
	importBatchArgsProto := getImportBatchArgsProto(task.TableName, task.FilePath)
	log.Infof("Start splitting table %q: data-file: %q", task.TableName, origDataFile)

	err := state.PrepareForFileImport(task.FilePath, task.TableName)
	if err != nil {
		utils.ErrExit("preparing for file import: %s", err)
	}
	log.Infof("Collect all interrupted/remaining splits.")
	pendingBatches, lastBatchNumber, lastOffset, fileFullySplit, err := state.Recover(task.FilePath, task.TableName)
	if err != nil {
		utils.ErrExit("recovering state for table %q: %s", task.TableName, err)
	}
	for _, batch := range pendingBatches {
		submitBatch(batch, updateProgressFn, importBatchArgsProto)
	}
	if !fileFullySplit {
		splitFilesForTable(state, origDataFile, task.TableName, lastBatchNumber, lastOffset, updateProgressFn, importBatchArgsProto)
	}
}

func splitFilesForTable(state *ImportDataState, filePath string, t string,
	lastBatchNumber int64, lastOffset int64, updateProgressFn func(int64), importBatchArgsProto *tgtdb.ImportBatchArgs) {
	log.Infof("Split data file %q: tableName=%q, largestSplit=%v, largestOffset=%v", filePath, t, lastBatchNumber, lastOffset)
	batchNum := lastBatchNumber + 1
	numLinesTaken := lastOffset

	reader, err := dataStore.Open(filePath)
	if err != nil {
		utils.ErrExit("preparing reader for split generation on file %q: %v", filePath, err)
	}

	dataFile, err := datafile.NewDataFile(filePath, reader, dataFileDescriptor)
	if err != nil {
		utils.ErrExit("open datafile %q: %v", filePath, err)
	}
	defer dataFile.Close()

	log.Infof("Skipping %d lines from %q", lastOffset, filePath)
	err = dataFile.SkipLines(lastOffset)
	if err != nil {
		utils.ErrExit("skipping line for offset=%d: %v", lastOffset, err)
	}

	var readLineErr error = nil
	var line string
	var batchWriter *BatchWriter
	header := ""
	if dataFileDescriptor.HasHeader {
		header = dataFile.GetHeader()
	}
	for readLineErr == nil {

		if batchWriter == nil {
			batchWriter = state.NewBatchWriter(filePath, t, batchNum)
			err := batchWriter.Init()
			if err != nil {
				utils.ErrExit("initializing batch writer for table %q: %s", t, err)
			}
			if header != "" && dataFileDescriptor.FileFormat == datafile.CSV {
				err = batchWriter.WriteHeader(header)
				if err != nil {
					utils.ErrExit("writing header for table %q: %s", t, err)
				}
			}
		}

		line, readLineErr = dataFile.NextLine()
		if readLineErr == nil || (readLineErr == io.EOF && line != "") {
			// handling possible case: last dataline(i.e. EOF) but no newline char at the end
			numLinesTaken += 1
		}
		if line != "" {
			table := batchWriter.tableName
			// can't use importBatchArgsProto.Columns as to use case insenstiive column names
			line, err = valueConverter.ConvertRow(table, TableToColumnNames[table], line)
			if err != nil {
				utils.ErrExit("transforming line number=%d for table %q in file %s: %s", batchWriter.NumRecordsWritten+1, t, filePath, err)
			}
		}
		err = batchWriter.WriteRecord(line)
		if err != nil {
			utils.ErrExit("Write to batch %d: %s", batchNum, err)
		}
		if batchWriter.NumRecordsWritten == batchSize ||
			dataFile.GetBytesRead() >= tdb.MaxBatchSizeInBytes() ||
			readLineErr != nil {

			isLastBatch := false
			if readLineErr == io.EOF {
				isLastBatch = true
			} else if readLineErr != nil {
				utils.ErrExit("read line from data file %q: %s", filePath, readLineErr)
			}

			offsetEnd := numLinesTaken
			batch, err := batchWriter.Done(isLastBatch, offsetEnd, dataFile.GetBytesRead())
			if err != nil {
				utils.ErrExit("finalizing batch %d: %s", batchNum, err)
			}
			batchWriter = nil
			dataFile.ResetBytesRead()
			submitBatch(batch, updateProgressFn, importBatchArgsProto)

			if !isLastBatch {
				batchNum += 1
			}
		}
	}
	log.Infof("splitFilesForTable: done splitting data file %q for table %q", filePath, t)
}

func executePostImportDataSqls() {
	sequenceFilePath := filepath.Join(exportDir, "data", "postdata.sql")
	if utils.FileOrFolderExists(sequenceFilePath) {
		fmt.Printf("setting resume value for sequences %10s\n", "")
		executeSqlFile(sequenceFilePath, "SEQUENCE", func(_, _ string) bool { return false })
	}
}

func submitBatch(batch *Batch, updateProgressFn func(int64), importBatchArgsProto *tgtdb.ImportBatchArgs) {
	batchImportPool.Go(func() {
		// There are `poolSize` number of competing go-routines trying to invoke COPY.
		// But the `connPool` will allow only `parallelism` number of connections to be
		// used at a time. Thus limiting the number of concurrent COPYs to `parallelism`.
		importBatch(batch, importBatchArgsProto)
		if reportProgressInBytes {
			updateProgressFn(batch.ByteCount)
		} else {
			updateProgressFn(batch.RecordCount)
		}
	})
	log.Infof("Queued batch: %s", spew.Sdump(batch))
}

func importBatch(batch *Batch, importBatchArgsProto *tgtdb.ImportBatchArgs) {
	err := batch.MarkPending()
	if err != nil {
		utils.ErrExit("marking batch %d as pending: %s", batch.Number, err)
	}
	log.Infof("Importing %q", batch.FilePath)

	importBatchArgs := *importBatchArgsProto
	importBatchArgs.FilePath = batch.FilePath
	importBatchArgs.RowsPerTransaction = batch.OffsetEnd - batch.OffsetStart

	var rowsAffected int64
	sleepIntervalSec := 0
	for attempt := 0; attempt < COPY_MAX_RETRY_COUNT; attempt++ {
		rowsAffected, err = tdb.ImportBatch(batch, &importBatchArgs, exportDir, TableNameToSchema[batch.TableName])
		if err == nil || tdb.IsNonRetryableCopyError(err) {
			break
		}
		log.Warnf("COPY FROM file %q: %s", batch.FilePath, err)
		sleepIntervalSec += 10
		if sleepIntervalSec > MAX_SLEEP_SECOND {
			sleepIntervalSec = MAX_SLEEP_SECOND
		}
		log.Infof("sleep for %d seconds before retrying the file %s (attempt %d)",
			sleepIntervalSec, batch.FilePath, attempt)
		time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
	}
	log.Infof("%q => %d rows affected", batch.FilePath, rowsAffected)
	if err != nil {
		utils.ErrExit("import %q into %s: %s", batch.FilePath, batch.TableName, err)
	}
	err = batch.MarkDone()
	if err != nil {
		utils.ErrExit("marking batch %q as done: %s", batch.FilePath, err)
	}
}

func newTargetConn() *pgx.Conn {
	conn, err := pgx.Connect(context.Background(), tconf.GetConnectionUri())
	if err != nil {
		utils.WaitChannel <- 1
		<-utils.WaitChannel
		utils.ErrExit("connect to target db: %s", err)
	}

	setTargetSchema(conn)
	return conn
}

// TODO: Eventually get rid of this function in favour of TargetYugabyteDB.setTargetSchema().
func setTargetSchema(conn *pgx.Conn) {
	if sourceDBType == POSTGRESQL || tconf.Schema == YUGABYTEDB_DEFAULT_SCHEMA {
		// For PG, schema name is already included in the object name.
		// No need to set schema if importing in the default schema.
		return
	}
	checkSchemaExistsQuery := fmt.Sprintf("SELECT count(schema_name) FROM information_schema.schemata WHERE schema_name = '%s'", tconf.Schema)
	var cntSchemaName int

	if err := conn.QueryRow(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		utils.ErrExit("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, tconf.Host, err)
	} else if cntSchemaName == 0 {
		utils.ErrExit("schema '%s' does not exist in target", tconf.Schema)
	}

	setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", tconf.Schema)
	_, err := conn.Exec(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q: %s", setSchemaQuery, tconf.Host, err)
	}

	if sourceDBType == ORACLE && enableOrafce {
		// append oracle schema in the search_path for orafce
		updateSearchPath := `SELECT set_config('search_path', current_setting('search_path') || ', oracle', false)`
		_, err := conn.Exec(context.Background(), updateSearchPath)
		if err != nil {
			utils.ErrExit("unable to update search_path for orafce extension: %v", err)
		}
	}
}

func dropIdx(conn *pgx.Conn, idxName string) {
	dropIdxQuery := fmt.Sprintf("DROP INDEX IF EXISTS %s", idxName)
	log.Infof("Dropping index: %q", dropIdxQuery)
	_, err := conn.Exec(context.Background(), dropIdxQuery)
	if err != nil {
		utils.ErrExit("Failed in dropping index %q: %v", idxName, err)
	}
}

func executeSqlFile(file string, objType string, skipFn func(string, string) bool) {
	log.Infof("Execute SQL file %q on target %q", file, tconf.Host)
	conn := newTargetConn()

	if sourceDBType == ORACLE && enableOrafce {
        setOrafceSearchPath(conn)
    }

	defer func() {
		if conn != nil {
			conn.Close(context.Background())
		}
	}()

	sqlInfoArr := createSqlStrInfoArray(file, objType)
	for _, sqlInfo := range sqlInfoArr {
		if conn == nil {
			conn = newTargetConn()
		}

		setOrSelectStmt := strings.HasPrefix(strings.ToUpper(sqlInfo.stmt), "SET ") ||
			strings.HasPrefix(strings.ToUpper(sqlInfo.stmt), "SELECT ")
		if !setOrSelectStmt && skipFn != nil && skipFn(objType, sqlInfo.stmt) {
			continue
		}

		err := executeSqlStmtWithRetries(&conn, sqlInfo, objType)
		if err != nil {
			conn.Close(context.Background())
			conn = nil
		}
	}
}

func setOrafceSearchPath(conn *pgx.Conn) {
	// append oracle schema in the search_path for orafce
	updateSearchPath := `SELECT set_config('search_path', current_setting('search_path') || ', oracle', false)`
	_, err := conn.Exec(context.Background(), updateSearchPath)
	if err != nil {
		utils.ErrExit("unable to update search_path for orafce extension: %v", err)
	}
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

func executeSqlStmtWithRetries(conn **pgx.Conn, sqlInfo sqlInfo, objType string) error {
	var err error
	log.Infof("On %s run query:\n%s\n", tconf.Host, sqlInfo.formattedStmt)
	for retryCount := 0; retryCount <= DDL_MAX_RETRY_COUNT; retryCount++ {
		if retryCount > 0 { // Not the first iteration.
			log.Infof("Sleep for 5 seconds before retrying for %dth time", retryCount)
			time.Sleep(time.Second * 5)
			log.Infof("RETRYING DDL: %q", sqlInfo.stmt)
		}
		_, err = (*conn).Exec(context.Background(), sqlInfo.formattedStmt)
		if err == nil {
			utils.PrintSqlStmtIfDDL(sqlInfo.stmt, utils.GetObjectFileName(filepath.Join(exportDir, "schema"), objType))
			return nil
		}

		log.Errorf("DDL Execution Failed for %q: %s", sqlInfo.formattedStmt, err)
		if strings.Contains(strings.ToLower(err.Error()), "conflicts with higher priority transaction") {
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()
			continue
		} else if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(SCHEMA_VERSION_MISMATCH_ERR)) &&
			objType == "INDEX" || objType == "PARTITION_INDEX" { // retriable error
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()

			// Extract the schema name and add to the index name
			fullyQualifiedObjName, err := getIndexName(sqlInfo.stmt, sqlInfo.objName)
			if err != nil {
				utils.ErrExit("extract qualified index name from DDL [%v]: %v", sqlInfo.stmt, err)
			}

			// DROP INDEX in case INVALID index got created
			dropIdx(*conn, fullyQualifiedObjName)
			continue
		} else if missingRequiredSchemaObject(err) {
			log.Infof("deffering execution of SQL: %s", sqlInfo.formattedStmt)
			defferedSqlStmts = append(defferedSqlStmts, sqlInfo)
		} else if isAlreadyExists(err.Error()) {
			// pg_dump generates `CREATE SCHEMA public;` in the schemas.sql. Because the `public`
			// schema already exists on the target YB db, the create schema statement fails with
			// "already exists" error. Ignore the error.
			if bool(tconf.IgnoreIfExists) || strings.EqualFold(strings.Trim(sqlInfo.stmt, " \n"), "CREATE SCHEMA public;") {
				err = nil
			}
		}
		break // no more iteration in case of non retriable error
	}
	if err != nil {
		if missingRequiredSchemaObject(err) {
			// Do nothing
		} else {
			utils.PrintSqlStmtIfDDL(sqlInfo.stmt, utils.GetObjectFileName(filepath.Join(exportDir, "schema"), objType))
			color.Red(fmt.Sprintf("%s\n", err.Error()))
			if tconf.ContinueOnError {
				log.Infof("appending stmt to failedSqlStmts list: %s\n", utils.GetSqlStmtToPrint(sqlInfo.stmt))
				errString := "/*\n" + err.Error() + "\n*/\n"
				failedSqlStmts = append(failedSqlStmts, errString+sqlInfo.formattedStmt)
			} else {
				utils.ErrExit("error: %s\n", err)
			}
		}
	}
	return err
}

// TODO: This function is a duplicate of the one in tgtdb/yb.go. Consolidate the two.
func getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return tconf.Schema // default set to "public"
}

func prepareTableToColumns(tasks []*ImportFileTask) {
	for _, task := range tasks {
		table := task.TableName
		var columns []string
		if dataFileDescriptor.TableNameToExportedColumns != nil {
			columns = dataFileDescriptor.TableNameToExportedColumns[table]
		} else if dataFileDescriptor.HasHeader {
			// File is either exported from debezium OR this is `import data file` case.
			reader, err := dataStore.Open(task.FilePath)
			if err != nil {
				utils.ErrExit("datastore.Open %q: %v", task.FilePath, err)
			}
			df, err := datafile.NewDataFile(task.FilePath, reader, dataFileDescriptor)
			if err != nil {
				utils.ErrExit("opening datafile %q: %v", task.FilePath, err)
			}
			header := df.GetHeader()
			columns = strings.Split(header, dataFileDescriptor.Delimiter)
			log.Infof("read header from file %q: %s", task.FilePath, header)
			log.Infof("header row split using delimiter %q: %v\n", dataFileDescriptor.Delimiter, columns)
			df.Close()
		}
		TableToColumnNames[table] = columns
	}
}

func quoteIdentifierIfRequired(identifier string) string {
	if sqlname.IsQuoted(identifier) {
		return identifier
	}
	// TODO: Use either sourceDBType or source.DBType throughout the code.
	// In the export code path source.DBType is used. In the import code path
	// sourceDBType is used.
	dbType := source.DBType
	if dbType == "" {
		dbType = sourceDBType
	}
	if sqlname.IsReservedKeywordPG(identifier) ||
		(dbType == POSTGRESQL && sqlname.IsCaseSensitive(identifier, dbType)) {
		return fmt.Sprintf(`"%s"`, identifier)
	}
	return identifier
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
	importCmd.AddCommand(importDataCmd)
	importDataCmd.AddCommand(importDataToCmd)
	importDataToCmd.AddCommand(importDataToTargetCmd)

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
	registerImportDataFlags(importDataCmd)
	registerImportDataFlags(importDataToTargetCmd)
}
