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
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// =============================================== schema optimization changes report

// File names for optimization reports
const (
	RedundantIndexesFileName         = "redundant_indexes.sql"
	SchemaOptimizationReportFileName = "schema_optimization_report"
)

// RedundantIndexChange represents the removal of redundant indexes that are fully
// covered by stronger indexes, improving performance and reducing storage overhead.
type RedundantIndexChange struct {
	Title                    string              `json:"title"`
	Description              string              `json:"description"`
	ReferenceFile            string              `json:"reference_file"`
	ReferenceFileDisplayName string              `json:"reference_file_display_name"`
	TableToRemovedIndexesMap map[string][]string `json:"table_to_removed_indexes_map"`
}

// IsEmpty returns true if no redundant indexes were removed
func (r *RedundantIndexChange) IsEmpty() bool {
	return r == nil || len(r.TableToRemovedIndexesMap) == 0
}

// AppliedShardingRecommendationChange represents the application of sharding recommendations
// to database objects (tables or materialized views) for improved performance.
type AppliedShardingRecommendationChange struct {
	Title                    string   `json:"title"`
	Description              string   `json:"description"`
	ReferenceFile            string   `json:"reference_file"`
	ReferenceFileDisplayName string   `json:"reference_file_display_name"`
	ShardedObjects           []string `json:"sharded_objects"`
	CollocatedObjects        []string `json:"collocated_objects"`
}

// IsEmpty returns true if no sharding recommendations were applied
func (a *AppliedShardingRecommendationChange) IsEmpty() bool {
	return a == nil || (len(a.ShardedObjects) == 0)
}

// SchemaOptimizationReport represents a comprehensive report of schema optimizations
// applied during the export process, including redundant index removal and sharding recommendations.
type SchemaOptimizationReport struct {
	// Metadata about the export process
	VoyagerVersion        string `json:"voyager_version"`
	SourceDatabaseName    string `json:"source_database_name"`
	SourceDatabaseSchema  string `json:"source_database_schema"`
	SourceDatabaseVersion string `json:"source_database_version"`

	// Optimization changes applied
	RedundantIndexChange        *RedundantIndexChange                `json:"redundant_index_change,omitempty"`
	TableShardingRecommendation *AppliedShardingRecommendationChange `json:"table_sharding_recommendation,omitempty"`
	MviewShardingRecommendation *AppliedShardingRecommendationChange `json:"mview_sharding_recommendation,omitempty"`
}

// HasOptimizations returns true if any optimizations were applied
func (s *SchemaOptimizationReport) HasOptimizations() bool {
	return !s.RedundantIndexChange.IsEmpty() ||
		!s.TableShardingRecommendation.IsEmpty() ||
		!s.MviewShardingRecommendation.IsEmpty()
}

// NewRedundantIndexChange creates a new RedundantIndexChange with default values
func NewRedundantIndexChange() *RedundantIndexChange {
	return &RedundantIndexChange{
		Title:                    "Removed Redundant Indexes",
		Description:              "The following indexes were identified as redundant and removed. These indexes were fully covered by stronger indexes—indexes that share the same leading key columns (in order) and potentially include additional columns, making the redundant ones unnecessary.",
		TableToRemovedIndexesMap: make(map[string][]string),
	}
}

// NewAppliedShardingRecommendationChange creates a new AppliedShardingRecommendationChange with default values
func NewAppliedShardingRecommendationChange(objectType string) *AppliedShardingRecommendationChange {
	var title, description string
	switch objectType {
	case TABLE:
		title = "Applied Sharding Recommendations to Tables"
		description = "Sharding recommendations from the assessment have been applied to the tables to optimize data distribution and performance. Tables will be created as colocated automatically according to the target database configuration."
	case MVIEW:
		title = "Applied Sharding Recommendations to Materialized Views"
		description = "Sharding recommendations from the assessment have been applied to the mviews to optimize data distribution and performance. MViews will be created as colocated automatically according to the target database configuration."
	default:
		title = "Applied Sharding Recommendations"
		description = "Sharding recommendations from the assessment have been applied to optimize data distribution and performance."
	}

	return &AppliedShardingRecommendationChange{
		Title:             title,
		Description:       description,
		ShardedObjects:    make([]string, 0),
		CollocatedObjects: make([]string, 0),
	}
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

//go:embed templates/schema_optimization_report.template
var optimizationChangesTemplate []byte

// generatePerformanceOptimizationReport generates an HTML report detailing performance optimization changes applied to the exported schema.
// It reports the removal of redundant indexes and the application of sharding recommendations to tables and materialized views (mviews).
// The report includes references to the relevant SQL files and lists the modified objects.
// Parameters:
//   - redundantIndexes: list of redundant index names that were removed.
//   - tables: list of table names to which sharding recommendations were applied.
//   - mviews: list of materialized view names to which sharding recommendations were applied.
func generatePerformanceOptimizationReport(redundantIndexes []string, shardedTables []string, shardedMviews []string) error {

	if source.DBType != POSTGRESQL {
		//NOt generating the report in case other than PG
		return nil
	}

	var err error
	var redundantIndexChange *RedundantIndexChange
	if len(redundantIndexes) > 0 {
		redundantIndexChange = NewRedundantIndexChange()
		redundantIndexChange.ReferenceFile = filepath.Join(exportDir, "schema", "tables", RedundantIndexesFileName)
		redundantIndexChange.ReferenceFileDisplayName = RedundantIndexesFileName

		tableToIndexMap := make(map[string][]string)
		for _, index := range redundantIndexes {
			splits := strings.Split(index, " ON ")
			if len(splits) != 2 {
				log.Warnf("Redundant index is not in correct format (idx ON tbl) - %v", index)
				continue
			}
			indexName := splits[0]
			tableName := splits[1]
			tableToIndexMap[tableName] = append(tableToIndexMap[tableName], indexName)
		}
		redundantIndexChange.TableToRemovedIndexesMap = tableToIndexMap
	}

	var appliedRecommendationTable, appliedRecommendationMview *AppliedShardingRecommendationChange
	//If assessment recommendations are applied
	if assessmentRecommendationsApplied {
		tableFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), TABLE)
		//To tables then add that change
		if utils.FileOrFolderExists(tableFile) && len(shardedTables) > 0 { // only display this in case there is any modifield sharded tables
			appliedRecommendationTable = NewAppliedShardingRecommendationChange(TABLE)
			appliedRecommendationTable.ReferenceFile = tableFile
			appliedRecommendationTable.ReferenceFileDisplayName = filepath.Base(tableFile)
			appliedRecommendationTable.ShardedObjects = shardedTables
			appliedRecommendationTable.CollocatedObjects, err = getColocatedObjects(tableFile, shardedTables, TABLE)
			if err != nil {
				return fmt.Errorf("error getting other objects: %w", err)
			}
		}
		mviewFile := utils.GetObjectFilePath(filepath.Join(exportDir, "schema"), MVIEW)
		//To mviews then add that change separately
		if utils.FileOrFolderExists(mviewFile) && len(shardedMviews) > 0 { // only display this in case there is any modifield sharded mview
			appliedRecommendationMview = NewAppliedShardingRecommendationChange(MVIEW)
			appliedRecommendationMview.ReferenceFile = mviewFile
			appliedRecommendationMview.ReferenceFileDisplayName = filepath.Base(mviewFile)
			appliedRecommendationMview.ShardedObjects = shardedMviews
			appliedRecommendationMview.CollocatedObjects, err = getColocatedObjects(mviewFile, shardedMviews, MVIEW)
			if err != nil {
				return fmt.Errorf("error getting other objects: %w", err)
			}
		}
	}

	htmlReportFilePath := filepath.Join(exportDir, "reports", fmt.Sprintf("%s%s", SchemaOptimizationReportFileName, HTML_EXTENSION))
	log.Infof("writing changes report to file: %s", htmlReportFilePath)

	file, err := os.Create(htmlReportFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file for %q: %w", filepath.Base(htmlReportFilePath), err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Errorf("failed to close file %q: %v", htmlReportFilePath, err)
		}
	}()
	tmpl := template.Must(template.New("report").Parse(string(optimizationChangesTemplate)))
	report := NewSchemaOptimizationReport(
		utils.YB_VOYAGER_VERSION,
		source.DBName,
		strings.Join(strings.Split(source.Schema, "|"), ", "),
		source.DBVersion,
	)
	report.RedundantIndexChange = redundantIndexChange
	report.TableShardingRecommendation = appliedRecommendationTable
	report.MviewShardingRecommendation = appliedRecommendationMview

	if report.HasOptimizations() {
		err = tmpl.Execute(file, report)
		if err != nil {
			return fmt.Errorf("failed to render the assessment report: %w", err)
		}

		color.Green("\nSome Optimization changes were applied to the exported schema, refer to the detailed report for more information: %s", htmlReportFilePath)
	}

	return nil
}

func getColocatedObjects(objectFile string, shardedObjects []string, objType string) ([]string, error) {
	sqlInfoArr := parseSqlFileForObjectType(objectFile, objType)
	allObject := make([]string, 0)
	for _, sqlInfo := range sqlInfoArr {
		sql := sqlInfo.formattedStmt
		parseTree, err := queryparser.Parse(sql)
		if err != nil {
			return nil, fmt.Errorf("error parsing sql statement: %w", err)
		}
		objT, objectName := queryparser.GetObjectTypeAndObjectName(parseTree)
		if objT != objType {
			continue
		}
		allObject = append(allObject, objectName)
	}
	colocated, _ := lo.Difference(allObject, shardedObjects)
	return colocated, nil
}
