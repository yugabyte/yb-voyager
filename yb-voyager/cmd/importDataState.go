package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type ImportDataState struct {
	exportDir string
}

func NewImportDataState(exportDir string) *ImportDataState {
	return &ImportDataState{exportDir: exportDir}
}

func (s *ImportDataState) Recover(tableName string) ([]*Batch, int64, int64, bool, error) {
	var pendingBatches []*Batch

	lastBatchNumber := int64(0)
	lastOffset := int64(0)
	fileFullySplit := false
	pattern := fmt.Sprintf("%s/%s/data/%s.[0-9]*.[0-9]*.[0-9]*.[CPD]", exportDir, metaInfoDirName, tableName)
	batchFiles, _ := filepath.Glob(pattern)

	doneSplitRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.[D]$", tableName)
	doneSplitRegexp := regexp.MustCompile(doneSplitRegexStr)
	splitFileNameRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.[CPD]$", tableName)
	splitFileNameRegex := regexp.MustCompile(splitFileNameRegexStr)

	for _, filepath := range batchFiles {
		matches := splitFileNameRegex.FindAllStringSubmatch(filepath, -1)
		for _, match := range matches {
			/*
				offsets are 0-based, while numLines are 1-based
				offsetStart is the line in original datafile from where current split starts
				offsetEnd   is the line in original datafile from where next split starts
			*/
			splitNum, _ := strconv.ParseInt(match[1], 10, 64)
			offsetEnd, _ := strconv.ParseInt(match[2], 10, 64)
			numLines, _ := strconv.ParseInt(match[3], 10, 64)
			offsetStart := offsetEnd - numLines
			if splitNum == LAST_SPLIT_NUM {
				fileFullySplit = true
			}
			if splitNum > lastBatchNumber {
				lastBatchNumber = splitNum
			}
			if offsetEnd > lastOffset {
				lastOffset = offsetEnd
			}
			if !doneSplitRegexp.MatchString(filepath) {
				batch := &Batch{
					SchemaName:  "",
					TableName:   tableName,
					FilePath:    filepath,
					Number:      splitNum,
					OffsetStart: offsetStart,
					OffsetEnd:   offsetEnd,
					Interrupted: true,
				}
				pendingBatches = append(pendingBatches, batch)
			}
		}
	}
	return pendingBatches, lastBatchNumber, lastOffset, fileFullySplit, nil
}

func (s *ImportDataState) NewBatchWriter(tableName string, batchNumber int64) *BatchWriter {
	return &BatchWriter{tableName: tableName, batchNumber: batchNumber}
}

type BatchWriter struct {
	tableName   string
	batchNumber int64

	NumRecordsWritten      int64
	flagFirstRecordWritten bool

	outFile *os.File
	w       *bufio.Writer
}

func (bw *BatchWriter) Init() error {
	currTmpFileName := fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDirName, bw.tableName, bw.batchNumber)
	log.Infof("current temp file: %s", currTmpFileName)
	outFile, err := os.Create(currTmpFileName)
	if err != nil {
		return fmt.Errorf("create file %q: %s", currTmpFileName, err)
	}
	bw.outFile = outFile
	bw.w = bufio.NewWriter(outFile)
	return nil
}

func (bw *BatchWriter) WriteHeader(header string) error {
	_, err := bw.w.WriteString(header + "\n")
	if err != nil {
		return fmt.Errorf("write header to %q: %s", bw.outFile.Name(), err)
	}
	return nil
}

func (bw *BatchWriter) WriteRecord(record string) error {
	if record == "" {
		return nil
	}
	var err error
	if bw.flagFirstRecordWritten {
		_, err = bw.w.WriteString("\n")
		if err != nil {
			return fmt.Errorf("write to %q: %s", bw.outFile.Name(), err)
		}
	}
	_, err = bw.w.WriteString(record)
	if err != nil {
		return fmt.Errorf("write record to %q: %s", bw.outFile.Name(), err)
	}
	bw.NumRecordsWritten++
	bw.flagFirstRecordWritten = true
	if bw.w.Buffered() == 4*1024*1024 {
		err = bw.w.Flush()
		if err != nil {
			return fmt.Errorf("flush %q: %s", bw.outFile.Name(), err)
		}
		bw.w.Reset(bw.outFile)
	}
	return nil
}

func (bw *BatchWriter) Done(isLastBatch bool, offsetEnd int64, byteCount int64) (*Batch, error) {
	err := bw.w.Flush()
	if err != nil {
		return nil, fmt.Errorf("flush %q: %s", bw.outFile.Name(), err)
	}
	tmpFileName := bw.outFile.Name()
	err = bw.outFile.Close()
	if err != nil {
		return nil, fmt.Errorf("close %q: %s", bw.outFile.Name(), err)
	}

	batchNumber := bw.batchNumber
	if isLastBatch {
		batchNumber = LAST_SPLIT_NUM
	}
	batchFilePath := fmt.Sprintf("%s/%s/data/%s.%d.%d.%d.%d.C",
		exportDir, metaInfoDirName, bw.tableName, batchNumber, offsetEnd, bw.NumRecordsWritten, byteCount)
	log.Infof("Renaming %q to %q", tmpFileName, batchFilePath)
	err = os.Rename(tmpFileName, batchFilePath)
	if err != nil {
		return nil, fmt.Errorf("rename %q to %q: %s", tmpFileName, batchFilePath, err)
	}
	batch := &Batch{
		SchemaName:  "",
		TableName:   bw.tableName,
		FilePath:    batchFilePath,
		Number:      batchNumber,
		OffsetStart: offsetEnd - bw.NumRecordsWritten,
		OffsetEnd:   offsetEnd,
		Interrupted: false,
	}
	return batch, nil
}

type Batch struct {
	TableName           string
	SchemaName          string
	FilePath            string
	OffsetStart         int64
	OffsetEnd           int64
	TmpConnectionString string
	Number              int64
	Interrupted         bool
}

func (batch *Batch) Open() (*os.File, error) {
	return os.Open(batch.FilePath)
}

func (batch *Batch) MarkPending() error {
	// Rename the file to .P
	inProgressFilePath := getInProgressFilePath(batch)
	log.Infof("Renaming file from %q to %q", batch.FilePath, inProgressFilePath)
	err := os.Rename(batch.FilePath, inProgressFilePath)
	if err != nil {
		return fmt.Errorf("rename %q to %q: %w", batch.FilePath, inProgressFilePath, err)
	}
	batch.FilePath = inProgressFilePath
	return nil
}

func (batch *Batch) MarkDone() error {
	inProgressFilePath := getInProgressFilePath(batch)
	doneFilePath := getDoneFilePath(batch)
	log.Infof("Renaming %q => %q", inProgressFilePath, doneFilePath)
	err := os.Rename(inProgressFilePath, doneFilePath)
	if err != nil {
		return fmt.Errorf("rename %q => %q: %w", inProgressFilePath, doneFilePath, err)
	}

	if truncateSplits {
		err = os.Truncate(doneFilePath, 0)
		if err != nil {
			log.Warnf("truncate file %q: %s", doneFilePath, err)
		}
	}
	batch.FilePath = doneFilePath
	return nil
}
