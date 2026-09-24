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

package cdcbench

import (
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// MockTargetDB mocks ONLY ExecuteBatch on the target DB: batches succeed
// without touching a database, and events/batches are counted. Every other
// TargetDB method comes from the embedded TargetYugabyteDB; none of them is
// reachable from the streaming path when ExecuteBatch succeeds, and an
// unexpected call panics loudly (nil receiver internals) rather than
// silently faking behavior.
type MockTargetDB struct {
	tgtdb.TargetYugabyteDB
	// ExecDelay simulates target batch-commit latency (CDCBENCH_EXEC_DELAY_MS).
	ExecDelay time.Duration

	// metadata backs Query/QueryRow/Exec/WithTx for the streaming bootstrap's
	// live-migration bookkeeping (see mock_metadata.go)
	metadata *sql.DB
	// uniqueIndexes: per "schema.table" unique indexes from the artifact manifest
	uniqueIndexes map[string][]tgtdb.UniqueIndex

	batches atomic.Int64
	events  atomic.Int64
}

// NewMockTargetDB returns a mock whose ExecuteBatch succeeds without a
// database and whose streaming-metadata queries run against an in-memory
// store. uniqueIndexes (per "schema.table", from the artifact manifest)
// answers GetTableToUniqueIndexesMap, which production fetches live from the
// import target for conflict detection.
func NewMockTargetDB(execDelay time.Duration, uniqueIndexes map[string][]tgtdb.UniqueIndex) (*MockTargetDB, error) {
	metadata, err := newMockMetadataStore()
	if err != nil {
		return nil, err
	}
	return &MockTargetDB{ExecDelay: execDelay, metadata: metadata, uniqueIndexes: uniqueIndexes}, nil
}

// GetTableToUniqueIndexesMap serves the unique-index metadata captured in the
// artifact manifest, keyed back onto the requested table tuples.
func (m *MockTargetDB) GetTableToUniqueIndexesMap(tableList []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex], error) {
	result := utils.NewStructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]()
	for _, table := range tableList {
		if indexes, ok := m.uniqueIndexes[table.AsQualifiedCatalogName()]; ok {
			result.Put(table, indexes)
		}
	}
	return result, nil
}

func (m *MockTargetDB) Close() {
	if m.metadata != nil {
		_ = m.metadata.Close() // mock teardown; close error is not actionable
	}
}

func (m *MockTargetDB) ExecuteBatch(migrationUUID uuid.UUID, batch *tgtdb.EventBatch) error {
	if m.ExecDelay > 0 {
		time.Sleep(m.ExecDelay)
	}
	m.batches.Add(1)
	m.events.Add(int64(len(batch.Events)))
	return nil
}

func (m *MockTargetDB) Batches() int64 { return m.batches.Load() }
func (m *MockTargetDB) Events() int64  { return m.events.Load() }
