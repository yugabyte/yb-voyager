package pbreporter

type ExportProgressReporter interface {
	TableExportStarted(tableName string)
	TableExportDone(tableName string)
	SetTotalRowCount(tableName string, totalRowCount int)
	SetExportedRowCount(tableName string, exportedRowCount int)
}
