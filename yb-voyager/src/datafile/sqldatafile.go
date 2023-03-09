package datafile

import (
	"bufio"
	"io"
	"strings"
)

type SqlDataFile struct {
	closer         io.Closer
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
	df.closer.Close()
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

func newSqlDataFile(readCloser io.ReadCloser, descriptor *Descriptor) (*SqlDataFile, error) {
	reader := bufio.NewReader(readCloser)
	sqlDataFile := &SqlDataFile{
		closer:         readCloser,
		reader:         reader,
		insideCopyStmt: false,
	}
	return sqlDataFile, nil
}
