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

// QueryStats provides database-agnostic access to query performance metrics
// This interface will abstract access to query statistics across different database systems (PostgreSQL, Oracle, MySQL)
type QueryStats interface {
	GetQueryID() int64
	GetQueryText() string

	GetExecutionCount() int64

	GetRowsProcessed() int64

	// Timing metrics (always in milliseconds for consistency)
	// few of these might not be relevant for all databases types
	GetTotalExecutionTime() float64
	GetAverageExecutionTime() float64
	GetMinExecutionTime() float64
	GetMaxExecutionTime() float64

	GetDatabaseType() string
}

func convertPgssToQueryStats(pgss []*pgss.PgStatStatements) []QueryStats {
	queryStats := make([]QueryStats, len(pgss))
	for i := 0; i < len(pgss); i++ {
		queryStats[i] = pgss[i]
	}
	return queryStats
}

// ================================ Comparison Report Types =================================

type ComparisonReport struct {
	GeneratedAt  time.Time
	SourceDBType string
	TargetDBType string
	Summary      ReportSummary

	// focussed views: top by impact and top by slowdown
	TopByImpact   []*QueryComparison
	TopBySlowdown []*QueryComparison

	// Every query in the source and target databases
	AllComparisons []*QueryComparison
}

type QueryComparison struct {
	Query         string
	SourceStats   QueryStats // nil if MatchStatus == TARGET_ONLY
	TargetStats   QueryStats // nil if MatchStatus == SOURCE_ONLY
	ImpactScore   float64    // 0 if not MATCHED
	SlowdownRatio float64    // 0 if not MATCHED
	MatchStatus   MatchStatus
}

type MatchStatus string

const (
	MATCHED     MatchStatus = "MATCHED"
	SOURCE_ONLY MatchStatus = "SOURCE_ONLY"
	TARGET_ONLY MatchStatus = "TARGET_ONLY"
)

type ReportSummary struct {
	TotalQueries      int
	MatchedQueries    int
	SourceOnlyQueries int
	TargetOnlyQueries int
}

func (c *QueryComparison) calculateMetrics() {
	if c.MatchStatus != MATCHED {
		return
	}

	// Impact score is the difference in total execution time between source and target
	// number of calls can be different, so we need to normalize by number of calls
	// Formula: yb_total_exec_time - pg_total_exec_time
	sourceNormalized := c.SourceStats.GetAverageExecutionTime() * float64(c.TargetStats.GetExecutionCount())
	targetTotal := c.TargetStats.GetTotalExecutionTime()
	c.ImpactScore = targetTotal - sourceNormalized

	// Slowdown ratio is the ratio of target average execution time to source average execution time
	// Formula: (yb_avg_exec_time + 2) / (pg_avg_exec_time + 2)
	c.SlowdownRatio = (c.TargetStats.GetAverageExecutionTime() + 2) / (c.SourceStats.GetAverageExecutionTime() + 2)
}
