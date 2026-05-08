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

package queryissue

import "strings"

// CoveringIndexRecommendation is the final, per-index output carried from assessment
// to the issue-emission path. Produced after parameter-substitution EXPLAIN walks
// and the update/read ratio filter; every surviving missing column becomes an
// INCLUDE candidate.
type CoveringIndexRecommendation struct {
	// IncludeColumns are the final columns to add in INCLUDE(...) after caps.
	IncludeColumns []string

	// RelevantQueries are top-K high-impact queries that benefit from this index becoming covering.
	RelevantQueries []RelevantQuery

	// TotalCalls is the sum of calls across all relevant queries that triggered this recommendation.
	TotalCalls int64

	// TotalExecTimeMs is the sum of total_exec_time (ms) across relevant queries.
	TotalExecTimeMs float64

	// DroppedColumns lists candidates that were considered but excluded, with a reason.
	// Currently the only reason emitted is "write_heavy" (from the update/read ratio filter).
	DroppedColumns []DroppedColumn
}

// RelevantQuery is a summary of a pg_stat_statements entry that benefits from a recommendation.
type RelevantQuery struct {
	Text        string
	Calls       int64
	TotalExecMs float64
}

// DroppedColumn records a candidate column that was considered but excluded and why.
type DroppedColumn struct {
	Name   string
	Reason string
}

// CoveringIndexRecommendationKey is the canonical lookup key for per-index
// recommendations: lowercased, dot-joined schema and index name.
func CoveringIndexRecommendationKey(schema, indexName string) string {
	schema = strings.TrimSpace(schema)
	indexName = strings.TrimSpace(indexName)
	return strings.ToLower(schema + "." + indexName)
}
