package tgtdb

import (
	"fmt"
	"strings"
)

type TargetDB interface {
	Init() error
	Finalize()
	InitConnPool() error
	CleanFileImportState(filePath, tableName string) error
	GetVersion() string
	CreateVoyagerSchema() error
	GetNonEmptyTables(tableNames []string) []string
	IsNonRetryableCopyError(err error) bool
	ImportBatch(batch Batch, args *ImportBatchArgs) (int64, error)
	// TODO: ConnPool() is a temporary method. It should eventually be removed.
	ConnPool() *ConnectionPool
}

func NewTargetDB(tconf *TargetConf) TargetDB {
	return newTargetYugabyteDB(tconf)
}

type ImportBatchArgs struct {
	FilePath  string
	TableName string
	Columns   []string

	FileFormat string
	HasHeader  bool
	Delimiter  string
	QuoteChar  byte
	EscapeChar byte
	NullString string

	RowsPerTransaction int
}

func (args *ImportBatchArgs) GetYBCopyStatement() string {
	columns := ""
	if len(args.Columns) > 0 {
		columns = fmt.Sprintf("(%s)", strings.Join(args.Columns, ", "))
	}
	options := []string{
		fmt.Sprintf("FORMAT '%s'", args.FileFormat),
		fmt.Sprintf("ROWS_PER_TRANSACTION %v", args.RowsPerTransaction),
	}
	if args.HasHeader {
		options = append(options, "HEADER")
	}
	if args.Delimiter != "" {
		options = append(options, fmt.Sprintf("DELIMITER E'%c'", []rune(args.Delimiter)[0]))
	}
	if args.QuoteChar != 0 {
		// TODO: confirm if the following cast to string is correct.
		options = append(options, fmt.Sprintf("QUOTE E'%s'", string(args.QuoteChar)))
	}
	if args.EscapeChar != 0 {
		options = append(options, fmt.Sprintf("ESCAPE E'%s'", string(args.EscapeChar)))
	}
	if args.NullString != "" {
		options = append(options, fmt.Sprintf("NULL '%s'", args.NullString))
	}
	return fmt.Sprintf(`COPY %s %s FROM STDIN WITH (%s)`, args.TableName, columns, strings.Join(options, ", "))
}
