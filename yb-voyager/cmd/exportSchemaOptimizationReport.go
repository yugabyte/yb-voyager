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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/sqltransformer"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// =============================================== schema optimization changes report

var schemaOptimizationReport *SchemaOptimizationReport

// File names for optimization reports
const (
	RedundantIndexesFileName                        = "redundant_indexes.sql"
	SchemaOptimizationReportFileName                = "schema_optimization_report"
	REDUNDANT_INDEXES_DESCRIPTION                   = "The following indexes were identified as redundant. These indexes were fully covered by stronger indexes—indexes that share the same leading key columns (in order) and potentially include additional columns, making the redundant ones unnecessary."
	APPLIED_RECOMMENDATIONS_NOT_APPLIED_DESCRIPTION = "Sharding recommendations were not applied due to the skip-recommendations flag. Modify the schema manually as per the recommendations in assessment report."
	REDUNDANT_INDEXES_NOT_APPLIED_DESCRIPTION       = REDUNDANT_INDEXES_DESCRIPTION + "\nThese indexes were not removed due to the skip-performance-optimizations flag. Remove them manually from the schema."
	RANGE_SHARDED_SECONDARY_INDEXES_DESCRIPTION     = "The following secondary indexes were configured to be range-sharded indexes in YugabyteDB. This helps in giving the flexibility to execute range-based queries, and avoids potential hotspots that come with hash-sharded indexes such as index on low cardinality column, index on high percentage of NULLs and index on high percentage of particular value."
)

// SchemaOptimizationReport represents a comprehensive report of schema optimizations
// applied during the export process, including redundant index removal and sharding recommendations.
type SchemaOptimizationReport struct {
	// Metadata about the export process
	VoyagerVersion        string `json:"voyager_version"`
	SourceDatabaseName    string `json:"source_database_name"`
	SourceDatabaseSchema  string `json:"source_database_schema"`
	SourceDatabaseVersion string `json:"source_database_version"`

	// Optimization changes applied
	RedundantIndexChange        *RedundantIndexChange         `json:"redundant_index_change,omitempty"`
	TableShardingRecommendation *ShardingRecommendationChange `json:"table_sharding_recommendation,omitempty"`
	MviewShardingRecommendation *ShardingRecommendationChange `json:"mview_sharding_recommendation,omitempty"`
	SecondaryIndexToRangeChange *SecondaryIndexToRangeChange  `json:"secondary_index_to_range_change,omitempty"`
}

// HasOptimizations returns true if any optimizations were applied
func (s *SchemaOptimizationReport) HasOptimizations() bool {
	return !s.RedundantIndexChange.IsEmpty() ||
		!s.TableShardingRecommendation.IsEmpty() ||
		!s.MviewShardingRecommendation.IsEmpty()
}

// NewSchemaOptimizationReport creates a new SchemaOptimizationReport with the given metadata
func NewSchemaOptimizationReport(voyagerVersion, dbName, dbSchema, dbVersion string) *SchemaOptimizationReport {
	return &SchemaOptimizationReport{
		VoyagerVersion:        voyagerVersion,
		SourceDatabaseName:    dbName,
		SourceDatabaseSchema:  dbSchema,
		SourceDatabaseVersion: dbVersion,
	}
}

// RedundantIndexChange represents the removal of redundant indexes that are fully
// covered by stronger indexes, improving performance and reducing storage overhead.
type RedundantIndexChange struct {
	Title                    string              `json:"title"`
	Description              string              `json:"description"`
	ReferenceFile            string              `json:"reference_file"`
	TableToRemovedIndexesMap map[string][]string `json:"table_to_removed_indexes_map"`
	IsApplied                bool                `json:"is_applied"`
}

// IsEmpty returns true if no redundant indexes were removed
func (r *RedundantIndexChange) IsEmpty() bool {
	return r == nil || len(r.TableToRemovedIndexesMap) == 0
}

// NewRedundantIndexChange creates a new RedundantIndexChange with default values
func NewRedundantIndexChange(applied bool, referenceFile string, tableToRemovedIndexesMap map[string][]string) *RedundantIndexChange {
	title := "Redundant Indexes - Removed"
	description := REDUNDANT_INDEXES_DESCRIPTION
	if !applied {
		title = "Redundant Indexes - Not Removed"
		description = REDUNDANT_INDEXES_NOT_APPLIED_DESCRIPTION
	}
	return &RedundantIndexChange{
		Title:                    title,
		Description:              description,
		ReferenceFile:            referenceFile,
		TableToRemovedIndexesMap: tableToRemovedIndexesMap,
		IsApplied:                applied,
	}
}

// ShardingRecommendationChange represents the application of sharding recommendations
// to database objects (tables or materialized views) for improved performance.
type ShardingRecommendationChange struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	ReferenceFile     string   `json:"reference_file"`
	ShardedObjects    []string `json:"sharded_objects"`
	CollocatedObjects []string `json:"collocated_objects"`
	IsApplied         bool     `json:"is_applied"`
}

// IsEmpty returns true if no sharding recommendations were applied
func (a *ShardingRecommendationChange) IsEmpty() bool {
	return a == nil || (len(a.ShardedObjects) == 0)
}

// NewAppliedShardingRecommendationChange creates a new AppliedShardingRecommendationChange with default values
func NewAppliedShardingRecommendationChange(objectType string, applied bool, referenceFile string, shardedObjects []string, colocatedObjects []string) *ShardingRecommendationChange {
	var title, description string
	switch objectType {
	case TABLE:
		title = "Sharding Recommendations to Tables - Applied"
		description = "Sharding recommendations from the assessment have been applied to the tables to optimize data distribution and performance. Tables will be created as colocated automatically according to the target database configuration."
	case MVIEW:
		title = "Sharding Recommendations to Materialized Views - Applied"
		description = "Sharding recommendations from the assessment have been applied to the mviews to optimize data distribution and performance. MViews will be created as colocated automatically according to the target database configuration."
	default:
		title = "Sharding Recommendations - Applied"
		description = "Sharding recommendations from the assessment have been applied to optimize data distribution and performance."
	}

	if !applied {
		title = "Sharding Recommendations - Not Applied"
		description = APPLIED_RECOMMENDATIONS_NOT_APPLIED_DESCRIPTION
	}
	return &ShardingRecommendationChange{
		Title:             title,
		Description:       description,
		ReferenceFile:     referenceFile,
		ShardedObjects:    shardedObjects,
		CollocatedObjects: colocatedObjects,
		IsApplied:         applied,
	}
}

type SecondaryIndexToRangeChange struct {
	Title                   string            `json:"title"`
	Description             string            `json:"description"`
	HyperLinksInDescription map[string]string `json:"hyper_links_in_description"` //map of text and hyperlink on that text

	ReferenceFile   string              `json:"reference_file"`
	ModifiedIndexes map[string][]string `json:"modified_indexes"`
	IsApplied       bool                `json:"is_applied"`
}

func NewSecondaryIndexToRangeChange(applied bool, referenceFile string, modifiedIndexes map[string][]string) *SecondaryIndexToRangeChange {
	title := "Secondary Indexes to be range-sharded - Applied"
	description := "The following secondary indexes were configured to be range-sharded indexes in YugabyteDB."
	if !applied {
		title = "Secondary Indexes to be range-sharded - Not Applied"
		description = "Due to the skip-performance-optimizations flag, all the btree indexes were not converted to range-sharded indexes. Modify the indexes to be range-sharded manually."
	}
	description += "The range-sharded indexes helps in giving the flexibility to execute range-based queries, and avoids potential hotspots that come with hash-sharded indexes such as index on low cardinality column, index on high percentage of NULLs, and index on high percentage of particular value. Refer to sharding strategy in documentation for more information."
	return &SecondaryIndexToRangeChange{
		Title:       title,
		Description: description,
		HyperLinksInDescription: map[string]string{
			"index on low cardinality column":              "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-low-cardinality-column",
			"index on high percentage of NULLs":            "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-a-high-percentage-of-null-values",
			"index on high percentage of particular value": "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-high-percentage-of-a-particular-value",
			"documentation":                    "https://docs.yugabyte.com/preview/architecture/docdb-sharding/sharding/",
		},
		ReferenceFile:   referenceFile,
		ModifiedIndexes: modifiedIndexes,
		IsApplied:       applied,
	}
}

func buildRedundantIndexChange(indexTransformer *sqltransformer.IndexFileTransformer) *RedundantIndexChange {
	if indexTransformer == nil {
		return nil
	}
	// Get relative path from reports directory to the redundant indexes file
	reportsDir := filepath.Join(exportDir, "reports")
	redundantIndexesFile := indexTransformer.RedundantIndexesFileName
	relativePath, err := filepath.Rel(reportsDir, redundantIndexesFile)
	if err == nil {
		//If no error, then use the relative path
		redundantIndexesFile = relativePath
	}

	redundantIndexesToRemove := indexTransformer.RedundantIndexesToExistingIndexToRemove.ActualKeys()
	redundantIndexes := indexTransformer.RemovedRedundantIndexes
	if skipPerfOptimizations {
		redundantIndexes = redundantIndexesToRemove
	}
	return NewRedundantIndexChange(!bool(skipPerfOptimizations), redundantIndexesFile, getTableToIndexMap(redundantIndexes))
}

func buildShardingTableRecommendationChange(shardedTables []string, colocatedTables []string) *ShardingRecommendationChange {
	if !assessmentRecommendationsApplied { //If assessment recommendations not applied and skip recommendations is true, then show that its not applied
		if skipRecommendations {
			return NewAppliedShardingRecommendationChange("", false, "", nil, nil) // Dummy entry for both table and mview as no need to show two
		}
		return nil
	}

	referenceTableFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), TABLE)
	//To tables then add that change
	if !utils.FileOrFolderExists(referenceTableFile) || len(shardedTables) == 0 { // only display this in case there is any modifield sharded tables
		return nil
	}

	return NewAppliedShardingRecommendationChange(TABLE, true, getRelativePathWithReportsDir(referenceTableFile), shardedTables, colocatedTables)
}

func buildShardingMviewRecommendationChange(shardedMviews []string, colocatedMviews []string) *ShardingRecommendationChange {
	if !assessmentRecommendationsApplied { //If assessment recommendations not applied, we are already addding a generic section for Sharding recommendations not applied with table case above
		return nil
	}
	referenceMviewFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), MVIEW)
	//To mviews then add that change separately
	if !utils.FileOrFolderExists(referenceMviewFile) || len(shardedMviews) == 0 { // only display this in case there is any modifield sharded mview
		return nil
	}
	return NewAppliedShardingRecommendationChange(MVIEW, true, getRelativePathWithReportsDir(referenceMviewFile), shardedMviews, colocatedMviews)
}

func buildSecondaryIndexToRangeChange(indexTransformer *sqltransformer.IndexFileTransformer) *SecondaryIndexToRangeChange {
	if indexTransformer == nil {
		return nil
	}
	if skipPerfOptimizations {
		return NewSecondaryIndexToRangeChange(false, "", nil)
	}
	referenceIndexesFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), INDEX)
	return NewSecondaryIndexToRangeChange(true, getRelativePathWithReportsDir(referenceIndexesFile), getTableToIndexMap(indexTransformer.ModifiedIndexesToRange))
}

//go:embed templates/schema_optimization_report.template
var optimizationChangesTemplate []byte

// generatePerformanceOptimizationReport generates an HTML report detailing performance optimization changes applied to the exported schema.
// It reports the removal of redundant indexes and the application of sharding recommendations to tables and materialized views (mviews).
// The report includes references to the relevant SQL files and lists the modified objects.
// Parameters:
//   - redundantIndexes: list of redundant index names that were removed.
//   - tables: list of table names to which sharding recommendations were applied.
//   - mviews: list of materialized view names to which sharding recommendations were applied.
func generatePerformanceOptimizationReport(indexTransformer *sqltransformer.IndexFileTransformer, shardedTables []string, shardedMviews []string, colocatedTables []string, colocatedMviews []string) error {

	if source.DBType != POSTGRESQL {
		//Not generating the report in case other than PG
		return nil
	}
	htmlReportFilePath := filepath.Join(exportDir, "reports", fmt.Sprintf("%s%s", SchemaOptimizationReportFileName, HTML_EXTENSION))
	log.Infof("writing changes report to file: %s", htmlReportFilePath)

	funcMap := template.FuncMap{
		"replaceLinks": func(text string, links map[string]string) string {
			result := text
			for textToReplace, url := range links {
				linkHTML := fmt.Sprintf(`<a href="%s" target="_blank" title="%s">%s</a>`, url, url, textToReplace)
				result = strings.ReplaceAll(result, textToReplace, linkHTML)
			}
			return result
		},
	}

	tmpl := template.Must(template.New("report").Funcs(funcMap).Parse(string(optimizationChangesTemplate)))
	schemaOptimizationReport = NewSchemaOptimizationReport(
		utils.YB_VOYAGER_VERSION,
		source.DBName,
		strings.Join(strings.Split(source.Schema, "|"), ", "),
		source.DBVersion,
	)
	schemaOptimizationReport.RedundantIndexChange = buildRedundantIndexChange(indexTransformer)
	schemaOptimizationReport.TableShardingRecommendation = buildShardingTableRecommendationChange(shardedTables, colocatedTables)
	schemaOptimizationReport.MviewShardingRecommendation = buildShardingMviewRecommendationChange(shardedMviews, colocatedMviews)
	schemaOptimizationReport.SecondaryIndexToRangeChange = buildSecondaryIndexToRangeChange(indexTransformer)

	if schemaOptimizationReport.HasOptimizations() {
		file, err := os.Create(htmlReportFilePath)
		if err != nil {
			return fmt.Errorf("failed to create file for %q: %w", filepath.Base(htmlReportFilePath), err)
		}
		err = tmpl.Execute(file, schemaOptimizationReport)
		if err != nil {
			return fmt.Errorf("failed to render the schema optimization report: %w", err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf("failed to close file %q: %w", htmlReportFilePath, err)
		}

		color.Green("\nSome Optimization changes were applied to the exported schema, refer to the detailed report for more information: %s", htmlReportFilePath)
	}

	return nil
}

func getTableToIndexMap(indexes []*sqlname.ObjectNameQualifiedWithTableName) map[string][]string {
	tableToIndexMap := make(map[string][]string)
	for _, obj := range indexes {
		tableName := obj.GetQualifiedTableName()
		indexName := obj.GetObjectName()
		tableToIndexMap[tableName] = append(tableToIndexMap[tableName], indexName)
	}
	return tableToIndexMap
}

func getRelativePathWithReportsDir(filePath string) string {
	reportsDir := filepath.Join(exportDir, "reports")
	relativePath, err := filepath.Rel(reportsDir, filePath)
	if err == nil {
		//If no error, then use the relative path
		filePath = relativePath
	}
	return filePath
}
