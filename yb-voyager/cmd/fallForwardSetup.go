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
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var fallForwardSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "This command will set up and import data into fall forward database",
	Long:  `This command connects to the fall forward database using the parameters provided and starts the importing process.`,

	Run: func(cmd *cobra.Command, args []string) {
		importType = SNAPSHOT_AND_CHANGES
		tconf.TargetDBType = ORACLE
		importerRole = FF_DB_IMPORTER_ROLE
		importDataCmd.PreRun(cmd, args)
		importDataCmd.Run(cmd, args)
	},
}

func init() {
	fallForwardCmd.AddCommand(fallForwardSetupCmd)
	registerCommonGlobalFlags(fallForwardSetupCmd)
	registerCommonImportFlags(fallForwardSetupCmd, FF_DB_IMPORTER_ROLE)
	registerImportDataFlags(fallForwardSetupCmd)
	hideFlagsInFallFowardCmds(fallForwardSetupCmd)
}

func updateFallForwarDBExistsInMetaDB() {
	err := UpdateMigrationStatusRecord(func(record *MigrationStatusRecord) {
		record.FallForwarDBExists = true
	})
	if err != nil {
		utils.ErrExit("error while updating fall forward db exists in meta db: %v", err)
	}
}
