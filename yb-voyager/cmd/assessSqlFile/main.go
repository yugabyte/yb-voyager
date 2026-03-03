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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

type assessedIssue struct {
	IssueType   string `json:"issue_type"`
	Name        string `json:"name"`
	ObjectType  string `json:"object_type,omitempty"`
	ObjectName  string `json:"object_name,omitempty"`
	Description string `json:"description"`
	Impact      string `json:"impact,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
	DocsLink    string `json:"docs_link,omitempty"`
	GHIssue     string `json:"gh_issue,omitempty"`
	SqlPreview  string `json:"sql_preview"`
}

var perfIssueTypes = map[string]bool{
	"HOTSPOTS_ON_TIMESTAMP_INDEX":                true,
	"HOTSPOTS_ON_DATE_INDEX":                     true,
	"HOTSPOTS_ON_TIMESTAMP_PK_UK":                true,
	"HOTSPOTS_ON_DATE_PK_UK":                     true,
	"REDUNDANT_INDEXES":                          true,
	"MISSING_FOREIGN_KEY_INDEX":                  true,
	"MISSING_PRIMARY_KEY_WHEN_UNIQUE_NOT_NULL":   true,
}

func main() {
	targetVersionStr := flag.String("target-db-version", "2024.2.3.1", "Target YugabyteDB version (e.g. 2024.2.3.1, 2025.1.0.0)")
	outputFormat := flag.String("format", "text", "Output format: text or json")
	verbose := flag.Bool("verbose", false, "Enable verbose/debug logging")
	skipPerf := flag.Bool("skip-perf-issues", false, "Skip performance/optimization issues (hotspots, missing FK indexes, PK recommendations)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <sql-file> [sql-file...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Assess SQL files for YugabyteDB compatibility issues.\n")
		fmt.Fprintf(os.Stderr, "Each SQL file may contain multiple statements separated by semicolons.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s migrations.sql\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --target-db-version 2025.1.0.0 schema.sql data.sql\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --format json migrations.sql\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --skip-perf-issues migrations.sql\n", os.Args[0])
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if *verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	targetDbVersion, err := ybversion.NewYBVersion(*targetVersionStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid target DB version %q: %v\n", *targetVersionStr, err)
		fmt.Fprintf(os.Stderr, "Supported version series: 2024.1, 2024.2, 2025.1, 2025.2, 2.23, 2.25\n")
		os.Exit(1)
	}

	allStatements, err := collectStatements(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading SQL files: %v\n", err)
		os.Exit(1)
	}

	if len(allStatements) == 0 {
		fmt.Fprintf(os.Stderr, "No SQL statements found in the provided files.\n")
		os.Exit(0)
	}

	issues := runAssessment(allStatements, targetDbVersion, *skipPerf)

	switch *outputFormat {
	case "json":
		printJSON(issues)
	default:
		printText(issues)
	}

	if len(issues) > 0 {
		os.Exit(2) // non-zero exit to indicate issues found
	}
}

type statement struct {
	SQL      string
	File     string
	StmtNum  int
}

func collectStatements(files []string) ([]statement, error) {
	var result []statement
	for _, file := range files {
		stmts, err := splitSqlFile(file)
		if err != nil {
			return nil, fmt.Errorf("processing %s: %w", file, err)
		}
		result = append(result, stmts...)
	}
	return result, nil
}

func splitSqlFile(filePath string) ([]statement, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	sqlContent := string(content)
	if strings.TrimSpace(sqlContent) == "" {
		return nil, nil
	}

	rawStmts, err := splitIntoStatements(sqlContent)
	if err != nil {
		return nil, fmt.Errorf("splitting statements: %w", err)
	}

	var result []statement
	for i, sql := range rawStmts {
		sql = strings.TrimSpace(sql)
		if sql == "" {
			continue
		}
		result = append(result, statement{
			SQL:     sql,
			File:    filepath.Base(filePath),
			StmtNum: i + 1,
		})
	}
	return result, nil
}

// splitIntoStatements uses pg_query to parse the full SQL content
// and extracts individual statements based on their byte offsets.
func splitIntoStatements(sqlContent string) ([]string, error) {
	parseResult, err := pg_query.Parse(sqlContent)
	if err != nil {
		// If full parse fails, fall back to semicolon splitting with dollar-quote awareness
		return fallbackSplit(sqlContent), nil
	}

	var stmts []string
	for _, rawStmt := range parseResult.Stmts {
		start := int(rawStmt.StmtLocation)
		end := start + int(rawStmt.StmtLen)
		if rawStmt.StmtLen == 0 {
			end = len(sqlContent)
		}
		if end > len(sqlContent) {
			end = len(sqlContent)
		}
		stmtText := strings.TrimSpace(sqlContent[start:end])
		stmtText = strings.TrimRight(stmtText, ";")
		stmtText = strings.TrimSpace(stmtText)
		if stmtText != "" {
			stmts = append(stmts, stmtText+";")
		}
	}
	return stmts, nil
}

// fallbackSplit is a simple semicolon-based splitter that respects dollar-quoted strings.
func fallbackSplit(sqlContent string) []string {
	var stmts []string
	var current strings.Builder
	inDollarQuote := false
	dollarTag := ""

	lines := strings.Split(sqlContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}

		if inDollarQuote {
			current.WriteString(line)
			current.WriteString("\n")
			if strings.Contains(line, dollarTag) {
				// Check if this is the closing dollar quote
				afterFirst := strings.SplitN(line, dollarTag, 2)
				if len(afterFirst) > 1 && strings.Contains(afterFirst[1], dollarTag) {
					// both opening and closing on same line (after the opening), not helpful
				}
				// Simple heuristic: if we see the tag and we're already inside, treat as close
				parts := strings.Split(line, dollarTag)
				if len(parts) >= 2 { // has at least one occurrence
					inDollarQuote = false
				}
			}
			if !inDollarQuote {
				rest := strings.TrimSpace(current.String())
				rest = strings.TrimRight(rest, ";")
				rest = strings.TrimSpace(rest)
				if rest != "" {
					stmts = append(stmts, rest+";")
				}
				current.Reset()
			}
			continue
		}

		// Check for dollar-quote start
		if idx := strings.Index(line, "$$"); idx >= 0 {
			inDollarQuote = true
			dollarTag = "$$"
			current.WriteString(line)
			current.WriteString("\n")
			// Check if it closes on the same line (two occurrences)
			afterFirst := line[idx+2:]
			if strings.Contains(afterFirst, "$$") {
				inDollarQuote = false
				rest := strings.TrimSpace(current.String())
				rest = strings.TrimRight(rest, ";")
				rest = strings.TrimSpace(rest)
				if rest != "" {
					stmts = append(stmts, rest+";")
				}
				current.Reset()
			}
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			stmt = strings.TrimRight(stmt, ";")
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				stmts = append(stmts, stmt+";")
			}
			current.Reset()
		}
	}

	// Handle any remaining content
	remaining := strings.TrimSpace(current.String())
	remaining = strings.TrimRight(remaining, ";")
	remaining = strings.TrimSpace(remaining)
	if remaining != "" {
		stmts = append(stmts, remaining+";")
	}

	return stmts
}

func runAssessment(stmts []statement, targetDbVersion *ybversion.YBVersion, skipPerf bool) []assessedIssue {
	detector := queryissue.NewParserIssueDetector()

	// Phase 1: Parse all DDLs to build schema metadata
	fmt.Fprintf(os.Stderr, "Phase 1: Building schema metadata from %d statements...\n", len(stmts))
	ddlCount := 0
	for _, stmt := range stmts {
		err := detector.ParseAndProcessDDL(stmt.SQL)
		if err != nil {
			log.Debugf("[%s stmt#%d] DDL parse skipped: %v", stmt.File, stmt.StmtNum, err)
			continue
		}
		ddlCount++
	}
	fmt.Fprintf(os.Stderr, "  Processed %d DDL statements for metadata.\n", ddlCount)

	detector.FinalizeTablesMetadata()

	// Phase 2: Detect issues in all statements
	fmt.Fprintf(os.Stderr, "Phase 2: Detecting compatibility issues...\n")
	var allIssues []assessedIssue

	for _, stmt := range stmts {
		issues, err := detector.GetAllIssues(stmt.SQL, targetDbVersion)
		if err != nil {
			log.Debugf("[%s stmt#%d] Issue detection error: %v", stmt.File, stmt.StmtNum, err)
			continue
		}

		for _, qi := range issues {
			if skipPerf && perfIssueTypes[qi.Type] {
				continue
			}
			sqlPreview := truncateSQL(stmt.SQL, 200)
			allIssues = append(allIssues, assessedIssue{
				IssueType:   qi.Type,
				Name:        qi.Name,
				ObjectType:  qi.ObjectType,
				ObjectName:  qi.ObjectName,
				Description: qi.Description,
				Impact:      qi.Impact,
				Suggestion:  qi.Suggestion,
				DocsLink:    qi.DocsLink,
				GHIssue:     qi.GH,
				SqlPreview:  sqlPreview,
			})
		}

		allIssues = append(allIssues, detectAlterDropAddColumn(stmt.SQL)...)
	}

	// Phase 3: Detect cross-statement issues (missing FK indexes, PK recommendations)
	if !skipPerf {
		fkIssues := detector.DetectMissingForeignKeyIndexes()
		for _, qi := range fkIssues {
			preview := buildCrossStmtPreview(qi)
			allIssues = append(allIssues, assessedIssue{
				IssueType:   qi.Type,
				Name:        qi.Name,
				ObjectType:  qi.ObjectType,
				ObjectName:  qi.ObjectName,
				Description: qi.Description,
				Impact:      qi.Impact,
				Suggestion:  qi.Suggestion,
				DocsLink:    qi.DocsLink,
				GHIssue:     qi.GH,
				SqlPreview:  preview,
			})
		}

		pkIssues := detector.DetectPrimaryKeyRecommendations()
		for _, qi := range pkIssues {
			preview := buildCrossStmtPreview(qi)
			allIssues = append(allIssues, assessedIssue{
				IssueType:   qi.Type,
				Name:        qi.Name,
				ObjectType:  qi.ObjectType,
				ObjectName:  qi.ObjectName,
				Description: qi.Description,
				Impact:      qi.Impact,
				Suggestion:  qi.Suggestion,
				DocsLink:    qi.DocsLink,
				GHIssue:     qi.GH,
				SqlPreview:  preview,
			})
		}
	}

	return allIssues
}

// detectAlterDropAddColumn detects Prisma-style "DROP COLUMN x, ADD COLUMN x newtype" in a single ALTER.
// This is how Prisma changes column types — and the new type might not be supported on YB.
func detectAlterDropAddColumn(sql string) []assessedIssue {
	parseResult, err := pg_query.Parse(sql)
	if err != nil {
		return nil
	}

	var issues []assessedIssue
	for _, rawStmt := range parseResult.Stmts {
		alter := rawStmt.Stmt.GetAlterTableStmt()
		if alter == nil || len(alter.Cmds) < 2 {
			continue
		}

		tableName := alter.Relation.Relname
		if alter.Relation.Schemaname != "" {
			tableName = alter.Relation.Schemaname + "." + tableName
		}

		droppedCols := make(map[string]bool)
		type addedCol struct {
			name     string
			typeName string
		}
		var addedCols []addedCol

		for _, cmd := range alter.Cmds {
			atCmd := cmd.GetAlterTableCmd()
			switch atCmd.Subtype {
			case pg_query.AlterTableType_AT_DropColumn:
				droppedCols[atCmd.Name] = true
			case pg_query.AlterTableType_AT_AddColumn:
				colDef := atCmd.Def.GetColumnDef()
				if colDef == nil {
					continue
				}
				var typeNames []string
				if colDef.TypeName != nil {
					for _, n := range colDef.TypeName.Names {
						typeNames = append(typeNames, n.GetString_().Sval)
					}
				}
				typeName := strings.Join(typeNames, ".")
				addedCols = append(addedCols, addedCol{name: colDef.Colname, typeName: typeName})
			}
		}

		for _, ac := range addedCols {
			if !droppedCols[ac.name] {
				continue
			}
			issues = append(issues, assessedIssue{
				IssueType:   "ALTER_DROP_ADD_COLUMN_TYPE_CHANGE",
				Name:        "Column type change via DROP+ADD",
				ObjectType:  "TABLE",
				ObjectName:  tableName,
				Description: fmt.Sprintf("Column %q is dropped and re-added with type %q in a single ALTER statement. Verify the new type is supported on YugabyteDB and that any dependent objects (views, indexes, constraints) are handled correctly.", ac.name, ac.typeName),
				Impact:      "LEVEL_2",
				SqlPreview:  truncateSQL(sql, 200),
			})
		}
	}
	return issues
}

func buildCrossStmtPreview(qi queryissue.QueryIssue) string {
	if qi.SqlStatement != "" {
		return truncateSQL(qi.SqlStatement, 200)
	}
	var parts []string
	if qi.ObjectName != "" {
		parts = append(parts, fmt.Sprintf("table=%s", qi.ObjectName))
	}
	if fkCols, ok := qi.Details["ForeignKeyColumnNames"]; ok {
		parts = append(parts, fmt.Sprintf("fk_columns=(%s)", fkCols))
	}
	if refTable, ok := qi.Details["ReferencedTableName"]; ok {
		parts = append(parts, fmt.Sprintf("references=%s", refTable))
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return "(cross-statement issue, no single SQL)"
}

func truncateSQL(sql string, maxLen int) string {
	sql = strings.Join(strings.Fields(sql), " ")
	if len(sql) > maxLen {
		return sql[:maxLen] + "..."
	}
	return sql
}

func printText(issues []assessedIssue) {
	if len(issues) == 0 {
		fmt.Println("\nNo compatibility issues found!")
		return
	}

	fmt.Printf("\n=== YugabyteDB Compatibility Assessment ===\n")
	fmt.Printf("Found %d issue(s):\n\n", len(issues))

	// Group by issue type for readability
	grouped := make(map[string][]assessedIssue)
	var order []string
	for _, issue := range issues {
		if _, exists := grouped[issue.IssueType]; !exists {
			order = append(order, issue.IssueType)
		}
		grouped[issue.IssueType] = append(grouped[issue.IssueType], issue)
	}

	issueNum := 1
	for _, issueType := range order {
		group := grouped[issueType]
		fmt.Printf("--- %s (%d occurrence(s)) ---\n", group[0].Name, len(group))

		// Check if all descriptions in the group are the same
		allSameDesc := true
		for _, item := range group {
			if item.Description != group[0].Description {
				allSameDesc = false
				break
			}
		}

		if allSameDesc && group[0].Description != "" {
			fmt.Printf("    Description: %s\n", group[0].Description)
		}
		if group[0].Suggestion != "" {
			fmt.Printf("    Suggestion:  %s\n", group[0].Suggestion)
		}
		if group[0].DocsLink != "" {
			fmt.Printf("    Docs:        %s\n", group[0].DocsLink)
		}
		if group[0].GHIssue != "" {
			fmt.Printf("    GH Issue:    %s\n", group[0].GHIssue)
		}
		fmt.Println()
		for _, item := range group {
			objInfo := ""
			if item.ObjectType != "" || item.ObjectName != "" {
				objInfo = fmt.Sprintf(" [%s %s]", item.ObjectType, item.ObjectName)
			}
			if !allSameDesc && item.Description != "" {
				fmt.Printf("  %d.%s %s\n     SQL: %s\n", issueNum, objInfo, item.Description, item.SqlPreview)
			} else {
				fmt.Printf("  %d.%s SQL: %s\n", issueNum, objInfo, item.SqlPreview)
			}
			issueNum++
		}
		fmt.Println()
	}
}

func printJSON(issues []assessedIssue) {
	output := struct {
		TotalIssues int             `json:"total_issues"`
		Issues      []assessedIssue `json:"issues"`
	}{
		TotalIssues: len(issues),
		Issues:      issues,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}
