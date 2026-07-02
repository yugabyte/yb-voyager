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
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
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

	batches atomic.Int64
	events  atomic.Int64
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

func (m *MockTargetDB) reset() {
	m.batches.Store(0)
	m.events.Store(0)
}
