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

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type FileBatchProducer struct {
	task  *ImportFileTask
	state *ImportDataState

	pendingBatches  []*Batch
	lastBatchNumber int64
	lastOffset      int64
	fileFullySplit  bool
	completed       bool

	datafile *datafile.DataFile
	header   string
}

func NewFileBatchProducer(task *ImportFileTask, state *ImportDataState) (*FileBatchProducer, error) {
	err := state.PrepareForFileImport(task.FilePath, task.TableNameTup)
	if err != nil {
		return nil, fmt.Errorf("preparing for file import: %s", err)
	}
	pendingBatches, lastBatchNumber, lastOffset, fileFullySplit, err := state.Recover(task.FilePath, task.TableNameTup)
	if err != nil {
		return nil, fmt.Errorf("recovering state for table: %q: %s", task.TableNameTup, err)
	}
	var completed bool
	if len(pendingBatches) == 0 && fileFullySplit {
		completed = true
	}

	return &FileBatchProducer{
		task:            task,
		state:           state,
		pendingBatches:  pendingBatches,
		lastBatchNumber: lastBatchNumber,
		lastOffset:      lastOffset,
		fileFullySplit:  fileFullySplit,
		completed:       completed,
	}, nil
}

func (p *FileBatchProducer) Done() bool {
	return p.completed
}

func (p *FileBatchProducer) NextBatch() (*Batch, error) {
	if len(p.pendingBatches) > 0 {
		batch := p.pendingBatches[0]
		p.pendingBatches = p.pendingBatches[1:]
		// file is fully split and returning the last batch, so mark the producer as completed
		if len(p.pendingBatches) == 0 && p.fileFullySplit {
			p.completed = true
		}
		return batch, nil
	}

	return p.produceNextBatch()
}

func (p *FileBatchProducer) produceNextBatch() (*Batch, error) {
	if p.datafile == nil {
		err := p.openDataFile()
		if err != nil {
			return nil, err
		}
	}

	batchWriter, err := p.newBatchWriter()
	if err != nil {
		return nil, err
	}

}

func (p *FileBatchProducer) openDataFile() error {
	reader, err := dataStore.Open(p.task.FilePath)
	if err != nil {
		return fmt.Errorf("preparing reader for split generation on file: %q: %v", p.task.FilePath, err)
	}

	dataFile, err := datafile.NewDataFile(p.task.FilePath, reader, dataFileDescriptor)

	if err != nil {
		return fmt.Errorf("open datafile: %q: %v", p.task.FilePath, err)
	}
	p.datafile = &dataFile

	log.Infof("Skipping %d lines from %q", p.lastOffset, p.task.FilePath)
	err = dataFile.SkipLines(p.lastOffset)
	if err != nil {
		return fmt.Errorf("skipping line for offset=%d: %v", p.lastOffset, err)
	}
	if dataFileDescriptor.HasHeader {
		p.header = dataFile.GetHeader()
	}
	return nil
}

func (p *FileBatchProducer) newBatchWriter() (*BatchWriter, error) {
	batchNum := p.lastBatchNumber + 1
	batchWriter := p.state.NewBatchWriter(p.task.FilePath, p.task.TableNameTup, batchNum)
	err := batchWriter.Init()
	if err != nil {
		return nil, fmt.Errorf("initializing batch writer for table: %q: %s", p.task.TableNameTup, err)
	}
	// Write the header if necessary
	if p.header != "" && dataFileDescriptor.FileFormat == datafile.CSV {
		err = batchWriter.WriteHeader(p.header)
		if err != nil {
			utils.ErrExit("writing header for table: %q: %s", p.task.TableNameTup, err)
		}
	}
	return batchWriter, nil
}

func (p *FileBatchProducer) finalizeBatch(batchWriter *BatchWriter, isLastBatch bool, offsetEnd int64, bytesInBatch int64) (*Batch, error) {
	batchNum := p.lastBatchNumber + 1
	batch, err := batchWriter.Done(isLastBatch, offsetEnd, bytesInBatch)
	if err != nil {
		utils.ErrExit("finalizing batch %d: %s", batchNum, err)
	}
	batchWriter = nil
	p.lastBatchNumber = batchNum
	return batch, nil
	// submitBatch(batch, updateProgressFn, importBatchArgsProto)

	// Increment batchNum only if this is not the last batch
	// if !isLastBatch {
	// 	batchNum++
	// }
}
