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
*/package srcdb

import (
	"fmt"
	"os"
	"path"
	"strings"
)

func findAllExecutablesInPath(executableName string) ([]string, error) {
	pathString := os.Getenv("PATH")
	if pathString == "" {
		return nil, fmt.Errorf("PATH environment variable is not set")
	}
	paths := strings.Split(pathString, string(os.PathListSeparator))
	var result []string
	for _, dir := range paths {
		fullPath := path.Join(dir, executableName)
		if _, err := os.Stat(fullPath); err == nil {
			result = append(result, fullPath)
		}
	}
	return result, nil
}
