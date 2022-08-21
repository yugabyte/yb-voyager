package libmig

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Batch struct {
	TableID     *TableID
	BatchNumber int

	Desc *DataFileDescriptor

	// BaseFile* attributes of a batch never change. They are required to recover after restart.
	BaseFileName          string
	StartOffsetInBaseFile int64
	EndOffsetInBaseFile   int64
	IsFinalBatch          bool

	// This FileName starts with same as BaseFileName. But when the batch fails and is dumped in the
	// `failed/` directory, the following attribute change to point to the new file.
	FileName    string
	StartOffset int64
	EndOffset   int64

	Header      string
	RecordCount int

	NumRecordsImported int64
	Err                string
	ImportAttempts     int
}

func (b *Batch) Reader() (io.ReadCloser, error) {
	var reader io.ReadCloser
	var err error

	switch b.Desc.FileType {
	case FILE_TYPE_CSV:
		reader, err = NewFileSegmentReader(b.FileName, b.StartOffset, b.EndOffset)
	case FILE_TYPE_ORA2PG:
		// `insideCopyStmt` is false only for the first batch.
		reader, err = NewOra2pgFileSegmentReader(b.FileName, b.StartOffset, b.EndOffset, b.BatchNumber > 1)
	default:
		panic(fmt.Sprintf("unknown file-type: %q", b.Desc.FileType))
	}
	// Ensure that every batch gets the header.
	if err == nil && b.Header != "" {
		reader = NewConcatReadCloser(strings.NewReader(b.Header+"\n"), reader)
	}
	if err != nil {
		return reader, fmt.Errorf("prepare reader: %w", err)
	}
	return reader, nil
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
	if b.BatchNumber == 1 {
		// StartOffsetInBaseFile is considered as 0 (even if there is a header line).
		return b.EndOffsetInBaseFile
	} else {
		return b.EndOffsetInBaseFile - b.StartOffsetInBaseFile
	}
}

func LoadBatchFrom(fileName string) (*Batch, error) {
	bs, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", fileName, err)
	}
	b := &Batch{}
	err = json.Unmarshal(bs, b)
	if err != nil {
		return nil, fmt.Errorf("parse contents of %s: %w", fileName, err)
	}
	return b, nil
}

//===============================================================================

type ConcatReadCloser struct {
	first  io.Reader
	second io.Reader

	multiReader io.Reader
}

func NewConcatReadCloser(first, second io.Reader) *ConcatReadCloser {
	return &ConcatReadCloser{
		first:       first,
		second:      second,
		multiReader: io.MultiReader(first, second),
	}
}

func (cr *ConcatReadCloser) Read(buf []byte) (int, error) {
	return cr.multiReader.Read(buf)
}

func (cr *ConcatReadCloser) Close() error {
	for _, r := range []io.Reader{cr.first, cr.second} {
		c, ok := r.(io.Closer)
		if ok {
			err := c.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//===============================================================================

type BatchGenerator struct {
	FileName string
	TableID  *TableID
	Desc     *DataFileDescriptor

	dataFile        DataFile
	lastBatchNumber int
	header          string
}

func NewBatchGenerator(fileName string, tableID *TableID, desc *DataFileDescriptor) *BatchGenerator {
	return &BatchGenerator{FileName: fileName, TableID: tableID, Desc: desc}
}

func (bg *BatchGenerator) Init(dataFile DataFile, lastBatch *Batch) error {
	log.Infof("Initialise batch generator")
	bg.dataFile = dataFile
	if lastBatch != nil {
		// Start from where we left off.
		bg.lastBatchNumber = lastBatch.BatchNumber
	}
	if bg.Desc.HasHeader {
		header, err := dataFile.GetHeader() // For ora2pg file type, header will be "".
		if err != nil {
			return fmt.Errorf("get header from data file: %w", err)
		}
		bg.header = header
	}
	log.Infof("Starting batch generation from index %v. Header %q.", bg.lastBatchNumber+1, bg.header)
	return nil
}

func (bg *BatchGenerator) NextBatch(batchSize int) (*Batch, bool, error) {
	var batch *Batch

	if bg.Desc.HasHeader && bg.dataFile.Offset() == 0 {
		err := bg.dataFile.SkipHeader()
		if err != nil {
			return nil, false, err
		}
	}
	startOffset := bg.dataFile.Offset()
	n, eof, err := bg.dataFile.SkipRecords(batchSize)
	endOffset := bg.dataFile.Offset()

	if n > 0 {
		bg.lastBatchNumber++
		batch = &Batch{
			TableID:     bg.TableID,
			BatchNumber: bg.lastBatchNumber,

			Desc: bg.Desc,

			BaseFileName:          bg.FileName,
			StartOffsetInBaseFile: startOffset,
			EndOffsetInBaseFile:   endOffset,

			FileName:    bg.FileName,
			StartOffset: startOffset,
			EndOffset:   endOffset,

			IsFinalBatch: eof,
			Header:       bg.header,
			RecordCount:  n,
		}
	}
	return batch, eof, err
}
