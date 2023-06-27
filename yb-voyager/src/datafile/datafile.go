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
package datafile

import (
	"fmt"
	"io"
	"regexp"
)

const (
	CSV  = "csv"
	SQL  = "sql"
	TEXT = "text"
)

type DataFile interface {
	SkipLines(numLines int64) error
	NextLine() (string, error)
	GetBytesRead() int64
	ResetBytesRead()
	GetHeader() string
	Close()
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

func NewDataFile(fileName string, reader io.ReadCloser, descriptor *Descriptor) (DataFile, error) {
	switch descriptor.FileFormat {
	case CSV:
		return newCsvDataFile(fileName, reader, descriptor)
	case TEXT:
		return newTextDataFile(fileName, reader, descriptor)
	case SQL:
		return newSqlDataFile(fileName, reader, descriptor)
	default:
		panic(fmt.Sprintf("Unknown file type %q", descriptor.FileFormat))

	}
}
