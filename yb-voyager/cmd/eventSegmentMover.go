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
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type EventSegmentMover struct {
	MoveDestination string
}

type Segments struct {
	SegmentNum      int
	SegmentFilePath string
}

func NewEventSegmentMover(dest string) *EventSegmentMover {
	return &EventSegmentMover{
		MoveDestination: dest,
	}
}

func (m *EventSegmentMover) getImportCount() (int, error) {
	msr, err := GetMigrationStatusRecord()
	if err != nil {
		return 0, err
	}
	if msr == nil {
		return 0, fmt.Errorf("migration status record not found")
	}
	if msr.FallForwarDBExists {
		return 2, nil
	}
	return 1, nil
}

func (m *EventSegmentMover) Run() {
	importCount, err := m.getImportCount()
	if err != nil {
		utils.ErrExit("error while getting import count: %v", err)
	}
	log.Infof("Import count: %d", importCount)

	for {
		segmentsToArchive := metaDB.GetSegmentsToBeArchived(importCount)

		for _, segment := range segmentsToArchive {
			if m.MoveDestination != "" {
				segmentFileName := filepath.Base(segment.SegmentFilePath)
				segmentNewPath := fmt.Sprintf("%s/%s", m.MoveDestination, segmentFileName)

				if utils.FileOrFolderExists(segmentNewPath) {
					err = os.Remove(segmentNewPath)
					if err != nil {
						utils.ErrExit("error while deleting file: %v", err)
					}
					log.Infof("Deleted existing file %s", segmentNewPath)
				}

				sourceFile, err := os.Open(segment.SegmentFilePath)
				if err != nil {
					utils.ErrExit("error while opening file to archive: %v", err)
				}
				destinationFile, err := os.Create(segmentNewPath)
				if err != nil {
					utils.ErrExit("error while creating file in archive destination: %v", err)
				}
				_, err = io.Copy(destinationFile, sourceFile)
				if err != nil {
					utils.ErrExit("error while copying file to archive destination: %v", err)
				}

				metaDB.UpdateSegmentArchiveLocation(segment.SegmentNum, segmentNewPath)
				sourceFile.Close()
				destinationFile.Close()
				log.Infof("Moved segment file %s to %s", segment.SegmentFilePath, segmentNewPath)
			}
		}
		log.Info("Archiver EventSegmentMover sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}
