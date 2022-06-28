package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

type Batch struct {
	FileName    string
	BatchNumber int
	StartOffset int64
	EndOffset   int64
	RecordCount int
	Desc        *DataFileDescriptor
}

func (b *Batch) Reader() (io.ReadCloser, error) {
	switch b.Desc.FileType {
	case FILE_TYPE_CSV:
		return NewFileSegmentReader(b.FileName, b.StartOffset, b.EndOffset)
	case FILE_TYPE_ORA2PG:
		// `insideCopyStmt` is false only for the first batch.
		return NewOra2pgFileSegmentReader(b.FileName, b.StartOffset, b.EndOffset, b.BatchNumber > 1)
	default:
		panic(fmt.Sprintf("unknown file-type: %q", b.Desc.FileType))
	}
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

type BatchGenerator struct {
	FileName string
	TableID  *TableID
	Desc     *DataFileDescriptor

	dataFile        DataFile
	lastBatchNumber int
}

func NewBatchGenerator(fileName string, tableID *TableID, desc *DataFileDescriptor) *BatchGenerator {
	return &BatchGenerator{FileName: fileName, TableID: tableID, Desc: desc}
}

func (mgr *BatchGenerator) Init(dataFile DataFile, lastBatch *Batch) error {
	// Start from where we left off.
	mgr.dataFile = dataFile
	if lastBatch != nil {
		mgr.lastBatchNumber = lastBatch.BatchNumber
	}
	return nil
}

func (mgr *BatchGenerator) NextBatch(batchSize int) (*Batch, bool, error) {
	var batch *Batch

	startOffset := mgr.dataFile.Offset()
	n, eof, err := mgr.dataFile.SkipRecords(batchSize)
	endOffset := mgr.dataFile.Offset()

	if n > 0 {
		mgr.lastBatchNumber++
		batch = &Batch{
			FileName:    mgr.FileName,
			BatchNumber: mgr.lastBatchNumber,
			StartOffset: startOffset,
			EndOffset:   endOffset,
			RecordCount: n,
			Desc:        mgr.Desc,
		}
	}
	return batch, eof, err
}
