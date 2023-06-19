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
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

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

func createContainerClient(url string) (*container.Client, error) {
	var err error
	cred, err := createCredentials()
	if err != nil {
		utils.ErrExit("create azure blob shared key credential: %w", err)
	}
	containerClient, err := container.NewClient(url, cred, nil)
	if err != nil {
		return nil, err
	}
	return containerClient, nil
}

func ValidateObjectURL(datadir string) error {
	containerUrl, err := url.Parse(datadir)
	if err != nil {
		return err
	}
	container := containerUrl.Path
	if container == "" {
		return fmt.Errorf("missing bucket in azure blob url %v", datadir)
	}
	service := containerUrl.Host
	if service == "" {
		return fmt.Errorf("missing service in azure blob url %v", datadir)
	}
	return nil
}

func SplitObjectPath(objectPath string) (string, string, string, error) {
	err := ValidateObjectURL(objectPath)
	if err != nil {
		return "", "", "", err
	}
	objectUrl, err := url.Parse(objectPath)
	if err != nil {
		return "", "", "", err
	}
	serviceUrl := objectUrl.Host
	blobPath := objectUrl.Path[1:]
	container := strings.Split(blobPath, "/")[0]
	key := strings.TrimPrefix(blobPath, container)[1:] //remove the first "/"
	return serviceUrl, container, key, nil
}

func ListAllObjects(containerURL string) ([]string, error) {
	createClientIfNotExists(containerURL)
	var keys []string
	_, containerName, _, err := SplitObjectPath(containerURL)
	if err != nil {
		return nil, err
	}
	pager := client.NewListBlobsFlatPager(containerName, nil)
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, blob := range page.Segment.BlobItems {
			keys = append(keys, *blob.Name)
		}
	}
	return keys, nil
}

func GetHeadObject(objectURL string) (*blob.Attributes, error) {
	serviceName, containerName, key, err := SplitObjectPath(objectURL)
	if err != nil {
		return nil, err
	}
	url := "https://" + serviceName
	url = strings.Join([]string{url, containerName}, "/")
	containerClient, err := createContainerClient(url)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	bucket, err := azureblob.OpenBucket(ctx, containerClient, nil)
	if err != nil {
		return nil, err
	}
	defer bucket.Close()
	blobAttributes, err := bucket.Attributes(ctx, key)
	if err != nil {
		return nil, err
	}
	return blobAttributes, nil
}

func NewObjectReader(objectURL string) (io.ReadCloser, error) {
	createClientIfNotExists(objectURL)
	_, containerName, key, err := SplitObjectPath(objectURL)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	get, err := client.DownloadStream(ctx, containerName, key, nil)
	if err != nil {
		return nil, err
	}
	retryReader := get.NewRetryReader(ctx, &azblob.RetryReaderOptions{})
	return retryReader, nil
}
