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
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const TOP_N = 20

//go:embed templates/performance_comparison_report.template
var performanceComparisonHtmlTemplate string

type QueryPerformanceComparator struct {
	SourceQueryStats []QueryStats
	TargetQueryStats []QueryStats

	Report *ComparisonReport

	// access to MSR, AssessmentDB, TargetDB
	msr          *metadb.MigrationStatusRecord
	assessmentDB *migassessment.AssessmentDB
	targetDB     *tgtdb.TargetYugabyteDB
}

func NewQueryPerformanceComparator(msr *metadb.MigrationStatusRecord, assessmentDB *migassessment.AssessmentDB, targetDB *tgtdb.TargetYugabyteDB) (*QueryPerformanceComparator, error) {
	sourceQueryStats, err := assessmentDB.GetSourceQueryStats()
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

		msr:          msr,
		assessmentDB: assessmentDB,
		targetDB:     targetDB,
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
		Summary: ReportSummary{
			VoyagerVersion:    utils.YB_VOYAGER_VERSION,
			SourceDBVersion:   c.msr.SourceDBConf.DBVersion,
			TargetDBVersion:   c.targetDB.GetVersion(),
			SourceDBName:      c.msr.SourceDBConf.DBName,
			TargetDBName:      c.targetDB.Tconf.DBName,
			TotalQueries:      len(allComparisons),
			MatchedQueries:    lo.CountBy(allComparisons, func(c *QueryComparison) bool { return c.MatchStatus == MATCHED }),
			SourceOnlyQueries: lo.CountBy(allComparisons, func(c *QueryComparison) bool { return c.MatchStatus == SOURCE_ONLY }),
			TargetOnlyQueries: lo.CountBy(allComparisons, func(c *QueryComparison) bool { return c.MatchStatus == TARGET_ONLY }),
		},

		AllComparisons: allComparisons,
		TopByImpact:    topByImpactView,
		TopBySlowdown:  topBySlowdownView,
	}

	// 5. console summary
	fmt.Println(c.consoleSummary())

	return nil
}

func (c *QueryPerformanceComparator) GenerateReport(exportDir string) error {
	if c.Report == nil {
		return fmt.Errorf("no comparison report available, Compare() must be executed first")
	}

	// 1. Generate HTML report
	err := c.generateHTMLReport(exportDir)
	if err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	// 2. Generate JSON report
	err = c.generateJSONReport(exportDir)
	if err != nil {
		return fmt.Errorf("failed to generate JSON report: %w", err)
	}

	return nil
}

func (c *QueryPerformanceComparator) consoleSummary() string {
	if c.Report == nil {
		return ""
	}

	s := c.Report.Summary
	consoleSummary := fmt.Sprintf("total queries: %d (matched: %d, source-only: %d, target-only: %d)",
		s.TotalQueries, s.MatchedQueries, s.SourceOnlyQueries, s.TargetOnlyQueries)
	log.Info(consoleSummary)
	return consoleSummary
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

func (c *QueryPerformanceComparator) generateHTMLReport(exportDir string) error {
	// Create reports directory
	reportsDir := filepath.Join(exportDir, "reports")
	err := os.MkdirAll(reportsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Parse template
	tmpl, err := template.New("performance-comparison-report").Parse(performanceComparisonHtmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	// Create HTML file
	htmlPath := filepath.Join(reportsDir, "performance-comparison-report.html")
	file, err := os.Create(htmlPath)
	if err != nil {
		return fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer file.Close()

	// Sort AllComparisons for HTML display: MATCHED -> TARGET_ONLY -> SOURCE_ONLY, then by call frequency
	sortedReport := *c.Report // Create a copy
	sortedComparisons := make([]*QueryComparison, len(c.Report.AllComparisons))
	copy(sortedComparisons, c.Report.AllComparisons) // deep copy to avoid modifying the original report

	sort.Slice(sortedComparisons, func(i, j int) bool {
		// Define priority order for match status
		statusPriority := map[MatchStatus]int{
			MATCHED:     1,
			TARGET_ONLY: 2,
			SOURCE_ONLY: 3,
		}

		iPriority := statusPriority[sortedComparisons[i].MatchStatus]
		jPriority := statusPriority[sortedComparisons[j].MatchStatus]

		// If different match status, sort by status priority
		if iPriority != jPriority {
			return iPriority < jPriority
		}

		// If same match status, sort by call frequency (descending)
		iCalls := getCallCount(sortedComparisons[i])
		jCalls := getCallCount(sortedComparisons[j])
		return iCalls > jCalls
	})
	sortedReport.AllComparisons = sortedComparisons

	// Execute template
	err = tmpl.Execute(file, &sortedReport)
	if err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	utils.PrintAndLog("HTML report generated at: %s", htmlPath)
	return nil
}

func (c *QueryPerformanceComparator) generateJSONReport(exportDir string) error {
	// Create reports directory
	reportsDir := filepath.Join(exportDir, "reports")
	err := os.MkdirAll(reportsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(c.Report, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON report: %w", err)
	}

	// Write JSON file
	jsonPath := filepath.Join(reportsDir, "performance-comparison-report.json")
	err = os.WriteFile(jsonPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	utils.PrintAndLog("JSON report generated at: %s", jsonPath)
	return nil
}

// getCallCount returns the call count for sorting purposes
// For MATCHED queries, use source calls (primary data source)
// For SOURCE_ONLY queries, use source calls
// For TARGET_ONLY queries, use target calls
func getCallCount(comparison *QueryComparison) int64 {
	switch comparison.MatchStatus {
	case MATCHED, SOURCE_ONLY:
		if comparison.SourceStats != nil {
			return comparison.SourceStats.GetExecutionCount()
		}
		return 0
	case TARGET_ONLY:
		if comparison.TargetStats != nil {
			return comparison.TargetStats.GetExecutionCount()
		}
		return 0
	default:
		return 0
	}
}
