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
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
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
		sourceDBDetails := callhome.SourceDBDetails{
			DBType:    source.DBType,
			DBVersion: source.DBVersion,
			DBSize:    source.DBSize,
		}
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
			NumColocatedTables:              len(sizingRecommedation.ColocatedTables),
			ColocatedReasoning:              sizingRecommedation.ColocatedReasoning,
			NumShardedTables:                len(sizingRecommedation.ShardedTables),
			NumNodes:                        sizingRecommedation.NumNodes,
			VCPUsPerInstance:                sizingRecommedation.VCPUsPerInstance,
			MemoryPerInstance:               sizingRecommedation.MemoryPerInstance,
			OptimalSelectConnectionsPerNode: sizingRecommedation.OptimalSelectConnectionsPerNode,
			OptimalInsertConnectionsPerNode: sizingRecommedation.OptimalInsertConnectionsPerNode,
			EstimatedTimeInMinForImport:     sizingRecommedation.EstimatedTimeInMinForImport,
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
		Error:                          callhome.SanitizeErrorMsg(errMsg),
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

func anonymizeAssessmentIssuesForCallhomePayload(assessmentIssues []AssessmentIssue) []callhome.AssessmentIssueCallhome {
	/*
		Case to skip for sql statement anonymization:
			1. if issue type is unsupported query construct or unsupported plpgsql object
			2. if object type is view or materialized view
			3. if sql statement is empty
	*/
	shouldSkipAnonymization := func(issue AssessmentIssue) bool {
		skipCategories := []string{UNSUPPORTED_QUERY_CONSTRUCTS_CATEGORY, UNSUPPORTED_PLPGSQL_OBJECTS_CATEGORY, UNSUPPORTED_DATATYPES_CATEGORY}
		skipObjects := []string{constants.VIEW, constants.MATERIALIZED_VIEW, constants.TRIGGER, constants.FUNCTION, constants.PROCEDURE}

		return slices.Contains(skipCategories, issue.Category) ||
			slices.Contains(skipObjects, issue.ObjectType) ||
			issue.SqlStatement == ""
	}

	anonymizedIssues := make([]callhome.AssessmentIssueCallhome, len(assessmentIssues))
	for i, issue := range assessmentIssues {
		anonymizedIssues[i] = callhome.NewAssessmentIssueCallhome(issue.Category, issue.CategoryDescription, issue.Type, issue.Name, issue.Impact, issue.ObjectType, issue.Details)

		// special handling for extensions issue: adding extname to issue.Name
		if issue.Type == queryissue.UNSUPPORTED_EXTENSION {
			anonymizedIssues[i].Name = queryissue.AppendObjectNameToIssueName(issue.Name, issue.ObjectName)
		}

		if shouldSkipAnonymization(issue) {
			continue
		}

		var err error
		anonymizedIssues[i].SqlStatement, err = anonymizer.AnonymizeSql(issue.SqlStatement)
		if err != nil {
			anonymizedIssues[i].SqlStatement = "" // set to empty string to avoid sending the sql statement (safety net)
			log.Warnf("failed to anonymize sql statement for issue %s: %v", issue.Name, err)
		}
	}

	return anonymizedIssues
}

func getAnonymizedDDLs(sourceDBConf *srcdb.Source) []string {
	// env var to enable sending anonymized DDLs to call home
	if !utils.GetEnvAsBool("SEND_ANONYMIZED_DDLS", false) {
		return []string{}
	}

	if sourceDBConf.DBType != constants.POSTGRESQL {
		return []string{}
	}

	skipObjectTypeList := []string{"PROCEDURE", "FUNCTION", "TRIGGER", "VIEW", "MVIEW"}
	collectedDDLs := collectAllDDLs(sourceDBConf, skipObjectTypeList)
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
