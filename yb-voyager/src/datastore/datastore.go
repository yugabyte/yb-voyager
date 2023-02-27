package datastore

import (
	"io"
	"strings"
)

type Datastore interface {
	Glob(string) ([]string, error)
	AbsolutePath(string) (string, error)
	FileSize(string) (int64, error)
	Join(...string) string
	Open() io.ReadCloser
}

func NewDataStore(dataDir string) Datastore {
	if strings.HasPrefix(dataDir, "s3://") {
		return NewS3Datastore(dataDir)
	}
	return NewLocalDatastore(dataDir)
}
