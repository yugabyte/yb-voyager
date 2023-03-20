package utils

import (
	"io"
	"time"
)

type TailReader struct {
	r io.Reader
}

func NewTailReader(r io.Reader) *TailReader {
	return &TailReader{r: r}
}

// Read the underlying io.Reader and return the contents.
// If the underlying reader returns io.EOF, keep on retrying until some data is available.
func (t *TailReader) Read(p []byte) (n int, err error) {
	for {
		n, err = t.r.Read(p)
		if n > 0 {
			return n, nil
		}
		if err != io.EOF {
			return 0, err
		}
		time.Sleep(1 * time.Second)
	}
}
