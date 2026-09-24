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
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withLogSettings sets the package-level log settings for the duration of a test and
// restores them afterwards, so tests do not leak state into each other.
func withLogSettings(t *testing.T, maxSizeMB int, maxBackups int) {
	t.Helper()
	origSize, origBackups := LogMaxSizeMB, LogMaxBackups
	t.Cleanup(func() { LogMaxSizeMB, LogMaxBackups = origSize, origBackups })
	LogMaxSizeMB, LogMaxBackups = maxSizeMB, maxBackups
}

func TestValidateLogSettings(t *testing.T) {
	tests := []struct {
		name       string
		maxSizeMB  int
		maxBackups int
		wantErr    string
	}{
		{
			name:       "defaults are valid",
			maxSizeMB:  DefaultLogMaxSizeMB,
			maxBackups: DefaultLogMaxBackups,
		},
		{
			name:       "unlimited backups sentinel is valid",
			maxSizeMB:  DefaultLogMaxSizeMB,
			maxBackups: LogMaxBackupsUnlimited,
		},
		{
			name:       "smallest valid values",
			maxSizeMB:  1,
			maxBackups: 1,
		},
		{
			name:       "zero max size is rejected",
			maxSizeMB:  0,
			maxBackups: DefaultLogMaxBackups,
			wantErr:    "invalid log-max-size-mb: 0. Must be a positive integer",
		},
		{
			name:       "negative max size is rejected",
			maxSizeMB:  -1,
			maxBackups: DefaultLogMaxBackups,
			wantErr:    "invalid log-max-size-mb: -1. Must be a positive integer",
		},
		{
			// 0 is rejected rather than silently meaning "retain all", which is how
			// lumberjack itself interprets MaxBackups == 0. A user passing 0 to cap
			// disk usage would otherwise get unbounded growth.
			name:       "zero max backups is rejected rather than meaning unlimited",
			maxSizeMB:  DefaultLogMaxSizeMB,
			maxBackups: 0,
			wantErr:    "invalid log-max-backups: 0. Must be a positive integer, or -1 to retain all rotated log files",
		},
		{
			name:       "negative max backups other than the sentinel is rejected",
			maxSizeMB:  DefaultLogMaxSizeMB,
			maxBackups: -2,
			wantErr:    "invalid log-max-backups: -2. Must be a positive integer, or -1 to retain all rotated log files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withLogSettings(t, tt.maxSizeMB, tt.maxBackups)

			err := ValidateLogSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLumberjackMaxBackups(t *testing.T) {
	// lumberjack has no sentinel of its own: MaxBackups == 0 (with MaxAge 0 and no
	// compression) is what makes it retain every rotated file.
	assert.Equal(t, 0, LumberjackMaxBackups(LogMaxBackupsUnlimited),
		"unlimited sentinel should map to lumberjack's retain-all value")
	assert.Equal(t, DefaultLogMaxBackups, LumberjackMaxBackups(DefaultLogMaxBackups))
	assert.Equal(t, 1, LumberjackMaxBackups(1))
}

func TestValidateLogLevel(t *testing.T) {
	origLevel := LogLevel
	t.Cleanup(func() { LogLevel = origLevel })

	for _, level := range validLogLevels {
		LogLevel = level
		require.NoError(t, ValidateLogLevel(), "level %q should be valid", level)
	}

	// Levels are normalised to lower case rather than rejected.
	LogLevel = "DEBUG"
	require.NoError(t, ValidateLogLevel())
	assert.Equal(t, DEBUG, LogLevel)

	LogLevel = "verbose"
	err := ValidateLogLevel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level: verbose")
}
