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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var wait bool

var cutoverCmd = &cobra.Command{
	Use:   "cutover",
	Short: "Initiate cutover to YugabyteDB",
	Long:  `Initiate cutover to YugabyteDB`,

	Run: func(cmd *cobra.Command, args []string) {
		err := InitiatePrimarySwitch(cmd.Use)
		if err != nil {
			utils.ErrExit("failed to initiate cutover: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(cutoverCmd)
	cutoverCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
	cutoverCmd.Flags().BoolVarP(&wait, "wait", "w", false,
		"wait for the cutover to complete")
}

func InitiatePrimarySwitch(action string) error {
	triggerName, err := getTriggerName(action)
	if err != nil {
		return fmt.Errorf("failed to get trigger name: %w", err)
	}
	err = createTriggerIfNotExists(triggerName)
	if err != nil {
		return err
	}

	if wait {
		utils.PrintAndLog("waiting for %s to complete", action)
		err = waitForDBSwitchOverToComplete(action)
		if err != nil {
			return fmt.Errorf("failed waiting for %s to complete: %w", action, err)
		}
		utils.PrintAndLog("%s completed...", action)
	}
	return nil
}

func createTriggerIfNotExists(triggerName string) error {
	triggerFPath := filepath.Join(exportDir, "metainfo", "triggers", triggerName)
	if utils.FileOrFolderExists(triggerFPath) {
		utils.PrintAndLog("%s already initiated, wait for it to complete", triggerName)
		return nil
	}
	file, err := os.Create(triggerFPath)
	if err != nil {
		return fmt.Errorf("failed to create trigger file(%s): %w", triggerFPath, err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("failed to close trigger file(%s): %w", triggerFPath, err)
	}
	utils.PrintAndLog("%s initiated", triggerName)
	return nil
}

/*
trigger file name will be based on the command and db type
args[0] is the action
args[1] will be exporter or importer
args[2] will be db type
*/
func getTriggerName(args ...string) (string, error) {
	switch len(args) {
	case 0:
		return "", fmt.Errorf("no arguments passed to getTriggerName")
	case 1: // for cutover or fall-forward commands
		return args[0], nil
	case 3: // for exporter or importer commands
		if args[1] == "exporter" {
			if args[2] != YUGABYTEDB {
				return "cutover.source", nil
			} else {
				return "fallforward.target", nil
			}
		} else if args[1] == "importer" {
			if args[2] != YUGABYTEDB {
				return "fallforward.ff", nil
			} else if args[2] == YUGABYTEDB {
				return "cutover.target", nil
			}
		} else {
			return "", fmt.Errorf("invalid arg=%s passed to getTriggerName", args[1])
		}
	default:
		return "", fmt.Errorf("invalid number of arguments passed to getTriggerFileName")
	}
	return "", nil
}

func waitForDBSwitchOverToComplete(action string) error {
	triggerName, err := getTriggerName(action, "importer", YUGABYTEDB)
	if err != nil {
		return fmt.Errorf("failed to get trigger name for checking if switchover is complete: %v", err)
	}
	triggerFPath := filepath.Join(exportDir, "metainfo", "triggers", triggerName)
	for {
		if utils.FileOrFolderExists(triggerFPath) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

func exitIfDBSwitchedOver(triggerName string) {
	if !dbzm.IsMigrationInStreamingMode(exportDir) {
		return
	}
	triggerFPath := filepath.Join(exportDir, "metainfo", "triggers", triggerName)
	if utils.FileOrFolderExists(triggerFPath) {
		utils.PrintAndLog("cutover already complete")
		// Question: do we need to support start clean flag with cutover
		os.Exit(0)
	}
}
