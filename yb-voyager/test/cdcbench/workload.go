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

// Package cdcbench is a benchmark framework for the live-migration CDC ingest
// path (import side). Each workload's change events are generated once by a
// real `yb-voyager export data` run (Debezium) against a testcontainers
// PostgreSQL source, cached on disk, and then replayed through the real
// import streaming code with only TargetDB.ExecuteBatch mocked.
//
// The framework deliberately knows nothing about the cmd package: the pieces
// that need cmd internals (bootstrap, the segment streaming loop, conflict
// cache depth) are injected as Hooks by a thin test shim in cmd.
package cdcbench

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// artifactFormatVersion invalidates all cached artifacts when the generation
// logic changes in a way that affects artifact content.
const artifactFormatVersion = "2" // v2: manifest carries unique-index metadata

// Workload defines one benchmark scenario. The SQL fields fully determine the
// generated artifact; changing any of them invalidates the cached artifact
// for the workload (content-hash keyed).
type Workload struct {
	// Name is the sub-benchmark name: run one workload with
	//   go test -tags cdc_benchmark -bench 'CDCIngest/<Name>' ./cmd/
	Name string
	// SchemaSQL creates the source tables (include PKs, unique constraints,
	// and ALTER TABLE ... REPLICA IDENTITY FULL for before-images).
	SchemaSQL string
	// SeedSQL populates the initial rows (exported as the snapshot, not as
	// change events).
	SeedSQL string
	// DMLSQL runs while Debezium streams; every row change becomes one CDC
	// event in the artifact.
	DMLSQL string
	// TableList is passed to `export data --table-list`.
	TableList []string
	// ExpectedEvents is the exact number of change events DMLSQL produces.
	// Used to wait for the export to drain and asserted during benchmark runs.
	ExpectedEvents int
	// ExpectConflicts asserts conflict detection behavior per run:
	// false => exactly zero conflicts detected, true => at least one.
	// (Exact counts are deliberately not asserted: re-detection on rescan
	// makes them implementation- and timing-dependent.)
	ExpectConflicts bool
}

// Workload name prefixes encode intent (and group the summary table):
//
//	oltp-     realistic customer pattern, measured for throughput
//	schema-   schema-shape probe (index count, row width, key structure)
//	edge-     degenerate op-mix corner case
//	conflict- engineered, semantically REAL conflicts
//
// Names describe the workload's construction, never its current detection
// outcome; workloads asserting known false positives of current semantics say
// so in their registration comment instead.
var categoryOrder = map[string]int{"oltp": 0, "schema": 1, "edge": 2, "conflict": 3}

func (w Workload) category() string {
	prefix, _, _ := strings.Cut(w.Name, "-")
	return prefix
}

func (w Workload) validate() error {
	switch {
	case w.Name == "":
		return fmt.Errorf("workload name is empty")
	case w.SchemaSQL == "" || w.SeedSQL == "" || w.DMLSQL == "":
		return fmt.Errorf("workload %q: SchemaSQL, SeedSQL and DMLSQL are all required", w.Name)
	case len(w.TableList) == 0:
		return fmt.Errorf("workload %q: TableList is empty", w.Name)
	case w.ExpectedEvents <= 0:
		return fmt.Errorf("workload %q: ExpectedEvents must be > 0", w.Name)
	}
	if _, known := categoryOrder[w.category()]; !known {
		return fmt.Errorf("workload %q: name must start with one of the category prefixes (oltp-, schema-, edge-, conflict-)", w.Name)
	}
	return nil
}

// hash returns a short content hash identifying the artifact this workload
// definition produces.
func (w Workload) hash() string {
	h := sha256.New()
	for _, part := range []string{
		artifactFormatVersion, w.Name, w.SchemaSQL, w.SeedSQL, w.DMLSQL,
		strings.Join(w.TableList, ","), fmt.Sprintf("%d", w.ExpectedEvents),
	} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}

var registry = map[string]Workload{}

// Register adds a workload to the benchmark suite. Call from an init()
// function; duplicate names panic.
func Register(w Workload) {
	if err := w.validate(); err != nil {
		panic(fmt.Sprintf("cdcbench: invalid workload: %v", err))
	}
	if _, exists := registry[w.Name]; exists {
		panic(fmt.Sprintf("cdcbench: workload %q registered twice", w.Name))
	}
	registry[w.Name] = w
}

// Workloads returns all registered workloads sorted by name.
func Workloads() []Workload {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Workload, 0, len(names))
	for _, name := range names {
		result = append(result, registry[name])
	}
	return result
}
