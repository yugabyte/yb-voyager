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
	status := getCutoverStatus()
	fmt.Printf("cutover to target status: ")
	switch status {
	case NOT_INITIATED:
		color.Red("%s\n", NOT_INITIATED)
	case INITIATED:
		color.Yellow("%s\n", INITIATED)
	case COMPLETED:
		color.Green("%s\n", COMPLETED)
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("analyze schema report summary: load migration status record: %s", err)
	}
	if msr.FallbackEnabled {
		reportFallBackStatus()
	} else if msr.FallForwardEnabled {
		reportFallForwardStatus()
	}
}

func reportFallBackStatus() {
	status := getFallBackStatus()
	fmt.Printf("cutover to source status: ")
	switch status {
	case NOT_INITIATED:
		color.Red("%s\n", status)
	case INITIATED:
		color.Yellow("%s\n", status)
	case COMPLETED:
		color.Green("%s\n", COMPLETED)
	}
}

func reportFallForwardStatus() {
	status := getFallForwardStatus()
	fmt.Printf("cutover to source-replica status: ")
	switch status {
	case NOT_INITIATED:
		color.Red("%s\n", status)
	case INITIATED:
		color.Yellow("%s\n", status)
	case COMPLETED:
		color.Green("%s\n", COMPLETED)
	}
}
