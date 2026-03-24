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
	config              *TestConfig
	exportDir           string
	backupDir           string
	sourceContainer     testcontainers.TestContainer
	targetContainer     testcontainers.TestContainer
	exportCmd           *testutils.VoyagerCommandRunner
	importCmd           *testutils.VoyagerCommandRunner
	exportFromTargetCmd *testutils.VoyagerCommandRunner
	importToSourceCmd   *testutils.VoyagerCommandRunner
	metaDB              *metadb.MetaDB
	ctx                 context.Context
	t                   *testing.T
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
	// Kill any lingering Debezium processes before removing the export dir
	// (the PID is read from a lock file inside the export dir).
	lm.KillDebezium(SOURCE_DB_EXPORTER_ROLE)

	// Execute cleanup SQL
	lm.sourceContainer.ExecuteSqlsOnDB(lm.config.SourceDB.DatabaseName, lm.config.CleanupSQL...)
	lm.targetContainer.ExecuteSqlsOnDB(lm.config.TargetDB.DatabaseName, lm.config.CleanupSQL...)

	if lm.config.SourceDB.DatabaseName != "" {
		pg := lm.sourceContainer.(*testcontainers.PostgresContainer)
		err := pg.DropDatabase(lm.config.SourceDB.DatabaseName)
		if err != nil {
			lm.t.Fatalf("failed to drop source database: %v", err)
		}
	}
	if lm.config.TargetDB.DatabaseName != "" {
		yb := lm.targetContainer.(*testcontainers.YugabyteDBContainer)
		err := yb.DropDatabase(lm.config.TargetDB.DatabaseName)
		if err != nil {
			lm.t.Fatalf("failed to drop target database: %v", err)
		}
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
	return lm.startExportData(async, extraArgs, SNAPSHOT_AND_CHANGES, nil)
}

func (lm *LiveMigrationTest) startExportData(async bool, extraArgs map[string]string, exportType string, env []string) error {

	if len(env) > 0 {
		fmt.Printf("Starting export data with export type %s and env %v\n", exportType, env)
	} else {
		fmt.Printf("Starting export data with export type %s\n", exportType)
	}
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
		"--export-type", exportType,
		"--parallel-jobs", "1",
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.exportCmd = testutils.NewVoyagerCommandRunner(lm.sourceContainer, "export data", args, onStart, async).WithEnv(env...)
	err := lm.exportCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start export data: %w", err)
	}
	if len(env) > 0 {
		fmt.Printf("Export data with export type %s started with env %v\n", exportType, env)
	} else {
		fmt.Printf("Export data with export type %s started\n", exportType)
	}
	return nil
}

func (lm *LiveMigrationTest) StartExportDataChangesOnly(async bool, extraArgs map[string]string) error {
	return lm.startExportData(async, extraArgs, CHANGES_ONLY, nil)
}

// StartExportDataWithEnv starts export data with additional environment variables.
func (lm *LiveMigrationTest) StartExportDataWithEnv(async bool, extraArgs map[string]string, env []string) error {
	return lm.startExportData(async, extraArgs, SNAPSHOT_AND_CHANGES, env)
}

// StartImportData starts import data command
func (lm *LiveMigrationTest) StartImportData(async bool, extraArgs map[string]string) error {
	return lm.startImportData(async, extraArgs, nil)
}

// StartImportDataWithEnv starts import data with additional environment variables.
// This is useful for failpoint injection and tuning knobs like batch sizes.
func (lm *LiveMigrationTest) StartImportDataWithEnv(async bool, extraArgs map[string]string, env []string) error {
	return lm.startImportData(async, extraArgs, env)
}

func (lm *LiveMigrationTest) startImportData(async bool, extraArgs map[string]string, env []string) error {
	if len(env) > 0 {
		fmt.Printf("Starting import data with env %v\n", env)
	} else {
		fmt.Printf("Starting import data\n")
	}
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

	lm.importCmd = testutils.NewVoyagerCommandRunner(lm.targetContainer, "import data", args, onStart, async).WithEnv(env...)
	err := lm.importCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start import data: %w", err)
	}
	if len(env) > 0 {
		fmt.Printf("Import data started with env\n")
	} else {
		fmt.Printf("Import data started\n")
	}
	return nil
}

func (lm *LiveMigrationTest) StartExportDataFromTarget(async bool, extraArgs map[string]string) error {
	return lm.startExportDataFromTarget(async, extraArgs, nil)
}

func (lm *LiveMigrationTest) startExportDataFromTarget(async bool, extraArgs map[string]string, env []string) error {
	if len(env) > 0 {
		fmt.Printf("Starting export data from target with env %v\n", env)
	} else {
		fmt.Printf("Starting export data from target\n")
	}
	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for export to start
		}
	}

	targetConfig := lm.targetContainer.GetConfig()
	args := []string{
		"--export-dir", lm.exportDir,
		"--disable-pb", "true",
		"--target-db-password", targetConfig.Password,
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.exportFromTargetCmd = testutils.NewVoyagerCommandRunner(nil, "export data from target", args, onStart, async).WithEnv(env...)
	err := lm.exportFromTargetCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start export data: %w", err)
	}
	if len(env) > 0 {
		fmt.Printf("Export data from target started with env %v\n", env)
	} else {
		fmt.Printf("Export data from target started\n")
	}
	return nil
}

func (lm *LiveMigrationTest) StartExportDataFromTargetWithEnv(async bool, extraArgs map[string]string, env []string) error {
	return lm.startExportDataFromTarget(async, extraArgs, env)
}

func (lm *LiveMigrationTest) StartImportDataToSource(async bool, extraArgs map[string]string) error {
	return lm.startImportDataToSource(async, extraArgs, nil)
}

func (lm *LiveMigrationTest) startImportDataToSource(async bool, extraArgs map[string]string, env []string) error {
	if len(env) > 0 {
		fmt.Printf("Starting import data to source with env %v\n", env)
	} else {
		fmt.Printf("Starting import data to source\n")
	}
	var onStart func()
	if async {
		onStart = func() {
			time.Sleep(5 * time.Second) // Wait for import to start
		}
	}
	sourceConfig := lm.sourceContainer.GetConfig()
	args := []string{
		"--export-dir", lm.exportDir,
		"--disable-pb", "true",
		"--source-db-password", sourceConfig.Password,
		"--yes",
	}
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	lm.importToSourceCmd = testutils.NewVoyagerCommandRunner(nil, "import data to source", args, onStart, async).WithEnv(env...)
	err := lm.importToSourceCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to start import data: %w", err)
	}
	if len(env) > 0 {
		fmt.Printf("Import data to source started with env %v\n", env)
	} else {
		fmt.Printf("Import data to source started\n")
	}
	return nil
}

func (lm *LiveMigrationTest) StartImportDataToSourceWithEnv(async bool, extraArgs map[string]string, env []string) error {
	return lm.startImportDataToSource(async, extraArgs, env)
}

func (lm *LiveMigrationTest) WaitForExportFromTargetFailpointAndProcessCrash(t *testing.T, markerPath string, markerTimeout, exitTimeout time.Duration) error {
	return testutils.WaitForFailpointAndProcessCrash(t, lm.exportFromTargetCmd, markerPath, markerTimeout, exitTimeout)
}

func (lm *LiveMigrationTest) WaitForImportToSourceFailpointAndProcessCrash(t *testing.T, markerPath string, markerTimeout, exitTimeout time.Duration) error {
	return testutils.WaitForFailpointAndProcessCrash(t, lm.importToSourceCmd, markerPath, markerTimeout, exitTimeout)
}

func (lm *LiveMigrationTest) WaitForExportFailpointAndProcessCrash(t *testing.T, markerPath string, markerTimeout, exitTimeout time.Duration) error {
	return testutils.WaitForFailpointAndProcessCrash(t, lm.exportCmd, markerPath, markerTimeout, exitTimeout)
}

// StopExportData stops the running export data command
func (lm *LiveMigrationTest) StopExportData() error {
	fmt.Printf("Stopping export data\n")
	if lm.exportCmd == nil {
		return goerrors.Errorf("export command not started")
	}
	err := lm.exportCmd.GracefulStop(20)
	if err != nil {
		return goerrors.Errorf("failed to stop export data: %w", err)
	}
	fmt.Printf("Export data stopped\n")
	return nil
}

func (lm *LiveMigrationTest) StopExportDataFromTarget() error {
	fmt.Printf("Stopping export data from target\n")
	if lm.exportFromTargetCmd == nil {
		return goerrors.Errorf("export from target command not started")
	}
	err := lm.exportFromTargetCmd.GracefulStop(20)
	if err != nil {
		return goerrors.Errorf("failed to stop export data from target: %w", err)
	}
	fmt.Printf("Export data from target stopped\n")
	return nil
}

// KillDebezium force-kills the Debezium Java process for the given exporter role.
// Debezium runs as a separate child Java process and can outlive the `yb-voyager` parent if
// the parent is SIGKILLed. Cleanup() calls this automatically for SOURCE_DB_EXPORTER_ROLE.
func (lm *LiveMigrationTest) KillDebezium(exporterRole string) {
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(lm.exportDir, exporterRole)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		lm.t.Logf("WARNING: failed to read Debezium PID from exportDir: %v", err)
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		lm.t.Logf("WARNING: failed to parse Debezium PID %q: %v", pidStr, err)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		lm.t.Logf("WARNING: failed to find Debezium process pid=%d: %v", pid, err)
		return
	}
	if err := proc.Kill(); err != nil {
		lm.t.Logf("WARNING: failed to kill Debezium process pid=%d: %v", pid, err)
		return
	}
	lm.t.Logf("Killed Debezium process pid=%d", pid)
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

func (lm *LiveMigrationTest) StopImportDataToSource() error {
	fmt.Printf("Stopping import data to source\n")
	if lm.importToSourceCmd == nil {
		return goerrors.Errorf("import to source command not started")
	}
	if err := lm.importToSourceCmd.Kill(); err != nil {
		return goerrors.Errorf("killing the import data to source process errored: %w", err)
	}
	err := lm.importToSourceCmd.Wait()
	if err != nil {
		lm.t.Logf("Async import to source run exited with error (expected): %v", err)
	} else {
		lm.t.Logf("Async import to source run completed unexpectedly")
	}
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
func (lm *LiveMigrationTest) GetCurrentExportDir() string {
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			testutils.FatalIfError(lm.t, err, "failed to initialize meta db")
		}
	}
	msr, err := lm.metaDB.GetMigrationStatusRecord()
	if err != nil {
		testutils.FatalIfError(lm.t, err, "failed to get migration status record")
	}
	if msr == nil {
		testutils.FatalIfError(lm.t, goerrors.Errorf("migration status record not found"), "migration status record not found")
	}
	if msr.LatestIterationNumber > 0 {
		return GetIterationExportDir(msr.GetIterationsDir(lm.exportDir), msr.LatestIterationNumber)
	}
	return lm.exportDir
}

// ReadMigrationUUID reads the migration UUID from the export directory's metaDB.
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

func (lm *LiveMigrationTest) GetImportRunner() *testutils.VoyagerCommandRunner {
	return lm.importCmd
}

func (lm *LiveMigrationTest) WaitForImportFailpointAndProcessCrash(t *testing.T, markerPath string, markerTimeout, exitTimeout time.Duration) error {
	return testutils.WaitForFailpointAndProcessCrash(t, lm.importCmd, markerPath, markerTimeout, exitTimeout)
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

// GetImportToSourceCommandStderr gets stderr from import to source command
func (lm *LiveMigrationTest) GetImportToSourceCommandStderr() string {
	if lm.importToSourceCmd == nil {
		return ""
	}
	return lm.importToSourceCmd.Stderr()
}

// GetImportToSourceCommandStdout gets stdout from import to source command
func (lm *LiveMigrationTest) GetImportToSourceCommandStdout() string {
	if lm.importToSourceCmd == nil {
		return ""
	}
	return lm.importToSourceCmd.Stdout()
}

// GetExportFromTargetCommandStderr gets stderr from export from target command

// GetExportFromTargetCommandStdout gets stdout from export from target command
func (lm *LiveMigrationTest) GetExportFromTargetCommandStdout() string {
	if lm.exportFromTargetCmd == nil {
		return ""
	}
	return lm.exportFromTargetCmd.Stdout()
}

// GetExportCommandFromTargetStderr gets stderr from export command from target
func (lm *LiveMigrationTest) GetExportCommandFromTargetStderr() string {
	if lm.exportFromTargetCmd == nil {
		return ""
	}
	return lm.exportFromTargetCmd.Stderr()
}

func (lm *LiveMigrationTest) EndMigration(extraArgs map[string]string, withBackup bool) error {
	fmt.Printf("Ending migration\n")
	if withBackup {
		lm.backupDir = testutils.CreateBackupDir(lm.t)
		defer testutils.RemoveTempExportDir(lm.backupDir)
	}

	args := []string{
		"--export-dir", lm.exportDir,
		"--yes",
	}
	if withBackup {
		args = append(args, "--backup-dir", lm.backupDir)
		args = append(args, "--backup-schema-files", "true")
		args = append(args, "--backup-data-files", "true")
		args = append(args, "--backup-log-files", "true")
		args = append(args, "--save-migration-reports", "true")
	} else {
		args = append(args, "--backup-schema-files", "false")
		args = append(args, "--backup-data-files", "false")
		args = append(args, "--backup-log-files", "false")
		args = append(args, "--save-migration-reports", "false")
	}

	// Add extra args
	for key, value := range extraArgs {
		args = append(args, key, value)
	}

	cutoverCmd := testutils.NewVoyagerCommandRunner(nil, "end migration", args, nil, false).WithEnv(
		fmt.Sprintf("SOURCE_DB_PASSWORD=%s", lm.sourceContainer.GetConfig().Password),
		fmt.Sprintf("TARGET_DB_PASSWORD=%s", lm.targetContainer.GetConfig().Password),
	)
	err := cutoverCmd.Run()
	if err != nil {
		return goerrors.Errorf("failed to initiate cutover: %w", err)
	}
	fmt.Printf("Migration ended\n")
	return nil
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
func (lm *LiveMigrationTest) WaitForCutoverComplete(iterationNumber int, cutoverTimeout time.Duration) error {
	fmt.Printf("Waiting for cutover complete\n")

	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return goerrors.Errorf("failed to initialize meta db: %w", err)
		}
	}

	ok := utils.RetryWorkWithTimeout(1, cutoverTimeout, func() bool {
		return lm.getCutoverStatus(iterationNumber) == COMPLETED
	})

	if !ok {
		return goerrors.Errorf("cutover did not complete within %v", cutoverTimeout)
	}
	fmt.Printf("Cutover complete\n")
	//update the export and import commands to the new export and import commands
	lm.exportFromTargetCmd = lm.importCmd
	lm.importToSourceCmd = lm.exportCmd
	return nil
}

// WaitForCutoverSourceComplete waits until cutover to source is done
func (lm *LiveMigrationTest) WaitForCutoverSourceComplete(iterationNumber int, cutoverTimeout time.Duration) error {
	fmt.Printf("Waiting for cutover to source complete\n")

	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return goerrors.Errorf("failed to initialize meta db: %w", err)
		}
	}

	ok := utils.RetryWorkWithTimeout(1, cutoverTimeout, func() bool {
		return lm.getCutoverToSourceStatus(iterationNumber) == COMPLETED
	})

	if !ok {
		return goerrors.Errorf("cutover to source did not complete within %v", cutoverTimeout)
	}
	fmt.Printf("Cutover to source complete\n")
	lm.exportCmd = lm.importToSourceCmd
	lm.importCmd = lm.exportFromTargetCmd
	return nil
}

func (lm *LiveMigrationTest) WaitForNextIterationInitialized(waitTimeout time.Duration, iterationNo int) error {
	fmt.Printf("Waiting for next iteration initialized\n")
	// Initialize metaDB if not already done
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return goerrors.Errorf("failed to initialize meta db: %w", err)
		}
	}
	msr, err := lm.metaDB.GetMigrationStatusRecord()
	if err != nil {
		return goerrors.Errorf("failed to get migration status record: %w", err)
	}
	if msr.LatestIterationNumber == 0 {
		return nil
	}

	fmt.Printf("Waiting for next iteration initialized: iterationNo = %d\n", iterationNo)
	var iterationMetaDB *metadb.MetaDB
	if iterationNo > 0 {
		iterationsExportDir := msr.GetIterationsDir(lm.exportDir)
		iterationExportDir := GetIterationExportDir(iterationsExportDir, iterationNo)
		if !utils.FileOrFolderExists(iterationExportDir) {
			return goerrors.Errorf("iteration export directory does not exist")
		}
		fmt.Printf("Iteration export directory exists: %s\n", iterationExportDir)
		iterationMetaDB, err = metadb.NewMetaDB(iterationExportDir)
		if err != nil {
			return goerrors.Errorf("failed to create iteration meta db: %w", err)
		}
	} else {
		iterationMetaDB = lm.metaDB
	}

	ok := utils.RetryWorkWithTimeout(1, waitTimeout, func() bool {
		msr, err := iterationMetaDB.GetMigrationStatusRecord()
		if err != nil {
			return false
		}
		return msr.NextIterationInitialized
	})
	if !ok {
		return goerrors.Errorf("next iteration did not initialize within %v", waitTimeout)
	}
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
func (lm *LiveMigrationTest) getCutoverStatus(iterationNumber int) string {
	if lm.metaDB == nil {
		return ""
	}
	if iterationNumber == 0 {
		return getCutoverStatus(lm.metaDB)
	}

	iterationCutoverMap := collectCutoverStatusRowsForAllIterations(lm.exportDir, lm.metaDB)
	rows := iterationCutoverMap[iterationNumber]
	if len(rows) < 1 {
		return NOT_INITIATED
	}
	return rows[0].Status
}

// getCutoverToSourceStatus gets the current cutover to source status
func (lm *LiveMigrationTest) getCutoverToSourceStatus(iterationNumber int) string {
	if lm.metaDB == nil {
		return ""
	}
	if iterationNumber == 0 {
		return getCutoverToSourceStatus(lm.exportDir, lm.metaDB)
	}

	iterationCutoverMap := collectCutoverStatusRowsForAllIterations(lm.exportDir, lm.metaDB)
	rows := iterationCutoverMap[iterationNumber]
	if len(rows) < 2 {
		return NOT_INITIATED
	}
	return rows[1].Status
}

type DataMigrationReport struct {
	RowData []*rowData
}

// GetDataMigrationReport retrieves the migration report
func (lm *LiveMigrationTest) GetDataMigrationReport() (*DataMigrationReport, error) {
	if lm.metaDB == nil {
		err := lm.InitMetaDB()
		if err != nil {
			return nil, goerrors.Errorf("failed to initialize meta db: %w", err)
		}
	}

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
