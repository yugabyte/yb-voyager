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
package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"gocloud.dev/blob/s3blob"
)

var client *s3.Client

func createClientIfNotExists() {
	if client != nil {
		return
	}
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		utils.ErrExit("load s3 config: %w", err)
	}
	client = s3.NewFromConfig(cfg)
}

func ValidateObjectURL(datadir string) error {
	u, err := url.Parse(datadir)
	if err != nil {
		return err
	}
	bucket := u.Host
	if bucket == "" {
		return fmt.Errorf("missing bucket in s3 url %v", datadir)
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
		return "", "", fmt.Errorf("missing bucket in s3 url %v", objectPath)
	}
	if key == "" {
		return "", "", fmt.Errorf("missing key in s3 url %v", objectPath)
	}
	return bucket, key, nil
}

func ListAllObjects(bucket string) ([]string, error) {
	createClientIfNotExists()
	// Use paginator, default list objects API has a fetch limit.
	p := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: &bucket,
	})

	var i int
	var objectNames []string
	for p.HasMorePages() {
		i++
		page, err := p.NextPage(context.TODO())
		if err != nil {
			utils.ErrExit("failed to get page %v, %w", i, err)
		}
		// Log the objects found
		for _, obj := range page.Contents {
			objectNames = append(objectNames, *obj.Key)
		}
	}
	return objectNames, nil
}

func GetHeadObject(object string) (*s3.HeadObjectOutput, error) {
	createClientIfNotExists()
	bucket, key, err := SplitObjectPath(object)
	if err != nil {
		return nil, err
	}
	headObj := s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	result, err := client.HeadObject(context.Background(), &headObj)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func NewObjectReader(object string) (io.ReadCloser, error) {
	createClientIfNotExists()
	bucketName, keyName, err := SplitObjectPath(object)
	if err != nil {
		return nil, err
	}
	bucket, err := s3blob.OpenBucketV2(context.Background(), client, bucketName, nil)
	if err != nil {
		utils.ErrExit("open bucket: %w", err)
	}
	return bucket.NewReader(context.Background(), keyName, nil)
}
