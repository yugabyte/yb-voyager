package datafile

import (
	"bufio"
	"os"
	"regexp"

	log "github.com/sirupsen/logrus"
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
	IsDataLine(line string) bool
	Close()
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

func OpenDataFile(filePath string, descriptor *Descriptor) (DataFile, error) {
	var df DataFile

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	if descriptor.FileType == CSV {
		// TODO: resolve the issue in counting bytes with csvreader
		// reader := csv.NewReader(file)
		// reader.Comma = []rune(descriptor.Delimiter)[0]
		// reader.FieldsPerRecord = -1 // last line can be '\.'
		// reader.LazyQuotes = true    // to ignore quotes in fileds
		reader := bufio.NewReader(file)
		df = &CsvDataFile{
			file:      file,
			reader:    reader,
			Delimiter: descriptor.Delimiter,
		}
		log.Infof("created csv data file struct for file: %s", filePath)
	} else if descriptor.FileType == SQL {
		reader := bufio.NewReader(file)
		df = &SqlDataFile{
			file:           file,
			reader:         reader,
			insideCopyStmt: false,
		}
		log.Infof("created sql data file struct for file: %s",filePath)
	}

	return df, err
}
