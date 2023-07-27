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
package tgtdb

import (
	"hash/fnv"
	"sync"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var NUM_PARTITIONS = 4

type TargetEventQueue struct {
	numPartitions int
	partitions    []*TargetEventQueuePartition
}

func NewTargetEventQueue() *TargetEventQueue {
	var partitions []*TargetEventQueuePartition
	for i := 0; i < NUM_PARTITIONS; i++ {
		partitions = append(partitions, newTargetEventQueuePartition(i))
	}
	return &TargetEventQueue{numPartitions: NUM_PARTITIONS, partitions: partitions}
}

func (teq *TargetEventQueue) GetNumPartitions() int {
	return teq.numPartitions
}

func (teq *TargetEventQueue) InsertEvent(e *Event) {
	partitionNoToInsert := teq.getPartitionNoForEvent(e)
	teq.partitions[partitionNoToInsert].InsertEvent(e)
	// utils.PrintAndLog("Inserted event %+v into partition %v", e, partitionNoToInsert)
}

func (teq *TargetEventQueue) getPartitionNoForEvent(e *Event) int {
	return int(teq.getEventHash(e) % (uint64(teq.numPartitions)))
}

func (teq *TargetEventQueue) getEventHash(e *Event) uint64 {
	delimiter := "-"
	stringToBeHashed := e.SchemaName + delimiter + e.TableName
	for _, value := range e.Key {
		stringToBeHashed += delimiter + value
	}
	hash := fnv.New64a()
	hash.Write([]byte(stringToBeHashed))
	hashValue := hash.Sum64()
	return hashValue
}

func (teq *TargetEventQueue) GetNextBatchFromPartition(partitionNo int) []*Event {
	return teq.partitions[partitionNo].GetNextBatch()
}

var MAX_EVENTS_PER_BATCH = 10000
var MAX_TIME_PER_BATCH = 2000 //ms
var MAX_BATCHES_IN_QUEUE = 2

type TargetEventQueuePartition struct {
	partitionNo          int
	buffer               *[]*Event
	eventBatchQueue      chan []*Event
	lastBatchCreatedTime int64 //timestamp epoch
	bufferAccess         sync.Mutex
}

func newTargetEventQueuePartition(partitionNo int) *TargetEventQueuePartition {
	eventBatchChannel := make(chan []*Event, MAX_BATCHES_IN_QUEUE)
	newBuffer := make([]*Event, 0)
	teqp := &TargetEventQueuePartition{
		partitionNo:     partitionNo,
		eventBatchQueue: eventBatchChannel,
		buffer:          &newBuffer,
	}
	go teqp.generateBatchIfTimeThresholdMet()
	return teqp
}

func (teqp *TargetEventQueuePartition) InsertEvent(e *Event) {
	teqp.bufferAccess.Lock()
	*teqp.buffer = append(*teqp.buffer, e)
	teqp.generateBatchIfSizeThresholdMet()
	teqp.bufferAccess.Unlock()
}

// TODO: time based batch generation as well.
func (teqp *TargetEventQueuePartition) generateBatchIfSizeThresholdMet() {
	if len(*teqp.buffer) >= MAX_EVENTS_PER_BATCH {
		teqp.generateBatchFromBuffer()
	}
}

func (teqp *TargetEventQueuePartition) generateBatchIfTimeThresholdMet() {
	for {
		time.Sleep(time.Duration(MAX_TIME_PER_BATCH) * time.Millisecond)
		teqp.bufferAccess.Lock()
		if len(*teqp.buffer) == 0 {
			// nothing to batch yet
			continue
		}
		teqp.generateBatchFromBuffer()
		teqp.bufferAccess.Unlock()
	}
}

func (teqp *TargetEventQueuePartition) generateBatchFromBuffer() {
	eventBatch := *teqp.buffer
	teqp.eventBatchQueue <- eventBatch
	newBuffer := make([]*Event, 0)
	teqp.buffer = &newBuffer
	teqp.lastBatchCreatedTime = time.Now().UnixMilli()
	utils.PrintAndLog("partition: %v - Created batch of events %v", teqp.partitionNo, len(eventBatch))
}

func (teqp *TargetEventQueuePartition) GetNextBatch() []*Event {
	select {
	case eventBatch := <-teqp.eventBatchQueue:
		return eventBatch
	default:
		return nil
	}
}
