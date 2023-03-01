package datastore

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Datastore interface {
	Glob(string) ([]string, error)
	AbsolutePath(string) (string, error)
	FileSize(string) (int64, error)
	Join(...string) string
	Open(string) (io.ReadCloser, error)
}

func NewDataStore(location string) Datastore {
	trueLocation, err := filepath.EvalSymlinks(location)
	if err != nil {
		utils.ErrExit("unable to resolve location of datastore")
	}
	if strings.HasPrefix(trueLocation, "s3://") {
		return NewS3Datastore(trueLocation)
	}
	return NewLocalDatastore(trueLocation)
}
