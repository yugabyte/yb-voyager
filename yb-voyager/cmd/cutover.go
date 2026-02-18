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
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var cutoverCmd = &cobra.Command{
	Use:   "cutover",
	Short: "Prepare to point your application to a different database during live migration.",
	Long:  "",
}

var cutoverRootCmd = &cobra.Command{
	Use:   cutoverCmd.Use,
	Short: "Get cutover related information. To initiate cutover, refer to `yb-voyager initiate cutover to` command.",
	Long:  "",
}

var cutoverToCmd = &cobra.Command{
	Use:   "to",
	Short: cutoverCmd.Short,
	Long:  cutoverCmd.Long,
}

func init() {
	cutoverToCmd.PersistentFlags().BoolVarP(&utils.DoNotPrompt, "yes", "y", false,
		"assume answer as yes for all questions during migration (default false)")
	cutoverToCmd.PersistentFlags().MarkHidden("yes") //for non TTY shell e.g jenkins for docker case
}

func InitiateCutover(dbRole string, prepareforFallback bool, useYBgRPCConnector bool) error {
	userFacingActionMsg := fmt.Sprintf("cutover to %s", dbRole)
	if !utils.AskPrompt(fmt.Sprintf("Are you sure you want to initiate %s? (y/n)", userFacingActionMsg)) {
		utils.PrintAndLogf("Aborting %s", userFacingActionMsg)
		return nil
	}
	alreadyInitiated := false
	alreadyInitiatedMsg := fmt.Sprintf("cutover to %s already initiated, wait for it to complete", dbRole)

	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch dbRole {
		case "target":
			if record.CutoverToTargetRequested {
				alreadyInitiated = true
				return
			}
			record.CutoverToTargetRequested = true
			record.CutoverTimings.ToTargetRequestedAt = utils.GetCurrentTimestamp()
			if prepareforFallback {
				record.FallbackEnabled = true
			}
			if useYBgRPCConnector {
				record.UseYBgRPCConnector = true
			}
		case "source-replica":
			if record.CutoverToSourceReplicaRequested {
				alreadyInitiated = true
				return
			}
			record.CutoverToSourceReplicaRequested = true
			record.CutoverTimings.ToSourceReplicaRequestedAt = utils.GetCurrentTimestamp()
		case "source":
			if record.CutoverToSourceRequested {
				alreadyInitiated = true
				return
			}
			record.CutoverToSourceRequested = true
			record.CutoverTimings.ToSourceRequestedAt = utils.GetCurrentTimestamp()
		}
	})
	if err != nil {
		return fmt.Errorf("failed to update MSR: %w", err)
	}

	if alreadyInitiated {
		utils.PrintAndLog(alreadyInitiatedMsg)
	} else {
		utils.PrintAndLogf("%s initiated, wait for it to complete", userFacingActionMsg)
	}
	return nil
}

func markCutoverProcessed(importerOrExporterRole string) error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch importerOrExporterRole {
		case SOURCE_DB_EXPORTER_ROLE:
			record.CutoverProcessedBySourceExporter = true
			record.CutoverTimings.ProcessedBySourceExporterAt = utils.GetCurrentTimestamp()
		case TARGET_DB_IMPORTER_ROLE:
			record.CutoverProcessedByTargetImporter = true
			record.CutoverTimings.ProcessedByTargetImporterAt = utils.GetCurrentTimestamp()
		case TARGET_DB_EXPORTER_FF_ROLE:
			record.CutoverToSourceReplicaProcessedByTargetExporter = true
			record.CutoverTimings.ToSourceReplicaProcessedByTargetExporterAt = utils.GetCurrentTimestamp()
		case TARGET_DB_EXPORTER_FB_ROLE:
			record.CutoverToSourceProcessedByTargetExporter = true
			record.CutoverTimings.ToSourceProcessedByTargetExporterAt = utils.GetCurrentTimestamp()
		case SOURCE_REPLICA_DB_IMPORTER_ROLE:
			record.CutoverToSourceReplicaProcessedBySRImporter = true
			record.CutoverTimings.ToSourceReplicaProcessedBySRImporterAt = utils.GetCurrentTimestamp()
		case SOURCE_DB_IMPORTER_ROLE:
			record.CutoverToSourceProcessedBySourceImporter = true
			record.CutoverTimings.ToSourceProcessedBySourceImporterAt = utils.GetCurrentTimestamp()
		default:
			panic(fmt.Sprintf("invalid role %s", importerOrExporterRole))
		}
	})
	return err
}

func ExitIfAlreadyCutover(importerOrExporterRole string) {
	if !dbzm.IsMigrationInStreamingMode(exportDir) {
		return
	}

	record, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record to check cutover: %s", err)
	}
	cTAlreadyCompleted := "cutover already completed for this migration, aborting..."
	cSRAlreadyCompleted := "cutover to source-replica already completed for this migration, aborting..."
	cSAlreadyCompleted := "cutover to source already completed for this migration, aborting..."
	switch importerOrExporterRole {
	case SOURCE_DB_EXPORTER_ROLE:
		if record.CutoverProcessedBySourceExporter {
			utils.ErrExit(cTAlreadyCompleted)
		}
	case TARGET_DB_IMPORTER_ROLE:
		if record.CutoverProcessedByTargetImporter {
			utils.ErrExit(cTAlreadyCompleted)
		}
	case TARGET_DB_EXPORTER_FF_ROLE:
		if record.CutoverToSourceReplicaProcessedByTargetExporter {
			utils.ErrExit(cSRAlreadyCompleted)
		}
	case TARGET_DB_EXPORTER_FB_ROLE:
		if record.CutoverToSourceProcessedByTargetExporter {
			utils.ErrExit(cSAlreadyCompleted)
		}
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		if record.CutoverToSourceReplicaProcessedBySRImporter {
			utils.ErrExit(cSRAlreadyCompleted)
		}
	case SOURCE_DB_IMPORTER_ROLE:
		if record.CutoverToSourceProcessedBySourceImporter {
			utils.ErrExit(cSAlreadyCompleted)
		}
	default:
		panic(fmt.Sprintf("invalid role %s", importerOrExporterRole))
	}
}

// CalculateCutoverTimingsForTarget calculates cutover timing metrics for cutover to target
func CalculateCutoverTimingsForTarget(record *metadb.MigrationStatusRecord) *callhome.CutoverTimings {
	if !record.CutoverToTargetRequested {
		return nil
	}

	requestedAt := record.CutoverTimings.ToTargetRequestedAt
	var completedAt time.Time

	// Determine completion time based on whether fall-forward/fall-back is enabled
	if record.FallForwardEnabled && !record.CutoverTimings.ExportFromTargetFallForwardStartedAt.IsZero() {
		completedAt = record.CutoverTimings.ExportFromTargetFallForwardStartedAt
	} else if record.FallbackEnabled && !record.CutoverTimings.ExportFromTargetFallBackStartedAt.IsZero() {
		completedAt = record.CutoverTimings.ExportFromTargetFallBackStartedAt
	} else if !record.FallForwardEnabled && !record.FallbackEnabled && !record.CutoverTimings.ProcessedByTargetImporterAt.IsZero() {
		// No fall-forward/fall-back, cutover completes when target importer is done
		completedAt = record.CutoverTimings.ProcessedByTargetImporterAt
	} else {
		// Cutover probably not yet complete
		return nil
	}

	// Check if timestamps are valid
	if requestedAt.IsZero() || completedAt.IsZero() {
		return nil
	}

	log.Infof("CalculateCutoverTimingsForTarget: total cutover to target time: %d seconds", int64(completedAt.Sub(requestedAt).Seconds()))
	return &callhome.CutoverTimings{
		TotalCutoverTimeSec: int64(completedAt.Sub(requestedAt).Seconds()),
		CutoverType:         "target",
	}
}

// CalculateCutoverTimingsForSource calculates cutover timing metrics for cutover to source
func CalculateCutoverTimingsForSource(record *metadb.MigrationStatusRecord) *callhome.CutoverTimings {
	if !record.CutoverToSourceRequested || !record.CutoverToSourceProcessedBySourceImporter {
		return nil
	}

	requestedAt := record.CutoverTimings.ToSourceRequestedAt
	completedAt := record.CutoverTimings.ToSourceProcessedBySourceImporterAt
	if requestedAt.IsZero() || completedAt.IsZero() {
		return nil
	}

	log.Infof("CalculateCutoverTimingsForSource: total cutover to source time: %d seconds", int64(completedAt.Sub(requestedAt).Seconds()))
	return &callhome.CutoverTimings{
		TotalCutoverTimeSec: int64(completedAt.Sub(requestedAt).Seconds()),
		CutoverType:         "source",
	}
}

// CalculateCutoverTimingsForSourceReplica calculates cutover timing metrics for cutover to source-replica
func CalculateCutoverTimingsForSourceReplica(record *metadb.MigrationStatusRecord) *callhome.CutoverTimings {
	if !record.CutoverToSourceReplicaRequested || !record.CutoverToSourceReplicaProcessedBySRImporter {
		return nil
	}

	requestedAt := record.CutoverTimings.ToSourceReplicaRequestedAt
	completedAt := record.CutoverTimings.ToSourceReplicaProcessedBySRImporterAt
	if requestedAt.IsZero() || completedAt.IsZero() {
		return nil
	}

	log.Infof("CalculateCutoverTimingsForSourceReplica: total cutover to source-replica time: %d seconds", int64(completedAt.Sub(requestedAt).Seconds()))
	return &callhome.CutoverTimings{
		TotalCutoverTimeSec: int64(completedAt.Sub(requestedAt).Seconds()),
		CutoverType:         "source-replica",
	}
}
