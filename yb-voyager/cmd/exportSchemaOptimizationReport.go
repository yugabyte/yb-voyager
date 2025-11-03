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

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/sqltransformer"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// =============================================== schema optimization changes report

var schemaOptimizationReport *SchemaOptimizationReport

// File names for optimization reports
const (
	RedundantIndexesFileName                        = "redundant_indexes.sql"
	SCHEMA_OPTIMIZATION_REPORT_FILE_NAME            = "schema_optimization_report"
	REDUNDANT_INDEXES_DESCRIPTION                   = "The following indexes were identified as redundant. These indexes were fully covered by stronger indexes—indexes that share the same leading key columns (in order) and potentially include additional columns, making the redundant ones unnecessary."
	APPLIED_RECOMMENDATIONS_NOT_APPLIED_DESCRIPTION = "Colocation recommendations were not applied due to the skip-colocation-recommendations flag. Modify the schema manually as per the recommendations in assessment report."
	REDUNDANT_INDEXES_NOT_APPLIED_DESCRIPTION       = REDUNDANT_INDEXES_DESCRIPTION + "\nThese indexes were not removed due to the skip-performance-recommendations flag. Remove them manually from the schema."
	RANGE_SHARDED_SECONDARY_INDEXES_DESCRIPTION     = "The following secondary indexes were configured to be range-sharded indexes in YugabyteDB. This helps in giving the flexibility to execute range-based queries, and avoids potential hotspots that come with hash-sharded indexes such as index on low cardinality column, index on high percentage of NULLs and index on high percentage of particular value."
)

// SchemaOptimizationReport represents a comprehensive report of schema optimizations
// applied during the export process, including redundant index removal and Colocation recommendations.
type SchemaOptimizationReport struct {
	// Metadata about the export process
	VoyagerVersion        string `json:"voyager_version"`
	SourceDatabaseName    string `json:"source_database_name"`
	SourceDatabaseSchema  string `json:"source_database_schema"`
	SourceDatabaseVersion string `json:"source_database_version"`

	// Optimization changes applied
	RedundantIndexChange             *RedundantIndexChange             `json:"redundant_index_change,omitempty"`
	TableColocationRecommendation    *ColocationRecommendationChange   `json:"table_colocation_recommendation,omitempty"`
	MviewColocationRecommendation    *ColocationRecommendationChange   `json:"mview_colocation_recommendation,omitempty"`
	SecondaryIndexToRangeChange      *SecondaryIndexToRangeChange      `json:"secondary_index_to_range_change,omitempty"`
	PKHashShardingChange             *PKHashShardingChange             `json:"pk_hash_sharding_change,omitempty"`
	PKOnTimestampRangeShardingChange *PKOnTimestampRangeShardingChange `json:"pk_on_timestamp_range_sharding_change,omitempty"`
	UKRangeShardingChange            *UKRangeSplittingChange           `json:"uk_range_splitting_change,omitempty"`
}

// HasOptimizations returns true if any optimizations were applied
func (s *SchemaOptimizationReport) HasOptimizations() bool {
	return (s.RedundantIndexChange != nil && s.RedundantIndexChange.Exist()) ||
		(s.TableColocationRecommendation != nil && s.TableColocationRecommendation.Exist()) ||
		(s.MviewColocationRecommendation != nil && s.MviewColocationRecommendation.Exist()) ||
		(s.PKHashShardingChange != nil && s.PKHashShardingChange.Exist()) ||
		(s.PKOnTimestampRangeShardingChange != nil && s.PKOnTimestampRangeShardingChange.Exist()) ||
		(s.SecondaryIndexToRangeChange != nil && s.SecondaryIndexToRangeChange.Exist()) ||
		(s.UKRangeShardingChange != nil && s.UKRangeShardingChange.Exist())
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

func (s *SchemaOptimizationReport) Summary() string {
	//summary for all the changes whether applied or not
	summary := ""
	idx := 1
	if s.RedundantIndexChange != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.RedundantIndexChange.Summary())
		idx++
	}
	if s.TableColocationRecommendation != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.TableColocationRecommendation.Summary())
		idx++
	}
	if s.MviewColocationRecommendation != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.MviewColocationRecommendation.Summary())
		idx++
	}
	if s.SecondaryIndexToRangeChange != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.SecondaryIndexToRangeChange.Summary())
		idx++
	}
	if s.PKHashShardingChange != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.PKHashShardingChange.Summary())
		idx++
	}
	if s.PKOnTimestampRangeShardingChange != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.PKOnTimestampRangeShardingChange.Summary())
		idx++
	}
	if s.UKRangeShardingChange != nil {
		summary += fmt.Sprintf("%d. %s\n", idx, s.UKRangeShardingChange.Summary())
		idx++
	}
	return summary
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

// Exist returns true if no redundant indexes were removed
func (r *RedundantIndexChange) Exist() bool {
	return len(r.TableToRemovedIndexesMap) > 0
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

func (r *RedundantIndexChange) Summary() string {
	return modifyTitleColorBasedOnIsApplied(r.Title, r.IsApplied)
}

// ColocationRecommendationChange represents the application of Colocation recommendations
// to database objects (tables or materialized views) for improved performance.
type ColocationRecommendationChange struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	ReferenceFile     string   `json:"reference_file"`
	ShardedObjects    []string `json:"sharded_objects"`
	CollocatedObjects []string `json:"collocated_objects"`
	IsApplied         bool     `json:"is_applied"`
}

// Exist returns true if no sharded objects were present
func (a *ColocationRecommendationChange) Exist() bool {
	return len(a.ShardedObjects) > 0 || len(a.CollocatedObjects) > 0
}

// NewAppliedColocationRecommendationChange creates a new AppliedColocationRecommendationChange with default values
func NewAppliedColocationRecommendationChange(objectType string, applied bool, referenceFile string, shardedObjects []string, colocatedObjects []string) *ColocationRecommendationChange {
	var title, description string
	switch objectType {
	case TABLE:
		title = "Colocation Recommendations to Tables - Applied"
		description = "Colocation recommendations from the assessment have been applied to the tables to optimize data distribution and performance. Tables will be created as colocated automatically according to the target database configuration."
	case MVIEW:
		title = "Colocation Recommendations to Materialized Views - Applied"
		description = "Colocation recommendations from the assessment have been applied to the mviews to optimize data distribution and performance. MViews will be created as colocated automatically according to the target database configuration."
	default:
		title = "Colocation Recommendations - Applied"
		description = "Colocation recommendations from the assessment have been applied to optimize data distribution and performance."
	}

	if !applied {
		title = "Colocation Recommendations - Not Applied"
		description = APPLIED_RECOMMENDATIONS_NOT_APPLIED_DESCRIPTION
	}
	return &ColocationRecommendationChange{
		Title:             title,
		Description:       description,
		ReferenceFile:     referenceFile,
		ShardedObjects:    shardedObjects,
		CollocatedObjects: colocatedObjects,
		IsApplied:         applied,
	}
}

func (c *ColocationRecommendationChange) Summary() string {
	return modifyTitleColorBasedOnIsApplied(c.Title, c.IsApplied)
}

func NewNotAppliedColocationRecommendationChangeWhenAssessNotDoneDirectly() *ColocationRecommendationChange {
	return &ColocationRecommendationChange{
		Title:             "Colocation Recommendations - Not Applied",
		Description:       "Colocation recommendations were not applied since assessment was not run on the source database. Run the 'assess-migration' command explicitly to produce precise recommendations and re run the 'export schema' command to apply them.",
		ReferenceFile:     "",
		ShardedObjects:    nil,
		CollocatedObjects: nil,
		IsApplied:         false,
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
	description := "All the btree secondary indexes were configured to be range-sharded indexes in YugabyteDB."
	if !applied {
		title = "Secondary Indexes to be range-sharded - Not Applied"
		description = "Due to the skip-performance-recommendations flag, all the btree indexes were not converted to range-sharded indexes. Modify the indexes to be range-sharded manually."
	}
	description += "The range-sharded indexes helps in giving the flexibility to execute range-based queries, and avoids potential hotspots that come with hash-sharded indexes such as index on low cardinality column, index on high percentage of NULLs, and index on high percentage of particular value. Refer to sharding strategy in documentation for more information."
	return &SecondaryIndexToRangeChange{
		Title:       title,
		Description: description,
		HyperLinksInDescription: map[string]string{
			"index on low cardinality column":              "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-low-cardinality-column",
			"index on high percentage of NULLs":            "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-a-high-percentage-of-null-values",
			"index on high percentage of particular value": "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-high-percentage-of-a-particular-value",
			"documentation": "https://docs.yugabyte.com/preview/architecture/docdb-sharding/sharding/",
		},
		ReferenceFile:   referenceFile,
		ModifiedIndexes: modifiedIndexes,
		IsApplied:       applied,
	}
}

func (s *SecondaryIndexToRangeChange) Exist() bool {
	return len(s.ModifiedIndexes) > 0
}

func (s *SecondaryIndexToRangeChange) Summary() string {
	return modifyTitleColorBasedOnIsApplied(s.Title, s.IsApplied)
}

type PKHashShardingChange struct {
	Title                   string            `json:"title"`
	Description             string            `json:"description"`
	HyperLinksInDescription map[string]string `json:"hyper_links_in_description"`
	ModifiedTables          []string          `json:"modified_tables"`
	IsApplied               bool              `json:"is_applied"`
}

func NewPKHashShardingChange(applied bool, modifiedTables []string) *PKHashShardingChange {
	title := "Primary Key Constraints to be hash-sharded - Applied"
	description := "The Primary key constraints that are not on the timestamp or date types as first column were configured to be hash-sharded in YugabyteDB. This helps in giving randomize distribution of unique values of the Primary key across the nodes and helps in avoiding the hotspots that comes with the range-sharding for increasing nature of these values. Refer to sharding strategy in documentation for more information."
	if !applied {
		title = "Primary Key Constraints to be hash-sharded - Not Applied"
		description = "Due to the skip-performance-optimizations flag, the Primary key constraints that are not on the timestamp or date types as first column were not configured to be hash-sharded. Modify the Primary key constraints to be hash-sharded manually. The Primary key Constraints as hash-sharded helps in giving randomize distribution of unique values of the Primary key across the nodes and helps in avoiding the hotspots that comes with the range-sharding for increasing nature of these values. Refer to sharding strategy in documentation for more information. "
	}
	return &PKHashShardingChange{
		Title:       title,
		Description: description,
		IsApplied:   applied,
		HyperLinksInDescription: map[string]string{
			"documentation": "https://docs.yugabyte.com/preview/architecture/docdb-sharding/sharding/",
		},
		ModifiedTables: modifiedTables,
	}
}

func (p *PKHashShardingChange) Exist() bool {
	return true
}

func (p *PKHashShardingChange) Summary() string {
	return modifyTitleColorBasedOnIsApplied(p.Title, p.IsApplied)
}

type PKOnTimestampRangeShardingChange struct {
	Title                   string            `json:"title"`
	Description             string            `json:"description"`
	HyperLinksInDescription map[string]string `json:"hyper_links_in_description"`
	ModifiedTables          []string          `json:"modified_tables"`
	IsApplied               bool              `json:"is_applied"`
}

func NewPKOnTimestampRangeShardingChange(applied bool, modifiedTables []string) *PKOnTimestampRangeShardingChange {
	title := "Primary Key Constraints on the timestamp or date as first column to be range-sharded - Applied"
	description := "The Primary key constraints on the timestamp or date as first column were configured to be range-sharded in YugabyteDB."
	if !applied {
		title = "Primary Key Constraints on the timestamp or date as first column to be range-sharded - Not Applied"
		description = "Due to the skip-performance-optimizations flag, the Primary key constraints on the timestamp or date as first column were not configured to be range-sharded. Modify those Primary key constraints on to be range-sharded manually."
	}
	description += "The range-sharded indexes helps in giving the flexibility to execute range-based queries. Refer to sharding strategy in documentation for more information."
	return &PKOnTimestampRangeShardingChange{
		Title:       title,
		Description: description,
		IsApplied:   applied,
		HyperLinksInDescription: map[string]string{
			"documentation": "https://docs.yugabyte.com/preview/architecture/docdb-sharding/sharding/",
		},
		ModifiedTables: modifiedTables,
	}
}

func (p *PKOnTimestampRangeShardingChange) Exist() bool {
	return true
}

func (p *PKOnTimestampRangeShardingChange) Summary() string {
	return modifyTitleColorBasedOnIsApplied(p.Title, p.IsApplied)
}

type UKRangeSplittingChange struct {
	Title                   string            `json:"title"`
	Description             string            `json:"description"`
	HyperLinksInDescription map[string]string `json:"hyper_links_in_description"`
	IsApplied               bool              `json:"is_applied"`
}

func NewUKRangeSplittingChange(applied bool) *UKRangeSplittingChange {
	title := "Unique Key Constraints to be range-sharded - Applied"
	description := "All the unique key constraints were configured to be range-sharded in YugabyteDB."
	if !applied {
		title = "Unique Key Constraints to be range-sharded - Not Applied"
		description = "Due to the skip-performance-optimizations flag, all the unique key constraints were not configured to be range-sharded. Modify all the unique key constraints to be range-sharded manually."
	}
	description += "The range-sharded indexes helps in giving the flexibility to execute range-based queries, and avoids potential hotspot that comes with hash-sharded indexes such as index on high percentage of NULLs. Refer to sharding strategy in documentation for more information."
	return &UKRangeSplittingChange{
		Title:       title,
		Description: description,
		IsApplied:   applied,
		HyperLinksInDescription: map[string]string{
			"documentation":                     "https://docs.yugabyte.com/preview/architecture/docdb-sharding/sharding/",
			"index on high percentage of NULLs": "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#index-on-column-with-a-high-percentage-of-null-values",
		},
	}
}

func (p *UKRangeSplittingChange) Exist() bool {
	return p != nil
}

func (u *UKRangeSplittingChange) Summary() string {
	return modifyTitleColorBasedOnIsApplied(u.Title, u.IsApplied)
}

/*
TODO: currently we are using the multiple things to figure out the change is applied or not.
like configuration - skipRecommedations/skipPerfOptimizations etc...
but we should only rely on the transformer if  the particular optimization is applied or not
and those config parameters to be passed to transformer to apply the change or not
and schema optimization report should just be the reader of transformer for any such information
currenlty tehre are inconsitency where we don't make the colocation related change with  transformer its directly done in the export schema layer
will have to depend on something like AssessmentNotDoneDirectly flag for proper reporting

*/

func buildRedundantIndexChange(indexTransformer *sqltransformer.IndexFileTransformer) *RedundantIndexChange {
	if indexTransformer == nil {
		return nil
	}
	if len(indexTransformer.RedundantIndexesToExistingIndexToRemove.Keys()) == 0 {
		//Do not add redundant index change if no redundant indexes found
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

func buildColocationTableRecommendationChange(tableTransformer *sqltransformer.TableFileTransformer, isMigrationAssessmentDoneDirectly bool) *ColocationRecommendationChange {
	if tableTransformer == nil {
		return nil
	}
	if !tableTransformer.ColocationRecommendationsApplied { //If assessment recommendations not applied and skip recommendations is true, then show that its not applied
		if skipRecommendations {
			return NewAppliedColocationRecommendationChange("", false, "", nil, nil) // Dummy entry for both table and mview as no need to show two
		} else if !isMigrationAssessmentDoneDirectly {
			return NewNotAppliedColocationRecommendationChangeWhenAssessNotDoneDirectly() // Dummy entry for both table and mview as no need to show two
		}
		return nil
	}

	referenceTableFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), TABLE)
	//To tables then add that change
	if !utils.FileOrFolderExists(referenceTableFile) {
		return nil
	}

	return NewAppliedColocationRecommendationChange(TABLE, true, getRelativePathWithReportsDir(referenceTableFile), tableTransformer.ShardedTables, tableTransformer.ColocatedTables)
}

func buildColocationMviewRecommendationChange(mviewTransformer *sqltransformer.MviewFileTransformer) *ColocationRecommendationChange {
	if mviewTransformer == nil {
		return nil
	}
	if !mviewTransformer.ColocationRecommendationsApplied { //If assessment recommendations not applied, we are already addding a generic section for Colocation recommendations not applied with table case above
		return nil
	}
	referenceMviewFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), MVIEW)
	//To mviews then add that change separately
	if !utils.FileOrFolderExists(referenceMviewFile) {
		return nil
	}
	return NewAppliedColocationRecommendationChange(MVIEW, true, getRelativePathWithReportsDir(referenceMviewFile), mviewTransformer.ShardedMviews, mviewTransformer.ColocatedMviews)
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
// It reports the removal of redundant indexes and the application of Colocation recommendations to tables and materialized views (mviews).
// The report includes references to the relevant SQL files and lists the modified objects.
// Parameters:
//   - redundantIndexes: list of redundant index names that were removed.
//   - tables: list of table names to which Colocation recommendations were applied.
//   - mviews: list of materialized view names to which Colocation recommendations were applied.
func generatePerformanceOptimizationReport(indexTransformer *sqltransformer.IndexFileTransformer, tableTransformer *sqltransformer.TableFileTransformer, mviewTransformer *sqltransformer.MviewFileTransformer) error {

	if source.DBType != POSTGRESQL {
		//Not generating the report in case other than PG
		return nil
	}

	htmlReportFilePath := filepath.Join(exportDir, "reports", fmt.Sprintf("%s%s", SCHEMA_OPTIMIZATION_REPORT_FILE_NAME, HTML_EXTENSION))
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

	isMigrationAssessmentDoneDirectly, err := IsMigrationAssessmentDoneDirectly(metaDB)
	if err != nil {
		return fmt.Errorf("failed to check if migration assessment is done via export schema: %w", err)
	}

	schemaOptimizationReport.TableColocationRecommendation = buildColocationTableRecommendationChange(tableTransformer, isMigrationAssessmentDoneDirectly)
	schemaOptimizationReport.MviewColocationRecommendation = buildColocationMviewRecommendationChange(mviewTransformer)
	schemaOptimizationReport.SecondaryIndexToRangeChange = buildSecondaryIndexToRangeChange(indexTransformer)

	var shardingChangesApplied bool
	pkTablesOnTimestampOrDate, pkTablesWithHashSharded := []string{}, []string{}
	if tableTransformer != nil {
		shardingChangesApplied = tableTransformer.AppliedHashOrRangeShardingStrategyToConstraints
		pkTablesOnTimestampOrDate = tableTransformer.PKTablesOnTimestampWithRangeSharded
		pkTablesWithHashSharded = tableTransformer.PKTablesWithHashSharded
	}
	schemaOptimizationReport.PKHashShardingChange = NewPKHashShardingChange(shardingChangesApplied, pkTablesWithHashSharded)
	schemaOptimizationReport.PKOnTimestampRangeShardingChange = NewPKOnTimestampRangeShardingChange(shardingChangesApplied, pkTablesOnTimestampOrDate)
	schemaOptimizationReport.UKRangeShardingChange = NewUKRangeSplittingChange(shardingChangesApplied)

	if schemaOptimizationReport.HasOptimizations() {
		file, err := os.Create(htmlReportFilePath)
		if err != nil {
			return fmt.Errorf("failed to create file for %q: %w", filepath.Base(htmlReportFilePath), err)
		}

		jsonFilePath := filepath.Join(exportDir, "reports", fmt.Sprintf("%s.json", SCHEMA_OPTIMIZATION_REPORT_FILE_NAME))
		jsonFile := jsonfile.NewJsonFile[SchemaOptimizationReport](jsonFilePath)
		err = jsonFile.Create(schemaOptimizationReport)
		if err != nil {
			return fmt.Errorf("failed to create json report: %w", err)
		}

		err = tmpl.Execute(file, schemaOptimizationReport)
		if err != nil {
			return fmt.Errorf("failed to render the schema optimization report: %w", err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf("failed to close file %q: %w", htmlReportFilePath, err)
		}

		utils.PrintAndLogfInfo("\nSchema optimization changes\n\n")
		utils.PrintAndLog(schemaOptimizationReport.Summary())
		utils.PrintAndLogf("Refer to the detailed report for more information: %s\n", utils.Path.Sprintf(htmlReportFilePath))
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

func modifyTitleColorBasedOnIsApplied(title string, isApplied bool) string {
	splits := strings.Split(title, " - ")
	if len(splits) < 2 {
		return title
	}
	return fmt.Sprintf("%s - %s", splits[0], lo.Ternary(isApplied, utils.SuccessColor.Sprint(splits[1]), utils.ErrorColor.Sprint(splits[1])))
}
