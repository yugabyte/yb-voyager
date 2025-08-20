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
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	_ "github.com/godror/godror"
	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	"github.com/hashicorp/go-version"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/go-ps"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"golang.org/x/term"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/anon"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

const (
	ANONYMISATION_SALT_SIZE = 16 // Size of salt in bytes, can be adjusted as needed
)

var (
	metaDB                 *metadb.MetaDB
	anonymizer             *anon.VoyagerAnonymizer
	PARENT_COMMAND_USAGE   = "Parent command. Refer to the sub-commands for usage help."
	startTime              time.Time
	targetDbVersionStrFlag string
	targetDbVersion        *ybversion.YBVersion
)

func PrintElapsedDuration() {
	uninitialisedTimestamp := time.Time{}
	if startTime == uninitialisedTimestamp {
		return
	}
	log.Infof("End time: %s\n", time.Now())
	timeTakenByCurrentVoyagerInvocation := time.Since(startTime)
	log.Infof("Time taken: %s (%.2f seconds)\n",
		timeTakenByCurrentVoyagerInvocation,
		timeTakenByCurrentVoyagerInvocation.Seconds())
}

func updateFilePaths(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	var requiredMap map[string]string

	// TODO: handle the case if table name has double quotes/case sensitive

	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(filepath.Join(exportDir, "data"), false)
		for _, key := range sortedKeys {
			tableName := tablesProgressMetadata[key].TableName
			fullTableName := tableName.ForKey()
			table := tableName.ForMinOutput()
			if _, ok := requiredMap[fullTableName]; ok { // checking if toc/dump has data file for table
				tablesProgressMetadata[key].InProgressFilePath = filepath.Join(exportDir, "data", requiredMap[fullTableName])
				tablesProgressMetadata[key].FinalFilePath = filepath.Join(exportDir, "data", table+"_data.sql")
			} else {
				log.Infof("deleting an entry %q from tablesProgressMetadata: ", key)
				delete(tablesProgressMetadata, key)
			}
		}
	} else if source.DBType == "oracle" || source.DBType == "mysql" {
		for _, key := range sortedKeys {
			_, tname := tablesProgressMetadata[key].TableName.ForCatalogQuery()
			targetTableName := tname
			tablesProgressMetadata[key].InProgressFilePath = filepath.Join(exportDir, "data", "tmp_"+targetTableName+"_data.sql")
			tablesProgressMetadata[key].FinalFilePath = filepath.Join(exportDir, "data", targetTableName+"_data.sql")
		}
	}

	logMsg := "After updating data file paths, TablesProgressMetadata:"
	for _, key := range sortedKeys {
		logMsg += fmt.Sprintf("%+v\n", tablesProgressMetadata[key])
	}
	log.Infof(logMsg)
}

func getMappingForTableNameVsTableFileName(dataDirPath string, noWait bool) map[string]string {
	tocTextFilePath := filepath.Join(dataDirPath, "toc.txt")
	if noWait && !utils.FileOrFolderExists(tocTextFilePath) { // to avoid infine wait for export data status command
		return nil
	}
	for !utils.FileOrFolderExists(tocTextFilePath) {
		time.Sleep(time.Second * 1)
	}

	pgRestorePath, binaryCheckIssue, err := srcdb.GetAbsPathOfPGCommandAboveVersion("pg_restore", source.DBVersion)
	if err != nil {
		utils.ErrExit("could not get absolute path of pg_restore command: %s", err)
	} else if binaryCheckIssue != "" {
		utils.ErrExit("could not get absolute path of pg_restore command: %s", binaryCheckIssue)
	}
	pgRestoreCmd := exec.Command(pgRestorePath, "-l", dataDirPath)
	stdOut, err := pgRestoreCmd.Output()
	log.Infof("cmd: %s", pgRestoreCmd.String())
	log.Infof("output: %s", string(stdOut))
	if err != nil {
		utils.ErrExit("couldn't parse the TOC file to collect the tablenames for data files: %v", err)
	}

	tableNameVsFileNameMap := make(map[string]string)
	var sequencesPostData strings.Builder

	lines := strings.Split(string(stdOut), "\n")
	for _, line := range lines {
		// example of line: 3725; 0 16594 TABLE DATA public categories ds2
		parts := strings.Split(line, " ")

		if len(parts) < 8 { // those lines don't contain table/sequences related info
			continue
		} else if parts[3] == "TABLE" && parts[4] == "DATA" {
			fileName := strings.Trim(parts[0], ";") + ".dat"
			schemaName := parts[5]
			tableName := parts[6]
			if nameContainsCapitalLetter(tableName) || sqlname.IsReservedKeywordPG(tableName) {
				// Surround the table name with double quotes.
				tableName = fmt.Sprintf("\"%s\"", tableName)
			}
			fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)
			table, err := namereg.NameReg.LookupTableName(fullTableName)
			if err != nil {
				utils.ErrExit("lookup table in name registry: %q: %v", fullTableName, err)
			}
			tableNameVsFileNameMap[table.ForKey()] = fileName
		}
	}

	tocTextFileDataBytes, err := os.ReadFile(tocTextFilePath)
	if err != nil {
		utils.ErrExit("Failed to read file: %q: %v", tocTextFilePath, err)
	}

	tocTextFileData := strings.Split(string(tocTextFileDataBytes), "\n")
	numLines := len(tocTextFileData)
	setvalRegex := regexp.MustCompile("(?i)SELECT.*setval")

	for i := 0; i < numLines; i++ {
		if setvalRegex.MatchString(tocTextFileData[i]) {
			sequencesPostData.WriteString(tocTextFileData[i])
			sequencesPostData.WriteString("\n")
		}
	}

	//extracted SQL for setval() and put it into a postexport.sql file
	os.WriteFile(filepath.Join(dataDirPath, "postdata.sql"), []byte(sequencesPostData.String()), 0644)
	return tableNameVsFileNameMap
}

func UpdateTableApproxRowCount(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	utils.PrintAndLog("calculating approx num of rows to export for each table...")
	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	for _, key := range sortedKeys {
		approxRowCount := source.DB().GetTableApproxRowCount(tablesProgressMetadata[key].TableName)
		tablesProgressMetadata[key].CountTotalRows = approxRowCount
	}

	log.Tracef("After updating total approx row count, TablesProgressMetadata: %+v", tablesProgressMetadata)
}

func GetTableRowCount(filePath string) map[string]int64 {
	tableRowCountMap := make(map[string]int64)

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		utils.ErrExit("read file: %q: %s", filePath, err)
	}

	lines := strings.Split(strings.Trim(string(fileBytes), "\n"), "\n")

	for _, line := range lines {
		tableName := strings.Split(line, ",")[0]
		rowCount := strings.Split(line, ",")[1]
		rowCountInt64, _ := strconv.ParseInt(rowCount, 10, 64)

		tableRowCountMap[tableName] = rowCountInt64
	}

	log.Infof("tableRowCountMap: %v", tableRowCountMap)
	return tableRowCountMap
}

func getExportedRowCountSnapshot(exportDir string) map[string]int64 {
	tableRowCount := map[string]int64{}
	for _, fileEntry := range datafile.OpenDescriptor(exportDir).DataFileList {
		tableRowCount[fileEntry.TableName] += fileEntry.RowCount
	}
	return tableRowCount
}

func getLeafPartitionsFromRootTable() map[string][]string {
	leafPartitions := make(map[string][]string)
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}
	if msr.SourceDBConf.DBType != POSTGRESQL {
		return leafPartitions
	}
	tables := msr.TableListExportedFromSource
	for leaf, root := range msr.SourceRenameTablesMap {
		//Using the SQLName here to avoid creating the NameTuples manually for leafTable case as in a case partition names changes on target
		//NameRegistry won't be able to figure out the map of source->target tuples.
		leafTable := sqlname.NewSourceNameFromQualifiedName(getQuotedFromUnquoted(leaf))
		rootTable := sqlname.NewSourceNameFromQualifiedName(getQuotedFromUnquoted(root))
		leaf = leafTable.Qualified.MinQuoted
		if leafTable.SchemaName.MinQuoted == "public" {
			leaf = leafTable.ObjectName.MinQuoted
		}
		root = rootTable.Qualified.MinQuoted
		if !lo.Contains(tables, root) {
			continue
		}
		//Adding a Qualified.MinQuoted to key and values which is similar to NameTuple.ForOutput();
		leafPartitions[root] = append(leafPartitions[root], leaf)
	}

	return leafPartitions
}

func getQuotedFromUnquoted(t string) string {
	//To preserve case sensitiveness in the Unquoted
	parts := strings.Split(t, ".")
	s, t := parts[0], parts[1]
	return fmt.Sprintf(`%s."%s"`, s, t)
}

func displayExportedRowCountSnapshot(snapshotViaDebezium bool) {
	fmt.Printf("snapshot export report\n")
	uitable := uitable.New()

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	leafPartitions := getLeafPartitionsFromRootTable()
	if !snapshotViaDebezium {
		exportedRowCount := getExportedRowCountSnapshot(exportDir)
		if source.Schema != "" {
			addHeader(uitable, "SCHEMA", "TABLE", "ROW COUNT")
		} else {
			addHeader(uitable, "DATABASE", "TABLE", "ROW COUNT")
		}
		keys := lo.Keys(exportedRowCount)
		sort.Slice(keys, func(i, j int) bool {
			return exportedRowCount[keys[i]] > exportedRowCount[keys[j]]
		})

		for _, key := range keys {
			table, err := namereg.NameReg.LookupTableName(key)
			if err != nil {
				utils.ErrExit("lookup table in name registry: %q: %v", key, err)
			}
			displayTableName := table.CurrentName.Unqualified.MinQuoted
			//Using the ForOutput() as a key for leafPartitions map as we are populating the map in that way.
			partitions := leafPartitions[table.ForOutput()]
			if source.DBType == POSTGRESQL && partitions != nil && msr.IsExportTableListSet {
				partitions := strings.Join(partitions, ", ")
				displayTableName = fmt.Sprintf("%s (%s)", table.CurrentName.Unqualified.MinQuoted, partitions)
			}
			schema := table.SourceName.SchemaName
			uitable.AddRow(schema, displayTableName, exportedRowCount[key])
		}
		if len(keys) > 0 {
			fmt.Print("\n")
			fmt.Println(uitable)
			fmt.Print("\n")
		}
		return
	}

	exportStatus, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
	if err != nil {
		utils.ErrExit("failed to read export status during data export snapshot-and-changes report display: %v", err)
	}
	for i, tableStatus := range exportStatus.Tables {
		if i == 0 {
			if tableStatus.SchemaName != "" {
				addHeader(uitable, "SCHEMA", "TABLE", "ROW COUNT")
			} else {
				addHeader(uitable, "DATABASE", "TABLE", "ROW COUNT")
			}
		}
		table, err := namereg.NameReg.LookupTableName(fmt.Sprintf("%s.%s", tableStatus.SchemaName, tableStatus.TableName))
		if err != nil {
			utils.ErrExit("lookup table  in name registry : %q: %v", tableStatus.TableName, err)
		}
		displayTableName := table.CurrentName.Unqualified.MinQuoted
		partitions := leafPartitions[table.ForOutput()]
		if source.DBType == POSTGRESQL && partitions != nil {
			partitions := strings.Join(partitions, ", ")
			displayTableName = fmt.Sprintf("%s (%s)", table.CurrentName.Unqualified.MinQuoted, partitions)
		}
		schema := table.CurrentName.SchemaName
		uitable.AddRow(schema, displayTableName, tableStatus.ExportedRowCountSnapshot)

	}
	fmt.Print("\n")
	fmt.Println(uitable)
	fmt.Print("\n")
}

func renameDatafileDescriptor(exportDir string) {
	datafileDescriptor := datafile.OpenDescriptor(exportDir)
	log.Infof("Parsed DataFileDescriptor: %v", spew.Sdump(datafileDescriptor))
	for _, fileEntry := range datafileDescriptor.DataFileList {
		renamedTable, isRenamed := renameTableIfRequired(fileEntry.TableName)
		if isRenamed {
			fileEntry.TableName = renamedTable
		}
	}
	for k, v := range datafileDescriptor.TableNameToExportedColumns {
		renamedTable, isRenamed := renameTableIfRequired(k)
		if isRenamed {
			datafileDescriptor.TableNameToExportedColumns[renamedTable] = v
			delete(datafileDescriptor.TableNameToExportedColumns, k)
		}
	}
	datafileDescriptor.Save()
}

func displayImportedRowCountSnapshot(state *ImportDataState, tasks []*ImportFileTask, errorHandler importdata.ImportDataErrorHandler) {
	if importerRole == IMPORT_FILE_ROLE {
		fmt.Printf("import report\n")
	} else {
		fmt.Printf("snapshot import report\n")
	}
	tableList := importFileTasksToTableNameTuples(tasks)
	err := retrieveMigrationUUID()
	if err != nil {
		utils.ErrExit("could not retrieve migration UUID: %w", err)
	}
	uitable := uitable.New()

	// TODO: refactor this; we don't need to pass the dbType as a parameter,
	// we can just pass the importerRole directly.
	var dbType string
	switch importerRole {
	case IMPORT_FILE_ROLE:
		dbType = "target-file"
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		dbType = "source-replica"
	case TARGET_DB_IMPORTER_ROLE:
		dbType = "target"
	}

	snapshotRowCount, err := getImportedSnapshotRowsMap(dbType)
	if err != nil {
		utils.ErrExit("failed to get imported snapshot rows map: %v", err)
	}

	keys := make([]sqlname.NameTuple, 0, len(snapshotRowCount.Keys()))
	snapshotRowCount.IterKV(func(k sqlname.NameTuple, v RowCountPair) (bool, error) {
		keys = append(keys, k)
		return true, nil
	})

	sort.Slice(keys, func(i, j int) bool {
		val1, _ := snapshotRowCount.Get(keys[i])
		val2, _ := snapshotRowCount.Get(keys[j])
		return val1.Imported > val2.Imported
	})

	hasErrors := false
	for _, tableName := range keys {
		rowCountPair, _ := snapshotRowCount.Get(tableName)
		if rowCountPair.Errored > 0 {
			hasErrors = true
			break
		}
	}

	for i, tableName := range keys {
		if i == 0 {
			if hasErrors {
				addHeader(uitable, "SCHEMA", "TABLE", "IMPORTED ROW COUNT", "ERRORED ROW COUNT")
			} else {
				addHeader(uitable, "SCHEMA", "TABLE", "IMPORTED ROW COUNT")
			}
		}
		s, t := tableName.ForCatalogQuery()
		rowCountPair, _ := snapshotRowCount.Get(tableName)
		if hasErrors {
			uitable.AddRow(s, t, rowCountPair.Imported, rowCountPair.Errored)
		} else {
			uitable.AddRow(s, t, rowCountPair.Imported)
		}
	}
	if len(tableList) > 0 {
		fmt.Printf("\n")
		fmt.Println(uitable)
		fmt.Printf("\n")
	}
	if hasErrors {
		// in case there are errored rows, and we are in on-pk-conflict ignore mode,
		// it is possible that the batches which errored out were partially ingested.
		if tconf.OnPrimaryKeyConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE {
			utils.PrintAndLog(color.YellowString("Note: It is possible that the table row count on the target DB may not match the IMPORTED ROW COUNT as some batches may have been partially ingested."))
		}
		utils.PrintAndLog(color.RedString("Errored snapshot rows are stashed in %q", errorHandler.GetErrorsLocation()))
	}
}

// setup a project having subdirs for various database objects IF NOT EXISTS
func CreateMigrationProjectIfNotExists(dbType string, exportDir string) {
	// TODO: add a check/prompt if any directories apart from required ones are present in export-dir
	var projectSubdirs = []string{
		"schema", "data", "reports",
		"assessment", "assessment/metadata", "assessment/dbs", "assessment/metadata/schema", "assessment/reports",
		"metainfo", "metainfo/data", "metainfo/conf", "metainfo/ssl",
		"temp", "temp/ora2pg_temp_dir", "temp/schema",
	}

	log.Info("Creating a project directory if not exists...")
	//Assuming export directory as a project directory
	projectDirPath := exportDir
	for _, subdir := range projectSubdirs {
		err := exec.Command("mkdir", "-p", filepath.Join(projectDirPath, subdir)).Run()
		if err != nil {
			utils.ErrExit("couldn't create sub-directories under: %q: %v", projectDirPath, err)
		}
	}

	// creating subdirs under schema dir
	for _, schemaObjectType := range source.ExportObjectTypeList {
		if schemaObjectType == "INDEX" || schemaObjectType == "FOREIGN TABLE" || schemaObjectType == "ROW SECURITY" ||
			schemaObjectType == "OPERATOR FAMILY" || schemaObjectType == "OPERATOR CLASS" { //no separate dir for indexes
			continue
		}
		databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"
		if schemaObjectType == "POLICY" {
			databaseObjectDirName = "policies"
		}

		err := exec.Command("mkdir", "-p", filepath.Join(schemaDir, databaseObjectDirName)).Run()
		if err != nil {
			utils.ErrExit("couldn't create sub-directories under: %q: %v", schemaDir, err)
		}
	}

	metaDB = initMetaDB(exportDir)
}

func initMetaDB(migrationExportDir string) *metadb.MetaDB {
	err := metadb.CreateAndInitMetaDBIfRequired(migrationExportDir)
	if err != nil {
		utils.ErrExit("could not create and init meta db: %w", err)
	}
	metaDBInstance, err := metadb.NewMetaDB(migrationExportDir)
	if err != nil {
		utils.ErrExit("failed to initialize meta db: %s", err)
	}
	err = metaDBInstance.InitMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not init migration status record: %w", err)
	}

	// TODO: initialising anonymizer should be a top-level function call, like in root.go
	// but right now, initMetaDB is called from multiple places(from CreateMigrationProjectIfNotExists and root.go in some case)
	// so just keeping it here until we refactor and cleanup the code.
	err = initAnonymizer(metaDBInstance)
	if err != nil {
		utils.ErrExit("could not initialize anonymizer: %v", err)
	}

	return metaDBInstance
}

func initAnonymizer(metaDBInstance *metadb.MetaDB) error {
	// generate salt and initialise the anonymiser
	salt, err := loadOrGenerateAnonymisationSalt(metaDBInstance)
	if err != nil {
		utils.ErrExit("could not load or generate anonymisation salt: %v", err)
	}

	anonymizer, err = anon.NewVoyagerAnonymizer(salt)
	if err != nil {
		return fmt.Errorf("could not create anonymizer: %w", err)
	} else if anonymizer == nil {
		return errors.New("anonymizer is nil, this should not happen")
	}
	return nil
}

func loadOrGenerateAnonymisationSalt(metaDB *metadb.MetaDB) (string, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return "", fmt.Errorf("error getting migration status record: %w", err)
	}

	var salt string
	if msr != nil && msr.AnonymizerSalt != "" {
		salt = msr.AnonymizerSalt
	} else {
		salt, err = utils.GenerateAnonymisationSalt(ANONYMISATION_SALT_SIZE)
		if err != nil {
			return "", fmt.Errorf("error generating salt: %w", err)
		}

		// Store the generated salt in the migration status record
		err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			if record == nil { // should not happen, but just in case
				record = &metadb.MigrationStatusRecord{}
			}
			record.AnonymizerSalt = salt
		})
		if err != nil {
			return "", fmt.Errorf("error updating migration status record with salt: %w", err)
		}
	}
	return salt, nil
}

func detectVersionCompatibility(msrVoyagerVersionString string, migrationExportDir string) {
	// If the msr VoyagerVersion is less than the PREVIOUS_BREAKING_CHANGE_VERSION, then the export-dir is not compatible with the current Voyager version.
	// This version will always be a final release version and never "main" or "rc" version.
	previousBreakingChangeVersion, err := version.NewVersion(utils.PREVIOUS_BREAKING_CHANGE_VERSION)
	if err != nil {
		utils.ErrExit("could not create version from %q: %v", utils.PREVIOUS_BREAKING_CHANGE_VERSION, err)
	}

	var versionCheckFailed bool

	if msrVoyagerVersionString == "main" {
		// If the export-dir was created using the main branch, then the current version should also be the main branch.
		if utils.YB_VOYAGER_VERSION != "main" {
			versionCheckFailed = true
		}
	} else if msrVoyagerVersionString != "" {
		msrVoyagerFinalVersion := msrVoyagerVersionString
		if strings.Contains(msrVoyagerFinalVersion, "rc") {
			msrVoyagerFinalVersion, err = utils.GetFinalReleaseVersionFromRCVersion(msrVoyagerFinalVersion)
			if err != nil {
				utils.ErrExit("could not get final release version from rc version %q: %v", msrVoyagerFinalVersion, err)
			}
		}

		msrVoyagerVersion, err := version.NewVersion(msrVoyagerFinalVersion)
		if err != nil {
			utils.ErrExit("could not create version from %q: %v", msrVoyagerFinalVersion, err)
		}

		if msrVoyagerVersion.LessThan(previousBreakingChangeVersion) {
			versionCheckFailed = true
		} else {
			// If the export-dir was created using a version greater than or equal to the PREVIOUS_BREAKING_CHANGE_VERSION,
			// then if the current voyager version does not match the export-dir version, then just print a note warning the user.
			noteString := fmt.Sprintf(color.YellowString("Note: The export-dir %q was created using voyager version %q. "+
				"The current version is %q."),
				migrationExportDir, msrVoyagerVersionString, utils.YB_VOYAGER_VERSION)

			if utils.YB_VOYAGER_VERSION == "main" {
				// In this case we won't be able to convert the version using version.NewVersion() as "main" is not a valid version.
				// Moreover, we know here that the msrVoyagerVersion is not "main" as we have already handled that case above.
				// Therefore, the current version and the msrVoyagerVersion will not be equal.
				utils.PrintAndLog("%s", noteString)
			} else {
				currentVersion, err := version.NewVersion(utils.YB_VOYAGER_VERSION)
				if err != nil {
					utils.ErrExit("could not create version from %q: %v", utils.YB_VOYAGER_VERSION, err)
				}
				if !currentVersion.Equal(msrVoyagerVersion) {
					utils.PrintAndLog("%s", noteString)
				}
			}
		}
	}

	if versionCheckFailed {
		userFacingMsg := fmt.Sprintf("\nThe export-dir %q was created using voyager version %q. "+
			"However, the current version %q requires the export-dir to be created using version %q or later. "+
			"Either use a compatible version to continue the migration or start afresh with a new export-dir. ",
			migrationExportDir, msrVoyagerVersionString, utils.YB_VOYAGER_VERSION, utils.PREVIOUS_BREAKING_CHANGE_VERSION)
		if msrVoyagerVersionString == "" { //In case the export dir is already started from older version that will not have VoyagerVersion field in MSR
			userFacingMsg = fmt.Sprintf("\nThe export-dir %q was created using older version. "+
				"However, the current version %q requires the export-dir to be created using version %q or later. "+
				"Either use a compatible version to continue the migration or start afresh with a new export-dir. ",
				migrationExportDir, utils.YB_VOYAGER_VERSION, utils.PREVIOUS_BREAKING_CHANGE_VERSION)
		}
		utils.ErrExit(userFacingMsg)
	}
}

func initAssessmentDB() {
	err := migassessment.InitAssessmentDB()
	if err != nil {
		utils.ErrExit("error creating and initializing assessment DB: %v", err)
	}

	assessmentDB, err = migassessment.NewAssessmentDB(source.DBType)
	if err != nil {
		utils.ErrExit("error creating assessment DB instance: %v", err)
	}
}

func InitNameRegistry(
	exportDir string, role string,
	sconf *srcdb.Source, sdb srcdb.SourceDB,
	tconf *tgtdb.TargetConf, tdb tgtdb.TargetDB,
	reregisterYBNames bool) error {

	var sdbReg namereg.SourceDBInterface
	var ybdb namereg.YBDBInterface
	var sourceDbType, sourceDbSchema, sourceDbName string
	var targetDBSchema string

	if sconf != nil {
		sourceDbType = sconf.DBType
		sourceDbName = sconf.DBName
		sourceDbSchema = sconf.Schema
	}
	if sdb != nil {
		sdbReg = sdb.(namereg.SourceDBInterface)
	}

	if tconf != nil {
		targetDBSchema = tconf.Schema
	}
	var ok bool
	if tdb != nil && lo.Contains([]string{TARGET_DB_IMPORTER_ROLE, IMPORT_FILE_ROLE}, role) {
		ybdb, ok = tdb.(namereg.YBDBInterface)
		if !ok {
			return fmt.Errorf("expected targetDB to adhere to YBDBRegirsty")
		}
	}
	nameregistryParams := namereg.NameRegistryParams{
		FilePath:       fmt.Sprintf("%s/metainfo/name_registry.json", exportDir),
		Role:           role,
		SourceDBType:   sourceDbType,
		SourceDBSchema: sourceDbSchema,
		SourceDBName:   sourceDbName,
		TargetDBSchema: targetDBSchema,
		SDB:            sdbReg,
		YBDB:           ybdb,
	}

	err := namereg.InitNameRegistry(nameregistryParams)
	if err != nil {
		return err
	}
	if reregisterYBNames {
		// clean up yb names and re-init.
		err := namereg.NameReg.UnRegisterYBNames()
		if err != nil {
			return fmt.Errorf("unregister yb names: %v", err)
		}
		err = namereg.NameReg.Init()
		if err != nil {
			return fmt.Errorf("init name registry: %v", err)
		}
	}
	return nil
}

// sets the global variable migrationUUID after retrieving it from MigrationStatusRecord
func retrieveMigrationUUID() error {
	if migrationUUID != uuid.Nil {
		return nil
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("retrieving migration status record: %w", err)
	}
	if msr == nil {
		return fmt.Errorf("migration status record not found")
	}

	migrationUUID = uuid.MustParse(msr.MigrationUUID)
	utils.PrintAndLog("migrationID: %s", migrationUUID)
	return nil
}

func nameContainsCapitalLetter(name string) bool {
	for _, c := range name {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}

func getCutoverStatus() string {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}

	a := msr.CutoverToTargetRequested
	b := msr.CutoverProcessedBySourceExporter
	c := msr.CutoverProcessedByTargetImporter

	if !a {
		return NOT_INITIATED
	}
	if msr.FallForwardEnabled && a && b && c && msr.ExportFromTargetFallForwardStarted {
		return COMPLETED
	} else if msr.FallbackEnabled && a && b && c && msr.ExportFromTargetFallBackStarted {
		return COMPLETED
	} else if !msr.FallForwardEnabled && !msr.FallbackEnabled && a && b && c {
		return COMPLETED
	}
	return INITIATED
}

func checkStreamingMode() (bool, error) {
	var err error
	migrationStatus, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return false, fmt.Errorf("error while fetching migration status record: %w", err)
	}
	streamChanges := changeStreamingIsEnabled(migrationStatus.ExportType)
	return streamChanges, nil
}

func getCutoverToSourceReplicaStatus() string {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}
	a := msr.CutoverToSourceReplicaRequested
	b := msr.CutoverToSourceReplicaProcessedByTargetExporter
	c := msr.CutoverToSourceReplicaProcessedBySRImporter

	if !a {
		return NOT_INITIATED
	} else if a && b && c {
		return COMPLETED
	}
	return INITIATED
}

func getCutoverToSourceStatus() string {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}
	a := msr.CutoverToSourceRequested
	b := msr.CutoverToSourceProcessedByTargetExporter
	c := msr.CutoverToSourceProcessedBySourceImporter

	if !a {
		return NOT_INITIATED
	} else if a && b && c {
		return COMPLETED
	}
	return INITIATED
}

func getPassword(cmd *cobra.Command, cliArgName, envVarName string) (string, error) {
	if cmd.Flags().Changed(cliArgName) {
		return cmd.Flag(cliArgName).Value.String(), nil
	}
	if os.Getenv(envVarName) != "" {
		return os.Getenv(envVarName), nil
	}
	fmt.Printf("Password to connect to %s (In addition, you can also set the password using the environment variable `%s`): ",
		strings.TrimSuffix(cliArgName, "-password"), envVarName)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	fmt.Print("\n")
	return string(bytePassword), nil
}

func addHeader(table *uitable.Table, cols ...string) {
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()
	columns := lo.Map(cols, func(col string, _ int) interface{} {
		return headerfmt(col)
	})
	table.AddRow(columns...)
}

func GetSourceDBTypeFromMSR() string {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}
	if msr == nil {
		utils.ErrExit("migration status record not found")
	}
	return msr.SourceDBConf.DBType
}

func validateMetaDBCreated() {
	if !metaDBIsCreated(exportDir) {
		utils.ErrExit("ERROR no metadb found in export-dir")
	}
}

func getImportTableList(sourceTableList []string) ([]sqlname.NameTuple, error) {
	if importerRole == IMPORT_FILE_ROLE {
		return nil, nil
	}
	var tableList []sqlname.NameTuple
	sqlname.SourceDBType = source.DBType
	for _, qualifiedTableName := range sourceTableList {
		table, err := namereg.NameReg.LookupTableName(qualifiedTableName)
		if err != nil {
			return nil, fmt.Errorf("lookup table %s in name registry : %v", qualifiedTableName, err)
		}
		tableList = append(tableList, table)
	}
	return tableList, nil
}

func hideImportFlagsInFallForwardOrBackCmds(cmd *cobra.Command) {
	var flags = []string{"target-db-type", "import-type", "target-endpoints", "use-public-ip", "continue-on-error", "table-list",
		"table-list-file-path", "exclude-table-list", "exclude-table-list-file-path", "enable-upsert"}
	for _, flagName := range flags {
		flag := cmd.Flags().Lookup(flagName)
		if flag != nil {
			flag.Hidden = true
		}
	}
}

func hideExportFlagsInFallForwardOrBackCmds(cmd *cobra.Command) {
	var flags = []string{"source-db-type", "export-type", "parallel-jobs", "start-clean", "allow-oracle-clob-data-export"}
	for _, flagName := range flags {
		flag := cmd.Flags().Lookup(flagName)
		if flag != nil {
			flag.Hidden = true
		}
	}
}

func GetDefaultPGSchema(schema string, separator string) (string, bool) {
	// second return value is true if public is not included in the schema
	// which indicates that the no default schema
	schemas := strings.Split(schema, separator)
	if len(schemas) == 1 {
		return schema, false
	} else if slices.Contains(schemas, "public") {
		return "public", false
	} else {
		return "", true
	}
}

func CleanupChildProcesses() {
	myPid := os.Getpid()
	processes, err := ps.Processes()
	if err != nil {
		log.Errorf("failed to read processes: %v", err)
	}
	childProcesses := lo.Filter(processes, func(process ps.Process, _ int) bool {
		return process.PPid() == myPid
	})
	if len(childProcesses) > 0 {
		log.Info("Cleaning up child processes...")
	}
	for _, process := range childProcesses {
		pid := process.Pid()
		log.Infof("shutting down child pid %d", pid)
		err := ShutdownProcess(pid, 60)
		if err != nil {
			log.Errorf("shut down child pid %d : %v", pid, err)
		} else {
			log.Infof("shut down child pid %d", pid)
		}
	}
	PrintElapsedDuration()
}

// this function wait for process to exit after signalling it to stop
func ShutdownProcess(pid int, forceShutdownAfterSeconds int) error {
	err := signalProcess(pid, syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("send sigterm to %d: %v", pid, err)
	}
	waitForProcessToExit(pid, forceShutdownAfterSeconds)
	return nil
}

func signalProcess(pid int, signal syscall.Signal) error {
	process, _ := os.FindProcess(pid) // Always succeeds on Unix systems
	log.Infof("sending signal=%s to process with PID=%d", signal.String(), pid)
	err := process.Signal(signal)
	if err != nil {
		return fmt.Errorf("sending signal=%s signal to process with PID=%d: %w", signal.String(), pid, err)
	}

	return nil
}

func waitForProcessToExit(pid int, forceShutdownAfterSeconds int) {
	// Reference: https://mezhenskyi.dev/posts/go-linux-processes/
	// Poll every 2 sec to make sure process is stopped
	// here process.Signal(syscall.Signal(0)) will return error only if process is not running
	process, _ := os.FindProcess(pid) // Always succeeds on Unix systems
	secondsWaited := 0
	for {
		time.Sleep(time.Second * 2)
		secondsWaited += 2
		err := process.Signal(syscall.Signal(0))
		if err != nil {
			log.Infof("process with PID=%d is stopped", process.Pid)
			return
		}
		if forceShutdownAfterSeconds > 0 && secondsWaited > forceShutdownAfterSeconds {
			log.Infof("force shutting down pid %d", pid)
			process.Signal(syscall.SIGKILL)
		}
	}
}

func initBaseSourceEvent(bev *cp.BaseEvent, eventType string) {
	*bev = cp.BaseEvent{
		EventType:     eventType,
		MigrationUUID: migrationUUID,
		DBType:        source.DBType,
		DatabaseName:  source.DBName,
		SchemaNames:   cp.GetSchemaList(source.Schema),
		DBIP:          utils.LookupIP(source.Host),
		Port:          source.Port,
		DBVersion:     source.DBVersion,
	}
}

func initBaseTargetEvent(bev *cp.BaseEvent, eventType string) {
	*bev = cp.BaseEvent{
		EventType:     eventType,
		MigrationUUID: migrationUUID,
		DBType:        tconf.TargetDBType,
		DatabaseName:  tconf.DBName,
		SchemaNames:   []string{tconf.Schema},
		DBIP:          utils.LookupIP(tconf.Host),
		Port:          tconf.Port,
		DBVersion:     tconf.DBVersion,
	}
}

func renameTableIfRequired(table string) (string, bool) {
	// required to rename the table name from leaf to root partition in case of pg_dump
	// to be load data in target using via root table
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %s", err)
	}

	if msr == nil || msr.SourceDBConf == nil { // this shouldn't hit in migration flow, adding just to avoid nil pointer dereference error
		return table, false
	}

	sourceDBType = msr.SourceDBConf.DBType
	sourceDBTypeInMigration := msr.SourceDBConf.DBType
	schema := msr.SourceDBConf.Schema
	sqlname.SourceDBType = source.DBType
	if source.DBType != POSTGRESQL && source.DBType != YUGABYTEDB {
		return table, false
	}
	if source.DBType == POSTGRESQL && msr.SourceRenameTablesMap == nil ||
		source.DBType == YUGABYTEDB && msr.TargetRenameTablesMap == nil {
		return table, false
	}
	if sourceDBTypeInMigration != POSTGRESQL && source.DBType == YUGABYTEDB {
		schema = source.Schema
	}
	renameTablesMap := msr.SourceRenameTablesMap
	if source.DBType == YUGABYTEDB {
		renameTablesMap = msr.TargetRenameTablesMap
	}
	defaultSchema, noDefaultSchema := GetDefaultPGSchema(schema, "|")
	if noDefaultSchema && len(strings.Split(table, ".")) <= 1 {
		utils.ErrExit("no default schema found to qualify table: %s", table)
	}
	tableName := sqlname.NewSourceNameFromMaybeQualifiedName(table, defaultSchema)
	fromTable := tableName.Qualified.Unquoted

	if renameTablesMap[fromTable] != "" {
		tableTup, err := namereg.NameReg.LookupTableName(renameTablesMap[fromTable])
		if err != nil {
			utils.ErrExit("lookup failed for the table:  %s", renameTablesMap[fromTable])
		}

		return tableTup.ForMinOutput(), true
	}
	return table, false
}

// TODO: ideally original function renameTableIfRequired should be made to return NameTuple instead of string
// but that will require a lot of changes in the codebase. So, keeping this function as a wrapper to do same but return Tuple
func getRenamedTableTuple(table sqlname.NameTuple) (sqlname.NameTuple, bool) {
	renamedTable, isRenamed := renameTableIfRequired(table.ForKey())
	// no need to lookup the same table
	if !isRenamed {
		return table, false
	}

	tableTuple, err := namereg.NameReg.LookupTableName(renamedTable)
	if err != nil {
		utils.ErrExit("lookup table %s in name registry : %v", renamedTable, err)
	}
	return tableTuple, isRenamed
}

func getExportedSnapshotRowsMap(exportSnapshotStatus *ExportSnapshotStatus) (*utils.StructMap[sqlname.NameTuple, int64], *utils.StructMap[sqlname.NameTuple, []string], error) {
	snapshotRowsMap := utils.NewStructMap[sqlname.NameTuple, int64]()
	snapshotStatusMap := utils.NewStructMap[sqlname.NameTuple, []string]()

	for _, tableStatus := range exportSnapshotStatus.Tables {
		nt, err := namereg.NameReg.LookupTableName(tableStatus.TableName)
		if err != nil {
			return nil, nil, fmt.Errorf("lookup table [%s] from name registry: %v", tableStatus.TableName, err)
		}
		existingSnapshotRows, _ := snapshotRowsMap.Get(nt)
		snapshotRowsMap.Put(nt, existingSnapshotRows+tableStatus.ExportedRowCountSnapshot)
		existingStatuses, _ := snapshotStatusMap.Get(nt)
		existingStatuses = append(existingStatuses, tableStatus.Status)
		snapshotStatusMap.Put(nt, existingStatuses)
	}

	return snapshotRowsMap, snapshotStatusMap, nil
}

func getImportedSnapshotRowsMap(dbType string) (*utils.StructMap[sqlname.NameTuple, RowCountPair], error) {
	switch dbType {
	case "target":
		importerRole = TARGET_DB_IMPORTER_ROLE
	case "target-file":
		importerRole = IMPORT_FILE_ROLE
	case "source-replica":
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
	}
	state := NewImportDataState(exportDir)
	var snapshotDataFileDescriptor *datafile.Descriptor

	if dataFileDescriptor != nil {
		// in case of import-data and import-data-file, import-data-to-source-replica,
		// the data file descriptor is already loaded in memory
		snapshotDataFileDescriptor = dataFileDescriptor
	} else {
		// get data-migration-report use-case where
		// we need to read the data file descriptor from export-dir
		dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
		if utils.FileOrFolderExists(dataFileDescriptorPath) {
			snapshotDataFileDescriptor = datafile.OpenDescriptor(exportDir)
		}
	}
	snapshotRowsMap := utils.NewStructMap[sqlname.NameTuple, RowCountPair]()
	nameTupleTodataFilesMap := utils.NewStructMap[sqlname.NameTuple, []string]()
	if snapshotDataFileDescriptor != nil {
		for _, fileEntry := range snapshotDataFileDescriptor.DataFileList {
			nt, err := namereg.NameReg.LookupTableName(fileEntry.TableName)
			if err != nil {
				return nil, fmt.Errorf("lookup table name from data file descriptor %s : %v", fileEntry.TableName, err)
			}
			list, ok := nameTupleTodataFilesMap.Get(nt)
			if !ok {
				list = []string{}
			}
			list = append(list, fileEntry.FilePath)
			nameTupleTodataFilesMap.Put(nt, list)
		}
	}

	err := nameTupleTodataFilesMap.IterKV(func(nt sqlname.NameTuple, dataFilePaths []string) (bool, error) {
		for _, dataFilePath := range dataFilePaths {
			importedRowCount, err := state.GetImportedRowCount(dataFilePath, nt)
			if err != nil {
				return false, fmt.Errorf("could not fetch imported row count for table %q: %w", nt, err)
			}
			erroredRowCount, err := state.GetErroredRowCount(dataFilePath, nt)
			if err != nil {
				return false, fmt.Errorf("could not fetch errored row count for table %q: %w", nt, err)
			}
			existingRowCountPair, _ := snapshotRowsMap.Get(nt)
			existingRowCountPair.Imported += importedRowCount
			existingRowCountPair.Errored += erroredRowCount
			snapshotRowsMap.Put(nt, existingRowCountPair)
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error getting row count of tables: %v", err)
	}
	return snapshotRowsMap, nil
}

func getImportedSizeMap() (*utils.StructMap[sqlname.NameTuple, int64], error) { //used for import data file case right now
	importerRole = IMPORT_FILE_ROLE
	state := NewImportDataState(exportDir)
	dataFileDescriptor, err := prepareDummyDescriptor(state)
	if err != nil {
		return nil, fmt.Errorf("prepare dummy descriptor: %w", err)
	}
	snapshotRowsMap := utils.NewStructMap[sqlname.NameTuple, int64]()
	for _, fileEntry := range dataFileDescriptor.DataFileList {
		nt, err := namereg.NameReg.LookupTableName(fileEntry.TableName)
		if err != nil {
			return nil, fmt.Errorf("lookup table name from data file descriptor %s : %v", fileEntry.TableName, err)
		}
		byteCount, err := state.GetImportedByteCount(fileEntry.FilePath, nt)
		if err != nil {
			return nil, fmt.Errorf("could not fetch snapshot row count for table %q: %w", nt, err)
		}
		exisitingByteCount, _ := snapshotRowsMap.Get(nt)
		snapshotRowsMap.Put(nt, exisitingByteCount+byteCount)
	}
	return snapshotRowsMap, nil
}

func storeTableListInMSR(tableList []sqlname.NameTuple) error {
	minQuotedTableList := lo.Uniq(lo.Map(tableList, func(table sqlname.NameTuple, _ int) string {
		// Store list of tables in MSR with root table in case of partitions
		renamedTable, isRenamed := renameTableIfRequired(table.ForOutput())
		if isRenamed {
			tuple, err := namereg.NameReg.LookupTableName(renamedTable)
			if err != nil {
				return fmt.Sprintf("lookup table %s in name registry : %v", renamedTable, err)
			}
			return tuple.ForOutput()
		}
		return renamedTable
	}))
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.TableListExportedFromSource = minQuotedTableList
		record.SourceExportedTableListWithLeafPartitions = lo.Map(tableList, func(t sqlname.NameTuple, _ int) string {
			return t.ForOutput()
		})
	})
	if err != nil {
		return fmt.Errorf("update migration status record: %v", err)
	}
	return nil
}

// =====================================================================

// TODO: consider merging all unsupported field with single AssessmentReport struct member as AssessmentIssue
// TODO: some of the fields have pointers, to be on safer side, we should convert all pointers fields to the actual type(store copy)
type AssessmentReport struct {
	VoyagerVersion                 string                                `json:"VoyagerVersion"`
	TargetDBVersion                *ybversion.YBVersion                  `json:"TargetDBVersion"`
	MigrationComplexity            string                                `json:"MigrationComplexity"`
	MigrationComplexityExplanation string                                `json:"MigrationComplexityExplanation"`
	SchemaSummary                  utils.SchemaSummary                   `json:"SchemaSummary"`
	Sizing                         *migassessment.SizingAssessmentReport `json:"Sizing"`
	Issues                         []AssessmentIssue                     `json:"AssessmentIssues"`
	TableIndexStats                *[]migassessment.TableIndexStats      `json:"TableIndexStats"`
	Notes                          []string                              `json:"Notes"`

	// fields going to be deprecated
	UnsupportedDataTypes       []utils.TableColumnsDataTypes     `json:"-"`
	UnsupportedDataTypesDesc   string                            `json:"-"`
	UnsupportedFeatures        []UnsupportedFeature              `json:"-"`
	UnsupportedFeaturesDesc    string                            `json:"-"`
	UnsupportedQueryConstructs []utils.UnsupportedQueryConstruct `json:"-"`
	UnsupportedPlPgSqlObjects  []UnsupportedFeature              `json:"-"`
	MigrationCaveats           []UnsupportedFeature              `json:"-"`
}

// Fields apart from Category, CategoryDescription, TypeName and Impact will be populated only if/when available
type AssessmentIssue struct {
	Category               string                          `json:"Category"` // expected values: unsupported_features, unsupported_query_constructs, migration_caveats, unsupported_plpgsql_objects, unsupported_datatype
	CategoryDescription    string                          `json:"CategoryDescription"`
	Type                   string                          `json:"Type"` // Ex: GIN_INDEXES, SECURITY_INVOKER_VIEWS, STORED_GENERATED_COLUMNS
	Name                   string                          `json:"Name"` // Ex: GIN Indexes, Security Invoker Views, Stored Generated Columns
	Description            string                          `json:"Description"`
	Impact                 string                          `json:"Impact"`     // // Level-1, Level-2, Level-3 (no default: need to be assigned for each issue)
	ObjectType             string                          `json:"ObjectType"` // For datatype category, ObjectType will be datatype (for eg "geometry")
	ObjectName             string                          `json:"ObjectName"`
	SqlStatement           string                          `json:"SqlStatement"`
	DocsLink               string                          `json:"DocsLink"`
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion `json:"MinimumVersionsFixedIn"` // key: series (2024.1, 2.21, etc)
	Details                map[string]interface{}          `json:"Details,omitempty"`
}

type UnsupportedFeature struct {
	FeatureName            string                          `json:"FeatureName"`
	Objects                []ObjectInfo                    `json:"Objects"`
	DisplayDDL             bool                            `json:"-"` // just used by html format to display the DDL for some feature and object names for other
	DocsLink               string                          `json:"DocsLink,omitempty"`
	FeatureDescription     string                          `json:"FeatureDescription,omitempty"`
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion `json:"MinimumVersionsFixedIn"` // key: series (2024.1, 2.21, etc)
}

type ObjectInfo struct {
	ObjectType   string `json:"ObjectType,omitempty"`
	ObjectName   string
	SqlStatement string
}

// ======================================================================
type BulkAssessmentReport struct {
	Details []AssessmentDetail `json:"Detail"`
	Notes   []string           `json:"Notes"`
}

type AssessmentDetail struct {
	Schema             string `json:"Schema"`
	DatabaseIdentifier string `json:"DatabaseIdentifier"`
	ReportPath         string `json:"ReportPath"`
	Status             string `json:"Status"`
}

type AssessMigrationDBConfig struct {
	DbType   string
	Host     string
	Port     string
	DbName   string
	SID      string
	TnsAlias string
	User     string
	Password string
	Schema   string
}

// =============== for yugabyted controlplane ==============//
// TODO: see if this can be accommodated in controlplane pkg, facing pkg cyclic dependency issue

/*
Version History
1.0: Introduced AssessmentIssue field for storing assessment issues in flattened format
1.1: Added TargetDBVersion and AssessmentIssueYugabyteD.MinimumVersionFixedIn
1.2: Syncing it with original AssessmentIssue(adding fields Category, CategoryDescription, Type, Name, Description, Impact, ObjectType) and MigrationComplexityExplanation;
1.3: Moved Sizing, TableIndexStats, Notes, fields out from depcreated AssessmentJsonReport field to top level struct
1.4: Removed field 'ParallelVoyagerJobs` from sizing recommendation
1.5: Changed type of the Details field from json.RawMessage to map[string]interface{}
*/
var ASSESS_MIGRATION_YBD_PAYLOAD_VERSION = "1.5"

// TODO: decouple this struct from utils.AnalyzeSchemaIssue struct, right now its tightly coupled;
// Similarly for migassessment.SizingAssessmentReport and migassessment.TableIndexStats
type AssessMigrationPayload struct {
	PayloadVersion                 string
	VoyagerVersion                 string
	TargetDBVersion                *ybversion.YBVersion
	MigrationComplexity            string
	MigrationComplexityExplanation string
	SchemaSummary                  utils.SchemaSummary
	AssessmentIssues               []AssessmentIssueYugabyteD
	SourceSizeDetails              SourceDBSizeDetails
	TargetRecommendations          TargetSizingRecommendations
	ConversionIssues               []utils.AnalyzeSchemaIssue
	Sizing                         *migassessment.SizingAssessmentReport
	TableIndexStats                *[]migassessment.TableIndexStats
	Notes                          []string
	// Depreacted: AssessmentJsonReport is deprecated; use the fields directly inside struct
	AssessmentJsonReport AssessmentReportYugabyteD
}

type AssessmentIssueYugabyteD struct {
	Category               string                          `json:"Category"` // expected values: unsupported_features, unsupported_query_constructs, migration_caveats, unsupported_plpgsql_objects, unsupported_datatype
	CategoryDescription    string                          `json:"CategoryDescription"`
	Type                   string                          `json:"Type"`                   // Ex: GIN_INDEXES, SECURITY_INVOKER_VIEWS, STORED_GENERATED_COLUMNS
	Name                   string                          `json:"Name"`                   // Ex: GIN Indexes, Security Invoker Views, Stored Generated Columns
	Description            string                          `json:"Description"`            // description based on type/name
	Impact                 string                          `json:"Impact"`                 // Level-1, Level-2, Level-3 (no default: need to be assigned for each issue)
	ObjectType             string                          `json:"ObjectType"`             // For datatype category, ObjectType will be datatype (for eg "geometry")
	ObjectName             string                          `json:"ObjectName"`             // Fully qualified object name(empty if NA, eg UQC)
	SqlStatement           string                          `json:"SqlStatement"`           // DDL or DML(UQC)
	DocsLink               string                          `json:"DocsLink"`               // docs link based on the subtype
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion `json:"MinimumVersionsFixedIn"` // key: series (2024.1, 2.21, etc)

	// Store Type-specific details - extensible, can refer any struct
	Details map[string]interface{} `json:"Details,omitempty"`
}

// To be deprecated in future
type AssessmentReportYugabyteD struct {
	VoyagerVersion             string                                `json:"VoyagerVersion"`
	TargetDBVersion            *ybversion.YBVersion                  `json:"TargetDBVersion"`
	MigrationComplexity        string                                `json:"MigrationComplexity"`
	SchemaSummary              utils.SchemaSummary                   `json:"SchemaSummary"`
	Sizing                     *migassessment.SizingAssessmentReport `json:"Sizing"`
	TableIndexStats            *[]migassessment.TableIndexStats      `json:"TableIndexStats"`
	Notes                      []string                              `json:"Notes"`
	UnsupportedDataTypes       []utils.TableColumnsDataTypes         `json:"UnsupportedDataTypes"` // using utils.TableColumnsDataTypes struct for ybd since it's unlikely to change rather removed
	UnsupportedDataTypesDesc   string                                `json:"UnsupportedDataTypesDesc"`
	UnsupportedFeatures        []UnsupportedFeature                  `json:"UnsupportedFeatures"` // using UnsupportedFeature struct for ybd since it's unlikely to change rather removed
	UnsupportedFeaturesDesc    string                                `json:"UnsupportedFeaturesDesc"`
	UnsupportedQueryConstructs []utils.UnsupportedQueryConstruct     `json:"UnsupportedQueryConstructs"`
	UnsupportedPlPgSqlObjects  []UnsupportedFeature                  `json:"UnsupportedPlPgSqlObjects"`
	MigrationCaveats           []UnsupportedFeature                  `json:"MigrationCaveats"`
}

// RowCountPair holds imported and errored row counts for a table.
type RowCountPair struct {
	Imported int64
	Errored  int64
}

/*
	Sample of extensibility

	type QueryConstuctDetails struct {
		FunctionNames	[]string
		ColumnNames		[]string
	}
*/

type SourceDBSizeDetails struct {
	TotalDBSize        int64
	TotalTableSize     int64
	TotalIndexSize     int64
	TotalTableRowCount int64
}

type TargetSizingRecommendations struct {
	TotalColocatedSize int64
	TotalShardedSize   int64
}

//====== AssesmentReport struct methods ======//

func ParseJSONToAssessmentReport(reportPath string) (*AssessmentReport, error) {
	if !utils.FileOrFolderExists(reportPath) {
		return nil, fmt.Errorf("report file %q does not exist", reportPath)
	}

	var report AssessmentReport
	err := jsonfile.NewJsonFile[AssessmentReport](reportPath).Load(&report)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json report file %q: %w", reportPath, err)
	}

	return &report, nil
}

func (ar *AssessmentReport) AppendIssues(issues ...AssessmentIssue) {
	ar.Issues = append(ar.Issues, issues...)
}

func (ar *AssessmentReport) GetShardedTablesRecommendation() ([]string, error) {
	if ar.Sizing == nil {
		return nil, fmt.Errorf("sizing report is null, can't fetch sharded tables")
	}

	return ar.Sizing.SizingRecommendation.ShardedTables, nil
}

func (ar *AssessmentReport) GetColocatedTablesRecommendation() ([]string, error) {
	if ar.Sizing == nil {
		return nil, fmt.Errorf("sizing report is null, can't fetch colocated tables")
	}

	return ar.Sizing.SizingRecommendation.ColocatedTables, nil
}

func (ar *AssessmentReport) GetClusterSizingRecommendation() string {
	if ar.Sizing == nil {
		return ""
	}

	if ar.Sizing.FailureReasoning != "" {
		return ar.Sizing.FailureReasoning
	}

	return fmt.Sprintf("Num Nodes: %f, vCPU per instance: %d, Memory per instance: %d, Estimated Import Time: %f minutes",
		ar.Sizing.SizingRecommendation.NumNodes, ar.Sizing.SizingRecommendation.VCPUsPerInstance,
		ar.Sizing.SizingRecommendation.MemoryPerInstance, ar.Sizing.SizingRecommendation.EstimatedTimeInMinForImport)
}

func (ar *AssessmentReport) GetTotalTableRowCount() int64 {
	if ar.TableIndexStats == nil {
		return -1
	}

	var totalTableRowCount int64
	for _, stat := range ar.getTableStats() {
		totalTableRowCount += utils.SafeDereferenceInt64(stat.RowCount)
	}
	return totalTableRowCount
}

func (ar *AssessmentReport) GetTotalTableSize() int64 {
	if ar.TableIndexStats == nil {
		return -1
	}

	var totalTableSize int64
	for _, stat := range ar.getTableStats() {
		totalTableSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
	}
	return totalTableSize
}

func (ar *AssessmentReport) GetTotalIndexSize() int64 {
	if ar.TableIndexStats == nil {
		return -1
	}

	var totalIndexSize int64
	for _, stat := range ar.getIndexStats() {
		totalIndexSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
	}
	return totalIndexSize
}

func (ar *AssessmentReport) GetTotalColocatedSize(dbType string) (int64, error) {
	if ar.TableIndexStats == nil {
		return -1, nil
	}

	colocatedTables, err := ar.GetColocatedTablesRecommendation()
	if err != nil {
		return -1, fmt.Errorf("failed to get the colocated tables recommendation: %w", err)
	}

	var totalColocatedSize int64
	for _, stat := range ar.getTableStats() {
		var tableName string
		switch dbType {
		case ORACLE:
			tableName = stat.ObjectName // in case of oracle, colocatedTables have unqualified table names
		case POSTGRESQL:
			tableName = fmt.Sprintf("%s.%s", stat.SchemaName, stat.ObjectName)
		default:
			return -1, fmt.Errorf("dbType %s is not yet supported for calculating size details", dbType)
		}

		if slices.Contains(colocatedTables, tableName) {
			totalColocatedSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
		}
	}

	return totalColocatedSize, nil
}

func (ar *AssessmentReport) GetTotalShardedSize(dbType string) (int64, error) {
	if ar.TableIndexStats == nil {
		return -1, nil
	}

	shardedTables, err := ar.GetShardedTablesRecommendation()
	if err != nil {
		return -1, fmt.Errorf("failed to get the sharded tables recommendation: %w", err)
	}

	var totalShardedSize int64
	for _, stat := range ar.getTableStats() {
		var tableName string
		switch dbType {
		case ORACLE:
			tableName = stat.ObjectName // in case of oracle, shardedTables have unqualified table names
		case POSTGRESQL:
			tableName = fmt.Sprintf("%s.%s", stat.SchemaName, stat.ObjectName)
		default:
			return -1, fmt.Errorf("dbType %s is not yet supported for calculating size details", dbType)
		}

		if slices.Contains(shardedTables, tableName) {
			totalShardedSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
		}
	}

	return totalShardedSize, nil
}

func (ar *AssessmentReport) getTableStats() []*migassessment.TableIndexStats {
	var res []*migassessment.TableIndexStats
	for _, stat := range *ar.TableIndexStats {
		if !stat.IsIndex {
			res = append(res, &stat)
		}
	}
	return res
}

func (ar *AssessmentReport) getIndexStats() []*migassessment.TableIndexStats {
	var res []*migassessment.TableIndexStats
	for _, stat := range *ar.TableIndexStats {
		if stat.IsIndex {
			res = append(res, &stat)
		}
	}
	return res
}

func GetJsonAssessmentReportPath() string {
	return filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME))
}

// ===== AssessMigrationDBConfig struct methods =====
func (dbConfig *AssessMigrationDBConfig) GetDatabaseIdentifier() string {
	switch {
	case dbConfig.TnsAlias != "":
		return dbConfig.TnsAlias
	case dbConfig.DbName != "":
		return dbConfig.DbName
	case dbConfig.SID != "":
		return dbConfig.SID
	default:
		return ""
	}
}

func (dbConfig *AssessMigrationDBConfig) GetSchemaIdentifier() string {
	identifier := dbConfig.GetDatabaseIdentifier()
	return fmt.Sprintf("%s-%s", identifier, dbConfig.Schema)
}

func (dbConfig *AssessMigrationDBConfig) GetAssessmentExportDirPath() string {
	return fmt.Sprintf("%s/%s-%s-export-dir", bulkAssessmentDir, dbConfig.GetDatabaseIdentifier(), dbConfig.Schema)
}

func (dbConfig *AssessMigrationDBConfig) GetHtmlAssessmentReportPath() string {
	exportDir := dbConfig.GetAssessmentExportDirPath()
	return filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, HTML_EXTENSION))
}

func (dbConfig *AssessMigrationDBConfig) GetJsonAssessmentReportPath() string {
	exportDir := dbConfig.GetAssessmentExportDirPath()
	return filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, JSON_EXTENSION))
}

// path to the assessment report without extension(like .json or .html).
// example: bulkAssessmentDir/*_export-dir/assessment/reports/migration_assessment_report
func (dbConfig *AssessMigrationDBConfig) GetAssessmentReportBasePath() string {
	exportDir := dbConfig.GetAssessmentExportDirPath()
	return filepath.Join(exportDir, "assessment", "reports", ASSESSMENT_FILE_NAME)
}

func (dbConfig *AssessMigrationDBConfig) GetAssessmentLogFilePath() string {
	return fmt.Sprintf("%s/logs/yb-voyager-assess-migration.log", dbConfig.GetAssessmentExportDirPath())
}

// ==========================================================================

func shouldSendCallhome() bool {
	//here checking the startTime is initialised or not as can be seen in root.go we
	//initialise startTime once we have logging setup after all initial checks
	//e.g. if someone hasn't provided the correct export-dir because of which initialisation step
	//isn't run and command fails with export-dir doesn't exists and we send payload on exits as well
	//so in payload we were having start-time as uninitialised ("0001-01-01 00:00:00") and time taken (9223372037)
	//was getting calculated as very big value so the callhome was not accepting the request due large value in int column
	//psycopg2.errors.NumericValueOutOfRange: value "9223372037" is out of range for type integer
	uninitialisedTimestamp := time.Time{}
	return bool(callhome.SendDiagnostics) && !startTime.Equal(uninitialisedTimestamp)
}

func createCallhomePayload() callhome.Payload {
	var payload callhome.Payload
	payload.MigrationUUID = migrationUUID
	payload.PhaseStartTime = startTime.UTC().Format("2006-01-02 15:04:05.999999")
	payload.YBVoyagerVersion = utils.YB_VOYAGER_VERSION
	payload.TimeTakenSec = int(math.Ceil(time.Since(startTime).Seconds()))
	payload.CollectedAt = time.Now().UTC().Format("2006-01-02 15:04:05.999999")

	return payload
}

func PackAndSendCallhomePayloadOnExit() {
	if callHomeErrorOrCompletePayloadSent {
		return
	}

	var exitErr error
	var status string
	if utils.ErrExitErr != nil {
		exitErr = utils.ErrExitErr
		status = ERROR
	} else {
		status = EXIT
	}

	switch currentCommand {
	case assessMigrationCmd.CommandPath():
		packAndSendAssessMigrationPayload(status, exitErr)
	case assessMigrationBulkCmd.CommandPath():
		packAndSendAssessMigrationBulkPayload(status, exitErr)
	case exportSchemaCmd.CommandPath():
		packAndSendExportSchemaPayload(status, exitErr)
	case analyzeSchemaCmd.CommandPath():
		packAndSendAnalyzeSchemaPayload(status, exitErr)
	case importSchemaCmd.CommandPath():
		packAndSendImportSchemaPayload(status, exitErr)
	case exportDataCmd.CommandPath(), exportDataFromSrcCmd.CommandPath():
		packAndSendExportDataPayload(status, exitErr)
	case exportDataFromTargetCmd.CommandPath():
		packAndSendExportDataFromTargetPayload(status, exitErr)
	case importDataCmd.CommandPath(), importDataToTargetCmd.CommandPath():
		packAndSendImportDataToTargetPayload(status, exitErr)
	case importDataToSourceCmd.CommandPath():
		packAndSendImportDataToSourcePayload(status, exitErr)
	case importDataToSourceReplicaCmd.CommandPath():
		packAndSendImportDataToSrcReplicaPayload(status, exitErr)
	case endMigrationCmd.CommandPath():
		packAndSendEndMigrationPayload(status, exitErr)
	case importDataFileCmd.CommandPath():
		packAndSendImportDataFilePayload(status, exitErr)
	}
}

func updateExportSnapshotDataStatsInPayload(exportDataPayload *callhome.ExportDataPhasePayload) {
	//Updating the payload with totalRows and LargestTableRows for both debezium/non-debezium case
	if !useDebezium || (changeStreamingIsEnabled(exportType) && source.DBType == POSTGRESQL) {
		//non-debezium and pg live migration snapshot case reading the export_snapshot_status.json file
		if exportSnapshotStatusFile != nil {
			exportStatusSnapshot, err := exportSnapshotStatusFile.Read()
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					log.Infof("callhome: failed to read export status file: %v", err)
				}
			} else {
				exportedSnapshotRow, _, err := getExportedSnapshotRowsMap(exportStatusSnapshot)
				if err != nil {
					log.Infof("callhome: error while getting exported snapshot rows map: %v", err)
				}
				exportedSnapshotRow.IterKV(func(key sqlname.NameTuple, value int64) (bool, error) {
					exportDataPayload.TotalRows += value
					if value >= exportDataPayload.LargestTableRows {
						exportDataPayload.LargestTableRows = value
					}
					return true, nil
				})
			}
		}
		switch source.DBType {
		case POSTGRESQL:
			exportDataPayload.ExportSnapshotMechanism = "pg_dump"
		case ORACLE, MYSQL:
			exportDataPayload.ExportSnapshotMechanism = "ora2pg"
		}
	} else {
		//debezium case reading export_status.json file
		exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
		dbzmStatus, err := dbzm.ReadExportStatus(exportStatusFilePath)
		if err != nil {
			log.Infof("callhome: error in reading export status: %v", err)
		}
		if dbzmStatus != nil {
			for _, tableExportStatus := range dbzmStatus.Tables {
				exportDataPayload.TotalRows += tableExportStatus.ExportedRowCountSnapshot
				if tableExportStatus.ExportedRowCountSnapshot > exportDataPayload.LargestTableRows {
					exportDataPayload.LargestTableRows = tableExportStatus.ExportedRowCountSnapshot
				}
			}
		}
		exportDataPayload.ExportSnapshotMechanism = "debezium"
	}
}

func sendCallhomePayloadAtIntervals() {
	for {
		if callHomeErrorOrCompletePayloadSent {
			//for just that corner case if there is some timing clash where complete and in-progress payload are sent together
			break
		}
		time.Sleep(15 * time.Minute)
		switch currentCommand {
		case exportDataCmd.CommandPath(), exportDataFromSrcCmd.CommandPath():
			packAndSendExportDataPayload(INPROGRESS, nil)
		case exportDataFromTargetCmd.CommandPath():
			packAndSendExportDataFromTargetPayload(INPROGRESS, nil)
		case importDataCmd.CommandPath(), importDataToTargetCmd.CommandPath():
			packAndSendImportDataToTargetPayload(INPROGRESS, nil)
		case importDataToSourceCmd.CommandPath():
			packAndSendImportDataToSourcePayload(INPROGRESS, nil)
		case importDataToSourceReplicaCmd.CommandPath():
			packAndSendImportDataToSrcReplicaPayload(INPROGRESS, nil)
		case importDataFileCmd.CommandPath():
			packAndSendImportDataFilePayload(INPROGRESS, nil)
		}
	}
}

// Adding it here instead of utils package to avoid circular dependency issues
func ParseJsonToAnalyzeSchemaReport(reportPath string) (*utils.SchemaReport, error) {
	if !utils.FileOrFolderExists(reportPath) {
		return nil, fmt.Errorf("report file %q does not exist", reportPath)
	}

	var report utils.SchemaReport
	err := jsonfile.NewJsonFile[utils.SchemaReport](reportPath).Load(&report)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json report file %q: %w", reportPath, err)
	}

	return &report, nil
}
