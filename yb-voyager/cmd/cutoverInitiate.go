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

var cutoverInitiateCmd = &cobra.Command{
	Use:   "initiate",
	Short: "Initiate cutover to YugabyteDB",
	Long:  `Initiate cutover to YugabyteDB`,

	Run: func(cmd *cobra.Command, args []string) {
		validateExportDirFlag()
		var err error
		metaDB, err = metadb.NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("failed to create metaDB: %w", err)
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
}
