package libmig

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
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
		return nil, fmt.Errorf("open %s: %w", fileName, err)
	}
	if endOffset == -1 {
		fi, err := fh.Stat()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", fileName, err)
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
	log.Infof("New segment reader created %s [%v -> %v]", fileName, startOffset, endOffset)
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
