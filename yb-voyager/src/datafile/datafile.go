package datafile

import (
	"fmt"
	"io"
	"regexp"
)

const (
	CSV  = "csv"
	SQL  = "sql"
	TEXT = "text"
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

func NewDataFile(fileName string, reader io.ReadCloser, descriptor *Descriptor) (DataFile, error) {
	switch descriptor.FileFormat {
	case CSV:
		return newCsvDataFile(fileName, reader, descriptor)
	case TEXT:
		return newTextDataFile(fileName, reader, descriptor)
	case SQL:
		return newSqlDataFile(fileName, reader, descriptor)
	default:
		panic(fmt.Sprintf("Unknown file type %q", descriptor.FileFormat))

	}
}
