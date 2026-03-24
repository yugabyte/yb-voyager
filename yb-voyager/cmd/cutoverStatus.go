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
	"time"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	NOT_INITIATED = "NOT_INITIATED"
	INITIATED     = "INITIATED"
	COMPLETED     = "COMPLETED"

	DIRECTION_SOURCE_TO_TARGET         = "source → target"
	DIRECTION_TARGET_TO_SOURCE         = "target → source"
	DIRECTION_TARGET_TO_SOURCE_REPLICA = "target → source-replica"
)

type cutoverStatusRow struct {
	Direction   string
	Status      string
	RequestedAt time.Time
	//TODO: Add completed at
}

var cutoverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints status of the cutover to YugabyteDB",
	Long:  `Prints status of the cutover to YugabyteDB`,

	Run: func(cmd *cobra.Command, args []string) {
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error getting migration status record: %s", err)
		}
		if msr == nil {
			utils.ErrExit("migration status record not found; has the migration been started?")
		}

		fmt.Println()

		if !msr.IsParentMigration() || msr.LatestIterationNumber == 0 {
			rows := collectCutoverStatusRows(exportDir, metaDB)
			renderCutoverStatusTable(rows)
			return
		}

		iterationToRows := collectCutoverStatusRowsForAllIterations(exportDir, metaDB)
		for i := 0; i <= msr.LatestIterationNumber; i++ {
			if msr.LatestIterationNumber == i {
				utils.PrintAndLogfPhase("\nIteration %d (current):", i)
			} else {
				utils.PrintAndLogfPhase("\nIteration %d:", i)
			}
			rows := iterationToRows[i]
			renderCutoverStatusTable(rows)
		}
	},
}

// collect cutover status rows for all iterations
func collectCutoverStatusRowsForAllIterations(exportDir string, metaDB *metadb.MetaDB) map[int][]cutoverStatusRow {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %s", err)
	}
	if msr.LatestIterationNumber == 0 {
		return nil
	}
	iterationToRows := make(map[int][]cutoverStatusRow)

	//collect cutover status rows for parent migration
	rows := collectCutoverStatusRows(exportDir, metaDB)
	iterationToRows[0] = rows

	for i := 1; i <= msr.LatestIterationNumber; i++ {
		iterationsDir := msr.GetIterationsDir(exportDir)
		iterationExportDir := GetIterationExportDir(iterationsDir, i)
		iterationMetaDB, err := metadb.NewMetaDB(iterationExportDir)
		if err != nil {
			utils.ErrExit("error getting iteration meta db: %s", err)
		}
		rows := collectCutoverStatusRows(iterationExportDir, iterationMetaDB)
		iterationToRows[i] = rows
	}
	return iterationToRows
}

func init() {
	cutoverRootCmd.AddCommand(cutoverStatusCmd)
	registerExportDirFlag(cutoverStatusCmd)
	registerConfigFileFlag(cutoverStatusCmd)
}

func collectCutoverStatusRows(exportDir string, metaDB *metadb.MetaDB) []cutoverStatusRow {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %s", err)
	}

	var rows []cutoverStatusRow

	toTargetStatus := getCutoverStatus(metaDB)
	rows = append(rows, cutoverStatusRow{
		Direction:   DIRECTION_SOURCE_TO_TARGET,
		Status:      toTargetStatus,
		RequestedAt: msr.CutoverTimings.ToTargetRequestedAt,
	})

	if msr.FallbackEnabled {
		toSourceStatus := getCutoverToSourceStatus(exportDir, metaDB)
		rows = append(rows, cutoverStatusRow{
			Direction:   DIRECTION_TARGET_TO_SOURCE,
			Status:      toSourceStatus,
			RequestedAt: msr.CutoverTimings.ToSourceRequestedAt,
		})
	}

	if msr.FallForwardEnabled {
		toSRStatus := getCutoverToSourceReplicaStatus(metaDB)
		rows = append(rows, cutoverStatusRow{
			Direction:   DIRECTION_TARGET_TO_SOURCE_REPLICA,
			Status:      toSRStatus,
			RequestedAt: msr.CutoverTimings.ToSourceReplicaRequestedAt,
		})
	}

	return rows
}

func renderCutoverStatusTable(rows []cutoverStatusRow) {
	table := uitable.New()
	table.Separator = " | "

	addHeader(table, "CUTOVER DIRECTION", "STATUS", "REQUESTED AT")
	table.AddRow()
	for _, row := range rows {
		requestedAt := "-"
		if row.Status != NOT_INITIATED {
			requestedAt = row.RequestedAt.UTC().Format(time.DateTime + " UTC")
		}
		table.AddRow(row.Direction, colorizeStatus(row.Status), requestedAt)
	}
	fmt.Println(table)
	fmt.Println()
}

func colorizeStatus(status string) string {
	switch status {
	case NOT_INITIATED:
		return color.RedString(status)
	case INITIATED:
		return color.YellowString(status)
	case COMPLETED:
		return color.GreenString(status)
	default:
		return status
	}
}
