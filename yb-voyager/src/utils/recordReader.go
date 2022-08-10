package utils

import (
	"bufio"
	"io"
	"os"
)

type RecordReader struct {
	file       *os.File
	NumRecords int64
	reader     *bufio.Reader
}

func (rr *RecordReader) Read(p []byte) (n int, err error) {
	n, err = rr.reader.Read(p)

	for i := 0; i < n; i++ {
		if string(p[i]) == "\n" {
			rr.NumRecords++
		}
	}

	return n, err
}

func (rr *RecordReader) RestartFromBegin() {
	rr.file.Seek(0, io.SeekStart)
	rr.reader.Reset(rr.file)
	rr.NumRecords = 0
}

func NewRecordReader(file *os.File) *RecordReader {
	return &RecordReader{
		reader:     bufio.NewReader(file),
		file:       file,
		NumRecords: 0,
	}
}
