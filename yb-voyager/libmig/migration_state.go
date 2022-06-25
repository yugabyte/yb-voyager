package main

import (
	"errors"
	"fmt"
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
	return LoadBatchFrom(filePath)
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

func (migstate *MigrationState) NewPendingBatch(tableID *TableID, batch *Batch) error {
	// Save the batch.
	batchFileName := migstate.pendingBatchPath(tableID, batch)
	err := batch.SaveTo(batchFileName)
	if err != nil {
		return err
	}

	// Update the `last` link.
	lastPath := filepath.Join(migstate.tableDir(tableID), "last")
	tmpPath := filepath.Join(migstate.tableDir(tableID), "last.tmp")
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

func (migstate *MigrationState) BatchDone(tableID *TableID, batch *Batch) error {
	fromPath := migstate.pendingBatchPath(tableID, batch)
	toPath := migstate.doneBatchPath(tableID, batch)
	err := os.Rename(fromPath, toPath)
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

func (migstate *MigrationState) pendingBatchPath(tableID *TableID, batch *Batch) string {
	baseName := fmt.Sprintf("%s.%d", tableID.TableName, batch.BatchNumber)
	return filepath.Join(migstate.pendingDir(tableID), baseName)
}

func (migstate *MigrationState) doneBatchPath(tableID *TableID, batch *Batch) string {
	baseName := fmt.Sprintf("%s.%d", tableID.TableName, batch.BatchNumber)
	return filepath.Join(migstate.doneDir(tableID), baseName)
}

func (migstate *MigrationState) failedBatchPath(tableID *TableID, batch *Batch) string {
	baseName := fmt.Sprintf("%s.%d", tableID.TableName, batch.BatchNumber)
	return filepath.Join(migstate.failedDir(tableID), baseName)
}
