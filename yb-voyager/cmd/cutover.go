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
	Short: cutoverCmd.Short,
	Long:  cutoverCmd.Long,
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
}

func InitiateCutover(dbRole string) error {
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

// func createTriggerIfNotExists(triggerName string) error {
// 	cutoverMsg := "cutover already initiated, wait for it to complete"
// 	fallforwardMsg := "cutover to source-replica already initiated, wait for it to complete"
// 	fallbackMsg := "cutover to source already initiated, wait for it to complete"
// 	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
// 		switch triggerName {
// 		case "cutover":
// 			if record.CuCutoverToTargetRequested
// 				utils.PrintAndLog(cutoverMsg)
// 			}
// 			// if the above check fails, we will set this to true otherwise its a no-op
// 			record.CuCutoverToTargetRequested true
// 		case "cutover.source":
// 			if record.CutoverProcessedBySourceExporter {
// 				utils.PrintAndLog(cutoverMsg)
// 			}
// 			record.CutoverProcessedBySourceExporter = true
// 		case "cutover.target":
// 			if record.CutoverProcessedByTargetImporter {
// 				utils.PrintAndLog(cutoverMsg)
// 			}
// 			record.CutoverProcessedByTargetImporter = true
// 		case "fallforward":
// 			if record.CutoverToSourceReplicaRequested {
// 				utils.PrintAndLog(fallforwardMsg)
// 			}
// 			record.CutoverToSourceReplicaRequested = true
// 		case "fallforward.target":
// 			if record.CutoverToSourceReplicaProcessedByTargetExporter {
// 				utils.PrintAndLog(fallforwardMsg)
// 			}
// 			record.CutoverToSourceReplicaProcessedByTargetExporter = true
// 		case "fallforward.ff":
// 			if record.CutoverToSourceReplicaProcessedBySRImporter {
// 				utils.PrintAndLog(fallforwardMsg)
// 			}
// 			record.CutoverToSourceReplicaProcessedBySRImporter = true
// 		case "fallback":
// 			if record.CutoverToSourceRequested {
// 				utils.PrintAndLog(fallbackMsg)
// 			}
// 			record.CutoverToSourceRequested = true
// 		case "fallback.target":
// 			if record.CutoverToSourceProcessedByTargetExporter {
// 				utils.PrintAndLog(fallbackMsg)
// 			}
// 			record.CutoverToSourceProcessedByTargetExporter = true
// 		case "fallback.source":
// 			if record.CutoverToSourceProcessedBySourceImporter {
// 				utils.PrintAndLog(fallbackMsg)
// 			}
// 			record.CutoverToSourceProcessedBySourceImporter = true
// 		default:
// 			panic("invalid trigger name")
// 		}
// 	})
// 	if err != nil {
// 		log.Errorf("creating trigger(%s): %v", triggerName, err)
// 		return fmt.Errorf("creating trigger(%s): %w", triggerName, err)
// 	}
// 	return nil
// }

func getTriggerName(importerOrExporterRole string) (string, error) {
	switch importerOrExporterRole {
	case SOURCE_DB_EXPORTER_ROLE:
		return "cutover.source", nil
	case TARGET_DB_IMPORTER_ROLE:
		return "cutover.target", nil
	case TARGET_DB_EXPORTER_FF_ROLE:
		return "fallforward.target", nil
	case TARGET_DB_EXPORTER_FB_ROLE:
		return "fallback.target", nil
	case FF_DB_IMPORTER_ROLE:
		return "fallforward.ff", nil
	case FB_DB_IMPORTER_ROLE:
		return "fallback.source", nil
	default:
		return "", fmt.Errorf("invalid role %s", importerOrExporterRole)
	}
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
		case FF_DB_IMPORTER_ROLE:
			record.CutoverToSourceReplicaProcessedBySRImporter = true
		case FB_DB_IMPORTER_ROLE:
			record.CutoverToSourceProcessedBySourceImporter = true
		default:
			panic(fmt.Sprintf("invalid role %s", importerOrExporterRole))
		}
	})
	return err
}

func exitIfDBSwitchedOver(triggerName string) {
	if !dbzm.IsMigrationInStreamingMode(exportDir) {
		return
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("exit if db switched over for trigger(%s) exists: load migration status record: %s", triggerName, err)
	}
	cutoverMsg := "cutover already completed for this migration, aborting..."
	fallforwardMsg := "cutover to source-replica already completed for this migration, aborting..."
	fallbackMsg := "cutover to source already completed for this migration, aborting..."
	switch triggerName { // only these trigger names required to be checked for db switch over
	case "cutover.source":
		if msr.CutoverProcessedBySourceExporter {
			utils.ErrExit(cutoverMsg)
		}
	case "cutover.target":
		if msr.CutoverProcessedByTargetImporter {
			utils.ErrExit(cutoverMsg)
		}
	case "fallforward.target":
		if msr.CutoverToSourceReplicaProcessedByTargetExporter {
			utils.ErrExit(fallforwardMsg)
		}
	case "fallforward.ff":
		if msr.CutoverToSourceReplicaProcessedBySRImporter {
			utils.ErrExit(fallforwardMsg)
		}
	case "fallback.source":
		if msr.CutoverToSourceProcessedBySourceImporter {
			utils.ErrExit(fallbackMsg)
		}
	case "fallback.target":
		if msr.CutoverToSourceProcessedByTargetExporter {
			utils.ErrExit(fallbackMsg)
		}
	default:
		panic("invalid trigger name - " + triggerName)
	}
}
