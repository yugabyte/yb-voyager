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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/compareperf"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	// skipCategoriesForAnonymization contains issue categories that should be skipped during SQL anonymization
	skipCategoriesForAnonymization = []string{
		UNSUPPORTED_QUERY_CONSTRUCTS_CATEGORY,
		UNSUPPORTED_PLPGSQL_OBJECTS_CATEGORY,
	}

	// skipObjectTypesForAnonymization contains object types that should be skipped during SQL anonymization
	skipObjectTypesForAnonymization = []string{
		constants.VIEW,
		constants.MATERIALIZED_VIEW,
		constants.TRIGGER,
		constants.FUNCTION,
		constants.PROCEDURE,
	}
)

func packAndSendAssessMigrationPayload(status string, errMsg error) {
	var err error
	if !shouldSendCallhome() {
		return
	}

	payload := createCallhomePayload()
	payload.MigrationPhase = ASSESS_MIGRATION_PHASE
	payload.Status = status
	if assessmentMetadataDirFlag == "" {
		sourceDBDetails := anonymizeSourceDBDetails(&source)
		payload.SourceDBDetails = callhome.MarshalledJsonString(sourceDBDetails)
	}

	var tableSizingStats, indexSizingStats []callhome.ObjectSizingStats
	if assessmentReport.TableIndexStats != nil {
		for _, stat := range *assessmentReport.TableIndexStats {
			newStat := callhome.ObjectSizingStats{
				ReadsPerSecond:  utils.SafeDereferenceInt64(stat.ReadsPerSecond),
				WritesPerSecond: utils.SafeDereferenceInt64(stat.WritesPerSecond),
				SizeInBytes:     utils.SafeDereferenceInt64(stat.SizeInBytes),
			}

			// Anonymizing schema and object names
			sname, err := anonymizer.AnonymizeSchemaName(stat.SchemaName)
			if err != nil {
				log.Warnf("failed to anonymize schema name %s: %v", stat.SchemaName, err)
				newStat.SchemaName = constants.OBFUSCATE_STRING
			} else {
				newStat.SchemaName = sname
			}

			if stat.IsIndex {
				iName, err := anonymizer.AnonymizeIndexName(stat.ObjectName)
				if err != nil {
					log.Warnf("failed to anonymize index name %s: %v", stat.ObjectName, err)
					newStat.ObjectName = constants.OBFUSCATE_STRING
				} else {
					newStat.ObjectName = iName
				}
				indexSizingStats = append(indexSizingStats, newStat)
			} else {
				tName, err := anonymizer.AnonymizeTableName(stat.ObjectName)
				if err != nil {
					log.Warnf("failed to anonymize table name %s: %v", stat.ObjectName, err)
					newStat.ObjectName = constants.OBFUSCATE_STRING
				} else {
					newStat.ObjectName = tName
				}
				tableSizingStats = append(tableSizingStats, newStat)
			}
		}
	}
	schemaSummaryCopy := utils.SchemaSummary{
		Notes: assessmentReport.SchemaSummary.Notes,
		DBObjects: lo.Map(schemaAnalysisReport.SchemaSummary.DBObjects, func(dbObject utils.DBObject, _ int) utils.DBObject {
			dbObject.ObjectNames = ""
			dbObject.Details = "" // not useful, either static or sometimes sensitive(oracle indexes) information
			return dbObject
		}),
	}

	anonymizedIssues := anonymizeAssessmentIssuesForCallhomePayload(assessmentReport.Issues)

	var callhomeSizingAssessment callhome.SizingCallhome
	if assessmentReport.Sizing != nil {
		sizingRecommedation := &assessmentReport.Sizing.SizingRecommendation
		callhomeSizingAssessment = callhome.SizingCallhome{
			ColocatedTables:                 anonymizeQualifiedTableNames(sizingRecommedation.ColocatedTables),
			ColocatedReasoning:              sizingRecommedation.ColocatedReasoning,
			ShardedTables:                   anonymizeQualifiedTableNames(sizingRecommedation.ShardedTables),
			NumNodes:                        sizingRecommedation.NumNodes,
			VCPUsPerInstance:                sizingRecommedation.VCPUsPerInstance,
			MemoryPerInstance:               sizingRecommedation.MemoryPerInstance,
			OptimalSelectConnectionsPerNode: sizingRecommedation.OptimalSelectConnectionsPerNode,
			OptimalInsertConnectionsPerNode: sizingRecommedation.OptimalInsertConnectionsPerNode,
			EstimatedTimeInMinForImport:     sizingRecommedation.EstimatedTimeInMinForImport,
			EstimatedTimeInMinForImportWithoutRedundantIndexes: sizingRecommedation.EstimatedTimeInMinForImportWithoutRedundantIndexes,
		}
	}

	assessPayload := callhome.AssessMigrationPhasePayload{
		PayloadVersion:                 callhome.ASSESS_MIGRATION_CALLHOME_PAYLOAD_VERSION,
		TargetDBVersion:                assessmentReport.TargetDBVersion,
		Sizing:                         &callhomeSizingAssessment,
		MigrationComplexity:            assessmentReport.MigrationComplexity,
		MigrationComplexityExplanation: assessmentReport.MigrationComplexityExplanation,
		SchemaSummary:                  callhome.MarshalledJsonString(schemaSummaryCopy),
		Issues:                         anonymizedIssues,
		Error:                          callhome.SanitizeErrorMsg(errMsg, anonymizer),
		TableSizingStats:               callhome.MarshalledJsonString(tableSizingStats),
		IndexSizingStats:               callhome.MarshalledJsonString(indexSizingStats),
		SourceConnectivity:             assessmentMetadataDirFlag == "",
		IopsInterval:                   intervalForCapturingIOPS,
		ControlPlaneType:               getControlPlaneType(),
		AnonymizedDDLs:                 getAnonymizedDDLs(&source),
	}

	payload.PhasePayload = callhome.MarshalledJsonString(assessPayload)
	err = callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func anonymizeQualifiedTableNames(tableNames []string) []string {
	return lo.Map(tableNames, func(tableName string, _ int) string {
		anonymizedName, err := anonymizer.AnonymizeQualifiedTableName(tableName)
		if err != nil {
			log.Warnf("failed to anonymize table name %s: %v", tableName, err)
			return constants.OBFUSCATE_STRING
		}
		return anonymizedName
	})
}

// anonymizeSourceDBDetails creates anonymized source DB details for callhome
func anonymizeSourceDBDetails(source *srcdb.Source) callhome.SourceDBDetails {
	details := callhome.SourceDBDetails{
		DBType:             source.DBType,
		DBVersion:          source.DBVersion,
		DBSize:             source.DBSize,
		DBSystemIdentifier: source.DBSystemIdentifier,
	}

	// Anonymize database name
	if source.DBName != "" {
		anonymizedDBName, err := anonymizer.AnonymizeDatabaseName(source.DBName)
		if err != nil {
			log.Warnf("failed to anonymize database name %s: %v", source.DBName, err)
			details.DBName = constants.OBFUSCATE_STRING
		} else {
			details.DBName = anonymizedDBName
		}
	}

	// Anonymize schema names
	if source.Schema != "" {
		schemaList := source.GetSchemaList()
		anonymizedSchemas := make([]string, 0, len(schemaList))
		for _, schemaName := range schemaList {
			if schemaName == "" {
				continue
			}
			anonymizedSchema, err := anonymizer.AnonymizeSchemaName(schemaName)
			if err != nil {
				log.Warnf("failed to anonymize schema name %s: %v", schemaName, err)
				anonymizedSchemas = append(anonymizedSchemas, constants.OBFUSCATE_STRING)
			} else {
				anonymizedSchemas = append(anonymizedSchemas, anonymizedSchema)
			}
		}
		details.SchemaNames = anonymizedSchemas
	}

	return details
}

// ============================assess migration callhome payload information============================

func anonymizeAssessmentIssuesForCallhomePayload(assessmentIssues []AssessmentIssue) []callhome.AssessmentIssueCallhome {
	/*
		Case to skip for sql statement anonymization:
			1. if issue type is unsupported query construct or unsupported plpgsql object
			2. if object type is view or materialized view
	*/
	shouldSkipAnonymization := func(issue AssessmentIssue) bool {
		return slices.Contains(skipCategoriesForAnonymization, issue.Category) ||
			slices.Contains(skipObjectTypesForAnonymization, issue.ObjectType)
	}

	var err error
	anonymizedIssues := make([]callhome.AssessmentIssueCallhome, len(assessmentIssues))
	for i, issue := range assessmentIssues {
		anonymizedIssues[i] = callhome.NewAssessmentIssueCallhome(issue.Category, issue.CategoryDescription, issue.Type, issue.Name, issue.Impact, issue.ObjectType, issue.Details)

		if shouldSkipAnonymization(issue) {
			continue
		}

		// special handling for extensions issue: adding extname to issue.Name
		if issue.Type == queryissue.UNSUPPORTED_EXTENSION {
			anonymizedIssues[i].Name = queryissue.AppendObjectNameToIssueName(issue.Name, issue.ObjectName)
		}

		if issue.Category == UNSUPPORTED_DATATYPES_CATEGORY {
			anonymizedIssues[i].ObjectName, err = anonymizer.AnonymizeQualifiedColumnName(issue.ObjectName)
			if err != nil {
				log.Warnf("failed to anonymize object name %s: %v", issue.ObjectName, err)
				anonymizedIssues[i].ObjectName = constants.OBFUSCATE_STRING
			}
		}

		if issue.SqlStatement != "" {
			var err error
			anonymizedIssues[i].SqlStatement, err = anonymizer.AnonymizeSql(issue.SqlStatement)
			if err != nil {
				anonymizedIssues[i].SqlStatement = "" // set to empty string to avoid sending the sql statement (safety net)
				log.Warnf("failed to anonymize sql statement for issue %s: %v", issue.Name, err)
			}
		}

		// Anonymize details map
		if issue.Details != nil {
			anonymizedIssues[i].Details = anonymizeIssueDetailsForCallhome(anonymizedIssues[i].Details)
		}
	}

	return anonymizedIssues
}

// anonymizeIssueDetailsForCallhome anonymizes sensitive information in the details map for callhome payload
// Currently handles FK_COLUMN_NAMES, can be extended for other sensitive keys
func anonymizeIssueDetailsForCallhome(details map[string]interface{}) map[string]interface{} {
	if details == nil {
		return nil
	}

	anonymizedDetails := make(map[string]interface{})

	for key, value := range details {
		if key == queryissue.FK_COLUMN_NAMES {
			if strValue, ok := value.(string); ok && strValue != "" {
				// Split the comma-separated column names
				columnNames := strings.Split(strValue, ", ") // TODO: We can probably store column names as a list of strings in the issue details map
				var anonymizedColumns []string

				for _, columnName := range columnNames {
					columnName = strings.TrimSpace(columnName)
					if columnName != "" {
						anonymizedColumn, err := anonymizer.AnonymizeQualifiedColumnName(columnName)
						if err != nil {
							log.Warnf("failed to anonymize FK column name %s: %v", columnName, err)
							anonymizedColumns = append(anonymizedColumns, "column_xxx")
						} else {
							anonymizedColumns = append(anonymizedColumns, anonymizedColumn)
						}
					}
				}

				// Join the anonymized column names back with comma
				anonymizedDetails[key] = strings.Join(anonymizedColumns, ", ")
			} else {
				anonymizedDetails[key] = value
			}
		} else if key == "PrimaryKeyColumnOptions" {
			if pkOptions, ok := value.([][]string); ok {
				var anonymizedOptions [][]string
				for _, option := range pkOptions {
					var anonymizedOption []string
					for _, columnName := range option {
						anonymizedColumn, err := anonymizer.AnonymizeQualifiedColumnName(columnName)
						if err != nil {
							log.Warnf("failed to anonymize PK column name %s: %v", columnName, err)
							anonymizedOption = append(anonymizedOption, "column_xxx")
						} else {
							anonymizedOption = append(anonymizedOption, anonymizedColumn)
						}
					}
					anonymizedOptions = append(anonymizedOptions, anonymizedOption)
				}
				anonymizedDetails[key] = anonymizedOptions
			} else {
				anonymizedDetails[key] = value
			}
		} else {
			// Non-sensitive keys are kept as-is
			anonymizedDetails[key] = value
		}
	}

	return anonymizedDetails
}

var DDL_ANONYMIZATION_FEATURE_ENABLED = true

func getAnonymizedDDLs(sourceDBConf *srcdb.Source) []string {
	// env var to enable sending anonymized DDLs to call home
	// Note: enabled by default
	if !utils.GetEnvAsBool("SEND_ANONYMIZED_DDLS", DDL_ANONYMIZATION_FEATURE_ENABLED) {
		log.Infof("DDL anonymization feature is disabled")
		return []string{}
	}

	if sourceDBConf.DBType != constants.POSTGRESQL {
		return []string{}
	}

	collectedDDLs := collectAllDDLs(sourceDBConf, skipObjectTypesForAnonymization)
	if len(collectedDDLs) == 0 {
		return []string{}
	}

	var anonymizedDDLs []string
	for _, ddl := range collectedDDLs {
		anonymizedDDL, err := anonymizer.AnonymizeSql(ddl)
		if err != nil {
			// log the error and continue with the next DDL
			log.Warnf("failed to anonymize DDL: %v", err)
			continue
		}
		anonymizedDDLs = append(anonymizedDDLs, anonymizedDDL)
	}

	// for testing store the generated list in a file in exportDir
	// use a env var to enable this
	if utils.GetEnvAsBool("STORE_ANONYMIZED_DDLS", false) {
		filePath := filepath.Join(exportDir, "anonymized_ddls.txt")
		err := os.WriteFile(filePath, []byte(strings.Join(anonymizedDDLs, "\n")), 0644)
		if err != nil {
			log.Warnf("failed to store anonymized DDLs in file %s: %v", filePath, err)
		}
	}

	return anonymizedDDLs
}

// ============================export schema callhome payload information============================

const (
	REDUNDANT_INDEX_CHANGE_TYPE                 = "redundant_index"
	TABLE_COLOCATION_RECOMMENDATION_CHANGE_TYPE = "table_sharding_recommendation"
	MVIEW_COLOCATION_RECOMMENDATION_CHANGE_TYPE = "mview_sharding_recommendation"
	SECONDARY_INDEX_TO_RANGE_CHANGE_TYPE        = "secondary_index_to_range"
)

func packAndSendExportSchemaPayload(status string, errorMsg error) {
	if !shouldSendCallhome() {
		return
	}
	payload := createCallhomePayload()
	payload.MigrationPhase = EXPORT_SCHEMA_PHASE
	payload.Status = status
	sourceDBDetails := anonymizeSourceDBDetails(&source)
	schemaOptimizationChanges := buildCallhomeSchemaOptimizationChanges()

	payload.SourceDBDetails = callhome.MarshalledJsonString(sourceDBDetails)
	assessRunInExportSchema, err := IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		log.Infof("callhome: failed to get migration assessment done via export schema: %v", err)
	}
	exportSchemaPayload := callhome.ExportSchemaPhasePayload{
		StartClean:                bool(startClean),
		AppliedRecommendations:    assessmentRecommendationsApplied,
		UseOrafce:                 bool(source.UseOrafce),
		CommentsOnObjects:         bool(source.CommentsOnObjects),
		Error:                     callhome.SanitizeErrorMsg(errorMsg, anonymizer),
		SkipRecommendations:       bool(skipRecommendations),
		AssessRunInExportSchema:   assessRunInExportSchema,
		SkipPerfOptimizations:     bool(skipPerfOptimizations),
		ControlPlaneType:          getControlPlaneType(),
		SchemaOptimizationChanges: schemaOptimizationChanges,
	}

	payload.PhasePayload = callhome.MarshalledJsonString(exportSchemaPayload)

	err = callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func buildCallhomeSchemaOptimizationChanges() []callhome.SchemaOptimizationChange {
	if schemaOptimizationReport == nil {
		return nil
	}
	//For individual change, adding the anonymized object names to the callhome payload
	schemaOptimizationChanges := make([]callhome.SchemaOptimizationChange, 0)
	if schemaOptimizationReport.RedundantIndexChange != nil {
		schemaOptimizationChanges = append(schemaOptimizationChanges, callhome.SchemaOptimizationChange{
			OptimizationType: REDUNDANT_INDEX_CHANGE_TYPE,
			IsApplied:        schemaOptimizationReport.RedundantIndexChange.IsApplied,
			Objects:          getAnonymizedIndexObjectsFromIndexToTableMap(schemaOptimizationReport.RedundantIndexChange.TableToRemovedIndexesMap),
		})
	}
	if schemaOptimizationReport.TableColocationRecommendation != nil {
		objects := make([]string, 0)
		for _, obj := range schemaOptimizationReport.TableColocationRecommendation.ShardedObjects {
			anonymizedObj, err := anonymizer.AnonymizeQualifiedTableName(obj)
			if err != nil {
				log.Errorf("callhome: failed to anonymise table-%s: %v", obj, err)
				continue
			}
			objects = append(objects, anonymizedObj)
		}
		schemaOptimizationChanges = append(schemaOptimizationChanges, callhome.SchemaOptimizationChange{
			OptimizationType: TABLE_COLOCATION_RECOMMENDATION_CHANGE_TYPE,
			IsApplied:        schemaOptimizationReport.TableColocationRecommendation.IsApplied,
			Objects:          objects,
		})
	}
	if schemaOptimizationReport.MviewColocationRecommendation != nil {
		objects := make([]string, 0)
		for _, obj := range schemaOptimizationReport.MviewColocationRecommendation.ShardedObjects {
			anonymizedObj, err := anonymizer.AnonymizeQualifiedMViewName(obj)
			if err != nil {
				log.Errorf("callhome: failed to anonymise mview-%s: %v", obj, err)
				continue
			}
			objects = append(objects, anonymizedObj)
		}
		schemaOptimizationChanges = append(schemaOptimizationChanges, callhome.SchemaOptimizationChange{
			OptimizationType: MVIEW_COLOCATION_RECOMMENDATION_CHANGE_TYPE,
			IsApplied:        schemaOptimizationReport.MviewColocationRecommendation.IsApplied,
			Objects:          schemaOptimizationReport.MviewColocationRecommendation.ShardedObjects,
		})
	}
	if schemaOptimizationReport.SecondaryIndexToRangeChange != nil {
		schemaOptimizationChanges = append(schemaOptimizationChanges, callhome.SchemaOptimizationChange{
			OptimizationType: SECONDARY_INDEX_TO_RANGE_CHANGE_TYPE,
			IsApplied:        schemaOptimizationReport.SecondaryIndexToRangeChange.IsApplied,
			Objects:          getAnonymizedIndexObjectsFromIndexToTableMap(schemaOptimizationReport.SecondaryIndexToRangeChange.ModifiedIndexes),
		})
	}
	return schemaOptimizationChanges
}

func getAnonymizedIndexObjectsFromIndexToTableMap(indexToTableMap map[string][]string) []string {
	objects := make([]string, 0)
	for tbl, indexes := range indexToTableMap {
		for _, index := range indexes {
			anonymizedInd, err := anonymizer.AnonymizeIndexName(index)
			if err != nil {
				log.Errorf("callhome: failed to anonymise index-%s: %v", index, err)
				continue
			}
			anonymizedTbl, err := anonymizer.AnonymizeQualifiedTableName(tbl)
			if err != nil {
				log.Errorf("callhome: failed to anonymise table-%s: %v", tbl, err)
				continue
			}
			objects = append(objects, fmt.Sprintf("%s ON %s", anonymizedInd, anonymizedTbl))
		}
	}
	return objects
}

// ============================compare performance callhome payload information============================

func packAndSendComparePerformancePayload(status string, errorMsg error, comparator *compareperf.QueryPerformanceComparator) {
	if !shouldSendCallhome() {
		return
	}

	payload := createCallhomePayload()
	payload.MigrationPhase = COMPARE_PERFORMANCE_PHASE
	payload.Status = status
	payload.TargetDBDetails = callhome.MarshalledJsonString(targetDBDetails)

	comparePerformancePayload := buildCallhomeComparePerformancePayload(comparator, errorMsg)
	payload.PhasePayload = callhome.MarshalledJsonString(comparePerformancePayload)
	err := callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

// buildCallhomeComparePerformancePayload builds the payload for compare performance callhome
// Collects performance comparison metrics from the comparator for matched queries only
func buildCallhomeComparePerformancePayload(comparator *compareperf.QueryPerformanceComparator, errorMsg error) *callhome.ComparePerformancePhasePayload {
	payload := callhome.ComparePerformancePhasePayload{
		PayloadVersion: callhome.COMPARE_PERFORMANCE_PAYLOAD_VERSION,
		Error:          callhome.SanitizeErrorMsg(errorMsg, anonymizer),
	}

	if comparator == nil || comparator.Report == nil {
		return &payload
	}

	var queryMetrics []callhome.QueryMetric
	for _, comparison := range comparator.Report.AllComparisons {
		if comparison.MatchStatus != compareperf.MATCHED {
			continue
		}
		// not expected though since they are matched queries but just to be safe
		if comparison.SourceStats == nil || comparison.TargetStats == nil {
			continue
		}

		queryMetric := callhome.QueryMetric{
			QueryLabel:    generateQueryLabel(comparison.Query),
			SlowdownRatio: comparison.SlowdownRatio,
			ImpactScore:   comparison.ImpactScore,
			SourceStats: callhome.QueryStatsCallhome{
				ExecutionCount:  comparison.SourceStats.ExecutionCount,
				RowsProcessed:   comparison.SourceStats.RowsProcessed,
				TotalExecTime:   comparison.SourceStats.TotalExecTime,
				AverageExecTime: comparison.SourceStats.AverageExecTime,
				MinExecTime:     comparison.SourceStats.MinExecTime,
				MaxExecTime:     comparison.SourceStats.MaxExecTime,
			},
			TargetStats: callhome.QueryStatsCallhome{
				ExecutionCount:  comparison.TargetStats.ExecutionCount,
				RowsProcessed:   comparison.TargetStats.RowsProcessed,
				TotalExecTime:   comparison.TargetStats.TotalExecTime,
				AverageExecTime: comparison.TargetStats.AverageExecTime,
				MinExecTime:     comparison.TargetStats.MinExecTime,
				MaxExecTime:     comparison.TargetStats.MaxExecTime,
			},
		}
		queryMetrics = append(queryMetrics, queryMetric)
	}

	// Populate summary and metrics
	payload.TotalQueries = comparator.Report.Summary.TotalQueries
	payload.MatchedQueries = comparator.Report.Summary.MatchedQueries
	payload.SourceOnlyQueries = comparator.Report.Summary.SourceOnlyQueries
	payload.TargetOnlyQueries = comparator.Report.Summary.TargetOnlyQueries
	payload.QueryMetrics = queryMetrics
	return &payload
}

// generateQueryLabel creates a simple privacy-safe label for a query to send to callhome
// label is combination of
// 1. first part is first word of the query in uppercase like SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER etc.
// 2. second part denotes the number of words in the query like SIMPLE(0-100), MEDIUM(100-500), COMPLEX(500+)
func generateQueryLabel(queryText string) string {
	// empty query is not expected but good to have for observability in callhome
	if len(strings.TrimSpace(queryText)) == 0 {
		return "EMPTY_QUERY"
	}

	words := strings.Fields(queryText)
	if len(words) == 0 {
		return "EMPTY_QUERY"
	}

	var firstPart, secondPart string
	firstPart = strings.ToUpper(words[0])
	queryLen := len(queryText)
	switch {
	case queryLen >= 500:
		secondPart = "COMPLEX"
	case queryLen >= 100:
		secondPart = "MEDIUM"
	default:
		secondPart = "SIMPLE"
	}
	return firstPart + "_" + secondPart
}
