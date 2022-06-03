package datafile

import (
	"bufio"
	"os"
	"strings"
)

type CsvDataFile struct {
	file      *os.File
	reader    *bufio.Reader
	bytesRead int64
	Delimiter string
	Header    string
	DataFile
}

func (csvdf *CsvDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; {
		line, err := csvdf.NextLine()
		if err != nil {
			return err
		}
		if csvdf.IsDataLine(line) {
			i++
		}
	}
	csvdf.ResetBytesRead()
	return nil
}

func (csvdf *CsvDataFile) NextLine() (string, error) {
	/*
		// TODO: resolve the issue in counting bytes with csvreader
		// here csv reader can be more useful in case filter table's columns
		// currently using normal file reader will also work
		fields, err := csvdf.reader.Read()
		if err != nil {
			return "", err
		}

		line := strings.Join(fields, csvdf.Delimiter)
		csvdf.bytesRead += int64(len(line + "\n")) // using the line in actual form to calculate bytes
	*/

	line, err := csvdf.reader.ReadString('\n')

	csvdf.bytesRead += int64(len(line))

	line = strings.Trim(line, "\n") // to get the raw row
	return line, err
}

func (csvdf *CsvDataFile) Close() {
	csvdf.file.Close()
}

func (csvdf *CsvDataFile) GetBytesRead() int64 {
	return csvdf.bytesRead
}

func (csvdf *CsvDataFile) ResetBytesRead() {
	csvdf.bytesRead = 0
}

func (csvdf *CsvDataFile) IsDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	return !(emptyLine || newLineChar || endOfCopy)
}
