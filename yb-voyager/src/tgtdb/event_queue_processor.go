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

type TargetEventQueueProcessor struct {
	tgtDB      TargetDB
	queue      PartitionedBatchedEventQueue
	numWorkers int
	workers    []*TargetEventQueueWorker
}

func NewTargetEventQueueProcessor(tgtDB TargetDB, teq *TargetEventQueue, numWorkers int) *TargetEventQueueProcessor {
	teqp := &TargetEventQueueProcessor{tgtDB: tgtDB, queue: teq, numWorkers: numWorkers}
	for i := 0; i < numWorkers; i++ {
		worker := NewTargetEventQueueWorker(tgtDB, teq, i)
		teqp.workers = append(teqp.workers, worker)
	}
	return teqp
}

func (processor *TargetEventQueueProcessor) Start() {
	processor.assignPartitionsToWorkers()
	for _, worker := range processor.workers {
		go worker.Work()
	}
}

func (processor *TargetEventQueueProcessor) assignPartitionsToWorkers() {
	numPartitions := processor.queue.GetNumPartitions()
	for p := 0; p < numPartitions; p++ {
		assignedWorkerNo := p % processor.numWorkers
		processor.workers[assignedWorkerNo].AssignPartition(p)
		utils.PrintAndLog("assigning partition %v to worker num %v", p, assignedWorkerNo)
	}
}

type TargetEventQueueWorker struct {
	workerNo   int
	partitions []int
	tgtDB      TargetDB
	queue      PartitionedBatchedEventQueue
}

func NewTargetEventQueueWorker(tgtDB TargetDB, teq *TargetEventQueue, workerNo int) *TargetEventQueueWorker {
	return &TargetEventQueueWorker{tgtDB: tgtDB, queue: teq, workerNo: workerNo}
}

func (worker *TargetEventQueueWorker) AssignPartition(p int) {
	worker.partitions = append(worker.partitions, p)
}

func (worker *TargetEventQueueWorker) Work() {
	for {
		for _, partition := range worker.partitions {
			nextEventBatch := worker.queue.GetNextBatchFromPartition(partition)
			if nextEventBatch == nil {
				continue
			}
			utils.PrintAndLog("worker: %v: processing batch from partition %v : %+v", worker.workerNo, partition, nextEventBatch)
			err := worker.tgtDB.ExecuteBatch(nextEventBatch)
			if err != nil {
				utils.ErrExit("error in executing batch on target: %w", err)
			}
		}
	}
}
