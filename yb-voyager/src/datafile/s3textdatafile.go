package datafile

import (
	"bufio"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type S3TextDataFile struct {
	file      *os.File
	reader    *bufio.Reader
	bytesRead int64
	Delimiter string
	Header    string
	DataFile
}

func (df *S3TextDataFile) SkipLines(numLines int64) error {
	for i := int64(1); i <= numLines; i++ {
		_, err := df.NextLine()
		if err != nil {
			return err
		}
	}
	df.ResetBytesRead()
	return nil
}

func (df *S3TextDataFile) NextLine() (string, error) {
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

func (df *S3TextDataFile) Close() {
	df.file.Close()
}

func (df *S3TextDataFile) GetBytesRead() int64 {
	return df.bytesRead
}

func (df *S3TextDataFile) ResetBytesRead() {
	df.bytesRead = 0
}

func (df *S3TextDataFile) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	return !(emptyLine || newLineChar || endOfCopy)
}

func (df *S3TextDataFile) GetHeader() string {
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

func openS3TextDataFile(filePath string, descriptor *Descriptor) (*S3TextDataFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	s3TextDataFile := &S3TextDataFile{
		file:      file,
		reader:    reader,
		Delimiter: descriptor.Delimiter,
	}
	log.Infof("created text data file struct for file: %s", filePath)

	return s3TextDataFile, err
}
