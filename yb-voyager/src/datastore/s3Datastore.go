// Implementation of the datastore interface for when the relevant data files are hosted on an s3 bucket.
package datastore

import (
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/s3"
)

type S3DataStore struct {
	url        *url.URL
	bucketName string
}

func NewS3DataStore(resourceName string) *S3DataStore {
	url, err := url.Parse(resourceName)
	if err != nil {
		utils.ErrExit("invalid s3 resource URL")
	}
	return &S3DataStore{url: url, bucketName: url.Host}
}

// Search and return all keys within the bucket matching the giving pattern.
func (ds *S3DataStore) Glob(pattern string) ([]string, error) {
	allKeys, err := s3.ListAllObjects(ds.bucketName)
	if err != nil {
		return nil, err
	}
	regexPattern := regexp.MustCompile(strings.Replace(pattern, "*", ".*", -1))
	var resultSet []string
	for _, value := range allKeys {
		if regexPattern.MatchString(value) {
			resultSet = append(resultSet, ds.url.String()+"/"+value) // Simulate /path/to/data-dir/file behaviour.
		}
	}
	return resultSet, nil
}

// No-op for S3 URLs.
func (ds *S3DataStore) AbsolutePath(filePath string) (string, error) {
	return filePath, nil
}

func (ds *S3DataStore) FileSize(filePath string) (int64, error) {
	headObject, err := s3.GetHeadObject(filePath)
	if err != nil {
		return 0, err
	}
	return headObject.ContentLength, nil
}

// filepath.Join converts URL (s3://...) to directory-like string (s3:/...).
func (ds *S3DataStore) Join(elem ...string) string {
	finalPath := ""
	finalPath += strings.Join(elem, "/")
	return finalPath
}

func (ds *S3DataStore) Open(resourceName string) (io.ReadCloser, error) {
	// resourceName is hidden underneath a symlink for s3 objects.
	s3Resource, err := os.Readlink(resourceName)
	if err != nil {
		utils.ErrExit("unable to resolve symlink %v to s3 resource: %v", resourceName, err)
	}
	reader, err := s3.OpenReadableObject(s3Resource)
	return reader, err
}
