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
	Short: "TODO: add short description",
	Long:  `TODO: add long description`,

	Run: func(cmd *cobra.Command, args []string) {
		InitiatePrimarySwitch(cmd.Use)
	},
}

func init() {
	rootCmd.AddCommand(cutoverCmd)
	cutoverCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
	cutoverCmd.Flags().BoolVarP(&wait, "wait", "w", false,
		"wait for the cutover to complete")
}

func InitiatePrimarySwitch(action string) {
	createTriggerIfNotExists(getTriggerName(action))
	if wait {
		utils.PrintAndLog("waiting for %s to complete", action)
		waitForDBSwitchOverToComplete(action)
		utils.PrintAndLog("%s completed...", action)
	}
}

func createTriggerIfNotExists(triggerName string) {
	triggerFPath := filepath.Join(exportDir, "metainfo", "triggers", triggerName)
	if utils.FileOrFolderExists(triggerFPath) {
		utils.PrintAndLog("%s already initiated, wait for it to complete", triggerName)
		return
	}
	file, err := os.Create(triggerFPath)
	if err != nil {
		utils.ErrExit("failed to create trigger file(%s): %v", triggerFPath, err)
	}

	err = file.Close()
	if err != nil {
		utils.ErrExit("failed to close trigger file(%s): %w", triggerFPath, err)
	}
	utils.PrintAndLog("%s initiated", triggerName)
}

/*
trigger file name will be based on the command and db type
args[0] is the action
args[1] will be exporter or importer
args[2] will be db type
*/
func getTriggerName(args ...string) string {
	switch len(args) {
	case 0:
		panic("no arguments passed to getTriggerFileName")
	case 1: // for cutover or fall-forward commands
		return args[0]
	case 3: // for exporter or importer commands
		if args[1] == "exporter" {
			if args[2] != YUGABYTEDB {
				return "cutover.source"
			} else {
				return "fallforward.target"
			}
		} else if args[1] == "importer" {
			if args[2] != YUGABYTEDB {
				return "fallforward.ff"
			} else if args[2] == YUGABYTEDB {
				return "cutover.target"
			}
		} else {
			panic("invalid argument passed to getTriggerFileName")
		}
	default:
		panic("invalid number of arguments passed to getTriggerFileName")
	}
	return ""
}

func waitForDBSwitchOverToComplete(action string) {
	triggerFPath := filepath.Join(exportDir, "metainfo", "triggers", getTriggerName(action, "importer", YUGABYTEDB))
	for {
		if utils.FileOrFolderExists(triggerFPath) {
			break
		}
		time.Sleep(2 * time.Second)
	}
}

func exitIfDBSwitchedOver(triggerName string) {
	if !dbzm.IsLiveMigrationInStreamingMode(exportDir) {
		return
	}
	triggerFPath := filepath.Join(exportDir, "metainfo", "triggers", triggerName)
	if utils.FileOrFolderExists(triggerFPath) {
		utils.PrintAndLog("cutover already complete")
		// Question: do we need to support start clean flag with cutover
		os.Exit(0)
	}
}
