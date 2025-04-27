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
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// Helper to reset cmd state
func resetAssessMigrationCmd() {
	assessMigrationCmd.Run = func(cmd *cobra.Command, args []string) {}
	assessMigrationCmd.PreRun = func(cmd *cobra.Command, args []string) {}
	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {}
}

func setupConfigFile(t *testing.T, configContent string) (string, string) {
	configDir, err := os.MkdirTemp("/tmp", "config-dir-*")
	require.NoError(t, err)

	file, err := testutils.CreateTempFile(configDir, configContent, "yaml")
	require.NoError(t, err)
	return file, configDir
}

func setupExportDir(t *testing.T) string {
	exportDir, err := os.MkdirTemp("/tmp", "export-dir-*")
	require.NoError(t, err)

	CreateMigrationProjectIfNotExists(POSTGRESQL, exportDir)
	return exportDir
}

func TestAssessMigrationConfigBinding_SuccessCases(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetAssessMigrationCmd()

	configContent := fmt.Sprintf(`
export-dir: %s
source:
  db-type: postgresql
  db-name: test_db
  db-user: test_user
  db-schema: public
  db-host: localhost
  db-port: 5432

assess-migration:
  iops-capture-interval: 20
  target-db-version: 2.19.0.0
  assessment-metadata-dir: /tmp/assessment-metadata
`, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{"assess-migration", "--config-file", configFile})
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir)

	// Assertions on source config
	assert.Equal(t, "test_db", source.DBName)
	assert.Equal(t, "test_user", source.User)
	assert.Equal(t, "localhost", source.Host)
	assert.Equal(t, "public", source.Schema)
	assert.Equal(t, "postgresql", source.DBType)
	assert.Equal(t, 5432, source.Port)

	// Assertions on assess-migration config
	assert.Equal(t, int64(20), intervalForCapturingIOPS)
	assert.Equal(t, "2.19.0.0", targetDbVersionStrFlag)
	assert.Equal(t, "/tmp/assessment-metadata", assessmentMetadataDirFlag)
}

func TestAssessMigrationConfigBinding_CLIOverridesConfig(t *testing.T) {
	exportDir := setupExportDir(t)
	defer os.RemoveAll(exportDir)

	resetAssessMigrationCmd()

	configContent := fmt.Sprintf(`
export-dir: %s

source:
  db-type: postgresql
  db-name: test_db
  db-user: test_user
  db-schema: public
  db-host: localhost
  db-port: 5432

assess-migration:
  iops-capture-interval: 10
  target-db-version: 2.19.0.0
  assessment-metadata-dir: /tmp/assessment-metadata
`, exportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir) // <- Clean after test finishes

	rootCmd.SetArgs([]string{
		"assess-migration",
		"--config-file", configFile,
		"--iops-capture-interval", "50", // CLI override
	})
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assert that CLI wins
	assert.Equal(t, int64(50), intervalForCapturingIOPS)
	assert.Equal(t, "2.19.0.0", targetDbVersionStrFlag)                    // CLI does not override this
	assert.Equal(t, "/tmp/assessment-metadata", assessmentMetadataDirFlag) // CLI does not override this
}

func TestAssessMigrationConfigBinding_MissingFields(t *testing.T) {
	exportDir := setupExportDir(t)
	defer os.RemoveAll(exportDir)

	resetAssessMigrationCmd()

	// No assess-migration section at all
	configContent := fmt.Sprintf(`
export-dir: %s
source:
  db-type: postgresql
  db-name: test_db
  db-user: test_user
  db-schema: public
  db-host: localhost
  db-port: 5432
`, exportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{"assess-migration", "--config-file", configFile})
	err := rootCmd.Execute()

	require.NoError(t, err)                               // No error because defaults apply
	assert.Equal(t, int64(120), intervalForCapturingIOPS) // default value from your cmd.Flags()
}

func TestAssessMigrationConfigBinding_WrongSectionsIgnored(t *testing.T) {
	exportDir := setupExportDir(t)
	defer os.RemoveAll(exportDir)

	resetAssessMigrationCmd()

	// Wrong section
	configContent := fmt.Sprintf(`
export-dir: %s
export-data:
  table-list: a,b,c
`, exportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{"assess-migration", "--config-file", configFile})
	err := rootCmd.Execute()

	require.NoError(t, err)

	// Nothing should be set from wrong section
	assert.Equal(t, int64(120), intervalForCapturingIOPS) // default
	assert.Equal(t, "", targetDbVersionStrFlag)
}

func TestAssessMigrationConfigBinding_AliasHandling(t *testing.T) {
	// For now this is dummy, because assess-migration has no alias currently
	// If you add alias like ["assess-schema"], update here

	t.Skip("No alias support yet for assess-migration")
}
