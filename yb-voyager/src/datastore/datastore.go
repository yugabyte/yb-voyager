package datastore

import (
	"io"
	"strings"
)

type DataStore interface {
	Glob(string) ([]string, error)
	AbsolutePath(string) (string, error)
	FileSize(string) (int64, error)
	Join(...string) string
	Open(string) (io.ReadCloser, error)
}

func NewDataStore(location string) DataStore {
	if strings.HasPrefix(location, "s3://") {
		return NewS3DataStore(location)
	} else if strings.HasPrefix(location, "gs://") {
		return NewGCSDataStore(location)
	}
	return NewLocalDataStore(location)

}
