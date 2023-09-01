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
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	NOT_INITIATED = "NOT_INITIATED"
	INITIATED     = "INITIATED"
	COMPLETED     = "COMPLETED"
)

var cutoverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints status of the cutover to YugabyteDB",
	Long:  `Prints status of the cutover to YugabyteDB`,

	Run: func(cmd *cobra.Command, args []string) {
		checkAndReportCutoverStatus()
	},
}

func init() {
	cutoverCmd.AddCommand(cutoverStatusCmd)
	cutoverStatusCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
}

func checkAndReportCutoverStatus() {
	cutoverFpath := filepath.Join(exportDir, "metainfo", "triggers", "cutover")
	cutoverSrcFpath := filepath.Join(exportDir, "metainfo", "triggers", "cutover.source")
	cutoverTgtFpath := filepath.Join(exportDir, "metainfo", "triggers", "cutover.target")
	fallforwardSynchronizeStartedFpath := filepath.Join(exportDir, "metainfo", "triggers", "fallforward.synchronize")

	a := utils.FileOrFolderExists(cutoverFpath)
	b := utils.FileOrFolderExists(cutoverSrcFpath)
	c := utils.FileOrFolderExists(cutoverTgtFpath)
	d := utils.FileOrFolderExists(fallforwardSynchronizeStartedFpath)
	migrationStatusRecord, err := GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("could not fetch MigrationstatusRecord: %w", err)
	}
	ffDBExists := migrationStatusRecord.FallForwarDBExists

	fmt.Printf("cutover status: ")
	if !a {
		color.Red("%s\n", NOT_INITIATED)
	} else if !ffDBExists && a && b && c {
		color.Green("%s\n", COMPLETED)
	} else if a && b && c && d {
		color.Green("%s\n", COMPLETED)
	} else {
		color.Yellow("%s\n", INITIATED)
	}
}
