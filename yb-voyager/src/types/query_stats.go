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

import (
	"math"

	log "github.com/sirupsen/logrus"
)

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

// MergeQueryStatsBasedOnQuery merges query stats with the same query text.
// Used to aggregate stats from multiple source nodes (primary + replicas).
// Mirrors the logic of pgss.MergePgStatStatementsBasedOnQuery but operates on QueryStats directly
// to avoid conversion overhead between types and maintain a clean, direct code path.
//
// Merge key: Uses query text (not queryid) because:
// - QueryIDs can differ for the same query across nodes due to user context, DB OIDs, or PG version differences
// - Query text is the semantic truth - same text = same query, regardless of queryid
func MergeQueryStatsBasedOnQuery(entries []*QueryStats) []*QueryStats {
	queryMap := make(map[string]*QueryStats)

	for _, entry := range entries {
		if existing, ok := queryMap[entry.QueryText]; !ok {
			// First occurrence of this query text - add to map
			queryMap[entry.QueryText] = entry
		} else {
			// Duplicate query text from another node - merge stats
			existing.mergeWith(entry)
		}
	}

	// Convert map to slice
	mergedEntries := make([]*QueryStats, 0, len(queryMap))
	for _, entry := range queryMap {
		mergedEntries = append(mergedEntries, entry)
	}

	return mergedEntries
}

// mergeWith merges another QueryStats entry into this one.
// Additive: ExecutionCount, RowsProcessed, TotalExecTime (summed)
// Recalculated: AverageExecTime (from merged totals)
// Preserved: QueryID, QueryText (from first)
// Min/Max: MinExecTime, MaxExecTime (preserved across merges)
func (qs *QueryStats) mergeWith(other *QueryStats) {
	first := *qs // Save original values

	// Keep original QueryID and QueryText
	qs.QueryID = first.QueryID
	qs.QueryText = first.QueryText

	// Sum: calls, rows, total time
	qs.ExecutionCount = first.ExecutionCount + other.ExecutionCount
	qs.RowsProcessed = first.RowsProcessed + other.RowsProcessed
	qs.TotalExecTime = first.TotalExecTime + other.TotalExecTime

	// Recalculate: average execution time (mean = combined_total_exec_time / combined_calls)
	if qs.ExecutionCount == 0 {
		// ideally this is not expected but since we have observed this in YB, we should handle it
		log.Warnf("calls is 0 for query stats entry: %+v", qs)
		qs.AverageExecTime = 0
	} else {
		qs.AverageExecTime = qs.TotalExecTime / float64(qs.ExecutionCount)
	}

	// Preserve: min and max
	qs.MinExecTime = math.Min(first.MinExecTime, other.MinExecTime)
	qs.MaxExecTime = math.Max(first.MaxExecTime, other.MaxExecTime)
}
