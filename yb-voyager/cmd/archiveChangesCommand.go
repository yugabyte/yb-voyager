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
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var StopArchiverSignal bool
var moveDestination string
var deleteSegments utils.BoolStr
var utilizationThreshold int

func archiveChangesCommandFn(cmd *cobra.Command, args []string) {
	if moveDestination != "" && deleteSegments {
		utils.ErrExit("only one of the --move-to and --delete-changes-without-archiving should be set")
	}
	if moveDestination == "" && !deleteSegments {
		utils.ErrExit("one of the --move-to and --delete-changes-without-archiving must be set")
	}

	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ArchivingEnabled = true
	})

	copier := NewEventSegmentCopier(moveDestination)
	deleter := NewEventSegmentDeleter(utilizationThreshold)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := copier.Run()
		if err != nil {
			utils.ErrExit("copying segments: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := deleter.Run()
		if err != nil {
			utils.ErrExit("deleting segments: %v", err)
		}
	}()

	wg.Wait()
	if StopArchiverSignal {
		utils.PrintAndLog("\n\nReceived signal to terminate due to end migration command.\nArchiving changes completed. Exiting...")
	}
}

func init() {
	rootCmd.AddCommand(archiveChangesCmd)
	registerArchiveChangesFlags(archiveChangesCmd)
}

func registerArchiveChangesFlags(cmd *cobra.Command) {
	registerCommonGlobalFlags(cmd)

	cmd.Flags().StringVar(&moveDestination, "move-to", "",
		"Path to the directory where the imported change events are to be moved to. "+
			"Note that, the changes are deleted from the export-dir only after the disk utilisation exceeds 70%.")

	BoolVar(cmd.Flags(), &deleteSegments, "delete-changes-without-archiving", false,
		"Delete the imported changes without archiving them. Note that: the changes are deleted from the export-dir only after disk utilisation exceeds 70%.")

	cmd.Flags().IntVar(&utilizationThreshold, "fs-utilization-threshold", 70,
		"disk utilization threshold in percentage")
}

func validateCommonArchiveFlags() {
	validateMoveToFlag()
}

func validateMoveToFlag() {
	if moveDestination != "" {
		if !utils.FileOrFolderExists(moveDestination) {
			utils.ErrExit("move destination %q doesn't exists.\n", moveDestination)
		} else {
			var err error
			moveDestination, err = filepath.Abs(moveDestination)
			if err != nil {
				utils.ErrExit("Failed to get absolute path for move destination %q: %v\n", moveDestination, err)
			}
			moveDestination = filepath.Clean(moveDestination)
			fmt.Printf("Note: Using %q as move destination\n", moveDestination)
		}
	}
}


///////////////////////////////////////////////////////////////////////////////////////////////////////

type EventSegmentDeleter struct {
	FSUtilisationThreshold int
}

func NewEventSegmentDeleter(fsUtilisationThreshold int) *EventSegmentDeleter {
	return &EventSegmentDeleter{
		FSUtilisationThreshold: fsUtilisationThreshold,
	}
}

func (d *EventSegmentDeleter) getImportCount() (int, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return 0, fmt.Errorf("get migration status record: %v", err)
	}
	if msr == nil {
		return 0, fmt.Errorf("migration status record not found")
	}
	if msr.FallForwardEnabled {
		return 2, nil
	}
	return 1, nil
}

func (d *EventSegmentDeleter) isFSUtilisationExceeded() bool {
	if StopArchiverSignal { // if stop archiver signal is received, then need to archive/delete all segments
		return true
	}
	fsUtilization, err := utils.GetFSUtilizationPercentage(exportDir)
	if err != nil {
		utils.ErrExit("get fs utilization: %v", err)
	}

	return fsUtilization > d.FSUtilisationThreshold
}

func (d *EventSegmentDeleter) deleteSegment(segment utils.Segment) error {
	if utils.FileOrFolderExists(segment.FilePath) {
		err := os.Remove(segment.FilePath)
		if err != nil {
			return fmt.Errorf("delete segment file %s: %v", segment.FilePath, err)
		}
	}
	err := metaDB.MarkSegmentDeleted(segment.Num)
	if err != nil {
		return fmt.Errorf("mark segment %d as deleted: %v", segment.Num, err)
	}
	utils.PrintAndLog("event queue segment file %s deleted", segment.FilePath)
	return nil
}

func (d *EventSegmentDeleter) Run() error {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		if !d.isFSUtilisationExceeded() {
			continue
		}
		importCount, err := d.getImportCount()
		if err != nil {
			return fmt.Errorf("get number of importers for deleter: %w", err)
		}
		segmentsToBeDeleted, err := metaDB.GetSegmentsToBeDeleted()
		if err != nil {
			return fmt.Errorf("get segments to be deleted: %v", err)
		}

		if StopArchiverSignal && len(segmentsToBeDeleted) == 0 {
			pendingSegments, err := metaDB.GetPendingSegments(importCount)
			if err != nil {
				return fmt.Errorf("stop archiver signal to deleter: %v", err)
			}

			if len(pendingSegments) == 0 {
				log.Infof("all the segments are deleted, can proceed to end migration")
				return nil
			}
		}

		for _, segment := range segmentsToBeDeleted {
			err := d.deleteSegment(segment)
			if err != nil {
				return fmt.Errorf("delete segment %s: %v", segment.FilePath, err)
			}
			if !d.isFSUtilisationExceeded() {
				break
			}
		}
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type EventSegmentCopier struct {
	Dest string
}

func NewEventSegmentCopier(dest string) *EventSegmentCopier {
	return &EventSegmentCopier{
		Dest: dest,
	}
}

func (m *EventSegmentCopier) getImportCount() (int, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return 0, fmt.Errorf("get migration status record: %v", err)
	}
	if msr == nil {
		return 0, fmt.Errorf("migration status record not found")
	}
	if msr.FallForwardEnabled {
		return 2, nil
	}
	return 1, nil
}

func (m *EventSegmentCopier) ifExistsDeleteSegmentFileFromArchive(segmentNewPath string) error {
	if utils.FileOrFolderExists(segmentNewPath) {
		err := os.Remove(segmentNewPath)
		if err != nil {
			return fmt.Errorf("delete %s: %w", segmentNewPath, err)
		}
		log.Infof("Deleted existing file %s", segmentNewPath)
	}
	return nil
}

func (m *EventSegmentCopier) copySegmentFile(segment utils.Segment, segmentNewPath string) error {
	sourceFile, err := os.Open(segment.FilePath)
	if err != nil {
		return fmt.Errorf("open segment file %s : %v", segment.FilePath, err)
	}
	defer sourceFile.Close()
	destinationFile, err := os.Create(segmentNewPath)
	if err != nil {
		return fmt.Errorf("create file %s : %v", segmentNewPath, err)
	}
	defer destinationFile.Close()
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("copy file %s : %v", segment.FilePath, err)
	}
	return nil
}

func (m *EventSegmentCopier) Run() error {
	var importCount int
	for {
		newImportCount, err := m.getImportCount()
		if err != nil {
			return fmt.Errorf("get number of importers for copier: %w", err)
		}
		if newImportCount != importCount {
			importCount = newImportCount
			utils.PrintAndLog("Importer count: %d", importCount)
		}

		segmentsToArchive, err := metaDB.GetSegmentsToBeArchived(importCount)
		if err != nil {
			return fmt.Errorf("get segments to be archived: %v", err)
		}

		if StopArchiverSignal && len(segmentsToArchive) == 0 {
			pendingSegments, err := metaDB.GetPendingSegments(importCount)
			if err != nil {
				return fmt.Errorf("stop archiver signal to copier: %v", err)
			}

			if len(pendingSegments) == 0 {
				log.Infof("all the segments are archived, can proceed to end migration")
				return nil
			}
		}

		for _, segment := range segmentsToArchive {
			var segmentNewPath string
			if m.Dest != "" {
				segmentFileName := filepath.Base(segment.FilePath)
				segmentNewPath = fmt.Sprintf("%s/%s", m.Dest, segmentFileName)

				err := m.ifExistsDeleteSegmentFileFromArchive(segmentNewPath)
				if err != nil {
					return fmt.Errorf("delete existing file %s : %v", segmentNewPath, err)
				}

				err = m.copySegmentFile(segment, segmentNewPath)
				if err != nil {
					return fmt.Errorf("copy file %s : %v", segment.FilePath, err)
				}
				utils.PrintAndLog("event queue segment file %s archived to %s", segment.FilePath, segmentNewPath)
			}
			err = metaDB.UpdateSegmentArchiveLocation(segment.Num, segmentNewPath)
			if err != nil {
				return fmt.Errorf("update segment archive location in metaDB for segment %s : %v", segment.FilePath, err)
			}
		}
		time.Sleep(10 * time.Second)
	}
}
