package dbzm

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	MODE_SNAPSHOT_EXPORT = "snapshot_export"
	MODE_STREAMING       = "streaming"
)

type TableExportStatus struct {
	DatabaseName string `json:"database_name"`
	SchemaName   string `json:"schema_name"`
	TableName    string `json:"table_name"`

	FileName         string `json:"file_name"`
	ExportedRowCount int64  `json:"exported_row_count"`
	CopyStmt         string `json:"copy_stmt"`
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
		if err == os.ErrNotExist {
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
