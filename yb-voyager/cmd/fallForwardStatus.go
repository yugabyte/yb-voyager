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

var fallforwardStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints status of the fall-forward switchover",
	Long:  `Prints status of the fall-forward switchover`,

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		reportFallForwardStatus()
	},
}

func init() {
	fallForwardCmd.AddCommand(fallforwardStatusCmd)
	fallforwardStatusCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
}

func reportFallForwardStatus() {
	status := getFallForwardStatus()
	fmt.Printf("fall-forward status: ")
	switch status {
	case NOT_INITIATED:
		color.Red("%s\n", status)
	case INITIATED:
		color.Yellow("%s\n", status)
	case COMPLETED:
		color.Green("%s\n", COMPLETED)
	}
}
