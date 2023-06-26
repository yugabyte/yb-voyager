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
// Implementation of the datastore interface for when the relevant data files are hosted on an gcs bucket.
package datastore

import (
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/gcs"
)

type GCSDataStore struct {
	url        *url.URL
	bucketName string
}

func NewGCSDataStore(resourceName string) *GCSDataStore {
	url, err := url.Parse(resourceName)
	if err != nil {
		utils.ErrExit("invalid gcs resource URL %v", resourceName)
	}
	return &GCSDataStore{url: url, bucketName: url.Host}
}

// Search and return all keys within the bucket matching the giving pattern.
func (ds *GCSDataStore) Glob(pattern string) ([]string, error) {
	objectNames, err := gcs.ListAllObjects(ds.url)
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
			resultSet = append(resultSet, objectName) 
		}
	}
	return resultSet, nil
}

// No-op for GCS URLs.
func (ds *GCSDataStore) AbsolutePath(filePath string) (string, error) {
	return filePath, nil
}

func (ds *GCSDataStore) FileSize(filePath string) (int64, error) {
	objAttrs, err := gcs.GetObjAttrs(filePath)
	if err != nil {
		return 0, fmt.Errorf("get attributes of %q: %w",filePath, err)
	}
	return objAttrs.Size, nil
}

func (ds *GCSDataStore) Open(resourceName string) (io.ReadCloser, error) {
	if strings.HasPrefix(resourceName, "gs://") {
		return gcs.NewObjectReader(resourceName)
	}
	// if resourceName is hidden underneath a symlink for gcs objects...
	objectPath, err := os.Readlink(resourceName)
	if err != nil {
		utils.ErrExit("unable to resolve symlink %v to gcs resource: %w", resourceName, err)
	}
	return gcs.NewObjectReader(objectPath)
}
