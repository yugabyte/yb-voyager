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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

const cacheDepthSampleInterval = 25 * time.Millisecond

// Hooks are the cmd-package internals the framework cannot reach itself,
// injected as closures by the benchmark shim in cmd.
type Hooks struct {
	// Bootstrap prepares the import runtime for streaming from exportDir
	// (metaDB, name registry, migration UUID, target conf, table list, value
	// converter, partitioning map) with the given target-DB mock installed.
	// Called once per run, on a fresh copy of the artifact.
	Bootstrap func(exportDir string, mock tgtdb.TargetDB) error
	// StreamAll replays the entire event queue through the real streaming
	// code (the segment loop) and returns when the queue ends. This is the
	// timed region.
	StreamAll func() error
	// CacheDepth returns the current conflict-detection cache depth; sampled
	// concurrently while StreamAll runs.
	CacheDepth func() int
}

func (h Hooks) validate() error {
	if h.Bootstrap == nil || h.StreamAll == nil || h.CacheDepth == nil {
		return fmt.Errorf("cdcbench: all Hooks (Bootstrap, StreamAll, CacheDepth) must be provided")
	}
	return nil
}

// conflictHook counts "conflict detected" log lines emitted by the conflict
// detection cache, so per-run conflict expectations are asserted, not assumed.
type conflictHook struct{ count atomic.Int64 }

func (h *conflictHook) Levels() []log.Level { return []log.Level{log.InfoLevel} }
func (h *conflictHook) Fire(e *log.Entry) error {
	if strings.Contains(e.Message, "conflict detected") {
		h.count.Add(1)
	}
	return nil
}

var (
	hookOnce      sync.Once
	conflictCount = &conflictHook{}
)

// Run executes every registered workload as a sub-benchmark. Each b.N
// iteration replays the workload's artifact once through the real streaming
// path (fresh artifact copy per iteration). Reported metrics per workload:
// events/s, conflicts/op, batches/op, cache-depth-avg, cache-depth-max.
func Run(b *testing.B, hooks Hooks) {
	if err := hooks.validate(); err != nil {
		b.Fatal(err)
	}
	workloads := Workloads()
	if len(workloads) == 0 {
		b.Fatal("cdcbench: no workloads registered")
	}
	for _, w := range workloads {
		w := w
		b.Run(w.Name, func(b *testing.B) {
			runWorkload(b, w, hooks)
		})
	}
}

func runWorkload(b *testing.B, w Workload, hooks Hooks) {
	b.Helper()
	pristine := EnsureArtifact(b, w)

	execDelay := time.Duration(0)
	if ms := os.Getenv("CDCBENCH_EXEC_DELAY_MS"); ms != "" {
		n, err := strconv.Atoi(ms)
		if err != nil {
			b.Fatalf("cdcbench: invalid CDCBENCH_EXEC_DELAY_MS=%q", ms)
		}
		execDelay = time.Duration(n) * time.Millisecond
	}

	// production-like logging: Info level; output discarded unless
	// CDCBENCH_LOG_DIR asks for per-run log files
	log.SetLevel(log.InfoLevel)
	hookOnce.Do(func() { log.AddHook(conflictCount) })

	mock := &MockTargetDB{ExecDelay: execDelay}

	var (
		totalEvents    int64
		totalBatches   int64
		totalConflicts int64
		totalElapsed   time.Duration
		depthAvgSum    int64
		depthMax       int
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		runDir := filepath.Join(b.TempDir(), fmt.Sprintf("run%d", i))
		if err := copyDir(pristine, runDir); err != nil {
			b.Fatalf("cdcbench: copy artifact: %v", err)
		}
		logDest, closeLog := runLogDest(b, w, i)
		log.SetOutput(logDest)

		mock.reset()
		conflictCount.count.Store(0)
		if err := hooks.Bootstrap(runDir, mock); err != nil {
			b.Fatalf("cdcbench: bootstrap: %v", err)
		}

		samplerStop := make(chan struct{})
		samplerDone := make(chan struct{})
		var samples []int
		go func() {
			defer close(samplerDone)
			ticker := time.NewTicker(cacheDepthSampleInterval)
			defer ticker.Stop()
			for {
				select {
				case <-samplerStop:
					return
				case <-ticker.C:
					samples = append(samples, hooks.CacheDepth())
				}
			}
		}()

		b.StartTimer()
		start := time.Now()
		err := hooks.StreamAll()
		elapsed := time.Since(start)
		b.StopTimer()

		close(samplerStop)
		<-samplerDone
		closeLog()
		if err != nil {
			b.Fatalf("cdcbench: streaming: %v", err)
		}

		// per-run assertions: measured, not assumed
		events := mock.Events()
		conflicts := conflictCount.count.Load()
		if events != int64(w.ExpectedEvents) {
			b.Fatalf("cdcbench: run %d processed %d events, artifact has %d", i, events, w.ExpectedEvents)
		}
		if w.ExpectConflicts && conflicts == 0 {
			b.Fatalf("cdcbench: run %d expected conflicts but detected none", i)
		}
		if !w.ExpectConflicts && conflicts > 0 {
			b.Fatalf("cdcbench: run %d expected no conflicts but detected %d", i, conflicts)
		}

		totalEvents += events
		totalBatches += mock.Batches()
		totalConflicts += conflicts
		totalElapsed += elapsed
		sum := 0
		for _, d := range samples {
			sum += d
			if d > depthMax {
				depthMax = d
			}
		}
		if len(samples) > 0 {
			depthAvgSum += int64(sum / len(samples))
		}

		os.RemoveAll(runDir)
	}

	n := int64(b.N)
	b.ReportMetric(float64(totalEvents)/totalElapsed.Seconds(), "events/s")
	b.ReportMetric(float64(totalConflicts)/float64(n), "conflicts/op")
	b.ReportMetric(float64(totalBatches)/float64(n), "batches/op")
	b.ReportMetric(float64(depthAvgSum)/float64(n), "cache-depth-avg")
	b.ReportMetric(float64(depthMax), "cache-depth-max")
}

// runLogDest returns the logrus output for one run: a per-run file under
// CDCBENCH_LOG_DIR if set, io.Discard otherwise.
func runLogDest(b *testing.B, w Workload, run int) (io.Writer, func()) {
	dir := os.Getenv("CDCBENCH_LOG_DIR")
	if dir == "" {
		return io.Discard, func() {}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		b.Fatalf("cdcbench: create CDCBENCH_LOG_DIR: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-run%d.log", w.Name, run))
	f, err := os.Create(path)
	if err != nil {
		b.Fatalf("cdcbench: create run log %s: %v", path, err)
	}
	return f, func() { f.Close() }
}
