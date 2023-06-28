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
package gcs

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var client *storage.Client

func createClientIfNotExists() {
	if client != nil {
		return
	}
	// Creates a client with default credentials.
	c, err := storage.NewClient(context.Background())
	if err != nil {
		utils.ErrExit("create gcs client: %w", err)
	}
	client = c

}

func ValidateObjectURL(datadir string) error {
	u, err := url.Parse(datadir)
	if err != nil {
		return fmt.Errorf("parsing the object of %q: %w", datadir, err)
	}
	bucket := u.Host
	if bucket == "" {
		return fmt.Errorf("missing bucket in gcs url %v", datadir)
	}
	return nil
}

// TODO: factor out the duplicate code in these packages
func splitObjectPath(objectPath string) (string, string, error) {
	u, err := url.Parse(objectPath)
	if err != nil {
		return "", "", fmt.Errorf("parse URL %s: %w", objectPath, err)
	}
	bucket := u.Host
	key := u.Path[1:] //remove initial "/", unable to find object with it
	if bucket == "" {
		return "", "", fmt.Errorf("missing bucket in gcs url %v", objectPath)
	}
	if key == "" {
		return "", "", fmt.Errorf("missing key in gcs url %v", objectPath)
	}
	return bucket, key, nil
}

func ListAllObjects(url *url.URL) ([]string, error) {
	createClientIfNotExists()
	bucket := url.Host
	prefix := ""
	query := &storage.Query{}
	if url.Path != "" {
		prefix = url.Path[1:] //remove initial "/", unable to find object with it
		query = &storage.Query{Prefix: prefix}
	}
	objectIter := client.Bucket(bucket).Objects(context.Background(), query)
	var objectNames []string
	for {
		attrs, err := objectIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return objectNames, fmt.Errorf("Bucket(%q).Objects: %w", bucket, err)
		}
		objectName := attrs.Name
		if prefix != "" {
			objectName = strings.TrimPrefix(attrs.Name, prefix)
			objectName = strings.TrimPrefix(objectName, "/") //remove initial "/"
		}
		objectNames = append(objectNames, objectName)
	}
	return objectNames, nil
}

func GetObjAttrs(object string) (*storage.ObjectAttrs, error) {
	var objattrs *storage.ObjectAttrs
	createClientIfNotExists()
	bucket, key, err := splitObjectPath(object)
	if err != nil {
		return nil, fmt.Errorf("split object path of %q: %w", object, err)
	}
	objattrs, err = client.Bucket(bucket).Object(key).Attrs(context.Background())
	if err != nil {
		return nil, fmt.Errorf("attributes of object %q: %w", object, err)
	}
	return objattrs, nil
}

func NewObjectReader(object string) (io.ReadCloser, error) {
	createClientIfNotExists()
	bucketName, keyName, err := splitObjectPath(object)
	if err != nil {
		return nil, fmt.Errorf("split object path of %q: %w", object, err)
	}
	r, err := client.Bucket(bucketName).Object(keyName).NewReader(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get reader for %q: %w", object, err)
	}
	return r, nil
}
