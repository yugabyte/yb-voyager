package readcloser

import (
	"bufio"
	"os"
)

type TextReadCloser struct {
	file   *os.File
	reader *bufio.Reader
}

func NewTextReadCloser() *TextReadCloser {
	return &TextReadCloser{}
}

func (rc *TextReadCloser) Close() error {
	return rc.file.Close()
}

func (rc *TextReadCloser) Open(filePath string) error {
	file, err := os.Open(filePath)
	rc.file = file
	rc.reader = bufio.NewReader(file)
	return err
}

func (rc *TextReadCloser) ReadString(delim byte) (string, error) {
	return rc.reader.ReadString(delim)
}
