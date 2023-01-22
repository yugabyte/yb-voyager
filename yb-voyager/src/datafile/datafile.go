package datafile

import (
	"fmt"
	"regexp"
)

const (
	CSV  = "csv"
	SQL  = "sql"
	TEXT = "text"
	CSV_NO_NEWLINE = "csv_no_newline"
)

type DataFile interface {
	SkipLines(numLines int64) error
	NextLine() (string, error)
	GetBytesRead() int64
	ResetBytesRead()
	GetHeader() string
	Close()
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

func OpenDataFile(filePath string, descriptor *Descriptor) (DataFile, error) {
	switch descriptor.FileFormat {
	case CSV:
		return openCsvDataFile(filePath, descriptor)
	case TEXT, CSV_NO_NEWLINE:
		return openTextDataFile(filePath, descriptor)
	case SQL:
		return openSqlDataFile(filePath, descriptor)
	default:
		panic(fmt.Sprintf("Unknown file type %q", descriptor.FileFormat))

	}
}
