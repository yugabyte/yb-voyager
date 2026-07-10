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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

const (
	benchDBPrefix = "cdcbench_"

	streamingStartTimeout = 5 * time.Minute
	drainTimeout          = 10 * time.Minute
	exportExitTimeout     = 4 * time.Minute
	drainPollInterval     = 2 * time.Second
)

// artifactRoot returns the directory artifacts are cached in:
// CDCBENCH_ARTIFACT_DIR if set, else <this package's dir>/artifacts.
func artifactRoot() (string, error) {
	if dir := os.Getenv("CDCBENCH_ARTIFACT_DIR"); dir != "" {
		return dir, nil
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate cdcbench package directory; set CDCBENCH_ARTIFACT_DIR")
	}
	return filepath.Join(filepath.Dir(thisFile), "artifacts"), nil
}

type artifactManifest struct {
	Hash        string `json:"hash"`
	Events      int    `json:"events"`
	GeneratedAt string `json:"generated_at"`
	VoyagerBin  string `json:"voyager_bin"`
}

// EnsureArtifact returns the pristine export-dir for the workload, generating
// (and caching) it if needed. Skips the benchmark when generation
// prerequisites (yb-voyager binary with Debezium, Docker) are unavailable.
func EnsureArtifact(b *testing.B, w Workload) string {
	b.Helper()
	root, err := artifactRoot()
	if err != nil {
		b.Fatalf("cdcbench: %v", err)
	}
	dir := filepath.Join(root, fmt.Sprintf("%s-%s", w.Name, w.hash()))
	exportDir := filepath.Join(dir, "export")

	if os.Getenv("CDCBENCH_REGEN") != "1" {
		var manifest artifactManifest
		if raw, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
			if json.Unmarshal(raw, &manifest) == nil && manifest.Hash == w.hash() {
				return exportDir
			}
		}
	}

	voyagerBin := os.Getenv("CDCBENCH_VOYAGER_BIN")
	if voyagerBin == "" {
		voyagerBin, err = exec.LookPath("yb-voyager")
		if err != nil {
			b.Skipf("cdcbench: workload %q needs artifact generation but no yb-voyager binary found "+
				"(install voyager incl. debezium-server, or set CDCBENCH_VOYAGER_BIN)", w.Name)
		}
	}

	b.Logf("cdcbench: generating artifact for workload %q (%d events) — one-time, cached under %s", w.Name, w.ExpectedEvents, dir)
	if err := generateArtifact(b, w, voyagerBin, dir, exportDir); err != nil {
		os.RemoveAll(dir) // don't leave a half-built artifact behind
		b.Fatalf("cdcbench: generating artifact for workload %q: %v", w.Name, err)
	}
	return exportDir
}

// shared source-PG container, started lazily on first generation and
// terminated by cleanupSourceContainer (registered via b.Cleanup in Run).
// A sticky start error makes later workloads skip instead of re-dialing a
// broken Docker; the container itself is restartable after cleanup so
// -count=N runs that need regeneration keep working.
var (
	pgMu        sync.Mutex
	pgContainer testcontainers.TestContainer
	pgStartErr  error
)

func sourceContainer(b *testing.B) testcontainers.TestContainer {
	b.Helper()
	pgMu.Lock()
	defer pgMu.Unlock()
	if pgStartErr != nil {
		b.Skipf("cdcbench: cannot start source postgres container (is Docker running?): %v", pgStartErr)
	}
	if pgContainer == nil {
		c := testcontainers.NewTestContainer("postgresql", &testcontainers.ContainerConfig{
			ForLive: true, // wal_level=logical for CDC
		})
		if err := c.Start(context.Background()); err != nil {
			pgStartErr = err
			b.Skipf("cdcbench: cannot start source postgres container (is Docker running?): %v", err)
		}
		pgContainer = c
	}
	return pgContainer
}

// cleanupSourceContainer terminates the shared source container if it was
// started, so benchmark runs don't leave containers behind.
func cleanupSourceContainer() {
	pgMu.Lock()
	defer pgMu.Unlock()
	if pgContainer != nil {
		pgContainer.Terminate(context.Background())
		pgContainer = nil
	}
}

func generateArtifact(b *testing.B, w Workload, voyagerBin, dir, exportDir string) error {
	pg := sourceContainer(b)
	host, port, err := pg.GetHostPort()
	if err != nil {
		return fmt.Errorf("source container host/port: %w", err)
	}
	config := pg.GetConfig()
	dbName := benchDBPrefix + w.hash()

	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "export-data.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	connStr := func(db string) string {
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", config.User, config.Password, host, port, db)
	}

	// fresh database + schema + seed (seed rows land in the snapshot, not the queue).
	// DROP/CREATE DATABASE must be single-statement calls: they cannot run inside
	// the implicit transaction of a multi-statement simple-protocol script.
	if err := execSQLScript(connStr(config.DBName), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName)); err != nil {
		return fmt.Errorf("drop database %s: %w", dbName, err)
	}
	if err := execSQLScript(connStr(config.DBName), fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		return fmt.Errorf("create database %s: %w", dbName, err)
	}
	// drop leftover inactive replication slots from previous generations (cluster-wide)
	_ = execSQLScript(connStr(config.DBName),
		"SELECT pg_drop_replication_slot(slot_name) FROM pg_replication_slots WHERE active = false;")
	if err := execSQLScript(connStr(dbName), w.SchemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if err := execSQLScript(connStr(dbName), w.SeedSQL); err != nil {
		return fmt.Errorf("apply seed: %w", err)
	}

	// start real export data (snapshot-and-changes -> Debezium streaming)
	ctx, cancel := context.WithTimeout(context.Background(), streamingStartTimeout+drainTimeout+exportExitTimeout)
	defer cancel()
	export := exec.CommandContext(ctx, voyagerBin, "export", "data",
		"--export-dir", exportDir,
		"--source-db-type", "postgresql",
		"--source-db-host", host,
		"--source-db-port", fmt.Sprintf("%d", port),
		"--source-db-user", config.User,
		"--source-db-password", config.Password,
		"--source-db-name", dbName,
		"--source-db-schema", "public",
		"--export-type", "snapshot-and-changes",
		"--table-list", strings.Join(w.TableList, ","),
		"--disable-pb=true", "--yes")
	export.Env = append(os.Environ(), "YB_VOYAGER_SEND_DIAGNOSTICS=0")
	export.Stdout = logFile
	export.Stderr = logFile
	if err := export.Start(); err != nil {
		return fmt.Errorf("start export data: %w", err)
	}
	exportDone := make(chan error, 1)
	go func() { exportDone <- export.Wait() }()

	queueGlob := filepath.Join(exportDir, "data", "queue", "segment.*.ndjson")
	fail := func(stage string, err error) error {
		return fmt.Errorf("%s: %w\n--- export-data.log tail ---\n%s", stage, err, fileTail(logPath, 20))
	}

	// wait for the streaming phase (first queue segment appears)
	if err := pollUntil(streamingStartTimeout, drainPollInterval, exportDone, func() (bool, error) {
		matches, _ := filepath.Glob(queueGlob)
		return len(matches) > 0, nil
	}); err != nil {
		return fail("waiting for streaming phase", err)
	}
	time.Sleep(5 * time.Second) // let debezium settle into streaming

	b.Logf("cdcbench: [%s] streaming reached; running DML (%d events)...", w.Name, w.ExpectedEvents)
	if err := execSQLScript(connStr(dbName), w.DMLSQL); err != nil {
		return fail("apply DML", err)
	}

	// wait for all events to drain into queue segments (count stable and >= expected)
	prev, stable := -1, 0
	if err := pollUntil(drainTimeout, drainPollInterval, exportDone, func() (bool, error) {
		count, err := countQueueEvents(queueGlob)
		if err != nil {
			return false, err
		}
		if count >= w.ExpectedEvents && count == prev {
			stable++
		} else {
			stable = 0
		}
		prev = count
		return stable >= 3, nil
	}); err != nil {
		return fail(fmt.Sprintf("waiting for %d events to drain (last count %d)", w.ExpectedEvents, prev), err)
	}

	// cutover terminates the queue with a cutover event and stops the exporter
	cutover := exec.CommandContext(ctx, voyagerBin, "initiate", "cutover", "to", "target",
		"--export-dir", exportDir, "--prepare-for-fall-back", "false", "--yes")
	cutover.Env = export.Env
	cutover.Stdout = logFile
	cutover.Stderr = logFile
	if err := cutover.Run(); err != nil {
		return fail("initiate cutover to target", err)
	}
	select {
	case <-time.After(exportExitTimeout):
		_ = export.Process.Kill()
		return fail("export did not exit after cutover", fmt.Errorf("timeout after %s", exportExitTimeout))
	case err := <-exportDone:
		if err != nil {
			// exporter exiting non-zero after cutover is tolerated as long as the
			// queue is complete; completeness is validated below
			b.Logf("cdcbench: [%s] export exited with: %v (validating artifact anyway)", w.Name, err)
		}
	}

	// make the artifact self-contained for the import side
	if err := patchNameRegistryYBNames(exportDir); err != nil {
		return fmt.Errorf("patch name registry: %w", err)
	}

	manifest := artifactManifest{
		Hash:        w.hash(),
		Events:      w.ExpectedEvents,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		VoyagerBin:  voyagerBin,
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0644); err != nil {
		return err
	}
	b.Logf("cdcbench: [%s] artifact generated at %s", w.Name, dir)
	return nil
}

// execSQLScript runs a (possibly multi-statement, DO-block-containing) SQL
// script using the simple query protocol.
func execSQLScript(connStr, script string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	// PgConn().Exec uses the simple protocol, which supports multi-statement scripts
	_, err = conn.PgConn().Exec(ctx, script).ReadAll()
	return err
}

// pollUntil polls cond until it returns true, the timeout elapses, or the
// export process exits early (procDone).
func pollUntil(timeout, interval time.Duration, procDone <-chan error, cond func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		ok, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %s", timeout)
		}
		select {
		case err := <-procDone:
			return fmt.Errorf("export data exited early: %v", err)
		case <-ticker.C:
		}
	}
}

func countQueueEvents(glob string) (int, error) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		total += strings.Count(string(raw), "\n")
	}
	return total, nil
}

// patchNameRegistryYBNames fills the YB-side names in the artifact's name
// registry. Export-only artifacts have YBTableNames=null; a production
// `import data` run fills them by querying the target. For plain lowercase
// tables the content is mechanical: identical to the source names.
func patchNameRegistryYBNames(exportDir string) error {
	path := filepath.Join(exportDir, "metainfo", "name_registry.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var reg map[string]any
	if err := json.Unmarshal(raw, &reg); err != nil {
		return err
	}
	if reg["YBTableNames"] != nil {
		return nil
	}
	reg["YBSchemaNames"] = reg["SourceDBSchemaNames"]
	reg["DefaultYBSchemaName"] = reg["DefaultSourceDBSchemaName"]
	reg["YBTableNames"] = reg["SourceDBTableNames"]
	out, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

func fileTail(path string, lines int) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(cannot read %s: %v)", path, err)
	}
	all := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}

// copyDir recursively copies a pristine artifact into a per-run directory
// (benchmark runs mutate the artifact's metaDB and segment bookkeeping).
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	})
}
