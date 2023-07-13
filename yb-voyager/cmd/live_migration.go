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

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func streamChanges() error {
	eventQueue := tgtdb.NewEventQueue(exportDir)
	log.Infof("Streaming changes from %s", eventQueue.QueueDirPath)
	for { // continuously get next segments to stream
		segments, err := eventQueue.GetNextSegments()
		log.Infof("got %d segments to stream", len(segments))
		if err != nil {
			return err
		}

		if segments == nil {
			utils.PrintAndLog("no segments to stream. Sleeping for 2 second.")
			time.Sleep(2 * time.Second)
			continue
		}

		for _, segment := range segments {
			err := streamChangesForSegment(segment)
			if err != nil {
				return fmt.Errorf("error streaming changes for segment %s: %v", segment.FilePath, err)
			}
		}
	}
}

func streamChangesForSegment(segment *tgtdb.EventQueueSegment) error {
	err := segment.Open()
	if err != nil {
		return err
	}
	defer segment.Close()
	log.Infof("streaming changes for segment %s", segment.FilePath)
	for !segment.IsProcessed() {
		event, err := segment.NextEvent()
		if err != nil {
			return err
		}

		if event == nil && segment.IsProcessed() {
			break
		}

		err = handleEvent(event)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
	}

	err = segment.MarkProcessed()
	if err != nil {
		return fmt.Errorf("error marking segment %s as processed: %v", segment.FilePath, err)
	}
	log.Infof("segment %s is completed and marked as processed", segment.FilePath)
	return nil
}

func handleEvent(event *tgtdb.Event) error {
	log.Debugf("Handling event: %v", event)

	// TODO: Convert values in the event to make it suitable for target DB.
	batch := []*tgtdb.Event{event}
	err := tdb.ExecuteBatch(batch)
	if err != nil {
		return fmt.Errorf("error executing batch: %v", err)
	}
	return nil
}
