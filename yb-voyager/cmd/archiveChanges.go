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

var archiveChangesCmd = &cobra.Command{
	Use:   "changes",
	Short: "This command will archive the streaming data from the source database",
	Long:  `This command will archive the streaming data from the source database.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateCommonArchiveFlags()
		var err error
		metaDB, err = NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("Failed to initialize meta db: %s", err)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		mover := NewEventSegmentMover(moveDestination)
		if deleteSegments {
			deleter := NewEventSegmentDeleter(utilizationThreshold)
			go deleter.Run()
		}
		mover.Run()
	},
}

func init() {
	archiveCmd.AddCommand(archiveChangesCmd)
	registerCommonArchiveFlags(archiveChangesCmd)
}
