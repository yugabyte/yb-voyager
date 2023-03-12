package datastore

import (
	"io"
	"os"
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
	if strings.HasPrefix(location, "s3://") {
		return NewS3Datastore(location)
	}
	trueLocation, err := os.Readlink(location) //need to differentiate between actual DNE case and symlink case somehow...
	if err != nil {
		utils.ErrExit("unable to resolve location of datastore %v", err)
	}
	if strings.HasPrefix(trueLocation, "s3://") {
		return NewS3Datastore(trueLocation)
	}
	return NewLocalDatastore(trueLocation)
}
