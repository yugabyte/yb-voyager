package dbzm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	MODE_SNAPSHOT  = "SNAPSHOT"
	MODE_STREAMING = "STREAMING"
)

type TableExportStatus struct {
	Sno          int    `json:"sno"`
	DatabaseName string `json:"database_name"`
	SchemaName   string `json:"schema_name"`
	TableName    string `json:"table_name"`

	FileName                 string `json:"file_name"`
	ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
}

type ExportStatus struct {
	Mode   string              `json:"mode"`
	Tables []TableExportStatus `json:"tables"`
}

func (status *ExportStatus) SnapshotExportIsComplete() bool {
	// When streaming mode is active, we assume that the snapshot export is complete.
	return status.Mode == MODE_STREAMING
}

func ReadExportStatus(statusFilePath string) (*ExportStatus, error) {
	file, err := os.Open(statusFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open file %s: %v", statusFilePath, err)
	}
	defer file.Close()

	var status ExportStatus
	err = json.NewDecoder(file).Decode(&status)
	if err != nil {
		return nil, fmt.Errorf("failed to decode export status file %s: %v", statusFilePath, err)
	}
	return &status, nil
}

func IsLiveMigrationInSnapshotMode(exportDir string) bool {
	statusFilePath := filepath.Join(exportDir, "export_status.json")
	status, err := ReadExportStatus(statusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", statusFilePath, err)
	}
	return status != nil && status.Mode == MODE_SNAPSHOT
}

func IsLiveMigrationInStreamingMode(exportDir string) bool {
	statusFilePath := filepath.Join(exportDir, "export_status.json")
	status, err := ReadExportStatus(statusFilePath)
	if err != nil {
		utils.ErrExit("Failed to read export status file %s: %v", statusFilePath, err)
	}
	return status != nil && status.Mode == MODE_STREAMING
}

// get table with largest sno
func (status *ExportStatus) GetTableWithLargestSno() *TableExportStatus {
	var table *TableExportStatus
	for i := range status.Tables {
		if table == nil || status.Tables[i].Sno > table.Sno {
			table = &status.Tables[i]
		}
	}
	if table != nil {
		log.Infof("GetTableWithLargestSno(): table=%q with largest sno: %v\n", table.TableName, table.Sno)
	}
	return table
}

// get table's sno
func (status *ExportStatus) GetTableSno(tableName string, schemaName string) int {
	tableSno := math.MaxInt32
	for i := 0; i < len(status.Tables); i++ {
		if status.Tables[i].TableName == tableName && status.Tables[i].SchemaName == schemaName {
			tableSno = status.Tables[i].Sno
			break
		}
	}

	log.Infof("GetTableSno(): table=%q with sno: %d\n", tableName, tableSno)
	return tableSno
}

// check given tablename is exported if sno is lesser than largest sno
func (status *ExportStatus) IsTableExported(tableName string) bool {
	largestSno := status.GetTableWithLargestSno().Sno
	for i := range status.Tables {
		if status.Tables[i].TableName == tableName && status.Tables[i].Sno < largestSno {
			return true
		}
	}
	return false
}

func (status *ExportStatus) InProgressTable() string {
	if status.SnapshotExportIsComplete() {
		return ""
	}

	return status.GetTableWithLargestSno().TableName
}

func (status *ExportStatus) AreTablesExporting() bool {
	return status.GetTableWithLargestSno() != nil
}

// read status file again for updated status
func (status *ExportStatus) Refresh(statusFilePath string) error {
	newStatus, err := ReadExportStatus(statusFilePath)
	if err != nil {
		return err
	}
	log.Infof("refreshed status: %+v", newStatus)
	*status = *newStatus
	return nil
}

func (status *ExportStatus) GetTableExportedRowCount(tableName string) int64 {
	for i := range status.Tables {
		if status.Tables[i].TableName == tableName {
			return status.Tables[i].ExportedRowCountSnapshot
		}
	}
	return 0
}
