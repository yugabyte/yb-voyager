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
package az

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"gocloud.dev/blob"
	"gocloud.dev/blob/azureblob"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var client *azblob.Client

func createCredentials() (*azidentity.DefaultAzureCredential, error) {
	// cred represents the default Oauth token used to authenticate the account in the url.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

// creates a client for the account in the url with the default creds.
func createClientIfNotExists(datadir string) {
	var err error
	url, err := url.Parse(datadir)
	if err != nil {
		utils.ErrExit("parse azure blob url: %w", err)
	}
	serviceUrl := "https://" + url.Host
	cred, err := createCredentials()
	if err != nil {
		utils.ErrExit("create azure blob shared key credential: %w", err)
	}
	client, err = azblob.NewClient(serviceUrl, cred, nil)
	if err != nil {
		utils.ErrExit("create azure blob client: %w", err)
	}
}

// creates a client to access the blob properties for the blob in the container
// (or bucket) under the account in the url with the default creds.
func createContainerClient(url string) (*container.Client, error) {
	var err error
	cred, err := createCredentials()
	if err != nil {
		utils.ErrExit("create azure blob shared key credential: %w", err)
	}
	containerClient, err := container.NewClient(url, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create azure blob container client: %w", err)
	}
	return containerClient, nil
}

// check if url is in format
// https://<account_name>.blob.core.windows.net/<container_name or bucket_name>
func ValidateObjectURL(datadir string) error {
	containerUrl, err := url.Parse(datadir)
	if err != nil {
		return fmt.Errorf("parsing the object of %q: %w", datadir, err)
	}
	container := containerUrl.Path
	if container == "" {
		return fmt.Errorf("missing bucket in azure blob url %v", datadir)
	}
	service := containerUrl.Host
	if service == "" {
		return fmt.Errorf("missing service in azure blob url %v", datadir)
	} else if !strings.Contains(service, ".blob.") {
		return fmt.Errorf("invalid service in azure blob url %v", datadir)
	}
	return nil
}

func splitObjectPath(objectPath string) (string, string, string, error) {
	err := ValidateObjectURL(objectPath)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid azure blob url %v: %w", objectPath, err)
	}
	objectUrl, err := url.Parse(objectPath)
	if err != nil {
		return "", "", "", fmt.Errorf("parsing the object of %q: %w", objectPath, err)
	}
	serviceUrl := objectUrl.Host
	blobPath := objectUrl.Path[1:]
	container := strings.Split(blobPath, "/")[0]
	key := ""
	if len(blobPath) > len(container) {
		key = strings.TrimPrefix(blobPath, container)[1:] //remove the first "/"
	}
	return serviceUrl, container, key, nil
}

func ListAllObjects(containerURL string) ([]string, error) {
	createClientIfNotExists(containerURL)
	var keys []string
	_, containerName, key, err := splitObjectPath(containerURL)
	if err != nil {
		return nil, fmt.Errorf("splitting object path of %q: %w", containerURL, err)
	}
	options := &container.ListBlobsFlatOptions{}
	if key != "" {
		options = &container.ListBlobsFlatOptions{Prefix: &key}
	}
	pager := client.NewListBlobsFlatPager(containerName, options)
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("listing all objects of %q: %w", containerURL, err)
		}
		for _, blob := range page.Segment.BlobItems {
			objectName := *blob.Name
			if key != "" {
				objectName = strings.TrimPrefix(objectName, key)[1:] //remove the first "/"
			}
			keys = append(keys, objectName)
		}
	}
	return keys, nil
}

func GetHeadObject(objectURL string) (*blob.Attributes, error) {
	serviceName, containerName, key, err := splitObjectPath(objectURL)
	if err != nil {
		return nil, fmt.Errorf("splitting object path of %q: %w", objectURL, err)
	}
	url := "https://" + serviceName
	url = strings.Join([]string{url, containerName}, "/")
	containerClient, err := createContainerClient(url)
	if err != nil {
		return nil, fmt.Errorf("creating container client for %q: %w", url, err)
	}
	ctx := context.Background()
	// using OpenBucket API to get the attributes of the blob in the container
	bucket, err := azureblob.OpenBucket(ctx, containerClient, nil)
	if err != nil {
		return nil, fmt.Errorf("opening bucket for %q: %w", url, err)
	}
	defer bucket.Close()
	blobAttributes, err := bucket.Attributes(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("getting attributes of %q: %w", objectURL, err)
	}
	return blobAttributes, nil
}

func NewObjectReader(objectURL string) (io.ReadCloser, error) {
	createClientIfNotExists(objectURL)
	_, containerName, key, err := splitObjectPath(objectURL)
	if err != nil {
		return nil, fmt.Errorf("splitting object path of %q: %w", objectURL, err)
	}
	ctx := context.Background()
	get, err := client.DownloadStream(ctx, containerName, key, nil)
	if err != nil {
		return nil, fmt.Errorf("downloading stream of %q: %w", objectURL, err)
	}
	retryReader := get.NewRetryReader(ctx, &azblob.RetryReaderOptions{})
	return retryReader, nil
}
