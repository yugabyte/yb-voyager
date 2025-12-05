//go:build integration_voyager_command

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
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// ============================================================
// CORE TEST HELPER
// ============================================================

// LiveMigrationTest manages the entire test lifecycle
type LiveMigrationTest struct {
	config          *TestConfig
	exportDir       string
	sourceContainer testcontainers.TestContainer
	targetContainer testcontainers.TestContainer
	exportCmd       *testutils.VoyagerCommandRunner
	importCmd       *testutils.VoyagerCommandRunner
	metaDB          *metadb.MetaDB
	ctx             context.Context
	t               *testing.T
}

// TestConfig holds all configuration upfront
type TestConfig struct {
	// Container configs
	SourceDB ContainerConfig
	TargetDB ContainerConfig

	// Schema setup
	SchemaName     string
	SchemaSQL      []string // CREATE statements
	InitialDataSQL []string // INSERT statements
	CleanupSQL     []string // DROP statements

	// Default command arguments (can be overridden per call)
	DefaultExportArgs map[string]string
	DefaultImportArgs map[string]string

	// Timeouts and retries
	SnapshotTimeout  time.Duration
	StreamingTimeout time.Duration
	CutoverTimeout   time.Duration
}

type ContainerConfig struct {
	Type    string // "postgresql", "yugabytedb", etc.
	ForLive bool   // Whether to configure for live migration
}

// ============================================================
// INITIALIZATION
// ============================================================

// NewLiveMigrationTest creates a new test helper
func NewLiveMigrationTest(t *testing.T, config *TestConfig) *LiveMigrationTest {
	return &LiveMigrationTest{
		config:    config,
		exportDir: testutils.CreateTempExportDir(),
		ctx:       context.Background(),
		t:         t,
	}
}

// SetupContainers starts source and target containers
func (lm *LiveMigrationTest) SetupContainers(ctx context.Context) error {
	lm.ctx = ctx

	// Start source container
	containerConfig := &testcontainers.ContainerConfig{
		ForLive: lm.config.SourceDB.ForLive,
	}
	lm.sourceContainer = testcontainers.NewTestContainer(lm.config.SourceDB.Type, containerConfig)
	if err := lm.sourceContainer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start source container: %w", err)
	}

	// Start target container
	targetContainerConfig := &testcontainers.ContainerConfig{
		ForLive: lm.config.TargetDB.ForLive,
	}
	lm.targetContainer = testcontainers.NewTestContainer(lm.config.TargetDB.Type, targetContainerConfig)
	if err := lm.targetContainer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start target container: %w", err)
	}

	return nil
}

// SetupSchema creates schema on source and target, registers cleanup
func (lm *LiveMigrationTest) SetupSchema() error {
	// Execute schema SQL on source
	if len(lm.config.SchemaSQL) > 0 {
		lm.sourceContainer.ExecuteSqls(lm.config.SchemaSQL...)
	}

	// Execute initial data SQL on source
	if len(lm.config.InitialDataSQL) > 0 {
		lm.sourceContainer.ExecuteSqls(lm.config.InitialDataSQL...)
	}

	// Execute schema SQL on target (usually just CREATE statements, no data)
	if len(lm.config.SchemaSQL) > 0 {
		lm.targetContainer.ExecuteSqls(lm.config.SchemaSQL...)
	}

	return nil
}

// Cleanup runs all cleanup operations (called via defer)
func (lm *LiveMigrationTest) Cleanup() {
	// Execute cleanup SQL
	if len(lm.config.CleanupSQL) > 0 {
		lm.sourceContainer.ExecuteSqls(lm.config.CleanupSQL...)
		lm.targetContainer.ExecuteSqls(lm.config.CleanupSQL...)
	}

	// Remove export directory
	testutils.RemoveTempExportDir(lm.exportDir)
}

// ============================================================
// MIGRATION COMMANDS
// ============================================================

// StartExportData starts export data command
func (lm *LiveMigrationTest) StartExportData(async bool, extraArgs map[string]string) error {
	// Build args from defaults and extras
	args := lm.buildArgs(lm.config.DefaultExportArgs, extraArgs)
	args = append(args, "--export-dir", lm.exportDir, "--yes")

	// Convert map args to slice
	argSlice := lm.mapArgsToSlice(args)

	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for export to start
		}
	}

	lm.exportCmd = testutils.NewVoyagerCommandRunner(lm.sourceContainer, "export data", argSlice, onStart, async)
	return lm.exportCmd.Run()
}

// StartImportData starts import data command
func (lm *LiveMigrationTest) StartImportData(async bool, extraArgs map[string]string) error {
	// Build args from defaults and extras
	args := lm.buildArgs(lm.config.DefaultImportArgs, extraArgs)
	args = append(args, "--export-dir", lm.exportDir, "--yes")

	// Convert map args to slice
	argSlice := lm.mapArgsToSlice(args)

	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for import to start
		}
	}

	lm.importCmd = testutils.NewVoyagerCommandRunner(lm.targetContainer, "import data", argSlice, onStart, async)
	return lm.importCmd.Run()
}

// StopExportData stops the running export data command
func (lm *LiveMigrationTest) StopExportData() error {
	if lm.exportCmd == nil {
		return fmt.Errorf("export command not started")
	}
	return lm.exportCmd.Kill()
}

// StopImportData stops the running import data command
func (lm *LiveMigrationTest) StopImportData() error {
	if lm.importCmd == nil {
		return fmt.Errorf("import command not started")
	}
	return lm.importCmd.Kill()
}

// InitiateCutover initiates cutover to target
func (lm *LiveMigrationTest) InitiateCutover(prepareForFallback bool, extraArgs map[string]string) error {
	args := []string{
		"--export-dir", lm.exportDir,
		"--yes",
		"--prepare-for-fall-back", fmt.Sprintf("%t", prepareForFallback),
	}

	// Add extra args
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	cutoverCmd := testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", args, nil, false)
	return cutoverCmd.Run()
}

// ============================================================
// WAIT CONDITIONS
// ============================================================

// WaitForSnapshotComplete waits until snapshot phase is done
func (lm *LiveMigrationTest) WaitForSnapshotComplete(tableName string, expectedRows int64) error {
	timeout := lm.config.SnapshotTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ok := utils.RetryWorkWithTimeout(1*time.Second, timeout, func() bool {
		return lm.snapshotPhaseCompleted(tableName, expectedRows)
	})

	if !ok {
		return fmt.Errorf("snapshot phase did not complete within %v", timeout)
	}
	return nil
}

// WaitForStreamingComplete waits until streaming events are processed
func (lm *LiveMigrationTest) WaitForStreamingComplete(tableName string, expectedInserts int64) error {
	timeout := lm.config.StreamingTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ok := utils.RetryWorkWithTimeout(1*time.Second, timeout, func() bool {
		return lm.streamingPhaseCompleted(tableName, expectedInserts)
	})

	if !ok {
		return fmt.Errorf("streaming phase did not complete within %v", timeout)
	}
	return nil
}

// WaitForCutoverComplete waits until cutover is done
func (lm *LiveMigrationTest) WaitForCutoverComplete() error {
	timeout := lm.config.CutoverTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		var err error
		lm.metaDB, err = metadb.NewMetaDB(lm.exportDir)
		if err != nil {
			return fmt.Errorf("failed to initialize meta db: %w", err)
		}
	}

	ok := utils.RetryWorkWithTimeout(1*time.Second, timeout, func() bool {
		return lm.getCutoverStatus() == COMPLETED
	})

	if !ok {
		return fmt.Errorf("cutover did not complete within %v", timeout)
	}
	return nil
}

// WaitForImportError waits for import command to fail with error
func (lm *LiveMigrationTest) WaitForImportError(expectedError string) error {
	if lm.importCmd == nil {
		return fmt.Errorf("import command not started")
	}

	err := lm.importCmd.Wait()
	if err == nil {
		return fmt.Errorf("expected import to fail but it succeeded")
	}

	// Check if error message contains expected substring
	if expectedError != "" {
		stderr := lm.importCmd.Stderr()
		if !contains(stderr, expectedError) {
			return fmt.Errorf("error message does not contain expected string '%s': %s", expectedError, stderr)
		}
	}

	return nil
}

// WaitForImportExit waits for import command to exit (error or success)
func (lm *LiveMigrationTest) WaitForImportExit() error {
	if lm.importCmd == nil {
		return fmt.Errorf("import command not started")
	}

	// Wait and ignore the error (can be success or failure)
	_ = lm.importCmd.Wait()
	return nil
}

// ============================================================
// DATA OPERATIONS & VALIDATION
// ============================================================

// ExecuteOnSource executes SQL statements on source database
func (lm *LiveMigrationTest) ExecuteOnSource(sqlStatements ...string) error {
	lm.sourceContainer.ExecuteSqls(sqlStatements...)
	return nil
}

// ExecuteOnTarget executes SQL statements on target database
func (lm *LiveMigrationTest) ExecuteOnTarget(sqlStatements ...string) error {
	lm.targetContainer.ExecuteSqls(sqlStatements...)
	return nil
}

// ValidateDataConsistency compares data between source and target
func (lm *LiveMigrationTest) ValidateDataConsistency(tables []string, orderBy string) error {
	sourceConn, err := lm.sourceContainer.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get source connection: %w", err)
	}

	targetConn, err := lm.targetContainer.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get target connection: %w", err)
	}

	for _, table := range tables {
		if err := testutils.CompareTableData(lm.ctx, sourceConn, targetConn, table, orderBy); err != nil {
			return fmt.Errorf("table data mismatch for %s: %w", table, err)
		}
	}

	return nil
}

// WithSourceConn provides source database connection to callback
func (lm *LiveMigrationTest) WithSourceConn(fn func(*sql.DB) error) error {
	conn, err := lm.sourceContainer.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get source connection: %w", err)
	}
	return fn(conn)
}

// WithTargetConn provides target database connection to callback
func (lm *LiveMigrationTest) WithTargetConn(fn func(*sql.DB) error) error {
	conn, err := lm.targetContainer.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get target connection: %w", err)
	}
	return fn(conn)
}

// WithSourceTargetConn provides both connections to callback
func (lm *LiveMigrationTest) WithSourceTargetConn(fn func(source, target *sql.DB) error) error {
	sourceConn, err := lm.sourceContainer.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get source connection: %w", err)
	}

	targetConn, err := lm.targetContainer.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get target connection: %w", err)
	}

	return fn(sourceConn, targetConn)
}

// ============================================================
// HELPER/INSPECTION METHODS
// ============================================================

// GetDataMigrationReport retrieves the migration report
func (lm *LiveMigrationTest) GetDataMigrationReport() (*DataMigrationReport, error) {
	err := testutils.NewVoyagerCommandRunner(nil, "get data-migration-report", []string{
		"--export-dir", lm.exportDir,
		"--output-format", "json",
		"--source-db-password", lm.sourceContainer.GetConfig().Password,
		"--target-db-password", lm.targetContainer.GetConfig().Password,
	}, nil, true).Run()
	if err != nil {
		return nil, fmt.Errorf("get data-migration-report command failed: %w", err)
	}

	reportFilePath := filepath.Join(lm.exportDir, "reports", "data-migration-report.json")
	if !utils.FileOrFolderExists(reportFilePath) {
		return nil, fmt.Errorf("report file does not exist")
	}

	jsonFile := jsonfile.NewJsonFile[[]*rowData](reportFilePath)
	rowData, err := jsonFile.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading data-migration-report: %w", err)
	}

	return &DataMigrationReport{RowData: *rowData}, nil
}

// GetExportDir returns the export directory path
func (lm *LiveMigrationTest) GetExportDir() string {
	return lm.exportDir
}

// GetImportCommandStderr gets stderr from import command
func (lm *LiveMigrationTest) GetImportCommandStderr() string {
	if lm.importCmd == nil {
		return ""
	}
	return lm.importCmd.Stderr()
}

// GetImportCommandStdout gets stdout from import command
func (lm *LiveMigrationTest) GetImportCommandStdout() string {
	if lm.importCmd == nil {
		return ""
	}
	return lm.importCmd.Stdout()
}

// ============================================================
// INTERNAL HELPERS
// ============================================================

// buildArgs merges default args with extra args (extra args override defaults)
func (lm *LiveMigrationTest) buildArgs(defaults, extras map[string]string) []string {
	// Start with defaults
	merged := make(map[string]string)
	for k, v := range defaults {
		merged[k] = v
	}

	// Override with extras
	for k, v := range extras {
		merged[k] = v
	}

	// Convert to slice
	var args []string
	for k, v := range merged {
		args = append(args, k, v)
	}
	return args
}

// mapArgsToSlice converts map to slice (already a slice, just return it)
func (lm *LiveMigrationTest) mapArgsToSlice(args []string) []string {
	return args
}

// snapshotPhaseCompleted checks if snapshot phase is complete
func (lm *LiveMigrationTest) snapshotPhaseCompleted(tableName string, expectedRows int64) bool {
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		return false
	}

	exportSnapshot := int64(0)
	importSnapshot := int64(0)

	for _, row := range report.RowData {
		if row.TableName == tableName {
			if row.DBType == "source" {
				exportSnapshot = row.ExportedSnapshotRows
			}
			if row.DBType == "target" {
				importSnapshot = row.ImportedSnapshotRows
			}
		}
	}

	return exportSnapshot == expectedRows && exportSnapshot == importSnapshot
}

// streamingPhaseCompleted checks if streaming phase is complete
func (lm *LiveMigrationTest) streamingPhaseCompleted(tableName string, expectedInserts int64) bool {
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		return false
	}

	exportInserts := int64(0)
	importInserts := int64(0)

	for _, row := range report.RowData {
		if row.TableName == tableName {
			if row.DBType == "source" {
				exportInserts = row.ExportedInserts
			}
			if row.DBType == "target" {
				importInserts = row.ImportedInserts
			}
		}
	}

	return exportInserts == expectedInserts && exportInserts == importInserts
}

// getCutoverStatus gets the current cutover status
func (lm *LiveMigrationTest) getCutoverStatus() string {
	if lm.metaDB == nil {
		return ""
	}

	msr, err := lm.metaDB.GetMigrationStatusRecord()
	if err != nil {
		return ""
	}

	a := msr.CutoverToTargetRequested
	b := msr.CutoverProcessedBySourceExporter
	c := msr.CutoverProcessedByTargetImporter

	if !a {
		return NOT_INITIATED
	}
	if msr.FallForwardEnabled && a && b && c && msr.ExportFromTargetFallForwardStarted {
		return COMPLETED
	} else if msr.FallbackEnabled && a && b && c && msr.ExportFromTargetFallBackStarted {
		return COMPLETED
	} else if !msr.FallForwardEnabled && !msr.FallbackEnabled && a && b && c {
		return COMPLETED
	}

	return INITIATED
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// DataMigrationReport represents the migration report structure
type DataMigrationReport struct {
	RowData []*rowData
}
