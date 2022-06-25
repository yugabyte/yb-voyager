package main

import (
	"bufio"
	"os"
)

type DataFile interface {
	Open() error
	Offset() int64
	SkipRecords(n int) (int, bool, error)
}

func NewDataFile(fileName string, offset int64) DataFile {
	return NewCSVDataFile(fileName, offset)
}

//============================================================================

type CSVDataFile struct {
	*baseDataFile
}

func NewCSVDataFile(fileName string, offset int64) *CSVDataFile {
	df := &CSVDataFile{}
	base := newBaseDataFile(fileName, offset, df.isDataLine)
	df.baseDataFile = base
	return df
}

func (df *CSVDataFile) isDataLine(line string) bool {
	return !(line == "" || line == `\.`)
}

//============================================================================

type baseDataFile struct {
	FileName string
	offset   int64

	fh      *os.File
	scanner *bufio.Scanner

	isDataLine func(string) bool
}

func newBaseDataFile(fileName string, offset int64, isDataLine func(string) bool) *baseDataFile {
	return &baseDataFile{FileName: fileName, offset: offset, isDataLine: isDataLine}
}

func (df *baseDataFile) Open() error {
	fh, err := os.Open(df.FileName)
	if err != nil {
		return err
	}
	_, err = fh.Seek(df.offset, 0)
	if err != nil {
		return err
	}
	df.fh = fh
	df.scanner = bufio.NewScanner(fh)
	return nil
}

func (df *baseDataFile) Offset() int64 {
	return df.offset
}

func (df *baseDataFile) SkipRecords(n int) (int, bool, error) {
	count := 0
	for count < n && df.scanner.Scan() {
		line := df.scanner.Text()
		df.offset += int64(len(line)) + 1 // Add 1 to account for '\n'.
		if df.isDataLine(line) {
			count++
		}
	}
	eof := df.scanner.Err() == nil && count < n
	return count, eof, df.scanner.Err()
}

//============================================================================
