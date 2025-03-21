/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
		_, _, err := df.NextLine()
		if err != nil {
			return err
		}
	}
	df.ResetBytesRead(0)
	return nil
}

func (df *TextDataFile) NextLine() (string, int64, error) {
	var line string
	var err error
	var currentBytesRead int64
	for {
		line, err = df.reader.ReadString('\n')
		currentBytesRead += int64(len(line))
		if df.isDataLine(line) || err != nil {
			break
		}
	}
	df.bytesRead += currentBytesRead
	line = strings.Trim(line, "\n") // to get the raw row
	return line, currentBytesRead, err
}

func (df *TextDataFile) Close() {
	df.closer.Close()
}

func (df *TextDataFile) GetBytesRead() int64 {
	return df.bytesRead
}

func (df *TextDataFile) ResetBytesRead(bytes int64) {
	df.bytesRead = bytes
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

	line, _, err := df.NextLine()
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
