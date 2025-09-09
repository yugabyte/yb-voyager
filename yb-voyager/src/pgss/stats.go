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

// QueryStats represents a pg_stat_statements entry
// All field names follow PostgreSQL 13+ conventions for consistency across versions
// This struct serves as the single source of truth for PGSS data throughout the system
type QueryStats struct {
	// Core identification fields
	QueryID int64  `json:"queryid" db:"queryid"` // Unique identifier for the normalized query
	Query   string `json:"query" db:"query"`     // The query text

	// Execution metrics
	Calls int64 `json:"calls" db:"calls"` // Number of times executed
	Rows  int64 `json:"rows" db:"rows"`   // Total rows retrieved or affected

	// Timing metrics (all in milliseconds, normalized to PG 13+ column names)
	TotalExecTime  float64 `json:"total_exec_time" db:"total_exec_time"`   // total_time (PG11-12) -> total_exec_time (PG13+)
	MeanExecTime   float64 `json:"mean_exec_time" db:"mean_exec_time"`     // mean_time (PG11-12) -> mean_exec_time (PG13+)
	MinExecTime    float64 `json:"min_exec_time" db:"min_exec_time"`       // min_time (PG11-12) -> min_exec_time (PG13+)
	MaxExecTime    float64 `json:"max_exec_time" db:"max_exec_time"`       // max_time (PG11-12) -> max_exec_time (PG13+)
	StddevExecTime float64 `json:"stddev_exec_time" db:"stddev_exec_time"` // stddev_time (PG11-12) -> stddev_exec_time (PG13+)
}

// GetSlowdownRatio calculates how much slower this query is compared to a baseline
func (q *QueryStats) GetSlowdownRatio(baseline *QueryStats) float64 {
	if baseline == nil || baseline.MeanExecTime <= 0 {
		return 0.0
	}
	return q.MeanExecTime / baseline.MeanExecTime
}
