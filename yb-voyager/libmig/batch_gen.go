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

	df              DataFile
	lastBatchNumber int
}

func NewBatchGenerator(fileName string, tableID *TableID, desc *DataFileDescriptor) *BatchGenerator {
	return &BatchGenerator{FileName: fileName, TableID: tableID, Desc: desc}
}

func (mgr *BatchGenerator) Init(lastBatch *Batch) error {
	// Start from where we left off.
	offset := int64(0)
	if lastBatch != nil {
		offset = lastBatch.EndOffset
		mgr.lastBatchNumber = lastBatch.BatchNumber
	}

	// Open DataFile and jump to the correct offset.
	mgr.df = NewDataFile(mgr.FileName, offset, mgr.Desc)
	err := mgr.df.Open()
	if err != nil {
		return err
	}
	return err
}

func (mgr *BatchGenerator) NextBatch(batchSize int) (*Batch, bool, error) {
	var batch *Batch

	startOffset := mgr.df.Offset()
	n, eof, err := mgr.df.SkipRecords(batchSize)
	endOffset := mgr.df.Offset()

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
