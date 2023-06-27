/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Implementation of the datastore interface for when the relevant data files are hosted on an s3 bucket.
package datastore

import (
	"io"
	"net/url"
	"os"
	"fmt"
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
		utils.ErrExit("invalid s3 resource URL %v", resourceName)
	}
	return &S3DataStore{url: url, bucketName: url.Host}
}

// Search and return all keys within the bucket matching the giving pattern.
func (ds *S3DataStore) Glob(pattern string) ([]string, error) {
	objectNames, err := s3.ListAllObjects(ds.bucketName)
	if err != nil {
		return nil, fmt.Errorf("listing all objects of %q: %w", pattern, err)
	}
	pattern = strings.Replace(pattern, "*", ".*", -1)
	pattern = ds.url.String() + "/" + pattern
	re := regexp.MustCompile(pattern)
	var resultSet []string
	for _, objectName := range objectNames {
		objectName = ds.url.String() + "/" + objectName
		if re.MatchString(objectName) {
			resultSet = append(resultSet, objectName) // Simulate /path/to/data-dir/file behaviour.
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
		return 0, fmt.Errorf("get head object of %q: %w", filePath, err)
	}
	return headObject.ContentLength, nil
}

func (ds *S3DataStore) Open(resourceName string) (io.ReadCloser, error) {
	if strings.HasPrefix(resourceName, "s3://") {
		return s3.NewObjectReader(resourceName)
	}
	// if resourceName is hidden underneath a symlink for s3 objects...
	objectPath, err := os.Readlink(resourceName)
	if err != nil {
		utils.ErrExit("unable to resolve symlink %v to s3 resource: %w", resourceName, err)
	}
	return s3.NewObjectReader(objectPath)
}
