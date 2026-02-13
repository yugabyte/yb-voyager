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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// Phase names used across all workflows.
const (
	PhaseAssess = "Assess"
	PhaseSchema = "Schema"
	PhaseData   = "Data"
	PhaseEnd    = "End"
)

// Step IDs used to identify individual steps in a workflow.
const (
	StepAssess              = "assess"
	StepExportSchema        = "export-schema"
	StepAnalyzeSchema       = "analyze-schema"
	StepImportSchema        = "import-schema"
	StepExportData          = "export-data"
	StepImportData          = "import-data"
	StepCutoverToTarget     = "cutover-to-target"
	StepFinalizeSchema      = "finalize-schema"
	StepExportDataFromTgt   = "export-data-from-target"
	StepImportDataToSource  = "import-data-to-source"
	StepImportDataToReplica = "import-data-to-replica"
	StepCutoverToSource     = "cutover-to-source"
	StepCutoverToReplica    = "cutover-to-replica"
	StepEnd                 = "end"
)

// WorkflowStep represents one command in a migration workflow.
type WorkflowStep struct {
	ID          string                                         // unique step identifier
	DisplayName string                                         // human-readable name for display
	Phase       string                                         // high-level phase this step belongs to
	Command     string                                         // CLI command string (e.g., "export schema")
	IsDone      func(msr *metadb.MigrationStatusRecord) bool   // checks MSR for completion
}

// Workflow is an ordered sequence of steps for a migration type.
type Workflow struct {
	Name  string           // "offline", "live", "live-fall-back", "live-fall-forward"
	Steps []WorkflowStep
}

// offlineWorkflow defines the step sequence for offline (snapshot-only) migrations.
var offlineWorkflow = &Workflow{
	Name: "offline",
	Steps: []WorkflowStep{
		{
			ID: StepAssess, DisplayName: "Assess Migration", Phase: PhaseAssess,
			Command: "assess-migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.MigrationAssessmentDone },
		},
		{
			ID: StepExportSchema, DisplayName: "Export Schema", Phase: PhaseSchema,
			Command: "export schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportSchemaDone },
		},
		{
			ID: StepAnalyzeSchema, DisplayName: "Analyze Schema", Phase: PhaseSchema,
			Command: "analyze-schema",
			IsDone: func(msr *metadb.MigrationStatusRecord) bool {
				return schemaIsAnalyzed()
			},
		},
		{
			ID: StepImportSchema, DisplayName: "Import Schema", Phase: PhaseSchema,
			Command: "import schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportSchemaDone },
		},
		{
			ID: StepExportData, DisplayName: "Export Data", Phase: PhaseData,
			Command: "export data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportDataDone },
		},
		{
			ID: StepImportData, DisplayName: "Import Data", Phase: PhaseData,
			Command: "import data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportDataDone },
		},
		{
			ID: StepEnd, DisplayName: "End Migration", Phase: PhaseEnd,
			Command: "end migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.EndMigrationRequested },
		},
	},
}

// liveWorkflow defines the step sequence for live (CDC) migrations without fall-back/fall-forward.
var liveWorkflow = &Workflow{
	Name: "live",
	Steps: []WorkflowStep{
		{
			ID: StepAssess, DisplayName: "Assess Migration", Phase: PhaseAssess,
			Command: "assess-migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.MigrationAssessmentDone },
		},
		{
			ID: StepExportSchema, DisplayName: "Export Schema", Phase: PhaseSchema,
			Command: "export schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportSchemaDone },
		},
		{
			ID: StepAnalyzeSchema, DisplayName: "Analyze Schema", Phase: PhaseSchema,
			Command: "analyze-schema",
			IsDone: func(msr *metadb.MigrationStatusRecord) bool {
				return schemaIsAnalyzed()
			},
		},
		{
			ID: StepImportSchema, DisplayName: "Import Schema", Phase: PhaseSchema,
			Command: "import schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportSchemaDone },
		},
		{
			ID: StepExportData, DisplayName: "Export Data", Phase: PhaseData,
			Command: "export data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportDataDone },
		},
		{
			ID: StepImportData, DisplayName: "Import Data", Phase: PhaseData,
			Command: "import data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportDataDone },
		},
		{
			ID: StepCutoverToTarget, DisplayName: "Cutover to Target", Phase: PhaseData,
			Command: "initiate-cutover-to-target",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverProcessedByTargetImporter },
		},
		{
			ID: StepFinalizeSchema, DisplayName: "Finalize Schema", Phase: PhaseSchema,
			Command: "finalize-schema-post-data-import",
			IsDone: func(msr *metadb.MigrationStatusRecord) bool {
				// Heuristic: not easily detectable from MSR alone.
				return false
			},
		},
		{
			ID: StepEnd, DisplayName: "End Migration", Phase: PhaseEnd,
			Command: "end migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.EndMigrationRequested },
		},
	},
}

// liveFallBackWorkflow defines the step sequence for live migrations with fall-back support.
var liveFallBackWorkflow = &Workflow{
	Name: "live-fall-back",
	Steps: []WorkflowStep{
		{
			ID: StepAssess, DisplayName: "Assess Migration", Phase: PhaseAssess,
			Command: "assess-migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.MigrationAssessmentDone },
		},
		{
			ID: StepExportSchema, DisplayName: "Export Schema", Phase: PhaseSchema,
			Command: "export schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportSchemaDone },
		},
		{
			ID: StepAnalyzeSchema, DisplayName: "Analyze Schema", Phase: PhaseSchema,
			Command: "analyze-schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return schemaIsAnalyzed() },
		},
		{
			ID: StepImportSchema, DisplayName: "Import Schema", Phase: PhaseSchema,
			Command: "import schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportSchemaDone },
		},
		{
			ID: StepExportData, DisplayName: "Export Data", Phase: PhaseData,
			Command: "export data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportDataDone },
		},
		{
			ID: StepImportData, DisplayName: "Import Data", Phase: PhaseData,
			Command: "import data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportDataDone },
		},
		{
			ID: StepCutoverToTarget, DisplayName: "Cutover to Target", Phase: PhaseData,
			Command: "initiate-cutover-to-target",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverProcessedByTargetImporter },
		},
		{
			ID: StepExportDataFromTgt, DisplayName: "Export Data from Target", Phase: PhaseData,
			Command: "export data from target",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportFromTargetFallBackStarted },
		},
		{
			ID: StepImportDataToSource, DisplayName: "Import Data to Source", Phase: PhaseData,
			Command: "import data to source",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverToSourceProcessedBySourceImporter },
		},
		{
			ID: StepCutoverToSource, DisplayName: "Cutover to Source", Phase: PhaseData,
			Command: "initiate-cutover-to-source",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverToSourceRequested },
		},
		{
			ID: StepFinalizeSchema, DisplayName: "Finalize Schema", Phase: PhaseSchema,
			Command: "finalize-schema-post-data-import",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return false },
		},
		{
			ID: StepEnd, DisplayName: "End Migration", Phase: PhaseEnd,
			Command: "end migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.EndMigrationRequested },
		},
	},
}

// liveFallForwardWorkflow defines the step sequence for live migrations with fall-forward support.
var liveFallForwardWorkflow = &Workflow{
	Name: "live-fall-forward",
	Steps: []WorkflowStep{
		{
			ID: StepAssess, DisplayName: "Assess Migration", Phase: PhaseAssess,
			Command: "assess-migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.MigrationAssessmentDone },
		},
		{
			ID: StepExportSchema, DisplayName: "Export Schema", Phase: PhaseSchema,
			Command: "export schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportSchemaDone },
		},
		{
			ID: StepAnalyzeSchema, DisplayName: "Analyze Schema", Phase: PhaseSchema,
			Command: "analyze-schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return schemaIsAnalyzed() },
		},
		{
			ID: StepImportSchema, DisplayName: "Import Schema", Phase: PhaseSchema,
			Command: "import schema",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportSchemaDone },
		},
		{
			ID: StepExportData, DisplayName: "Export Data", Phase: PhaseData,
			Command: "export data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportDataDone },
		},
		{
			ID: StepImportData, DisplayName: "Import Data", Phase: PhaseData,
			Command: "import data",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ImportDataDone },
		},
		{
			ID: StepCutoverToTarget, DisplayName: "Cutover to Target", Phase: PhaseData,
			Command: "initiate-cutover-to-target",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverProcessedByTargetImporter },
		},
		{
			ID: StepExportDataFromTgt, DisplayName: "Export Data from Target", Phase: PhaseData,
			Command: "export data from target",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.ExportFromTargetFallForwardStarted },
		},
		{
			ID: StepImportDataToReplica, DisplayName: "Import Data to Replica", Phase: PhaseData,
			Command: "import data to source-replica",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverToSourceReplicaProcessedBySRImporter },
		},
		{
			ID: StepCutoverToReplica, DisplayName: "Cutover to Replica", Phase: PhaseData,
			Command: "initiate-cutover-to-source-replica",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.CutoverToSourceReplicaRequested },
		},
		{
			ID: StepFinalizeSchema, DisplayName: "Finalize Schema", Phase: PhaseSchema,
			Command: "finalize-schema-post-data-import",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return false },
		},
		{
			ID: StepEnd, DisplayName: "End Migration", Phase: PhaseEnd,
			Command: "end migration",
			IsDone:  func(msr *metadb.MigrationStatusRecord) bool { return msr.EndMigrationRequested },
		},
	},
}

// resolveWorkflow selects the appropriate workflow based on the MigrationStatusRecord.
// Falls back to offlineWorkflow if MSR is nil or has no specific export type set.
func resolveWorkflow(msr *metadb.MigrationStatusRecord) *Workflow {
	if msr == nil {
		return offlineWorkflow
	}
	switch {
	case msr.ExportType == utils.SNAPSHOT_AND_CHANGES && msr.FallbackEnabled:
		return liveFallBackWorkflow
	case msr.ExportType == utils.SNAPSHOT_AND_CHANGES && msr.FallForwardEnabled:
		return liveFallForwardWorkflow
	case msr.ExportType == utils.SNAPSHOT_AND_CHANGES:
		return liveWorkflow
	default:
		return offlineWorkflow
	}
}

// NOTE: schemaIsAnalyzed() is defined in analyzeSchema.go and reused here
// via the IsDone closures in the workflow step definitions.
