package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
)

type Batch struct {
	FileName    string
	BatchNumber int
	StartOffset int64
	EndOffset   int64
	RecordCount int
}

func (b *Batch) Reader() (io.ReadCloser, error) {
	return NewFileSegmentReader(b.FileName, b.StartOffset, b.EndOffset)
}

func (b *Batch) SaveTo(fileName string) error {
	bs, err := json.Marshal(b)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, bs, 0644)
	return err
}

func LoadBatchFrom(fileName string) (*Batch, error) {
	bs, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	b := &Batch{}
	err = json.Unmarshal(bs, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

//===============================================================================

type BatchManager struct {
	migState *MigrationState

	FileName        string
	TableID         *TableID
	df              DataFile
	lastBatchNumber int
}

func NewBatchManager(migState *MigrationState, fileName string, tableID *TableID) *BatchManager {
	return &BatchManager{migState: migState, FileName: fileName, TableID: tableID}
}

func (mgr *BatchManager) Init() error {
	// TODO Implement StartClean.
	err := mgr.migState.PrepareForImport(mgr.TableID)
	if err != nil {
		return err
	}

	offset := int64(0)
	lastBatch, err := mgr.migState.GetLastBatch(mgr.TableID)
	// TODO: GetLastBatch() should return a higher level error than the os.ErrNotExist.
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if lastBatch != nil {
		offset = lastBatch.EndOffset
		mgr.lastBatchNumber = lastBatch.BatchNumber
	}
	mgr.df = NewDataFile(mgr.FileName, offset)
	err = mgr.df.Open()
	return err
}

func (mgr *BatchManager) PendingBatches() ([]*Batch, error) {
	return mgr.migState.PendingBatches(mgr.TableID)
}

func (mgr *BatchManager) NextBatch(batchSize int) (*Batch, bool, error) {
	var batch *Batch
	df := mgr.df

	startOffset := df.Offset()
	n, eof, err := df.SkipRecords(batchSize)
	endOffset := df.Offset()

	if n > 0 {
		mgr.lastBatchNumber++
		batch = &Batch{
			FileName:    mgr.FileName,
			BatchNumber: mgr.lastBatchNumber,
			StartOffset: startOffset,
			EndOffset:   endOffset,
			RecordCount: n,
		}
		err2 := mgr.migState.NewPendingBatch(mgr.TableID, batch)
		if err2 != nil {
			return nil, false, err2
		}
	}
	return batch, eof, err
}

func (mgr *BatchManager) BatchDone(batch *Batch) error {
	log.Infof("Batch %s %d: DONE", mgr.TableID, batch.BatchNumber)
	return mgr.migState.BatchDone(mgr.TableID, batch)
}
