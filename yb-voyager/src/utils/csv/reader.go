package csv

import (
	"fmt"
	"io"
	"os"

	"github.com/edsrzf/mmap-go"
)

type Reader struct {
	QuoteChar  byte
	EscapeChar byte

	fileName string
	file     *os.File
	mmap     mmap.MMap
	buf      []byte
}

func Open(fileName string) (*Reader, error) {
	// Create a read-only mmap of the file.
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	mmap_, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}
	r := &Reader{QuoteChar: '"', EscapeChar: '"', fileName: fileName, file: f, mmap: mmap_, buf: []byte(mmap_)}
	return r, nil
}

func (r *Reader) Close() error {

	err := r.mmap.Unmap()
	if err != nil {
		return err
	}
	return r.file.Close()
}

func (r *Reader) Read() (string, error) {
	i := 0
	for {
		if len(r.buf) == 0 { // Empty buffer.
			return "", io.EOF
		}
		if i == len(r.buf) {
			// No newline found in the buffer.
			// Return the entire remaining buffer as the last record.
			line := string(r.buf)
			r.buf = r.buf[:0]
			return line, nil
		}
		if r.buf[i] == '\n' {
			// Found a newline that is outside of a quoted field.
			line := string(r.buf[:i+1]) // including the newline.
			r.buf = r.buf[i+1:]         // reading after the newline.
			if len(line) == 0 {         // Skip empty lines.
				i = 0 // Reset the index to the beginning of the buffer.
				continue
			}
			return line, nil
		}
		if r.buf[i] != r.QuoteChar {
			i++
			continue
		}
		// Found a quote.
		i++ // Enter the quoted field.
		// Find the next unescaped quote.
		for ; i < len(r.buf); i++ {
			if r.buf[i] != r.QuoteChar {
				continue
			}
			// Found a quote.
			if r.QuoteChar == r.EscapeChar {
				if i+1 < len(r.buf) && r.buf[i+1] == r.QuoteChar {
					// The i'th quote is escaping the i+1'th quote.
					i++ // Skip the next quote as well.
				} else {
					break // Found the end of the quoted field.
				}
			} else {
				// Check for an escaped quote.
				escaped := r.buf[i-1] == r.EscapeChar && r.buf[i-2] != r.EscapeChar
				if !escaped {
					break
				}
			}
		}
		if i == len(r.buf) {
			return "", fmt.Errorf("unterminated quoted field")
		}
		i++
	}
}
