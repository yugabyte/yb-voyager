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
// Implementation of datastore for when the relevant data files are available on the same machine running yb-voyager.
package datastore

import (
	"io"
	"os"
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type LocalDataStore struct {
	dataDir string
}

func NewLocalDataStore(dataDir string) *LocalDataStore {
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		utils.ErrExit("failed to get absolute path of directory %q: %s", dataDir, err)
	}
	dataDir = filepath.Clean(dataDir)
	return &LocalDataStore{dataDir: dataDir}
}

// Search and return all files in the dataDir matching the given pattern.
func (ds *LocalDataStore) Glob(pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(ds.dataDir, pattern))
}

func (ds *LocalDataStore) AbsolutePath(file string) (string, error) {
	return filepath.Abs(file)
}

func (ds *LocalDataStore) FileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

func (ds *LocalDataStore) Open(filePath string) (io.ReadCloser, error) {
	return os.Open(filePath)
}
