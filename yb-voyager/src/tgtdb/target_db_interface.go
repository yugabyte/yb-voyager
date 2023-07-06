package tgtdb

// Not used yet.
type ImportBatchArgs struct {
	TableName string
	Columns   []string
	Format    *string
	Header    *bool
	Delimiter *rune
	Quote     *rune
	Escape    *rune
	Null      *string

	RowsPerTransaction int
}

type TargetDB interface {
	Init() error
	Finalize()
	InitConnPool() error
	CleanFileImportState(filePath, tableName string) error
	GetVersion() string
	CreateVoyagerSchema() error
	GetNonEmptyTables(tableNames []string) []string
	IsNonRetryableCopyError(err error) bool
	// TODO: Replace `copyCommand` with `*ImportBatchArgs`.
	ImportBatch(batch Batch, copyCommand string) (int64, error)
	// TODO: ConnPool() is a temporary method. It should eventually be removed.
	ConnPool() *ConnectionPool
}

func NewTargetDB(tconf *TargetConf) TargetDB {
	return newTargetYugabyteDB(tconf)
}
