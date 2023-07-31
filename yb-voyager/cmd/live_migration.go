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
	"hash/fnv"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var NUM_TARGET_EVENT_CHANNELS = 64
var TARGET_EVENT_CHANNEL_SIZE = 2000 // has to be > MAX_EVENTS_PER_BATCH
var MAX_EVENTS_PER_BATCH = 1000
var MAX_TIME_PER_BATCH = 2000 //ms
var END_OF_SOURCE_QUEUE_SEGMENT_EVENT = &tgtdb.Event{Op: "end_of_source_queue_segment"}

func streamChanges() error {
	sourceEventQueue := NewSourceEventQueue(exportDir)
	// setup target channels
	var targetEventChans []chan *tgtdb.Event
	targetEventChans = make([]chan *tgtdb.Event, NUM_TARGET_EVENT_CHANNELS)
	var eventProcessingDoneChans []chan bool
	eventProcessingDoneChans = make([]chan bool, NUM_TARGET_EVENT_CHANNELS)
	// start target event channel processors
	for i := 0; i < NUM_TARGET_EVENT_CHANNELS; i++ {
		go process_events_from_target_channel(targetEventChans[i], eventProcessingDoneChans[i])
	}

	log.Infof("streaming changes from %s", sourceEventQueue.QueueDirPath)
	for { // continuously get next segments to stream
		segment, err := sourceEventQueue.GetNextSegment()
		if err != nil {
			if segment == nil && errors.Is(err, os.ErrNotExist) {
				time.Sleep(2 * time.Second)
				continue
			}
			return fmt.Errorf("error getting next segment to stream: %v", err)
		}
		log.Infof("got next segment to stream: %v", segment)

		err = streamChangesFromSegment(segment, targetEventChans)
		if err != nil {
			return fmt.Errorf("error streaming changes for segment %s: %v", segment.FilePath, err)
		}
	}
}

func streamChangesFromSegment(segment *SourceEventQueueSegment, targetEventChans []chan *tgtdb.Event) error {
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

		err = handleEvent(event, targetEventChans)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
	}

	log.Infof("finished streaming changes from segment %s", segment.FilePath)
	// send end of segment marker on all target channels
	for i := 0; i < NUM_TARGET_EVENT_CHANNELS; i++ {
		targetEventChans[i] <- END_OF_SOURCE_QUEUE_SEGMENT_EVENT
	}

	// TODO: printing this line until some user stats are available.
	fmt.Printf("finished streaming changes from segment %s\n", filepath.Base(segment.FilePath))
	return nil
}

func handleEvent(event *tgtdb.Event, targetEventChans []chan *tgtdb.Event) error {
	log.Debugf("Handling event: %v", event)
	tableName := event.TableName
	if sourceDBType == "postgresql" && event.SchemaName != "public" {
		tableName = event.SchemaName + "." + event.TableName
	}
	// preparing value converters for the streaming mode
	err := valueConverter.ConvertEvent(event, tableName)
	if err != nil {
		return fmt.Errorf("error transforming event key fields: %v", err)
	}
	insertIntoTargetEventChan(event, targetEventChans)
	return nil
}

func insertIntoTargetEventChan(e *tgtdb.Event, targetEventChans []chan *tgtdb.Event) {
	// hash event
	delimiter := "-"
	stringToBeHashed := e.SchemaName + delimiter + e.TableName
	for _, value := range e.Key {
		stringToBeHashed += delimiter + *value
	}
	hash := fnv.New64a()
	hash.Write([]byte(stringToBeHashed))
	hashValue := hash.Sum64()

	// insert into channel
	chanNo := int(hashValue % (uint64(NUM_TARGET_EVENT_CHANNELS)))
	targetEventChans[chanNo] <- e
}

func process_events_from_target_channel(targetEventChan chan *tgtdb.Event, done chan bool) {
	endOfProcessing := false
	for !endOfProcessing {
		batch := []*tgtdb.Event{}
		for {
			// read from channel until MAX_EVENTS_PER_BATCH or MAX_TIME_PER_BATCH
			select {
			case event := <-targetEventChan:
				if event == END_OF_SOURCE_QUEUE_SEGMENT_EVENT {
					endOfProcessing = true
					break
				}
				batch = append(batch, event)
				if len(batch) >= MAX_EVENTS_PER_BATCH {
					break
				}
			case <-time.After(time.Duration(MAX_TIME_PER_BATCH) * time.Millisecond):
				break
			}
		}
		if len(batch) == 0 {
			continue
		}
		// ready to process batch
		err := tdb.ExecuteBatch(batch)
		if err != nil {
			utils.ErrExit("error executing batch: %v", err)
		}
	}
	done <- true
}
