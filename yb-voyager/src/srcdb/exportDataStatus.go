package srcdb

type TableExportStatus struct {
	TableName                string `json:"table_name"` // table.Qualified.MinQuoted
	FileName                 string `json:"file_name"`
	Status                   string `json:"status"`
	ExportedRowCountSnapshot int64  `json:"exported_row_count_snapshot"`
}

type ExportSnapshotStatus struct {
	Tables map[string]*TableExportStatus `json:"tables"`
}

func NewExportSnapshotStatus() *ExportSnapshotStatus {
	return &ExportSnapshotStatus{
		Tables: make(map[string]*TableExportStatus),
	}
}
