// Implementation of the datastore interface for when the relevant data files are hosted on an azure blob storage
package datastore

import (
	"io"
	"net/url"
	"os"
	"strings"
	"regexp"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/az"
)

type AzDataStore struct {
	url        *url.URL
	bucketName string
}

func NewAzDataStore(resourceName string) *AzDataStore {
	url, err := url.Parse(resourceName)
	if err != nil {
		utils.ErrExit("invalid azure resource URL %v", resourceName)
	}
	return &AzDataStore{url: url, bucketName: url.Path[1:]}
}

// Search and return all keys within the bucket matching the giving pattern.
func (ds *AzDataStore) Glob(pattern string) ([]string, error) {
	allKeys, err := az.ListAllObjects(ds.url.String())
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

// No-op for Azure URLs.
func (ds *AzDataStore) AbsolutePath(filePath string) (string, error) {
	return filePath, nil
}

func (ds *AzDataStore) FileSize(filePath string) (int64, error) {
	headObject, err := az.GetHeadObject(filePath)
	if err != nil {
		return 0, err
	}
	return headObject.Size, nil
}

// filepath.Join joins elements with the "/" path separator.
func (ds *AzDataStore) Join(elem ...string) string {
	finalPath := ""
	finalPath += strings.Join(elem, "/")
	return finalPath
}

// Open the file at the given path for reading.
func (ds *AzDataStore) Open(resourceName string) (io.ReadCloser, error) {
	if strings.HasPrefix(resourceName, "https://") {
		return az.NewObjectReader(resourceName)
	}
	// if resourceName is hidden underneath a symlink for az blobs...
	objectPath, err := os.Readlink(resourceName)
	if err != nil {
		return nil, err
	}
	return az.NewObjectReader(objectPath)
}
