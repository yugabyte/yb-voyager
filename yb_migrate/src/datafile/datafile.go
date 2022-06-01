package datafile

const (
	CSV = "csv"
	SQL = "sql"
)

type DataFile interface {
	SkipLines(numLines int64) error
	NextLine() (string, error)
	Close()
}
