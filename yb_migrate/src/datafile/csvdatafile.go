package datafile

import (
	"encoding/csv"
	"os"
	"strings"
)

type CsvDataFile struct {
	file      *os.File
	reader    *csv.Reader
	Delimiter string
	Header    string
	DataFile
}

func (csvdf *CsvDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; i++ {
		_, err := csvdf.NextLine()
		if err != nil {
			return err
		}
	}
	return nil
}

func (csvdf *CsvDataFile) NextLine() (string, error) {
	// here csv reader can be more useful in case filter table's columns
	// currently using normal file reader will also work
	line, err := csvdf.reader.Read()
	if err != nil {
		return "", err
	}

	return strings.Join(line, csvdf.Delimiter), err
}

func (csvdf *CsvDataFile) Close() {
	csvdf.file.Close()
}
