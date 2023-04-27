package dbzm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	MODE_SNAPSHOT  = "SNAPSHOT"
	MODE_STREAMING = "STREAMING"
)

type TableExportStatus struct {
	DatabaseName string `json:"database_name"`
	SchemaName   string `json:"schema_name"`
	TableName    string `json:"table_name"`

	FileName         string `json:"file_name"`
	ExportedRowCount int64  `json:"exported_row_count"`
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
