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
var deleteSegments bool
var utilizationThreshold int

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "This command will archive the streaming data from the source database",
	Long:  `This command currently has one subcommand to archive streaming data: changes.`,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}

func registerCommonArchiveFlags(cmd *cobra.Command) {
	registerCommonGlobalFlags(cmd)

	cmd.Flags().StringVar(&moveDestination, "move-to", "",
		"destination to move exported data to")

	cmd.Flags().BoolVar(&deleteSegments, "delete", false,
		"delete exported data after moving it to destination, default is false")

	cmd.Flags().IntVar(&utilizationThreshold, "fs-utilization-threshold", 70,
		"disk utilization threshold in percentage")
}

func validateCommonArchiveFlags() {
	validateExportDirFlag()
	validateMoveToFlag()
}

func validateMoveToFlag() {
	if !utils.FileOrFolderExists(moveDestination) {
		utils.ErrExit("move destination %q doesn't exists.\n", moveDestination)
	} else {
		var err error
		moveDestination, err = filepath.Abs(moveDestination)
		if err != nil {
			utils.ErrExit("Failed to get absolute path for move destination %q: %v\n", moveDestination, err)
		}
		moveDestination = filepath.Clean(moveDestination)
		fmt.Printf("Note: Using %q as move destination\n", moveDestination)
	}
}
