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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/segmentcleanup"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	cleanupPolicy               string
	cleanupArchiveDir           string
	cleanupUtilizationThreshold int
)

var segmentCleanupCmd = &cobra.Command{
	Use: "changes",
	Short: "Archive or clean up processed CDC event queue segments during live migration.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/cutover-archive/archive-changes/",
	Long: fmt.Sprintf(`Manage the lifecycle of processed CDC event segments during live migration.

This command handles old CDC events that have already been applied to the
destination database. It supports three policies to control what happens
to processed segments:

  delete  — delete processed segments once fs utilization exceeds the threshold.
 archive — copy processed segments to --archive-dir, then delete the originals.

The --policy flag is required. Accepted values: %s`, strings.Join(segmentcleanup.ValidPolicyNames, ", ")),

	PreRun: func(cmd *cobra.Command, args []string) {
		validateSegmentCleanupFlags(cmd)
	},

	Run: segmentCleanupCommandFn,
}

func segmentCleanupCommandFn(cmd *cobra.Command, args []string) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	if msr == nil {
		utils.ErrExit("migration status record not found; ensure export has been initiated before running archive changes")
	}

	if msr.IsIteration() {
		utils.PrintAndLogf("Archive changes command should only be run on the main export directory '%s', not on iterations", utils.Path.Sprint(msr.GetParentExportDir(exportDir)))
		if !utils.AskPrompt(fmt.Sprintf("Are you sure you want to run the command on the iteration %d?", msr.IterationNo)) {
			utils.ErrExit("Aborting. Run the command on the main export directory '%s' instead.", utils.Path.Sprint(msr.GetParentExportDir(exportDir)))
		}
	}

	if !msr.ExportDataSourceDebeziumStarted {
		utils.ErrExit("the streaming phase of export data has not started yet — archive changes can only be run after streaming begins")
	}

	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ArchivingEnabled = true
		record.SegmentCleanupRunning = true
	})
	resetSegmentCleanupRunning := func() {
		metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.SegmentCleanupRunning = false
		})
	}
	defer resetSegmentCleanupRunning()

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Also register as an atexit handler so the flag is cleared when
	// utils.ErrExit calls atexit.Exit, which bypasses Go defers.
	atexit.Register(resetSegmentCleanupRunning)

	cfg := segmentcleanup.Config{
		Policy:                 cleanupPolicy,
		ExportDir:              exportDir,
		ArchiveDir:             cleanupArchiveDir,
		FSUtilizationThreshold: cleanupUtilizationThreshold,
	}

	cleaner := segmentcleanup.NewSegmentCleaner(cfg, metaDB)

	go waitForWorkflowEnd(cleaner, ctx)

	if err := cleaner.Run(); err != nil {
		utils.ErrExit("archive changes failed: %v", err)
	}
	msr, err = metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	if msr.RestartDataMigrationSourceTargetNextIteration {
		utils.PrintAndLogfSuccess("\nArchived all the changes for the iteration 0.")
	} else {
		utils.PrintAndLogfSuccess("\nArchived all the changes.")
		return
	}

	lastArchivedIteration := 0
	currentDB := metaDB
	for {

		if StopArchiverSignal {
			//if end migration signal is received, then break the loop
			break
		}

		msr, err = currentDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error getting migration status record: %v", err)
		}

		if !msr.RestartDataMigrationSourceTargetNextIteration {
			break
		}

		if !msr.NextIterationInitialized {
			time.Sleep(5 * time.Second)
			continue
		}

		nextIteration := lastArchivedIteration + 1
		parentMSR, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error getting parent migration status record: %v", err)
		}
		iterationsExportDir := parentMSR.GetIterationsDir(exportDir)
		iterationExportDir := GetIterationExportDir(iterationsExportDir, nextIteration)

		iterationMetaDB, err := metadb.NewMetaDB(iterationExportDir)
		if err != nil {
			utils.ErrExit("error opening iteration %d meta db: %v", nextIteration, err)
		}

		var iterationArchiveDir string
		if cleanupArchiveDir != "" {
			iterationArchiveDir = filepath.Join(cleanupArchiveDir,
				"live-data-migration-iterations",
				fmt.Sprintf("live-data-migration-iteration-%d", nextIteration))
			err = os.MkdirAll(iterationArchiveDir, 0755)
			if err != nil {
				utils.ErrExit("creating archive directory: %v", err)
			}
		}

		cfg = segmentcleanup.Config{
			Policy:                 cleanupPolicy,
			ExportDir:              iterationExportDir,
			ArchiveDir:             iterationArchiveDir,
			FSUtilizationThreshold: cleanupUtilizationThreshold,
		}

		cleaner = segmentcleanup.NewSegmentCleaner(cfg, iterationMetaDB)

		go waitForWorkflowEndForDB(cleaner, ctx, iterationMetaDB, iterationExportDir)

		if err := cleaner.Run(); err != nil {
			utils.ErrExit("archive changes failed for iteration %d: %v", nextIteration, err)
		}
		utils.PrintAndLogfSuccess("\nArchived all the changes for iteration %d.", nextIteration)

		lastArchivedIteration = nextIteration
		currentDB = iterationMetaDB
	}

	utils.PrintAndLogfSuccess("\nArchived all the changes for all iterations.")
}

func workflowEndedForDB(db *metadb.MetaDB, dir string) bool {
	if StopArchiverSignal {
		return true
	}
	msr, err := db.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	if getCutoverStatus(db) != COMPLETED {
		return false
	}
	switch true {
	case msr.FallbackEnabled:
		return getCutoverToSourceStatus(dir, db) == COMPLETED
	case msr.FallForwardEnabled:
		return getCutoverToSourceReplicaStatus(db) == COMPLETED
	}
	return true
}

func waitForWorkflowEnd(cleaner *segmentcleanup.SegmentCleaner, ctx context.Context) {
	waitForWorkflowEndForDB(cleaner, ctx, metaDB, exportDir)
}

func waitForWorkflowEndForDB(cleaner *segmentcleanup.SegmentCleaner, ctx context.Context, db *metadb.MetaDB, dir string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if workflowEndedForDB(db, dir) {
				cleaner.SignalStop()
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func init() {
	archiveCmd.AddCommand(segmentCleanupCmd)
	registerCommonGlobalFlags(segmentCleanupCmd)

	segmentCleanupCmd.Flags().StringVar(&cleanupPolicy, "policy", "",
		fmt.Sprintf("cleanup policy for processed segments; accepted values: %s (required)", strings.Join(segmentcleanup.ValidPolicyNames, ", ")))
	segmentCleanupCmd.MarkFlagRequired("policy")
	segmentCleanupCmd.MarkPersistentFlagRequired("export-dir")

	segmentCleanupCmd.Flags().StringVar(&cleanupArchiveDir, "archive-dir", "",
		"directory to archive processed segments to (required when --policy=archive)")

	segmentCleanupCmd.Flags().IntVar(&cleanupUtilizationThreshold, "fs-utilization-threshold", 70,
		"disk utilization percentage above which cleanup actions are triggered (used with --policy=delete)")
}

func validateSegmentCleanupFlags(cmd *cobra.Command) {
	if !segmentcleanup.IsValidPolicy(cleanupPolicy) {
		utils.ErrExit("invalid --policy %q: must be one of %s", cleanupPolicy, strings.Join(segmentcleanup.ValidPolicyNames, ", "))
	}
	if cleanupPolicy == segmentcleanup.PolicyArchive && cleanupArchiveDir == "" {
		utils.ErrExit("--archive-dir is required when --policy=archive")
	}
	if cleanupPolicy != segmentcleanup.PolicyArchive && cleanupArchiveDir != "" {
		utils.ErrExit("--archive-dir can only be used with --policy=archive")
	}
	if cleanupArchiveDir != "" && !utils.FileOrFolderExists(cleanupArchiveDir) {
		utils.ErrExit("archive directory %q does not exist", cleanupArchiveDir)
	}
	if cleanupPolicy == segmentcleanup.PolicyArchive && cmd.Flags().Changed("fs-utilization-threshold") {
		utils.ErrExit("--fs-utilization-threshold cannot be used with --policy=archive")
	}
}
