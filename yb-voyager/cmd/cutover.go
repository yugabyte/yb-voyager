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

	log "github.com/sirupsen/logrus"
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

func InitiatePrimarySwitch(action string) error {
	if !utils.AskPrompt(fmt.Sprintf("Are you sure you want to initiate %s? (y/n)", action)) {
		utils.PrintAndLog("Aborting %s", action)
		return nil
	}
	triggerName := action
	err := createTriggerIfNotExists(triggerName)
	if err != nil {
		return err
	}
	utils.PrintAndLog("%s initiated, wait for it to complete", action)
	return nil
}

func createTriggerIfNotExists(triggerName string) error {
	cutoverMsg := "cutover already initiated, wait for it to complete"
	fallforwardMsg := "fallforward already initiated, wait for it to complete"
	fallbackMsg := "fallback already initiated, wait for it to complete"
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		switch triggerName {
		case "cutover":
			if record.CutoverRequested {
				utils.PrintAndLog(cutoverMsg)
			}
			// if the above check fails, we will set this to true otherwise its a no-op
			record.CutoverRequested = true
		case "cutover.source":
			if record.CutoverProcessedBySourceExporter {
				utils.PrintAndLog(cutoverMsg)
			}
			record.CutoverProcessedBySourceExporter = true
		case "cutover.target":
			if record.CutoverProcessedByTargetImporter {
				utils.PrintAndLog(cutoverMsg)
			}
			record.CutoverProcessedByTargetImporter = true
		case "fallforward":
			if record.FallForwardSwitchRequested {
				utils.PrintAndLog(fallforwardMsg)
			}
			record.FallForwardSwitchRequested = true
		case "fallforward.target":
			if record.FallForwardSwitchProcessedByTargetExporter {
				utils.PrintAndLog(fallforwardMsg)
			}
			record.FallForwardSwitchProcessedByTargetExporter = true
		case "fallforward.ff":
			if record.FallForwardSwitchProcessedByFFImporter {
				utils.PrintAndLog(fallforwardMsg)
			}
			record.FallForwardSwitchProcessedByFFImporter = true
		case "fallback":
			if record.FallBackSwitchRequested {
				utils.PrintAndLog(fallbackMsg)
			}
			record.FallBackSwitchRequested = true
		case "fallback.target":
			if record.FallBackSwitchProcessedByTargetExporter {
				utils.PrintAndLog(fallbackMsg)
			}
			record.FallBackSwitchProcessedByTargetExporter = true
		case "fallback.source":
			if record.FallBackSwitchProcessedByFBImporter {
				utils.PrintAndLog(fallbackMsg)
			}
			record.FallBackSwitchProcessedByFBImporter = true
		default:
			panic("invalid trigger name")
		}
	})
	if err != nil {
		log.Errorf("creating trigger(%s): %v", triggerName, err)
		return fmt.Errorf("creating trigger(%s): %w", triggerName, err)
	}
	return nil
}

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

func exitIfDBSwitchedOver(triggerName string) {
	if !dbzm.IsMigrationInStreamingMode(exportDir) {
		return
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("exit if db switched over for trigger(%s) exists: load migration status record: %s", triggerName, err)
	}
	cutoverMsg := "cutover already completed for this migration, aborting..."
	fallforwardMsg := "fallforward already completed for this migration, aborting..."
	fallbackMsg := "fallback already completed for this migration, aborting..."
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
		if msr.FallForwardSwitchProcessedByTargetExporter {
			utils.ErrExit(fallforwardMsg)
		}
	case "fallforward.ff":
		if msr.FallForwardSwitchProcessedByFFImporter {
			utils.ErrExit(fallforwardMsg)
		}
	case "fallback.source":
		if msr.FallBackSwitchProcessedByFBImporter {
			utils.ErrExit(fallbackMsg)
		}
	case "fallback.target":
		if msr.FallBackSwitchProcessedByTargetExporter {
			utils.ErrExit(fallbackMsg)
		}
	default:
		panic("invalid trigger name - " + triggerName)
	}
}
