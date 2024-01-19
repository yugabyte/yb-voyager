package srcdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type TableExportStatus struct {
	TableName                string `json:"table_name"` // table.Qualified.MinQuoted
	FileName                 string `json:"file_name"`
	Status                   string `json:"status"`
	ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
}

type ExportSnapshotStatus struct {
	Tables     map[string]*TableExportStatus `json:"tables"`
	statusLock sync.Mutex                    `json:"-"` // not serialized
	FilePath   string                        `json:"-"` // not serialized
}

func NewExportSnapshotStatus(exportDir string) *ExportSnapshotStatus {
	return &ExportSnapshotStatus{
		Tables:   make(map[string]*TableExportStatus),
		FilePath: filepath.Join(exportDir, "metainfo", "export_snapshot_status.json"),
	}
}

func ReadExportStatus(statusFilePath string) (*ExportSnapshotStatus, error) {
	file, err := os.Open(statusFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open file %s: %v", statusFilePath, err)
	}
	defer file.Close()

	var status ExportSnapshotStatus
	err = json.NewDecoder(file).Decode(&status)
	if err != nil {
		return nil, fmt.Errorf("failed to decode export status file %s: %v", statusFilePath, err)
	}
	return &status, nil
}

func (status *ExportSnapshotStatus) Create() error {
	status.statusLock.Lock()
	defer status.statusLock.Unlock()

	file, err := os.Create(status.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", status.FilePath, err)
	}
	defer file.Close()

	finalStatus := prettifyExportSnapshotStatus(status)

	err = json.NewEncoder(file).Encode(finalStatus)
	if err != nil {
		return fmt.Errorf("failed to encode export status file %s: %v", status.FilePath, err)
	}
	return nil
}

func (status *ExportSnapshotStatus) Update(key string, updatedRows int64, filePath string, progressStatus int) error {
	status.statusLock.Lock()
	defer status.statusLock.Unlock()

	file, err := os.OpenFile(status.FilePath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", status.FilePath, err)
	}
	defer file.Close()

	status.Tables[key] = &TableExportStatus{
		TableName:                status.Tables[key].TableName,
		FileName:                 filePath,
		Status:                   utils.TableMetadataStatusMap[progressStatus],
		ExportedRowCountSnapshot: updatedRows,
	}
	finalStatus := prettifyExportSnapshotStatus(status)

	_, err = file.WriteString(finalStatus)
	if err != nil {
		return fmt.Errorf("failed to encode export status file %s: %v", status.FilePath, err)
	}
	return nil
}

func prettifyExportSnapshotStatus(status *ExportSnapshotStatus) string {
	jsonBytes, err := json.Marshal(status)
	if err != nil {
		panic(err)
	}
	statusJsonString := string(jsonBytes)
	finalStatus := utils.PrettifyJsonString(statusJsonString)
	return finalStatus
}

func (status *ExportSnapshotStatus) GetTableExportStatus(tableName, schemaName string) *TableExportStatus {
	status.statusLock.Lock()
	defer status.statusLock.Unlock()

	table := sqlname.NewSourceNameFromMaybeQualifiedName(tableName, schemaName)
	return status.Tables[table.Qualified.MinQuoted]
}
