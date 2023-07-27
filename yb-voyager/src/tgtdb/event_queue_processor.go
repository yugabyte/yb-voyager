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

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

type PartitionedBatchedEventQueue interface {
	GetNextBatchFromPartition(partitionNo int) []*Event
	GetNumPartitions() int
}

type TargetEventQueueConsumer struct {
	tgtDB   TargetDB
	queue   PartitionedBatchedEventQueue
	workers []*TargetEventQueueWorker
}

func NewTargetEventQueueConsumer(tgtDB TargetDB, teq *TargetEventQueue) *TargetEventQueueConsumer {
	teqc := &TargetEventQueueConsumer{tgtDB: tgtDB, queue: teq}
	numPartitions := teq.GetNumPartitions()
	for p := 0; p < numPartitions; p++ {
		worker := NewTargetEventQueueWorker(tgtDB, teq, p, p)
		teqc.workers = append(teqc.workers, worker)
	}
	return teqc
}

func (consumer *TargetEventQueueConsumer) Start() {
	for _, worker := range consumer.workers {
		go worker.Work()
	}
}

// func (processor *TargetEventQueueProcessor) assignPartitionsToWorkers() {
// 	numPartitions := processor.queue.GetNumPartitions()
// 	for p := 0; p < numPartitions; p++ {
// 		assignedWorkerNo := p % processor.numWorkers
// 		processor.workers[assignedWorkerNo].AssignPartition(p)
// 		utils.PrintAndLog("assigning partition %v to worker num %v", p, assignedWorkerNo)
// 	}
// }

type TargetEventQueueWorker struct {
	workerNo    int
	partitionNo int
	tgtDB       TargetDB
	queue       PartitionedBatchedEventQueue
}

func NewTargetEventQueueWorker(tgtDB TargetDB, teq *TargetEventQueue, workerNo int, partitionNo int) *TargetEventQueueWorker {
	return &TargetEventQueueWorker{tgtDB: tgtDB, queue: teq, workerNo: workerNo, partitionNo: partitionNo}
}

// func (worker *TargetEventQueueWorker) AssignPartition(p int) {
// 	worker.partitions = append(worker.partitions, p)
// }

func (worker *TargetEventQueueWorker) Work() {
	for {
		nextEventBatch := worker.queue.GetNextBatchFromPartition(worker.partitionNo)
		if nextEventBatch == nil {
			continue
		}
		utils.PrintAndLog("worker: %v: processing batch from partition %v : %+v", worker.workerNo, worker.partitionNo, nextEventBatch)
		err := worker.tgtDB.ExecuteBatch(nextEventBatch)
		if err != nil {
			utils.ErrExit("error in executing batch on target: %w", err)
		}
	}
}
