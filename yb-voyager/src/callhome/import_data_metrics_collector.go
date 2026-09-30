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
package callhome

import (
	"errors"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/anon"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// ImportDataMetricsCollector is responsible for collecting metrics about the current import run.
// It provides thread-safe access to increment and retrieve snapshot progress metrics.
type ImportDataMetricsCollector struct {
	sync.RWMutex               // embedded for thread-safe access
	snapshotTotalRows          int64
	snapshotTotalBytes         int64
	currentParallelConnections int
	cdcConflictCountPerTable   map[string]int64

	// anonymizer turns a table name into the form safe to put in a callhome payload. It is
	// set once at construction and never reassigned, so reading it needs no lock.
	anonymizer                  *anon.VoyagerAnonymizer
	nameTuplesToAnonymizeTables *utils.StructMap[sqlname.NameTuple, string]
}

func NewImportDataMetricsCollector(anonymizer *anon.VoyagerAnonymizer) *ImportDataMetricsCollector {
	return &ImportDataMetricsCollector{
		snapshotTotalRows:           0,
		snapshotTotalBytes:          0,
		currentParallelConnections:  0,
		cdcConflictCountPerTable:    make(map[string]int64),
		anonymizer:                  anonymizer,
		nameTuplesToAnonymizeTables: utils.NewStructMap[sqlname.NameTuple, string](),
	}
}

// AnonymizedTableName returns the table's name in the form safe to report, and false when it
// cannot be produced. It takes no lock, so IncrementConflictCountForTable can call it while
// holding the write lock (RWMutex is not reentrant).
func (c *ImportDataMetricsCollector) AnonymizedTableName(table sqlname.NameTuple) (string, bool, error) {
	c.RLock()
	defer c.RUnlock()
	anon, ok := c.nameTuplesToAnonymizeTables.Get(table)
	if ok {
		return anon, true, nil
	}
	if c.anonymizer == nil {
		return "", false, errors.New("no anonymizer")
	}
	schema, name := table.ForCatalogQuery()
	anonSchema, err := c.anonymizer.AnonymizeSchemaName(schema)
	if err != nil {
		return "", false, fmt.Errorf("anonymize table name: %w", err)
	}
	anonName, err := c.anonymizer.AnonymizeTableName(name)
	if err != nil {
		return "", false, fmt.Errorf("anonymize schema name: %w", err)
	}
	anonymized := anonSchema + "." + anonName
	c.nameTuplesToAnonymizeTables.Put(table, anonymized)
	return anonymized, true, nil
}

func (c *ImportDataMetricsCollector) IncrementSnapshotProgress(rows int64, bytes int64) {
	c.Lock()
	defer c.Unlock()
	c.snapshotTotalRows += rows
	c.snapshotTotalBytes += bytes
}

// IncrementConflictCountForTable counts one detected conflict against the table's anonymized
// name. A table that cannot be anonymized is counted under a shared placeholder rather than
// dropped, so the total stays accurate even when the per-table breakdown is incomplete.
func (c *ImportDataMetricsCollector) IncrementConflictCountForTable(table sqlname.NameTuple) {
	anonymized, ok, err := c.AnonymizedTableName(table)
	if err != nil {
		log.Warnf("callhome: %v", err)
	}
	if !ok {
		log.Warnf("callhome: no anonymized name for %s; counting its conflicts under %q", table.ForOutput(), constants.OBFUSCATE_STRING)
		anonymized = constants.OBFUSCATE_STRING
	}
	c.Lock()
	c.cdcConflictCountPerTable[anonymized]++
	c.Unlock()
}

func (c *ImportDataMetricsCollector) SetCurrentParallelConnections(connections int) {
	c.Lock()
	defer c.Unlock()
	c.currentParallelConnections = connections
}

func (c *ImportDataMetricsCollector) GetCurrentParallelConnections() int {
	c.RLock()
	defer c.RUnlock()
	return c.currentParallelConnections
}

func (c *ImportDataMetricsCollector) GetSnapshotTotalRows() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.snapshotTotalRows
}

func (c *ImportDataMetricsCollector) GetSnapshotTotalBytes() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.snapshotTotalBytes
}

func (c *ImportDataMetricsCollector) GetCdcConflictCountPerTable() map[string]int64 {
	c.RLock()
	defer c.RUnlock()
	out := make(map[string]int64, len(c.cdcConflictCountPerTable))
	for k, v := range c.cdcConflictCountPerTable {
		out[k] = v
	}
	return out
}
