package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

type ImportDataState struct {
	exportDir string
}

func NewImportDataState(exportDir string) *ImportDataState {
	return &ImportDataState{exportDir: exportDir}
}

func (s *ImportDataState) GetPendingBatches(tableName string) ([]*Batch, error) {
	return s.getBatches(tableName, "CP")
}

func (s *ImportDataState) GetCompletedBatches(tableName string) ([]*Batch, error) {
	return s.getBatches(tableName, "D")
}

func (s *ImportDataState) GetAllBatches(tableName string) ([]*Batch, error) {
	return s.getBatches(tableName, "CPD")
}

type TableImportState string

const (
	TABLE_IMPORT_NOT_STARTED TableImportState = "TABLE_IMPORT_NOT_STARTED"
	TABLE_IMPORT_IN_PROGRESS TableImportState = "TABLE_IMPORT_IN_PROGRESS"
	TABLE_IMPORT_COMPLETED   TableImportState = "TABLE_IMPORT_COMPLETED"
)

func (s *ImportDataState) GetTableImportState(tableName string) (TableImportState, error) {
	batches, err := s.GetAllBatches(tableName)
	if err != nil {
		return TABLE_IMPORT_NOT_STARTED, fmt.Errorf("error while getting all batches for %s: %w", tableName, err)
	}
	if len(batches) == 0 {
		return TABLE_IMPORT_NOT_STARTED, nil
	}
	batchGenerationCompleted := false
	interruptedCount, doneCount := 0, 0
	for _, batch := range batches {
		if batch.IsDone() {
			doneCount++
		} else if batch.IsInterrupted() {
			interruptedCount++
		}
		if batch.Number == LAST_SPLIT_NUM {
			batchGenerationCompleted = true
		}
	}
	if doneCount == len(batches) && batchGenerationCompleted {
		return TABLE_IMPORT_COMPLETED, nil
	}
	if interruptedCount == 0 && doneCount == 0 {
		return TABLE_IMPORT_NOT_STARTED, nil
	}
	return TABLE_IMPORT_IN_PROGRESS, nil
}

func (s *ImportDataState) Recover(tableName string) ([]*Batch, int64, int64, bool, error) {
	var pendingBatches []*Batch

	lastBatchNumber := int64(0)
	lastOffset := int64(0)
	fileFullySplit := false

	batches, err := s.GetAllBatches(tableName)
	if err != nil {
		return nil, 0, 0, false, fmt.Errorf("error while getting all batches for %s: %w", tableName, err)
	}
	for _, batch := range batches {
		/*
			offsets are 0-based, while numLines are 1-based
			offsetStart is the line in original datafile from where current split starts
			offsetEnd   is the line in original datafile from where next split starts
		*/
		if batch.Number == LAST_SPLIT_NUM {
			fileFullySplit = true
		}
		if batch.Number > lastBatchNumber {
			lastBatchNumber = batch.Number
		}
		if batch.OffsetEnd > lastOffset {
			lastOffset = batch.OffsetEnd
		}
		if !batch.IsDone() {
			pendingBatches = append(pendingBatches, batch)
		}
	}
	return pendingBatches, lastBatchNumber, lastOffset, fileFullySplit, nil
}

func (s *ImportDataState) Clean(tableName string, conn *pgx.Conn) error {
	log.Infof("Cleaning import data state for table %q.", tableName)
	batches, err := s.GetAllBatches(tableName)
	if err != nil {
		return fmt.Errorf("error while getting all batches for %s: %w", tableName, err)
	}
	for _, batch := range batches {
		err = batch.Delete()
		if err != nil {
			return fmt.Errorf("error while deleting batch %d for %s: %w", batch.Number, tableName, err)
		}
	}
	// Delete all entries from ybvoyager_metadata.ybvoyager_import_data_batches_metainfo for this table.
	metaInfoTableName := "ybvoyager_metadata.ybvoyager_import_data_batches_metainfo"
	cmd := fmt.Sprintf(`DELETE FROM %s WHERE file_name LIKE '%s.%%'`, metaInfoTableName, tableName)
	res, err := conn.Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("remove %q related entries from %s: %w", tableName, metaInfoTableName, err)
	}
	log.Infof("query: [%s] => rows affected %v", cmd, res.RowsAffected())
	return nil
}

func (s *ImportDataState) GetImportedRowCount(tableNames []string) (map[string]int64, error) {
	result := make(map[string]int64)

	for _, table := range tableNames {
		batches, err := s.GetCompletedBatches(table)
		if err != nil {
			return nil, fmt.Errorf("error while getting completed batches for %s: %w", table, err)
		}
		for _, batch := range batches {
			result[table] += batch.RecordCount
		}
		if result[table] == 0 {
			// Import not started.
			result[table] = -1
		}
	}
	return result, nil
}

func (s *ImportDataState) GetImportedByteCount(tableNames []string) (map[string]int64, error) {
	result := make(map[string]int64)

	for _, table := range tableNames {
		batches, err := s.GetCompletedBatches(table)
		if err != nil {
			return nil, fmt.Errorf("error while getting completed batches for %s: %w", table, err)
		}
		for _, batch := range batches {
			result[table] += batch.ByteCount
		}
		if result[table] == 0 {
			// Import not started.
			result[table] = -1
		}
	}
	return result, nil
}

func (s *ImportDataState) NewBatchWriter(tableName string, batchNumber int64) *BatchWriter {
	return &BatchWriter{state: s, tableName: tableName, batchNumber: batchNumber}
}

func (s *ImportDataState) getBatches(tableName string, states string) ([]*Batch, error) {
	var result []*Batch

	pattern := fmt.Sprintf("%s/%s/data/%s.[0-9]*.[0-9]*.[0-9]*.[%s]", exportDir, metaInfoDirName, tableName, states)
	batchFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("error while globbing for %s: %w", pattern, err)
	}
	splitFileNameRegexStr := fmt.Sprintf(`.+/%s\.(\d+)\.(\d+)\.(\d+)\.(\d+)\.[CPD]$`, tableName)
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
			recordCount, _ := strconv.ParseInt(match[3], 10, 64)
			byteCount, _ := strconv.ParseInt(match[4], 10, 64)
			offsetStart := offsetEnd - recordCount
			batch := &Batch{
				SchemaName:  "",
				TableName:   tableName,
				FilePath:    filepath,
				Number:      splitNum,
				OffsetStart: offsetStart,
				OffsetEnd:   offsetEnd,
				ByteCount:   byteCount,
				RecordCount: recordCount,
			}
			result = append(result, batch)
		}
	}
	return result, nil

}

//============================================================================

type BatchWriter struct {
	state *ImportDataState

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
		RecordCount: bw.NumRecordsWritten,
		ByteCount:   byteCount,
	}
	return batch, nil
}

//============================================================================

type Batch struct {
	Number              int64
	TableName           string
	SchemaName          string
	FilePath            string
	OffsetStart         int64
	OffsetEnd           int64
	RecordCount         int64
	ByteCount           int64
	TmpConnectionString string
	Interrupted         bool
}

func (batch *Batch) Open() (*os.File, error) {
	return os.Open(batch.FilePath)
}

func (batch *Batch) Delete() error {
	err := os.RemoveAll(batch.FilePath)
	if err != nil {
		return fmt.Errorf("remove %q: %s", batch.FilePath, err)
	}
	log.Infof("Deleted %q", batch.FilePath)
	batch.FilePath = ""
	return nil
}

func (batch *Batch) ImportIsNotStarted() bool {
	return strings.HasSuffix(batch.FilePath, ".C")
}

func (batch *Batch) IsInterrupted() bool {
	return strings.HasSuffix(batch.FilePath, ".P")
}

func (batch *Batch) IsDone() bool {
	return strings.HasSuffix(batch.FilePath, ".D")
}

func (batch *Batch) MarkPending() error {
	// Rename the file to .P
	inProgressFilePath := batch.getInProgressFilePath()
	log.Infof("Renaming file from %q to %q", batch.FilePath, inProgressFilePath)
	err := os.Rename(batch.FilePath, inProgressFilePath)
	if err != nil {
		return fmt.Errorf("rename %q to %q: %w", batch.FilePath, inProgressFilePath, err)
	}
	batch.FilePath = inProgressFilePath
	return nil
}

func (batch *Batch) MarkDone() error {
	inProgressFilePath := batch.getInProgressFilePath()
	doneFilePath := batch.getDoneFilePath()
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

func (batch *Batch) IsAlreadyImported(tx pgx.Tx) (bool, int64, error) {
	var rowsImported int64
	fileName := filepath.Base(batch.getInProgressFilePath())
	schemaName := getTargetSchemaName(batch.TableName)
	query := fmt.Sprintf(
		"SELECT rows_imported FROM ybvoyager_metadata.ybvoyager_import_data_batches_metainfo WHERE schema_name = '%s' AND file_name = '%s';",
		schemaName, fileName)
	err := tx.QueryRow(context.Background(), query).Scan(&rowsImported)
	if err == nil {
		log.Infof("%v rows from %q are already imported", rowsImported, fileName)
		return true, rowsImported, nil
	}
	if err == pgx.ErrNoRows {
		log.Infof("%q is not imported yet", fileName)
		return false, 0, nil
	}
	return false, 0, fmt.Errorf("check if %s is already imported: %w", fileName, err)
}

func (batch *Batch) RecordEntryInDB(tx pgx.Tx, rowsAffected int64) error {
	// Record an entry in ybvoyager_metadata.ybvoyager_import_data_batches_metainfo, that the split is imported.
	fileName := filepath.Base(batch.getInProgressFilePath())
	schemaName := getTargetSchemaName(batch.TableName)
	cmd := fmt.Sprintf(
		`INSERT INTO ybvoyager_metadata.ybvoyager_import_data_batches_metainfo (schema_name, file_name, rows_imported)
		VALUES ('%s', '%s', %v);`, schemaName, fileName, rowsAffected)
	_, err := tx.Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("insert into ybvoyager_metadata.ybvoyager_import_data_batches_metainfo: %w", err)
	}
	log.Infof("Inserted (%q, %q, %v) in ybvoyager_metadata.ybvoyager_import_data_batches_metainfo", schemaName, fileName, rowsAffected)
	return nil
}

func (batch *Batch) getInProgressFilePath() string {
	return batch.FilePath[0:len(batch.FilePath)-1] + "P" // *.C -> *.P
}

func (batch *Batch) getDoneFilePath() string {
	return batch.FilePath[0:len(batch.FilePath)-1] + "D" // *.P -> *.D
}
