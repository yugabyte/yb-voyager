// Implementation of the datastore interface for when the relevant data files are hosted on an gcs bucket.
package datastore

import (
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

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
	allKeys, err := gcs.ListAllObjects(ds.bucketName)
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

// No-op for GCS URLs.
func (ds *GCSDataStore) AbsolutePath(filePath string) (string, error) {
	return filePath, nil
}

func (ds *GCSDataStore) FileSize(filePath string) (int64, error) {
	objAttrs, err := gcs.GetObjAttrs(filePath)
	if err != nil {
		return 0, err
	}
	return objAttrs.Size, nil
}

// filepath.Join converts URL (gs://...) to directory-like string (gs:/...).
func (ds *GCSDataStore) Join(elem ...string) string {
	finalPath := ""
	finalPath += strings.Join(elem, "/")
	return finalPath
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
