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
	Use:   "changes",
	Short: "This command will archive the streaming data from the source database",
	Long:  `This command will archive the streaming data from the source database.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateCommonArchiveFlags()
	},

	Run: archiveChangesCommandFn,
}

func archiveChangesCommandFn(cmd *cobra.Command, args []string) {
	var err error
	metaDB, err = NewMetaDB(exportDir)
	if err != nil {
		utils.ErrExit("Failed to initialize meta db: %s", err)
	}

	moveToChanged := cmd.Flags().Changed("move-to")
	deleteChanged := cmd.Flags().Changed("delete")
	if (moveToChanged && !deleteChanged) || (!moveToChanged && deleteChanged) {
		copier := NewEventSegmentCopier(moveDestination)
		deleter := NewEventSegmentDeleter(utilizationThreshold)
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := copier.Run()
			if err != nil {
				utils.ErrExit("Error while copying segments: %v", err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := deleter.Run()
			if err != nil {
				utils.ErrExit("Error while deleting segments: %v", err)
			}
		}()

		wg.Wait()
	} else {
		utils.ErrExit("Either move-to or delete flag should be specified")
	}
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

func (d *EventSegmentDeleter) isFSUtilisationExceeded() (bool, error) {
	fsUtilization, err := utils.GetFSUtilizationPercentage(exportDir)
	if err != nil {
		return false, fmt.Errorf("error while getting fs utilization: %v", err)
	}

	log.Infof("FS Utilisation: %d%%", fsUtilization)
	return fsUtilization > d.FSUtilisationThreshold, nil
}

func (d *EventSegmentDeleter) deleteSegments() error {
	segmentsToBeDeleted, err := metaDB.GetSegmentsToBeDeleted()
	if err != nil {
		return fmt.Errorf("error while getting segments to be deleted: %v", err)
	}
	for _, segment := range segmentsToBeDeleted {
		if utils.FileOrFolderExists(segment.SegmentFilePath) {
			err := os.Remove(segment.SegmentFilePath)
			if err != nil {
				return fmt.Errorf("error while deleting segment file %s: %v", segment.SegmentFilePath, err)
			}
		}
		err := metaDB.MarkSegmentDeleted(segment.SegmentNum)
		if err != nil {
			return fmt.Errorf("error while marking segment %d as deleted: %v", segment.SegmentNum, err)
		}
		log.Infof("Deleted segment file %s", segment.SegmentFilePath)
	}
	return nil
}

func (d *EventSegmentDeleter) Run() error {
	for {
		fsUtilizationExceeded, err := d.isFSUtilisationExceeded()
		if err != nil {
			return fmt.Errorf("error while checking fs utilization: %v", err)
		}
		if fsUtilizationExceeded {
			err := d.deleteSegments()
			if err != nil {
				return fmt.Errorf("error while deleting segments: %v", err)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type EventSegmentCopier struct {
	MoveDestination string
}

type Segment struct {
	SegmentNum      int
	SegmentFilePath string
}

func NewEventSegmentCopier(dest string) *EventSegmentCopier {
	return &EventSegmentCopier{
		MoveDestination: dest,
	}
}

func (m *EventSegmentCopier) getImportCount() (int, error) {
	msr, err := GetMigrationStatusRecord()
	if err != nil {
		return 0, fmt.Errorf("error while getting migration status record: %v", err)
	}
	if msr == nil {
		return 0, fmt.Errorf("migration status record not found")
	}
	if msr.FallForwarDBExists {
		return 2, nil
	}
	return 1, nil
}

func (m *EventSegmentCopier) ifExistsDeleteSegmentFileFromArchive(segmentNewPath string) error {
	if utils.FileOrFolderExists(segmentNewPath) {
		err := os.Remove(segmentNewPath)
		if err != nil {
			return err
		}
		log.Infof("Deleted existing file %s", segmentNewPath)
	}
	return nil
}

func (m *EventSegmentCopier) copySegmentFile(segment Segment, segmentNewPath string) error {
	sourceFile, err := os.Open(segment.SegmentFilePath)
	if err != nil {
		return fmt.Errorf("error while opening segment file %s : %v", segment.SegmentFilePath, err)
	}
	defer sourceFile.Close()
	destinationFile, err := os.Create(segmentNewPath)
	if err != nil {
		return fmt.Errorf("error while creating file %s : %v", segmentNewPath, err)
	}
	defer destinationFile.Close()
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("error while copying file %s : %v", segment.SegmentFilePath, err)
	}
	return nil
}

func (m *EventSegmentCopier) Run() error {
	importCount, err := m.getImportCount()
	if err != nil {
		return fmt.Errorf("error while getting import count: %v", err)
	}
	log.Infof("Import count: %d", importCount)

	for {
		segmentsToArchive, err := metaDB.GetSegmentsToBeArchived(importCount)
		if err != nil {
			return fmt.Errorf("error while getting segments to be archived: %v", err)
		}
		for _, segment := range segmentsToArchive {
			if m.MoveDestination != "" {
				segmentFileName := filepath.Base(segment.SegmentFilePath)
				segmentNewPath := fmt.Sprintf("%s/%s", m.MoveDestination, segmentFileName)

				err := m.ifExistsDeleteSegmentFileFromArchive(segmentNewPath)
				if err != nil {
					return fmt.Errorf("error while deleting existing file %s : %v", segmentNewPath, err)
				}

				err = m.copySegmentFile(segment, segmentNewPath)
				if err != nil {
					return fmt.Errorf("error while copying file %s : %v", segment.SegmentFilePath, err)
				}

				err = metaDB.UpdateSegmentArchiveLocation(segment.SegmentNum, segmentNewPath)
				if err != nil {
					return fmt.Errorf("error while updating segment archive location in metaDB for segment %s : %v", segment.SegmentFilePath, err)
				}
				log.Infof("Copied segment file %s to %s", segment.SegmentFilePath, segmentNewPath)
			}
		}
		time.Sleep(10 * time.Second)
	}
}
