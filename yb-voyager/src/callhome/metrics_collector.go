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
	"sync"
)

// ImportDataMetricsCollector is responsible for collecting metrics about the current import run.
// It provides thread-safe access to increment and retrieve snapshot progress metrics.
type ImportDataMetricsCollector struct {
	sync.RWMutex          // embedded for thread-safe access
	runSnapshotTotalRows  int64
	runSnapshotTotalBytes int64
}

func NewImportDataMetricsCollector() *ImportDataMetricsCollector {
	return &ImportDataMetricsCollector{
		runSnapshotTotalRows:  0,
		runSnapshotTotalBytes: 0,
	}
}

func (c *ImportDataMetricsCollector) IncrementSnapshotProgress(rows int64, bytes int64) {
	c.Lock()
	defer c.Unlock()
	c.runSnapshotTotalRows += rows
	c.runSnapshotTotalBytes += bytes
}

func (c *ImportDataMetricsCollector) GetRunSnapshotTotalRows() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.runSnapshotTotalRows
}

func (c *ImportDataMetricsCollector) GetRunSnapshotTotalBytes() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.runSnapshotTotalBytes
}
