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
	"encoding/json"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

/*
YUGABYTED DATA FLOW - Migration Assessment Completed Event:

1. createMigrationAssessmentCompletedEventForYugabyteD() (here)
   - Creates AssessMigrationPayloadYugabyteD struct with all assessment data
   - MARSHALS struct to JSON string: json.Marshal(payload) -> string
   - Sets ev.Report = payloadStr (type: string)

2. MigrationAssessmentCompletedEvent.Report flows to yugabyted.go
   - Report field type: string (already marshaled JSON)

3. yugabyted.MigrationAssessmentCompleted() in src/cp/yugabyted/yugabyted.go
   - Receives ev.Report as JSON string
   - Directly inserts the JSON string into SQL database
   - NO unmarshaling - database stores it as TEXT/JSON column

Result: Single marshal (struct -> JSON string), no unmarshal
*/

func createMigrationAssessmentCompletedEventForYugabyteD() *cp.MigrationAssessmentCompletedEvent {
	ev := &cp.MigrationAssessmentCompletedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")

	totalColocatedSize, err := assessmentReport.GetTotalColocatedSize(source.DBType)
	if err != nil {
		utils.PrintAndLogf("failed to calculate the total colocated table size from tableIndexStats: %v", err)
	}

	totalShardedSize, err := assessmentReport.GetTotalShardedSize(source.DBType)
	if err != nil {
		utils.PrintAndLogf("failed to calculate the total sharded table size from tableIndexStats: %v", err)
	}

	assessmentIssues := convertAssessmentIssueToYugabyteDAssessmentIssue(assessmentReport)

	allNotesText := lo.Map(assessmentReport.Notes, func(note NoteInfo, _ int) string {
		return note.Text
	})

	payload := AssessMigrationPayloadYugabyteD{
		PayloadVersion:                 ASSESS_MIGRATION_YBD_PAYLOAD_VERSION,
		VoyagerVersion:                 assessmentReport.VoyagerVersion,
		TargetDBVersion:                assessmentReport.TargetDBVersion,
		MigrationComplexity:            assessmentReport.MigrationComplexity,
		MigrationComplexityExplanation: assessmentReport.MigrationComplexityExplanation,
		SchemaSummary:                  assessmentReport.SchemaSummary,
		AssessmentIssues:               assessmentIssues,
		SourceSizeDetails: SourceDBSizeDetails{
			TotalIndexSize:     assessmentReport.GetTotalIndexSize(),
			TotalTableSize:     assessmentReport.GetTotalTableSize(),
			TotalTableRowCount: assessmentReport.GetTotalTableRowCount(),
			TotalDBSize:        source.DBSize,
		},
		TargetRecommendations: TargetSizingRecommendations{
			TotalColocatedSize: totalColocatedSize,
			TotalShardedSize:   totalShardedSize,
		},
		ConversionIssues: schemaAnalysisReport.Issues,
		Sizing:           assessmentReport.Sizing,
		TableIndexStats:  assessmentReport.TableIndexStats,

		Notes: allNotesText, // for backward compatibility
		AssessmentJsonReport: AssessmentReportYugabyteD{ // for backward compatibility
			VoyagerVersion:             assessmentReport.VoyagerVersion,
			TargetDBVersion:            assessmentReport.TargetDBVersion,
			MigrationComplexity:        assessmentReport.MigrationComplexity,
			SchemaSummary:              assessmentReport.SchemaSummary,
			Sizing:                     assessmentReport.Sizing,
			TableIndexStats:            assessmentReport.TableIndexStats,
			Notes:                      allNotesText,
			UnsupportedDataTypes:       assessmentReport.UnsupportedDataTypes,
			UnsupportedDataTypesDesc:   assessmentReport.UnsupportedDataTypesDesc,
			UnsupportedFeatures:        assessmentReport.UnsupportedFeatures,
			UnsupportedFeaturesDesc:    assessmentReport.UnsupportedFeaturesDesc,
			UnsupportedQueryConstructs: assessmentReport.UnsupportedQueryConstructs,
			UnsupportedPlPgSqlObjects:  assessmentReport.UnsupportedPlPgSqlObjects,
			MigrationCaveats:           assessmentReport.MigrationCaveats,
		},
	}

	// classify notes into GeneralNotes, ColocatedShardedNotes, SizingNotes
	for _, note := range assessmentReport.Notes {
		switch note.Type {
		case GeneralNotes:
			payload.GeneralNotes = append(payload.GeneralNotes, note.Text)
		case ColocatedShardedNotes:
			payload.ColocatedShardedNotes = append(payload.ColocatedShardedNotes, note.Text)
		case SizingNotes:
			payload.SizingNotes = append(payload.SizingNotes, note.Text)
		}
	}

	// Embed the raw AssessmentReport JSON so it can be round-tripped through the control plane
	rawReportBytes, err := json.Marshal(assessmentReport)
	if err != nil {
		utils.PrintAndLogf("Failed to marshal raw assessment report for control plane payload: %s", err)
	} else {
		payload.RawAssessmentJsonReport = string(rawReportBytes)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		utils.PrintAndLogf("Failed to serialise the final report to json (ERR IGNORED): %s", err)
	}

	ev.Report = string(payloadBytes)
	log.Infof("assess migration payload send to yugabyted: %s", ev.Report)
	return ev
}
