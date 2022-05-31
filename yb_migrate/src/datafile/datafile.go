package datafile

import (
	"bufio"
	"encoding/csv"
	"os"
)

const (
	CSV = "csv"
	SQL = "sql"
)

type DataFile interface {
	SkipLines(numLines int64) error
	NextLine() (string, error)
	Close()
}

func OpenDataFile(filePath string, descriptor *Descriptor) (DataFile, error) {
	var df DataFile

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	if descriptor.FileType == CSV {
		reader := csv.NewReader(file)
		reader.Comma = []rune(descriptor.Delimiter)[0]
		reader.FieldsPerRecord = -1 // last line can be '\.'
		reader.LazyQuotes = true    // to ignore quotes in fileds
		df = &CsvDataFile{
			file:      file,
			reader:    reader,
			Delimiter: descriptor.Delimiter,
		}
	} else if descriptor.FileType == SQL {
		reader := bufio.NewReader(file)
		df = &SqlDataFile{
			file:   file,
			reader: reader,
		}
	}

	return df, err
}
