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
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type FileBatchProducer struct {
	task  *ImportFileTask
	state *ImportDataState

	pendingBatches  []*Batch //pending batches after recovery
	lastBatchNumber int64    // batch number of the last batch that was produced
	lastOffset      int64    // file offset from where the last batch was produced, only used in recovery
	fileFullySplit  bool     // if the file is fully split into batches
	completed       bool     // if all batches have been produced

	dataFile      datafile.DataFile
	header        string
	numLinesTaken int64 // number of lines read from the file
	// line that was read from file while producing the previous batch
	// but not added to the batch because adding it would breach size/row based thresholds.
	lineFromPreviousBatch string
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
	completed := len(pendingBatches) == 0 && fileFullySplit

	return &FileBatchProducer{
		task:            task,
		state:           state,
		pendingBatches:  pendingBatches,
		lastBatchNumber: lastBatchNumber,
		lastOffset:      lastOffset,
		fileFullySplit:  fileFullySplit,
		completed:       completed,
		numLinesTaken:   lastOffset,
	}, nil
}

func (p *FileBatchProducer) Done() bool {
	return p.completed
}

func (p *FileBatchProducer) NextBatch() (*Batch, error) {
	if p.Done() {
		return nil, fmt.Errorf("already completed producing all batches")
	}
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
	if p.dataFile == nil {
		err := p.openDataFile()
		if err != nil {
			return nil, err
		}
	}

	var readLineErr error
	var line string
	var currentBytesRead int64
	batchNum := p.lastBatchNumber + 1

	batchWriter, err := p.newBatchWriter()
	if err != nil {
		return nil, err
	}
	// in the previous batch, a line was read from file but not added to the batch
	// because adding it would breach size/row based thresholds.
	// Add that line to the current batch.
	if p.lineFromPreviousBatch != "" {
		err = batchWriter.WriteRecord(p.lineFromPreviousBatch)
		if err != nil {
			return nil, fmt.Errorf("Write to batch %d: %s", batchNum, err)
		}
		p.lineFromPreviousBatch = ""
	}

	for readLineErr == nil {

		line, currentBytesRead, readLineErr = p.dataFile.NextLine()
		if readLineErr == nil || (readLineErr == io.EOF && line != "") {
			// handling possible case: last dataline(i.e. EOF) but no newline char at the end
			p.numLinesTaken += 1
		}
		log.Debugf("Batch %d: totalBytesRead %d, currentBytes %d \n", batchNum, p.dataFile.GetBytesRead(), currentBytesRead)
		if currentBytesRead > tdb.MaxBatchSizeInBytes() {
			//If a row is itself larger than MaxBatchSizeInBytes erroring out
			ybSpecificMsg := ""
			if tconf.TargetDBType == YUGABYTEDB {
				ybSpecificMsg = ", but should be strictly lower than the the rpc_max_message_size on YugabyteDB (default 267386880 bytes)"
			}
			return nil, fmt.Errorf("record of size %d larger than max batch size: record num=%d for table %q in file %s is larger than the max batch size %d bytes. Max Batch size can be changed using env var MAX_BATCH_SIZE_BYTES%s", currentBytesRead, p.numLinesTaken, p.task.TableNameTup.ForOutput(), p.task.FilePath, tdb.MaxBatchSizeInBytes(), ybSpecificMsg)
		}
		if line != "" {
			// can't use importBatchArgsProto.Columns as to use case insenstiive column names
			columnNames, _ := TableToColumnNames.Get(p.task.TableNameTup)
			line, err = valueConverter.ConvertRow(p.task.TableNameTup, columnNames, line)
			if err != nil {
				return nil, fmt.Errorf("transforming line number=%d for table: %q in file %s: %s", p.numLinesTaken, p.task.TableNameTup.ForOutput(), p.task.FilePath, err)
			}

			// Check if adding this record exceeds the max batch size
			if batchWriter.NumRecordsWritten == batchSizeInNumRows ||
				p.dataFile.GetBytesRead() > tdb.MaxBatchSizeInBytes() { // GetBytesRead - returns the total bytes read until now including the currentBytesRead

				// Finalize the current batch without adding the record
				batch, err := p.finalizeBatch(batchWriter, false, p.numLinesTaken-1, p.dataFile.GetBytesRead()-currentBytesRead)
				if err != nil {
					return nil, err
				}

				//carry forward the bytes to next batch
				p.dataFile.ResetBytesRead(currentBytesRead)
				p.lineFromPreviousBatch = line

				return batch, nil
			}

			// Write the record to the current batch
			err = batchWriter.WriteRecord(line)
			if err != nil {
				return nil, fmt.Errorf("Write to batch %d: %s", batchNum, err)
			}
		}

		// Finalize the batch if it's the last line or the end of the file and reset the bytes read to 0
		if readLineErr == io.EOF {
			batch, err := p.finalizeBatch(batchWriter, true, p.numLinesTaken, p.dataFile.GetBytesRead())
			if err != nil {
				return nil, err
			}

			p.completed = true
			// TODO: resetting bytes read to 0 is technically not correct if we are adding a header
			// to each batch file. Currently header bytes are only considered in the first batch.
			// For the rest of the batches, header bytes are ignored, since we are resetting it to 0.
			p.dataFile.ResetBytesRead(0)
			return batch, nil
		} else if readLineErr != nil {
			return nil, fmt.Errorf("read line from data file: %q: %s", p.task.FilePath, readLineErr)
		}
	}
	// ideally should not reach here
	return nil, fmt.Errorf("could not produce next batch: err: %w", readLineErr)
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
	p.dataFile = dataFile

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
}

func (p *FileBatchProducer) Close() {
	if p.dataFile != nil {
		p.dataFile.Close()
	}
}
