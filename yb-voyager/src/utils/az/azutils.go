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

// creates a client for the account in the url with the default creds.
func createClientIfNotExists(dataDir string) {
	var err error
	url, err := url.Parse(dataDir)
	if err != nil {
		utils.ErrExit("parse azure blob url for dataDir %s: %w", dataDir, err)
	}
	serviceUrl := "https://" + url.Host
	// cred represents the default Oauth token used to authenticate the account in the url.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
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
	// cred represents the default Oauth token used to authenticate the account in the url.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
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
func ValidateObjectURL(dataDir string) error {
	dataDirUrl, err := url.Parse(dataDir)
	if err != nil {
		return fmt.Errorf("parsing the object of %q: %w", dataDir, err)
	}
	containerDir := dataDirUrl.Path
	if containerDir == "" {
		return fmt.Errorf("missing bucket in azure blob url %v", dataDir)
	}
	service := dataDirUrl.Host
	if service == "" {
		return fmt.Errorf("missing service in azure blob url %v", dataDir)
	} else if !strings.Contains(service, ".blob.") {
		return fmt.Errorf("invalid service in azure blob url %v", dataDir)
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

func ListAllObjects(dataDirUrl string) ([]string, error) {
	createClientIfNotExists(dataDirUrl)
	var keys []string
	_, containerName, key, err := splitObjectPath(dataDirUrl)
	if err != nil {
		return nil, fmt.Errorf("splitting object path of %q: %w", dataDirUrl, err)
	}
	options := &container.ListBlobsFlatOptions{}
	if key != "" {
		options = &container.ListBlobsFlatOptions{Prefix: &key}
	}
	pager := client.NewListBlobsFlatPager(containerName, options)
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("listing all objects of %q: %w", dataDirUrl, err)
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
	url := fmt.Sprintf("https://%s/%s", serviceName, containerName)
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
		return nil, fmt.Errorf("create download stream for %q: %w", objectURL, err)
	}
	retryReader := get.NewRetryReader(ctx, &azblob.RetryReaderOptions{MaxRetries: 10})
	return retryReader, nil
}
