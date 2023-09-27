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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var withFallBack bool

var cutoverInitiateCmd = &cobra.Command{
	Use:   "initiate",
	Short: "Initiate cutover to YugabyteDB",
	Long:  `Initiate cutover to YugabyteDB`,

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		var err error
		metaDB, err = metadb.NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("Failed to initialize meta db: %s", err)
		}
		if withFallBack {
			updateFallBackEnabledInMetaDB()
		}
		err = InitiatePrimarySwitch("cutover")
		if err != nil {
			utils.ErrExit("failed to initiate cutover: %v", err)
		}
	},
}

func init() {
	cutoverCmd.AddCommand(cutoverInitiateCmd)
	cutoverInitiateCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
	cutoverInitiateCmd.Flags().BoolVar(&withFallBack, "with-fall-back", false,
		"assume answer as yes for all questions during migration (default false)")
}

func updateFallBackEnabledInMetaDB() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.FallbackEnabled = true
	})
	if err != nil {
		utils.ErrExit("error while updating fall back enabled in meta db: %v", err)
	}
}
