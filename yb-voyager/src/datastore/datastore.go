package datastore

import "strings"

type Datastore interface {
	Glob(string) ([]string, error)
	AbsolutePath(string) (string, error)
}

type TableToFileMap map[string]string

// var (
// 	TableToFileMap map[string]string
// )

func NewDataStore(dataDir string) Datastore {
	if strings.HasPrefix(dataDir, "s3://") {
		return NewS3Datastore(dataDir)
	}
	return NewLocalDatastore(dataDir)
}
