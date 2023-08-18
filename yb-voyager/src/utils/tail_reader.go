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
	"fmt"
	"io"
	"time"
)

type TailReader struct {
	r                    io.Reader
	bytesRead            int64
	getLastValidOffsetFn func() (int64, error)
}

func NewTailReader(r io.Reader, getLastValidOffsetFn func() (int64, error)) *TailReader {
	return &TailReader{r: r, getLastValidOffsetFn: getLastValidOffsetFn}
}

// Read the underlying io.Reader and return the contents.
// If the underlying reader returns io.EOF, keep on retrying until some data is available.
func (t *TailReader) Read(p []byte) (n int, err error) {
	for {
		lastOffset, err := t.getLastValidOffsetFn()
		if err != nil {
			return 0, fmt.Errorf("failed to get last valid offset: %w", err)
		}
		if t.bytesRead == lastOffset {
			time.Sleep(1 * time.Second)
			continue
		}
		if lastOffset-t.bytesRead < int64(len(p)) {
			p = p[:lastOffset-t.bytesRead]
		}

		n, err = t.r.Read(p)
		t.bytesRead += int64(n)
		return n, err
	}
}
