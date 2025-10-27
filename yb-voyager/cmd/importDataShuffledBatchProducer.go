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
	"context"
	"fmt"
	"math/rand"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/sync/semaphore"
)

var (
	parallelBatchProducerSemaphore = semaphore.NewWeighted(int64(5000))
)

// ShuffledBatchProducer wraps FileBatchProducer to provide concurrent batch production
// with random selection. It implements the BatchProducer interface.
type ShuffledBatchProducer struct {
	fileBatchProducer *FileBatchProducer
	batches           []*Batch
	mu                sync.Mutex
	cond              *sync.Cond // Condition variable for waiting
	producerFinished  bool
}

// NewShuffledBatchProducer creates a new ShuffledBatchProducer that wraps a FileBatchProducer.
// The producer goroutine is not started until Init() is called.
func NewShuffledBatchProducer(task *ImportFileTask, state *ImportDataState,
	errorHandler importdata.ImportDataErrorHandler) (*ShuffledBatchProducer, error) {

	fileBatchProducer, err := NewFileBatchProducer(task, state, errorHandler, nil)
	if err != nil {
		return nil, err
	}

	sbp := &ShuffledBatchProducer{
		fileBatchProducer: fileBatchProducer,
		batches:           make([]*Batch, 0),
	}

	// Initialize the condition variable with the mutex
	sbp.cond = sync.NewCond(&sbp.mu)

	return sbp, nil
}

// Init starts the producer goroutine that continuously produces batches from the underlying FileBatchProducer.
func (sbp *ShuffledBatchProducer) Init() {

	// Start producer goroutine
	go func() {
		log.Infof("Starting batch production for file: %s", sbp.fileBatchProducer.task.FilePath)
		ctx := context.Background()
		for {
			err := parallelBatchProducerSemaphore.Acquire(ctx, 1)
			if err != nil {
				utils.ErrExit("Failed to acquire semaphore: %v", err)
			}

			sbp.mu.Lock()
			if sbp.fileBatchProducer.Done() {
				sbp.producerFinished = true
				sbp.mu.Unlock()
				parallelBatchProducerSemaphore.Release(1)
				log.Infof("Producer finished for file: %s", sbp.fileBatchProducer.task.FilePath)
				break
			}
			batch, err := sbp.fileBatchProducer.NextBatch()
			if err != nil {
				sbp.mu.Unlock()
				parallelBatchProducerSemaphore.Release(1)
				utils.ErrExit("Producer error for file %s: %v", sbp.fileBatchProducer.task.FilePath, err)
			}

			sbp.batches = append(sbp.batches, batch)
			if sbp.fileBatchProducer.Done() {
				sbp.producerFinished = true
				log.Infof("Producer finished for file: %s", sbp.fileBatchProducer.task.FilePath)
			}
			sbp.cond.Signal() // Wake up one waiting consumer
			sbp.mu.Unlock()
			parallelBatchProducerSemaphore.Release(1)
		}
	}()
}

// Done returns true if all batches have been produced and consumed.
func (sbp *ShuffledBatchProducer) Done() bool {
	sbp.mu.Lock()
	defer sbp.mu.Unlock()
	return sbp.producerFinished && len(sbp.batches) == 0
}

// NextBatch returns a randomly selected batch from the available batches.
// If no batches are available, it waits until batches become available or the producer finishes.
func (sbp *ShuffledBatchProducer) NextBatch() (*Batch, error) {
	sbp.mu.Lock()
	defer sbp.mu.Unlock()

	// Wait for batches to become available
	for len(sbp.batches) == 0 && !sbp.producerFinished {
		sbp.cond.Wait() // This releases the lock and waits
		// When we wake up, the lock is re-acquired automatically
	}

	if len(sbp.batches) == 0 {
		return nil, fmt.Errorf("no more batches available")
	}

	// Pick random batch
	idx := rand.Intn(len(sbp.batches))
	batch := sbp.batches[idx]
	sbp.batches = append(sbp.batches[:idx], sbp.batches[idx+1:]...)

	log.Infof("Returning batch %d from shuffled producer for file: %s", batch.Number, sbp.fileBatchProducer.task.FilePath)
	return batch, nil
}

// Close cleans up resources used by the ShuffledBatchProducer.
func (sbp *ShuffledBatchProducer) Close() {
	sbp.mu.Lock()
	defer sbp.mu.Unlock()
	if sbp.fileBatchProducer != nil {
		sbp.fileBatchProducer.Close()
	}
}
