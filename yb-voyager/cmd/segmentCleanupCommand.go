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

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
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

	err = runSegmentCleaner(cfg, metaDB, ctx)
	if err != nil {
		utils.ErrExit("archive changes failed: %v", err)
	}

	packAndSendArchiveChangesPayload(COMPLETE, nil, metaDB, migrationUUID)

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

	currentDB := metaDB
	for {
		msr, err = currentDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error getting migration status record: %v", err)
		}

		if !msr.RestartDataMigrationSourceTargetNextIteration {
			break
		}

		nextIterationMetaDB, nextIterationConfig, nextIterationNum, nextIterationMigrationUUID, err := setupSegmentConfigForNextIteration(currentDB, msr.IterationNo)
		if err != nil {
			utils.ErrExit("error setting up segment config for next iteration: %v", err)
		}
		err = runSegmentCleaner(nextIterationConfig, nextIterationMetaDB, ctx)
		if err != nil {
			utils.ErrExit("archive changes failed for iteration %d: %v", nextIterationNum, err)
		}
		packAndSendArchiveChangesPayload(COMPLETE, nil, nextIterationMetaDB, nextIterationMigrationUUID)
		utils.PrintAndLogfSuccess("\nArchived all the changes for iteration %d.", nextIterationNum)

		currentDB = nextIterationMetaDB

		parentMSR, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("error getting migration status record: %v", err)
		}

		if StopArchiverSignal && parentMSR.LatestIterationNumber == nextIterationNum {
			//if end migration signal is received and no more iterations to process, then break the loop
			break
		}

	}
	utils.PrintAndLogfSuccess("\nArchived all the changes for all iterations.")
}

func runSegmentCleaner(config segmentcleanup.Config, metaDB *metadb.MetaDB, ctx context.Context) error {
	cleaner := segmentcleanup.NewSegmentCleaner(config, metaDB)

	go waitForWorkflowEndForDB(cleaner, ctx, metaDB, config.ExportDir)

	return cleaner.Run()
}

func setupSegmentConfigForNextIteration(currentDB *metadb.MetaDB, lastArchivedIteration int) (*metadb.MetaDB, segmentcleanup.Config, int, uuid.UUID, error) {
	err := waitUntilNextIterationInitialized(currentDB)
	if err != nil {
		return nil, segmentcleanup.Config{}, 0, uuid.Nil, fmt.Errorf("error waiting for next iteration initialized: %w", err)
	}

	nextIteration := lastArchivedIteration + 1
	parentMSR, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, segmentcleanup.Config{}, 0, uuid.Nil, fmt.Errorf("error getting parent migration status record: %w", err)
	}
	iterationsExportDir := parentMSR.GetIterationsDir(exportDir)
	iterationExportDir := GetIterationExportDir(iterationsExportDir, nextIteration)

	iterationMetaDB, err := metadb.NewMetaDB(iterationExportDir)
	if err != nil {
		return nil, segmentcleanup.Config{}, 0, uuid.Nil, fmt.Errorf("error opening iteration %d meta db: %w", nextIteration, err)
	}

	iterationMSR, err := iterationMetaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, segmentcleanup.Config{}, 0, uuid.Nil, fmt.Errorf("error getting migration status record for iteration %d: %w", nextIteration, err)
	}
	iterationMigrationUUID := uuid.MustParse(iterationMSR.MigrationUUID)
	var iterationArchiveDir string
	if cleanupArchiveDir != "" {
		iterationArchiveDir = filepath.Join(cleanupArchiveDir,
			"live-data-migration-iterations",
			fmt.Sprintf("live-data-migration-iteration-%d", nextIteration))
		err = os.MkdirAll(iterationArchiveDir, 0755)
		if err != nil {
			return nil, segmentcleanup.Config{}, 0, uuid.Nil, fmt.Errorf("creating archive directory: %w", err)
		}
	}
	nextIterationConfig := segmentcleanup.Config{
		Policy:                 cleanupPolicy,
		ExportDir:              iterationExportDir,
		ArchiveDir:             iterationArchiveDir,
		FSUtilizationThreshold: cleanupUtilizationThreshold,
	}
	return iterationMetaDB, nextIterationConfig, nextIteration, iterationMigrationUUID, nil
}

func workflowEndedForDB(db *metadb.MetaDB, dir string) bool {
	if StopArchiverSignal {
		return true
	}
	msr, err := db.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	if GetCutoverStatus(db) != COMPLETED {
		return false
	}
	switch true {
	case msr.FallbackEnabled:
		return GetCutoverToSourceStatus(dir, db) == COMPLETED
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

func packAndSendArchiveChangesPayload(status string, errorMsg error, metaDB *metadb.MetaDB, migrationUUID uuid.UUID) {
	if !shouldSendCallhome() {
		return
	}
	payload := createCallhomePayload(migrationUUID)
	payload.MigrationType = LIVE_MIGRATION
	payload.MigrationPhase = ARCHIVE_CHANGES_PHASE

	archiveChangesPayload := callhome.ArchiveChangesPhasePayload{
		PayloadVersion:   callhome.ARCHIVE_CHANGES_CALLHOME_PAYLOAD_VERSION,
		Policy:           cleanupPolicy,
		Error:            callhome.SanitizeErrorMsg(errorMsg, anonymizer),
		ControlPlaneType: getControlPlaneType(),
	}
	if cleanupPolicy == segmentcleanup.PolicyDelete {
		archiveChangesPayload.FSUtilizationThreshold = cleanupUtilizationThreshold
	}

	stats, err := metaDB.GetSegmentCleanupStats()
	if err != nil {
		log.Infof("callhome: error getting segment cleanup stats: %v", err)
	} else {
		archiveChangesPayload.TotalSegments = stats.TotalSegments
		archiveChangesPayload.ArchivedAndDeletedSegments = stats.ArchivedAndDeletedSegments
		archiveChangesPayload.PendingSegments = stats.PendingSegments
	}

	payload.PhasePayload = callhome.MarshalledJsonString(archiveChangesPayload)
	payload.Status = status

	err = callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
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
