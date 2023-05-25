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
package gcs

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"cloud.google.com/go/storage"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var client *storage.Client

func createClientIfNotExists() {
	if client != nil {
		return
	}
	// Creates a client that can only view public data - to be changed in final iteration after testing
	c, err := storage.NewClient(context.Background(), option.WithoutAuthentication())
	if err != nil {
		utils.ErrExit("create gcs client: %w", err)
	}
	client = c

}

func ValidateObjectURL(datadir string) error {
	u, err := url.Parse(datadir)
	if err != nil {
		return err
	}
	bucket := u.Host
	if bucket == "" {
		return fmt.Errorf("missing bucket in gcs url %v", datadir)
	}
	return nil
}

func SplitObjectPath(objectPath string) (string, string, error) {
	u, err := url.Parse(objectPath)
	if err != nil {
		return "", "", err
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

func ListAllObjects(bucket string) ([]string, error) {
	createClientIfNotExists()
	p := client.Bucket(bucket).Objects(context.Background(), nil)
	var objectNames []string
	for obj, err := p.Next(); err != iterator.Done; {
		if err != nil {
			return nil, err
		}
		fmt.Println(obj.Name)

		objectNames = append(objectNames, obj.Name)
	}
	return objectNames, nil
}

func GetObjAttrs(object string) (*storage.ObjectAttrs, error) {
	var objattrs *storage.ObjectAttrs
	createClientIfNotExists()
	bucket, key, err := SplitObjectPath(object)
	if err != nil {
		return nil, err
	}
	objattrs, err = client.Bucket(bucket).Object(key).Attrs(context.Background())
	if err != nil {
		return nil, err
	}
	return objattrs, nil
}

func NewObjectReader(object string) (io.ReadCloser, error) {
	createClientIfNotExists()
	bucketName, keyName, err := SplitObjectPath(object)
	if err != nil {
		return nil, err
	}
	objRef := client.Bucket(bucketName).Object(keyName)
	r, err := objRef.NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	return r, nil
}
