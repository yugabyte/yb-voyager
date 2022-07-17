package libmig

import (
	"io"
	"os"
)

type FileSegmentReader struct {
	fileName    string
	fh          *os.File
	StartOffset int64
	EndOffset   int64
	CurOffset   int64
}

func NewFileSegmentReader(fileName string, startOffset, endOffset int64) (*FileSegmentReader, error) {
	fh, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	if endOffset == -1 {
		fi, err := fh.Stat()
		if err != nil {
			return nil, err
		}
		endOffset = fi.Size()
	}
	r := &FileSegmentReader{
		fileName:    fileName,
		fh:          fh,
		StartOffset: startOffset,
		EndOffset:   endOffset,
		CurOffset:   startOffset,
	}
	return r, nil
}

func (r *FileSegmentReader) Read(p []byte) (n int, err error) {
	if r.CurOffset >= r.EndOffset {
		return 0, io.EOF
	}
	n, err = r.fh.ReadAt(p, r.CurOffset)
	if r.CurOffset+int64(n) >= r.EndOffset {
		n = int(r.EndOffset) - int(r.CurOffset)
		err = io.EOF
	}
	r.CurOffset = r.CurOffset + int64(n)
	return n, err
}

func (r *FileSegmentReader) Close() error {
	return r.fh.Close()
}
