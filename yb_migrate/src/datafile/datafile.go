package datafile

import (
	"fmt"
	"regexp"
)

const (
	CSV = "csv"
	SQL = "sql"
)

type DataFile interface {
	SkipLines(numLines int64) error
	NextLine() (string, error)
	GetBytesRead() int64
	ResetBytesRead()
	isDataLine(line string) bool
	GetCopyHeader() string
	Close()
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

func OpenDataFile(filePath string, descriptor *Descriptor) (DataFile, error) {
	switch descriptor.FileType {
	case CSV:
		return openCsvDataFile(filePath, descriptor)
	case SQL:
		return openSqlDataFile(filePath, descriptor)
	default:
		panic(fmt.Sprintf("Unknown file type %q", descriptor.FileType))

	}
}
