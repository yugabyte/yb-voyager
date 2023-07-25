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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var NUM_PARTITIONS = 1

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
	utils.PrintAndLog("Inserted event %v into partition %v", e, partitionNoToInsert)
}

func (teq *TargetEventQueue) getPartitionNoForEvent(e *Event) int {
	// TODO: use a hash on the event.
	return 0
}

func (teq *TargetEventQueue) GetNextBatchFromPartition(partitionNo int) []*Event {
	return teq.partitions[partitionNo].GetNextBatch()
}

var MAX_EVENTS_PER_BATCH = 2
var MAX_BATCHES_IN_QUEUE = 100

type TargetEventQueuePartition struct {
	partitionNo     int
	buffer          *[]*Event
	eventBatchQueue chan []*Event
}

func newTargetEventQueuePartition(partitionNo int) *TargetEventQueuePartition {
	eventBatchChannel := make(chan []*Event, MAX_BATCHES_IN_QUEUE)
	newBuffer := make([]*Event, 0)
	return &TargetEventQueuePartition{
		partitionNo:     partitionNo,
		eventBatchQueue: eventBatchChannel,
		buffer:          &newBuffer,
	}
}

func (teqp *TargetEventQueuePartition) InsertEvent(e *Event) {
	utils.PrintAndLog("inserting into partition %v, event=%v", teqp.partitionNo, e)
	*teqp.buffer = append(*teqp.buffer, e)
	// utils.PrintAndLog("inserted into partition buffer, now of length=%v", len(*teqp.buffer))
	teqp.generateBatchFromBufferIfRequired()
}

// TODO: time based batch generation as well.
func (teqp *TargetEventQueuePartition) generateBatchFromBufferIfRequired() {
	// utils.PrintAndLog("generating batch: len buffer = %v", len(*teqp.buffer))
	if len(*teqp.buffer) >= MAX_EVENTS_PER_BATCH {
		// utils.PrintAndLog("Creating batch of events")
		// generate batch from buffer
		// TODO: create a concrete struct for an event batch
		eventBatch := *teqp.buffer
		teqp.eventBatchQueue <- eventBatch
		newBuffer := make([]*Event, 0)
		teqp.buffer = &newBuffer
		utils.PrintAndLog("Created batch of events %v", eventBatch)
	} else {
		utils.PrintAndLog("Not reached batch limit yet =%v", len(*teqp.buffer))
	}
}

func (teqp *TargetEventQueuePartition) GetNextBatch() []*Event {
	// get from channel
	return <-teqp.eventBatchQueue
}
