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
package utils

import (
	"context"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func VerifyS3FromDataDir(datadir string) error {
	u, err := url.Parse(datadir)
	if err != nil {
		return err
	}
	bucket := u.Host
	if bucket == "" {
		return fmt.Errorf("missing bucket in s3 url")
	}
	return nil
}

func ListAllS3Objects(bucket string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		ErrExit("load s3 config: %v", err)
	}
	client := s3.NewFromConfig(cfg)

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
			ErrExit("failed to get page %v, %v", i, err)
		}
		// Log the objects found
		for _, obj := range page.Contents {
			objectNames = append(objectNames, *obj.Key)
		}
	}
	return objectNames, nil
}
