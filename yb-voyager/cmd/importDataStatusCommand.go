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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)
var ffDBPassword string 

func validateFallforwardPassword(cmd *cobra.Command) {
	if cmd.Flags().Changed("ff-db-password") {
		return
	}
	if os.Getenv("FF_DB_PASSWORD") != "" {
		tconf.Password = os.Getenv("FF_DB_PASSWORD")
		return
	}
	fmt.Print("Password to connect to fall-forward DB:")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		utils.ErrExit("read password: %v", err)
		return
	}
	fmt.Print("\n")
	ffDBPassword = string(bytePassword)
}

var importDataStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print status of an ongoing/completed data import.",

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		checkWithStreamingMode()
		if withStreamingMode {
			validateTargetPassword(cmd)
			validateFallforwardPassword(cmd) //TODO: handle in a better way
			reInitialisingTarget()
		}
		color.Cyan("Import Data Status for TargetDB\n")
		err := runImportDataStatusCmd(targetDB, targetDBConf, false)
		if err != nil {
			utils.ErrExit("error: %s\n", err)
		}
		if ffDB != nil {
			color.Cyan("Import Data Status for fall-forward DB\n")
			err = runImportDataStatusCmd(ffDB, ffDBConf, true)
			if err != nil {
				utils.ErrExit("error: %s\n", err)
			}
		}
	},
}

func init() {
	importDataCmd.AddCommand(importDataStatusCmd)

	//TODO: handle this in a better way
	importDataStatusCmd.Flags().StringVar(&ffDBPassword, "ff-db-password", "",
		"password with which to connect to the target fall-forward DB server")

	importDataStatusCmd.Flags().StringVar(&tconf.Password, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server")
}

// totalCount and importedCount store row-count for import data command and byte-count for import data file command.
type tableMigStatusOutputRow struct {
	schemaName         string
	tableName          string
	fileName           string
	status             string
	totalCount         int64
	importedCount      int64
	percentageComplete float64
	totalEvents        int64
	numInserts         int64
	numUpdates         int64
	numDeletes         int64
	finalRowCount      int64
}


var targetDB tgtdb.TargetDB
var ffDB tgtdb.TargetDB
var targetDBConf tgtdb.TargetConf
var ffDBConf tgtdb.TargetConf


// Note that the `import data status` is running in a separate process. It won't have access to the in-memory state
// held in the main `import data` process.
func runImportDataStatusCmd(tgtdb tgtdb.TargetDB, tgtconf tgtdb.TargetConf, isffDB bool) error {
	exportDataDoneFlagFilePath := filepath.Join(exportDir, "metainfo/flags/exportDataDone")
	_, err := os.Stat(exportDataDoneFlagFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot run `import data status` before data export is done")
		}
		return fmt.Errorf("check if data export is done: %w", err)
	}

	table, err := prepareImportDataStatusTable(tgtdb, tgtconf, isffDB)
	if err != nil {
		return fmt.Errorf("prepare import data status table: %w", err)
	}
	uiTable := uitable.New()
	headerfmt := color.New(color.FgGreen, color.Underline).SprintFunc()
	for i, row := range table {
		perc := fmt.Sprintf("%.2f", row.percentageComplete)
		if reportProgressInBytes {
			if i == 0 {
				uiTable.AddRow(headerfmt("TABLE"), headerfmt("FILE"), headerfmt("STATUS"), headerfmt("TOTAL SIZE"), headerfmt("IMPORTED SIZE"), headerfmt("PERCENTAGE"))
			}
			// case of importDataFileCommand where file size is available not row counts
			totalCount := utils.HumanReadableByteCount(row.totalCount)
			importedCount := utils.HumanReadableByteCount(row.importedCount)
			uiTable.AddRow(row.tableName, row.fileName, row.status, totalCount, importedCount, perc)
		} else if withStreamingMode {
			if i == 0 {
				uiTable.AddRow(headerfmt("SCHEMA"), headerfmt("TABLE"), headerfmt("STATUS"), headerfmt("SNAPSHOT ROW COUNT"), headerfmt("TOTAL CHANGES EVENTS"),
					headerfmt("INSERTS"), headerfmt("UPDATES"), headerfmt("DELETES"),
					headerfmt("FINAL ROW COUNT(SNAPSHOT + CHANGES)"))
			}
			uiTable.AddRow(row.schemaName, row.tableName, row.status, row.importedCount, row.totalEvents,
				row.numInserts, row.numUpdates, row.numDeletes,
				row.finalRowCount)
		} else {
			if i == 0 {
				uiTable.AddRow(headerfmt("TABLE"), headerfmt("FILE"), headerfmt("STATUS"), headerfmt("TOTAL ROWS"), headerfmt("IMPORTED ROWS"), headerfmt("PERCENTAGE"))
			}
			// case of importData where row counts is available
			uiTable.AddRow(row.tableName, row.fileName, row.status, row.totalCount, row.importedCount, perc)
		}
	}

	if len(table) > 0 {
		fmt.Print("\n")
		fmt.Println(uiTable)
		fmt.Print("\n")
	}

	return nil
}

func prepareDummyDescriptor(state *ImportDataState) (*datafile.Descriptor, error) {
	var dataFileDescriptor datafile.Descriptor

	tableToFilesMapping, err := state.DiscoverTableToFilesMapping()
	if err != nil {
		return nil, fmt.Errorf("discover table to files mapping: %w", err)
	}
	for tableName, filePaths := range tableToFilesMapping {
		for _, filePath := range filePaths {
			fileSize, err := datastore.NewDataStore(filePath).FileSize(filePath)
			if err != nil {
				return nil, fmt.Errorf("get file size of %q: %w", filePath, err)
			}
			dataFileDescriptor.DataFileList = append(dataFileDescriptor.DataFileList, &datafile.FileEntry{
				FilePath:  filePath,
				TableName: tableName,
				FileSize:  fileSize,
				RowCount:  -1,
			})
		}
	}
	return &dataFileDescriptor, nil
}

func reInitialisingTarget() {
	migrationStatus, err := GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("get migration status record: %w", err)
	}
	migrationUUID, err = uuid.Parse(migrationStatus.MigrationUUID)
	if err != nil {
		utils.ErrExit("parse migration UUID %q: %w", migrationStatus.MigrationUUID, err)
	}
	err = json.Unmarshal([]byte(migrationStatus.TargetConf), &targetDBConf)
	if err != nil {
		utils.ErrExit("unmarshal target DB conf: %w", err)
	}
	targetDBConf.Password = tconf.Password
	targetDB = tgtdb.NewTargetDB(&targetDBConf)
	err = targetDB.Init()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB: %w", err)
	}
	defer targetDB.Finalize()
	err = targetDB.InitConnPool()
	if err != nil {
		utils.ErrExit("Failed to initialize the target DB connection pool: %w", err)
	}
	if migrationStatus.FallForwarDBExists {
		err = json.Unmarshal([]byte(migrationStatus.FallForwardDBConf), &ffDBConf)
		if err != nil {
			utils.ErrExit("unmarshal fall-forward DB conf: %w", err)
		}
		ffDBConf.Password = ffDBPassword
		ffDB = tgtdb.NewTargetDB(&ffDBConf)
		err = ffDB.Init()
		if err != nil {
			utils.ErrExit("Failed to initialize the fall-forward DB: %s", err)
		}
		defer ffDB.Finalize()
		err = ffDB.InitConnPool()
		if err != nil {
			utils.ErrExit("Failed to initialize the fall-forward DB connection pool: %s", err)
		}
	}
}

func prepareImportDataStatusTable(tgtdb tgtdb.TargetDB, tgtconf tgtdb.TargetConf, isffDB bool) ([]*tableMigStatusOutputRow, error) {
	var table []*tableMigStatusOutputRow
	var err error
	tdb = tgtdb
	tconf = tgtconf
	var dataFileDescriptor *datafile.Descriptor
	if isffDB {
		importDestinationType = FF_DB
	} else {
		importDestinationType = TARGET_DB
	}
	state := NewImportDataState(exportDir)
	dataFileDescriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	if utils.FileOrFolderExists(dataFileDescriptorPath) {
		// Case of `import data` command where row counts are available.
		dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	} else {
		// Case of `import data file` command where row counts are not available.
		// Use file sizes for progress reporting.
		dataFileDescriptor, err = prepareDummyDescriptor(state)
		if err != nil {
			return nil, fmt.Errorf("prepare dummy descriptor: %w", err)
		}
	}

	for _, dataFile := range dataFileDescriptor.DataFileList {
		var totalCount, importedCount int64
		var err error
		var perc float64
		var status string
		reportProgressInBytes = reportProgressInBytes || dataFile.RowCount == -1
		if reportProgressInBytes {
			totalCount = dataFile.FileSize
			importedCount, err = state.GetImportedByteCount(dataFile.FilePath, dataFile.TableName)
		} else {
			totalCount = dataFile.RowCount
			importedCount, err = state.GetImportedRowCount(dataFile.FilePath, dataFile.TableName)
		}
		if err != nil {
			return nil, fmt.Errorf("compute imported data size: %w", err)
		}
		if totalCount != 0 {
			perc = float64(importedCount) * 100.0 / float64(totalCount)
		}
		switch true {
		case importedCount == totalCount:
			status = "DONE"
		case importedCount == 0:
			status = "NOT_STARTED"
		case importedCount < totalCount:
			status = "MIGRATING"
		}
		row := &tableMigStatusOutputRow{
			tableName:  dataFile.TableName,
			schemaName: getTargetSchemaName(dataFile.TableName),
		}
		if withStreamingMode {
			qualifiedTableName := qualifyTableName(tconf.Schema, row.tableName)
			eventCounter, err := tdb.GetImportedEventsStatsForTable(qualifiedTableName, migrationUUID)
			if err != nil {
				return nil, fmt.Errorf("get imported events stats for table %q: %w", qualifiedTableName, err)
			}
			row.totalEvents = eventCounter.TotalEvents
			row.numInserts = eventCounter.NumInserts
			row.numUpdates = eventCounter.NumUpdates
			row.numDeletes = eventCounter.NumDeletes
			row.importedCount, err = state.GetImportedRowCount(dataFile.FilePath, dataFile.TableName)
			if err != nil {
				return nil, fmt.Errorf("get imported row count for table %q: %w", qualifiedTableName, err)
			}
			row.finalRowCount = row.importedCount + row.numInserts - row.numDeletes
			if (isffDB && checkfallForwardComplete()) || (!isffDB && checkCutoverCompleted()) { //TODO: handle for fallforward-complete
				row.status = "DONE"
			} else {
				row.status = "STREAMING"
			}
		} else {
			row.fileName = path.Base(dataFile.FilePath)
			row.status = status
			row.totalCount = totalCount
			row.importedCount = importedCount
			row.percentageComplete = perc // TODO: for streaming mode, this should be the percentage of events processed.
		}

		table = append(table, row)
	}
	// First sort by status and then by table-name.
	sort.Slice(table, func(i, j int) bool {
		ordStates := map[string]int{"MIGRATING": 1, "DONE": 2, "NOT_STARTED": 3, "STREAMING": 4}
		row1 := table[i]
		row2 := table[j]
		if row1.status == row2.status {
			if row1.tableName == row2.tableName {
				return strings.Compare(row1.fileName, row2.fileName) < 0
			} else {
				return strings.Compare(row1.tableName, row2.tableName) < 0
			}
		} else {
			return ordStates[row1.status] < ordStates[row2.status]
		}
	})

	return table, nil
}

func qualifyTableName(targetSchema string, tableName string) string {
	if len(strings.Split(tableName, ".")) != 2 {
		tableName = fmt.Sprintf("%s.%s", targetSchema, tableName)
	}
	return tableName
}