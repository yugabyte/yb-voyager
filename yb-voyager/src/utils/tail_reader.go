/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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

func (t *TailReader) ReadLine() ([]byte, error) {
	// read line by line
	buf := make([]byte, 0, 4096)
	for {
		b := make([]byte, 1)
		_, err := t.Read(b)
		if err != nil {
			return buf, err
		}
		if b[0] == '\n' {
			break
		}
		buf = append(buf, b[0])
		if string(buf) == `\.` {
			return buf, io.EOF
		}
	}
	return buf, nil
}
