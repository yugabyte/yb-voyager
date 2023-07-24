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
package dbzm

import (
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)




func IsDebeziumForDataExport(exportDir string) bool {
	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json") //checking if this file exists to determine if debezium is being used
	return utils.FileOrFolderExists(exportStatusFilePath)
}