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
package importdata

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type ImportFileTask struct {
	ID           int
	FilePath     string
	TableNameTup sqlname.NameTuple
	RowCount     int64
	FileSize     int64
}

func (task *ImportFileTask) String() string {
	return fmt.Sprintf("{ID: %d, FilePath: %s, TableName: %s, RowCount: %d, FileSize: %d}", task.ID, task.FilePath, task.TableNameTup.ForOutput(), task.RowCount, task.FileSize)
}
