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
	"io"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/csv"
)

type CsvDataFile struct {
	reader     *csv.Reader
	bytesRead  int64
	Delimiter  string
	Header     string
	QuoteChar  string
	EscapeChar string
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
	var skippedByteCount int
	for {
		line, skippedByteCount, err = df.reader.Read()
		df.bytesRead += int64(len(line)) + int64(skippedByteCount)
		if err != nil {
			return "", err
		}
		if df.isDataLine(line) {
			break
		}
	}
	line = strings.Trim(line, "\n") // to get the raw row
	return line, err
}

func (df *CsvDataFile) Close() {
	df.reader.Close()
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

func newCsvDataFile(filePath string, fileReadCloser io.ReadCloser, descriptor *Descriptor) (*CsvDataFile, error) {
	reader, err := csv.NewReader(filePath, fileReadCloser)
	if err != nil {
		return nil, err
	}

	if descriptor.QuoteChar != 0 {
		reader.QuoteChar = descriptor.QuoteChar
	}
	if descriptor.EscapeChar != 0 {
		reader.EscapeChar = descriptor.EscapeChar
	}

	csvDataFile := &CsvDataFile{
		reader:    reader,
		Delimiter: descriptor.Delimiter,
	}
	log.Infof("created csv data file struct for file: %s", filePath)

	return csvDataFile, err
}
