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
	migState     *MigrationState
	batchManager *BatchManager

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
	}
}

func (op *ImportFileOp) Init(ctx context.Context) error {
	log.Infof("Init ImportFileOp")
	op.batchManager = NewBatchManager(op.migState, op.FileName, op.TableID)
	err := op.batchManager.Init()
	if err != nil {
		return err
	}

	return nil
}

func (op *ImportFileOp) Run(ctx context.Context) error {
	log.Infof("Run ImportFileOp")

	batches, err := op.batchManager.PendingBatches()
	if err != nil {
		return err
	}
	for _, batch := range batches {
		op.submitBatch(batch)
	}

	for {
		batch, eof, err := op.batchManager.NextBatch(op.BatchSize)
		if batch != nil {
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
	op.batchManager.BatchDone(batch)
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
