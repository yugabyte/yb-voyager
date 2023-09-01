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

var fallforwardStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints status of the fallforward to fallforward DB",
	Long:  `Prints status of the fallforward to fallforward DB`,

	Run: func(cmd *cobra.Command, args []string) {
		checkAndReportFallforwardStatus()
	},
}

func init() {
	fallForwardCmd.AddCommand(fallforwardStatusCmd)
	fallforwardStatusCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
}

func checkAndReportFallforwardStatus() {
	fallforwardFPath := filepath.Join(exportDir, "metainfo", "triggers", "fallforward")
	fallforwardTargetFPath := filepath.Join(exportDir, "metainfo", "triggers", "fallforward.target")
	fallforwardFFFPath := filepath.Join(exportDir, "metainfo", "triggers", "fallforward.ff")

	a := utils.FileOrFolderExists(fallforwardFPath)
	b := utils.FileOrFolderExists(fallforwardTargetFPath)
	c := utils.FileOrFolderExists(fallforwardFFFPath)

	fmt.Printf("fall-forward status: ")
	if !a {
		color.Red("%s\n", NOT_INITIATED)
	} else if a && b && c {
		color.Green("%s\n", COMPLETED)
	} else {
		color.Yellow("%s\n", INITIATED)
	}
}
