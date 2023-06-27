/*
Copyright (c) YugaByte, Inc.

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
// Implementation of the datastore interface for when the relevant data files are hosted on an azure blob storage
package datastore

import (
	"io"
	"net/url"
	"os"
	"strings"
	"fmt"
	"regexp"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/az"
)

type AzDataStore struct {
	url       	  *url.URL
}

func NewAzDataStore(dataDir string) *AzDataStore {
	url, err := url.Parse(dataDir)
	if err != nil {
		utils.ErrExit("invalid azure resource URL %v", dataDir)
	}
	return &AzDataStore{url: url}
}

// Search and return all keys within the bucket matching the giving pattern.
func (ds *AzDataStore) Glob(pattern string) ([]string, error) {
	objectNames, err := az.ListAllObjects(ds.url.String())
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

// No-op for Azure URLs.
func (ds *AzDataStore) AbsolutePath(filePath string) (string, error) {
	return filePath, nil
}

func (ds *AzDataStore) FileSize(filePath string) (int64, error) {
	headObject, err := az.GetHeadObject(filePath)
	if err != nil {
		return 0, fmt.Errorf("get head of object %q: %w",filePath, err)
	}
	return headObject.Size, nil
}

// Open the file at the given path for reading.
func (ds *AzDataStore) Open(objectPath string) (io.ReadCloser, error) {
	if strings.HasPrefix(objectPath, "https://") {
		return az.NewObjectReader(objectPath)
	}
	// if objectPath is hidden underneath a symlink for az blobs...
	objectPath, err := os.Readlink(objectPath)
	if err != nil {
		utils.ErrExit("unable to resolve symlink %v to gcs resource: %w", objectPath, err)
	}
	return az.NewObjectReader(objectPath)
}
