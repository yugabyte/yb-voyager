/*
Copyright (c) YugaByte, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

/*
metainfo/import_data_state/table::<table_name>/file::<base_name>:<path_hash>/

	link -> dataFile
	batch::<batch_num>.<offset_end>.<record_count>.<byte_count>.<state>
*/
type ImportDataState struct {
	exportDir string
	stateDir  string
}

func NewImportDataState(exportDir string) *ImportDataState {
	return &ImportDataState{exportDir: exportDir, stateDir: filepath.Join(exportDir, "metainfo", "import_data_state")}
}

func (s *ImportDataState) PrepareForFileImport(filePath, tableName string) error {
	fileStateDir := s.getFileStateDir(filePath, tableName)
	log.Infof("Creating %q.", fileStateDir)
	err := os.MkdirAll(fileStateDir, 0755)
	if err != nil {
		return fmt.Errorf("error while creating %q: %w", fileStateDir, err)
	}
	// Create a symlink to the filePath. The symLink is only for human consumption.
	// It helps in easily distinguishing in files with same names but different paths.
	symlinkPath := filepath.Join(fileStateDir, "link")
	log.Infof("Creating symlink %q -> %q.", symlinkPath, filePath)
	err = os.Symlink(filePath, symlinkPath)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error while creating symlink %q -> %q: %w", symlinkPath, filePath, err)
	}
	return nil
}

func (s *ImportDataState) GetPendingBatches(filePath, tableName string) ([]*Batch, error) {
	return s.getBatches(filePath, tableName, "CP")
}

func (s *ImportDataState) GetCompletedBatches(filePath, tableName string) ([]*Batch, error) {
	return s.getBatches(filePath, tableName, "D")
}

func (s *ImportDataState) GetAllBatches(filePath, tableName string) ([]*Batch, error) {
	return s.getBatches(filePath, tableName, "CPD")
}

type FileImportState string

const (
	FILE_IMPORT_STATE_UNKNOWN FileImportState = "FILE_IMPORT_STATE_UNKNOWN"
	FILE_IMPORT_NOT_STARTED   FileImportState = "FILE_IMPORT_NOT_STARTED"
	FILE_IMPORT_IN_PROGRESS   FileImportState = "FILE_IMPORT_IN_PROGRESS"
	FILE_IMPORT_COMPLETED     FileImportState = "FILE_IMPORT_COMPLETED"
)

func (s *ImportDataState) GetFileImportState(filePath, tableName string) (FileImportState, error) {
	batches, err := s.GetAllBatches(filePath, tableName)
	if err != nil {
		return FILE_IMPORT_STATE_UNKNOWN, fmt.Errorf("error while getting all batches for %s: %w", tableName, err)
	}
	if len(batches) == 0 {
		return FILE_IMPORT_NOT_STARTED, nil
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
		return FILE_IMPORT_COMPLETED, nil
	}
	if interruptedCount == 0 && doneCount == 0 {
		return FILE_IMPORT_NOT_STARTED, nil
	}
	return FILE_IMPORT_IN_PROGRESS, nil
}

func (s *ImportDataState) Recover(filePath, tableName string) ([]*Batch, int64, int64, bool, error) {
	var pendingBatches []*Batch

	lastBatchNumber := int64(0)
	lastOffset := int64(0)
	fileFullySplit := false

	batches, err := s.GetAllBatches(filePath, tableName)
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

func (s *ImportDataState) Clean(filePath string, tableName string, conn *pgx.Conn) error {
	log.Infof("Cleaning import data state for table %q.", tableName)
	fileStateDir := s.getFileStateDir(filePath, tableName)
	log.Infof("Removing %q.", fileStateDir)
	err := os.RemoveAll(fileStateDir)
	if err != nil {
		return fmt.Errorf("error while removing %q: %w", fileStateDir, err)
	}

	// Delete all entries from ybvoyager_metadata.ybvoyager_import_data_batches_metainfo for this table.
	schemaName := getTargetSchemaName(tableName)
	metaInfoTableName := "ybvoyager_metadata.ybvoyager_import_data_batches_metainfo"
	cmd := fmt.Sprintf(
		`DELETE FROM %s WHERE data_file_name = '%s' AND schema_name = '%s' AND table_name = '%s'`,
		metaInfoTableName, filePath, schemaName, tableName)
	res, err := conn.Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("remove %q related entries from %s: %w", tableName, metaInfoTableName, err)
	}
	log.Infof("query: [%s] => rows affected %v", cmd, res.RowsAffected())
	return nil
}

func (s *ImportDataState) GetImportedRowCount(filePath, tableName string) (int64, error) {
	batches, err := s.GetCompletedBatches(filePath, tableName)
	if err != nil {
		return -1, fmt.Errorf("error while getting completed batches for %s: %w", tableName, err)
	}
	result := int64(0)
	for _, batch := range batches {
		result += batch.RecordCount
	}
	return result, nil
}

func (s *ImportDataState) GetImportedByteCount(filePath, tableName string) (int64, error) {
	batches, err := s.GetCompletedBatches(filePath, tableName)
	if err != nil {
		return -1, fmt.Errorf("error while getting completed batches for %s: %w", tableName, err)
	}
	result := int64(0)
	for _, batch := range batches {
		result += batch.ByteCount
	}
	return result, nil
}

func (s *ImportDataState) NewBatchWriter(filePath, tableName string, batchNumber int64) *BatchWriter {
	return &BatchWriter{
		state:       s,
		filePath:    filePath,
		tableName:   tableName,
		batchNumber: batchNumber,
	}
}

func (s *ImportDataState) getBatches(filePath, tableName string, states string) ([]*Batch, error) {
	// result == nil: import not started.
	// empty result: import started but no batches created yet.
	result := []*Batch{}

	fileStateDir := s.getFileStateDir(filePath, tableName)
	// Check if the fileStateDir exists.
	_, err := os.Stat(fileStateDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("fileStateDir %q does not exist", fileStateDir)
			return nil, nil
		}
		return nil, fmt.Errorf("stat %q: %s", fileStateDir, err)
	}

	// Find regular files in the `fileStateDir` whose name starts with "batch::"
	files, err := os.ReadDir(fileStateDir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %s", fileStateDir, err)
	}
	for _, file := range files {
		if file.Type().IsRegular() && strings.HasPrefix(file.Name(), "batch::") {
			batchNum, offsetEnd, recordCount, byteCount, state, err := parseBatchFileName(file.Name())
			if err != nil {
				return nil, fmt.Errorf("parse batch file name %q: %w", file.Name(), err)
			}
			if !strings.Contains(states, state) {
				continue
			}
			batch := &Batch{
				SchemaName:   "",
				TableName:    tableName,
				FilePath:     filepath.Join(fileStateDir, file.Name()),
				BaseFilePath: filePath,
				Number:       batchNum,
				OffsetStart:  offsetEnd - recordCount,
				OffsetEnd:    offsetEnd,
				ByteCount:    byteCount,
				RecordCount:  recordCount,
			}
			result = append(result, batch)
		}
	}
	return result, nil

}

func parseBatchFileName(fileName string) (batchNum, offsetEnd, recordCount, byteCount int64, state string, err error) {
	md := strings.Split(strings.Split(fileName, "::")[1], ".")
	if len(md) != 5 {
		return 0, 0, 0, 0, "", fmt.Errorf("invalid batch file name %q", fileName)
	}
	batchNum, err = strconv.ParseInt(md[0], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("invalid batchNumber %q in the file name %q", md[0], fileName)
	}
	offsetEnd, err = strconv.ParseInt(md[1], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("invalid offsetEnd %q in the file name %q", md[1], fileName)
	}
	recordCount, err = strconv.ParseInt(md[2], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("invalid recordCount %q in the file name %q", md[2], fileName)
	}
	byteCount, err = strconv.ParseInt(md[3], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("invalid byteCount %q in the file name %q", md[3], fileName)
	}
	state = md[4]
	if !slices.Contains([]string{"C", "P", "D"}, state) {
		return 0, 0, 0, 0, "", fmt.Errorf("invalid state %q in the file name %q", md[4], fileName)
	}
	return batchNum, offsetEnd, recordCount, byteCount, state, nil
}

//============================================================================

func (s *ImportDataState) getTableStateDir(tableName string) string {
	return fmt.Sprintf("%s/table::%s", s.stateDir, tableName)
}

func (s *ImportDataState) getFileStateDir(filePath, tableName string) string {
	// NOTE: filePath must be absolute.
	hash := computePathHash(filePath)
	baseName := filepath.Base(filePath)
	return fmt.Sprintf("%s/file::%s::%s", s.getTableStateDir(tableName), baseName, hash)
}

func computePathHash(filePath string) string {
	hash := sha1.New()
	hash.Write([]byte(filePath))
	return hex.EncodeToString(hash.Sum(nil))[0:8]
}

//============================================================================

type BatchWriter struct {
	state *ImportDataState

	filePath    string
	tableName   string
	batchNumber int64

	NumRecordsWritten      int64
	flagFirstRecordWritten bool

	outFile *os.File
	w       *bufio.Writer
}

func (bw *BatchWriter) Init() error {
	fileStateDir := bw.state.getFileStateDir(bw.filePath, bw.tableName)
	currTmpFileName := fmt.Sprintf("%s/tmp::%v", fileStateDir, bw.batchNumber)
	log.Infof("current temp file: %s", currTmpFileName)
	outFile, err := os.Create(currTmpFileName)
	if err != nil {
		return fmt.Errorf("create file %q: %s", currTmpFileName, err)
	}
	bw.outFile = outFile
	bw.w = bufio.NewWriterSize(outFile, FOUR_MB)
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
	fileStateDir := bw.state.getFileStateDir(bw.filePath, bw.tableName)
	batchFilePath := fmt.Sprintf("%s/batch::%d.%d.%d.%d.C",
		fileStateDir, batchNumber, offsetEnd, bw.NumRecordsWritten, byteCount)
	log.Infof("Renaming %q to %q", tmpFileName, batchFilePath)
	err = os.Rename(tmpFileName, batchFilePath)
	if err != nil {
		return nil, fmt.Errorf("rename %q to %q: %s", tmpFileName, batchFilePath, err)
	}
	batch := &Batch{
		SchemaName:   "",
		TableName:    bw.tableName,
		FilePath:     batchFilePath,
		BaseFilePath: bw.filePath,
		Number:       batchNumber,
		OffsetStart:  offsetEnd - bw.NumRecordsWritten,
		OffsetEnd:    offsetEnd,
		RecordCount:  bw.NumRecordsWritten,
		ByteCount:    byteCount,
	}
	return batch, nil
}

//============================================================================

type Batch struct {
	Number              int64
	TableName           string
	SchemaName          string
	FilePath            string // Path of the batch file.
	BaseFilePath        string // Path of the original data file.
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

func (batch *Batch) IsNotStarted() bool {
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
	schemaName := getTargetSchemaName(batch.TableName)
	query := fmt.Sprintf(
		"SELECT rows_imported "+
			"FROM ybvoyager_metadata.ybvoyager_import_data_batches_metainfo "+
			"WHERE data_file_name = '%s' AND batch_number = %d AND schema_name = '%s' AND table_name = '%s';",
		batch.BaseFilePath, batch.Number, schemaName, batch.TableName)
	err := tx.QueryRow(context.Background(), query).Scan(&rowsImported)
	if err == nil {
		log.Infof("%v rows from %q are already imported", rowsImported, batch.FilePath)
		return true, rowsImported, nil
	}
	if err == pgx.ErrNoRows {
		log.Infof("%q is not imported yet", batch.FilePath)
		return false, 0, nil
	}
	return false, 0, fmt.Errorf("check if %s is already imported: %w", batch.FilePath, err)
}

func (batch *Batch) RecordEntryInDB(tx pgx.Tx, rowsAffected int64) error {
	// Record an entry in ybvoyager_metadata.ybvoyager_import_data_batches_metainfo, that the split is imported.
	schemaName := getTargetSchemaName(batch.TableName)
	cmd := fmt.Sprintf(
		`INSERT INTO ybvoyager_metadata.ybvoyager_import_data_batches_metainfo (data_file_name, batch_number, schema_name, table_name, rows_imported)
		VALUES ('%s', %d, '%s', '%s', %v);`,
		batch.BaseFilePath, batch.Number, schemaName, batch.TableName, rowsAffected)
	_, err := tx.Exec(context.Background(), cmd)
	if err != nil {
		return fmt.Errorf("insert into ybvoyager_metadata.ybvoyager_import_data_batches_metainfo: %w", err)
	}
	log.Infof("Inserted ('%s', %d, '%s', '%s', %v) in ybvoyager_metadata.ybvoyager_import_data_batches_metainfo",
		batch.BaseFilePath, batch.Number, schemaName, batch.TableName, rowsAffected)
	return nil
}

func (batch *Batch) getInProgressFilePath() string {
	return batch.FilePath[0:len(batch.FilePath)-1] + "P" // *.C -> *.P
}

func (batch *Batch) getDoneFilePath() string {
	return batch.FilePath[0:len(batch.FilePath)-1] + "D" // *.P -> *.D
}
