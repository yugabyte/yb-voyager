package main

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
			return err
		}
	}
	return nil
}

func (migstate *MigrationState) GetLastBatch(tableID *TableID) (*Batch, error) {
	filePath := filepath.Join(migstate.tableDir(tableID), "last")
	batch, err := LoadBatchFrom(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return batch, err
}

func (migstate *MigrationState) PendingBatches(tableID *TableID) ([]*Batch, error) {
	var batches []*Batch

	pendingDir := migstate.pendingDir(tableID)
	fileInfoList, err := ioutil.ReadDir(pendingDir)
	if err != nil {
		return nil, err
	}
	for _, fileInfo := range fileInfoList {
		fileName := filepath.Join(pendingDir, fileInfo.Name())
		batch, err := LoadBatchFrom(fileName)
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	return batches, nil
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
