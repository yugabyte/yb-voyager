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
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var NUM_EVENT_CHANNELS int
var EVENT_CHANNEL_SIZE int // has to be > MAX_EVENTS_PER_BATCH
var MAX_EVENTS_PER_BATCH int
var MAX_INTERVAL_BETWEEN_BATCHES int //ms
var END_OF_QUEUE_SEGMENT_EVENT = &tgtdb.Event{Op: "end_of_source_queue_segment"}

func init() {
	NUM_EVENT_CHANNELS = utils.GetEnvAsInt("NUM_EVENT_CHANNELS", 512)
	EVENT_CHANNEL_SIZE = utils.GetEnvAsInt("EVENT_CHANNEL_SIZE", 2000)
	MAX_EVENTS_PER_BATCH = utils.GetEnvAsInt("MAX_EVENTS_PER_BATCH", 2000)
	MAX_INTERVAL_BETWEEN_BATCHES = utils.GetEnvAsInt("MAX_INTERVAL_BETWEEN_BATCHES", 2000)
}

func streamChanges() error {
	eventChannelsMetaInfo, err := tdb.GetEventChannelsMetaInfo()
	if err != nil {
		return fmt.Errorf("failed to fetch event channel meta info from target : %w", err)
	}

	eventQueue := NewEventQueue(exportDir)
	// setup target event channels
	var evChans []chan *tgtdb.Event
	var processingDoneChans []chan bool
	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		evChans = append(evChans, make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE))
		processingDoneChans = append(processingDoneChans, make(chan bool, 1))
	}

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

		err = streamChangesFromSegment(segment, evChans, processingDoneChans, eventChannelsMetaInfo)
		if err != nil {
			return fmt.Errorf("error streaming changes for segment %s: %v", segment.FilePath, err)
		}
	}
}

func streamChangesFromSegment(segment *EventQueueSegment, evChans []chan *tgtdb.Event, processingDoneChans []chan bool, eventChannelsMetaInfo map[int]map[string]interface{}) error {
	err := segment.Open()
	if err != nil {
		return err
	}
	defer segment.Close()

	// start target event channel processors
	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		var chanLastAppliedVsn int64 = -1 // default to allow all events
		chanMetaInfo, exists := eventChannelsMetaInfo[i]
		if exists {
			lastAppliedVsn, lastAppliedVsnExists := chanMetaInfo["lastAppliedVsn"]
			if lastAppliedVsnExists {
				chanLastAppliedVsn = lastAppliedVsn.(int64)
			}
		}
		go processEvents(i, evChans[i], chanLastAppliedVsn, processingDoneChans[i])
	}

	log.Infof("streaming changes for segment %s", segment.FilePath)
	for !segment.IsProcessed() {
		event, err := segment.NextEvent()
		if err != nil {
			return err
		}

		if event == nil && segment.IsProcessed() {
			break
		}

		err = handleEvent(event, evChans)
		if err != nil {
			return fmt.Errorf("error handling event: %v", err)
		}
	}

	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		evChans[i] <- END_OF_QUEUE_SEGMENT_EVENT
	}

	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		<-processingDoneChans[i]
	}

	// TODO: printing this line until some user stats are available.
	utils.PrintAndLog("finished streaming changes from segment %s\n", filepath.Base(segment.FilePath))
	return nil
}

func handleEvent(event *tgtdb.Event, evChans []chan *tgtdb.Event) error {
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

	h := hashEvent(event)
	evChans[h] <- event
	return nil
}

// Returns a hash value between 0..NUM_EVENT_CHANNELS
func hashEvent(e *tgtdb.Event) int {
	hash := fnv.New64a()
	hash.Write([]byte(e.SchemaName + e.TableName))

	keyColumns := make([]string, 0)
	for k := range e.Key {
		keyColumns = append(keyColumns, k)
	}

	// sort to ensure input to hash is consistent.
	sort.Strings(keyColumns)
	for _, k := range keyColumns {
		hash.Write([]byte(*e.Key[k]))
	}
	return int(hash.Sum64() % (uint64(NUM_EVENT_CHANNELS)))
}

func processEvents(chanNo int, evChan chan *tgtdb.Event, lastAppliedVsn int64, done chan bool) {
	endOfProcessing := false
	for !endOfProcessing {
		batch := []*tgtdb.Event{}
		timer := time.NewTimer(time.Duration(MAX_INTERVAL_BETWEEN_BATCHES) * time.Millisecond)
	Batching:
		for {
			// read from channel until MAX_EVENTS_PER_BATCH or MAX_INTERVAL_BETWEEN_BATCHES
			select {
			case event := <-evChan:
				if event == END_OF_QUEUE_SEGMENT_EVENT {
					endOfProcessing = true
					break Batching
				}
				if event.Vsn <= lastAppliedVsn {
					utils.PrintAndLog("ignoreing event %v as vsn <= %v", event, lastAppliedVsn)
					continue
				}
				batch = append(batch, event)
				if len(batch) >= MAX_EVENTS_PER_BATCH {
					break Batching
				}
			case <-timer.C:
				break Batching
			}
		}
		timer.Stop()

		if len(batch) == 0 {
			continue
		}
		err := tdb.ExecuteBatch(batch)
		if err != nil {
			utils.ErrExit("error executing batch: %v", err)
		}
		log.Debugf("processEvents from channel %v: Executed Batch of size - %v", chanNo, len(batch))
	}
	done <- true
}
