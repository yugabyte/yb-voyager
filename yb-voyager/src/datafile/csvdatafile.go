package datafile

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/csv"

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
	var line string
	var err error
	for {
		startOffset := df.reader.InputOffset()
		// read a record from csv file
		_, err := df.reader.Read()
		if err != nil {
			return "", err
		}
		endOffset := df.reader.InputOffset()

		buf := make([]byte, endOffset-startOffset) // buffer size of the record/row in csv file
		_, err = df.file.ReadAt(buf, startOffset)
		if err != nil {
			return "", fmt.Errorf("read file %q [%v:%v]: %w", df.file.Name(), startOffset, endOffset, err)
		}
		line = string(buf)

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
	endOfCopy := (line == "\\." || line == "\\.\n")

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

	csvDataFile := &CsvDataFile{
		file:      file,
		reader:    reader,
		Delimiter: descriptor.Delimiter,
	}
	log.Infof("created csv data file struct for file: %s", filePath)

	return csvDataFile, err
}
