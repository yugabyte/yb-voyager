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

import log "github.com/sirupsen/logrus"

type PartitionedBatchedEventQueue interface {
	GetNextBatchFromPartition(partitionNo int) []*Event
	GetNumPartitions() int
}

type TargetEventQueueProcessor struct {
	tgtDB TargetDB
	queue PartitionedBatchedEventQueue
}

func NewTargetEventQueueProcessor(tgtDB TargetDB, teq *TargetEventQueue) *TargetEventQueueProcessor {
	return &TargetEventQueueProcessor{tgtDB: tgtDB, queue: teq}
}

func (processor *TargetEventQueueProcessor) Start() {
	go func() {
		// TODO: parallelize
		for p := 0; p < NUM_PARTITIONS; p++ {
			nextEventBatch := processor.queue.GetNextBatchFromPartition(p)
			log.Debugf("processing batch from partition %v : %v", p, nextEventBatch)
			processor.tgtDB.ExecuteBatch(nextEventBatch)
		}
	}()
}
