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
package dbzm

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
)

// debezium-<role>.log is rotated by the same settings as yb-voyager's own log file, so a
// user asking to retain all logs does not silently keep losing the debezium ones.
func TestSetupLogFile_RotationSettings(t *testing.T) {
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
			maxSizeMB:      25,
			maxBackups:     2,
			wantMaxSize:    25,
			wantMaxBackups: 2,
		},
		{
			name:           "unlimited sentinel maps to lumberjack retain-all",
			maxSizeMB:      config.DefaultLogMaxSizeMB,
			maxBackups:     config.LogMaxBackupsUnlimited,
			wantMaxSize:    200,
			wantMaxBackups: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exportDir := t.TempDir()
			d := NewDebezium(&Config{
				ExportDir:     exportDir,
				ExporterRole:  "source_db_exporter",
				LogMaxSizeMB:  tt.maxSizeMB,
				LogMaxBackups: tt.maxBackups,
			})
			d.cmd = exec.Command("true")

			require.NoError(t, d.setupLogFile())

			rotator, ok := d.cmd.Stdout.(*lumberjack.Logger)
			require.True(t, ok, "expected debezium stdout to be a lumberjack.Logger")
			assert.Same(t, rotator, d.cmd.Stderr, "stdout and stderr should share one rotator")
			assert.Equal(t, tt.wantMaxSize, rotator.MaxSize)
			assert.Equal(t, tt.wantMaxBackups, rotator.MaxBackups)

			wantPath, err := filepath.Abs(filepath.Join(exportDir, "logs", "debezium-source_db_exporter.log"))
			require.NoError(t, err)
			assert.Equal(t, wantPath, rotator.Filename)
		})
	}
}
