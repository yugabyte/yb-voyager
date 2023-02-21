// Implementation of datastore for when the relevant data files are available on the same machine running yb-voyager.
package datastore

import "path/filepath"

type LocalDatastore struct {
	dataDir string
}

func NewLocalDatastore(dataDir string) *LocalDatastore {
	return &LocalDatastore{dataDir: dataDir}
}

// Search and return all files in the dataDir matching the given pattern.
func (ds *LocalDatastore) Glob(pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(ds.dataDir, pattern))
}

func (ds *LocalDatastore) AbsolutePath(file string) (string, error) {
	return filepath.Abs(file)
}
