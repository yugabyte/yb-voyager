package readcloser

import (
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/s3"
)

type S3TextReadCloser struct {
	streamer *s3.S3DownloadStream
}

func NewS3TextReadCloser() *S3TextReadCloser {
	return &S3TextReadCloser{}
}

func (rc *S3TextReadCloser) Close() error {
	return rc.streamer.Close()
}

func (rc *S3TextReadCloser) Open(filePath string) error {
	//TODO
	return nil
}

func (rc *S3TextReadCloser) ReadString(delim byte) (string, error) {
	//TODO
	return "", nil
}
