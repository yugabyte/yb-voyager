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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var archiveChangesCmd = &cobra.Command{
	Use: "changes",
	Short: "This command archives and deletes the streaming data exported from the source database.\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/cutover-archive/archive-changes/",
	Long: `This command archives and deletes the streaming data exported from the source database. The data is deleted only if the disk utilization threshold is exceeded. Also, if the --move-to flag is used, only the archived data is deleted.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateCommonArchiveFlags()
	},

	Run: archiveChangesCommandFn,
}

func archiveChangesCommandFn(cmd *cobra.Command, args []string) {
	moveToChanged := cmd.Flags().Changed("move-to")
	deleteChanged := cmd.Flags().Changed("delete-changes-without-archiving")
	if moveToChanged == deleteChanged {
		utils.ErrExit("Either --move-to or --delete-changes-without-archiving flag should be set")
	}
	if !moveToChanged && deleteSegments.String() == "false" {
		utils.ErrExit("--delete-changes-without-archiving flag is set to false and --move-to flag is not set. Must set either one of them.")
	}

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
}

func init() {
	archiveCmd.AddCommand(archiveChangesCmd)
	registerCommonArchiveFlags(archiveChangesCmd)
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

func (d *EventSegmentDeleter) isFSUtilisationExceeded() bool {
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
		segmentsToBeDeleted, err := metaDB.GetSegmentsToBeDeleted()
		if err != nil {
			return fmt.Errorf("get segments to be deleted: %v", err)
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
			return fmt.Errorf("get number of importers: %w", err)
		}
		if newImportCount != importCount {
			importCount = newImportCount
			utils.PrintAndLog("Importer count: %d", importCount)
		}

		segmentsToArchive, err := metaDB.GetSegmentsToBeArchived(importCount)
		if err != nil {
			return fmt.Errorf("get segments to be archived: %v", err)
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
