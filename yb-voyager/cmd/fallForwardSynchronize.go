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

import "github.com/spf13/cobra"

var fallForwardSynchronizeCmd = &cobra.Command{
	Use:   "synchronize",
	Short: "fall-forward synchronize",
	Long:  `fall-forward synchronize`,

	Run: func(cmd *cobra.Command, args []string) {
		source.DBType = YUGABYTEDB
		exportType = CHANGES_ONLY
		exportDataCmd.PreRun(cmd, args)
		exportDataCmd.Run(cmd, args)
	},
}

func init() {
	fallForwardCmd.AddCommand(fallForwardSynchronizeCmd)
	registerCommonGlobalFlags(fallForwardSynchronizeCmd)
	registerCommonExportFlags(fallForwardSynchronizeCmd)
	registerExportDataFlags(fallForwardSynchronizeCmd)
	hideFlags(fallForwardSynchronizeCmd)
}
