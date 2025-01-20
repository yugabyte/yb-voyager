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

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var moveDestination string
var deleteSegments utils.BoolStr
var utilizationThreshold int

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive exported data that is saved in export-dir",
	Long:  ``,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}

func registerCommonArchiveFlags(cmd *cobra.Command) {
	registerCommonGlobalFlags(cmd)

	cmd.Flags().StringVar(&moveDestination, "move-to", "",
		"Path to the directory where the imported change events are to be moved to. "+
			"Note that, the changes are deleted from the export-dir only after the disk utilisation exceeds 70%.")

	BoolVar(cmd.Flags(), &deleteSegments, "delete-changes-without-archiving", false,
		"Delete the imported changes without archiving them. Note that: the changes are deleted from the export-dir only after disk utilisation exceeds 70%.")

	cmd.Flags().IntVar(&utilizationThreshold, "fs-utilization-threshold", 70,
		"disk utilization threshold in percentage")
}

func validateCommonArchiveFlags() {
	validateMoveToFlag()
}

func validateMoveToFlag() {
	if moveDestination != "" {
		if !utils.FileOrFolderExists(moveDestination) {
			utils.ErrExit("move destination doesn't exists: %q: \n", moveDestination)
		} else {
			var err error
			moveDestination, err = filepath.Abs(moveDestination)
			if err != nil {
				utils.ErrExit("Failed to get absolute path for move destination: %q: %v\n", moveDestination, err)
			}
			moveDestination = filepath.Clean(moveDestination)
			fmt.Printf("Note: Using %q as move destination\n", moveDestination)
		}
	}
}
