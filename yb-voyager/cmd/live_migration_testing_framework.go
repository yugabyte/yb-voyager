//go:build integration_live_migration || failpoint

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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	goerrors "github.com/go-errors/errors"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
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

	envVars        []string // env vars applied to the next command start
	exportCallback func()  // custom doDuringCmd for export; overrides default sleep
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
	Type         string // "postgresql", "yugabytedb", etc.
	ForLive      bool   // Whether to configure for live migration
	DatabaseName string // Database name to use for the container
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
		return goerrors.Errorf("failed to start source container: %w", err)
	}

	// Start target container
	targetContainerConfig := &testcontainers.ContainerConfig{
		ForLive: lm.config.TargetDB.ForLive,
	}
	lm.targetContainer = testcontainers.NewTestContainer(lm.config.TargetDB.Type, targetContainerConfig)
	if err := lm.targetContainer.Start(ctx); err != nil {
		return goerrors.Errorf("failed to start target container: %w", err)
	}

	if lm.config.SourceDB.DatabaseName != "" {
		pg := lm.sourceContainer.(*testcontainers.PostgresContainer)
		err := pg.CreateDatabase(lm.config.SourceDB.DatabaseName)
		if err != nil {
			return goerrors.Errorf("failed to create source database: %v", err)
		}
	}
	if lm.config.TargetDB.DatabaseName != "" {

		yb := lm.targetContainer.(*testcontainers.YugabyteDBContainer)
		err := yb.CreateDatabase(lm.config.TargetDB.DatabaseName)
		if err != nil {
			return goerrors.Errorf("failed to create target database: %v", err)
		}
	}
	fmt.Printf("Containers setup completed\n")
	return nil
}

// SetupSchema creates schema on source and target, registers cleanup
func (lm *LiveMigrationTest) SetupSchema() error {
	fmt.Printf("Setting up schema\n")
	// Execute schema SQL on source and target
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, lm.config.SchemaSQL...)
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, lm.config.SourceSetupSchemaSQL...)
	lm.targetContainer.ExecuteSqlsOnDB(lm.config.TargetDB.DatabaseName, lm.config.SchemaSQL...)

	// Execute initial data SQL on source
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, lm.config.InitialDataSQL...)
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
		return goerrors.Errorf("failed to initialize meta db: %w", err)
	}
	return nil
}

// Cleanup runs all cleanup operations (called via defer)
func (lm *LiveMigrationTest) Cleanup() {
	fmt.Printf("Cleaning up\n")

	// Kill any running commands
	if lm.exportCmd != nil {
		_ = lm.exportCmd.Kill()
	}
	if lm.importCmd != nil {
		_ = lm.importCmd.Kill()
	}
	lm.killDebezium()

	// Execute cleanup SQL
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, lm.config.CleanupSQL...)
	lm.targetContainer.ExecuteSqlsOnDB(lm.config.TargetDB.DatabaseName, lm.config.CleanupSQL...)

	if lm.config.SourceDB.DatabaseName != "" {
		pg := lm.sourceContainer.(*testcontainers.PostgresContainer)
		err := pg.DropDatabase(lm.config.SourceDB.DatabaseName)
		if err != nil {
			lm.t.Logf("WARNING: failed to drop source database: %v", err)
		}
	}
	if lm.config.TargetDB.DatabaseName != "" {
		yb := lm.targetContainer.(*testcontainers.YugabyteDBContainer)
		err := yb.DropDatabase(lm.config.TargetDB.DatabaseName)
		if err != nil {
			lm.t.Logf("WARNING: failed to drop target database: %v", err)
		}
	}

	// Stop containers
	if lm.sourceContainer != nil {
		lm.sourceContainer.Stop(lm.ctx)
	}
	if lm.targetContainer != nil {
		lm.targetContainer.Stop(lm.ctx)
	}

	// Remove export directory only if test passed
	if lm.t.Failed() {
		fmt.Printf("Test failed - preserving export directory for debugging: %s\n", lm.exportDir)
	} else {
		testutils.RemoveTempExportDir(lm.exportDir)
	}
	fmt.Printf("Cleanup completed\n")
}

// ============================================================
// MIGRATION COMMANDS
// ============================================================

// StartExportData starts export data command
func (lm *LiveMigrationTest) StartExportData(async bool, extraArgs map[string]string) error {
	fmt.Printf("Starting export data\n")
	var onStart func()
	if lm.exportCallback != nil {
		onStart = lm.exportCallback
		lm.exportCallback = nil
	} else if async {
		onStart = func() {
			time.Sleep(5 * time.Second)
		}
	}

	args := []string{
		"--export-dir", lm.exportDir,
		"--source-db-schema", strings.Join(lm.config.SchemaNames, ","),
		"--source-db-name", lm.config.SourceDB.DatabaseName,
		"--disable-pb", "true",
		"--export-type", SNAPSHOT_AND_CHANGES,
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.exportCmd = testutils.NewVoyagerCommandRunner(lm.sourceContainer, "export data", args, onStart, async)
	if len(lm.envVars) > 0 {
		lm.exportCmd.WithEnv(lm.envVars...)
	}
	err := lm.exportCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start export data: %w", err)
	}
	fmt.Printf("Export data started\n")
	return nil
}

func (lm *LiveMigrationTest) StartExportDataChangesOnly(async bool, extraArgs map[string]string) error {
	fmt.Printf("Starting export data changes only\n")
	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for export to start
		}
	}
	args := []string{
		"--export-dir", lm.exportDir,
		"--source-db-schema", strings.Join(lm.config.SchemaNames, ","),
		"--source-db-name", lm.config.SourceDB.DatabaseName,
		"--disable-pb", "true",
		"--export-type", CHANGES_ONLY,
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.exportCmd = testutils.NewVoyagerCommandRunner(lm.sourceContainer, "export data", args, onStart, async)
	err := lm.exportCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start export data: %w", err)
	}
	fmt.Printf("Export data changes only started\n")
	return nil
}

// StartImportData starts import data command
func (lm *LiveMigrationTest) StartImportData(async bool, extraArgs map[string]string) error {
	fmt.Printf("Starting import data\n")
	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second)
		}
	}

	args := []string{
		"--export-dir", lm.exportDir,
		"--disable-pb", "true",
		"--target-db-name", lm.config.TargetDB.DatabaseName,
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.importCmd = testutils.NewVoyagerCommandRunner(lm.targetContainer, "import data", args, onStart, async)
	if len(lm.envVars) > 0 {
		lm.importCmd.WithEnv(lm.envVars...)
	}
	err := lm.importCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start import data: %w", err)
	}
	fmt.Printf("Import data started\n")
	return nil
}

// StopExportData stops the running export data command
func (lm *LiveMigrationTest) StopExportData() error {
	fmt.Printf("Stopping export data\n")
	if lm.exportCmd == nil {
		return goerrors.Errorf("export command not started")
	}
	if err := lm.exportCmd.Kill(); err != nil {
		return goerrors.Errorf("killing the export data process errored: %w", err)
	}
	err := lm.exportCmd.Wait()
	if err != nil {
		lm.t.Logf("Async export run exited with error (expected): %v", err)
	} else {
		lm.t.Logf("Async export run completed unexpectedly")
	}
	fmt.Printf("Export data stopped\n")
	return nil
}

// StopImportData stops the running import data command
func (lm *LiveMigrationTest) StopImportData() error {
	fmt.Printf("Stopping import data\n")
	if lm.importCmd == nil {
		return goerrors.Errorf("import command not started")
	}
	//Stopping import command
	if err := lm.importCmd.Kill(); err != nil {
		return goerrors.Errorf("killing the import data process errored: %w", err)
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
		return goerrors.Errorf("import command not started")
	}
	for key, value := range extraArgs {
		lm.importCmd.AddArgs(key, value)
	}

	lm.importCmd.SetAsync(async)
	err := lm.importCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to resume import data: %w", err)
	}
	fmt.Printf("Import data resumed\n")
	return nil
}

func (lm *LiveMigrationTest) ResumeExportData(async bool) error {
	fmt.Printf("Resuming export data\n")
	if lm.exportCmd == nil {
		return goerrors.Errorf("export command not started")
	}
	lm.exportCmd.SetAsync(async)
	err := lm.exportCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to resume export data: %w", err)
	}
	fmt.Printf("Export data resumed\n")
	return nil
}

// InitiateCutover initiates cutover to target
func (lm *LiveMigrationTest) InitiateCutoverToTarget(prepareForFallback bool, extraArgs map[string]string) error {
	fmt.Printf("Initiating cutover to target\n")
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
		return goerrors.Errorf("failed to initiate cutover: %w", err)
	}
	fmt.Printf("Cutover initiated to target\n")
	return nil
}

// InitiateCutover initiates cutover to target
func (lm *LiveMigrationTest) InitiateCutoverToSource(extraArgs map[string]string) error {
	fmt.Printf("Initiating cutover to source\n")
	args := []string{
		"--export-dir", lm.exportDir,
		"--yes",
	}

	// Add extra args
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	cutoverCmd := testutils.NewVoyagerCommandRunner(nil, "initiate cutover to source", args, nil, false)
	err := cutoverCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to initiate cutover: %w", err)
	}
	fmt.Printf("Cutover initiated to source\n")
	return nil
}

func (lm *LiveMigrationTest) ExecuteSourceDelta() error {
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, lm.config.SourceDeltaSQL...)
	return nil
}

func (lm *LiveMigrationTest) ExecuteTargetDelta() error {
	lm.targetContainer.ExecuteSqlsOnDB(lm.config.TargetDB.DatabaseName, lm.config.TargetDeltaSQL...)
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
func (lm *LiveMigrationTest) WaitForSnapshotComplete(expectedData map[string]int64, snapshotTimeout time.Duration) error {
	fmt.Printf("Waiting for snapshot complete\n")

	ok := utils.RetryWorkWithTimeout(1, snapshotTimeout, func() bool {
		ok, err := lm.snapshotPhaseCompleted(expectedData)
		if err != nil {
			testutils.FatalIfError(lm.t, err, "failed to get data migration report")
			return false
		}
		return ok
	})

	if !ok {
		return goerrors.Errorf("snapshot phase did not complete within %v", snapshotTimeout)
	}
	fmt.Printf("Snapshot complete\n")
	return nil
}

// WaitForForwardStreamingComplete waits until streaming events are processed
func (lm *LiveMigrationTest) WaitForForwardStreamingComplete(expectedChanges map[string]ChangesCount, streamingTimeout time.Duration, streamingSleep time.Duration) error {
	fmt.Printf("Waiting for streaming complete\n")

	ok := utils.RetryWorkWithTimeout(streamingSleep, streamingTimeout, func() bool {
		ok, err := lm.streamingPhaseCompleted(expectedChanges, "source", "target")
		if err != nil {
			testutils.FatalIfError(lm.t, err, "failed to get data migration report")
			return false
		}
		return ok
	})

	if !ok {
		return goerrors.Errorf("streaming phase did not complete within %v", streamingTimeout)
	}
	fmt.Printf("Streaming complete\n")
	return nil
}

// WaitForForwardStreamingComplete waits until streaming events are processed
func (lm *LiveMigrationTest) WaitForFallbackStreamingComplete(expectedChanges map[string]ChangesCount, streamingTimeout time.Duration, streamingSleep time.Duration) error {
	fmt.Printf("Waiting for streaming complete\n")

	ok := utils.RetryWorkWithTimeout(streamingSleep, streamingTimeout, func() bool {
		ok, err := lm.streamingPhaseCompleted(expectedChanges, "target", "source")
		if err != nil {
			testutils.FatalIfError(lm.t, err, "failed to get data migration report")
			return false
		}
		return ok
	})

	if !ok {
		return goerrors.Errorf("streaming phase did not complete within %v", streamingTimeout)
	}
	fmt.Printf("Streaming complete\n")
	return nil
}

// WaitForCutoverComplete waits until cutover is done
func (lm *LiveMigrationTest) WaitForCutoverComplete(cutoverTimeout time.Duration) error {
	fmt.Printf("Waiting for cutover complete\n")

	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return goerrors.Errorf("failed to initialize meta db: %w", err)
		}
	}

	ok := utils.RetryWorkWithTimeout(1, cutoverTimeout, func() bool {
		return lm.getCutoverStatus() == COMPLETED
	})

	if !ok {
		return goerrors.Errorf("cutover did not complete within %v", cutoverTimeout)
	}
	fmt.Printf("Cutover complete\n")
	return nil
}

// WaitForCutoverSourceComplete waits until cutover to source is done
func (lm *LiveMigrationTest) WaitForCutoverSourceComplete(cutoverTimeout time.Duration) error {
	fmt.Printf("Waiting for cutover to source complete\n")

	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return goerrors.Errorf("failed to initialize meta db: %w", err)
		}
	}

	ok := utils.RetryWorkWithTimeout(1, cutoverTimeout, func() bool {
		return lm.getCutoverToSourceStatus() == COMPLETED
	})

	if !ok {
		return goerrors.Errorf("cutover to source did not complete within %v", cutoverTimeout)
	}
	fmt.Printf("Cutover to source complete\n")
	return nil
}

// ============================================================
// DATA OPERATIONS & VALIDATION
// ============================================================

// ExecuteOnSource executes SQL statements on source database (test-specific DB)
func (lm *LiveMigrationTest) ExecuteOnSource(sqlStatements ...string) error {
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, sqlStatements...)
	return nil
}

// ExecuteOnTarget executes SQL statements on target database (test-specific DB)
func (lm *LiveMigrationTest) ExecuteOnTarget(sqlStatements ...string) error {
	lm.targetContainer.ExecuteSqlsOnDB(lm.config.TargetDB.DatabaseName, sqlStatements...)
	return nil
}

// ValidateDataConsistency compares data between source and target
func (lm *LiveMigrationTest) ValidateDataConsistency(tables []string, orderBy string) error {
	fmt.Printf("Validating data consistency\n")
	return lm.WithSourceTargetConn(func(source, target *sql.DB) error {
		for _, table := range tables {
			if err := testutils.CompareTableData(lm.ctx, source, target, table, orderBy); err != nil {
				return goerrors.Errorf("table data mismatch for %s: %w", table, err)
			}
			fmt.Printf("Data consistency validated for %s\n", table)
		}
		return nil
	})
}

func (lm *LiveMigrationTest) ValidateRowCount(tables []string) error {
	fmt.Printf("Validating row count\n")
	return lm.WithSourceTargetConn(func(source, target *sql.DB) error {
		for _, table := range tables {
			if err := testutils.CompareRowCount(lm.ctx, source, target, table); err != nil {
				return goerrors.Errorf("row count mismatch for %s: %w", table, err)
			}
			fmt.Printf("Row count validated for %s\n", table)
		}
		return nil
	})
}

// WithSourceConn provides source database connection to callback (test-specific DB)
func (lm *LiveMigrationTest) WithSourceConn(fn func(*sql.DB) error) error {
	conn, err := lm.sourceContainer.GetConnectionWithDB(lm.config.SourceDB.DatabaseName)
	if err != nil {
		return goerrors.Errorf("failed to get source connection: %w", err)
	}
	defer conn.Close()
	return fn(conn)
}

// WithTargetConn provides target database connection to callback (test-specific DB)
func (lm *LiveMigrationTest) WithTargetConn(fn func(*sql.DB) error) error {
	conn, err := lm.targetContainer.GetConnectionWithDB(lm.config.TargetDB.DatabaseName)
	if err != nil {
		return goerrors.Errorf("failed to get target connection: %w", err)
	}
	defer conn.Close()
	return fn(conn)
}

// WithSourceTargetConn provides both connections to callback (test-specific DBs)
func (lm *LiveMigrationTest) WithSourceTargetConn(fn func(source, target *sql.DB) error) error {
	sourceConn, err := lm.sourceContainer.GetConnectionWithDB(lm.config.SourceDB.DatabaseName)
	if err != nil {
		return goerrors.Errorf("failed to get source connection: %w", err)
	}
	defer sourceConn.Close()

	targetConn, err := lm.targetContainer.GetConnectionWithDB(lm.config.TargetDB.DatabaseName)
	if err != nil {
		return goerrors.Errorf("failed to get target connection: %w", err)
	}
	defer targetConn.Close()

	return fn(sourceConn, targetConn)
}

// CheckIfReplicationSlotExists checks if a replication slot exists on the given database type
func (lm *LiveMigrationTest) CheckIfReplicationSlotExists(slotName string, dbType string) (bool, error) {
	var exists bool

	runWithConn := lm.WithTargetConn
	if dbType == "source" {
		runWithConn = lm.WithSourceConn
	}

	err := runWithConn(func(db *sql.DB) error {
		query := `SELECT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = $1);`
		if err := db.QueryRow(query, slotName).Scan(&exists); err != nil {
			return goerrors.Errorf("failed to check if replication slot exists on %s: %w", dbType, err)
		}
		return nil
	})

	return exists, err
}

// ============================================================
// INTERNAL HELPERS
// ============================================================

// snapshotPhaseCompleted checks if snapshot phase is complete
func (lm *LiveMigrationTest) snapshotPhaseCompleted(expectedData map[string]int64) (bool, error) {
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		return false, goerrors.Errorf("failed to get data migration report: %w", err)
	}
	allMatches := true
	for tableName, expectedRows := range expectedData {
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
		changesMatchForTable := exportSnapshot == expectedRows && exportSnapshot == importSnapshot
		if !changesMatchForTable {
			allMatches = false
			break
		}
	}
	return allMatches, nil
}

type ChangesCount struct {
	Inserts int64
	Updates int64
	Deletes int64
}

// streamingPhaseCompleted checks if streaming phase is complete
func (lm *LiveMigrationTest) streamingPhaseCompleted(changesCount map[string]ChangesCount, exportFrom string, importTo string) (bool, error) {
	fmt.Printf("Waiting for streaming complete\n")
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		return false, goerrors.Errorf("failed to get data migration report: %w", err)
	}

	allMatches := true
	for tableName, changesCount := range changesCount {
		exportInserts := int64(0)
		importInserts := int64(0)
		exportUpdates := int64(0)
		importUpdates := int64(0)
		exportDeletes := int64(0)
		importDeletes := int64(0)

		for _, row := range report.RowData {
			if row.TableName == tableName {
				if row.DBType == exportFrom {
					exportInserts = row.ExportedInserts
					exportUpdates = row.ExportedUpdates
					exportDeletes = row.ExportedDeletes
				}
				if row.DBType == importTo {
					importInserts = row.ImportedInserts
					importUpdates = row.ImportedUpdates
					importDeletes = row.ImportedDeletes
				}
			}
		}
		expectedInserts := changesCount.Inserts
		expectedUpdates := changesCount.Updates
		expectedDeletes := changesCount.Deletes
		changesMatchForTable := exportInserts == expectedInserts && exportInserts == importInserts &&
			exportUpdates == expectedUpdates && exportUpdates == importUpdates &&
			exportDeletes == expectedDeletes && exportDeletes == importDeletes

		if !changesMatchForTable {
			allMatches = false
			break
		}
	}

	return allMatches, nil
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

// getCutoverToSourceStatus gets the current cutover to source status
func (lm *LiveMigrationTest) getCutoverToSourceStatus() string {
	if lm.metaDB == nil {
		return ""
	}

	//set the global metaDB for running getCutoverToSourceStatus code

	metaDB = lm.metaDB

	return getCutoverToSourceStatus()
}

// ============================================================
// ENV VARS & CALLBACKS (for failpoint testing)
// ============================================================

// WithEnv sets environment variables that will be applied to the next
// StartExportData or StartImportData call. Call ClearEnv to reset.
func (lm *LiveMigrationTest) WithEnv(envVars ...string) *LiveMigrationTest {
	lm.envVars = envVars
	return lm
}

// ClearEnv removes all pending environment variables.
func (lm *LiveMigrationTest) ClearEnv() {
	lm.envVars = nil
}

// SetExportCallback sets a custom doDuringCmd callback for the next
// StartExportData call. It overrides the default 5-second sleep and is
// consumed (cleared) after one use.
func (lm *LiveMigrationTest) SetExportCallback(fn func()) {
	lm.exportCallback = fn
}

// ============================================================
// ACCESSORS (for custom operations in tests)
// ============================================================

// GetSourceContainer returns the source database container.
func (lm *LiveMigrationTest) GetSourceContainer() testcontainers.TestContainer {
	return lm.sourceContainer
}

// GetTargetContainer returns the target database container.
func (lm *LiveMigrationTest) GetTargetContainer() testcontainers.TestContainer {
	return lm.targetContainer
}

// GetSourceConnection returns a *sql.DB connected to the test-specific source database.
func (lm *LiveMigrationTest) GetSourceConnection() (*sql.DB, error) {
	return lm.sourceContainer.GetConnectionWithDB(lm.config.SourceDB.DatabaseName)
}

// GetTargetConnection returns a *sql.DB connected to the test-specific target database.
func (lm *LiveMigrationTest) GetTargetConnection() (*sql.DB, error) {
	return lm.targetContainer.GetConnectionWithDB(lm.config.TargetDB.DatabaseName)
}

// GetExportCmd returns the current export VoyagerCommandRunner (may be nil).
func (lm *LiveMigrationTest) GetExportCmd() *testutils.VoyagerCommandRunner {
	return lm.exportCmd
}

// GetImportCmd returns the current import VoyagerCommandRunner (may be nil).
func (lm *LiveMigrationTest) GetImportCmd() *testutils.VoyagerCommandRunner {
	return lm.importCmd
}

// ============================================================
// FAILPOINT TEST HELPERS
// ============================================================

// WaitForStreamingMode polls until the export status shows streaming mode.
func (lm *LiveMigrationTest) WaitForStreamingMode(timeout time.Duration, pollInterval time.Duration) error {
	statusPath := filepath.Join(lm.exportDir, "data", "export_status.json")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := dbzm.ReadExportStatus(statusPath)
		if err == nil && status != nil && status.Mode == dbzm.MODE_STREAMING {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return goerrors.Errorf("timed out waiting for export streaming mode after %v", timeout)
}

// StopExportAndFreezeQueue kills the export process, kills orphan Debezium,
// and removes the export lockfile to allow subsequent import or resume runs.
func (lm *LiveMigrationTest) StopExportAndFreezeQueue() error {
	if lm.exportCmd != nil {
		_ = lm.exportCmd.Kill()
	}
	lm.killDebezium()
	_ = os.Remove(filepath.Join(lm.exportDir, ".export-dataLockfile.lck"))
	time.Sleep(2 * time.Second)
	return nil
}

// RemoveImportLockfile removes the import data lockfile.
func (lm *LiveMigrationTest) RemoveImportLockfile() {
	_ = os.Remove(filepath.Join(lm.exportDir, ".import-dataLockfile.lck"))
}

// ReadMigrationUUID reads the migration UUID from the export directory's metadb.
func (lm *LiveMigrationTest) ReadMigrationUUID() (string, error) {
	if err := lm.InitMetaDB(); err != nil {
		return "", err
	}
	msr, err := lm.metaDB.GetMigrationStatusRecord()
	if err != nil {
		return "", err
	}
	if msr == nil {
		return "", goerrors.Errorf("migration status record not found")
	}
	return msr.MigrationUUID, nil
}

// killDebezium force-kills any Debezium Java process associated with this export directory.
func (lm *LiveMigrationTest) killDebezium() {
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(lm.exportDir, SOURCE_DB_EXPORTER_ROLE)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
	lm.t.Logf("Killed Debezium process pid=%d", pid)
}

// ============================================================
// REPORTING
// ============================================================

type DataMigrationReport struct {
	RowData []*rowData
}

// GetDataMigrationReport retrieves the migration report
func (lm *LiveMigrationTest) GetDataMigrationReport() (*DataMigrationReport, error) {
	maxRetry := 5
	for {
		err := testutils.NewVoyagerCommandRunner(nil, "get data-migration-report", []string{
			"--export-dir", lm.exportDir,
			"--output-format", "json",
			"--source-db-password", lm.sourceContainer.GetConfig().Password,
			"--target-db-password", lm.targetContainer.GetConfig().Password,
		}, nil, true).Run()
		if err != nil {
			return nil, goerrors.Errorf("get data-migration-report command failed: %w", err)
		}

		reportFilePath := filepath.Join(lm.exportDir, "reports", "data-migration-report.json")
		if !utils.FileOrFolderExists(reportFilePath) {
			maxRetry--
			if maxRetry <= 0 {
				return nil, goerrors.Errorf("report file does not exist")
			}
			time.Sleep(2 * time.Second)
			continue
		}

		jsonFile := jsonfile.NewJsonFile[[]*rowData](reportFilePath)
		rowData, err := jsonFile.Read()
		if err != nil {
			return nil, goerrors.Errorf("error reading data-migration-report: %w", err)
		}

		return &DataMigrationReport{RowData: *rowData}, nil
	}
}
