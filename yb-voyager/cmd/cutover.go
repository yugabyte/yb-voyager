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

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
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
	triggerName, err := getTriggerName(action)
	if err != nil {
		return fmt.Errorf("get trigger name: %w", err)
	}
	err = createTriggerIfNotExists(triggerName)
	if err != nil {
		return err
	}
	log.Infof("cutover initiated, wait for it to complete")
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
		return fmt.Errorf("create trigger file(%s): %w", triggerFPath, err)
	}
	err = file.Close()
	if err != nil {
		return fmt.Errorf("close trigger file(%s): %w", triggerFPath, err)
	}
	log.Infof("created trigger file(%s)", triggerFPath)
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
