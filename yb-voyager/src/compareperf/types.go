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
package compareperf

import "github.com/yugabyte/yb-voyager/yb-voyager/src/pgss"

// QueryStats provides database-agnostic access to query performance metrics
// This interface will abstract access to query statistics across different database systems (PostgreSQL, Oracle, MySQL)
type QueryStats interface {
	GetQueryID() int64    // Unique identifier for the normalized query
	GetQueryText() string // The query text

	GetExecutionCount() int64 // Number of times query was executed

	GetRowsProcessed() int64 // Total rows retrieved or affected

	// Timing metrics (always in milliseconds for consistency)
	GetTotalExecutionTime() float64   // Total cumulative execution time
	GetAverageExecutionTime() float64 // Average execution time per query

	GetDatabaseType() string
}

func convertPgssToQueryStats(pgss []*pgss.PgStatStatements) []QueryStats {
	queryStats := make([]QueryStats, len(pgss))
	for i := 0; i < len(pgss); i++ {
		queryStats[i] = pgss[i]
	}
	return queryStats
}
