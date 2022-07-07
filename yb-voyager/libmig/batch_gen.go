package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

type Batch struct {
	TableID     *TableID
	BatchNumber int

	Desc *DataFileDescriptor

	// BaseFile* attributes of a batch never change. They are required to recover after restart.
	BaseFileName          string
	StartOffsetInBaseFile int64
	EndOffsetInBaseFile   int64

	// This FileName starts with same as BaseFileName. But when the batch fails and is dumped in the
	// `failed/` directory, the following attribute change to point to the new file.
	FileName    string
	StartOffset int64
	EndOffset   int64

	RecordCount int

	NumRecordsImported int
	Err                string
	ImportAttempts     int
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
	bs, err := json.MarshalIndent(b, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, bs, 0644)
	return err
}

func (b *Batch) SizeInBaseFile() int64 {
	return b.EndOffsetInBaseFile - b.StartOffsetInBaseFile
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
			TableID:     mgr.TableID,
			BatchNumber: mgr.lastBatchNumber,

			Desc: mgr.Desc,

			BaseFileName:          mgr.FileName,
			StartOffsetInBaseFile: startOffset,
			EndOffsetInBaseFile:   endOffset,

			FileName:    mgr.FileName,
			StartOffset: startOffset,
			EndOffset:   endOffset,

			RecordCount: n,
		}
	}
	return batch, eof, err
}
