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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
)

// restoreLogOutput restores logrus' global output after a test that calls InitLogging.
func restoreLogOutput(t *testing.T) {
	t.Helper()
	origOut, origLevel := log.StandardLogger().Out, log.GetLevel()
	t.Cleanup(func() {
		log.SetOutput(origOut)
		log.SetLevel(origLevel)
	})
}

// withLogSettings sets the resolved log settings for the duration of a test.
func withLogSettings(t *testing.T, maxSizeMB int, maxBackups int) {
	t.Helper()
	origSize, origBackups := config.LogMaxSizeMB, config.LogMaxBackups
	t.Cleanup(func() { config.LogMaxSizeMB, config.LogMaxBackups = origSize, origBackups })
	config.LogMaxSizeMB, config.LogMaxBackups = maxSizeMB, maxBackups
}

func TestInitLogging_RotationSettingsReachLumberjack(t *testing.T) {
	tests := []struct {
		name           string
		maxSizeMB      int
		maxBackups     int
		wantMaxSize    int
		wantMaxBackups int
	}{
		{
			name:           "defaults preserve the previously hardcoded behaviour",
			maxSizeMB:      config.DefaultLogMaxSizeMB,
			maxBackups:     config.DefaultLogMaxBackups,
			wantMaxSize:    200,
			wantMaxBackups: 10,
		},
		{
			name:           "custom values are passed through",
			maxSizeMB:      50,
			maxBackups:     3,
			wantMaxSize:    50,
			wantMaxBackups: 3,
		},
		{
			// The whole point of the sentinel: lumberjack retains every rotated file
			// when MaxBackups is 0 (MaxAge is 0 and compression is off).
			name:           "unlimited sentinel maps to lumberjack retain-all",
			maxSizeMB:      config.DefaultLogMaxSizeMB,
			maxBackups:     config.LogMaxBackupsUnlimited,
			wantMaxSize:    200,
			wantMaxBackups: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreLogOutput(t)
			logDir := t.TempDir()

			err := InitLogging(logDir, config.INFO, false, "test-cmd", tt.maxSizeMB, tt.maxBackups)
			require.NoError(t, err)

			rotator, ok := log.StandardLogger().Out.(*lumberjack.Logger)
			require.True(t, ok, "expected logrus output to be a lumberjack.Logger")
			assert.Equal(t, tt.wantMaxSize, rotator.MaxSize)
			assert.Equal(t, tt.wantMaxBackups, rotator.MaxBackups)
			assert.Equal(t, filepath.Join(logDir, "logs", "yb-voyager-test-cmd.log"), rotator.Filename)
			// MaxAge and Compress stay at their zero values, without which
			// MaxBackups == 0 would not actually retain everything.
			assert.Zero(t, rotator.MaxAge)
			assert.False(t, rotator.Compress)
		})
	}
}

// The behavioural guarantee behind --log-max-backups -1: rotated files are actually kept
// on disk. This exercises the real rotation path rather than only asserting the values
// handed to lumberjack, so a lumberjack upgrade that changed the meaning of MaxBackups
// would be caught here.
func TestInitLogging_UnlimitedBackupsRetainsRotatedFiles(t *testing.T) {
	countLogFiles := func(t *testing.T, logsDir string) int {
		t.Helper()
		entries, err := os.ReadDir(logsDir)
		require.NoError(t, err)
		return len(entries)
	}

	// ~64KB per line, 16 lines per MB, over a 1MB rotation threshold.
	writeAboutMB := func(mb int) {
		line := strings.Repeat("x", 64*1024)
		for i := 0; i < mb*16; i++ {
			log.Info(line)
		}
	}

	t.Run("unlimited retains every rotated file", func(t *testing.T) {
		restoreLogOutput(t)
		exportDir := t.TempDir()

		require.NoError(t, InitLogging(exportDir, config.INFO, false, "test-cmd", 1, config.LogMaxBackupsUnlimited))
		writeAboutMB(5)

		// Nothing is ever deleted, so there is no mill goroutine to race with.
		assert.Greater(t, countLogFiles(t, filepath.Join(exportDir, "logs")), 4,
			"every rotated log file should be retained")
	})

	t.Run("a backup limit still prunes", func(t *testing.T) {
		restoreLogOutput(t)
		exportDir := t.TempDir()
		logsDir := filepath.Join(exportDir, "logs")

		require.NoError(t, InitLogging(exportDir, config.INFO, false, "test-cmd", 1, 2))
		writeAboutMB(5)

		// lumberjack prunes asynchronously, so allow it to catch up.
		require.Eventually(t, func() bool { return countLogFiles(t, logsDir) <= 3 }, 10*time.Second, 50*time.Millisecond,
			"expected at most 2 backups plus the current file")
	})
}

func TestInitLogging_DisabledLoggingIgnoresRotation(t *testing.T) {
	restoreLogOutput(t)

	err := InitLogging(t.TempDir(), config.INFO, true, "status", config.DefaultLogMaxSizeMB, config.DefaultLogMaxBackups)
	require.NoError(t, err)
	assert.Equal(t, io.Discard, log.StandardLogger().Out)
}

func TestInitLogging_InvalidLogLevel(t *testing.T) {
	restoreLogOutput(t)

	err := InitLogging(t.TempDir(), "verbose", false, "test-cmd", config.DefaultLogMaxSizeMB, config.DefaultLogMaxBackups)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level verbose")
}

func TestLogSettingsCLIArgs(t *testing.T) {
	origLevel := config.LogLevel
	t.Cleanup(func() { config.LogLevel = origLevel })

	config.LogLevel = config.DEBUG
	withLogSettings(t, 50, config.LogMaxBackupsUnlimited)

	assert.Equal(t, []string{
		"--log-level", "debug",
		"--log-max-size-mb", "50",
		"--log-max-backups", "-1",
	}, logSettingsCLIArgs())
}

// The two commands below do not go through registerCommonGlobalFlags, so they need
// registerLogFlags directly. Without it their log settings would stay at Go's zero
// values and lumberjack would silently fall back to its own 100MB default.
func TestLogFlagsRegisteredOnCommandsBypassingCommonGlobalFlags(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"assess-migration-bulk":     assessMigrationBulkCmd,
		"get data-migration-report": getDataMigrationReportCmd,
	}

	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			flags := cmd.PersistentFlags()
			for _, flagName := range []string{"log-level", "log-max-size-mb", "log-max-backups"} {
				require.NotNil(t, flags.Lookup(flagName), "%s should register --%s", name, flagName)
			}
			assert.Equal(t, "200", flags.Lookup("log-max-size-mb").DefValue)
			assert.Equal(t, "10", flags.Lookup("log-max-backups").DefValue)
		})
	}
}
