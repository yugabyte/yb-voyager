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
	"math"
)

// PgStatStatements represents a pg_stat_statements entry
// All field names follow PostgreSQL 13+ conventions for consistency across versions
type PgStatStatements struct {
	// Core identification fields
	QueryID int64  `json:"queryid"` // Unique identifier for the normalized query
	Query   string `json:"query"`   // The query text

	// Execution metrics
	Calls int64 `json:"calls"` // Number of times executed
	Rows  int64 `json:"rows"`  // Total rows retrieved or affected

	// Note: psql script takes care of normalizing timing metrics to same naming convention:  *_exec_time
	TotalExecTime  float64 `json:"total_exec_time"`
	MeanExecTime   float64 `json:"mean_exec_time"`
	MinExecTime    float64 `json:"min_exec_time"`
	MaxExecTime    float64 `json:"max_exec_time"`
	StddevExecTime float64 `json:"stddev_exec_time"`
}

// Merge merges the stats for the entries with the same query
func (p *PgStatStatements) Merge(second *PgStatStatements) {
	first := *p // shallow copy is ok as no pointers in the struct

	// queryid and query are expected to be same for the entries to be merged
	p.QueryID = first.QueryID
	p.Query = first.Query
	p.Calls = first.Calls + second.Calls
	p.Rows = first.Rows + second.Rows
	p.TotalExecTime = first.TotalExecTime + second.TotalExecTime

	// Recalculate mean_exec_time after merging: mean = combined_total_exec_time / combined_calls
	p.MeanExecTime = p.TotalExecTime / float64(p.Calls)

	p.MinExecTime = math.Min(first.MinExecTime, second.MinExecTime)
	p.MaxExecTime = math.Max(first.MaxExecTime, second.MaxExecTime)

	// TODO: disabling stddev_exec_time until the ask/need comes up
	// p.StddevExecTime = mergeStddevPopulation(
	// 	first.Calls, first.TotalExecTime, first.StddevExecTime,
	// 	second.Calls, second.TotalExecTime, second.StddevExecTime,
	// )
}

// By ChatGPT:
// mergeStddevPopulation merges two groups' execution-time stddevs as POPULATION stddev.
// Inputs are per-group: n, total_time, stddev. Output is the combined stddev.
// TODO: revisit this in future if usage of stddev_exec_time is increased
// func mergeStddevPopulation(n1 int64, total1, std1 float64, n2 int64, total2, std2 float64) float64 {
// 	if n1 <= 0 {
// 		return std2
// 	}
// 	if n2 <= 0 {
// 		return std1
// 	}
// 	N1, N2 := float64(n1), float64(n2)
// 	n := N1 + N2

// 	mu := (total1 + total2) / n // combined mean
// 	mu1 := total1 / N1
// 	mu2 := total2 / N2

// 	// M2 = sum of squared deviations (population)
// 	M2 := N1*std1*std1 +
// 		N2*std2*std2 +
// 		N1*(mu1-mu)*(mu1-mu) +
// 		N2*(mu2-mu)*(mu2-mu)

// 	return math.Sqrt(M2 / n) // use / (n-1) for sample stddev, if ever needed
// }

// ================================ Merge PgStatStatements =================================

// MergePgStatStatements merges the stats for the entries with the same query
func MergePgStatStatements(entries []*PgStatStatements) []*PgStatStatements {
	queryMap := make(map[string]*PgStatStatements)

	for _, entry := range entries {
		if _, ok := queryMap[entry.Query]; !ok {
			queryMap[entry.Query] = entry
		} else {
			queryMap[entry.Query].Merge(entry)
		}
	}

	mergedEntries := make([]*PgStatStatements, 0, len(queryMap))
	for _, entry := range queryMap {
		mergedEntries = append(mergedEntries, entry)
	}
	return mergedEntries
}
