package libmig

import (
	"bufio"
	"io"
)

type Ora2pgFileSegmentReader struct {
	*FileSegmentReader
	insideCopyStmt bool

	scanner       *bufio.Scanner
	deferredBytes []byte
}

func NewOra2pgFileSegmentReader(
	fileName string, startOffset, endOffset int64, insideCopyStmt bool) (*Ora2pgFileSegmentReader, error) {

	segReader, err := NewFileSegmentReader(fileName, startOffset, endOffset)
	if err != nil {
		return nil, err
	}
	r := &Ora2pgFileSegmentReader{
		FileSegmentReader: segReader,
		insideCopyStmt:    insideCopyStmt,
		scanner:           bufio.NewScanner(segReader),
	}
	return r, nil
}

func (r *Ora2pgFileSegmentReader) Read(buf []byte) (n int, err error) {
	remaining := buf[:]

	outputLine := func(bs []byte) (deferredBytes []byte) {
		n := copy(remaining, bs)
		if n < len(bs) { // Fewer bytes were copied.
			deferredBytes = bs[n:]
		}
		remaining = remaining[n:]
		return
	}

	if len(r.deferredBytes) != 0 {
		r.deferredBytes = outputLine(r.deferredBytes)
		if len(remaining) == 0 {
			return len(buf), nil
		}
	}
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if r.isDataLine(line) {
			r.deferredBytes = outputLine([]byte(line + "\n"))
			if len(remaining) == 0 {
				break
			}
		}
	}

	if len(remaining) > 0 { // The for loop exited because Scan() returned false.
		err = r.scanner.Err()
		if err == nil {
			err = io.EOF
		}
	}
	return len(buf) - len(remaining), err
}

func (r *Ora2pgFileSegmentReader) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	if r.insideCopyStmt {
		if endOfCopy {
			r.insideCopyStmt = false
		}
		return !(emptyLine || newLineChar || endOfCopy)
	} else { // outside copy
		if reCopy.MatchString(line) {
			r.insideCopyStmt = true
		}
		return false
	}
}
