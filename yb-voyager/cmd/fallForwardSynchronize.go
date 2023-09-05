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
	Short: "This command exports the changes from YugabyteDB.",
	Long:  `This command connects to YugabyteDB and exports the changes received by it so that they can be imported into the fall forward database.`,

	Run: func(cmd *cobra.Command, args []string) {
		source.DBType = YUGABYTEDB
		exportType = CHANGES_ONLY
		exporterRole = TARGET_DB_EXPORTER_ROLE
		exportDataCmd.PreRun(cmd, args)
		createTriggerIfNotExists("fallforward.synchronize.started")
		exportDataCmd.Run(cmd, args)
	},
}

func init() {
	fallForwardCmd.AddCommand(fallForwardSynchronizeCmd)
	registerCommonGlobalFlags(fallForwardSynchronizeCmd)
	registerCommonExportFlags(fallForwardSynchronizeCmd)
	registerTargetDBAsSourceConnFlags(fallForwardSynchronizeCmd)
	registerExportDataFlags(fallForwardSynchronizeCmd)
	hideFlagsInFallFowardCmds(fallForwardSynchronizeCmd)
}
