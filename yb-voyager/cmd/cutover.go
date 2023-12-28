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

	"github.com/spf13/cobra"
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
	rootCmd.AddCommand(cutoverRootCmd)
	initiateCmd.AddCommand(cutoverCmd)
	cutoverCmd.AddCommand(cutoverToCmd)
	cutoverToCmd.PersistentFlags().BoolVarP(&utils.DoNotPrompt, "yes", "y", false,
		"assume answer as yes for all questions during migration (default false)")
	cutoverToCmd.PersistentFlags().MarkHidden("yes") //for non TTY shell e.g jenkins for docker case
}

func InitiateCutover(dbRole string, prepareforFallback bool) error {
	userFacingActionMsg := fmt.Sprintf("cutover to %s", dbRole)
	if !utils.AskPrompt(fmt.Sprintf("Are you sure you want to initiate %s? (y/n)", userFacingActionMsg)) {
		utils.PrintAndLog("Aborting %s", userFacingActionMsg)
		return nil
	}
	alreadyInitiatedMsg := fmt.Sprintf("cutover to %s already initiated, wait for it to complete", dbRole)

	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch dbRole {
		case "target":
			if record.CutoverToTargetRequested {
				utils.PrintAndLog(alreadyInitiatedMsg)
			}
			record.CutoverToTargetRequested = true
			if prepareforFallback {
				record.FallbackEnabled = true
			}
		case "source-replica":
			if record.CutoverToSourceReplicaRequested {
				utils.PrintAndLog(alreadyInitiatedMsg)
			}
			record.CutoverToSourceReplicaRequested = true
		case "source":
			if record.CutoverToSourceRequested {
				utils.PrintAndLog(alreadyInitiatedMsg)
			}
			record.CutoverToSourceRequested = true
		}

	})
	if err != nil {
		return fmt.Errorf("failed to update MSR: %w", err)
	}
	utils.PrintAndLog("%s initiated, wait for it to complete", userFacingActionMsg)
	return nil
}

func markCutoverProcessed(importerOrExporterRole string) error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch importerOrExporterRole {
		case SOURCE_DB_EXPORTER_ROLE:
			record.CutoverProcessedBySourceExporter = true
		case TARGET_DB_IMPORTER_ROLE:
			record.CutoverProcessedByTargetImporter = true
		case TARGET_DB_EXPORTER_FF_ROLE:
			record.CutoverToSourceReplicaProcessedByTargetExporter = true
		case TARGET_DB_EXPORTER_FB_ROLE:
			record.CutoverToSourceProcessedByTargetExporter = true
		case SOURCE_REPLICA_DB_IMPORTER_ROLE:
			record.CutoverToSourceReplicaProcessedBySRImporter = true
		case SOURCE_DB_IMPORTER_ROLE:
			record.CutoverToSourceProcessedBySourceImporter = true
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
		utils.ErrExit("exit if already cutover: load migration status record: %s", err)
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
