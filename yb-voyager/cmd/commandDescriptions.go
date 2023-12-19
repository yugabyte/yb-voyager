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

package cmd

import (
	"fmt"
	"strings"
)

const (
	EXPORT_DATA_SHORT_DESC = `Export tables' data (either snapshot-only or snapshot-and-changes) from source database to export-dir.
Note: For Oracle and MySQL, there is a beta feature to speed up the data export of snapshot, set the environment variable BETA_FAST_DATA_EXPORT=1 to try it out. 
You can refer to YB Voyager Documentation (https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#accelerate-data-export-for-mysql-and-oracle) for more details on this feature.
For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/data-migration/export-data/
Also export data(changes) from target YugabyteDB in the fall-back/fall-forward workflows.`

	EXPORT_SCHEMA_SHORT_DESC = `Export schema from source database into export-dir as .sql files
For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/export-schema/`
)

func formatCommandDescriptionString(input string) string {
	lines := strings.Split(input, "\n")
	maxLength := 0

	// Find the length of the longest line
	for _, line := range lines {
		if len(line) > maxLength {
			maxLength = len(line)
		}
	}

	// Create formatted output with aligned lines
	var formattedOutput strings.Builder
	for _, line := range lines {
		formattedOutput.WriteString(fmt.Sprintf("%-*s\n", maxLength, line))
	}

	return formattedOutput.String()
}
