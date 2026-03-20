/*
Copyright (c) YugabyteDB, Inc.

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
package exportdata

import (
	"context"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type PGDumpExporter struct {
	ectx *ExportContext

	tableList              []sqlname.NameTuple
	tablesProgressMetadata map[string]*utils.TableProgressMetadata
	snapshotStatusFile     *jsonfile.JsonFile[ExportSnapshotStatus]
}

func NewPGDumpExporter(ectx *ExportContext) *PGDumpExporter {
	return &PGDumpExporter{
		ectx:      ectx,
		tableList: append([]sqlname.NameTuple{}, ectx.FinalTableList...),
	}
}

func (e *PGDumpExporter) Setup() error {
	err := StoreTableListInMSR(e.ectx.MetaDB, e.ectx.Source, e.tableList)
	if err != nil {
		return fmt.Errorf("store table list in MSR: %w", err)
	}

	getRenamedTableTupleFn := func(table sqlname.NameTuple) (sqlname.NameTuple, bool) {
		return GetRenamedTableTuple(e.ectx.MetaDB, e.ectx.Source, table)
	}
	e.tablesProgressMetadata, e.snapshotStatusFile = InitializeExportTableMetadata(
		e.ectx.ExportDir, e.tableList, getRenamedTableTupleFn)

	err = e.ectx.MetaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.SnapshotMechanism = "pg_dump"
	})
	if err != nil {
		return fmt.Errorf("update SnapshotMechanism: update migration status record: %w", err)
	}

	log.Infof("Export table metadata: %s", spew.Sdump(e.tablesProgressMetadata))
	updateTableApproxRowCount(e.ectx.Source, e.ectx.ExportDir, e.tablesProgressMetadata)

	if e.ectx.Source.DBType == constants.POSTGRESQL {
		msr, err := e.ectx.MetaDB.GetMigrationStatusRecord()
		if err != nil {
			return fmt.Errorf("error getting migration status record: %w", err)
		}
		colToSeqMap, err := FetchOrRetrieveColToSeqMap(e.ectx.MetaDB, e.ectx.SourceDB, e.ectx.ExporterRole, e.ectx.StartClean, msr, e.tableList)
		if err != nil {
			return fmt.Errorf("error fetching the column to sequence mapping: %w", err)
		}
		for _, seq := range colToSeqMap {
			seqTuple, err := namereg.NameReg.LookupTableName(seq)
			if err != nil {
				return fmt.Errorf("lookup for sequence failed: %s: %w", seq, err)
			}
			e.tableList = append(e.tableList, seqTuple)
		}
	}

	return nil
}

func (e *PGDumpExporter) ExportSnapshot(ctx context.Context) error {
	if e.ectx.ExporterRole == constants.SOURCE_DB_EXPORTER_ROLE {
		event := e.createSnapshotExportStartedEvent()
		e.ectx.ControlPlane.SnapshotExportStarted(&event)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	exportDataStart := make(chan bool)
	quitChan := make(chan bool)
	exportSuccessChan := make(chan bool, 1)
	go func() {
		q := <-quitChan
		if q {
			log.Infoln("Cancel() being called, within PGDumpExporter.ExportSnapshot()")
			cancel()
			time.Sleep(time.Second * 5)
			utils.ErrExit("yb-voyager encountered internal error: "+
				"Check: %s/logs/yb-voyager-export-data.log for more details.", e.ectx.ExportDir)
		}
	}()

	fmt.Printf("Initiating data export.\n")
	utils.WaitGroup.Add(1)
	go e.ectx.SourceDB.ExportData(ctx, e.ectx.ExportDir, e.tableList, quitChan, exportDataStart, exportSuccessChan, e.ectx.TablesColumnList, "")
	<-exportDataStart

	if e.ectx.ExporterRole == constants.SOURCE_DB_EXPORTER_ROLE {
		exportDataTableMetrics := CreateUpdateExportedRowCountEventList(
			utils.GetSortedKeys(e.tablesProgressMetadata), e.tablesProgressMetadata, e.ectx.MigrationUUID)
		e.ectx.ControlPlane.UpdateExportedRowCount(exportDataTableMetrics)
	}

	UpdateFilePaths(e.ectx.Source, e.ectx.ExportDir, e.tablesProgressMetadata)
	utils.WaitGroup.Add(1)
	ExportDataStatus(ctx, e.tablesProgressMetadata, quitChan, exportSuccessChan, e.ectx.DisablePb,
		e.ectx.SourceDB, e.ectx.Source.DBType, e.ectx.ExporterRole, e.ectx.ControlPlane, e.ectx.MigrationUUID, e.snapshotStatusFile)

	utils.WaitGroup.Wait()
	if ctx.Err() != nil {
		return fmt.Errorf("ctx error(PGDumpExporter.ExportSnapshot): %w", ctx.Err())
	}

	return nil
}

func (e *PGDumpExporter) PostProcessing() error {
	e.ectx.SourceDB.ExportDataPostProcessing(e.ectx.ExportDir, e.tablesProgressMetadata)

	if e.ectx.Source.DBType == constants.POSTGRESQL {
		RenameDatafileDescriptor(e.ectx.MetaDB, e.ectx.Source, e.ectx.ExportDir)
	}

	DisplayExportedRowCountSnapshot(e.ectx.MetaDB, e.ectx.Source, e.ectx.ExportDir, false)

	if e.ectx.ExporterRole == constants.SOURCE_DB_EXPORTER_ROLE {
		event := e.createSnapshotExportCompletedEvent()
		e.ectx.ControlPlane.SnapshotExportCompleted(&event)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Control plane event helpers
// ---------------------------------------------------------------------------

func (e *PGDumpExporter) createSnapshotExportStartedEvent() cp.SnapshotExportStartedEvent {
	result := cp.SnapshotExportStartedEvent{}
	e.initBaseSourceEvent(&result.BaseEvent, "EXPORT DATA")
	return result
}

func (e *PGDumpExporter) createSnapshotExportCompletedEvent() cp.SnapshotExportCompletedEvent {
	result := cp.SnapshotExportCompletedEvent{}
	e.initBaseSourceEvent(&result.BaseEvent, "EXPORT DATA")
	return result
}

func (e *PGDumpExporter) initBaseSourceEvent(bev *cp.BaseEvent, eventType string) {
	*bev = cp.BaseEvent{
		EventType:     eventType,
		MigrationUUID: e.ectx.MigrationUUID,
		DBType:        e.ectx.Source.DBType,
		DatabaseName:  e.ectx.Source.DBName,
		SchemaNames:   cp.GetSchemaList(sqlname.JoinIdentifiersUnquoted(e.ectx.Source.Schemas, "|")),
		DBIP:          utils.LookupIP(e.ectx.Source.Host),
		Port:          e.ectx.Source.Port,
		DBVersion:     e.ectx.Source.DBVersion,
	}
}
