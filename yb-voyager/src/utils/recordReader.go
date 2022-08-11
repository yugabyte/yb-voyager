package utils

import (
	"bufio"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

type RecordReader struct {
	file       *os.File
	NumRecords int64
	reader     *bufio.Reader
	hasHeader  bool
}

func NewRecordReader(file *os.File, hasHeader bool) *RecordReader {
	rr := &RecordReader{
		reader:     bufio.NewReader(file),
		file:       file,
		NumRecords: initializeNumRecords(hasHeader),
		hasHeader:  hasHeader,
	}
	log.Infof("Created NewRecordReader for filename=%q, NumRecords=%d, hasHeader=%t",
		rr.file.Name(), rr.NumRecords, rr.hasHeader)
	return rr
}

func (rr *RecordReader) Read(p []byte) (n int, err error) {
	n, err = rr.reader.Read(p)

	/*
		Amit's comment
		If the CSV file is UTF-8 encoded (which is essential), then this code may not be correct. Refer https://golangbyexample.com/iterate-over-a-string-golang/ . Reading each byte separately may lead to incorrectly classifying some bytes as \n.
		Unfortunately, a for-range loop may not also be enough. Consider a case where only the first byte of a multi-byte character was included in the buffer p. In that case, the for loop will fail with “invalid byte in utf-8” error.
	*/
	for i := 0; i < n; i++ {
		if string(p[i]) == "\n" {
			rr.NumRecords++
		}
	}

	return n, err
}

func (rr *RecordReader) RestartFromBegin() {
	_, err := rr.file.Seek(0, io.SeekStart)
	if err != nil {
		ErrExit("unable to reset offset to beginning for file %q: %v", rr.file.Name(), err)
	}
	rr.reader.Reset(rr.file)
	rr.NumRecords = initializeNumRecords(rr.hasHeader)
}

func initializeNumRecords(hasHeader bool) int64 {
	var numRecords int64 = 0
	if hasHeader {
		numRecords = -1
	}
	return numRecords
}
