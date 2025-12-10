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
	"strings"
	"testing"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

/* ============================================================
Live Migration Testing Framework
It only supports for the normal live migration workflow and live migration workflows
*/

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

	SnapshotTimeout  time.Duration
	StreamingTimeout time.Duration
	StreamingSleep   time.Duration
	CutoverTimeout   time.Duration
}

// TestConfig holds all configuration upfront
type TestConfig struct {
	// Container configs
	SourceDB ContainerConfig
	TargetDB ContainerConfig

	// Schema setup
	SchemaNames          []string
	SchemaSQL            []string // CREATE statements
	SourceSetupSchemaSQL []string // ALTER REPLICA IDENTITIY ones statements
	InitialDataSQL       []string // INSERT statements

	SourceDeltaSQL []string // I/U/D statements
	TargetDeltaSQL []string // I/U/D statements

	CleanupSQL []string // DROP statements
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
	fmt.Printf("Setting up containers\n")
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

	fmt.Printf("Containers setup completed\n")
	return nil
}

// SetupSchema creates schema on source and target, registers cleanup
func (lm *LiveMigrationTest) SetupSchema() error {
	fmt.Printf("Setting up schema\n")
	// Execute schema SQL on source and target
	lm.sourceContainer.ExecuteSqls(lm.config.SchemaSQL...)
	lm.sourceContainer.ExecuteSqls(lm.config.SourceSetupSchemaSQL...)
	lm.targetContainer.ExecuteSqls(lm.config.SchemaSQL...)

	// Execute initial data SQL on source
	lm.sourceContainer.ExecuteSqls(lm.config.InitialDataSQL...)
	fmt.Printf("Schema setup completed\n")

	return nil
}

func (lm *LiveMigrationTest) InitMetaDB() error {
	if lm.metaDB != nil {
		return nil
	}
	var err error
	lm.metaDB, err = metadb.NewMetaDB(lm.exportDir)
	if err != nil {
		return fmt.Errorf("failed to initialize meta db: %w", err)
	}
	return nil
}
// Cleanup runs all cleanup operations (called via defer)
func (lm *LiveMigrationTest) Cleanup() {
	fmt.Printf("Cleaning up\n")
	// Execute cleanup SQL
	lm.sourceContainer.ExecuteSqls(lm.config.CleanupSQL...)
	lm.targetContainer.ExecuteSqls(lm.config.CleanupSQL...)

	// Remove export directory
	testutils.RemoveTempExportDir(lm.exportDir)
	fmt.Printf("Cleanup completed\n")
}

// ============================================================
// MIGRATION COMMANDS
// ============================================================

// StartExportData starts export data command
func (lm *LiveMigrationTest) StartExportData(async bool, extraArgs map[string]string) error {
	fmt.Printf("Starting export data\n")
	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for export to start
		}
	}

	args := []string{
		"--export-dir", lm.exportDir,
		"--source-db-schema", strings.Join(lm.config.SchemaNames, ","),
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.exportCmd = testutils.NewVoyagerCommandRunner(lm.sourceContainer, "export data", args, onStart, async)
	err := lm.exportCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start export data: %w", err)
	}
	fmt.Printf("Export data started\n")
	return nil
}

// StartImportData starts import data command
func (lm *LiveMigrationTest) StartImportData(async bool, extraArgs map[string]string) error {
	fmt.Printf("Starting import data\n")
	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for import to start
		}
	}

	args := []string{
		"--export-dir", lm.exportDir,
		"--disable-pb", "true",
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.importCmd = testutils.NewVoyagerCommandRunner(lm.targetContainer, "import data", args, onStart, async)
	err := lm.importCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start import data: %w", err)
	}
	fmt.Printf("Import data started\n")
	return nil
}

// StopExportData stops the running export data command
func (lm *LiveMigrationTest) StopExportData() error {
	fmt.Printf("Stopping export data\n")
	if lm.exportCmd == nil {
		return fmt.Errorf("export command not started")
	}
	if err := lm.exportCmd.Kill(); err != nil {
		return fmt.Errorf("killing the export data process errored: %w", err)
	}
	err := lm.exportCmd.Wait()
	if err != nil {
		return fmt.Errorf("failed to stop export data: %w", err)
	}
	fmt.Printf("Export data stopped\n")
	return nil
}

// StopImportData stops the running import data command
func (lm *LiveMigrationTest) StopImportData() error {
	fmt.Printf("Stopping import data\n")
	if lm.importCmd == nil {
		return fmt.Errorf("import command not started")
	}
	//Stopping import command
	if err := lm.importCmd.Kill(); err != nil {

	}
	err := lm.importCmd.Wait()
	if err != nil {
		lm.t.Logf("Async import run exited with error (expected): %v", err)
	} else {
		lm.t.Logf("Async import run completed unexpectedly")
	}

	fmt.Printf("Import data stopped\n")
	return nil
}

func (lm *LiveMigrationTest) ResumeImportData(async bool, extraArgs map[string]string) error {
	fmt.Printf("Resuming import data\n")
	if lm.importCmd == nil {
		return fmt.Errorf("import command not started")
	}
	for key, value := range extraArgs {
		lm.importCmd.AddArgs(key, value)
	}

	lm.importCmd.SetAsync(async)
	err := lm.importCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to resume import data: %w", err)
	}
	fmt.Printf("Import data resumed\n")
	return nil
}

func (lm *LiveMigrationTest) ResumeExportData(async bool) error {
	fmt.Printf("Resuming export data\n")
	if lm.exportCmd == nil {
		return fmt.Errorf("export command not started")
	}
	lm.exportCmd.SetAsync(async)
	err := lm.exportCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to resume export data: %w", err)
	}
	fmt.Printf("Export data resumed\n")
	return nil
}

// InitiateCutover initiates cutover to target
func (lm *LiveMigrationTest) InitiateCutover(prepareForFallback bool, extraArgs map[string]string) error {
	fmt.Printf("Initiating cutover\n")
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
	err := cutoverCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to initiate cutover: %w", err)
	}
	fmt.Printf("Cutover initiated\n")
	return nil
}

func (lm *LiveMigrationTest) ExecuteSourceDelta() error {
	lm.sourceContainer.ExecuteSqls(lm.config.SourceDeltaSQL...)
	return nil
}

func (lm *LiveMigrationTest) ExecuteTargetDelta() error {
	lm.targetContainer.ExecuteSqls(lm.config.TargetDeltaSQL...)
	return nil
}

// GetExportDir returns the export directory path
func (lm *LiveMigrationTest) GetExportDir() string {
	return lm.exportDir
}

func (lm *LiveMigrationTest) GetExportCommandStderr() string {
	if lm.exportCmd == nil {
		return ""
	}
	return lm.exportCmd.Stderr()
}

func (lm *LiveMigrationTest) GetExportCommandStdout() string {
	if lm.exportCmd == nil {
		return ""
	}
	return lm.exportCmd.Stdout()
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
// WAIT functions For Migration Phases to complete
// ============================================================

// WaitForSnapshotComplete waits until snapshot phase is done
func (lm *LiveMigrationTest) WaitForSnapshotComplete(tableName string, expectedRows int64) error {
	fmt.Printf("Waiting for snapshot complete\n")
	timeout := lm.SnapshotTimeout
	if timeout == 0 {
		timeout = 30
	}

	ok := utils.RetryWorkWithTimeout(1, timeout, func() bool {
		ok, err := lm.snapshotPhaseCompleted(tableName, expectedRows)
		if err != nil {
			testutils.FatalIfError(lm.t, err, "failed to get data migration report")
			return false
		}
		return ok
	})

	if !ok {
		return fmt.Errorf("snapshot phase did not complete within %v", timeout)
	}
	fmt.Printf("Snapshot complete\n")
	return nil
}

// WaitForStreamingComplete waits until streaming events are processed
func (lm *LiveMigrationTest) WaitForStreamingComplete(tableName string, expectedInserts int64, expectedUpdates int64, expectedDeletes int64) error {
	fmt.Printf("Waiting for streaming complete\n")
	timeout := lm.StreamingTimeout
	if timeout == 0 {
		timeout = 30
	}
	sleep := lm.StreamingSleep
	if sleep == 0 {
		sleep = 1
	}

	ok := utils.RetryWorkWithTimeout(sleep, timeout, func() bool {
		ok, err := lm.streamingPhaseCompleted(tableName, expectedInserts, expectedUpdates, expectedDeletes)
		if err != nil {
			testutils.FatalIfError(lm.t, err, "failed to get data migration report")
			return false
		}
		return ok
	})

	if !ok {
		return fmt.Errorf("streaming phase did not complete within %v", timeout)
	}
	fmt.Printf("Streaming complete\n")
	return nil
}

// WaitForCutoverComplete waits until cutover is done
func (lm *LiveMigrationTest) WaitForCutoverComplete() error {
	fmt.Printf("Waiting for cutover complete\n")
	timeout := lm.CutoverTimeout
	if timeout == 0 {
		timeout = 30
	}

	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return fmt.Errorf("failed to initialize meta db: %w", err)
		}
	}

	ok := utils.RetryWorkWithTimeout(1, timeout, func() bool {
		return lm.getCutoverStatus() == COMPLETED
	})

	if !ok {
		return fmt.Errorf("cutover did not complete within %v", timeout)
	}
	fmt.Printf("Cutover complete\n")
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
	fmt.Printf("Validating data consistency\n")
	lm.WithSourceTargetConn(func(source, target *sql.DB) error {
		for _, table := range tables {
			if err := testutils.CompareTableData(lm.ctx, source, target, table, orderBy); err != nil {
				return fmt.Errorf("table data mismatch for %s: %w", table, err)
			}
			fmt.Printf("Data consistency validated for %s\n", table)
		}
		return nil
	})

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
// INTERNAL HELPERS
// ============================================================

// snapshotPhaseCompleted checks if snapshot phase is complete
func (lm *LiveMigrationTest) snapshotPhaseCompleted(tableName string, expectedRows int64) (bool, error) {
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		return false, fmt.Errorf("failed to get data migration report: %w", err)
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
	return exportSnapshot == expectedRows && exportSnapshot == importSnapshot, nil
}

// streamingPhaseCompleted checks if streaming phase is complete
func (lm *LiveMigrationTest) streamingPhaseCompleted(tableName string, expectedInserts int64, expectedUpdates int64, expectedDeletes int64) (bool, error) {
	fmt.Printf("Waiting for streaming complete\n")
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		return false, fmt.Errorf("failed to get data migration report: %w", err)
	}

	exportInserts := int64(0)
	importInserts := int64(0)
	exportUpdates := int64(0)
	importUpdates := int64(0)
	exportDeletes := int64(0)
	importDeletes := int64(0)

	for _, row := range report.RowData {
		if row.TableName == tableName {
			if row.DBType == "source" {
				exportInserts = row.ExportedInserts
				exportUpdates = row.ExportedUpdates
				exportDeletes = row.ExportedDeletes
			}
			if row.DBType == "target" {
				importInserts = row.ImportedInserts
				importUpdates = row.ImportedUpdates
				importDeletes = row.ImportedDeletes
			}
		}
	}

	return exportInserts == expectedInserts && exportInserts == importInserts &&
		exportUpdates == expectedUpdates && exportUpdates == importUpdates &&
		exportDeletes == expectedDeletes && exportDeletes == importDeletes, nil
}

// getCutoverStatus gets the current cutover status
func (lm *LiveMigrationTest) getCutoverStatus() string {
	if lm.metaDB == nil {
		return ""
	}

	//set the global metaDB for running getCutoverStatus code

	metaDB = lm.metaDB

	return getCutoverStatus()
}

type DataMigrationReport struct {
	RowData []*rowData
}

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

	time.Sleep(10 * time.Second)

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
