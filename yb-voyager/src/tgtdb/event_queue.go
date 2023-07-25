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
	log "github.com/sirupsen/logrus"
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
	log.Debugf("Inserted event %v into partition %v", e, partitionNoToInsert)
}

func (teq *TargetEventQueue) getPartitionNoForEvent(e *Event) int {
	// TODO: use a hash
	return 0
}

func (teq *TargetEventQueue) GetNextBatchFromPartition(partitionNo int) []*Event {
	return teq.partitions[partitionNo].GetNextBatch()
}

var MAX_EVENTS_PER_BATCH = 1

type TargetEventQueuePartition struct {
	partitionNo     int
	buffer          *[]*Event
	eventBatchQueue chan []*Event
}

func newTargetEventQueuePartition(partitionNo int) *TargetEventQueuePartition {
	eventBatchChannel := make(chan []*Event)
	return &TargetEventQueuePartition{
		partitionNo:     partitionNo,
		eventBatchQueue: eventBatchChannel,
	}
}

func (teqp *TargetEventQueuePartition) InsertEvent(e *Event) {
	*teqp.buffer = (append(*teqp.buffer, e))
}

func (teqp *TargetEventQueuePartition) generateBatchFromBufferIfRequired(e *Event) {
	if len(*teqp.buffer) >= MAX_EVENTS_PER_BATCH {
		// generate batch from buffer
		// TODO: create a concrete struct for an event batch
		eventBatch := *teqp.buffer
		teqp.eventBatchQueue <- eventBatch
		newBuffer := make([]*Event, 0)
		teqp.buffer = &newBuffer
		log.Debugf("Created batch of events %v", eventBatch)
	}
}

func (teqp *TargetEventQueuePartition) GetNextBatch() []*Event {
	// get from channel
	return <-teqp.eventBatchQueue
}
