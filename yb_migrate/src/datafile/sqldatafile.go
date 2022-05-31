package datafile

import (
	"bufio"
	"os"
	"strings"
)

type SqlDataFile struct {
	file   *os.File
	reader *bufio.Reader
	DataFile
}

func (sqldf *SqlDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; i++ {
		_, err := sqldf.NextLine()
		if err != nil {
			return err
		}
	}
	return nil
}

func (sqldf *SqlDataFile) NextLine() (string, error) {
	line, err := sqldf.reader.ReadString('\n')
	line = strings.Trim(line, "\n") //to current only the current line content
	return line, err
}

func (sqldf *SqlDataFile) Close() {
	sqldf.file.Close()
}
