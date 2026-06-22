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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogicalConnectorYBVersion(t *testing.T) {
	tests := []struct {
		name     string
		jarName  string
		expected string
		wantErr  bool
	}{
		{
			name:     "current logical connector jar",
			jarName:  "yugabytedb-source-connector-dz.2.5.2.yb.2025.2.3-jar-with-dependencies.jar",
			expected: "2025.2.3",
		},
		{
			name:     "older YB release",
			jarName:  "yugabytedb-source-connector-dz.2.5.2.yb.2024.2.4-jar-with-dependencies.jar",
			expected: "2024.2.4",
		},
		{
			name:     "double-digit patch",
			jarName:  "yugabytedb-source-connector-dz.2.5.2.yb.2025.2.10-jar-with-dependencies.jar",
			expected: "2025.2.10",
		},
		{
			// The gRPC connector tag embeds "yb.grpc.<ver>"; the logical-connector
			// parser must not treat it as a logical-connector version.
			name:    "grpc connector jar is rejected",
			jarName: "debezium-connector-yugabytedb-dz.1.9.5.yb.grpc.2024.2.3.jar",
			wantErr: true,
		},
		{
			name:    "unrelated jar name",
			jarName: "some-random-library-1.2.3.jar",
			wantErr: true,
		},
		{
			// With the series-level regex, the full token "2025.2.SNAPSHOT.6" is captured
			// up to the first "-". The snapshot suffix is a connector-internal counter and
			// is harmlessly ignored when series reduction happens later via SeriesVersion.
			name:     "snapshot connector jar is now parsed successfully",
			jarName:  "yugabytedb-source-connector-dz.2.5.2.yb.2025.2.SNAPSHOT.6-jar-with-dependencies.jar",
			expected: "2025.2.SNAPSHOT.6",
			wantErr:  false,
		},
		{
			name:     "connector jar that ships a PATCH segment is accepted",
			jarName:  "yugabytedb-source-connector-dz.2.5.2.yb.2025.2.3.1-jar-with-dependencies.jar",
			expected: "2025.2.3.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogicalConnectorYBVersion(tt.jarName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetLogicalConnectorYBVersion(t *testing.T) {
	origDistDir := DEBEZIUM_DIST_DIR
	t.Cleanup(func() { DEBEZIUM_DIST_DIR = origDistDir })

	t.Run("picks the connector jar among other jars", func(t *testing.T) {
		distDir := t.TempDir()
		connectorDir := filepath.Join(distDir, "yb-connector")
		require.NoError(t, os.MkdirAll(connectorDir, 0755))
		// Noise jars that should be ignored, plus the actual connector jar.
		for _, name := range []string{
			"some-dependency-1.2.3.jar",
			"yugabytedb-source-connector-dz.2.5.2.yb.2025.2.3-jar-with-dependencies.jar",
		} {
			require.NoError(t, os.WriteFile(filepath.Join(connectorDir, name), []byte("x"), 0644))
		}
		DEBEZIUM_DIST_DIR = distDir

		got, err := GetLogicalConnectorYBVersion()
		assert.NoError(t, err)
		assert.Equal(t, "2025.2.3", got)
	})

	t.Run("returns the version when duplicate jars share the same version", func(t *testing.T) {
		distDir := t.TempDir()
		connectorDir := filepath.Join(distDir, "yb-connector")
		require.NoError(t, os.MkdirAll(connectorDir, 0755))
		for _, name := range []string{
			"yugabytedb-source-connector-dz.2.5.2.yb.2025.2.3-jar-with-dependencies.jar",
			"yugabytedb-source-connector-dz.2.5.2.yb.2025.2.3-shaded.jar",
		} {
			require.NoError(t, os.WriteFile(filepath.Join(connectorDir, name), []byte("x"), 0644))
		}
		DEBEZIUM_DIST_DIR = distDir

		got, err := GetLogicalConnectorYBVersion()
		assert.NoError(t, err)
		assert.Equal(t, "2025.2.3", got)
	})

	t.Run("errors when multiple connector jars have different versions", func(t *testing.T) {
		distDir := t.TempDir()
		connectorDir := filepath.Join(distDir, "yb-connector")
		require.NoError(t, os.MkdirAll(connectorDir, 0755))
		for _, name := range []string{
			"yugabytedb-source-connector-dz.2.5.2.yb.2025.2.3-jar-with-dependencies.jar",
			"yugabytedb-source-connector-dz.2.5.2.yb.2025.2.4-jar-with-dependencies.jar",
		} {
			require.NoError(t, os.WriteFile(filepath.Join(connectorDir, name), []byte("x"), 0644))
		}
		DEBEZIUM_DIST_DIR = distDir

		_, err := GetLogicalConnectorYBVersion()
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrMultipleLogicalConnectorVersions),
			"expected ErrMultipleLogicalConnectorVersions, got: %v", err)
	})

	t.Run("errors when no connector jar is present", func(t *testing.T) {
		distDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(distDir, "yb-connector"), 0755))
		DEBEZIUM_DIST_DIR = distDir

		_, err := GetLogicalConnectorYBVersion()
		assert.Error(t, err)
	})

	t.Run("errors when distribution dir is unresolved", func(t *testing.T) {
		DEBEZIUM_DIST_DIR = ""
		_, err := GetLogicalConnectorYBVersion()
		assert.Error(t, err)
	})

	t.Run("returns the token for a snapshot connector jar", func(t *testing.T) {
		distDir := t.TempDir()
		connectorDir := filepath.Join(distDir, "yb-connector")
		require.NoError(t, os.MkdirAll(connectorDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(connectorDir, "yugabytedb-source-connector-dz.2.5.2.yb.2025.2.SNAPSHOT.6-jar-with-dependencies.jar"),
			[]byte("x"), 0644))
		DEBEZIUM_DIST_DIR = distDir

		got, err := GetLogicalConnectorYBVersion()
		assert.NoError(t, err)
		assert.Equal(t, "2025.2.SNAPSHOT.6", got)
	})
}
