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
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/segmentcleanup"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	cleanupPolicy               string
	cleanupUtilizationThreshold int
)

var segmentCleanupCmd = &cobra.Command{
	Use:   "segmentcleanup",
	Short: "Clean up processed CDC event queue segments",
	Long: fmt.Sprintf(`Manage the lifecycle of processed CDC event segments during live migration.

Supported policies (--policy):
  delete  (default) — delete processed segments once fs utilization exceeds the threshold.
  retain  — keep all segments on disk; emit warnings if the threshold is exceeded.

Policies: %s`, strings.Join(segmentcleanup.ValidPolicyNames, ", ")),

	PreRun: func(cmd *cobra.Command, args []string) {
		validateSegmentCleanupFlags()
	},

	Run: segmentCleanupCommandFn,
}

func segmentCleanupCommandFn(cmd *cobra.Command, args []string) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	if msr == nil {
		utils.ErrExit("migration status record not found; ensure export has been initiated before running segmentcleanup")
	}
	if !msr.ExportDataSourceDebeziumStarted {
		utils.ErrExit("the streaming phase of export data has not started yet — this command can only be run after streaming begins")
	}

	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ArchivingEnabled = true
		record.SegmentCleanupRunning = true
	})
	defer func() {
		metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.SegmentCleanupRunning = false
		})
	}()

	cfg := segmentcleanup.Config{
		Policy:                 cleanupPolicy,
		ExportDir:              exportDir,
		FSUtilizationThreshold: cleanupUtilizationThreshold,
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	cleaner := segmentcleanup.NewSegmentCleaner(cfg, metaDB)
	
	go waitForWorkflowEnd(cleaner, ctx)

	if err := cleaner.Run(); err != nil {
		utils.ErrExit("segment cleanup failed: %v", err)
	}
}

func workflowEnded() bool {
	if StopArchiverSignal {
		//End migration command triggered archive changes to stop
		return true
	}
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error getting migration status record: %v", err)
	}
	switch true {
		case msr.FallbackEnabled:
			return getCutoverToSourceStatus(exportDir, metaDB) == COMPLETED
		case msr.FallForwardEnabled:
			return getCutoverToSourceReplicaStatus(metaDB) == COMPLETED
		default:
			return getCutoverStatus(metaDB) == COMPLETED
	}
	return false
}

func waitForWorkflowEnd(cleaner *segmentcleanup.SegmentCleaner, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if workflowEnded() {
				utils.PrintAndLogfSuccess("\nArchived all the changes.")
				cleaner.SignalStop()
				return
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func init() {
	rootCmd.AddCommand(segmentCleanupCmd)
	registerCommonGlobalFlags(segmentCleanupCmd)

	segmentCleanupCmd.Flags().StringVar(&cleanupPolicy, "policy", segmentcleanup.PolicyDelete,
		fmt.Sprintf("cleanup policy for processed segments (%s)", strings.Join(segmentcleanup.ValidPolicyNames, ", ")))

	segmentCleanupCmd.Flags().IntVar(&cleanupUtilizationThreshold, "fs-utilization-threshold", 70,
		"disk utilization percentage above which cleanup actions are triggered")
}

func validateSegmentCleanupFlags() {
	if !segmentcleanup.IsValidPolicy(cleanupPolicy) {
		utils.ErrExit("invalid --policy %q: must be one of %s", cleanupPolicy, strings.Join(segmentcleanup.ValidPolicyNames, ", "))
	}
}
