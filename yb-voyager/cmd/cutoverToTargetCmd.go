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

var prepareForFallBack utils.BoolStr

var cutoverToTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "Initiate cutover to target DB",
	Long:  `Initiate cutover to target DB`,

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		metaDB, err = metadb.NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("Failed to initialize meta db: %s", err)
		}
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("get migration status record: %v", err)
		}
		if msr == nil {
			utils.ErrExit("migration status record not found")
		}
		if !msr.FallForwardEnabled {
			// --prepare-for-fall-back is mandatory in this case.
			prepareForFallBackSpecified := cmd.Flags().Changed("prepare-for-fall-back")
			if !prepareForFallBackSpecified {
				utils.ErrExit(`missing required flag "--prepare-for-fall-back [yes|no]"`)
			}
		}
		if prepareForFallBack {
			if msr.FallForwardEnabled {
				utils.ErrExit("cannot prepare for fall-back. Fall-forward workflow already enabled.")
			}
		}
		err = InitiateCutover("target", bool(prepareForFallBack))
		if err != nil {
			utils.ErrExit("failed to initiate cutover: %v", err)
		}
	},
}

func init() {
	cutoverToCmd.AddCommand(cutoverToTargetCmd)
	registerExportDirFlag(cutoverToTargetCmd)
	BoolVar(cutoverToTargetCmd.Flags(), &prepareForFallBack, "prepare-for-fall-back", false,
		"prepare for fallback by streaming changes from target DB back to source DB. Not applicable for fall-forward workflow.")
}

func updateFallBackEnabledInMetaDB() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.FallbackEnabled = true
	})
	if err != nil {
		utils.ErrExit("error while updating fall back enabled in meta db: %v", err)
	}
}
