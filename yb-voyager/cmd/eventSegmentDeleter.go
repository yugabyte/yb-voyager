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
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type EventSegmentDeleter struct {
	FSUtilisationThreshold int
}

func NewEventSegmentDeleter(fsUtilisationThreshold int) *EventSegmentDeleter {
	return &EventSegmentDeleter{
		FSUtilisationThreshold: fsUtilisationThreshold,
	}
}

func (d *EventSegmentDeleter) isFSUtilisationExceeded() bool {
	DiskStats, err := utils.GetDiskStats(exportDir)
	if err != nil {
		utils.ErrExit("error while getting disk usage: %v", err)
	}

	usedPercentage := int(DiskStats.Used * 100 / DiskStats.Total)
	fmt.Println("Used percentage: ", usedPercentage)
	return usedPercentage > d.FSUtilisationThreshold
}

func (d *EventSegmentDeleter) deleteSegments() {
	segmentsToBeDeleted := metaDB.GetSegmentsToBeDeleted()
	for _, segment := range segmentsToBeDeleted {
		err := os.Remove(segment.SegmentFilePath)
		if err != nil {
			utils.ErrExit("error while deleting file: %v", err)
		}
		metaDB.UpdateSegmentDeletionStatus(segment.SegmentNum)
		log.Infof("Deleted segment file %s", segment.SegmentFilePath)
	}
}

func (d *EventSegmentDeleter) Run() {
	for {
		if d.isFSUtilisationExceeded() {
			fmt.Println("FS Utilisation exceeded, deleting segments")
			d.deleteSegments()
		}
		time.Sleep(5 * time.Second)
		fmt.Println("Archiver EventSegmentDeleter sleeping for 5 seconds")
		log.Info("Archiver EventSegmentDeleter sleeping for 5 seconds")
	}
}
