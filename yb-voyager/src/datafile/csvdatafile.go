package datafile

import (
	"bytes"
	"encoding/csv"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type CsvDataFile struct {
	file      *os.File
	reader    *csv.Reader
	bytesRead int64
	Delimiter string
	Header    string
	bytes     bytes.Buffer
	DataFile
}

func (df *CsvDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; i++ {
		_, err := df.NextLine()
		if err != nil {
			return err
		}
	}
	df.ResetBytesRead()
	return nil
}

func (df *CsvDataFile) NextLine() (string, error) {
	w := csv.NewWriter(&df.bytes)
	w.Comma = []rune(df.Delimiter)[0]
	var line string
	var err error
	for {
		// read a record from csv file
		fields, err := df.reader.Read()
		if err != nil {
			return "", err
		}

		// write the record to buffer and convert it to string following csv format
		err = w.Write(fields)
		if err != nil {
			return "", err
		}
		w.Flush()
		line = df.bytes.String()
		df.bytes.Reset()
		df.bytesRead += int64(len(line))
		if df.isDataLine(line) || err != nil {
			break
		}
	}
	line = strings.Trim(line, "\n") // to get the raw row
	return line, err
}

func (df *CsvDataFile) Close() {
	df.file.Close()
}

func (df *CsvDataFile) GetBytesRead() int64 {
	return df.bytesRead
}

func (df *CsvDataFile) ResetBytesRead() {
	df.bytesRead = 0
}

func (df *CsvDataFile) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n" || line == "\"\\.\"" || line == "\"\\.\"\n")

	return !(emptyLine || newLineChar || endOfCopy)
}

func (df *CsvDataFile) GetHeader() string {
	if df.Header != "" {
		return df.Header
	}

	line, err := df.NextLine()
	if err != nil {
		utils.ErrExit("finding header for csvdata file: %v", err)
	}

	df.Header = line
	return df.Header
}

func openCsvDataFile(filePath string, descriptor *Descriptor) (*CsvDataFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)
	reader.Comma = []rune(descriptor.Delimiter)[0]
	reader.FieldsPerRecord = -1 // fields not fixed for all rows, last line can be '\.'
	reader.LazyQuotes = true    // allow quotes in fields

	csvDataFile := &CsvDataFile{
		file:      file,
		reader:    reader,
		Delimiter: descriptor.Delimiter,
	}
	log.Infof("created csv data file struct for file: %s", filePath)

	return csvDataFile, err
}
