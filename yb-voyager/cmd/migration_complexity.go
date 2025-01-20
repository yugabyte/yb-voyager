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
package cmd

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
)

const NOT_AVAILABLE = "NOT AVAILABLE"

var (
	LEVEL_1_MEDIUM_THRESHOLD     = 20
	LEVEL_1_HIGH_THRESHOLD       = math.MaxInt32
	LEVEL_2_MEDIUM_THRESHOLD     = 10
	LEVEL_2_HIGH_THRESHOLD       = 100
	LEVEL_3_MEDIUM_THRESHOLD     = 0
	LEVEL_3_HIGH_THRESHOLD       = 4
	migrationComplexityRationale string
)

// Migration complexity calculation based on the detected assessment issues
func calculateMigrationComplexity(sourceDBType string, schemaDirectory string, assessmentReport AssessmentReport) string {
	if sourceDBType != ORACLE && sourceDBType != POSTGRESQL {
		return NOT_AVAILABLE
	}

	log.Infof("calculating migration complexity for %s...", sourceDBType)
	switch sourceDBType {
	case ORACLE:
		migrationComplexity, err := calculateMigrationComplexityForOracle(schemaDirectory)
		if err != nil {
			log.Errorf("failed to get migration complexity for oracle: %v", err)
			return NOT_AVAILABLE
		}
		return migrationComplexity
	case POSTGRESQL:
		return calculateMigrationComplexityForPG(assessmentReport)
	default:
		panic(fmt.Sprintf("unsupported source db type '%s' for migration complexity", sourceDBType))
	}
}

func calculateMigrationComplexityForPG(assessmentReport AssessmentReport) string {
	if assessmentReport.MigrationComplexity != "" {
		return assessmentReport.MigrationComplexity
	}

	counts := lo.CountValuesBy(assessmentReport.Issues, func(issue AssessmentIssue) string {
		return issue.Impact
	})
	l1IssueCount := counts[constants.IMPACT_LEVEL_1]
	l2IssueCount := counts[constants.IMPACT_LEVEL_2]
	l3IssueCount := counts[constants.IMPACT_LEVEL_3]

	log.Infof("issue counts: level-1=%d, level-2=%d, level-3=%d\n", l1IssueCount, l2IssueCount, l3IssueCount)

	// Determine complexity for each level
	comp1 := getComplexityForLevel(constants.IMPACT_LEVEL_1, l1IssueCount)
	comp2 := getComplexityForLevel(constants.IMPACT_LEVEL_2, l2IssueCount)
	comp3 := getComplexityForLevel(constants.IMPACT_LEVEL_3, l3IssueCount)
	complexities := []string{comp1, comp2, comp3}
	log.Infof("complexities according to each level: %v", complexities)

	finalComplexity := constants.MIGRATION_COMPLEXITY_LOW
	// If ANY level is HIGH => final is HIGH
	if slices.Contains(complexities, constants.MIGRATION_COMPLEXITY_HIGH) {
		finalComplexity = constants.MIGRATION_COMPLEXITY_HIGH
	} else if slices.Contains(complexities, constants.MIGRATION_COMPLEXITY_MEDIUM) {
		// Else if ANY level is MEDIUM => final is MEDIUM
		finalComplexity = constants.MIGRATION_COMPLEXITY_MEDIUM
	}

	migrationComplexityRationale = buildRationale(finalComplexity, l1IssueCount, l2IssueCount, l3IssueCount)
	return finalComplexity
}

// This is a temporary logic to get migration complexity for oracle based on the migration level from ora2pg report.
// Ideally, we should ALSO be considering the schema analysis report to get the migration complexity.
func calculateMigrationComplexityForOracle(schemaDirectory string) (string, error) {
	ora2pgReportPath := filepath.Join(schemaDirectory, "ora2pg_report.csv")
	if !utils.FileOrFolderExists(ora2pgReportPath) {
		return "", fmt.Errorf("ora2pg report file not found at %s", ora2pgReportPath)
	}
	file, err := os.Open(ora2pgReportPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", ora2pgReportPath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("Error while closing file %s: %v", ora2pgReportPath, err)
		}
	}()
	// Sample file contents

	// "dbi:Oracle:(DESCRIPTION = (ADDRESS = (PROTOCOL = TCP)(HOST = xyz)(PORT = 1521))(CONNECT_DATA = (SERVICE_NAME = DMS)))";
	// "Oracle Database 19c Enterprise Edition Release 19.0.0.0.0";"ASSESS_MIGRATION";"261.62 MB";"1 person-day(s)";"A-2";
	// "0/0/0.00";"0/0/0";"0/0/0";"25/0/6.50";"0/0/0.00";"0/0/0";"0/0/0";"0/0/0";"0/0/0";"3/0/1.00";"3/0/1.00";
	// "44/0/4.90";"27/0/2.70";"9/0/1.80";"4/0/16.00";"5/0/3.00";"2/0/2.00";"125/0/58.90"
	//
	// X/Y/Z - total/invalid/cost for each type of objects(table,function,etc). Last data element is the sum total.
	// total cost = 58.90 units (1 unit = 5 minutes). Therefore total cost is approx 1 person-days.
	// column 6 is Migration level.
	//  Migration levels:
	//     A - Migration that might be run automatically
	//     B - Migration with code rewrite and a human-days cost up to 5 days
	//     C - Migration with code rewrite and a human-days cost above 5 days
	// 	Technical levels:
	//     1 = trivial: no stored functions and no triggers
	//     2 = easy: no stored functions but with triggers, no manual rewriting
	//     3 = simple: stored functions and/or triggers, no manual rewriting
	//     4 = manual: no stored functions but with triggers or views with code rewriting
	//     5 = difficult: stored functions and/or triggers with code rewriting
	reader := csv.NewReader(file)
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		log.Errorf("error reading csv file %s: %v", ora2pgReportPath, err)
		return "", fmt.Errorf("error reading csv file %s: %w", ora2pgReportPath, err)
	}
	if len(rows) > 1 {
		return "", fmt.Errorf("invalid ora2pg report file format. Expected 1 row, found %d. contents = %v", len(rows), rows)
	}
	reportData := rows[0]
	migrationLevel := strings.Split(reportData[5], "-")[0]

	switch migrationLevel {
	case "A":
		return constants.MIGRATION_COMPLEXITY_LOW, nil
	case "B":
		return constants.MIGRATION_COMPLEXITY_MEDIUM, nil
	case "C":
		return constants.MIGRATION_COMPLEXITY_HIGH, nil
	default:
		return "", fmt.Errorf("invalid migration level [%s] found in ora2pg report %v", migrationLevel, reportData)
	}
}

// getComplexityLevel returns LOW, MEDIUM, or HIGH for a given impact level & count
func getComplexityForLevel(level string, count int) string {
	switch level {
	// -------------------------------------------------------
	// LEVEL_1:
	//   - LOW if count <= 20
	//   - MEDIUM if 20 < count < math.MaxInt32
	//   - HIGH if count >= math.MaxInt32 (not possible)
	// -------------------------------------------------------
	case constants.IMPACT_LEVEL_1:
		if count <= LEVEL_1_MEDIUM_THRESHOLD {
			return constants.MIGRATION_COMPLEXITY_LOW
		} else if count <= LEVEL_1_HIGH_THRESHOLD {
			return constants.MIGRATION_COMPLEXITY_MEDIUM
		}
		return constants.MIGRATION_COMPLEXITY_HIGH

	// -------------------------------------------------------
	// LEVEL_2:
	//   - LOW if count <= 10
	//   - MEDIUM if 10 < count <= 100
	//   - HIGH if count > 100
	// -------------------------------------------------------
	case constants.IMPACT_LEVEL_2:
		if count <= LEVEL_2_MEDIUM_THRESHOLD {
			return constants.MIGRATION_COMPLEXITY_LOW
		} else if count <= LEVEL_2_HIGH_THRESHOLD {
			return constants.MIGRATION_COMPLEXITY_MEDIUM
		}
		return constants.MIGRATION_COMPLEXITY_HIGH

	// -------------------------------------------------------
	// LEVEL_3:
	//   - LOW if count == 0
	//	 - MEDIUM if 0 < count <= 4
	//   - HIGH if count > 4
	// -------------------------------------------------------
	case constants.IMPACT_LEVEL_3:
		if count <= LEVEL_3_MEDIUM_THRESHOLD {
			return constants.MIGRATION_COMPLEXITY_LOW
		} else if count <= LEVEL_3_HIGH_THRESHOLD {
			return constants.MIGRATION_COMPLEXITY_MEDIUM
		}
		return constants.MIGRATION_COMPLEXITY_HIGH

	default:
		panic(fmt.Sprintf("unknown impact level %s for determining complexity", level))
	}
}

// ======================================= Migration Complexity Explanation ==========================================

// TODO: discuss if the html should be in main report or here
const explainTemplateHTML = `
{{- if .Summaries }}
	<p>Below is a breakdown of the issues detected in different categories for each impact level.</p>
	<table border="1" cellpadding="5" cellspacing="0" style="border-collapse: collapse;">
		<thead>
			<tr>
				<th>Category</th>
				<th>Level 1</th>
				<th>Level 2</th>
				<th>Level 3</th>
				<th>Total</th>
			</tr>
		</thead>
		<tbody>
		{{- range .Summaries }}
			<tr>
				<td>{{ .Category }}</td>
				<td>{{ index .ImpactCounts "LEVEL_1" }}</td>
				<td>{{ index .ImpactCounts "LEVEL_2" }}</td>
				<td>{{ index .ImpactCounts "LEVEL_3" }}</td>
				<td>{{ .TotalIssueCount }}</td>
			</tr>
		{{- end }}
		</tbody>
	</table>
{{- end }}

<p>
	<strong>Complexity:</strong> {{ .Complexity }}</br>
	<strong>Reasoning:</strong> {{ .ComplexityRationale }}
</p>

<p>
<strong>Impact Levels:</strong></br>
	Level 1: Resolutions are available with minimal effort.<br/>
	Level 2: Resolutions are available requiring moderate effort.<br/>
	Level 3: Resolutions may not be available or are complex.
</p>
`

const explainTemplateText = `Reasoning: {{ .ComplexityRationale }}`

type MigrationComplexityExplanationData struct {
	Summaries           []MigrationComplexityCategorySummary
	Complexity          string
	ComplexityRationale string // short reasoning or explanation text
}

type MigrationComplexityCategorySummary struct {
	Category        string
	TotalIssueCount int
	ImpactCounts    map[string]int // e.g. {"Level-1": 3, "Level-2": 5, "Level-3": 2}
}

func buildMigrationComplexityExplanation(sourceDBType string, assessmentReport AssessmentReport, reportFormat string) (string, error) {
	if sourceDBType != POSTGRESQL {
		return "", nil
	}

	var explanation MigrationComplexityExplanationData
	explanation.Complexity = assessmentReport.MigrationComplexity
	explanation.ComplexityRationale = migrationComplexityRationale

	explanation.Summaries = buildCategorySummary(assessmentReport.Issues)

	var tmpl *template.Template
	var err error
	if reportFormat == "html" {
		tmpl, err = template.New("Explain").Parse(explainTemplateHTML)
	} else {
		tmpl, err = template.New("Explain").Parse(explainTemplateText)
	}

	if err != nil {
		return "", fmt.Errorf("failed creating the explanation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, explanation); err != nil {
		return "", fmt.Errorf("failed executing the template with data: %w", err)
	}
	return buf.String(), nil
}

func buildRationale(finalComplexity string, l1Count int, l2Count int, l3Count int) string {
	switch finalComplexity {
	case constants.MIGRATION_COMPLEXITY_HIGH:
		return fmt.Sprintf("Found %d Level 2 issue(s) and %d Level 3 issue(s), resulting in HIGH migration complexity", l2Count, l3Count)
	case constants.MIGRATION_COMPLEXITY_MEDIUM:
		return fmt.Sprintf("Found %d Level 1 issue(s), %d Level 2 issue(s) and %d Level 3 issue(s), resulting in MEDIUM migration complexity", l1Count, l2Count, l3Count)
	case constants.MIGRATION_COMPLEXITY_LOW:
		return fmt.Sprintf("Found %d Level 1 issue(s) and %d Level 2 issue(s), resulting in LOW migration complexity", l1Count, l2Count)
	}
	return ""
}

func buildCategorySummary(issues []AssessmentIssue) []MigrationComplexityCategorySummary {
	if len(issues) == 0 {
		return nil

	}

	summaryMap := make(map[string]*MigrationComplexityCategorySummary)
	for _, issue := range issues {
		if issue.Category == "" {
			continue // skipping unknown category issues
		}

		if _, ok := summaryMap[issue.Category]; !ok {
			summaryMap[issue.Category] = &MigrationComplexityCategorySummary{
				Category:        issue.Category,
				TotalIssueCount: 0,
				ImpactCounts:    make(map[string]int),
			}
		}

		summaryMap[issue.Category].TotalIssueCount++
		summaryMap[issue.Category].ImpactCounts[issue.Impact]++
	}

	var result []MigrationComplexityCategorySummary
	for _, summary := range summaryMap {
		summary.Category = utils.SnakeCaseToTitleCase(summary.Category)
		result = append(result, *summary)
	}
	return result
}
