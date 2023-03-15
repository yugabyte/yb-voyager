// Implementation of datastore for when the relevant data files are available on the same machine running yb-voyager.
package datastore

import (
	"io"
	"os"
	"path/filepath"
)

type LocalDataStore struct {
	dataDir string
}

func NewLocalDataStore(dataDir string) *LocalDataStore {
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

func (ds *LocalDataStore) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (ds *LocalDataStore) Open(filePath string) (io.ReadCloser, error) {
	return os.Open(filePath)
}
