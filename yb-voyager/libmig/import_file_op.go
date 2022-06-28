package main

import (
	"context"
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
)

const (
	DEFAULT_BATCH_SIZE = 100_000
)

type ImportFileOp struct {
	migState *MigrationState

	FileName string
	TableID  *TableID
	Desc     *DataFileDescriptor

	BatchSize int

	batchGen                  *BatchGenerator
	dataFile                  DataFile
	lastBatchFromPrevRun      *Batch
	pendingBatchesFromPrevRun []*Batch
}

func NewImportFileOp(migState *MigrationState, fileName string, tableID *TableID, desc *DataFileDescriptor) *ImportFileOp {
	return &ImportFileOp{
		migState:  migState,
		FileName:  fileName,
		TableID:   tableID,
		Desc:      desc,
		BatchSize: DEFAULT_BATCH_SIZE,

		batchGen: NewBatchGenerator(fileName, tableID, desc),
	}
}

func (op *ImportFileOp) Run(ctx context.Context) error {
	log.Infof("Run ImportFileOp")

	// TODO Implement StartClean.
	err := op.migState.PrepareForImport(op.TableID)
	if err != nil {
		return err
	}
	err = op.loadStateFromPrevRun()
	if err != nil {
		return err
	}
	err = op.openDataFile()
	if err != nil {
		return err
	}
	// op.lastBatchFromPrevRun will be nil for first time execution.
	err = op.batchGen.Init(op.dataFile, op.lastBatchFromPrevRun)
	if err != nil {
		return err
	}

	for _, batch := range op.pendingBatchesFromPrevRun {
		op.submitBatch(batch)
	}
	for {
		batch, eof, err := op.batchGen.NextBatch(op.BatchSize)
		if batch != nil {
			err2 := op.migState.MarkBatchPending(op.TableID, batch)
			if err2 != nil {
				return err2
			}
			op.submitBatch(batch)
		}
		if eof {
			log.Info("Done splitting file.")
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (op *ImportFileOp) loadStateFromPrevRun() error {
	var err error
	op.lastBatchFromPrevRun, err = op.migState.GetLastBatch(op.TableID)
	if err != nil {
		return err
	}
	op.pendingBatchesFromPrevRun, err = op.migState.PendingBatches(op.TableID)
	if err != nil {
		return err
	}
	return nil
}

func (op *ImportFileOp) openDataFile() error {
	// Start from where we left off.
	offset := int64(0)
	if op.lastBatchFromPrevRun != nil {
		offset = op.lastBatchFromPrevRun.EndOffset
	}
	// Open DataFile and jump to the correct offset.
	op.dataFile = NewDataFile(op.FileName, offset, op.Desc)
	err := op.dataFile.Open()
	if err != nil {
		return err
	}
	return err
}

func (op *ImportFileOp) submitBatch(batch *Batch) {
	log.Infof("Submitting batch %d", batch.BatchNumber)
	_ = debugPrintBatch
	debugPrintBatch2(batch)
	op.migState.MarkBatchDone(op.TableID, batch)
}

func debugPrintBatch(batch *Batch) {
	r, err := batch.Reader()
	if err != nil {
		panic(err)
	}
	bs, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", string(bs))
}

func debugPrintBatch2(batch *Batch) {
	r, err := batch.Reader()
	if err != nil {
		panic(err)
	}

	var bs []byte
	buf := make([]byte, 100)
	for {
		n, err := r.Read(buf)
		bs = append(bs, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("%s", string(bs))
}
