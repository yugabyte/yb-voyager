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
	batchGen *BatchGenerator

	FileName string
	TableID  *TableID

	BatchSize int
}

func NewImportFileOp(migState *MigrationState, fileName string, tableID *TableID) *ImportFileOp {
	return &ImportFileOp{
		migState:  migState,
		FileName:  fileName,
		TableID:   tableID,
		BatchSize: DEFAULT_BATCH_SIZE,

		batchGen: NewBatchGenerator(fileName, tableID),
	}
}

func (op *ImportFileOp) Run(ctx context.Context) error {
	log.Infof("Run ImportFileOp")
	// TODO Implement StartClean.
	err := op.migState.PrepareForImport(op.TableID)
	if err != nil {
		return err
	}

	lastBatch, err := op.migState.GetLastBatch(op.TableID)
	if err != nil {
		return err
	}
	// lastBatch can be nil when no batch is generated yet.
	err = op.batchGen.Init(lastBatch)
	if err != nil {
		return err
	}

	batches, err := op.migState.PendingBatches(op.TableID)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		op.submitBatch(batch)
	}

	for {
		batch, eof, err := op.batchGen.NextBatch(op.BatchSize)
		if batch != nil {
			err2 := op.migState.NewPendingBatch(op.TableID, batch)
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

func (op *ImportFileOp) submitBatch(batch *Batch) {
	log.Infof("Submitting batch %d", batch.BatchNumber)
	debugPrintBatch(batch)
	op.migState.BatchDone(op.TableID, batch)
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
