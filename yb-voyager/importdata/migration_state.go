package importdata

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

/*
EXPORT_DIR/
	import-data/
		databaseName/
			schemaName/
				tableName/
					last
					pending/
					done/
					failed/
*/

type MigrationState struct {
	ExportDir string
}

func NewMigrationState(exportDir string) *MigrationState {
	return &MigrationState{ExportDir: exportDir}
}

// func (migstate *MigrationState)

func (migstate *MigrationState) PrepareForImport(tableID *TableID) error {
	log.Infof("Prepare for import: %s", tableID)
	dirs := []string{
		migstate.pendingDir(tableID),
		migstate.doneDir(tableID),
		migstate.failedDir(tableID),
	}
	for _, dir := range dirs {
		log.Infof("Create dir: %s\n", dir)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("create dir %q: %w", dir, err)
		}
	}
	return nil
}

func (migstate *MigrationState) CleanState(tableID *TableID) error {
	log.Infof("Cleaning state associated with: %s", tableID)
	dir := migstate.tableDir(tableID)
	log.Infof("Remove dir: %s\n", dir)
	err := os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("remove dir %q: %w", dir, err)
	}
	return nil
}

func (migstate *MigrationState) GetLastBatch(tableID *TableID) (*Batch, error) {
	filePath := filepath.Join(migstate.tableDir(tableID), "last")
	batch, err := LoadBatchFrom(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return batch, fmt.Errorf("load batch: %w", err)
	}
	return batch, nil
}

func (migstate *MigrationState) PendingBatches(tableID *TableID) ([]*Batch, error) {
	dirPath := migstate.pendingDir(tableID)
	return migstate.getBatches(dirPath)
}

func (migstate *MigrationState) DoneBatches(tableID *TableID) ([]*Batch, error) {
	dirPath := migstate.doneDir(tableID)
	return migstate.getBatches(dirPath)
}

func (migstate *MigrationState) getBatches(dirPath string) ([]*Batch, error) {
	var batches []*Batch
	fileInfoList, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dirPath, err)
	}
	for _, fileInfo := range fileInfoList {
		fileName := filepath.Join(dirPath, fileInfo.Name())
		batch, err := LoadBatchFrom(fileName)
		if err != nil {
			return nil, fmt.Errorf("load batch: %w", err)
		}
		batches = append(batches, batch)
	}
	return batches, nil
}

type ImportDataProgress struct {
	Progress map[string]*TableImportDataProgress
}

func newImportDataProgress() *ImportDataProgress {
	return &ImportDataProgress{Progress: make(map[string]*TableImportDataProgress)}
}

type TableImportDataProgress struct {
	*TableID
	State              string
	NumRecordsImported int64
	PercentComlete     float32
}

func (migstate *MigrationState) GetImportDataProgress(tableIDList []*TableID) (*ImportDataProgress, error) {
	var err error
	result := newImportDataProgress()

	if tableIDList == nil {
		tableIDList, err = migstate.getTableIDList()
		if err != nil {
			return nil, fmt.Errorf("find table list: %w", err)
		}
	}
	for _, tableID := range tableIDList {
		progress, err := migstate.getTableImportDataProgress(tableID)
		if err != nil {
			return nil, fmt.Errorf("get table import data progress for %s: %w", tableID, err)
		}
		result.Progress[tableID.QualifiedName()] = progress
	}
	return result, nil
}

func (migstate *MigrationState) getTableImportDataProgress(tableID *TableID) (*TableImportDataProgress, error) {
	progress := &TableImportDataProgress{}
	progress.TableID = tableID

	lastBatch, err := migstate.GetLastBatch(tableID)
	if err != nil {
		return nil, fmt.Errorf("get last batch of %s: %w", tableID, err)
	}
	if lastBatch == nil {
		progress.State = "NOT_STARTED"
		return progress, nil
	}
	pendingBatches, err := migstate.PendingBatches(tableID)
	if err != nil {
		return nil, fmt.Errorf("find pending batches of table %s: %w", tableID, err)
	}
	doneBatches, err := migstate.DoneBatches(tableID)
	if err != nil {
		return nil, fmt.Errorf("find completed batches of table %s: %w", tableID, err)
	}
	if len(pendingBatches) > 0 {
		progress.State = "IN_PROGRESS"
		for _, batch := range pendingBatches {
			if batch.Err != "" {
				progress.State = "FAILED"
			}
		}
	} else {
		progress.State = "DONE"
	}
	for _, batch := range doneBatches {
		progress.NumRecordsImported += batch.NumRecordsImported
		progress.PercentComlete += batch.ProgressContribution
	}
	return progress, nil
}

func (migstate *MigrationState) getTableIDList() ([]*TableID, error) {
	result := []*TableID{}

	dbNames, err := migstate.getDatabaseNames()
	if err != nil {
		return nil, fmt.Errorf("find database names: %w", err)
	}
	for _, dbName := range dbNames {
		schemaNames, err := migstate.getSchemaNames(dbName)
		if err != nil {
			return nil, fmt.Errorf("find schemas in db %q: %w", dbName, err)
		}
		for _, schemaName := range schemaNames {
			tableNames, err := migstate.getTableNames(dbName, schemaName)
			if err != nil {
				return nil, fmt.Errorf("find tables from db %q and schema %q: %w", dbName, schemaName, err)
			}
			for _, tableName := range tableNames {
				tableID := NewTableID(dbName, schemaName, tableName)
				result = append(result, tableID)
			}
		}
	}
	return result, nil
}

func (migstate *MigrationState) MarkBatchPending(batch *Batch) error {
	// Save the batch.
	batchFileName := migstate.pendingBatchPath(batch)
	err := batch.SaveTo(batchFileName)
	if err != nil {
		return err
	}

	// Update the `last` link.
	lastPath := filepath.Join(migstate.tableDir(batch.TableID), "last")
	tmpPath := filepath.Join(migstate.tableDir(batch.TableID), "last.tmp")
	// Link() fails if a link with the same name already exists.
	err = os.Remove(tmpPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	err = os.Link(batchFileName, tmpPath)
	if err != nil {
		return err
	}
	err = os.Rename(tmpPath, lastPath)
	if err != nil {
		return err
	}
	return nil
}

func (migstate *MigrationState) MarkBatchDone(batch *Batch) error {
	batch.ImportAttempts++
	batch.Err = ""
	batchFileName := migstate.pendingBatchPath(batch)
	err := batch.SaveTo(batchFileName)
	if err != nil {
		return err
	}

	fromPath := migstate.pendingBatchPath(batch)
	toPath := migstate.doneBatchPath(batch)
	err = os.Rename(fromPath, toPath)
	if err != nil {
		return err
	}
	return nil
}

func (migstate *MigrationState) MarkBatchFailed(batch *Batch, e error) error {
	failedPath := migstate.failedBatchPath(batch)
	tmpFailedPath := failedPath + ".tmp"

	_, err := os.Stat(failedPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("stat %q: %s\n", failedPath, err)
		return err
	}
	if errors.Is(err, fs.ErrNotExist) {
		_ = os.Remove(tmpFailedPath)
		err = migstate.writeBatchDataToFile(batch, tmpFailedPath)
		if err != nil {
			fmt.Printf("write batch %d: %s\n", batch.BatchNumber, tmpFailedPath)
			return err
		}
		err = os.Rename(tmpFailedPath, failedPath)
		if err != nil {
			fmt.Printf("rename %q -> %q: %s\n", tmpFailedPath, failedPath, err)
			return err
		}
	}
	batch.FileName = failedPath
	batch.StartOffset = 0
	batch.EndOffset = -1
	batch.Header = "" // If there was a header, it will be dumped in the batch data file.
	batch.Err = e.Error()
	batch.ImportAttempts++

	batchFileName := migstate.pendingBatchPath(batch)
	err = batch.SaveTo(batchFileName)
	if err != nil {
		return err
	}
	return nil
}

func (migstate *MigrationState) writeBatchDataToFile(batch *Batch, fileName string) error {
	fh, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer fh.Close()

	r, err := batch.Reader()
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(fh, r)
	if err != nil {
		return err
	}
	return nil
}

func (migstate *MigrationState) tableDir(tableID *TableID) string {
	return filepath.Join(migstate.ExportDir, "import-data", tableID.DatabaseName, tableID.SchemaName, tableID.TableName)
}

func (migstate *MigrationState) pendingDir(tableID *TableID) string {
	return filepath.Join(migstate.tableDir(tableID), "pending")
}

func (migstate *MigrationState) doneDir(tableID *TableID) string {
	return filepath.Join(migstate.tableDir(tableID), "done")
}

func (migstate *MigrationState) failedDir(tableID *TableID) string {
	return filepath.Join(migstate.tableDir(tableID), "failed")
}

func (migstate *MigrationState) pendingBatchPath(batch *Batch) string {
	baseName := fmt.Sprintf("%s.%d", batch.TableID.TableName, batch.BatchNumber)
	return filepath.Join(migstate.pendingDir(batch.TableID), baseName)
}

func (migstate *MigrationState) doneBatchPath(batch *Batch) string {
	baseName := fmt.Sprintf("%s.%d", batch.TableID.TableName, batch.BatchNumber)
	return filepath.Join(migstate.doneDir(batch.TableID), baseName)
}

func (migstate *MigrationState) failedBatchPath(batch *Batch) string {
	baseName := fmt.Sprintf("%s.%d", batch.TableID.TableName, batch.BatchNumber)
	return filepath.Join(migstate.failedDir(batch.TableID), baseName)
}

func (migstate *MigrationState) getDatabaseNames() ([]string, error) {
	dirPath := filepath.Join(migstate.ExportDir, "import-data")
	return migstate.listDBEntries(dirPath)
}

func (migstate *MigrationState) getSchemaNames(dbName string) ([]string, error) {
	dirPath := filepath.Join(migstate.ExportDir, "import-data", dbName)
	return migstate.listDBEntries(dirPath)
}

func (migstate *MigrationState) getTableNames(dbName, schemaName string) ([]string, error) {
	dirPath := filepath.Join(migstate.ExportDir, "import-data", dbName, schemaName)
	return migstate.listDBEntries(dirPath)
}

func (migstate *MigrationState) listDBEntries(dirPath string) ([]string, error) {
	fileInfoList, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dirPath, err)
	}
	result := []string{}
	for _, fileInfo := range fileInfoList {
		result = append(result, fileInfo.Name())
	}
	return result, nil
}
