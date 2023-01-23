package datafile

import (
	"fmt"
	"os"
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

func OpenDataFile(filePath string, descriptor *Descriptor) (DataFile, error) {
	switch descriptor.FileFormat {
	case CSV:
		if val, present := os.LookupEnv("CSV_NO_NEWLINE"); present && (val == "yes" || val == "true" || val == "1" || val == "y") {
			return openTextDataFile(filePath, descriptor)
		} else {
			return openCsvDataFile(filePath, descriptor)
		}
	case TEXT:
		return openTextDataFile(filePath, descriptor)
	case SQL:
		return openSqlDataFile(filePath, descriptor)
	default:
		panic(fmt.Sprintf("Unknown file type %q", descriptor.FileFormat))

	}
}
