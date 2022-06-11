package datafile

import (
	"bufio"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type CsvDataFile struct {
	file      *os.File
	reader    *bufio.Reader
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
	/*
		// TODO: resolve the issue in counting bytes with csvreader
		// here csv reader can be more useful in case filter table's columns
		// currently using normal file reader will also work
		fields, err := df.reader.Read()
		if err != nil {
			return "", err
		}

		line := strings.Join(fields, df.Delimiter)
		df.bytesRead += int64(len(line + "\n")) // using the line in actual form to calculate bytes
	*/

	var line string
	var err error
	for {
		line, err = df.reader.ReadString('\n')
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

	// TODO: resolve the issue in counting bytes with csvreader
	// reader := csv.NewReader(file)
	// reader.Comma = []rune(descriptor.Delimiter)[0]
	// reader.FieldsPerRecord = -1 // last line can be '\.'
	// reader.LazyQuotes = true    // to ignore quotes in fileds
	reader := bufio.NewReader(file)
	csvDataFile := &CsvDataFile{
		file:      file,
		reader:    reader,
		Delimiter: descriptor.Delimiter,
	}
	log.Infof("created csv data file struct for file: %s", filePath)

	return csvDataFile, err
}
