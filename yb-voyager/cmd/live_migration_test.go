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
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

func TestIsCDCSavepointFixedInTargetDBVersion(t *testing.T) {
	tests := []struct {
		name               string
		dbVersionStr       string
		expectedFixed      bool
		expectedFixVersion string
	}{
		{name: "empty version string", dbVersionStr: "", expectedFixed: false, expectedFixVersion: ""},
		{name: "malformed version string", dbVersionStr: "not-a-version", expectedFixed: false, expectedFixVersion: ""},

		{name: "2024.2 series below fix", dbVersionStr: "11.2-YB-2024.2.7.0-b1", expectedFixed: false, expectedFixVersion: "2024.2.8.0"},
		{name: "2024.2 series exactly at fix", dbVersionStr: "11.2-YB-2024.2.8.0-b85", expectedFixed: true, expectedFixVersion: "2024.2.8.0"},
		{name: "2024.2 series above fix", dbVersionStr: "11.2-YB-2024.2.9.0-b1", expectedFixed: true, expectedFixVersion: "2024.2.8.0"},

		{name: "2025.1 series below fix", dbVersionStr: "11.2-YB-2025.1.3.0-b1", expectedFixed: false, expectedFixVersion: "2025.1.4.0"},
		{name: "2025.1 series exactly at fix", dbVersionStr: "11.2-YB-2025.1.4.0-b42", expectedFixed: true, expectedFixVersion: "2025.1.4.0"},
		{name: "2025.1 series above fix", dbVersionStr: "11.2-YB-2025.1.5.0-b1", expectedFixed: true, expectedFixVersion: "2025.1.4.0"},

		{name: "2025.2 series below fix", dbVersionStr: "11.2-YB-2025.2.1.0-b1", expectedFixed: false, expectedFixVersion: "2025.2.2.0"},
		{name: "2025.2 series exactly at fix", dbVersionStr: "11.2-YB-2025.2.2.0-b10", expectedFixed: true, expectedFixVersion: "2025.2.2.0"},
		{name: "2025.2 series above fix", dbVersionStr: "11.2-YB-2025.2.3.0-b1", expectedFixed: true, expectedFixVersion: "2025.2.2.0"},

		{name: "2024.1 series has no fix entry", dbVersionStr: "11.2-YB-2024.1.5.0-b1", expectedFixed: false, expectedFixVersion: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed, fixVersion := isCDCSavepointFixedInTargetDBVersion(tt.dbVersionStr)
			assert.Equal(t, tt.expectedFixed, fixed)
			assert.Equal(t, tt.expectedFixVersion, fixVersion)
		})
	}
}

func TestShouldWarnServerSeriesNewerThanConnector(t *testing.T) {
	tests := []struct {
		name             string
		connectorVersion string
		serverYBVersion  string
		expectedWarn     bool
		wantErr          bool
	}{
		{"same release, server higher maintenance is NOT newer", "2025.2.3", "2025.2.9.0", false, false},
		{"same release, server higher connector-counter-equivalent is NOT newer", "2025.2.3", "2025.2.4.0", false, false},
		{"same release exact", "2025.2.3", "2025.2.0.0", false, false},
		{"server on newer release warns", "2024.2.5", "2025.1.0.0", true, false},
		{"server on older release does not warn", "2025.2.3", "2024.2.8.0", false, false},
		{"server on unrecognized (newer) release warns", "2025.2.3", "2026.1.0.0", true, false},
		{"connector 2-segment release tag, same release", "2025.2", "2025.2.9.0", false, false},
		{"preview server warns (connector not built for preview)", "2025.2.3", "2.25.1.0", true, false},
		{"stable-old server does not warn (connector is newer and backward-compatible)", "2025.2.3", "2.20.1.0", false, false},
		{"unparseable server version returns error", "2025.2.3", "not-a-version", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, _, err := shouldWarnServerSeriesNewerThanConnector(tt.connectorVersion, tt.serverYBVersion)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedWarn, warn)
		})
	}
}

func TestParseCdcPartitionKeyOverrides(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]importdata.CdcPartitionKeyOverride
		wantErr string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]importdata.CdcPartitionKeyOverride{},
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  map[string]importdata.CdcPartitionKeyOverride{},
		},
		{
			name:  "single pk override",
			input: "public.orders:pk",
			want:  map[string]importdata.CdcPartitionKeyOverride{"public.orders": {Strategy: importdata.PARTITION_BY_PK}},
		},
		{
			name:  "multiple overrides with semicolon",
			input: "public.orders:table;sales.events:pk",
			want: map[string]importdata.CdcPartitionKeyOverride{
				"public.orders": {Strategy: importdata.PARTITION_BY_TABLE},
				"sales.events":  {Strategy: importdata.PARTITION_BY_PK},
			},
		},
		{
			name:  "trims whitespace around entries",
			input: " public.orders : table ; sales.events : pk ",
			want: map[string]importdata.CdcPartitionKeyOverride{
				"public.orders": {Strategy: importdata.PARTITION_BY_TABLE},
				"sales.events":  {Strategy: importdata.PARTITION_BY_PK},
			},
		},
		{
			name:  "trailing semicolon ignored",
			input: "public.orders:pk;",
			want:  map[string]importdata.CdcPartitionKeyOverride{"public.orders": {Strategy: importdata.PARTITION_BY_PK}},
		},
		{
			name:  "single custom column",
			input: "public.orders:(customer_id)",
			want: map[string]importdata.CdcPartitionKeyOverride{
				"public.orders": {Strategy: importdata.PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}},
			},
		},
		{
			name:  "multi custom columns",
			input: "public.orders:(customer_id,region)",
			want: map[string]importdata.CdcPartitionKeyOverride{
				"public.orders": {Strategy: importdata.PARTITION_BY_CUSTOM, Columns: []string{"customer_id", "region"}},
			},
		},
		{
			name:  "custom columns trim inner whitespace and preserve order",
			input: "public.orders:( region , customer_id )",
			want: map[string]importdata.CdcPartitionKeyOverride{
				"public.orders": {Strategy: importdata.PARTITION_BY_CUSTOM, Columns: []string{"region", "customer_id"}},
			},
		},
		{
			name:  "custom mixed with pk override",
			input: "public.orders:(customer_id);sales.events:pk",
			want: map[string]importdata.CdcPartitionKeyOverride{
				"public.orders": {Strategy: importdata.PARTITION_BY_CUSTOM, Columns: []string{"customer_id"}},
				"sales.events":  {Strategy: importdata.PARTITION_BY_PK},
			},
		},
		{
			name:    "rejects missing colon",
			input:   "public.orders",
			wantErr: "expected format",
		},
		{
			name:    "rejects empty strategy",
			input:   "public.orders:",
			wantErr: "non-empty",
		},
		{
			name:    "rejects custom column list without parentheses",
			input:   "public.orders:customer_id",
			wantErr: "parenthesized custom key column list",
		},
		{
			name:    "rejects empty parenthesized custom key",
			input:   "public.orders:()",
			wantErr: "custom key column list is empty",
		},
		{
			name:    "rejects empty column in custom key",
			input:   "public.orders:(customer_id,,region)",
			wantErr: "empty column name",
		},
		{
			name:    "rejects duplicate column in custom key",
			input:   "public.orders:(customer_id,customer_id)",
			wantErr: "duplicate column",
		},
		{
			name:    "rejects duplicate table with conflicting values",
			input:   "public.orders:pk;public.orders:table",
			wantErr: "duplicate table",
		},
		{
			name:    "rejects duplicate table even with same value",
			input:   "public.orders:(customer_id);public.orders:(customer_id)",
			wantErr: "duplicate table",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCdcPartitionKeyOverrides(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestValidateCdcPartitionKeyFlags covers the flag-level guardrails in
// validateCdcPartitionKeyFlags: context restrictions (target/YB/PG/streaming),
// value validation, and the resume change-guards (including the positive
// same-config and start-clean bypass paths).
func TestValidateCdcPartitionKeyFlags(t *testing.T) {
	origImporterRole := importerRole
	origTargetDBType := tconf.TargetDBType
	origSourceDBType := sourceDBType
	origImportType := importType
	origKey := cdcPartitionKey
	origOverrides := cdcPartitionKeyOverrides
	origStartClean := startClean
	origMetaDB := metaDB
	t.Cleanup(func() {
		importerRole = origImporterRole
		tconf.TargetDBType = origTargetDBType
		sourceDBType = origSourceDBType
		importType = origImportType
		cdcPartitionKey = origKey
		cdcPartitionKeyOverrides = origOverrides
		startClean = origStartClean
		metaDB = origMetaDB
	})

	// newCmd registers the two flags bound to the package globals. StringVar resets
	// the globals to their defaults, so callers must set globals AFTER calling this.
	newCmd := func() *cobra.Command {
		c := &cobra.Command{Use: "import-data-test"}
		c.Flags().StringVar(&cdcPartitionKey, "cdc-partition-key", "auto", "")
		c.Flags().StringVar(&cdcPartitionKeyOverrides, "cdc-partition-key-overrides", "", "")
		return c
	}
	setValidContext := func() {
		importerRole = TARGET_DB_IMPORTER_ROLE
		tconf.TargetDBType = YUGABYTEDB
		sourceDBType = POSTGRESQL
		importType = SNAPSHOT_AND_CHANGES
		startClean = false
	}
	seedMetaDB := func(t *testing.T, mutate func(*metadb.ImportDataStatusRecord)) {
		dir, err := os.MkdirTemp("", "cdcpk-metadb-*")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		metaDB = CreateMigrationProjectIfNotExists(POSTGRESQL, dir)
		if mutate != nil {
			require.NoError(t, metaDB.UpdateImportDataStatusRecord(mutate))
		}
	}

	t.Run("rejected for non-target importer role", func(t *testing.T) {
		setValidContext()
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only supported for import data to target")
	})

	t.Run("rejected for non-yugabytedb target", func(t *testing.T) {
		setValidContext()
		tconf.TargetDBType = POSTGRESQL
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only supported for import data to target")
	})

	t.Run("allowed (no-op) for non-target when no flags passed", func(t *testing.T) {
		setValidContext()
		importerRole = SOURCE_REPLICA_DB_IMPORTER_ROLE
		cmd := newCmd() // no flags Set -> not passed
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})

	t.Run("rejected for offline migration", func(t *testing.T) {
		setValidContext()
		importType = SNAPSHOT_ONLY
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported for offline migration")
	})

	t.Run("rejected for non-postgres source", func(t *testing.T) {
		setValidContext()
		sourceDBType = ORACLE
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("cdc-partition-key", "pk"))
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only supported for PostgreSQL source")
	})

	t.Run("rejects empty cdc-partition-key", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, nil)
		cmd := newCmd()
		cdcPartitionKey = ""
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cdc-partition-key is required")
	})

	t.Run("rejects invalid cdc-partition-key value", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, nil)
		cmd := newCmd()
		cdcPartitionKey = "foo"
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cdc-partition-key")
	})

	t.Run("valid fresh start passes", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, nil) // no import started yet
		cmd := newCmd()
		cdcPartitionKey = "pk"
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})

	t.Run("resume with same config passes", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = importdata.PARTITION_BY_PK
			r.CdcPartitionKeyOverridesConfig = "test_schema.orders:table"
		})
		cmd := newCmd()
		cdcPartitionKey = importdata.PARTITION_BY_PK
		cdcPartitionKeyOverrides = "test_schema.orders:table"
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})

	t.Run("resume rejects changed global key", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = "auto"
		})
		cmd := newCmd()
		cdcPartitionKey = importdata.PARTITION_BY_PK
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changing cdc-partition-key is not allowed")
	})

	// NOTE: changed cdc-partition-key-overrides is no longer rejected here by a raw string
	// compare; the semantic per-table comparison is covered by
	// TestValidateCdcPartitioningStrategyUnchanged.

	t.Run("resume from older version without stored strategy is rejected", func(t *testing.T) {
		setValidContext()
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = ""
		})
		cmd := newCmd()
		cdcPartitionKey = importdata.PARTITION_BY_PK
		err := validateCdcPartitionKeyFlags(cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Resuming from an earlier version")
	})

	t.Run("start-clean bypasses resume change-guards", func(t *testing.T) {
		setValidContext()
		startClean = true
		seedMetaDB(t, func(r *metadb.ImportDataStatusRecord) {
			r.ImportDataStarted = true
			r.CdcPartitioningStrategyConfig = "auto"
		})
		cmd := newCmd()
		cdcPartitionKey = importdata.PARTITION_BY_PK
		require.NoError(t, validateCdcPartitionKeyFlags(cmd))
	})
}
