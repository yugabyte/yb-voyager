//go:build unit

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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
)

const QUERY_CALLS_FOR_TEST = 100

// Most unit tests don't need targetDB mock since core logic only uses SourceQueryStats and TargetQueryStats

// createMockComparator creates a mock comparator with predictable test data
// Parameters:
//   - matchedQueries: number of queries that exist in both source and target
//   - sourceOnly: number of queries that exist only in source
//   - targetOnly: number of queries that exist only in target
//   - targetSlowdownFactor: multiplier applied to source average time to derive target average in mocks.
//   - 1.0 = same speed; >1.0 = slower (e.g., 1.5 => 50% slower); <1.0 = faster (e.g., 0.8 => 20% faster).
//   - Formula: targetAvgTime = sourceAvgTime * targetSlowdownFactor
//
// Generated data patterns:
//   - Matched queries: source avg = 10ms; target avg = 10ms * targetSlowdownFactor
//   - Source-only queries: avg = 15ms (arbitrary for test variety; not used in slowdown/impact)
//   - Target-only queries: avg = 8ms  (arbitrary for test variety; not used in slowdown/impact)
//   - All queries have 100 execution calls for simple math
//   - Query names: "Query-1", "Query-2", etc. for matched queries
//   - Source-only: "SourceQuery-1", "SourceQuery-2", etc.
//   - Target-only: "TargetQuery-1", "TargetQuery-2", etc.
func createMockComparator(matchedQueries, sourceOnly, targetOnly int, targetSlowdownFactor float64) *QueryPerformanceComparator {
	var sourceStats []*types.QueryStats
	var targetStats []*types.QueryStats

	// Generate matched queries (exist in both source and target)
	for i := 0; i < matchedQueries; i++ {
		queryText := fmt.Sprintf("Query-%d", i+1)
		queryID := int64(i + 1)
		sourceAvgTime := 10.0
		targetAvgTime := sourceAvgTime * targetSlowdownFactor

		sourceStats = append(sourceStats, &types.QueryStats{
			QueryID:         queryID,
			QueryText:       queryText,
			ExecutionCount:  QUERY_CALLS_FOR_TEST,
			RowsProcessed:   QUERY_CALLS_FOR_TEST,
			TotalExecTime:   sourceAvgTime * float64(QUERY_CALLS_FOR_TEST),
			AverageExecTime: sourceAvgTime,
		})

		targetStats = append(targetStats, &types.QueryStats{
			QueryID:         queryID,
			QueryText:       queryText,
			ExecutionCount:  QUERY_CALLS_FOR_TEST,
			RowsProcessed:   QUERY_CALLS_FOR_TEST,
			TotalExecTime:   targetAvgTime * float64(QUERY_CALLS_FOR_TEST),
			AverageExecTime: targetAvgTime,
		})
	}

	// Generate source-only queries
	for i := 0; i < sourceOnly; i++ {
		queryText := fmt.Sprintf("SourceQuery-%d", i+1)
		queryID := int64(1000 + i + 1) // Use different ID range
		avgTime := 15.0                // Different from matched queries

		sourceStats = append(sourceStats, &types.QueryStats{
			QueryID:         queryID,
			QueryText:       queryText,
			ExecutionCount:  QUERY_CALLS_FOR_TEST,
			RowsProcessed:   QUERY_CALLS_FOR_TEST,
			TotalExecTime:   avgTime * float64(QUERY_CALLS_FOR_TEST),
			AverageExecTime: avgTime,
		})
	}

	// Generate target-only queries
	for i := 0; i < targetOnly; i++ {
		queryText := fmt.Sprintf("TargetQuery-%d", i+1)
		queryID := int64(2000 + i + 1) // Use different ID range
		avgTime := 8.0                 // Different from matched queries

		targetStats = append(targetStats, &types.QueryStats{
			QueryID:         queryID,
			QueryText:       queryText,
			ExecutionCount:  QUERY_CALLS_FOR_TEST,
			RowsProcessed:   QUERY_CALLS_FOR_TEST,
			TotalExecTime:   avgTime * float64(QUERY_CALLS_FOR_TEST),
			AverageExecTime: avgTime,
		})
	}

	return &QueryPerformanceComparator{
		SourceQueryStats: sourceStats,
		TargetQueryStats: targetStats,
		msr:              nil, // not required as of now for tests
		targetDB:         nil, // not required as of now for tests
	}
}

// TestGetTopBySlowdown tests the getTopBySlowdown method with predictable mock data
func TestGetTopBySlowdown(t *testing.T) {
	// Create comparator with 30 matched queries, target 50% slower
	comparator := createMockComparator(30, 10, 10, 1.5) // 1.5 = 50% slower
	allComparisons := comparator.matchQueries()
	for _, comp := range allComparisons {
		comp.calculateMetrics()
	}

	// All matched queries should have same slowdown ratio since same slowdown factor (source avg time is 10ms)
	expectedSlowdownRatio := (15.0 + SLOWDOWN_RATIO_OFFSET) / (10.0 + SLOWDOWN_RATIO_OFFSET)

	topN := 10
	topBySlowdown := comparator.getTopBySlowdown(allComparisons, topN)
	assert.Len(t, topBySlowdown, topN, "Should return only the 10 matched queries")
	for i, comp := range topBySlowdown {
		assert.Equal(t, MATCHED, comp.MatchStatus, "Query %d should be MATCHED", i)
		assert.InDelta(t, expectedSlowdownRatio, comp.SlowdownRatio, 0.001)
		assert.Contains(t, comp.Query, "Query-", "Should be a matched query")
	}

	// TODO: add tests with different slowdown ratios for matched queries to test sorting
}

// TestGetTopBySlowdown_TargetFaster tests when target is faster (slowdown < 1)
func TestGetTopBySlowdown_TargetFaster(t *testing.T) {
	// Create comparator where target is 20% faster, 0.8 = 20% faster
	comparator := createMockComparator(2, 0, 0, 0.8)
	expectedSlowdownRatio := (8.0 + SLOWDOWN_RATIO_OFFSET) / (10.0 + SLOWDOWN_RATIO_OFFSET)
	allComparisons := comparator.matchQueries()
	for _, comp := range allComparisons {
		comp.calculateMetrics()
	}

	topN := 10
	topBySlowdown := comparator.getTopBySlowdown(allComparisons, topN)

	// assertions
	assert.Len(t, topBySlowdown, 2, "Should return both matched queries")
	for _, comp := range topBySlowdown {
		assert.Equal(t, MATCHED, comp.MatchStatus, "Should be MATCHED")
		assert.InDelta(t, expectedSlowdownRatio, comp.SlowdownRatio, 0.001)
		assert.True(t, comp.SlowdownRatio < 1.0, "Slowdown should be < 1 when target is faster")
	}
}

// TestGetTopByImpact tests the getTopByImpact method with predictable mock data
func TestGetTopByImpact(t *testing.T) {
	// Create comparator with 30 matched queries, target 50% slower
	comparator := createMockComparator(30, 10, 10, 1.5)
	expectedImpactScore := 15.0*QUERY_CALLS_FOR_TEST - 10.0*QUERY_CALLS_FOR_TEST
	allComparisons := comparator.matchQueries()
	for _, comp := range allComparisons {
		comp.calculateMetrics()
	}

	topN := 10
	topByImpact := comparator.getTopByImpact(allComparisons, topN)

	// assertions
	assert.Len(t, topByImpact, topN, "Should return only the %d matched queries", topN)
	for i, comp := range topByImpact {
		assert.Equal(t, MATCHED, comp.MatchStatus, "Query %d should be MATCHED", i)
		assert.Equal(t, expectedImpactScore, comp.ImpactScore)
		assert.Contains(t, comp.Query, "Query-", "Should be a matched query")
	}
	// TODO: add tests with different impact scores for matched queries to test sorting
}

// TestGetTopByImpact_TargetFaster tests when target is faster (impact < 0)
func TestGetTopByImpact_TargetFaster(t *testing.T) {
	// Create comparator where target is 20% faster, 0.8 = 20% faster
	comparator := createMockComparator(2, 0, 0, 0.8)
	expectedImpactScore := 8.0*QUERY_CALLS_FOR_TEST - 10.0*QUERY_CALLS_FOR_TEST
	allComparisons := comparator.matchQueries()
	for _, comp := range allComparisons {
		comp.calculateMetrics()
	}

	topN := 10
	topByImpact := comparator.getTopByImpact(allComparisons, topN)

	// assertions
	assert.Len(t, topByImpact, 2, "Should return both matched queries")
	for _, comp := range topByImpact {
		assert.Equal(t, MATCHED, comp.MatchStatus, "Should be MATCHED")
		assert.Equal(t, expectedImpactScore, comp.ImpactScore)
		assert.True(t, comp.ImpactScore < 0.0, "Impact should be < 0 when target is faster")
	}
}

func TestNoQueriesMatched(t *testing.T) {
	// Create comparator where there are no matched queries
	comparator := createMockComparator(0, 10, 10, 1.5)
	allComparisons := comparator.matchQueries()
	for _, comp := range allComparisons {
		comp.calculateMetrics()
	}

	topN := 10
	topByImpact := comparator.getTopByImpact(allComparisons, topN)
	topBySlowdown := comparator.getTopBySlowdown(allComparisons, topN)

	// assertions
	assert.Len(t, topByImpact, 0, "Should return no matched queries")
	assert.Len(t, topBySlowdown, 0, "Should return no matched queries")
}
