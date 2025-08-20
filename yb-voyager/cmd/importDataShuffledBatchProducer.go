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
	"fmt"
	"math/rand"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ShuffledBatchProducer wraps FileBatchProducer to provide concurrent batch production
// with random selection. It implements the BatchProducer interface.
type ShuffledBatchProducer struct {
	fileBatchProducer *FileBatchProducer
	batches           []*Batch
	mu                sync.Mutex
	producerFinished  bool
}

func NewShuffledBatchProducer(task *ImportFileTask, state *ImportDataState,
	errorHandler importdata.ImportDataErrorHandler) (*ShuffledBatchProducer, error) {

	fileBatchProducer, err := NewFileBatchProducer(task, state, errorHandler)
	if err != nil {
		return nil, err
	}

	return &ShuffledBatchProducer{
		fileBatchProducer: fileBatchProducer,
		batches:           make([]*Batch, 0),
	}, nil
}

// Init starts the producer goroutine that continuously produces batches from the underlying FileBatchProducer.
func (sbp *ShuffledBatchProducer) Init() {
	log.Infof("Starting shuffled batch producer for file: %s", sbp.fileBatchProducer.task.FilePath)

	// Start producer goroutine
	go func() {
		for {
			sbp.mu.Lock()
			if sbp.fileBatchProducer.Done() {
				sbp.producerFinished = true
				sbp.mu.Unlock()
				log.Infof("Producer finished for file: %s", sbp.fileBatchProducer.task.FilePath)
				break
			}
			batch, err := sbp.fileBatchProducer.NextBatch()
			if err != nil {
				sbp.mu.Unlock()
				utils.ErrExit("Producer error for file %s: %v", sbp.fileBatchProducer.task.FilePath, err)
			}
			sbp.batches = append(sbp.batches, batch)
			sbp.mu.Unlock()
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
// If no batches are available and the producer has finished, returns an error.
func (sbp *ShuffledBatchProducer) NextBatch() (*Batch, error) {
	sbp.mu.Lock()
	defer sbp.mu.Unlock()

	if len(sbp.batches) == 0 {
		if sbp.producerFinished {
			return nil, fmt.Errorf("no more batches available")
		}
		// Could add timeout/wait logic here if needed
		return nil, fmt.Errorf("no batches available yet")
	}

	// Pick random batch
	idx := rand.Intn(len(sbp.batches))
	batch := sbp.batches[idx]
	sbp.batches = append(sbp.batches[:idx], sbp.batches[idx+1:]...)

	log.Debugf("Returning batch %d from shuffled producer for file: %s", batch.Number, sbp.fileBatchProducer.task.FilePath)
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
