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

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func createMigrationAssessmentStartedEvent() *cp.MigrationAssessmentStartedEvent {
	ev := &cp.MigrationAssessmentStartedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")
	return ev
}

// Enum definitions for different types of notes in the YugabyteD event payload
type NoteType int

const (
	GeneralNotes          NoteType = iota // 0
	ColocatedShardedNotes                 // 1
	SizingNotes                           // 2
)

func createMigrationAssessmentCompletedEvent() *cp.MigrationAssessmentCompletedEvent {
	ev := &cp.MigrationAssessmentCompletedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")

	totalColocatedSize, err := assessmentReport.GetTotalColocatedSize(source.DBType)
	if err != nil {
		utils.PrintAndLog("failed to calculate the total colocated table size from tableIndexStats: %v", err)
	}

	totalShardedSize, err := assessmentReport.GetTotalShardedSize(source.DBType)
	if err != nil {
		utils.PrintAndLog("failed to calculate the total sharded table size from tableIndexStats: %v", err)
	}

	assessmentIssues := convertAssessmentIssueToYugabyteDAssessmentIssue(assessmentReport)

	payload := AssessMigrationPayload{
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

		Notes: assessmentReport.Notes, // for backward compatibility
		AssessmentJsonReport: AssessmentReportYugabyteD{ // for backward compatibility
			VoyagerVersion:             assessmentReport.VoyagerVersion,
			TargetDBVersion:            assessmentReport.TargetDBVersion,
			MigrationComplexity:        assessmentReport.MigrationComplexity,
			SchemaSummary:              assessmentReport.SchemaSummary,
			Sizing:                     assessmentReport.Sizing,
			TableIndexStats:            assessmentReport.TableIndexStats,
			Notes:                      assessmentReport.Notes,
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
		noteType := GetNoteType(note)
		switch noteType {
		case GeneralNotes:
			payload.GeneralNotes = append(payload.GeneralNotes, note)
		case ColocatedShardedNotes:
			payload.ColocatedShardedNotes = append(payload.ColocatedShardedNotes, note)
		case SizingNotes:
			payload.SizingNotes = append(payload.SizingNotes, note)
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		utils.PrintAndLog("Failed to serialise the final report to json (ERR IGNORED): %s", err)
	}

	ev.Report = string(payloadBytes)
	log.Infof("assess migration payload send to yugabyted: %s", ev.Report)
	return ev
}
