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
package types

// QueryStats represents database-agnostic query performance metrics
// placed here to avoid import cycle issue among compareperf and migassessment
type QueryStats struct {
	QueryID         int64   `json:"queryid" db:"queryid"`
	QueryText       string  `json:"query" db:"query"`
	ExecutionCount  int64   `json:"calls" db:"calls"`
	RowsProcessed   int64   `json:"rows" db:"rows"`
	TotalExecTime   float64 `json:"total_exec_time" db:"total_exec_time"`
	AverageExecTime float64 `json:"mean_exec_time" db:"mean_exec_time"`
	MinExecTime     float64 `json:"min_exec_time" db:"min_exec_time"`
	MaxExecTime     float64 `json:"max_exec_time" db:"max_exec_time"`
}
