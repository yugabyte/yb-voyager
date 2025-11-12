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
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

/*
YB-AEON DATA FLOW - Migration Assessment Completed Event:

1. createMigrationAssessmentCompletedEventForYBAeon() (here)
   - Creates AssessMigrationPayloadYBM struct with all assessment data
   - NO marshaling - sets ev.Report = payload (type: interface{})
   - Struct is passed directly

2. MigrationAssessmentCompletedEvent.Report flows to ybaeon.go
   - Report field type: interface{} (containing AssessMigrationPayloadYBM struct)

3. ybaeon.MigrationAssessmentCompleted() in src/cp/ybaeon/ybaeon.go
   - Receives ev.Report as interface{} (still a struct)
   - Passes it to createAndSendEvent() as payload parameter

4. ybaeon.createAndSendEvent() in src/cp/ybaeon/ybaeon.go
   - Assigns payload to MigrationEvent.Payload field (type: interface{})
   - MARSHALS entire MigrationEvent (including nested payload struct) to JSON: json.Marshal(migrationEvent)
   - Sends JSON bytes to YB-Aeon API via HTTP POST

Result: Single marshal (entire event with nested struct -> JSON), no unmarshal
Fully decoupled from Yugabyted - different payload struct, different flow, no redundant marshal cycles
*/

// createMigrationAssessmentCompletedEventForYBAeon creates an event optimized for YB-Aeon
// Unlike yugabyted, YB-Aeon passes the payload struct directly without pre-marshaling
// Contains all current features without legacy/backward compatibility fields
func createMigrationAssessmentCompletedEventForYBAeon() *cp.MigrationAssessmentCompletedEvent {
	ev := &cp.MigrationAssessmentCompletedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")

	// Calculate sizing details (same as yugabyted, but without legacy fields)
	totalColocatedSize, err := assessmentReport.GetTotalColocatedSize(source.DBType)
	if err != nil {
		utils.PrintAndLog("failed to calculate the total colocated table size from tableIndexStats: %v", err)
	}

	totalShardedSize, err := assessmentReport.GetTotalShardedSize(source.DBType)
	if err != nil {
		utils.PrintAndLog("failed to calculate the total sharded table size from tableIndexStats: %v", err)
	}

	// Convert assessment issues to YBM format (similar to yugabyted conversion)
	assessmentIssuesYBM := convertAssessmentIssueToYBMAssessmentIssue(assessmentReport)

	// Create YBM payload with all current features
	payload := AssessMigrationPayloadYBM{
		PayloadVersion:                 ASSESS_MIGRATION_YBM_PAYLOAD_VERSION,
		VoyagerVersion:                 assessmentReport.VoyagerVersion,
		TargetDBVersion:                assessmentReport.TargetDBVersion,
		MigrationComplexity:            assessmentReport.MigrationComplexity,
		MigrationComplexityExplanation: assessmentReport.MigrationComplexityExplanation,
		SchemaSummary:                  assessmentReport.SchemaSummary,
		AssessmentIssues:               assessmentIssuesYBM,
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
	}

	// Classify notes into categories (same as yugabyted)
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

	// Pass the struct directly - NO marshaling to JSON string
	// The YBM handler will use it as-is and marshal only once when sending to API
	ev.Report = payload
	log.Infof("assess migration payload for YBM: version %s", ASSESS_MIGRATION_YBM_PAYLOAD_VERSION)
	return ev
}
