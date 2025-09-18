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
	"sort"
	"time"

	"github.com/samber/lo"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

const TOP_N = 20

type QueryPerformanceComparator struct {
	SourceQueryStats []QueryStats
	TargetQueryStats []QueryStats

	Report *ComparisonReport
}

func NewQueryPerformanceComparator(assessmentDirPath string, targetDB *tgtdb.TargetYugabyteDB) (*QueryPerformanceComparator, error) {
	migassessment.AssessmentDir = assessmentDirPath
	adb, err := migassessment.NewAssessmentDB()
	if err != nil {
		return nil, fmt.Errorf("failed to open assessment database: %w", err)
	}

	sourceQueryStats, err := adb.GetSourceQueryStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get source query stats: %w", err)
	}

	targetQueryStats, err := targetDB.CollectPgStatStatements()
	if err != nil {
		return nil, fmt.Errorf("failed to get target query stats: %w", err)
	}

	return &QueryPerformanceComparator{
		SourceQueryStats: convertPgssToQueryStats(sourceQueryStats),
		TargetQueryStats: convertPgssToQueryStats(targetQueryStats),
	}, nil
}

func (c *QueryPerformanceComparator) Compare() error {
	// 1. Match queries between source and target
	allComparisons := c.matchQueries()

	// 2. Calculate metrics for each comparison
	for _, comparison := range allComparisons {
		comparison.calculateMetrics()
	}

	// 3. Create views
	topByImpactView := c.getTopByImpact(allComparisons, TOP_N)
	topBySlowdownView := c.getTopBySlowdown(allComparisons, TOP_N)

	// 4. final report
	c.Report = &ComparisonReport{
		GeneratedAt:  time.Now(),
		SourceDBType: c.SourceQueryStats[0].GetDatabaseType(), // Corner case: what if its nil or len 0?
		TargetDBType: c.TargetQueryStats[0].GetDatabaseType(),
		Summary: ReportSummary{
			TotalQueries:      len(allComparisons),
			MatchedQueries:    lo.CountBy(allComparisons, func(c *QueryComparison) bool { return c.MatchStatus == MATCHED }),
			SourceOnlyQueries: lo.CountBy(allComparisons, func(c *QueryComparison) bool { return c.MatchStatus == SOURCE_ONLY }),
			TargetOnlyQueries: lo.CountBy(allComparisons, func(c *QueryComparison) bool { return c.MatchStatus == TARGET_ONLY }),
		},

		AllComparisons: allComparisons,
		TopByImpact:    topByImpactView,
		TopBySlowdown:  topBySlowdownView,
	}
	return nil
}

func (c *QueryPerformanceComparator) GenerateReport() error {
	var err error

	// 1. Generate HTML report
	err = c.generateHTMLReport()
	if err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	// 2. Generate JSON report
	err = c.generateJSONReport()
	if err != nil {
		return fmt.Errorf("failed to generate JSON report: %w", err)
	}

	return nil
}

func (c *QueryPerformanceComparator) ConsoleSummary() string {
	return ""
}

func (c *QueryPerformanceComparator) matchQueries() []*QueryComparison {
	var allComparisons []*QueryComparison

	for _, sourceQueryStat := range c.SourceQueryStats {
		matchFound := false
		for _, targetQueryStat := range c.TargetQueryStats {
			if sourceQueryStat.GetQueryText() == targetQueryStat.GetQueryText() {
				allComparisons = append(allComparisons, &QueryComparison{
					Query:       sourceQueryStat.GetQueryText(),
					SourceStats: sourceQueryStat,
					TargetStats: targetQueryStat,
					MatchStatus: MATCHED,
				})
				matchFound = true
				break
			}
		}

		if !matchFound {
			allComparisons = append(allComparisons, &QueryComparison{
				Query:       sourceQueryStat.GetQueryText(),
				SourceStats: sourceQueryStat,
				TargetStats: nil,
				MatchStatus: SOURCE_ONLY,
			})
		}
	}

	for _, targetQueryStat := range c.TargetQueryStats {
		matchFound := false
		for _, sourceQueryStat := range c.SourceQueryStats {
			if targetQueryStat.GetQueryText() == sourceQueryStat.GetQueryText() {
				matchFound = true
				break
			}
		}

		if !matchFound {
			allComparisons = append(allComparisons, &QueryComparison{
				Query:       targetQueryStat.GetQueryText(),
				TargetStats: targetQueryStat,
				SourceStats: nil,
				MatchStatus: TARGET_ONLY,
			})
		}
	}

	return allComparisons
}

func (c *QueryPerformanceComparator) getTopByImpact(allComparisons []*QueryComparison, limit int) []*QueryComparison {
	matched := lo.Filter(allComparisons, func(comp *QueryComparison, _ int) bool {
		return comp.MatchStatus == MATCHED
	})

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ImpactScore > matched[j].ImpactScore
	})

	return matched[:min(limit, len(matched))]
}

func (c *QueryPerformanceComparator) getTopBySlowdown(allComparisons []*QueryComparison, limit int) []*QueryComparison {
	matched := lo.Filter(allComparisons, func(comp *QueryComparison, _ int) bool {
		return comp.MatchStatus == MATCHED
	})

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].SlowdownRatio > matched[j].SlowdownRatio
	})

	return matched[:min(limit, len(matched))]
}

func (c *QueryPerformanceComparator) generateHTMLReport() error {
	return nil
}

func (c *QueryPerformanceComparator) generateJSONReport() error {
	return nil
}
