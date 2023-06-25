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
	  default:
		return NewLocalDataStore(location)
 	}

}
