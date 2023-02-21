// Implementation of the datastore interface for when the relevant data files are hosted on an s3 bucket.
package datastore

import (
	"net/url"
	"regexp"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type S3Datastore struct {
	url            *url.URL
	bucketName     string
	TableToFileMap map[string]string
}

func NewS3Datastore(resourceName string) *S3Datastore {
	url, err := url.Parse(resourceName)
	if err != nil {
		utils.ErrExit("invalid s3 resource URL")
	}
	return &S3Datastore{url: url, bucketName: url.Host}
}

// Search and return all keys within the bucket matching the giving pattern.
func (ds *S3Datastore) Glob(pattern string) ([]string, error) {
	allKeys, err := utils.ListAllS3Objects(ds.bucketName)
	if err != nil {
		return nil, err
	}
	regexPattern := regexp.MustCompile(pattern)
	var resultSet []string
	for _, value := range allKeys {
		if regexPattern.MatchString(value) {
			resultSet = append(resultSet, ds.url.String()+"/"+value) // Simulate /path/to/data-dir/file behaviour.
		}
	}
	return resultSet, nil
}

// No-op for S3 URLs.
func (ds *S3Datastore) AbsolutePath(file string) (string, error) {
	return file, nil
}
