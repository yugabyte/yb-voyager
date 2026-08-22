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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr string
	}{
		{"lowercase valid", "info", ""},
		{"uppercase gets normalized", "DEBUG", ""},
		{"all valid levels", "trace", ""},
		{"invalid level", "verbose", "invalid log level: verbose. Valid log levels = [trace debug info warn error fatal panic]"},
		{"empty level", "", "invalid log level: . Valid log levels = [trace debug info warn error fatal panic]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LogLevel = tt.level
			err := ValidateLogLevel()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				// ValidateLogLevel normalizes LogLevel to lowercase.
				assert.Equal(t, strings.ToLower(tt.level), LogLevel)
			}
		})
	}
}

func TestValidateLogSettings(t *testing.T) {
	tests := []struct {
		name       string
		maxSizeMB  int
		maxBackups int
		wantErr    string
	}{
		{"defaults are valid", DefaultLogMaxSizeMB, DefaultLogMaxBackups, ""},
		{"unlimited maxBackups sentinel is valid", 50, LogMaxBackupsUnlimited, ""},
		{"zero maxSizeMB is invalid", 0, DefaultLogMaxBackups, "invalid log-max-size-mb: 0. Must be a positive integer"},
		{"negative maxSizeMB is invalid", -2, DefaultLogMaxBackups, "invalid log-max-size-mb: -2. Must be a positive integer"},
		{"zero maxBackups is invalid", DefaultLogMaxSizeMB, 0, "invalid log-max-backups: 0. Must be a positive integer, or -1 to retain all rotated files"},
		{"negative maxBackups other than the unlimited sentinel is invalid", DefaultLogMaxSizeMB, -2, "invalid log-max-backups: -2. Must be a positive integer, or -1 to retain all rotated files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LogMaxSizeMB = tt.maxSizeMB
			LogMaxBackups = tt.maxBackups
			err := ValidateLogSettings()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
