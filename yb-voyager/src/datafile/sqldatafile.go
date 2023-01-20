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

func (df *SqlDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; i++ {
		_, err := df.NextLine()
		if err != nil {
			return err
		}
	}
	df.ResetBytesRead()
	return nil
}

func (df *SqlDataFile) NextLine() (string, error) {
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

func (df *SqlDataFile) Close() {
	df.file.Close()
}

func (df *SqlDataFile) GetBytesRead() int64 {
	return df.bytesRead
}

func (df *SqlDataFile) ResetBytesRead() {
	df.bytesRead = 0
}

func (df *SqlDataFile) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	if df.insideCopyStmt {
		if endOfCopy {
			df.insideCopyStmt = false
		}
		return !(emptyLine || newLineChar || endOfCopy)
	} else { // outside copy
		if reCopy.MatchString(line) {
			df.insideCopyStmt = true
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
