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

import (
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/pgss"
)

// QueryStats represents database-agnostic query performance metrics
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

// NewQueryStatsFromPgss converts PgStatStatements to QueryStats struct
func NewQueryStatsFromPgss(pgss *pgss.PgStatStatements) *QueryStats {
	return &QueryStats{
		QueryID:         pgss.QueryID,
		QueryText:       pgss.Query,
		ExecutionCount:  pgss.Calls,
		RowsProcessed:   pgss.Rows,
		TotalExecTime:   pgss.TotalExecTime,
		AverageExecTime: pgss.MeanExecTime,
		MinExecTime:     pgss.MinExecTime,
		MaxExecTime:     pgss.MaxExecTime,
	}
}

// ConvertPgssSliceToQueryStats converts slice of PgStatStatements to slice of QueryStats
func ConvertPgssSliceToQueryStats(pgssSlice []*pgss.PgStatStatements) []*QueryStats {
	queryStats := make([]*QueryStats, len(pgssSlice))
	for i, pgss := range pgssSlice {
		queryStats[i] = NewQueryStatsFromPgss(pgss)
	}
	return queryStats
}

// ================================ Comparison Report Types =================================

type ComparisonReport struct {
	GeneratedAt  time.Time
	SourceDBType string
	Summary      ReportSummary

	// focussed views: top by impact and top by slowdown
	TopByImpact   []*QueryComparison
	TopBySlowdown []*QueryComparison

	// Every query in the source and target databases
	AllComparisons []*QueryComparison
}

type QueryComparison struct {
	Query         string
	SourceStats   *QueryStats // nil if MatchStatus == TARGET_ONLY
	TargetStats   *QueryStats // nil if MatchStatus == SOURCE_ONLY
	ImpactScore   float64     // 0 if not MATCHED
	SlowdownRatio float64     // 0 if not MATCHED
	MatchStatus   MatchStatus
}

type MatchStatus string

const (
	MATCHED     MatchStatus = "MATCHED"
	SOURCE_ONLY MatchStatus = "SOURCE_ONLY"
	TARGET_ONLY MatchStatus = "TARGET_ONLY"
)

type ReportSummary struct {
	VoyagerVersion    string
	SourceDBVersion   string
	TargetDBVersion   string
	SourceDBName      string
	TargetDBName      string
	TotalQueries      int
	MatchedQueries    int
	SourceOnlyQueries int
	TargetOnlyQueries int
}

// ================================ Comparison Report Types Methods ================================

func (c *QueryComparison) calculateMetrics() {
	if c.MatchStatus != MATCHED || c.SourceStats == nil || c.TargetStats == nil {
		return
	}

	// Impact score is the difference in total execution time between source and target
	// number of calls can be different, so we need to normalize by number of calls
	// Formula: yb_total_exec_time - pg_total_exec_time
	sourceNormalized := c.SourceStats.AverageExecTime * float64(c.TargetStats.ExecutionCount)
	targetTotal := c.TargetStats.TotalExecTime
	c.ImpactScore = targetTotal - sourceNormalized

	// Slowdown ratio is the ratio of target average execution time to source average execution time
	// Formula: (yb_avg_exec_time + 2) / (pg_avg_exec_time + 2)
	c.SlowdownRatio = (c.TargetStats.AverageExecTime + 2) / (c.SourceStats.AverageExecTime + 2)
}
