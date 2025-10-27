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
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/prometheus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const FIRST_BATCH_NUM = 1

type FileBatchProducer struct {
	task  *ImportFileTask
	state *ImportDataState

	pendingBatches  []*Batch // pending batches after recovery
	lastBatchNumber int64    // batch number of the last batch that was produced
	lastOffset      int64    // file offset from where the last batch was produced, only used in recovery
	fileFullySplit  bool     // if the file is fully split into batches
	completed       bool     // if all batches have been produced

	dataFile        datafile.DataFile
	header          string
	headerByteCount int64
	numLinesTaken   int64 // number of lines read from the file
	// line that was read from file while producing the previous batch
	// but not added to the batch because adding it would breach size/row based thresholds.
	lineFromPreviousBatch string

	errorHandler importdata.ImportDataErrorHandler

	//required to print some information for users to display during batch production
	progressReporter *ImportDataProgressReporter
}

func NewFileBatchProducer(task *ImportFileTask, state *ImportDataState, errorHandler importdata.ImportDataErrorHandler, progressReporter *ImportDataProgressReporter) (*FileBatchProducer, error) {
	if errorHandler == nil {
		return nil, fmt.Errorf("errorHandler must not be nil")
	}

	err := state.PrepareForFileImport(task.FilePath, task.TableNameTup)
	if err != nil {
		return nil, fmt.Errorf("preparing for file import: %s", err)
	}

	pendingBatches, lastBatchNumber, lastOffset, fileFullySplit, err := state.Recover(task.FilePath, task.TableNameTup)
	if err != nil {
		return nil, fmt.Errorf("recovering state for table: %q: %s", task.TableNameTup, err)
	}
	completed := len(pendingBatches) == 0 && fileFullySplit

	// sort the pending batches by placing interrupted batches at front so that they are ingested first for that file ensuring recovery first.
	// TODO: Need to ensure the file/tasks picked are the priortised based on the inprogress ones being prioritised than the new ones.
	sort.Slice(pendingBatches, func(i, j int) bool {
		return pendingBatches[i].IsInterrupted()
	})

	return &FileBatchProducer{
		task:             task,
		state:            state,
		pendingBatches:   pendingBatches,
		lastBatchNumber:  lastBatchNumber,
		lastOffset:       lastOffset,
		fileFullySplit:   fileFullySplit,
		completed:        completed,
		numLinesTaken:    lastOffset,
		errorHandler:     errorHandler,
		progressReporter: progressReporter,
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
	// utils.IRP.RequestToRunLowPriorityIO()
	// defer utils.IRP.ReleaseLowPriorityIO()

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
		// Write the record to the current batch
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
		// TODO: fix. Here we compare line_bytes with max_batch_size_bytes
		// but below we compare header_bytes+line_bytes with max_batch_size_bytes
		// and that can result in disregarding the row and adding it to the next batch (lineFromPreviousBatch)
		if currentBytesRead > tdb.MaxBatchSizeInBytes() {
			//If a row is itself larger than MaxBatchSizeInBytes erroring out
			ybSpecificMsg := ""
			if tconf.TargetDBType == YUGABYTEDB {
				ybSpecificMsg = ", but should be strictly lower than the the rpc_max_message_size on YugabyteDB (default 267386880 bytes)"
			}
			errMsg := fmt.Errorf("record of size %d larger than max batch size: record num=%d for table %q in file %s is larger than the max batch size %d bytes. Max Batch size can be changed using env var MAX_BATCH_SIZE_BYTES%s", currentBytesRead, p.numLinesTaken, p.task.TableNameTup.ForOutput(), p.task.FilePath, tdb.MaxBatchSizeInBytes(), ybSpecificMsg)
			if p.errorHandler.ShouldAbort() {
				return nil, errMsg
			}
			err := p.handleRowProcessingErrorAndResetBytes(batchNum, line, errMsg, currentBytesRead)
			if err != nil {
				return nil, err
			}
			continue
		}
		if line != "" {
			// can't use importBatchArgsProto.Columns as to use case insenstiive column names
			columnNames, _ := TableToColumnNames.Get(p.task.TableNameTup)
			lineBeforeConversion := line
			line, err = valueConverter.ConvertRow(p.task.TableNameTup, columnNames, line)
			if err != nil {
				errMsg := fmt.Errorf("transforming line number=%d for table: %q in file %s: %s", p.numLinesTaken, p.task.TableNameTup.ForOutput(), p.task.FilePath, err)
				if p.errorHandler.ShouldAbort() {
					return nil, errMsg
				}
				err := p.handleRowProcessingErrorAndResetBytes(batchNum, lineBeforeConversion, errMsg, currentBytesRead)
				if err != nil {
					return nil, err
				}
				continue
			}
			batchBytesCount := p.dataFile.GetBytesRead() // GetBytesRead - returns the total bytes read until now including the currentBytesRead
			if p.header != "" {
				batchBytesCount += p.headerByteCount //include header bytes to batch bytes while checking the RPC size limit
			}

			// Check if adding this record exceeds the max batch size
			if batchWriter.NumRecordsWritten == batchSizeInNumRows ||
				batchBytesCount > tdb.MaxBatchSizeInBytes() {

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

			// TODO: fix. After writing the record, we should ideally check for
			// batchWriter.NumRecordsWritten == batchSizeInNumRows
			// instead of continuing and reading the next line.
		}

		// Finalize the batch if it's the last line or the end of the file and reset the bytes read to 0
		if readLineErr == io.EOF {
			batch, err := p.finalizeBatch(batchWriter, true, p.numLinesTaken, p.dataFile.GetBytesRead())
			if err != nil {
				return nil, err
			}

			p.completed = true

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

	//First read the header and then skip lines, because if we skip lines first then header won't be correct in case of resumption
	if dataFileDescriptor.HasHeader {
		p.header = dataFile.GetHeader()
		p.headerByteCount = dataFile.GetBytesRead()
		dataFile.ResetBytesRead(0) //reset the bytes read for header
	}

	log.Infof("Skipping %d lines from %q", p.lastOffset, p.task.FilePath)
	if p.progressReporter != nil {
		p.progressReporter.AddResumeInformation(p.task, fmt.Sprintf("Resuming from %d lines", p.lastOffset))
	}
	err = dataFile.SkipLines(p.lastOffset)
	if err != nil {
		return fmt.Errorf("skipping line for offset=%d: %v", p.lastOffset, err)
	}
	if p.progressReporter != nil {
		p.progressReporter.RemoveResumeInformation(p.task)
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

	// before we write the batch, we also store the processing errors that were encountered and stashed while
	// producing the batch.
	// It's important to do this before writing the batch, so that the processing errors are not lost.
	// If we fail after storing the processing errors, but before writing the batch, during resume, the batch
	// production will start from the previous batch's offset. Therefore, we will encounter all the processing errors again
	// and those will be accumulated and stored again (the file will be overwritten).
	err := p.errorHandler.FinalizeRowProcessingErrorsForBatch(batchNum, isLastBatch, p.task.TableNameTup, p.task.FilePath)
	if err != nil {
		return nil, fmt.Errorf("finalizing row processing errors for batch %d: %w", batchNum, err)
	}

	if p.header != "" && batchNum == FIRST_BATCH_NUM {
		//in the import-data-state of the batch include the header bytes only for the first batch so imported Bytes count is same as total bytes count
		bytesInBatch += p.headerByteCount
	}
	batch, err := batchWriter.Done(isLastBatch, offsetEnd, bytesInBatch)
	if err != nil {
		utils.ErrExit("finalizing batch %d: %s", batchNum, err)
	}

	// Record batch created metric
	prometheus.RecordSnapshotBatchCreated(p.task.TableNameTup, importerRole)

	batchWriter = nil
	p.lastBatchNumber = batchNum
	return batch, nil
}

func (p *FileBatchProducer) Close() {
	if p.dataFile != nil {
		p.dataFile.Close()
	}
}

func (p *FileBatchProducer) handleRowProcessingErrorAndResetBytes(currentBatchNumber int64, row string, rowErr error, currentBytesRead int64) error {
	handleErr := p.errorHandler.HandleRowProcessingError(row, currentBytesRead, rowErr, p.task.TableNameTup, p.task.FilePath, currentBatchNumber)
	if handleErr != nil {
		return fmt.Errorf("failed to handle row processing error: %w", handleErr)
	}
	// datafile.GetBytesRead tracks the total bytes read from the file in the current batch.
	// Since we are not adding the current row to the batch, we need to reset the bytes read
	// to the previous value before reading the current row.
	p.dataFile.ResetBytesRead(p.dataFile.GetBytesRead() - currentBytesRead)
	return nil
}
