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

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var cutoverCmd = &cobra.Command{
	Use:   "cutover",
	Short: "cutover has further subcommands 'initiate' and 'status' for cutover to YugabyteDB",
	Long:  "cutover has further subcommands 'initiate' and 'status' for cutover to YugabyteDB",
}

func init() {
	rootCmd.AddCommand(cutoverCmd)
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
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		log.Errorf("creating trigger(%s): %v", triggerName, err)
		return fmt.Errorf("creating trigger(%s): %w", triggerName, err)
	}

	if msr != nil && msr.IsTriggerExists(triggerName) {
		utils.PrintAndLog("%s already initiated, wait for it to complete", triggerName)
		return nil
	}

	err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		
		if record.Triggers == nil {
			record.Triggers = []string{triggerName}
		} else {
			record.Triggers = append(record.Triggers, triggerName)
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
	case TARGET_DB_EXPORTER_ROLE:
		return "fallforward.target", nil
	case FF_DB_IMPORTER_ROLE:
		return "fallforward.ff", nil
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
		utils.ErrExit("checking trigger(%s): %v", triggerName, err)
	}

	if msr != nil && msr.IsTriggerExists(triggerName) {
		utils.PrintAndLog("cutover already complete")
		// Question: do we need to support start clean flag with cutover
		os.Exit(0)
	}
}
