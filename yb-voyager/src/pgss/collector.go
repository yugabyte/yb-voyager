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
package pgss

import (
	"database/sql"
	"fmt"
)

// CollectFromPostgreSQL collects PGSS data directly from a PostgreSQL database or assessmentDB
// TODO: Implement when compare-performance command is developed
func CollectFromPostgreSQL(db *sql.DB, schemaName string) ([]QueryStats, error) {
	return nil, fmt.Errorf("CollectFromPostgreSQL not implemented yet - will be added for compare-performance command")
}

// CollectFromYugabyteDB collects PGSS data from YugabyteDB
// TODO: Implement when compare-performance command is developed
func CollectFromYugabyteDB(db *sql.DB, schemaName string) ([]QueryStats, error) {
	return nil, fmt.Errorf("CollectFromYugabyteDB not implemented yet - will be added for compare-performance command")
}
