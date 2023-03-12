package datafile

import (
	"bufio"
	"io"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TextDataFile struct {
	closer    io.Closer
	reader    *bufio.Reader
	bytesRead int64
	Delimiter string
	Header    string
	DataFile
}

func (df *TextDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; i++ {
		_, err := df.NextLine()
		if err != nil {
			return err
		}
	}
	df.ResetBytesRead()
	return nil
}

func (df *TextDataFile) NextLine() (string, error) {
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

func (df *TextDataFile) Close() {
	df.closer.Close()
}

func (df *TextDataFile) GetBytesRead() int64 {
	return df.bytesRead
}

func (df *TextDataFile) ResetBytesRead() {
	df.bytesRead = 0
}

func (df *TextDataFile) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	return !(emptyLine || newLineChar || endOfCopy)
}

func (df *TextDataFile) GetHeader() string {
	if df.Header != "" {
		return df.Header
	}

	line, err := df.NextLine()
	if err != nil {
		utils.ErrExit("finding header for text data file: %v", err)
	}

	df.Header = line
	return df.Header
}

func newTextDataFile(filePath string, readCloser io.ReadCloser, descriptor *Descriptor) (*TextDataFile, error) {
	textDataFile := &TextDataFile{
		closer:    readCloser,
		reader:    bufio.NewReader(readCloser),
		Delimiter: descriptor.Delimiter,
	}
	log.Infof("created text data file struct for file: %s", filePath)

	return textDataFile, nil
}
