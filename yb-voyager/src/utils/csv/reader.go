package csv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// A single record in a CSV file cannot be larger than this.
// If there is such a record, override this value with the environment variable CSV_READER_MAX_BUFFER_SIZE_MB.
var CSV_READER_MAX_BUFFER_SIZE = 32 * 1024 * 1024

func init() {
	// Override the default max buffer size from value provided in the environment.
	envMaxBufSize := os.Getenv("CSV_READER_MAX_BUFFER_SIZE_MB")
	if envMaxBufSize != "" {
		maxBufSize, err := strconv.Atoi(envMaxBufSize)
		if err != nil {
			panic(fmt.Sprintf("Invalid value for CSV_READER_MAX_BUFFER_SIZE_BYTES: %q", envMaxBufSize))
		}
		CSV_READER_MAX_BUFFER_SIZE = maxBufSize
		fmt.Printf("CSV_READER_MAX_BUFFER_SIZE: %d bytes\n", CSV_READER_MAX_BUFFER_SIZE)
	}
}

type Reader struct {
	QuoteChar  byte
	EscapeChar byte

	fileName     string
	file         *os.File
	buf          []byte
	remainingBuf []byte
	pendingBytes []byte
	eof          bool
}

func Open(fileName string) (*Reader, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", fileName, err)
	}
	buf := make([]byte, CSV_READER_MAX_BUFFER_SIZE)
	r := &Reader{QuoteChar: '"', EscapeChar: '"', fileName: fileName, file: f, buf: buf}
	return r, nil
}

func (r *Reader) Close() error {
	return r.file.Close()
}

func (r *Reader) Read() (string, error) {
retry:

	if len(r.remainingBuf) == 0 {
		n1 := len(r.pendingBytes)
		if n1 > 0 {
			// We have some pending bytes from the previous read.
			// Copy them to the beginning of the buffer.
			copy(r.buf, r.pendingBytes)
			r.pendingBytes = r.pendingBytes[:0]
		}
		// Read the next chunk of the file.
		n2, err := r.file.Read(r.buf[n1:])
		n := n1 + n2 // Total number of valid bytes in the buffer.
		if err != nil {
			if err == io.EOF {
				r.eof = true
			} else {
				return "", fmt.Errorf("error reading file %s: %v", r.fileName, err)
			}
		}
		r.remainingBuf = r.buf[:n] // Consume the valid bytes from the buffer.
	}
	if len(r.remainingBuf) == 0 && r.eof {
		return "", io.EOF
	}
	line, remainingBuf, err := r.read(r.remainingBuf)
	if len(remainingBuf) == len(r.remainingBuf) { // No progress.
		return "", fmt.Errorf("unterminated quoted field in file %s", r.fileName)
	}
	if err == errEndOfBuffer {
		// We have a partial line in the buffer.
		// Copy it to the pending bytes and read the next chunk.
		r.pendingBytes = append(r.pendingBytes, remainingBuf...)
		r.remainingBuf = r.remainingBuf[:0]
		goto retry
	}
	r.remainingBuf = remainingBuf
	if line == "\n" {
		// Skip empty lines.
		goto retry
	}
	return line, nil
}

var errEndOfBuffer = errors.New("end of buffer")

func (r *Reader) read(buf []byte) (string, []byte, error) {
	i := 0
	for {
		if len(buf) == 0 { // Empty buffer.
			return "", nil, errEndOfBuffer
		}
		if i == len(buf) {
			// No newline found in the buffer.
			return "", buf, errEndOfBuffer
		}
		if buf[i] == '\n' {
			// Found a newline that is outside of a quoted field.
			line := string(buf[:i+1]) // including the newline.
			buf = buf[i+1:]           // reading after the newline.
			return line, buf, nil
		}
		if buf[i] != r.QuoteChar {
			i++
			continue
		}
		// Found a quote.
		i++ // Enter the quoted field.
		// Find the next unescaped quote.
		for ; i < len(buf); i++ {
			if buf[i] != r.QuoteChar {
				continue
			}
			// Found a quote.
			if r.QuoteChar == r.EscapeChar {
				if i+1 < len(buf) && buf[i+1] == r.QuoteChar {
					// The i'th quote is escaping the i+1'th quote.
					i++ // Skip the next quote as well.
				} else {
					break // Found the end of the quoted field.
				}
			} else {
				// Check for an escaped quote.
				escaped := buf[i-1] == r.EscapeChar && buf[i-2] != r.EscapeChar
				if !escaped {
					break
				}
			}
		}
		if i == len(buf) {
			return "", buf, errEndOfBuffer
		}
		i++
	}
}
