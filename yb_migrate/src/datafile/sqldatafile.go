package datafile

import (
	"bufio"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

type SqlDataFile struct {
	file           *os.File
	reader         *bufio.Reader
	insideCopyStmt bool
	bytesRead      int64
	DataFile
}

func (sqldf *SqlDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; {
		line, err := sqldf.NextLine()
		if err != nil {
			return err
		}
		if sqldf.isDataLine(line) {
			i++
		}
	}
	sqldf.ResetBytesRead()
	return nil
}

func (sqldf *SqlDataFile) NextLine() (string, error) {
	var line string
	var err error
	for {
		line, err = sqldf.reader.ReadString('\n')
		sqldf.bytesRead += int64(len(line))
		if sqldf.isDataLine(line) || err != nil {
			break
		}
	}

	line = strings.Trim(line, "\n") //to current only the current line content
	return line, err
}

func (sqldf *SqlDataFile) Close() {
	sqldf.file.Close()
}

func (sqldf *SqlDataFile) GetBytesRead() int64 {
	return sqldf.bytesRead
}

func (sqldf *SqlDataFile) ResetBytesRead() {
	sqldf.bytesRead = 0
}

func (sqldf *SqlDataFile) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	if sqldf.insideCopyStmt {
		if endOfCopy {
			sqldf.insideCopyStmt = false
		}
		return !(emptyLine || newLineChar || endOfCopy)
	} else { // outside copy
		if reCopy.MatchString(line) {
			sqldf.insideCopyStmt = true
		}
		return false
	}
}

func openSqlDataFile(filePath string, descriptor *Descriptor) (*SqlDataFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	sqlDataFile := &SqlDataFile{
		file:           file,
		reader:         reader,
		insideCopyStmt: false,
	}
	log.Infof("created sql data file struct for file: %s", filePath)

	return sqlDataFile, err
}
