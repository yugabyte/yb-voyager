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
package datastore

import (
	"io"
	"strings"
)

type DataStore interface {
	Glob(string) ([]string, error)
	AbsolutePath(string) (string, error)
	FileSize(string) (int64, error)
	Open(string) (io.ReadCloser, error)
}

func NewDataStore(location string) DataStore {
	switch true {
	  case strings.HasPrefix(location, "s3://"):
		return NewS3DataStore(location)
	  case strings.HasPrefix(location, "gs://"):
		return NewGCSDataStore(location)
	  case strings.HasPrefix(location, "https://") 
		return NewAzDataStore(location)
	  default:
		return NewLocalDataStore(location)
 	}

}
