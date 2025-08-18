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
	ReferenceFileDisplayName string              `json:"reference_file_display_name"`
	TableToRemovedIndexesMap map[string][]string `json:"table_to_removed_indexes_map"`
	IsApplied                bool                `json:"is_applied"`
}

// IsEmpty returns true if no redundant indexes were removed
func (r *RedundantIndexChange) IsEmpty() bool {
	return r == nil || len(r.TableToRemovedIndexesMap) == 0
}

// NewRedundantIndexChange creates a new RedundantIndexChange with default values
func NewRedundantIndexChange(applied bool, referenceFile string, displayFileName string, tableToRemovedIndexesMap map[string][]string) *RedundantIndexChange {
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
		ReferenceFileDisplayName: displayFileName,
		TableToRemovedIndexesMap: tableToRemovedIndexesMap,
		IsApplied:                applied,
	}
}

// ShardingRecommendationChange represents the application of sharding recommendations
// to database objects (tables or materialized views) for improved performance.
type ShardingRecommendationChange struct {
	Title                    string   `json:"title"`
	Description              string   `json:"description"`
	ReferenceFile            string   `json:"reference_file"`
	ReferenceFileDisplayName string   `json:"reference_file_display_name"`
	ShardedObjects           []string `json:"sharded_objects"`
	CollocatedObjects        []string `json:"collocated_objects"`
	IsApplied                bool     `json:"is_applied"`
}

// IsEmpty returns true if no sharding recommendations were applied
func (a *ShardingRecommendationChange) IsEmpty() bool {
	return a == nil || (len(a.ShardedObjects) == 0)
}

// NewAppliedShardingRecommendationChange creates a new AppliedShardingRecommendationChange with default values
func NewAppliedShardingRecommendationChange(objectType string, applied bool) *ShardingRecommendationChange {
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
		ShardedObjects:    make([]string, 0),
		CollocatedObjects: make([]string, 0),
		IsApplied:         applied,
	}
}

type SecondaryIndexToRangeChange struct {
	Title                    string              `json:"title"`
	Description              string              `json:"description"`
	ReferenceFile            string              `json:"reference_file"`
	ReferenceFileDisplayName string              `json:"reference_file_display_name"`
	ModifiedIndexes          map[string][]string `json:"modified_indexes"`
	IsApplied                bool                `json:"is_applied"`
}

func NewSecondaryIndexToRangeChange(applied bool, referenceFile string, displayFileName string, modifiedIndexes map[string][]string) *SecondaryIndexToRangeChange {
	title := "Secondary Indexes to be range-sharded - Applied"
	description := "The following secondary indexes were converted to range-sharded indexes. This helps in distributing the data evenly across the nodes and improves the performance of these indexes."
	if !applied {
		title = "Secondary Indexes to be range-sharded - Not Applied"
		description = "Range-sharded secondary indexes helps in better data distribution and performance. Due to the skip-performance-optimizations flag, all the btree indexes were not converted to range-sharded indexes. Modify the indexes to be range-sharded manually."
	}
	return &SecondaryIndexToRangeChange{
		Title:                    title,
		Description:              description,
		ReferenceFile:            referenceFile,
		ReferenceFileDisplayName: displayFileName,
		ModifiedIndexes:          modifiedIndexes,
		IsApplied:                applied,
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
	return NewRedundantIndexChange(!bool(skipPerfOptimizations), redundantIndexesFile, RedundantIndexesFileName, GetTableToIndexMap(redundantIndexes))
}

func buildShardingTableRecommendationChange(shardedTables []string, colocatedTables []string) *ShardingRecommendationChange {
	if !assessmentRecommendationsApplied { //If assessment recommendations not applied and skip recommendations is true, then show that its not applied
		if skipRecommendations {
			return NewAppliedShardingRecommendationChange("", false) // Dummy entry for both table and mview as no need to show two
		}
		return nil
	}

	referenceTableFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), TABLE)
	//To tables then add that change
	if !utils.FileOrFolderExists(referenceTableFile) || len(shardedTables) == 0 { // only display this in case there is any modifield sharded tables
		return nil
	}
	// Get relative path from reports directory to the table file
	reportsDir := filepath.Join(exportDir, "reports")
	relativePath, err := filepath.Rel(reportsDir, referenceTableFile)
	if err == nil {
		//If no error, then use the relative path
		referenceTableFile = relativePath
	}

	change := NewAppliedShardingRecommendationChange(TABLE, true)
	change.ReferenceFile = referenceTableFile
	change.ReferenceFileDisplayName = "table.sql"
	change.ShardedObjects = shardedTables
	change.CollocatedObjects = colocatedTables
	return change
}

func buildShardingMviewRecommendationChange(shardedMviews []string, colocatedMviews []string) *ShardingRecommendationChange {
	referenceMviewFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), MVIEW)
	//To mviews then add that change separately
	if !utils.FileOrFolderExists(referenceMviewFile) || len(shardedMviews) == 0 { // only display this in case there is any modifield sharded mview
		return nil
	}
	// Get relative path from reports directory to the mview file
	reportsDir := filepath.Join(exportDir, "reports")
	relativePath, err := filepath.Rel(reportsDir, referenceMviewFile)
	if err == nil {
		//If no error, then use the relative path
		referenceMviewFile = relativePath
	}
	change := NewAppliedShardingRecommendationChange(MVIEW, true)
	change.ReferenceFile = referenceMviewFile
	change.ReferenceFileDisplayName = "mview.sql"
	change.ShardedObjects = shardedMviews
	change.CollocatedObjects = colocatedMviews
	return change
}

func buildSecondaryIndexToRangeChange(indexTransformer *sqltransformer.IndexFileTransformer) *SecondaryIndexToRangeChange {
	if indexTransformer == nil {
		return nil
	}
	if skipPerfOptimizations {
		return NewSecondaryIndexToRangeChange(false, "", "", nil)
	}
	referenceIndexesFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), INDEX)
	reportsDir := filepath.Join(exportDir, "reports")
	relativePath, err := filepath.Rel(reportsDir, referenceIndexesFile)
	if err == nil {
		//If no error, then use the relative path
		referenceIndexesFile = relativePath
	}
	return NewSecondaryIndexToRangeChange(true, referenceIndexesFile, "index.sql", GetTableToIndexMap(indexTransformer.ModifiedIndexesToRange))
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

	tmpl := template.Must(template.New("report").Parse(string(optimizationChangesTemplate)))
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

func GetTableToIndexMap(indexes []*sqlname.ObjectNameQualifiedWithTableName) map[string][]string {
	tableToIndexMap := make(map[string][]string)
	for _, obj := range indexes {
		tableName := obj.GetQualifiedTableName()
		indexName := obj.GetObjectName()
		tableToIndexMap[tableName] = append(tableToIndexMap[tableName], indexName)
	}
	return tableToIndexMap
}
