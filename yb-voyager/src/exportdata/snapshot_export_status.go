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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v8"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	pbreporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/pb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

//=====================================================================================

// For Non-debezium cases
type TableExportStatus struct {
	TableName                string `json:"table_name"` // table.Qualified.MinQuoted
	FileName                 string `json:"file_name"`
	Status                   string `json:"status"`
	ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
}

type ExportSnapshotStatus struct {
	Tables map[string]*TableExportStatus `json:"tables"`
}

func (e *ExportSnapshotStatus) GetTableStatusByTableName(tableName string) []*TableExportStatus {
	var tableStatus []*TableExportStatus
	for _, v := range e.Tables {
		if v.TableName == tableName {
			tableStatus = append(tableStatus, v)
		}
	}
	return tableStatus
}

func NewExportSnapshotStatus() *ExportSnapshotStatus {
	return &ExportSnapshotStatus{
		Tables: make(map[string]*TableExportStatus),
	}
}

//=====================================================================================

func InitializeExportTableMetadata(
	exportDir string,
	tableList []sqlname.NameTuple,
	getRenamedTableTupleFn func(sqlname.NameTuple) (sqlname.NameTuple, bool),
) (map[string]*utils.TableProgressMetadata, *jsonfile.JsonFile[ExportSnapshotStatus]) {

	tablesProgressMetadata := make(map[string]*utils.TableProgressMetadata)
	numTables := len(tableList)

	exportSnapshotStatusFilePath := filepath.Join(exportDir, "metainfo", "export_snapshot_status.json")
	exportSnapshotStatusFile := jsonfile.NewJsonFile[ExportSnapshotStatus](exportSnapshotStatusFilePath)

	exportSnapshotStatus := NewExportSnapshotStatus()

	for i := 0; i < numTables; i++ {
		tableName := tableList[i]
		key := tableName.ForKey()
		tablesProgressMetadata[key] = &utils.TableProgressMetadata{}

		tablesProgressMetadata[key].IsPartition = false
		tablesProgressMetadata[key].InProgressFilePath = ""
		tablesProgressMetadata[key].FinalFilePath = ""
		tablesProgressMetadata[key].CountTotalRows = int64(0)
		tablesProgressMetadata[key].CountLiveRows = int64(0)
		tablesProgressMetadata[key].Status = 0
		tablesProgressMetadata[key].FileOffsetToContinue = int64(0)
		tablesProgressMetadata[key].TableName = tableName
		exportSnapshotStatus.Tables[key] = &TableExportStatus{
			TableName:                key,
			FileName:                 "",
			Status:                   utils.TableMetadataStatusMap[tablesProgressMetadata[key].Status],
			ExportedRowCountSnapshot: int64(0),
		}

		renamedTableTup, isRenamed := getRenamedTableTupleFn(tableName)
		if isRenamed {
			tablesProgressMetadata[key].IsPartition = true
			tablesProgressMetadata[key].ParentTable = renamedTableTup
			exportSnapshotStatus.Tables[key].TableName = renamedTableTup.ForMinOutput()
		}
	}
	err := exportSnapshotStatusFile.Create(exportSnapshotStatus)
	if err != nil {
		utils.ErrExit("failed to create export status file: %w", err)
	}

	return tablesProgressMetadata, exportSnapshotStatusFile
}

func ExportDataStatus(
	ctx context.Context,
	tablesProgressMetadata map[string]*utils.TableProgressMetadata,
	quitChan, exportSuccessChan chan bool,
	disablePb bool,
	sourceDB srcdb.SourceDB,
	sourceDBType string,
	exporterRole string,
	controlPlane cp.ControlPlane,
	migrationUUID uuid.UUID,
	exportSnapshotStatusFile *jsonfile.JsonFile[ExportSnapshotStatus],
) {
	defer utils.WaitGroup.Done()
	go updateExportSnapshotStatus(ctx, tablesProgressMetadata, exportSnapshotStatusFile)

	// TODO: Figure out if we require quitChan2 (along with the entire goroutine below which updates quitChan).
	quitChan2 := make(chan bool)
	quit := false
	updateMetadataAndExit := false
	safeExit := false
	go func() {
		quit = <-quitChan2
		if quit {
			quitChan <- true
		}
	}()

	go func() {
		updateMetadataAndExit = <-exportSuccessChan
	}()

	numTables := len(tablesProgressMetadata)
	progressContainer := mpb.NewWithContext(ctx)

	doneCount := 0
	var exportedTables []string
	sortedKeys := utils.GetSortedKeys(tablesProgressMetadata)
	for doneCount < numTables && !quit {
		for _, key := range sortedKeys {
			if quit {
				break
			}
			if tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_NOT_STARTED && (utils.FileOrFolderExists(tablesProgressMetadata[key].InProgressFilePath) ||
				utils.FileOrFolderExists(tablesProgressMetadata[key].FinalFilePath)) {
				tablesProgressMetadata[key].Status = utils.TABLE_MIGRATION_IN_PROGRESS
				go startExportPB(progressContainer, key, quitChan2, disablePb, tablesProgressMetadata, sourceDB, sourceDBType, exporterRole, controlPlane, migrationUUID)
			} else if tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_DONE || (tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_NOT_STARTED && safeExit) {
				tablesProgressMetadata[key].Status = utils.TABLE_MIGRATION_COMPLETED
				exportedTables = append(exportedTables, key)
				doneCount++

				if exporterRole == constants.SOURCE_DB_EXPORTER_ROLE {
					exportDataTableMetrics := CreateUpdateExportedRowCountEventList([]string{key}, tablesProgressMetadata, migrationUUID)
					controlPlane.UpdateExportedRowCount(exportDataTableMetrics)
				}

				if doneCount == numTables {
					break
				}
			}

			if ctx.Err() != nil {
				fmt.Println(ctx.Err())
				break
			}
		}

		if ctx.Err() != nil {
			fmt.Println(ctx.Err())
			break
		}
		if updateMetadataAndExit {
			safeExit = true
			for _, key := range sortedKeys {
				if tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_IN_PROGRESS || tablesProgressMetadata[key].Status == utils.TABLE_MIGRATION_DONE {
					safeExit = false
					break
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	progressContainer.Wait()
	printExportedTables(exportedTables)
}

func startExportPB(
	progressContainer *mpb.Progress,
	mapKey string,
	quitChan chan bool,
	disablePb bool,
	tablesProgressMetadata map[string]*utils.TableProgressMetadata,
	sourceDB srcdb.SourceDB,
	sourceDBType string,
	exporterRole string,
	controlPlane cp.ControlPlane,
	migrationUUID uuid.UUID,
) {
	tableName := mapKey
	tableMetadata := tablesProgressMetadata[mapKey]

	pbr := pbreporter.NewExportPB(progressContainer, tableName, disablePb)
	pbr.SetTotalRowCount(tableMetadata.CountTotalRows, false)

	go func() {
		actualRowCount, err := sourceDB.GetTableRowCount(tableMetadata.TableName)
		if err != nil {
			log.Warnf("could not get actual row count for table=%s: %v", tableMetadata.TableName, err)
			return
		}
		log.Infof("Replacing actualRowCount=%d inplace of expectedRowCount=%d for table=%s",
			actualRowCount, tableMetadata.CountTotalRows, tableMetadata.TableName.ForUserQuery())
		pbr.SetTotalRowCount(actualRowCount, false)
		tableMetadata.CountTotalRows = actualRowCount
	}()

	tableDataFileName := tableMetadata.InProgressFilePath
	if utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
		tableDataFileName = tableMetadata.FinalFilePath
	}
	tableDataFile, err := os.Open(tableDataFileName)
	if err != nil {
		utils.PrintAndLogf("failed to open the data file %s for progress reporting: %q", tableDataFileName, err)
		quitChan <- true
		runtime.Goexit()
	}
	defer tableDataFile.Close()

	reader := bufio.NewReader(tableDataFile)

	go func() {
		for !pbr.IsComplete() {
			pbr.SetExportedRowCount(tableMetadata.CountLiveRows)
			time.Sleep(time.Millisecond * 500)

			if exporterRole == constants.SOURCE_DB_EXPORTER_ROLE {
				exportDataTableMetrics := CreateUpdateExportedRowCountEventList([]string{tableName}, tablesProgressMetadata, migrationUUID)
				controlPlane.UpdateExportedRowCount(exportDataTableMetrics)
			}
		}
	}()

	var line string
	insideCopyStmt := false
	prefix := ""

	readLines := func() {
		for {
			line, err = reader.ReadString('\n')
			if line != "" {
				if line[len(line)-1] == '\n' {
					if prefix != "" {
						line = prefix + line
						prefix = ""
					}
				} else {
					prefix = prefix + line
				}
			}
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				break
			} else if err != nil {
				utils.ErrExit("Error while reading file: %s: %v", tableDataFile.Name(), err)
			}
			if IsDataLine(line, sourceDBType, &insideCopyStmt) {
				tableMetadata.CountLiveRows += 1
			}
			if sourceDBType == "postgresql" && strings.HasPrefix(line, "\\.") {
				break
			}
		}
	}

	for !checkForEndOfFile(sourceDBType, tableMetadata, line) {
		readLines()
	}
	readLines()

	pbr.SetTotalRowCount(-1, true)
	tableMetadata.Status = utils.TABLE_MIGRATION_DONE
}

func updateExportSnapshotStatus(ctx context.Context, tablesProgressMetadata map[string]*utils.TableProgressMetadata, exportSnapshotStatusFile *jsonfile.JsonFile[ExportSnapshotStatus]) {
	updateTicker := time.NewTicker(1 * time.Second)
	defer updateTicker.Stop()
	for range updateTicker.C {
		select {
		case <-ctx.Done():
			return
		default:
			err := exportSnapshotStatusFile.Update(func(status *ExportSnapshotStatus) {
				for key := range tablesProgressMetadata {
					status.Tables[key].ExportedRowCountSnapshot = tablesProgressMetadata[key].CountLiveRows
					status.Tables[key].Status = utils.TableMetadataStatusMap[tablesProgressMetadata[key].Status]
					status.Tables[key].FileName = tablesProgressMetadata[key].FinalFilePath
				}
			})
			if err != nil {
				utils.ErrExit("failed to update export snapshot status: %w", err)
			}
		}
	}
}

func checkForEndOfFile(sourceDBType string, tableMetadata *utils.TableProgressMetadata, line string) bool {
	if sourceDBType == "postgresql" {
		if strings.HasPrefix(line, "\\.") {
			return true
		}
	} else if sourceDBType == "oracle" || sourceDBType == "mysql" {
		if !utils.FileOrFolderExists(tableMetadata.InProgressFilePath) && utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
			return true
		}
	}
	return false
}

func printExportedTables(exportedTables []string) {
	output := "Exported tables:- {"
	nt := len(exportedTables)
	for i := 0; i < nt; i++ {
		output += exportedTables[i]
		if i < nt-1 {
			output += ",  "
		}
	}
	output += "}"
	color.Yellow(output)
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

func IsDataLine(line string, sourceDBType string, insideCopyStmt *bool) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	if sourceDBType == "postgresql" {
		return !(emptyLine || newLineChar || endOfCopy)
	} else if sourceDBType == "oracle" || sourceDBType == "mysql" {
		if *insideCopyStmt {
			if endOfCopy {
				*insideCopyStmt = false
			}
			return !(emptyLine || newLineChar || endOfCopy)
		} else {
			if reCopy.MatchString(line) {
				*insideCopyStmt = true
			}
			return false
		}
	} else {
		panic("Invalid source db type")
	}
}

func CreateUpdateExportedRowCountEventList(tableNames []string, tablesProgressMetadata map[string]*utils.TableProgressMetadata, migrationUUID uuid.UUID) []*cp.UpdateExportedRowCountEvent {
	result := []*cp.UpdateExportedRowCountEvent{}
	var schemaName, tableName2 string

	for _, tableName := range tableNames {
		tableMetadata := tablesProgressMetadata[tableName]
		schemaName, tableName2 = tableMetadata.TableName.ForKeyTableSchema()
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
