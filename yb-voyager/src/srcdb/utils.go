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

	goerrors "github.com/go-errors/errors"
)

func findAllExecutablesInPath(executableName string) ([]string, error) {
	pathString := os.Getenv("PATH")
	if pathString == "" {
		return nil, goerrors.Errorf("PATH environment variable is not set")
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

func CheckSchemasHaveUsagePermissions(source *Source, isLiveMigration bool) error {
	schemasMissingUsage, err := source.DB().GetSchemasMissingUsagePermissions()
	if err != nil {
		return fmt.Errorf("get schemas missing usage permissions: %w", err)
	}
	if len(schemasMissingUsage) == 0 {
		return nil
	}

	link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
	if isLiveMigration {
		link = "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-migrate/#prepare-the-source-database"
	}
	return goerrors.Errorf("missing USAGE permission for user %s on schemas: [%s]\nCheck the documentation to prepare the database for migration: %s",
		source.User, strings.Join(schemasMissingUsage, ", "), link)
}
