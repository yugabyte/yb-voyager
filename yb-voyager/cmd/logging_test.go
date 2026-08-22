//go:build unit

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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
)

// resetLogOutput restores logrus's output to stderr so later tests (and other
// packages relying on default logging behavior) aren't affected by a redirected
// output left over from InitLogging.
func resetLogOutput(t *testing.T) {
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
	})
}

func TestInitLogging_DefaultLogDir(t *testing.T) {
	resetLogOutput(t)
	exportDir := t.TempDir()

	err := InitLogging(exportDir, "info", false, "test-cmd", logFileSettings{
		Dir:        "",
		MaxSizeMB:  config.DefaultLogMaxSizeMB,
		MaxBackups: config.DefaultLogMaxBackups,
	})
	require.NoError(t, err)

	expectedLogFile := filepath.Join(exportDir, "logs", "yb-voyager-test-cmd.log")
	assert.FileExists(t, expectedLogFile, "log file should be created at <export-dir>/logs by default")
}

func TestInitLogging_CustomLogDir(t *testing.T) {
	resetLogOutput(t)
	exportDir := t.TempDir()
	customLogDir := t.TempDir()

	err := InitLogging(exportDir, "info", false, "test-cmd", logFileSettings{
		Dir:        customLogDir,
		MaxSizeMB:  config.DefaultLogMaxSizeMB,
		MaxBackups: config.DefaultLogMaxBackups,
	})
	require.NoError(t, err)

	expectedLogFile := filepath.Join(customLogDir, "yb-voyager-test-cmd.log")
	assert.FileExists(t, expectedLogFile, "log file should be created under the custom log-dir")

	defaultLogDir := filepath.Join(exportDir, "logs")
	assert.NoDirExists(t, defaultLogDir, "the default <export-dir>/logs directory should not be created when log-dir is overridden")
}

func TestInitLogging_DisableLogging(t *testing.T) {
	resetLogOutput(t)
	exportDir := t.TempDir()

	err := InitLogging(exportDir, "info", true, "status", logFileSettings{
		Dir:        "",
		MaxSizeMB:  config.DefaultLogMaxSizeMB,
		MaxBackups: config.DefaultLogMaxBackups,
	})
	require.NoError(t, err)

	assert.Equal(t, io.Discard, log.StandardLogger().Out, "logging should be discarded when disableLogging is true")
	assert.NoDirExists(t, filepath.Join(exportDir, "logs"), "no log directory should be created when logging is disabled")
}

func TestInitLogging_RotationSettingsApplied(t *testing.T) {
	resetLogOutput(t)
	exportDir := t.TempDir()

	err := InitLogging(exportDir, "info", false, "test-cmd", logFileSettings{
		Dir:        "",
		MaxSizeMB:  7,
		MaxBackups: 3,
	})
	require.NoError(t, err)

	rotator, ok := log.StandardLogger().Out.(*lumberjack.Logger)
	require.True(t, ok, "logrus output should be a *lumberjack.Logger")
	assert.Equal(t, 7, rotator.MaxSize)
	assert.Equal(t, 3, rotator.MaxBackups)
}

// lumberjack has no "-1" concept of its own; it treats MaxBackups == 0 as "retain
// all", so our unlimited sentinel must be translated to that at the InitLogging boundary.
func TestInitLogging_UnlimitedMaxBackupsSentinelTranslatesToLumberjackZero(t *testing.T) {
	resetLogOutput(t)
	exportDir := t.TempDir()

	err := InitLogging(exportDir, "info", false, "test-cmd", logFileSettings{
		Dir:        "",
		MaxSizeMB:  config.DefaultLogMaxSizeMB,
		MaxBackups: config.LogMaxBackupsUnlimited,
	})
	require.NoError(t, err)

	rotator, ok := log.StandardLogger().Out.(*lumberjack.Logger)
	require.True(t, ok, "logrus output should be a *lumberjack.Logger")
	assert.Equal(t, 0, rotator.MaxBackups, "lumberjack's MaxBackups should be 0 (its own 'retain all') when ours is set to unlimited")
}

func TestInitLogging_InvalidLogLevel(t *testing.T) {
	resetLogOutput(t)
	exportDir := t.TempDir()

	err := InitLogging(exportDir, "not-a-level", false, "test-cmd", logFileSettings{
		Dir:        "",
		MaxSizeMB:  config.DefaultLogMaxSizeMB,
		MaxBackups: config.DefaultLogMaxBackups,
	})
	require.Error(t, err)
}

// logFileSettingsCLIArgs is what forwards a user's --log-dir/--log-max-size-mb/--log-max-backups
// into the next iteration's spawned yb-voyager subprocess (see startExportDataFromSourceOnNextIteration
// and its counterparts) when no shared config file is doing that forwarding generically.
func TestLogFileSettingsCLIArgs(t *testing.T) {
	origDir, origMaxSize, origMaxBackups := config.LogDir, config.LogMaxSizeMB, config.LogMaxBackups
	t.Cleanup(func() {
		config.LogDir, config.LogMaxSizeMB, config.LogMaxBackups = origDir, origMaxSize, origMaxBackups
	})

	t.Run("log-dir unset", func(t *testing.T) {
		config.LogDir = ""
		config.LogMaxSizeMB = 42
		config.LogMaxBackups = 5

		args := logFileSettingsCLIArgs()

		assert.Equal(t, []string{"--log-max-size-mb", "42", "--log-max-backups", "5"}, args,
			"log-dir should be omitted when unset so the child resolves its own <export-dir>/logs default")
	})

	t.Run("log-dir set", func(t *testing.T) {
		config.LogDir = "/custom/log/dir"
		config.LogMaxSizeMB = 7
		config.LogMaxBackups = 1

		args := logFileSettingsCLIArgs()

		assert.Equal(t, []string{"--log-max-size-mb", "7", "--log-max-backups", "1", "--log-dir", "/custom/log/dir"}, args)
	})
}

// assessMigrationBulkCmd and getDataMigrationReportCmd don't call registerCommonGlobalFlags
// (see its "Note" comment), so they register log flags directly via registerLogFlags.
// This guards against that call silently disappearing from either command.
func TestRegisterLogFlags_ExposedOnCommandsThatDontUseCommonGlobalFlags(t *testing.T) {
	for _, cmd := range []*cobra.Command{assessMigrationBulkCmd, getDataMigrationReportCmd} {
		t.Run(cmd.Name(), func(t *testing.T) {
			logLevelFlag := cmd.PersistentFlags().Lookup("log-level")
			require.NotNil(t, logLevelFlag, "log-level flag should be registered")
			assert.Equal(t, "info", logLevelFlag.DefValue)

			logDirFlag := cmd.PersistentFlags().Lookup("log-dir")
			require.NotNil(t, logDirFlag, "log-dir flag should be registered")
			assert.Equal(t, "", logDirFlag.DefValue)

			logMaxSizeFlag := cmd.PersistentFlags().Lookup("log-max-size-mb")
			require.NotNil(t, logMaxSizeFlag, "log-max-size-mb flag should be registered")
			assert.Equal(t, fmt.Sprintf("%d", config.DefaultLogMaxSizeMB), logMaxSizeFlag.DefValue)

			logMaxBackupsFlag := cmd.PersistentFlags().Lookup("log-max-backups")
			require.NotNil(t, logMaxBackupsFlag, "log-max-backups flag should be registered")
			assert.Equal(t, fmt.Sprintf("%d", config.DefaultLogMaxBackups), logMaxBackupsFlag.DefValue)
		})
	}
}
