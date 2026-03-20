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
package exportdata

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	goerrors "github.com/go-errors/errors"
	"github.com/gosuri/uitable"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// SOURCE_DB_EXPORTER_ROLE is re-exported from constants for internal use.
const SOURCE_DB_EXPORTER_ROLE = constants.SOURCE_DB_EXPORTER_ROLE

// =====================================================================================
// Table approx row count
// =====================================================================================

func updateTableApproxRowCount(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	utils.PrintAndLogf("calculating approx num of rows to export for each table...")
	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	for _, key := range sortedKeys {
		approxRowCount := source.DB().GetTableApproxRowCount(tablesProgressMetadata[key].TableName)
		tablesProgressMetadata[key].CountTotalRows = approxRowCount
	}
	log.Tracef("After updating total approx row count, TablesProgressMetadata: %+v", tablesProgressMetadata)
}

// =====================================================================================
// Rename / partition helpers
// =====================================================================================

func RenameTableIfRequired(metaDB *metadb.MetaDB, source *srcdb.Source, table string) (string, bool) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("Failed to get migration status record: %s", err)
	}

	if msr == nil || msr.SourceDBConf == nil {
		return table, false
	}

	sourceDBTypeInMigration := msr.SourceDBConf.DBType
	schema := msr.SourceDBConf.Schemas
	sqlname.SourceDBType = source.DBType
	if source.DBType != constants.POSTGRESQL && source.DBType != constants.YUGABYTEDB {
		return table, false
	}
	if source.DBType == constants.POSTGRESQL && msr.SourceRenameTablesMap == nil ||
		source.DBType == constants.YUGABYTEDB && msr.TargetRenameTablesMap == nil {
		return table, false
	}
	if sourceDBTypeInMigration != constants.POSTGRESQL && source.DBType == constants.YUGABYTEDB {
		schema = source.Schemas
	}
	renameTablesMap := msr.SourceRenameTablesMap
	if source.DBType == constants.YUGABYTEDB {
		renameTablesMap = msr.TargetRenameTablesMap
	}
	defaultSchema, noDefaultSchema := GetDefaultPGSchema(schema)
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

func GetRenamedTableTuple(metaDB *metadb.MetaDB, source *srcdb.Source, table sqlname.NameTuple) (sqlname.NameTuple, bool) {
	renamedTable, isRenamed := RenameTableIfRequired(metaDB, source, table.ForKey())
	if !isRenamed {
		return table, false
	}
	renamedTableTup, err := namereg.NameReg.LookupTableName(renamedTable)
	if err != nil {
		utils.ErrExit("lookup failed for the table: %s", renamedTable)
	}
	return renamedTableTup, true
}

func GetDefaultPGSchema(schema []sqlname.Identifier) (string, bool) {
	if len(schema) == 1 {
		return schema[0].MinQuoted, false
	} else if lo.ContainsBy(schema, func(s sqlname.Identifier) bool { return s.MinQuoted == "public" }) {
		return "public", false
	} else {
		return "", true
	}
}

func getQuotedFromUnquoted(t string) string {
	parts := strings.Split(t, ".")
	s, t := parts[0], parts[1]
	return fmt.Sprintf(`"%s"."%s"`, s, t)
}

// =====================================================================================
// Store table list in MSR
// =====================================================================================

func StoreTableListInMSR(metaDB *metadb.MetaDB, source *srcdb.Source, tableList []sqlname.NameTuple) error {
	minQuotedTableList := lo.Uniq(lo.Map(tableList, func(table sqlname.NameTuple, _ int) string {
		renamedTable, isRenamed := RenameTableIfRequired(metaDB, source, table.ForOutput())
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
		return goerrors.Errorf("update migration status record: %v", err)
	}
	return nil
}

// =====================================================================================
// Update file paths
// =====================================================================================

func UpdateFilePaths(source *srcdb.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	var requiredMap map[string]string

	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(filepath.Join(exportDir, "data"), source.DBVersion, false)
		for _, key := range sortedKeys {
			tableName := tablesProgressMetadata[key].TableName
			fullTableName := tableName.ForKey()
			table := tableName.ForMinOutput()
			if _, ok := requiredMap[fullTableName]; ok {
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
	log.Info(logMsg)
}

func getMappingForTableNameVsTableFileName(dataDirPath string, sourceDBVersion string, noWait bool) map[string]string {
	tocTextFilePath := filepath.Join(dataDirPath, "toc.txt")
	if noWait && !utils.FileOrFolderExists(tocTextFilePath) {
		return nil
	}
	for !utils.FileOrFolderExists(tocTextFilePath) {
		time.Sleep(time.Second * 1)
	}

	pgRestorePath, binaryCheckIssue, err := srcdb.GetAbsPathOfPGCommandAboveVersion("pg_restore", sourceDBVersion)
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
		parts := strings.Split(line, " ")

		if len(parts) < 8 {
			continue
		} else if parts[3] == "TABLE" && parts[4] == "DATA" {
			fileName := strings.Trim(parts[0], ";") + ".dat"
			schemaName := parts[5]
			tableName := parts[6]
			if nameContainsCapitalLetter(tableName) || sqlname.IsReservedKeywordPG(tableName) {
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

	os.WriteFile(filepath.Join(dataDirPath, "postdata.sql"), []byte(sequencesPostData.String()), 0644)
	return tableNameVsFileNameMap
}

func nameContainsCapitalLetter(name string) bool {
	for _, c := range name {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}

// =====================================================================================
// Rename data file descriptor
// =====================================================================================

func RenameDatafileDescriptor(metaDB *metadb.MetaDB, source *srcdb.Source, exportDir string) {
	datafileDescriptor := datafile.OpenDescriptor(exportDir)
	log.Infof("Parsed DataFileDescriptor: %v", spew.Sdump(datafileDescriptor))
	for _, fileEntry := range datafileDescriptor.DataFileList {
		renamedTable, isRenamed := RenameTableIfRequired(metaDB, source, fileEntry.TableName)
		if isRenamed {
			fileEntry.TableName = renamedTable
		}
	}
	for k, v := range datafileDescriptor.TableNameToExportedColumns {
		renamedTable, isRenamed := RenameTableIfRequired(metaDB, source, k)
		if isRenamed {
			datafileDescriptor.TableNameToExportedColumns[renamedTable] = v
			delete(datafileDescriptor.TableNameToExportedColumns, k)
		}
	}
	datafileDescriptor.Save()
}

// =====================================================================================
// Display exported row count
// =====================================================================================

func DisplayExportedRowCountSnapshot(metaDB *metadb.MetaDB, source *srcdb.Source, exportDir string, snapshotViaDebezium bool) {
	fmt.Printf("snapshot export report\n")
	uitbl := uitable.New()

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	leafPartitions := GetLeafPartitionsFromRootTable(metaDB)
	if !snapshotViaDebezium {
		exportedRowCount := getExportedRowCountSnapshot(exportDir)
		if len(source.Schemas) > 0 {
			addHeader(uitbl, "SCHEMA", "TABLE", "ROW COUNT")
		} else {
			addHeader(uitbl, "DATABASE", "TABLE", "ROW COUNT")
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
			partitions := leafPartitions[table.ForOutput()]
			if source.DBType == constants.POSTGRESQL && partitions != nil && msr.IsExportTableListSet {
				partitions := strings.Join(partitions, ", ")
				displayTableName = fmt.Sprintf("%s (%s)", table.CurrentName.Unqualified.MinQuoted, partitions)
			}
			schema := table.SourceName.SchemaName.MinQuoted
			uitbl.AddRow(schema, displayTableName, exportedRowCount[key])
		}
		if len(keys) > 0 {
			fmt.Print("\n")
			fmt.Println(uitbl)
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
				addHeader(uitbl, "SCHEMA", "TABLE", "ROW COUNT")
			} else {
				addHeader(uitbl, "DATABASE", "TABLE", "ROW COUNT")
			}
		}
		table, err := namereg.NameReg.LookupTableName(fmt.Sprintf("%s.%s", tableStatus.SchemaName, tableStatus.TableName))
		if err != nil {
			utils.ErrExit("lookup table  in name registry : %q: %v", tableStatus.TableName, err)
		}
		displayTableName := table.CurrentName.Unqualified.MinQuoted
		partitions := leafPartitions[table.ForOutput()]
		if source.DBType == constants.POSTGRESQL && partitions != nil {
			partitions := strings.Join(partitions, ", ")
			displayTableName = fmt.Sprintf("%s (%s)", table.CurrentName.Unqualified.MinQuoted, partitions)
		}
		schema := table.CurrentName.SchemaName.MinQuoted
		uitbl.AddRow(schema, displayTableName, tableStatus.ExportedRowCountSnapshot)

	}
	fmt.Print("\n")
	fmt.Println(uitbl)
	fmt.Print("\n")
}

func getExportedRowCountSnapshot(exportDir string) map[string]int64 {
	tableRowCount := map[string]int64{}
	for _, fileEntry := range datafile.OpenDescriptor(exportDir).DataFileList {
		tableRowCount[fileEntry.TableName] += fileEntry.RowCount
	}
	return tableRowCount
}

func GetLeafPartitionsFromRootTable(metaDB *metadb.MetaDB) map[string][]string {
	leafPartitions := make(map[string][]string)
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %v", err)
	}
	if msr.SourceDBConf.DBType != constants.POSTGRESQL {
		return leafPartitions
	}
	tables := msr.TableListExportedFromSource
	for leaf, root := range msr.SourceRenameTablesMap {
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
		leafPartitions[root] = append(leafPartitions[root], leaf)
	}

	return leafPartitions
}

func GetExportedSnapshotRowsMap(exportSnapshotStatus *ExportSnapshotStatus) (*utils.StructMap[sqlname.NameTuple, int64], *utils.StructMap[sqlname.NameTuple, []string], error) {
	snapshotRowsMap := utils.NewStructMap[sqlname.NameTuple, int64]()
	snapshotStatusMap := utils.NewStructMap[sqlname.NameTuple, []string]()

	for _, tableStatus := range exportSnapshotStatus.Tables {
		nt, err := namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableStatus.TableName)
		if err != nil {
			return nil, nil, goerrors.Errorf("lookup table [%s] from name registry: %v", tableStatus.TableName, err)
		}
		existingSnapshotRows, _ := snapshotRowsMap.Get(nt)
		snapshotRowsMap.Put(nt, existingSnapshotRows+tableStatus.ExportedRowCountSnapshot)
		existingStatuses, _ := snapshotStatusMap.Get(nt)
		existingStatuses = append(existingStatuses, tableStatus.Status)
		snapshotStatusMap.Put(nt, existingStatuses)
	}

	return snapshotRowsMap, snapshotStatusMap, nil
}

func addHeader(table *uitable.Table, cols ...string) {
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()
	columns := lo.Map(cols, func(col string, _ int) interface{} {
		return headerfmt(col)
	})
	table.AddRow(columns...)
}

// =====================================================================================
// Fetch or retrieve column-to-sequence map
// =====================================================================================

func FetchOrRetrieveColToSeqMap(metaDB *metadb.MetaDB, sourceDB srcdb.SourceDB, exporterRole string, startClean bool, msr *metadb.MigrationStatusRecord, tableList []sqlname.NameTuple) (map[string]string, error) {
	var storedColToSeqMap map[string]string
	switch exporterRole {
	case constants.SOURCE_DB_EXPORTER_ROLE:
		storedColToSeqMap = msr.SourceColumnToSequenceMapping
	case constants.TARGET_DB_EXPORTER_FB_ROLE, constants.TARGET_DB_EXPORTER_FF_ROLE:
		storedColToSeqMap = msr.TargetColumnToSequenceMapping
	}
	if storedColToSeqMap != nil && !startClean {
		return storedColToSeqMap, nil
	}
	colToSeqMap := sourceDB.GetColumnToSequenceMap(tableList)
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch exporterRole {
		case constants.SOURCE_DB_EXPORTER_ROLE:
			record.SourceColumnToSequenceMapping = colToSeqMap
		case constants.TARGET_DB_EXPORTER_FB_ROLE, constants.TARGET_DB_EXPORTER_FF_ROLE:
			record.TargetColumnToSequenceMapping = colToSeqMap
		}
	})
	if err != nil {
		return nil, fmt.Errorf("error in updating migration status record: %w", err)
	}
	return colToSeqMap, nil
}
