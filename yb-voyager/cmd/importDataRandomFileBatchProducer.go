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
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/rand"
)

type RandomBatchProducer struct {
	sequentialFileBatchProducer         BatchProducer
	task                                *ImportFileTask
	sequentiallyProducedBatches         []*Batch
	mu                                  sync.Mutex
	sequentialFileBatchProducerFinished bool
	producerWaitGroup                   sync.WaitGroup
	producerCtxCancel                   context.CancelFunc
}

func NewRandomFileBatchProducer(task *ImportFileTask, state *ImportDataState, errorHandler importdata.ImportDataErrorHandler, progressReporter *ImportDataProgressReporter) (*RandomBatchProducer, error) {
	sequentialFileBatchProducer, err := NewSequentialFileBatchProducer(task, state, errorHandler, progressReporter)
	if err != nil {
		return nil, fmt.Errorf("creating sequential file batch producer: %w", err)
	}

	return newRandomFileBatchProducer(sequentialFileBatchProducer, task), nil
}

// newRandomFileBatchProducer creates a RandomBatchProducer with the provided SequentialFileBatchProducer.
// This unexported function allows tests to inject a testable sequential producer.
func newRandomFileBatchProducer(sequentialFileBatchProducer BatchProducer, task *ImportFileTask) *RandomBatchProducer {
	producerCtx, producerCtxCancel := context.WithCancel(context.Background())
	rbp := &RandomBatchProducer{
		sequentialFileBatchProducer: sequentialFileBatchProducer,
		task:                        task,
		sequentiallyProducedBatches: make([]*Batch, 0),
		producerCtxCancel:           producerCtxCancel,
	}
	rbp.producerWaitGroup.Add(1)

	go func() {
		defer rbp.producerWaitGroup.Done()
		err := rbp.startProducingBatches(producerCtx)
		if err != nil {
			utils.ErrExit("error producing batches for table: %s, file: %s, err: %w", rbp.task.TableNameTup, rbp.task.FilePath, err)
		}
	}()
	return rbp
}

/*
sequential batch producer is done producing all batches, and all batches in memory(metadata) have been consumed (in random order)
*/
func (rbp *RandomBatchProducer) Done() bool {
	rbp.mu.Lock()
	defer rbp.mu.Unlock()
	return rbp.sequentialFileBatchProducerFinished && len(rbp.sequentiallyProducedBatches) == 0
}

// Close cleans up resources used by the RandomBatchProducer.
// TODO: close sequential file batch producer after it's done instead of waiting for Close() to be called
func (rbp *RandomBatchProducer) Close() {
	// stop producer goroutine
	rbp.producerCtxCancel()
	rbp.producerWaitGroup.Wait()

	// close sequential file batch producer
	if rbp.sequentialFileBatchProducer != nil {
		rbp.sequentialFileBatchProducer.Close()
	}
}

func (rbp *RandomBatchProducer) IsBatchAvailable() bool {
	if rbp.Done() {
		return false
	}
	// not done. producer is still be producing batches in parallel
	rbp.mu.Lock()
	defer rbp.mu.Unlock()
	return len(rbp.sequentiallyProducedBatches) > 0
}

func (rbp *RandomBatchProducer) NextBatch() (*Batch, error) {
	rbp.mu.Lock()
	defer rbp.mu.Unlock()

	if len(rbp.sequentiallyProducedBatches) == 0 {
		// this could be either because
		// 1. sequential file batch producer is done
		// 2. sequential file batch producer is not done, and is producing in parallel.
		// It is the responsibility of the caller to deal with both the scenarios, we do not want to block the caller.
		return nil, fmt.Errorf("no batches available")
	}

	// Pick random batch
	idx := rand.Intn(len(rbp.sequentiallyProducedBatches))
	batch := rbp.sequentiallyProducedBatches[idx]
	rbp.sequentiallyProducedBatches = append(rbp.sequentiallyProducedBatches[:idx], rbp.sequentiallyProducedBatches[idx+1:]...)

	log.Infof("Returning batch %d from random batch producer for file: %s", batch.Number, rbp.task.FilePath)
	return batch, nil
}

func (rbp *RandomBatchProducer) startProducingBatches(ctx context.Context) error {
	log.Infof("Starting to produce batches for file: %s", rbp.task.FilePath)
	for !rbp.sequentialFileBatchProducerFinished {
		batch, err := rbp.sequentialFileBatchProducer.NextBatch()
		if err != nil {
			return err
		}

		// critical section - append batch to sequentiallyProducedBatches, update sequentialFileBatchProducerFinished
		rbp.mu.Lock()
		rbp.sequentiallyProducedBatches = append(rbp.sequentiallyProducedBatches, batch)
		if rbp.sequentialFileBatchProducer.Done() {
			rbp.sequentialFileBatchProducerFinished = true
		}
		rbp.mu.Unlock()

		// handle closed ctx
		select {
		case <-ctx.Done():
			log.Infof("Producer context done, stopping production of batches for file: %s", rbp.task.FilePath)
			return nil
		default:
			// continue
		}
	}

	rbp.sequentialFileBatchProducer.Close()
	log.Infof("Sequential file batch producer finished producing all batches for file: %s", rbp.task.FilePath)
	return nil
}
