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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

func streamChanges() error {
	eventQueue := NewEventQueue(exportDir)
	log.Infof("streaming changes from %s", eventQueue.QueueDirPath)
	for { // continuously get next segments to stream
		segment, err := eventQueue.GetNextSegment()
		if err != nil {
			if segment == nil && errors.Is(err, os.ErrNotExist) {
				time.Sleep(2 * time.Second)
				continue
			}
			return fmt.Errorf("error getting next segment to stream: %v", err)
		}
		log.Infof("got next segment to stream: %v", segment)

		err = streamChangesFromSegment(segment)
		if err != nil {
			return fmt.Errorf("error streaming changes for segment %s: %v", segment.FilePath, err)
		}
	}
}

func streamChangesFromSegment(segment *EventQueueSegment) error {
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

	log.Infof("finished streaming changes from segment %s", segment.FilePath)
	// TODO: printing this line until some user stats are available.
	fmt.Printf("finished streaming changes from segment %s\n", filepath.Base(segment.FilePath))
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
