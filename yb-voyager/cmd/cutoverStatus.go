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

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	NOT_INITIATED = color.YellowString("NOT_INITIATED")
	INITIATED     = color.RedString("INITIATED")
	COMPLETED     = color.GreenString("COMPLETED")
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
	status := checkCutoverStatus()
	fmt.Printf("cutover status: %s\n", status)
}
