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
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// Helper functions

// The function resetCmdAndEnvVars needs to be run before running the config file tests
// as env vars can have some values set to them in the test env.
func resetCmdAndEnvVars(cmd *cobra.Command) {
	cmd.Run = func(cmd *cobra.Command, args []string) {}
	cmd.PreRun = func(cmd *cobra.Command, args []string) {}
	cmd.PostRun = func(cmd *cobra.Command, args []string) {}
	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {}
	// go through all the env vars in confParamEnvVarPairs and unset them
	for _, envVar := range confParamEnvVarPairs {
		if envVar != "" {
			os.Unsetenv(envVar)
		}
	}

}

// The flags and the variables related to them need to be reset to their default values
// after running each test case. This is needed to ensure that the stale state does not
// get carried over to the next test case. Hence, it is run separetely from resetCmdAndEnvVars
// after the test case is run.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
		f.Changed = false
	})
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

type testContext struct {
	tmpExportDir string
	configFile   string
}

////////////////////////////// Validation Logic Tests //////////////////////////////

func setupViperFromYAML(t *testing.T, yamlContent string) *viper.Viper {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)

	v := viper.New()
	v.SetConfigFile(tmpFile.Name())
	err = v.ReadInConfig()
	require.NoError(t, err)

	return v
}

func TestValidateConfig_InvalidGlobalKeyOutput(t *testing.T) {
	yaml := `
bad-global-key: true
bad-global-key2: false
export-dir: /tmp/export
`
	v := setupViperFromYAML(t, yaml)

	err := validateConfigFile(v)

	// Check for expected error type (ValidationError)
	var validationErr *ConfigValidationError
	require.Error(t, err, "Expected an error for invalid global key")
	require.True(t, errors.As(err, &validationErr), "Expected error of type *ValidationError")

	// Now verify that the invalid global key is included in the ValidationError
	assert.True(t, validationErr.InvalidGlobalKeys.Contains("bad-global-key"), "Expected 'bad-global-key' to be in InvalidGlobalKeys")
	assert.True(t, validationErr.InvalidGlobalKeys.Contains("bad-global-key2"), "Expected 'bad-global-key2' to be in InvalidGlobalKeys")
	// Also make sure that this is the only key in InvalidGlobalKeys
	assert.Equal(t, 2, validationErr.InvalidGlobalKeys.Cardinality(), "Expected InvalidGlobalKeys to contain exactly one key")

	// Ensure that all other sets are empty
	assert.Empty(t, validationErr.InvalidSectionKeys, "Expected InvalidSectionKeys to be empty")
	assert.Empty(t, validationErr.InvalidSections, "Expected InvalidSections to be empty")
	assert.Empty(t, validationErr.ConflictingSections, "Expected ConflictingSections to be empty")
}

func TestValidateConfig_ConflictingAliasSectionsOutput(t *testing.T) {
	yaml := `
export-data:
  table-list: a,b
export-data-from-source:
  table-list: x,y
import-data:
  table-list: c,d
import-data-to-target:
  table-list: e,f
`
	v := setupViperFromYAML(t, yaml)

	err := validateConfigFile(v)

	// Check for expected error type (ValidationError)
	var validationErr *ConfigValidationError
	require.Error(t, err, "Expected an error for conflicting sections")
	require.True(t, errors.As(err, &validationErr), "Expected error of type *ConfigValidationError")

	// Now verify that exactly one conflicting sections array contains 'export-data' and 'export-data-from-source'
	assert.Len(t, validationErr.ConflictingSections, 2, "Expected exactly two conflicting sections array")

	// Now verify that one string array contains only 'export-data' and 'export-data-from-source'
	assert.Len(t, validationErr.ConflictingSections[0], 2, "Expected the conflicting sections array to contain exactly two sections")
	assert.ElementsMatch(t, validationErr.ConflictingSections[0], []string{"export-data", "export-data-from-source"}, "Expected 'export-data' and 'export-data-from-source' to be in the conflicting sections")
	// Now verify that the other string array contains only 'import-data' and 'import-data-to-target'
	assert.Len(t, validationErr.ConflictingSections[1], 2, "Expected the conflicting sections array to contain exactly two sections")
	assert.ElementsMatch(t, validationErr.ConflictingSections[1], []string{"import-data", "import-data-to-target"}, "Expected 'import-data' and 'import-data-to-target' to be in the conflicting sections")

	// Ensure that all other sets are empty
	assert.Empty(t, validationErr.InvalidGlobalKeys, "Expected InvalidGlobalKeys to be empty")
	assert.Empty(t, validationErr.InvalidSectionKeys, "Expected InvalidSectionKeys to be empty")
	assert.Empty(t, validationErr.InvalidSections, "Expected InvalidSections to be empty")
}

func TestValidateConfig_InvalidSectionOutput(t *testing.T) {
	yaml := `
bad-section:
  something: true
bad-section2:
  another-thing: false
`
	v := setupViperFromYAML(t, yaml)

	err := validateConfigFile(v)

	// Check for expected error type (ValidationError)
	var validationErr *ConfigValidationError
	require.Error(t, err, "Expected an error for invalid section")
	require.True(t, errors.As(err, &validationErr), "Expected error of type *ConfigValidationError")

	// Now verify that the invalid sections are included in the ValidationError
	assert.Equal(t, 2, validationErr.InvalidSections.Cardinality(), "Expected InvalidSections to contain exactly two sections")
	assert.True(t, validationErr.InvalidSections.Contains("bad-section"), "Expected 'bad-section' to be in InvalidSections")
	assert.True(t, validationErr.InvalidSections.Contains("bad-section2"), "Expected 'bad-section2' to be in InvalidSections")

	// Ensure that all other sets are empty
	assert.Empty(t, validationErr.InvalidGlobalKeys, "Expected InvalidGlobalKeys to be empty")
	assert.Empty(t, validationErr.InvalidSectionKeys, "Expected InvalidSectionKeys to be empty")
	assert.Empty(t, validationErr.ConflictingSections, "Expected ConflictingSections to be empty")
}

func TestValidateConfig_InvalidNestedKeyOutput(t *testing.T) {
	yaml := `
source:
  db-user: postgres
  invalid-key: value
target:
  db-user: yugabyte
  invalid-key: value
  invalid-key2: value
`
	v := setupViperFromYAML(t, yaml)
	err := validateConfigFile(v)

	// Check for expected error type (ValidationError)
	var validationErr *ConfigValidationError
	require.Error(t, err, "Expected an error for invalid nested key")
	require.True(t, errors.As(err, &validationErr), "Expected error of type *ConfigValidationError")

	// Now verify that the invalid nested keys are included in the ValidationError
	assert.Len(t, validationErr.InvalidSectionKeys, 2, "Expected InvalidSectionKeys to contain two sections")

	assert.True(t, validationErr.InvalidSectionKeys["source"].Contains("invalid-key"), "Expected 'invalid-key' to be in source InvalidSectionKeys")
	assert.Equal(t, 1, validationErr.InvalidSectionKeys["source"].Cardinality(), "Expected source InvalidSectionKeys to contain exactly one key")

	assert.True(t, validationErr.InvalidSectionKeys["target"].Contains("invalid-key"), "Expected 'invalid-key' to be in target InvalidSectionKeys")
	assert.True(t, validationErr.InvalidSectionKeys["target"].Contains("invalid-key2"), "Expected 'invalid-key2' to be in target InvalidSectionKeys")
	assert.Equal(t, 2, validationErr.InvalidSectionKeys["target"].Cardinality(), "Expected target InvalidSectionKeys to contain exactly one key")
}

func TestValidateConfig_ValidConfigOutput(t *testing.T) {
	yaml := `
export-dir: /tmp/export
source:
  db-type: postgresql
  db-name: test_db
  db-user: postgres
  db-host: localhost
  db-schema: public
  db-port: 5432
assess-migration:
  iops-capture-interval: 30
`
	v := setupViperFromYAML(t, yaml)
	err := validateConfigFile(v)
	require.NoError(t, err, "Expected no error for valid config")
}

////////////////////////////// Assess Migration Tests //////////////////////////////

func setupAssessMigrationContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(assessMigrationCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(assessMigrationCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin

source:
  name: test_source
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/cert.pem
  ssl-mode: require
  ssl-key: /path/to/key.pem
  ssl-root-cert: /path/to/root.pem
  ssl-crl: /path/to/crl.pem
  oracle-db-sid: test_sid
  oracle-home: /path/to/oracle/home
  oracle-tns-alias: test_tns_alias
  oracle-cdb-name: test_cdb_name
  oracle-cdb-sid: test_cdb_sid
  oracle-cdb-tns-alias: test_cdb_tns_alias

assess-migration:
  run-guardrails-checks: false
  iops-capture-interval: 20
  target-db-version: 2.19.0.0
  assessment-metadata-dir: /tmp/assessment-metadata
  invoked-by-export-schema: true
  report-unsupported-query-constructs: true
  report-unsupported-plpgsql-objects: true
`, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestAssessMigration_ConfigFileBinding(t *testing.T) {
	ctx := setupAssessMigrationContext(t)

	rootCmd.SetArgs([]string{
		"assess-migration",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")

	// Assertions on source config
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "test_password", source.Password, "Source password should match the config")
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "/path/to/cert.pem", source.SSLCertPath, "Source SSL cert should match the config")
	assert.Equal(t, "require", source.SSLMode, "Source SSL mode should match the config")
	assert.Equal(t, "/path/to/key.pem", source.SSLKey, "Source SSL key should match the config")
	assert.Equal(t, "/path/to/root.pem", source.SSLRootCert, "Source SSL root cert should match the config")
	assert.Equal(t, "/path/to/crl.pem", source.SSLCRL, "Source SSL CRL should match the config")
	assert.Equal(t, "test_sid", source.DBSid, "Source Oracle DB SID should match the config")
	assert.Equal(t, "/path/to/oracle/home", source.OracleHome, "Source Oracle home should match the config")
	assert.Equal(t, "test_tns_alias", source.TNSAlias, "Source Oracle TNS alias should match the config")
	// Dont test CDB fields as they are not available in assess migration command

	// Assertions on assess-migration config
	assert.Equal(t, utils.BoolStr(false), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, int64(20), intervalForCapturingIOPS, "intervalForCapturingIOPS should match the config")
	assert.Equal(t, "2.19.0.0", targetDbVersionStrFlag, "targetDbVersionStrFlag should match the config")
	assert.Equal(t, "/tmp/assessment-metadata", assessmentMetadataDirFlag, "assessmentMetadataDirFlag should match the config")
	assert.Equal(t, utils.BoolStr(true), invokedByExportSchema, "invokedByExportSchema should match the config")
	assert.Equal(t, "true", os.Getenv("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS"), "REPORT_UNSUPPORTED_QUERY_CONSTRUCTS should match the config value")
	assert.Equal(t, "true", os.Getenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS"), "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS should match the config value")
}

func TestAssessMigration_CLIOverridesConfig(t *testing.T) {
	// Test whether CLI overrides config
	ctx := setupAssessMigrationContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	rootCmd.SetArgs([]string{
		"assess-migration",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--iops-capture-interval", "50",
		"--target-db-version", "2024.1.1.0",
		"--assessment-metadata-dir", "/tmp/new-assessment-metadata",
		"--invoked-by-export-schema", "false",
		"--source-db-type", "postgres",
		"--source-db-host", "localhost2",
		"--source-db-port", "5433",
		"--source-db-user", "test_user2",
		"--source-db-password", "test_password2",
		"--source-db-name", "test_db2",
		"--source-db-schema", "public2",
		"--source-ssl-cert", "/path/to/cert2.pem",
		"--source-ssl-mode", "verify-full",
		"--source-ssl-key", "/path/to/key2.pem",
		"--source-ssl-root-cert", "/path/to/root2.pem",
		"--source-ssl-crl", "/path/to/crl2.pem",
		"--oracle-db-sid", "test_sid2",
		"--oracle-home", "/path/to/oracle/home2",
		"--oracle-tns-alias", "test_tns_alias2",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")

	// Assertions on source config
	assert.Equal(t, "postgres", source.DBType, "Source DB type should be overridden by CLI")
	assert.Equal(t, "localhost2", source.Host, "Source host should be overridden by CLI")
	assert.Equal(t, 5433, source.Port, "Source port should be overridden by CLI")
	assert.Equal(t, "test_user2", source.User, "Source user should be overridden by CLI")
	assert.Equal(t, "test_password2", source.Password, "Source password should be overridden by CLI")
	assert.Equal(t, "test_db2", source.DBName, "Source DB name should be overridden by CLI")
	assert.Equal(t, "public2", source.Schema, "Source schema should be overridden by CLI")
	assert.Equal(t, "/path/to/cert2.pem", source.SSLCertPath, "Source SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", source.SSLMode, "Source SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/key2.pem", source.SSLKey, "Source SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/root2.pem", source.SSLRootCert, "Source SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/crl2.pem", source.SSLCRL, "Source SSL CRL should be overridden by CLI")
	assert.Equal(t, "test_sid2", source.DBSid, "Source Oracle DB SID should be overridden by CLI")
	assert.Equal(t, "/path/to/oracle/home2", source.OracleHome, "Source Oracle home should be overridden by CLI")
	assert.Equal(t, "test_tns_alias2", source.TNSAlias, "Source Oracle TNS alias should be overridden by CLI")
	// Dont test CDB fields as they are not available in assess migration command

	// Assertions on assess-migration config
	assert.Equal(t, utils.BoolStr(true), source.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, int64(50), intervalForCapturingIOPS, "intervalForCapturingIOPS should be overridden by CLI")
	assert.Equal(t, "2024.1.1.0", targetDbVersionStrFlag, "targetDbVersionStrFlag should be overridden by CLI")
	assert.Equal(t, "/tmp/new-assessment-metadata", assessmentMetadataDirFlag, "assessmentMetadataDirFlag should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), invokedByExportSchema, "invokedByExportSchema should be overridden by CLI")
	assert.Equal(t, "true", os.Getenv("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS"), "REPORT_UNSUPPORTED_QUERY_CONSTRUCTS should match the config")
	assert.Equal(t, "true", os.Getenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS"), "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS should match the config")
}

func TestAssessMigration_EnvOverridesConfig(t *testing.T) {
	// Test whether env vars overrides config
	ctx := setupAssessMigrationContext(t)

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to assess-migration config
	os.Setenv("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS", "false")
	os.Setenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS", "false")
	rootCmd.SetArgs([]string{
		"assess-migration",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")

	// Assertions on source config
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "test_password", source.Password, "Source password should match the config")
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "/path/to/cert.pem", source.SSLCertPath, "Source SSL cert should match the config")
	assert.Equal(t, "require", source.SSLMode, "Source SSL mode should match the config")
	assert.Equal(t, "/path/to/key.pem", source.SSLKey, "Source SSL key should match the config")
	assert.Equal(t, "/path/to/root.pem", source.SSLRootCert, "Source SSL root cert should match the config")
	assert.Equal(t, "/path/to/crl.pem", source.SSLCRL, "Source SSL CRL should match the config")
	assert.Equal(t, "test_sid", source.DBSid, "Source Oracle DB SID should match the config")
	assert.Equal(t, "/path/to/oracle/home", source.OracleHome, "Source Oracle home should match the config")
	assert.Equal(t, "test_tns_alias", source.TNSAlias, "Source Oracle TNS alias should match the config")
	// Dont test CDB fields as they are not available in assess migration command

	// Assertions on assess-migration config
	assert.Equal(t, utils.BoolStr(false), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, int64(20), intervalForCapturingIOPS, "intervalForCapturingIOPS should match the config")
	assert.Equal(t, "2.19.0.0", targetDbVersionStrFlag, "targetDbVersionStrFlag should match the config")
	assert.Equal(t, "/tmp/assessment-metadata", assessmentMetadataDirFlag, "assessmentMetadataDirFlag should match the config")
	assert.Equal(t, utils.BoolStr(true), invokedByExportSchema, "invokedByExportSchema should match the config")
	assert.Equal(t, "false", os.Getenv("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS"), "REPORT_UNSUPPORTED_QUERY_CONSTRUCTS should be overridden by env var")
	assert.Equal(t, "false", os.Getenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS"), "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS should be overridden by env var")
}

func TestAssessMigration_GlobalVsLocalConfigPrecedence(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)
	// Run command export data from source but the config section is export-data
	resetCmdAndEnvVars(assessMigrationCmd)
	defer resetFlags(assessMigrationCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
source:
  db-type: postgresql
assess-migration:
  log-level: debug
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{
		"assess-migration",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global section config")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command section config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global section config")
}

////////////////////////////// Export Schema Tests ////////////////////////////////

func setupExportSchemaContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(exportSchemaCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(exportSchemaCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
source:
  name: test_source
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/cert.pem
  ssl-mode: require
  ssl-key: /path/to/key.pem
  ssl-root-cert: /path/to/root.pem
  ssl-crl: /path/to/crl.pem
  oracle-db-sid: test_sid
  oracle-home: /path/to/oracle/home
  oracle-tns-alias: test_tns_alias
  oracle-cdb-name: test_cdb_name
  oracle-cdb-sid: test_cdb_sid
  oracle-cdb-tns-alias: test_cdb_tns_alias
export-schema:
  run-guardrails-checks: false
  use-orafce: true
  comments-on-objects: true
  object-type-list: table,index,view
  exclude-object-type-list: materialized_view
  skip-sharding-recommendations: false
  assessment-report-path: /tmp/assessment-report
  assess-schema-before-export: false
  ybvoyager-skip-merge-constraints-transformation: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestExportSchemaConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupExportSchemaContext(t)

	rootCmd.SetArgs([]string{
		"export", "schema",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on source config
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "test_password", source.Password, "Source password should match the config")
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "/path/to/cert.pem", source.SSLCertPath, "Source SSL cert should match the config")
	assert.Equal(t, "require", source.SSLMode, "Source SSL mode should match the config")
	assert.Equal(t, "/path/to/key.pem", source.SSLKey, "Source SSL key should match the config")
	assert.Equal(t, "/path/to/root.pem", source.SSLRootCert, "Source SSL root cert should match the config")
	assert.Equal(t, "/path/to/crl.pem", source.SSLCRL, "Source SSL CRL should match the config")
	assert.Equal(t, "test_sid", source.DBSid, "Source Oracle DB SID should match the config")
	assert.Equal(t, "/path/to/oracle/home", source.OracleHome, "Source Oracle home should match the config")
	assert.Equal(t, "test_tns_alias", source.TNSAlias, "Source Oracle TNS alias should match the config")
	// Dont test CDB fields as they are not available in export schema command
	// Assertions on export-schema config
	assert.Equal(t, utils.BoolStr(false), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), source.UseOrafce, "UseOrafce should match the config")
	assert.Equal(t, utils.BoolStr(true), source.CommentsOnObjects, "CommentsOnObjects should match the config")
	assert.Equal(t, "table,index,view", source.StrExportObjectTypeList, "Export object type list should match the config")
	assert.Equal(t, "materialized_view", source.StrExcludeObjectTypeList, "Exclude object type list should match the config")
	assert.Equal(t, utils.BoolStr(false), skipRecommendations, "Skip recommendations should match the config")
	assert.Equal(t, "/tmp/assessment-report", assessmentReportPath, "Assessment report path should match the config")
	assert.Equal(t, utils.BoolStr(false), assessSchemaBeforeExport, "Assess schema before export should match the config")
	assert.Equal(t, "true", os.Getenv("YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS"), "YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS should match the config")
}

func TestExportSchemaConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupExportSchemaContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"export", "schema",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--use-orafce", "false",
		"--comments-on-objects", "false",
		"--object-type-list", "table,view",
		"--exclude-object-type-list", "materialized_view,index",
		"--skip-sharding-recommendations", "true",
		"--assessment-report-path", "/tmp/new-assessment-report",
		"--assess-schema-before-export", "true",
		"--source-db-type", "postgres",
		"--source-db-host", "localhost2",
		"--source-db-port", "5433",
		"--source-db-user", "test_user2",
		"--source-db-password", "test_password2",
		"--source-db-name", "test_db2",
		"--source-db-schema", "public2",
		"--source-ssl-cert", "/path/to/cert2.pem",
		"--source-ssl-mode", "verify-full",
		"--source-ssl-key", "/path/to/key2.pem",
		"--source-ssl-root-cert", "/path/to/root2.pem",
		"--source-ssl-crl", "/path/to/crl2.pem",
		"--oracle-db-sid", "test_sid2",
		"--oracle-home", "/path/to/oracle/home2",
		"--oracle-tns-alias", "test_tns_alias2",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on source config
	assert.Equal(t, "postgres", source.DBType, "Source DB type should be overridden by CLI")
	assert.Equal(t, "localhost2", source.Host, "Source host should be overridden by CLI")
	assert.Equal(t, 5433, source.Port, "Source port should be overridden by CLI")
	assert.Equal(t, "test_user2", source.User, "Source user should be overridden by CLI")
	assert.Equal(t, "test_password2", source.Password, "Source password should be overridden by CLI")
	assert.Equal(t, "test_db2", source.DBName, "Source DB name should be overridden by CLI")
	assert.Equal(t, "public2", source.Schema, "Source schema should be overridden by CLI")
	assert.Equal(t, "/path/to/cert2.pem", source.SSLCertPath, "Source SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", source.SSLMode, "Source SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/key2.pem", source.SSLKey, "Source SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/root2.pem", source.SSLRootCert, "Source SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/crl2.pem", source.SSLCRL, "Source SSL CRL should be overridden by CLI")
	assert.Equal(t, "test_sid2", source.DBSid, "Source Oracle DB SID should be overridden by CLI")
	assert.Equal(t, "/path/to/oracle/home2", source.OracleHome, "Source Oracle home should be overridden by CLI")
	assert.Equal(t, "test_tns_alias2", source.TNSAlias, "Source Oracle TNS alias should be overridden by CLI")
	// Dont test CDB fields as they are not available in export schema command
	// Assertions on export-schema config
	assert.Equal(t, utils.BoolStr(true), source.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), source.UseOrafce, "UseOrafce should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), source.CommentsOnObjects, "CommentsOnObjects should be overridden by CLI")
	assert.Equal(t, "table,view", source.StrExportObjectTypeList, "Export object type list should be overridden by CLI")
	assert.Equal(t, "materialized_view,index", source.StrExcludeObjectTypeList, "Exclude object type list should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), skipRecommendations, "Skip recommendations should be overridden by CLI")
	assert.Equal(t, "/tmp/new-assessment-report", assessmentReportPath, "Assessment report path should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), assessSchemaBeforeExport, "Assess schema before export should be overridden by CLI")
	assert.Equal(t, "true", os.Getenv("YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS"), "YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS should match the config")
}

func TestExportSchemaConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupExportSchemaContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to export-schema config
	os.Setenv("YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS", "true")
	rootCmd.SetArgs([]string{
		"export", "schema",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on source config
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "test_password", source.Password, "Source password should match the config")
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "/path/to/cert.pem", source.SSLCertPath, "Source SSL cert should match the config")
	assert.Equal(t, "require", source.SSLMode, "Source SSL mode should match the config")
	assert.Equal(t, "/path/to/key.pem", source.SSLKey, "Source SSL key should match the config")
	assert.Equal(t, "/path/to/root.pem", source.SSLRootCert, "Source SSL root cert should match the config")
	assert.Equal(t, "/path/to/crl.pem", source.SSLCRL, "Source SSL CRL should match the config")
	assert.Equal(t, "test_sid", source.DBSid, "Source Oracle DB SID should match the config")
	assert.Equal(t, "/path/to/oracle/home", source.OracleHome, "Source Oracle home should match the config")
	assert.Equal(t, "test_tns_alias", source.TNSAlias, "Source Oracle TNS alias should match the config")
	// Dont test CDB fields as they are not available in export schema command
	// Assertions on export-schema config
	assert.Equal(t, utils.BoolStr(false), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), source.UseOrafce, "UseOrafce should match the config")
	assert.Equal(t, utils.BoolStr(true), source.CommentsOnObjects, "CommentsOnObjects should match the config")
	assert.Equal(t, "table,index,view", source.StrExportObjectTypeList, "Export object type list should match the config")
	assert.Equal(t, "materialized_view", source.StrExcludeObjectTypeList, "Exclude object type list should match the config")
	assert.Equal(t, utils.BoolStr(false), skipRecommendations, "Skip recommendations should match the config")
	assert.Equal(t, "/tmp/assessment-report", assessmentReportPath, "Assessment report path should match the config")
	assert.Equal(t, utils.BoolStr(false), assessSchemaBeforeExport, "Assess schema before export should match the config")
	assert.Equal(t, "true", os.Getenv("YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS"), "YB_VOYAGER_SKIP_MERGE_CONSTRAINTS_TRANSFORMATIONS should be overridden by env var")
}

func TestExportSchema_GlobalVsLocalConfigPrecedence(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)
	// Run command export schema but the config section is export-schema
	resetCmdAndEnvVars(exportSchemaCmd)
	defer resetFlags(exportSchemaCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
export-schema:
  log-level: debug
`, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{
		"export", "schema",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global section config")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command section config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the gloabl section config")
}

////////////////////////////// Analyze Schema Tests ////////////////////////////////

func setupAnalyzeSchemaContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(analyzeSchemaCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(analyzeSchemaCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
analyze-schema:
  output-format: json
  target-db-version: 2024.1.1.0
  report-unsupported-plpgsql-objects: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestAnalyzeSchemaConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupAnalyzeSchemaContext(t)

	rootCmd.SetArgs([]string{
		"analyze-schema",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on analyze-schema config
	assert.Equal(t, "json", analyzeSchemaReportFormat, "Output format should match the config")
	assert.Equal(t, "2024.1.1.0", targetDbVersionStrFlag, "Target DB version should match the config")
	assert.Equal(t, "true", os.Getenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS"), "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS should match the config value")
}

func TestAnalyzeSchemaConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupAnalyzeSchemaContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"analyze-schema",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--profile", "false",
		"--output-format", "yaml",
		"--target-db-version", "2024.2.0.0",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by CLI")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on analyze-schema config
	assert.Equal(t, "yaml", analyzeSchemaReportFormat, "Output format should be overridden by CLI")
	assert.Equal(t, "2024.2.0.0", targetDbVersionStrFlag, "Target DB version should be overridden by CLI")
	assert.Equal(t, "true", os.Getenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS"), "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS should match the config value")
}

func TestAnalyzeSchemaConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupAnalyzeSchemaContext(t)

	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to analyze-schema config
	os.Setenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS", "false")
	rootCmd.SetArgs([]string{
		"analyze-schema",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on analyze-schema config
	assert.Equal(t, "json", analyzeSchemaReportFormat, "Output format should match the config")
	assert.Equal(t, "2024.1.1.0", targetDbVersionStrFlag, "Target DB version should match the config")
	assert.Equal(t, "false", os.Getenv("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS"), "REPORT_UNSUPPORTED_PLPGSQL_OBJECTS should be overridden by env var")
}

func TestAnalyzeSchema_GlobalVsLocalConfigPrecedence(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)
	// Run command analyze schema but the config section is analyze-schema
	resetCmdAndEnvVars(analyzeSchemaCmd)
	defer resetFlags(analyzeSchemaCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
analyze-schema:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{
		"analyze-schema",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global section config")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command section config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global section config")
}

////////////////////////////// Export Data Tests ////////////////////////////////

func setupExportDataFromSourceContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(exportDataFromSrcCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(exportDataFromSrcCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
source:
  name: test_source
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/cert.pem
  ssl-mode: require
  ssl-key: /path/to/key.pem
  ssl-root-cert: /path/to/root.pem
  ssl-crl: /path/to/crl.pem
  oracle-db-sid: test_sid
  oracle-home: /path/to/oracle/home
  oracle-tns-alias: test_tns_alias
  oracle-cdb-name: test_cdb_name
  oracle-cdb-sid: test_cdb_sid
  oracle-cdb-tns-alias: test_cdb_tns_alias
export-data-from-source:
  run-guardrails-checks: false
  disable-pb: true
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  parallel-jobs: 4
  export-type: snapshot-and-changes
  queue-segment-max-bytes: 10485760
  debezium-dist-dir: /tmp/debezium
  beta-fast-data-export: true
  allow-oracle-clob-data-export: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestExportDataFromSourceConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupExportDataFromSourceContext(t)

	rootCmd.SetArgs([]string{
		"export", "data", "from", "source",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on source config
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "test_password", source.Password, "Source password should match the config")
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "/path/to/cert.pem", source.SSLCertPath, "Source SSL cert should match the config")
	assert.Equal(t, "require", source.SSLMode, "Source SSL mode should match the config")
	assert.Equal(t, "/path/to/key.pem", source.SSLKey, "Source SSL key should match the config")
	assert.Equal(t, "/path/to/root.pem", source.SSLRootCert, "Source SSL root cert should match the config")
	assert.Equal(t, "/path/to/crl.pem", source.SSLCRL, "Source SSL CRL should match the config")
	assert.Equal(t, "test_sid", source.DBSid, "Source Oracle DB SID should match the config")
	assert.Equal(t, "/path/to/oracle/home", source.OracleHome, "Source Oracle home should match the config")
	assert.Equal(t, "test_tns_alias", source.TNSAlias, "Source Oracle TNS alias should match the config")
	assert.Equal(t, "test_cdb_name", source.CDBName, "Source Oracle CDB name should match the config")
	assert.Equal(t, "test_cdb_sid", source.CDBSid, "Source Oracle CDB SID should match the config")
	assert.Equal(t, "test_cdb_tns_alias", source.CDBTNSAlias, "Source Oracle CDB TNS alias should match the config")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(false), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table1,table2", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table3,table4", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, 4, source.NumConnections, "Parallel jobs should match the config")
	assert.Equal(t, "snapshot-and-changes", exportType, "Export type should match the config")
	assert.Equal(t, utils.BoolStr(true), source.AllowOracleClobDataExport, "Allow Oracle CLOB data export should match the config")
	assert.Equal(t, "10485760", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should match the config")
	assert.Equal(t, "/tmp/debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should match the config")
	assert.Equal(t, "true", os.Getenv("BETA_FAST_DATA_EXPORT"), "BETA_FAST_DATA_EXPORT should match the config")
}
func TestExportDataFromSourceConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupExportDataFromSourceContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"export", "data", "from", "source",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--disable-pb", "false",
		"--exclude-table-list", "table5,table6",
		"--table-list", "table7,table8",
		"--exclude-table-list-file-path", "/tmp/new-exclude-tables.txt",
		"--table-list-file-path", "/tmp/new-table-list.txt",
		"--parallel-jobs", "8",
		"--export-type", "snapshot-only",
		"--allow-oracle-clob-data-export", "false",
		"--source-db-type", "postgres",
		"--source-db-host", "localhost2",
		"--source-db-port", "5433",
		"--source-db-user", "test_user2",
		"--source-db-password", "test_password2",
		"--source-db-name", "test_db2",
		"--source-db-schema", "public2",
		"--source-ssl-cert", "/path/to/cert2.pem",
		"--source-ssl-mode", "verify-full",
		"--source-ssl-key", "/path/to/key2.pem",
		"--source-ssl-root-cert", "/path/to/root2.pem",
		"--source-ssl-crl", "/path/to/crl2.pem",
		"--oracle-db-sid", "test_sid2",
		"--oracle-home", "/path/to/oracle/home2",
		"--oracle-tns-alias", "test_tns_alias2",
		"--oracle-cdb-name", "test_cdb_name2",
		"--oracle-cdb-sid", "test_cdb_sid2",
		"--oracle-cdb-tns-alias", "test_cdb_tns_alias2",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on source config
	assert.Equal(t, "postgres", source.DBType, "Source DB type should be overridden by CLI")
	assert.Equal(t, "localhost2", source.Host, "Source host should be overridden by CLI")
	assert.Equal(t, 5433, source.Port, "Source port should be overridden by CLI")
	assert.Equal(t, "test_user2", source.User, "Source user should be overridden by CLI")
	assert.Equal(t, "test_password2", source.Password, "Source password should be overridden by CLI")
	assert.Equal(t, "test_db2", source.DBName, "Source DB name should be overridden by CLI")
	assert.Equal(t, "public2", source.Schema, "Source schema should be overridden by CLI")
	assert.Equal(t, "/path/to/cert2.pem", source.SSLCertPath, "Source SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", source.SSLMode, "Source SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/key2.pem", source.SSLKey, "Source SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/root2.pem", source.SSLRootCert, "Source SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/crl2.pem", source.SSLCRL, "Source SSL CRL should be overridden by CLI")
	assert.Equal(t, "test_sid2", source.DBSid, "Source Oracle DB SID should be overridden by CLI")
	assert.Equal(t, "/path/to/oracle/home2", source.OracleHome, "Source Oracle home should be overridden by CLI")
	assert.Equal(t, "test_tns_alias2", source.TNSAlias, "Source Oracle TNS alias should be overridden by CLI")
	assert.Equal(t, "test_cdb_name2", source.CDBName, "Source Oracle CDB name should be overridden by CLI")
	assert.Equal(t, "test_cdb_sid2", source.CDBSid, "Source Oracle CDB SID should be overridden by CLI")
	assert.Equal(t, "test_cdb_tns_alias2", source.CDBTNSAlias, "Source Oracle CDB TNS alias should be overridden by CLI")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(true), source.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), disablePb, "Disable PB should be overridden by CLI")
	assert.Equal(t, "table5,table6", source.ExcludeTableList, "Exclude table list should be overridden by CLI")
	assert.Equal(t, "table7,table8", source.TableList, "Table list should be overridden by CLI")
	assert.Equal(t, "/tmp/new-exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should be overridden by CLI")
	assert.Equal(t, "/tmp/new-table-list.txt", tableListFilePath, "Table list file path should be overridden by CLI")
	assert.Equal(t, 8, source.NumConnections, "Parallel jobs should be overridden by CLI")
	assert.Equal(t, "snapshot-only", exportType, "Export type should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), source.AllowOracleClobDataExport, "Allow Oracle CLOB data export should be overridden by CLI")
	assert.Equal(t, "10485760", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should be overridden by env var")
	assert.Equal(t, "/tmp/debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should be overridden by env var")
	assert.Equal(t, "true", os.Getenv("BETA_FAST_DATA_EXPORT"), "BETA_FAST_DATA_EXPORT should be overridden by env var")
}

func TestExportDataFromSourceConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupExportDataFromSourceContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to export-data config
	os.Setenv("QUEUE_SEGMENT_MAX_BYTES", "20971520")
	os.Setenv("DEBEZIUM_DIST_DIR", "/tmp/new-debezium")
	os.Setenv("BETA_FAST_DATA_EXPORT", "false")

	rootCmd.SetArgs([]string{
		"export", "data", "from", "source",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on source config
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "test_password", source.Password, "Source password should match the config")
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "/path/to/cert.pem", source.SSLCertPath, "Source SSL cert should match the config")
	assert.Equal(t, "require", source.SSLMode, "Source SSL mode should match the config")
	assert.Equal(t, "/path/to/key.pem", source.SSLKey, "Source SSL key should match the config")
	assert.Equal(t, "/path/to/root.pem", source.SSLRootCert, "Source SSL root cert should match the config")
	assert.Equal(t, "/path/to/crl.pem", source.SSLCRL, "Source SSL CRL should match the config")
	assert.Equal(t, "test_sid", source.DBSid, "Source Oracle DB SID should match the config")
	assert.Equal(t, "/path/to/oracle/home", source.OracleHome, "Source Oracle home should match the config")
	assert.Equal(t, "test_tns_alias", source.TNSAlias, "Source Oracle TNS alias should match the config")
	assert.Equal(t, "test_cdb_name", source.CDBName, "Source Oracle CDB name should match the config")
	assert.Equal(t, "test_cdb_sid", source.CDBSid, "Source Oracle CDB SID should match the config")
	assert.Equal(t, "test_cdb_tns_alias", source.CDBTNSAlias, "Source Oracle CDB TNS alias should match the config")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(false), source.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table1,table2", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table3,table4", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, 4, source.NumConnections, "Parallel jobs should match the config")
	assert.Equal(t, "snapshot-and-changes", exportType, "Export type should match the config")
	assert.Equal(t, utils.BoolStr(true), source.AllowOracleClobDataExport, "Allow Oracle CLOB data export should match the config")
	assert.Equal(t, "20971520", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should be overridden by env var")
	assert.Equal(t, "/tmp/new-debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should be overridden by env var")
	assert.Equal(t, "false", os.Getenv("BETA_FAST_DATA_EXPORT"), "BETA_FAST_DATA_EXPORT should be overridden by env var")
}

func TestExportDataFromSourceAliasHandling(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)
	// Run command export data from source but the config section is export-data
	resetCmdAndEnvVars(exportDataFromSrcCmd)
	defer resetFlags(exportDataFromSrcCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
source:
  db-type: postgresql
  db-name: test_db
  db-user: test_user
  db-schema: public
  db-host: localhost
  db-port: 5432
export-data:
  disable-pb: true
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  parallel-jobs: 4
  export-type: snapshot-and-changes
  queue-segment-max-bytes: 10485760
  debezium-dist-dir: /tmp/debezium
  beta-fast-data-export: true
  allow-oracle-clob-data-export: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)
	rootCmd.SetArgs([]string{
		"export", "data", "from", "source",
		"--config-file", configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the config")
	// Assertions on source config
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table1,table2", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table3,table4", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, 4, source.NumConnections, "Parallel jobs should match the config")
	assert.Equal(t, "snapshot-and-changes", exportType, "Export type should match the config")
	assert.Equal(t, utils.BoolStr(true), source.AllowOracleClobDataExport, "Allow Oracle CLOB data export should match the config")
	assert.Equal(t, "10485760", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should match the config")
	assert.Equal(t, "/tmp/debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should match the config")
	assert.Equal(t, "true", os.Getenv("BETA_FAST_DATA_EXPORT"), "BETA_FAST_DATA_EXPORT should match the config")

	// Now reverse. Run command export data but the config section is export-data-from-source
	resetCmdAndEnvVars(exportDataCmd)
	defer resetFlags(exportDataCmd)
	configContent = fmt.Sprintf(`
export-dir: %s
source:
  db-type: postgresql
  db-name: test_db
  db-user: test_user
  db-schema: public
  db-host: localhost
  db-port: 5432
export-data-from-source:
  disable-pb: true
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  parallel-jobs: 4
  export-type: snapshot-and-changes
  queue-segment-max-bytes: 10485760
  debezium-dist-dir: /tmp/debezium
  beta-fast-data-export: true
  allow-oracle-clob-data-export: true
`, tmpExportDir)
	configFile, configDir = setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)
	rootCmd.SetArgs([]string{
		"export", "data",
		"--config-file", configFile,
	})

	err = rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the config")
	// Assertions on source config
	assert.Equal(t, "test_db", source.DBName, "Source DB name should match the config")
	assert.Equal(t, "test_user", source.User, "Source user should match the config")
	assert.Equal(t, "localhost", source.Host, "Source host should match the config")
	assert.Equal(t, "public", source.Schema, "Source schema should match the config")
	assert.Equal(t, "postgresql", source.DBType, "Source DB type should match the config")
	assert.Equal(t, 5432, source.Port, "Source port should match the config")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table1,table2", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table3,table4", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, 4, source.NumConnections, "Parallel jobs should match the config")
	assert.Equal(t, "snapshot-and-changes", exportType, "Export type should match the config")
	assert.Equal(t, utils.BoolStr(true), source.AllowOracleClobDataExport, "Allow Oracle CLOB data export should match the config")
	assert.Equal(t, "10485760", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should match the config")
	assert.Equal(t, "/tmp/debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should match the config")
	assert.Equal(t, "true", os.Getenv("BETA_FAST_DATA_EXPORT"), "BETA_FAST_DATA_EXPORT should match the config")
}

func TestExportDataFromSource_GlobalVsLocalConfigPrecedence(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(exportDataFromSrcCmd)
	defer resetFlags(exportDataFromSrcCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: debug
send-diagnostics: true
export-data-from-source:
  log-level: info
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{
		"export", "data", "from", "source",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global section config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the command section config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global section config")
}

////////////////////////////// Import Schema Tests ////////////////////////////////

func setupImportSchemaContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(importSchemaCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(importSchemaCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
target:
  name: test_target
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
import-schema:
  run-guardrails-checks: false
  continue-on-error: true
  object-type-list: table,index,view
  exclude-object-type-list: materialized_view
  straight-order: true
  ignore-exist: true
  enable-orafce: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestImportSchemaConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupImportSchemaContext(t)

	rootCmd.SetArgs([]string{
		"import", "schema",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-schema config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.ContinueOnError, "Continue on error should match the config")
	assert.Equal(t, "table,index,view", tconf.ImportObjects, "Object type list should match the config")
	assert.Equal(t, "materialized_view", tconf.ExcludeImportObjects, "Exclude object type list should match the config")
	assert.Equal(t, utils.BoolStr(true), importObjectsInStraightOrder, "Straight order flag should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.IgnoreIfExists, "Ignore exist flag should match the config")
	assert.Equal(t, utils.BoolStr(true), enableOrafce, "Enable orafce flag should match the config")
}

func TestImportSchemaConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupImportSchemaContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"import", "schema",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--continue-on-error", "false",
		"--object-type-list", "table,view",
		"--exclude-object-type-list", "materialized_view,index",
		"--straight-order", "false",
		"--ignore-exist", "false",
		"--enable-orafce", "false",
		"--target-db-host", "localhost2",
		"--target-db-port", "5433",
		"--target-db-user", "test_user2",
		"--target-db-password", "test_password2",
		"--target-db-name", "test_db2",
		"--target-db-schema", "public2",
		"--target-ssl-cert", "/path/to/ssl-cert2",
		"--target-ssl-mode", "verify-full",
		"--target-ssl-key", "/path/to/ssl-key2",
		"--target-ssl-root-cert", "/path/to/ssl-root-cert2",
		"--target-ssl-crl", "/path/to/ssl-crl2",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost2", tconf.Host, "Target host should be overridden by CLI")
	assert.Equal(t, 5433, tconf.Port, "Target port should be overridden by CLI")
	assert.Equal(t, "test_user2", tconf.User, "Target user should be overridden by CLI")
	assert.Equal(t, "test_password2", tconf.Password, "Target password should be overridden by CLI")
	assert.Equal(t, "test_db2", tconf.DBName, "Target DB name should be overridden by CLI")
	assert.Equal(t, "public2", tconf.Schema, "Target schema should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-cert2", tconf.SSLCertPath, "Target SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", tconf.SSLMode, "Target SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-key2", tconf.SSLKey, "Target SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-root-cert2", tconf.SSLRootCert, "Target SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-crl2", tconf.SSLCRL, "Target SSL CRL should be overridden by CLI")
	// Assertions on import-schema config
	assert.Equal(t, utils.BoolStr(true), tconf.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.ContinueOnError, "Continue on error should be overridden by CLI")
	assert.Equal(t, "table,view", tconf.ImportObjects, "Object type list should be overridden by CLI")
	assert.Equal(t, "materialized_view,index", tconf.ExcludeImportObjects, "Exclude object type list should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), importObjectsInStraightOrder, "Straight order flag should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.IgnoreIfExists, "Ignore exist flag should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), enableOrafce, "Enable orafce flag should be overridden by CLI")
}

func TestImportSchemaConfigBinding_GlobalVsLocalConfigPrecedence(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(importSchemaCmd)
	defer resetFlags(importSchemaCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
target:
  db-user: test_user
import-schema:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	defer os.RemoveAll(configDir)

	rootCmd.SetArgs([]string{
		"import", "schema",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global section config")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command section config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global section config")
}

///////////////////////////// Import Data Tests ////////////////////////////////

func setupImportDataContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(importDataCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(importDataCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
target:
  name: test_target
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
import-data:
  run-guardrails-checks: false
  batch-size: 1000
  parallel-jobs: 4
  enable-adaptive-parallelism: true
  adaptive-parallelism-max: 10
  skip-replication-checks: true
  disable-pb: true
  max-retries: 3
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  enable-upsert: true
  use-public-ip: true
  target-endpoints: endpoint1,endpoint2
  truncate-tables: true
  skip-node-health-checks: true
  skip-disk-usage-health-checks: true
  on-primary-key-conflict: IGNORE
  disable-transactional-writes: true
  truncate-splits: false
  csv-reader-max-buffer-size-bytes: 10485760
  error-policy-snapshot: stash-and-continue
  ybvoyager-max-colocated-batches-in-progress: 5
  num-event-channels: 8
  event-channel-size: 10000
  max-events-per-batch: 5000
  max-interval-between-batches: 2
  max-cpu-threshold: 80
  adaptive-parallelism-frequency-seconds: 30
  min-available-memory-threshold: 1000000000
  max-batch-size-bytes: 10485760
  ybvoyager-use-task-picker-for-import: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestImportDataConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupImportDataContext(t)

	rootCmd.SetArgs([]string{
		"import", "data",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-data config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, int64(1000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 4, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should match the config")
	assert.Equal(t, 10, tconf.MaxParallelism, "Adaptive parallelism max should match the config")
	assert.Equal(t, utils.BoolStr(true), skipReplicationChecks, "Skip replication checks should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should match the config")
	assert.Equal(t, "table1,table2", tconf.ExcludeTableList, "Exclude table list for importing data should match the config")
	assert.Equal(t, "table3,table4", tconf.TableList, "Table list for importing data should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path for importing data should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableUpsert, "Enable upsert for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.UsePublicIP, "Use public IP for importing data should match the config")
	assert.Equal(t, "endpoint1,endpoint2", tconf.TargetEndpoints, "Target endpoints for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipNodeHealthChecks, "Skip node health checks for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipDiskUsageHealthChecks, "Skip disk usage health checks for importing data should match the config")
	assert.Equal(t, "IGNORE", tconf.OnPrimaryKeyConflictAction, "On primary key conflict for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.DisableTransactionalWrites, "Disable transaction writes for importing data should match the config")
	assert.Equal(t, utils.BoolStr(false), truncateSplits, "Truncate splits for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the config")
	assert.Equal(t, importdata.StashAndContinueErrorPolicy, errorPolicySnapshotFlag, "error-policy-snapshot should match the config")

	assert.Equal(t, "5", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "8", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the config")
	assert.Equal(t, "10000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the config")
	assert.Equal(t, "5000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the config")
	assert.Equal(t, "2", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the config")
	assert.Equal(t, "80", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the config")
	assert.Equal(t, "30", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the config")
	assert.Equal(t, "1000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
	assert.Equal(t, "true", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the config")
}

func TestImportDataConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupImportDataContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"import", "data",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--batch-size", "2000",
		"--parallel-jobs", "8",
		"--enable-adaptive-parallelism", "false",
		"--adaptive-parallelism-max", "5",
		"--skip-replication-checks", "false",
		"--disable-pb", "false",
		"--max-retries", "5",
		"--exclude-table-list", "table5,table6",
		"--table-list", "table7,table8",
		"--exclude-table-list-file-path", "/tmp/new-exclude-tables.txt",
		"--table-list-file-path", "/tmp/new-table-list.txt",
		"--enable-upsert", "false",
		"--use-public-ip", "false",
		"--target-endpoints", "endpoint3,endpoint4",
		"--truncate-tables", "false",
		"--skip-node-health-checks", "false",
		"--skip-disk-usage-health-checks", "false",
		"--on-primary-key-conflict", "ERROR",
		"--disable-transactional-writes", "false",
		"--truncate-splits", "true",
		"--error-policy-snapshot", "abort",
		"--target-db-host", "localhost2",
		"--target-db-port", "5433",
		"--target-db-user", "test_user2",
		"--target-db-password", "test_password2",
		"--target-db-name", "test_db2",
		"--target-db-schema", "public2",
		"--target-ssl-cert", "/path/to/ssl-cert2",
		"--target-ssl-mode", "verify-full",
		"--target-ssl-key", "/path/to/ssl-key2",
		"--target-ssl-root-cert", "/path/to/ssl-root-cert2",
		"--target-ssl-crl", "/path/to/ssl-crl2",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost2", tconf.Host, "Target host should be overridden by CLI")
	assert.Equal(t, 5433, tconf.Port, "Target port should be overridden by CLI")
	assert.Equal(t, "test_user2", tconf.User, "Target user should be overridden by CLI")
	assert.Equal(t, "test_password2", tconf.Password, "Target password should be overridden by CLI")
	assert.Equal(t, "test_db2", tconf.DBName, "Target DB name should be overridden by CLI")
	assert.Equal(t, "public2", tconf.Schema, "Target schemashould be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-cert2", tconf.SSLCertPath, "Target SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", tconf.SSLMode, "Target SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-key2", tconf.SSLKey, "Target SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-root-cert2", tconf.SSLRootCert, "Target SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-crl2", tconf.SSLCRL, "Target SSL CRL should be overridden by CLI")
	// Assertions on import-data config
	assert.Equal(t, utils.BoolStr(true), tconf.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, int64(2000), batchSizeInNumRows, "Batch size should be overridden by CLI")
	assert.Equal(t, 8, tconf.Parallelism, "Parallel jobs should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should be overridden by CLI")
	assert.Equal(t, 5, tconf.MaxParallelism, "Adaptive parallelism max should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), skipReplicationChecks, "Skip replication checks should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), disablePb, "Disable PB should be overridden by CLI")
	assert.Equal(t, 5, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should be overridden by CLI")
	assert.Equal(t, "table5,table6", tconf.ExcludeTableList, "Exclude table list for importing data should be overridden by CLI")
	assert.Equal(t, "table7,table8", tconf.TableList, "Table list for importing data should be overridden by CLI")
	assert.Equal(t, "/tmp/new-exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path for importing data should be overridden by CLI")
	assert.Equal(t, "/tmp/new-table-list.txt", tableListFilePath, "Table list file path for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.EnableUpsert, "Enable upsert for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.UsePublicIP, "Use public IP for importing data should be overridden by CLI")
	assert.Equal(t, "endpoint3,endpoint4", tconf.TargetEndpoints, "Target endpoints for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), truncateTables, "Truncate tables for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), skipNodeHealthChecks, "Skip node health checks for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), skipDiskUsageHealthChecks, "Skip disk usage health checks for importing data should be overridden by CLI")
	assert.Equal(t, "ERROR", tconf.OnPrimaryKeyConflictAction, "On primary key conflict for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.DisableTransactionalWrites, "Disable transaction writes for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), truncateSplits, "Truncate splits for importing data should be overridden by CLI")
	assert.Equal(t, "10485760", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the config")
	assert.Equal(t, importdata.AbortErrorPolicy, errorPolicySnapshotFlag, "error-policy-snapshot should be overridden by the CLI")

	assert.Equal(t, "5", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "8", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the config")
	assert.Equal(t, "10000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the config")
	assert.Equal(t, "5000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the config")
	assert.Equal(t, "2", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the config")
	assert.Equal(t, "80", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the config")
	assert.Equal(t, "30", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the config")
	assert.Equal(t, "1000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
	assert.Equal(t, "true", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the config")
}

func TestImportDataConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupImportDataContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to import-data
	os.Setenv("CSV_READER_MAX_BUFFER_SIZE_BYTES", "20971520")
	os.Setenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS", "10")
	os.Setenv("NUM_EVENT_CHANNELS", "16")
	os.Setenv("EVENT_CHANNEL_SIZE", "20000")
	os.Setenv("MAX_EVENTS_PER_BATCH", "10000")
	os.Setenv("MAX_INTERVAL_BETWEEN_BATCHES", "5")
	os.Setenv("MAX_CPU_THRESHOLD", "90")
	os.Setenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS", "60")
	os.Setenv("MIN_AVAILABLE_MEMORY_THRESHOLD", "2000000000")
	os.Setenv("MAX_BATCH_SIZE_BYTES", "20971520")
	os.Setenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT", "false")

	rootCmd.SetArgs([]string{
		"import", "data",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-data config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, int64(1000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 4, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should match the config")
	assert.Equal(t, 10, tconf.MaxParallelism, "Adaptive parallelism max should match the env var")
	assert.Equal(t, utils.BoolStr(true), skipReplicationChecks, "Skip replication checks should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should match the config")
	assert.Equal(t, "table1,table2", tconf.ExcludeTableList, "Exclude table list for importing data should match the config")
	assert.Equal(t, "table3,table4", tconf.TableList, "Table list for importing data should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path for importing data should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableUpsert, "Enable upsert for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.UsePublicIP, "Use public IP for importing data should match the config")
	assert.Equal(t, "endpoint1,endpoint2", tconf.TargetEndpoints, "Target endpoints for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipNodeHealthChecks, "Skip node health checks for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipDiskUsageHealthChecks, "Skip disk usage health checks for importing data should match the config")
	assert.Equal(t, "IGNORE", tconf.OnPrimaryKeyConflictAction, "On primary key conflict for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.DisableTransactionalWrites, "Disable transaction writes for importing data should match the config")
	assert.Equal(t, utils.BoolStr(false), truncateSplits, "Truncate splits for importing data should match the config")
	assert.Equal(t, "20971520", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the config")
	assert.Equal(t, importdata.StashAndContinueErrorPolicy, errorPolicySnapshotFlag, "error-policy-snapshot should match the config")

	assert.Equal(t, "10", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the env var")
	assert.Equal(t, "16", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the env var")
	assert.Equal(t, "20000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the env var")
	assert.Equal(t, "10000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the env var")
	assert.Equal(t, "5", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the env var")
	assert.Equal(t, "90", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the env var")
	assert.Equal(t, "60", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the env var")
	assert.Equal(t, "2000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the env var")
	assert.Equal(t, "20971520", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the env var")
	assert.Equal(t, "false", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the env var")
}

func TestImportDataToTargetAliasHandling(t *testing.T) {
	// Import data to target has alias "import-data"
	// Setup config file for import-data and run import data to target command
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(importDataToTargetCmd)
	defer resetFlags(importDataToTargetCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
target:
  name: test_target
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
import-data:
  batch-size: 1000
  parallel-jobs: 4
  enable-adaptive-parallelism: true
  adaptive-parallelism-max: 10
  skip-replication-checks: true
  disable-pb: true
  max-retries: 3
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  enable-upsert: true
  use-public-ip: true
  target-endpoints: endpoint1,endpoint2
  truncate-tables: true
  csv-reader-max-buffer-size-bytes: 10485760
  error-policy-snapshot: stash-and-continue
  ybvoyager-max-colocated-batches-in-progress: 5
  num-event-channels: 8
  event-channel-size: 10000
  max-events-per-batch: 5000
  max-interval-between-batches: 2
  max-cpu-threshold: 80
  adaptive-parallelism-frequency-seconds: 30
  min-available-memory-threshold: 1000000000
  max-batch-size-bytes: 10485760
  ybvoyager-use-task-picker-for-import: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"import", "data", "to", "target",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-data config
	assert.Equal(t, int64(1000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 4, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should match the config")
	assert.Equal(t, 10, tconf.MaxParallelism, "Adaptive parallelism max should match the config")
	assert.Equal(t, utils.BoolStr(true), skipReplicationChecks, "Skip replication checks should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should match the config")
	assert.Equal(t, "table1,table2", tconf.ExcludeTableList, "Exclude table list for importing data should match the config")
	assert.Equal(t, "table3,table4", tconf.TableList, "Table list for importing data should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path for importing data should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableUpsert, "Enable upsert for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.UsePublicIP, "Use public IP for importing data should match the config")
	assert.Equal(t, "endpoint1,endpoint2", tconf.TargetEndpoints, "Target endpoints for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the config")
	assert.Equal(t, importdata.StashAndContinueErrorPolicy, errorPolicySnapshotFlag, "error-policy-snapshot should match the config")
	assert.Equal(t, "5", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "8", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the config")
	assert.Equal(t, "10000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the config")
	assert.Equal(t, "5000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the config")
	assert.Equal(t, "2", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the config")
	assert.Equal(t, "80", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the config")
	assert.Equal(t, "30", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the config")
	assert.Equal(t, "1000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
	assert.Equal(t, "true", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the config")

	// Setup config file for import data to targer and run command import data
	tmpExportDir = setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })
	resetCmdAndEnvVars(importDataCmd)
	defer resetFlags(importDataCmd)
	configContent = fmt.Sprintf(`
export-dir: %s
target:
  name: test_target
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
import-data-to-target:
  batch-size: 1000
  parallel-jobs: 4
  enable-adaptive-parallelism: true
  adaptive-parallelism-max: 10
  skip-replication-checks: true
  disable-pb: true
  max-retries: 3
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  enable-upsert: true
  use-public-ip: true
  target-endpoints: endpoint1,endpoint2
  truncate-tables: true
  csv-reader-max-buffer-size-bytes: 10485760
  error-policy-snapshot: stash-and-continue
  ybvoyager-max-colocated-batches-in-progress: 5
  num-event-channels: 8
  event-channel-size: 10000
  max-events-per-batch: 5000
  max-interval-between-batches: 2
  max-cpu-threshold: 80
  adaptive-parallelism-frequency-seconds: 30
  min-available-memory-threshold: 1000000000
  max-batch-size-bytes: 10485760
  ybvoyager-use-task-picker-for-import: true
`, tmpExportDir)
	configFile, configDir = setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"import", "data",
		"--config-file", configFile,
	})

	err = rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-data config
	assert.Equal(t, int64(1000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 4, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should match the config")
	assert.Equal(t, 10, tconf.MaxParallelism, "Adaptive parallelism max should match the config")
	assert.Equal(t, utils.BoolStr(true), skipReplicationChecks, "Skip replication checks should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should match the config")
	assert.Equal(t, "table1,table2", tconf.ExcludeTableList, "Exclude table list for importing data should match the config")
	assert.Equal(t, "table3,table4", tconf.TableList, "Table list for importing data should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path for importing data should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableUpsert, "Enable upsert for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.UsePublicIP, "Use public IP for importing data should match the config")
	assert.Equal(t, "endpoint1,endpoint2", tconf.TargetEndpoints, "Target endpoints for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the config")
	assert.Equal(t, importdata.StashAndContinueErrorPolicy, errorPolicySnapshotFlag, "error-policy-snapshot should match the config")

	assert.Equal(t, "5", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "8", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the config")
	assert.Equal(t, "10000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the config")
	assert.Equal(t, "5000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the config")
	assert.Equal(t, "2", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the config")
	assert.Equal(t, "80", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the config")
	assert.Equal(t, "30", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the config")
	assert.Equal(t, "1000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
	assert.Equal(t, "true", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the config")

}

func TestImportDataToTarget_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(importDataToTargetCmd)
	defer resetFlags(importDataToTargetCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
target:
  db-user: test_user
import-data-to-target:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"import", "data", "to", "target",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}

///////////////////////////// Import Data File Tests ////////////////////////////////

func setupImportDataFileContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(importDataFileCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(importDataFileCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
target:
  name: test_target
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
import-data-file:
  batch-size: 1000
  parallel-jobs: 4
  enable-adaptive-parallelism: true
  adaptive-parallelism-max: 10
  disable-pb: true
  max-retries: 3
  enable-upsert: true
  use-public-ip: true
  target-endpoints: endpoint1,endpoint2
  format: csv
  delimiter: ","
  data-dir: /path/to/data/dir
  file-table-map: table1,file1.csv;table2,file2.csv
  has-header: true
  escape-char: "\\"
  quote-char: '"'
  file-opts: option1=value1;option2=value2
  null-string: "\\N"
  truncate-tables: true
  disable-transactional-writes: true
  truncate-splits: false
  skip-replication-checks: true
  skip-node-health-checks: true
  skip-disk-usage-health-checks: true
  on-primary-key-conflict: IGNORE
  error-policy: stash-and-continue
  csv-reader-max-buffer-size-bytes: 10485760
  ybvoyager-max-colocated-batches-in-progress: 5
  max-cpu-threshold: 80
  adaptive-parallelism-frequency-seconds: 30
  min-available-memory-threshold: 1000000000
  max-batch-size-bytes: 10485760
  ybvoyager-use-task-picker-for-import: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestImportDataFileConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupImportDataFileContext(t)

	rootCmd.SetArgs([]string{
		"import", "data", "file",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-data-file config
	assert.Equal(t, int64(1000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 4, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should match the config")
	assert.Equal(t, 10, tconf.MaxParallelism, "Adaptive parallelism max should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableUpsert, "Enable upsert for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.UsePublicIP, "Use public IP for importing data should match the config")
	assert.Equal(t, "endpoint1,endpoint2", tconf.TargetEndpoints, "Target endpoints for importing data should match the config")
	assert.Equal(t, "csv", fileFormat, "Format for importing data should match the config")
	assert.Equal(t, ",", delimiter, "Delimiter for importing data should match the config")
	assert.Equal(t, "/path/to/data/dir", dataDir, "Data directory for importing data should match the config")
	assert.Equal(t, "table1,file1.csv;table2,file2.csv", fileTableMapping, "File table map for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), hasHeader, "Has header for importing data should match the config")
	assert.Equal(t, "\\", escapeChar, "Escape character for importing data should match the config")
	assert.Equal(t, "\"", quoteChar, "Quote character for importing data should match the config")
	assert.Equal(t, "option1=value1;option2=value2", fileOpts, "File options for importing data should match the config")
	assert.Equal(t, "\\N", nullString, "Null string for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.DisableTransactionalWrites, "Disable transactional writes for importing data should match the config")
	assert.Equal(t, utils.BoolStr(false), truncateSplits, "Truncate splits for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipReplicationChecks, "Skip replication checks for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipNodeHealthChecks, "Skip node health checks for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipDiskUsageHealthChecks, "Skip disk usage health checks for importing data should match the config")
	assert.Equal(t, "IGNORE", tconf.OnPrimaryKeyConflictAction, "On primary key conflict for importing data should match the config")
	assert.Equal(t, importdata.StashAndContinueErrorPolicy, errorPolicySnapshotFlag, "error-policy should match the config")

	assert.Equal(t, "10485760", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the config")
	assert.Equal(t, "5", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "80", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the config")
	assert.Equal(t, "30", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the config")
	assert.Equal(t, "1000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
	assert.Equal(t, "true", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the config")
}

func TestImportDataFileConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupImportDataFileContext(t)

	// Create a temporary export directory to test CLI overrides
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"import", "data", "file",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--profile", "false",
		"--batch-size", "2000",
		"--parallel-jobs", "8",
		"--enable-adaptive-parallelism", "false",
		"--adaptive-parallelism-max", "5",
		"--disable-pb", "false",
		"--max-retries", "5",
		"--enable-upsert", "false",
		"--use-public-ip", "false",
		"--target-endpoints", "endpoint3,endpoint4",
		"--format", "json",
		"--delimiter", ";",
		"--data-dir", "/tmp/data-dir",
		"--file-table-map", "table5,file5.json;table6,file6.json",
		"--has-header", "false",
		"--escape-char", "\\\\",
		"--quote-char", "'",
		"--file-opts", "option3=value3;option4=value4",
		"--null-string", "\\N2",
		"--truncate-tables", "false",
		"--disable-transactional-writes", "false",
		"--truncate-splits", "true",
		"--skip-replication-checks", "false",
		"--skip-node-health-checks", "false",
		"--skip-disk-usage-health-checks", "false",
		"--on-primary-key-conflict", "ERROR",
		"--error-policy", "abort",
		"--target-db-host", "localhost2",
		"--target-db-port", "5433",
		"--target-db-user", "test_user2",
		"--target-db-password", "test_password2",
		"--target-db-name", "test_db2",
		"--target-db-schema", "public2",
		"--target-ssl-cert", "/path/to/ssl-cert2",
		"--target-ssl-mode", "verify-full",
		"--target-ssl-key", "/path/to/ssl-key2",
		"--target-ssl-root-cert", "/path/to/ssl-root-cert2",
		"--target-ssl-crl", "/path/to/ssl-crl2",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")

	// Assertions on target config
	assert.Equal(t, "localhost2", tconf.Host, "Target host should be overridden by CLI")
	assert.Equal(t, 5433, tconf.Port, "Target port should be overridden by CLI")
	assert.Equal(t, "test_user2", tconf.User, "Target user should be overridden by CLI")
	assert.Equal(t, "test_password2", tconf.Password, "Target password should be overridden by CLI")
	assert.Equal(t, "test_db2", tconf.DBName, "Target DB name should be overridden by CLI")
	assert.Equal(t, "public2", tconf.Schema, "Target schema should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-cert2", tconf.SSLCertPath, "Target SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", tconf.SSLMode, "Target SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-key2", tconf.SSLKey, "Target SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-root-cert2", tconf.SSLRootCert, "Target SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-crl2", tconf.SSLCRL, "Target SSL CRL should be overridden by CLI")
	// Assertions on import-data-file config
	assert.Equal(t, int64(2000), batchSizeInNumRows, "Batch size should be overridden by CLI")
	assert.Equal(t, 8, tconf.Parallelism, "Parallel jobs should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should be overridden by CLI")
	assert.Equal(t, 5, tconf.MaxParallelism, "Adaptive parallelism max should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), disablePb, "Disable PB should be overridden by CLI")
	assert.Equal(t, 5, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.EnableUpsert, "Enable upsert for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.UsePublicIP, "Use public IP for importing data should be overridden by CLI")
	assert.Equal(t, "endpoint3,endpoint4", tconf.TargetEndpoints, "Target endpoints for importing data should be overridden by CLI")
	assert.Equal(t, "json", fileFormat, "Format for importing data should be overridden by CLI")
	assert.Equal(t, ";", delimiter, "Delimiter for importing data should be overridden by CLI")
	assert.Equal(t, "/tmp/data-dir", dataDir, "Data directory for importing data should be overridden by CLI")
	assert.Equal(t, "table5,file5.json;table6,file6.json", fileTableMapping, "File table map for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), hasHeader, "Has header for importing data should be overridden by CLI")
	assert.Equal(t, "\\\\", escapeChar, "Escape character for importing data should be overridden by CLI")
	assert.Equal(t, "'", quoteChar, "Quote character for importing data should be overridden by CLI")
	assert.Equal(t, "option3=value3;option4=value4", fileOpts, "File options for importing data should be overridden by CLI")
	assert.Equal(t, "\\N2", nullString, "Null string for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), truncateTables, "Truncate tables for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.DisableTransactionalWrites, "Disable transactional writes for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), truncateSplits, "Truncate splits for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), skipReplicationChecks, "Skip replication checks for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), skipNodeHealthChecks, "Skip node health checks for importing data should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), skipDiskUsageHealthChecks, "Skip disk usage health checks for importing data should be overridden by CLI")
	assert.Equal(t, "ERROR", tconf.OnPrimaryKeyConflictAction, "On primary key conflict for importing data should be overridden by CLI")
	assert.Equal(t, importdata.AbortErrorPolicy, errorPolicySnapshotFlag, "error-policy should be overridden by the CLI")

	assert.Equal(t, "10485760", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should be overridden by CLI")
	assert.Equal(t, "5", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "80", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the config")
	assert.Equal(t, "30", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the config")
	assert.Equal(t, "1000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
	assert.Equal(t, "true", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the config")
}

func TestImportDataFileConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupImportDataFileContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to import-data-file
	os.Setenv("CSV_READER_MAX_BUFFER_SIZE_BYTES", "20971520")
	os.Setenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS", "10")
	os.Setenv("MAX_CPU_THRESHOLD", "90")
	os.Setenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS", "60")
	os.Setenv("MIN_AVAILABLE_MEMORY_THRESHOLD", "2000000000")
	os.Setenv("MAX_BATCH_SIZE_BYTES", "20971520")
	os.Setenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT", "false")

	rootCmd.SetArgs([]string{
		"import", "data", "file",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on import-data-file config
	assert.Equal(t, int64(1000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 4, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableYBAdaptiveParallelism, "Enable adaptive parallelism should match the config")
	assert.Equal(t, 10, tconf.MaxParallelism, "Adaptive parallelism max should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.EnableUpsert, "Enable upsert for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.UsePublicIP, "Use public IP for importing data should match the config")
	assert.Equal(t, "endpoint1,endpoint2", tconf.TargetEndpoints, "Target endpoints for importing data should match the config")
	assert.Equal(t, "csv", fileFormat, "Format for importing data should match the config")
	assert.Equal(t, ",", delimiter, "Delimiter for importing data should match the config")
	assert.Equal(t, "/path/to/data/dir", dataDir, "Data directory for importing data should match the config")
	assert.Equal(t, "table1,file1.csv;table2,file2.csv", fileTableMapping, "File table map for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), hasHeader, "Has header for importing data should match the config")
	assert.Equal(t, "\\", escapeChar, "Escape character for importing data should match the config")
	assert.Equal(t, "\"", quoteChar, "Quote character for importing data should match the config")
	assert.Equal(t, "option1=value1;option2=value2", fileOpts, "File options for importing data should match the config")
	assert.Equal(t, "\\N", nullString, "Null string for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.DisableTransactionalWrites, "Disable transactional writes for importing data should match the config")
	assert.Equal(t, utils.BoolStr(false), truncateSplits, "Truncate splits for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipReplicationChecks, "Skip replication checks for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipNodeHealthChecks, "Skip node health checks for importing data should match the config")
	assert.Equal(t, utils.BoolStr(true), skipDiskUsageHealthChecks, "Skip disk usage health checks for importing data should match the config")
	assert.Equal(t, "IGNORE", tconf.OnPrimaryKeyConflictAction, "On primary key conflict for importing data should match the config")
	assert.Equal(t, importdata.StashAndContinueErrorPolicy, errorPolicySnapshotFlag, "error-policy should match the config")

	assert.Equal(t, "20971520", os.Getenv("CSV_READER_MAX_BUFFER_SIZE_BYTES"), "CSV reader max buffer size bytes should match the env var")
	assert.Equal(t, "10", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the env var")
	assert.Equal(t, "90", os.Getenv("MAX_CPU_THRESHOLD"), "Max CPU threshold for importing data should match the env var")
	assert.Equal(t, "60", os.Getenv("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS"), "Adaptive parallelism frequency seconds for importing data should match the env var")
	assert.Equal(t, "2000000000", os.Getenv("MIN_AVAILABLE_MEMORY_THRESHOLD"), "Min available memory threshold for importing data should match the env var")
	assert.Equal(t, "20971520", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the env var")
	assert.Equal(t, "false", os.Getenv("YBVOYAGER_USE_TASK_PICKER_FOR_IMPORT"), "YBVoyager use task picker for importing data should match the env var")
}

func TestImportDataFileConfigBinding_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(importDataFileCmd)
	defer resetFlags(importDataFileCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
target:
  db-user: test_user
import-data-file:
  log-level: debug
  data-dir: /path/to/data/dir
  file-table-map: table1,file1.csv;table2,file2.csv
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"import", "data", "file",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}

////////////////////////// Finalize Schema Post Data Import Tests //////////////////////////

func setupFinalizeSchemaPostDataImportContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(finalizeSchemaPostDataImportCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(finalizeSchemaPostDataImportCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
target:
  name: test_target
  db-host: localhost
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
finalize-schema-post-data-import:
  run-guardrails-checks: false
  continue-on-error: true
  ignore-exist: true
  refresh-mviews: true
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestFinalizeSchemaPostDataImportConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupFinalizeSchemaPostDataImportContext(t)

	rootCmd.SetArgs([]string{
		"finalize-schema-post-data-import",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")

	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")

	// Assertions on finalize-schema-post-data-import config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.ContinueOnError, "Continue on error should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.IgnoreIfExists, "Ignore exists should match the config")
	assert.Equal(t, utils.BoolStr(true), flagRefreshMViews, "Refresh materialized views should match the config")
}

func TestFinalizeSchemaPostDataImportConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupFinalizeSchemaPostDataImportContext(t)

	// Create a temporary export directory to test CLI overrides
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"finalize-schema-post-data-import",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--continue-on-error", "false",
		"--ignore-exist", "false",
		"--refresh-mviews", "false",
		"--target-db-host", "localhost2",
		"--target-db-port", "5433",
		"--target-db-user", "test_user2",
		"--target-db-password", "test_password2",
		"--target-db-name", "test_db2",
		"--target-db-schema", "public2",
		"--target-ssl-cert", "/path/to/ssl-cert2",
		"--target-ssl-mode", "verify-full",
		"--target-ssl-key", "/path/to/ssl-key2",
		"--target-ssl-root-cert", "/path/to/ssl-root-cert2",
		"--target-ssl-crl", "/path/to/ssl-crl2",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")

	// Assertions on target config
	assert.Equal(t, "localhost2", tconf.Host, "Target host should be overridden by CLI")
	assert.Equal(t, 5433, tconf.Port, "Target port should be overridden by CLI")
	assert.Equal(t, "test_user2", tconf.User, "Target user should be overridden by CLI")
	assert.Equal(t, "test_password2", tconf.Password, "Target password should be overridden by CLI")
	assert.Equal(t, "test_db2", tconf.DBName, "Target DB name should be overridden by CLI")
	assert.Equal(t, "public2", tconf.Schema, "Target schema should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-cert2", tconf.SSLCertPath, "Target SSL cert should be overridden by CLI")
	assert.Equal(t, "verify-full", tconf.SSLMode, "Target SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-key2", tconf.SSLKey, "Target SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-root-cert2", tconf.SSLRootCert, "Target SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-crl2", tconf.SSLCRL, "Target SSL CRL should be overridden by CLI")

	// Assertions on finalize-schema-post-data-import config
	assert.Equal(t, utils.BoolStr(true), tconf.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.ContinueOnError, "Continue on error should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), tconf.IgnoreIfExists, "Ignore exists should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), flagRefreshMViews, "Refresh materialized views should be overridden by CLI")
}

func TestFinalizeSchemaPostDataImportConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupFinalizeSchemaPostDataImportContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	rootCmd.SetArgs([]string{
		"finalize-schema-post-data-import",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")

	// Assertions on target config
	assert.Equal(t, "localhost", tconf.Host, "Target host should match the config")
	assert.Equal(t, 5432, tconf.Port, "Target port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Target user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Target password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Target DB name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Target schema should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Target SSL cert should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Target SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Target SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Target SSL CRL should match the config")
	// Assertions on finalize-schema-post-data-import config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.ContinueOnError, "Continue on error should match the config")
	assert.Equal(t, utils.BoolStr(true), tconf.IgnoreIfExists, "Ignore exists should match the config")
	assert.Equal(t, utils.BoolStr(true), flagRefreshMViews, "Refresh materialized views should match the config")
}

func TestFinalizeSchemaPostDataImportConfigBinding_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(finalizeSchemaPostDataImportCmd)
	defer resetFlags(finalizeSchemaPostDataImportCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
target:
  db-user: test_user
finalize-schema-post-data-import:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"finalize-schema-post-data-import",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}

///////////////////////////// Export Data From Target Tests ////////////////////////////////

func setupExportDataFromTargetContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(exportDataFromTargetCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(exportDataFromTargetCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
target:
  name: test_target
  db-host: 2.1.1.1
  db-port: 5432
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
export-data-from-target:
  disable-pb: true
  exclude-table-list: table1,table2
  table-list: table3,table4
  exclude-table-list-file-path: /tmp/exclude-tables.txt
  table-list-file-path: /tmp/table-list.txt
  yb-master-port: 2024
  queue-segment-max-bytes: 10485760
  debezium-dist-dir: /tmp/debezium
  transaction-ordering: true
`, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestExportDataFromTargetConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupExportDataFromTargetContext(t)

	rootCmd.SetArgs([]string{
		"export", "data", "from", "target",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "test_password", source.Password, "Target password should match the config")
	assert.Equal(t, "require", source.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", source.SSLRootCert, "Target SSL root cert should match the config")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table1,table2", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table3,table4", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, "2024", os.Getenv("YB_MASTER_PORT"), "YB_MASTER_PORT should match the config")
	assert.Equal(t, "10485760", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should match the config")
	assert.Equal(t, "/tmp/debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should match the config")
	assert.Equal(t, utils.BoolStr(true), transactionOrdering, "Transaction ordering should match the config")
}

func TestExportDataFromTargetConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupExportDataFromTargetContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"export", "data", "from", "target",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--profile", "false",
		"--disable-pb", "false",
		"--exclude-table-list", "table5,table6",
		"--table-list", "table7,table8",
		"--exclude-table-list-file-path", "/tmp/new-exclude-tables.txt",
		"--table-list-file-path", "/tmp/new-table-list.txt",
		"--transaction-ordering", "false",
		"--target-db-password", "test_password2",
		"--target-ssl-mode", "verify-full",
		"--target-ssl-root-cert", "/path/to/root2.pem",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on target config
	assert.Equal(t, "test_password2", source.Password, "Target password should be overridden by CLI")
	assert.Equal(t, "verify-full", source.SSLMode, "Target SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/root2.pem", source.SSLRootCert, "Target SSL root cert should be overridden by CLI")
	// Assertions on export-data config
	assert.Equal(t, utils.BoolStr(false), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table5,table6", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table7,table8", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/new-exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/new-table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, "2024", os.Getenv("YB_MASTER_PORT"), "YB_MASTER_PORT should match the config")
	assert.Equal(t, "10485760", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should match the config")
	assert.Equal(t, "/tmp/debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should match the config")
	assert.Equal(t, utils.BoolStr(false), transactionOrdering, "Transaction ordering should match the config")
}

func TestExportDataFromTargetConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupExportDataFromTargetContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to export-data config
	os.Setenv("QUEUE_SEGMENT_MAX_BYTES", "20971520")
	os.Setenv("DEBEZIUM_DIST_DIR", "/tmp/new-debezium")
	os.Setenv("YB_MASTER_PORT", "2025")

	rootCmd.SetArgs([]string{
		"export", "data", "from", "target",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on target config
	assert.Equal(t, "test_password", source.Password, "Target password should match the config")
	assert.Equal(t, "require", source.SSLMode, "Target SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", source.SSLRootCert, "Target SSL root cert should match the config")
	// // Assertions on export-data config
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "table1,table2", source.ExcludeTableList, "Exclude table list should match the config")
	assert.Equal(t, "table3,table4", source.TableList, "Table list should match the config")
	assert.Equal(t, "/tmp/exclude-tables.txt", excludeTableListFilePath, "Exclude table list file path should match the config")
	assert.Equal(t, "/tmp/table-list.txt", tableListFilePath, "Table list file path should match the config")
	assert.Equal(t, utils.BoolStr(true), transactionOrdering, "Transaction ordering should match the config")
	assert.Equal(t, "20971520", os.Getenv("QUEUE_SEGMENT_MAX_BYTES"), "QUEUE_SEGMENT_MAX_BYTES should be overridden by env var")
	assert.Equal(t, "/tmp/new-debezium", os.Getenv("DEBEZIUM_DIST_DIR"), "DEBEZIUM_DIST_DIR should be overridden by env var")
	assert.Equal(t, "2025", os.Getenv("YB_MASTER_PORT"), "YB_MASTER_PORT should be overridden by env var")
}

func TestExportDataFromTargetConfigBinding_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(exportDataFromTargetCmd)
	defer resetFlags(exportDataFromTargetCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
export-data-from-target:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"export", "data", "from", "target",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}

///////////////////////////// Import Data to Source Tests ////////////////////////////////

func setupImportDataToSourceContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(importDataToSourceCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(importDataToSourceCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
profile: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
import-data-to-source:
  run-guardrails-checks: false
  parallel-jobs: 10
  disable-pb: true
  num-event-channels: 8
  event-channel-size: 10000
  max-events-per-batch: 5000
  max-interval-between-batches: 2
  max-batch-size-bytes: 10485760
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestImportDataToSourceConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupImportDataToSourceContext(t)

	rootCmd.SetArgs([]string{
		"import", "data", "to", "source",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on import-data config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, 10, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "8", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the config")
	assert.Equal(t, "10000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the config")
	assert.Equal(t, "5000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the config")
	assert.Equal(t, "2", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
}

func TestImportDataToSourceConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupImportDataToSourceContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"import", "data", "to", "source",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--profile", "false",
		"--parallel-jobs", "8",
		"--disable-pb", "false",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on import-data config
	assert.Equal(t, utils.BoolStr(true), tconf.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, 8, tconf.Parallelism, "Parallel jobs should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), disablePb, "Disable PB should be overridden by CLI")
	assert.Equal(t, "8", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the config")
	assert.Equal(t, "10000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the config")
	assert.Equal(t, "5000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the config")
	assert.Equal(t, "2", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the config")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the config")
}

func TestImportDataToSourceConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupImportDataToSourceContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to import-data
	os.Setenv("NUM_EVENT_CHANNELS", "16")
	os.Setenv("EVENT_CHANNEL_SIZE", "20000")
	os.Setenv("MAX_EVENTS_PER_BATCH", "10000")
	os.Setenv("MAX_INTERVAL_BETWEEN_BATCHES", "5")
	os.Setenv("MAX_BATCH_SIZE_BYTES", "20971520")

	rootCmd.SetArgs([]string{
		"import", "data", "to", "source",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on import-data config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, 10, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, "16", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data should match the env var")
	assert.Equal(t, "20000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data should match the env var")
	assert.Equal(t, "10000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data should match the env var")
	assert.Equal(t, "5", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data should match the env var")
	assert.Equal(t, "20971520", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data should match the env var")
}

func TestImportDataToSourceConfigBinding_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(importDataToSourceCmd)
	defer resetFlags(importDataToSourceCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
import-data-to-source:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"import", "data", "to", "source",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}

// ///////////////////////// Import Data to Source Replica Tests ////////////////////////////////
func setupImportDataToSourceReplicaContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(importDataToSourceReplicaCmd)
	// override ErrExit to prevent the test from exiting because of some failed validations.
	// We only care about testing whether the configuration and CLI flags are set correctly
	utils.MonkeyPatchUtilsErrExitToIgnore()
	t.Cleanup(func() {
		utils.RestoreUtilsErrExit()
		resetFlags(importDataToSourceReplicaCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
control-plane-type: yugabyte
yugabyted-db-conn-string: postgres://test_user:test_password@localhost:5432/test_db?sslmode=require
java-home: /path/to/java/home
local-call-home-service-host: localhost
local-call-home-service-port: 8080
yb-tserver-port: 9000
tns-admin: /path/to/tns/admin
source:
  name: test_source
  db-host: source_host
  db-port: 1522
  db-user: test_user_source
  db-password: test_password_source
  db-name: test_db_source
  db-schema: public_source
  ssl-cert: /path/to/ssl-cert/source
  ssl-mode: verify-full
  ssl-key: /path/to/ssl-key/source
  ssl-root-cert: /path/to/ssl-root-cert/source
  ssl-crl: /path/to/ssl-crl/source
  oracle-db-sid: test_sid_source
  oracle-home: /path/to/oracle/home/source
  oracle-tns-alias: test_tns_alias_source
source-replica:
  name: test_source_replica
  db-host: 127.0.0.1
  db-port: 1521
  db-user: test_user
  db-password: test_password
  db-name: test_db
  db-schema: public
  ssl-cert: /path/to/ssl-cert
  ssl-mode: require
  ssl-key: /path/to/ssl-key
  ssl-root-cert: /path/to/ssl-root-cert
  ssl-crl: /path/to/ssl-crl
  db-sid: test_sid_source_replica
  oracle-home: /path/to/oracle/home/source-replica
  oracle-tns-alias: test_tns_alias_source_replica
import-data-to-source-replica:
  run-guardrails-checks: false
  batch-size: 10000
  parallel-jobs: 5
  truncate-tables: true
  disable-pb: true
  max-retries: 3
  ybvoyager-max-colocated-batches-in-progress: 2
  num-event-channels: 4
  event-channel-size: 5000
  max-events-per-batch: 2000
  max-interval-between-batches: 1
  max-batch-size-bytes: 5242880
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestImportDataToSourceReplicaConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupImportDataToSourceReplicaContext(t)

	rootCmd.SetArgs([]string{
		"import", "data", "to", "source-replica",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on source-replica config
	assert.Equal(t, "127.0.0.1", tconf.Host, "Source replica host should match the config")
	assert.Equal(t, 1521, tconf.Port, "Source replica port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Source replica user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Source replica password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Source replica db name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Source replica schema should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Source replica SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Source replica SSL cert should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Source replica SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Source replica SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Source replica SSL CRL should match the config")
	assert.Equal(t, "test_sid_source_replica", tconf.DBSid, "Source replica SID should match the config")
	assert.Equal(t, "/path/to/oracle/home/source-replica", tconf.OracleHome, "Source replica Oracle home should match the config")
	assert.Equal(t, "test_tns_alias_source_replica", tconf.TNSAlias, "Source replica Oracle TNS alias should match the config")
	// Assertions on import-data-to-source-replica config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, int64(10000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 5, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries for import should match the config")
	assert.Equal(t, "2", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "4", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data to source replica should match the config")
	assert.Equal(t, "5000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data to source replica should match the config")
	assert.Equal(t, "2000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data to source replica should match the config")
	assert.Equal(t, "1", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data to source replica should match the config")
	assert.Equal(t, "5242880", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data to source replica should match the config")
}

func TestImportDataToSourceReplicaConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupImportDataToSourceReplicaContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	// Test whether CLI overrides config
	rootCmd.SetArgs([]string{
		"import", "data", "to", "source-replica",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "false",
		"--run-guardrails-checks", "true",
		"--batch-size", "20000",
		"--parallel-jobs", "3",
		"--truncate-tables", "false",
		"--disable-pb", "false",
		"--max-retries", "5",
		"--source-replica-db-host", "localhost-replica",
		"--source-replica-db-port", "1522",
		"--source-replica-db-user", "test_user_2",
		"--source-replica-db-password", "test_password_2",
		"--source-replica-db-name", "test_db_2",
		"--source-replica-db-schema", "public_2",
		"--source-replica-ssl-cert", "/path/to/ssl-cert-2",
		"--source-replica-ssl-mode", "verify-full",
		"--source-replica-ssl-key", "/path/to/ssl-key-2",
		"--source-replica-ssl-root-cert", "/path/to/ssl-root-cert-2",
		"--source-replica-ssl-crl", "/path/to/ssl-crl-2",
		"--source-replica-db-sid", "test_sid_source_replica_2",
		"--oracle-home", "/path/to/oracle/home/source-replica-2",
		"--oracle-tns-alias", "test_tns_alias_source_replica_2",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, "yugabyte", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should match the config")
	assert.Equal(t, "postgres://test_user:test_password@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should match the config")
	assert.Equal(t, "/path/to/java/home", os.Getenv("JAVA_HOME"), "Java home should match the config")
	assert.Equal(t, "localhost", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should match the config")
	assert.Equal(t, "8080", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should match the config")
	assert.Equal(t, "9000", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should match the config")
	assert.Equal(t, "/path/to/tns/admin", os.Getenv("TNS_ADMIN"), "TNS admin should match the config")
	// Assertions on source-replica config
	assert.Equal(t, "localhost-replica", tconf.Host, "Source replica host should be overridden by CLI")
	assert.Equal(t, 1522, tconf.Port, "Source replica port should be overridden by CLI")
	assert.Equal(t, "test_user_2", tconf.User, "Source replica user should be overridden by CLI")
	assert.Equal(t, "test_password_2", tconf.Password, "Source replica password should be overridden by CLI")
	assert.Equal(t, "test_db_2", tconf.DBName, "Source replica db name should be overridden by CLI")
	assert.Equal(t, "public_2", tconf.Schema, "Source replica schema should be overridden by CLI")
	assert.Equal(t, "verify-full", tconf.SSLMode, "Source replica SSL mode should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-cert-2", tconf.SSLCertPath, "Source replica SSL cert should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-key-2", tconf.SSLKey, "Source replica SSL key should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-root-cert-2", tconf.SSLRootCert, "Source replica SSL root cert should be overridden by CLI")
	assert.Equal(t, "/path/to/ssl-crl-2", tconf.SSLCRL, "Source replica SSL CRL should be overridden by CLI")
	assert.Equal(t, "test_sid_source_replica_2", tconf.DBSid, "Source replica SID should be overridden by CLI")
	assert.Equal(t, "/path/to/oracle/home/source-replica-2", tconf.OracleHome, "Source replica Oracle home should be overridden by CLI")
	assert.Equal(t, "test_tns_alias_source_replica_2", tconf.TNSAlias, "Source replica Oracle TNS alias should be overridden by CLI")
	// Assertions on import-data-to-source-replica config
	assert.Equal(t, utils.BoolStr(true), tconf.RunGuardrailsChecks, "Run guardrails checks should be overridden by CLI")
	assert.Equal(t, int64(20000), batchSizeInNumRows, "Batch size should be overridden by CLI")
	assert.Equal(t, 3, tconf.Parallelism, "Parallel jobs should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), truncateTables, "Truncate tables should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), disablePb, "Disable PB should be overridden by CLI")
	assert.Equal(t, 5, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries for import should be overridden by CLI")
	assert.Equal(t, "2", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the config")
	assert.Equal(t, "4", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data to source replica should match the config")
	assert.Equal(t, "5000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data to source replica should be overridden by CLI")
	assert.Equal(t, "2000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data to source replica should be overridden by CLI")
	assert.Equal(t, "1", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data to source replica should be overridden by CLI")
	assert.Equal(t, "5242880", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data to source replica should match the config")
}

func TestImportDataToSourceReplicaConfigBinding_EnvOverridesConfig(t *testing.T) {
	ctx := setupImportDataToSourceReplicaContext(t)
	// Test whether env vars overrides config

	// env related to global flags
	os.Setenv("CONTROL_PLANE_TYPE", "yugabyte2")
	os.Setenv("YUGABYTED_DB_CONN_STRING", "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require")
	os.Setenv("JAVA_HOME", "/path/to/java/home2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_HOST", "localhost2")
	os.Setenv("LOCAL_CALL_HOME_SERVICE_PORT", "8081")
	os.Setenv("YB_TSERVER_PORT", "9001")
	os.Setenv("TNS_ADMIN", "/path/to/tns/admin2")

	// env related to import-data-to-source-replica
	os.Setenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS", "3")
	os.Setenv("NUM_EVENT_CHANNELS", "6")
	os.Setenv("EVENT_CHANNEL_SIZE", "6000")
	os.Setenv("MAX_EVENTS_PER_BATCH", "3000")
	os.Setenv("MAX_INTERVAL_BETWEEN_BATCHES", "3")
	os.Setenv("MAX_BATCH_SIZE_BYTES", "10485760")

	rootCmd.SetArgs([]string{
		"import", "data", "to", "source-replica",
		"--config-file", ctx.configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, "yugabyte2", os.Getenv("CONTROL_PLANE_TYPE"), "Control plane type should be overridden by env var")
	assert.Equal(t, "postgres://test_user2:test_password2@localhost:5432/test_db?sslmode=require", os.Getenv("YUGABYTED_DB_CONN_STRING"), "Yugabyted DB connection string should be overridden by env var")
	assert.Equal(t, "/path/to/java/home2", os.Getenv("JAVA_HOME"), "Java home should be overridden by env var")
	assert.Equal(t, "localhost2", os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST"), "Local call home service host should be overridden by env var")
	assert.Equal(t, "8081", os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT"), "Local call home service port should be overridden by env var")
	assert.Equal(t, "9001", os.Getenv("YB_TSERVER_PORT"), "YB TServer port should be overridden by env var")
	assert.Equal(t, "/path/to/tns/admin2", os.Getenv("TNS_ADMIN"), "TNS admin should be overridden by env var")
	// Assertions on source-replica config
	assert.Equal(t, "127.0.0.1", tconf.Host, "Source replica host should match the config")
	assert.Equal(t, 1521, tconf.Port, "Source replica port should match the config")
	assert.Equal(t, "test_user", tconf.User, "Source replica user should match the config")
	assert.Equal(t, "test_password", tconf.Password, "Source replica password should match the config")
	assert.Equal(t, "test_db", tconf.DBName, "Source replica db name should match the config")
	assert.Equal(t, "public", tconf.Schema, "Source replica schema should match the config")
	assert.Equal(t, "require", tconf.SSLMode, "Source replica SSL mode should match the config")
	assert.Equal(t, "/path/to/ssl-cert", tconf.SSLCertPath, "Source replica SSL cert should match the config")
	assert.Equal(t, "/path/to/ssl-key", tconf.SSLKey, "Source replica SSL key should match the config")
	assert.Equal(t, "/path/to/ssl-root-cert", tconf.SSLRootCert, "Source replica SSL root cert should match the config")
	assert.Equal(t, "/path/to/ssl-crl", tconf.SSLCRL, "Source replica SSL CRL should match the config")
	assert.Equal(t, "test_sid_source_replica", tconf.DBSid, "Source replica SID should match the config")
	assert.Equal(t, "/path/to/oracle/home/source-replica", tconf.OracleHome, "Source replica Oracle home should match the config")
	assert.Equal(t, "test_tns_alias_source_replica", tconf.TNSAlias, "Source replica Oracle TNS alias should match the config")
	// Assertions on import-data-to-source-replica config
	assert.Equal(t, utils.BoolStr(false), tconf.RunGuardrailsChecks, "Run guardrails checks should match the config")
	assert.Equal(t, int64(10000), batchSizeInNumRows, "Batch size should match the config")
	assert.Equal(t, 5, tconf.Parallelism, "Parallel jobs should match the config")
	assert.Equal(t, utils.BoolStr(true), truncateTables, "Truncate tables should match the config")
	assert.Equal(t, utils.BoolStr(true), disablePb, "Disable PB should match the config")
	assert.Equal(t, 3, EVENT_BATCH_MAX_RETRY_COUNT, "Max retries for import should match the config")
	assert.Equal(t, "3", os.Getenv("YBVOYAGER_MAX_COLOCATED_BATCHES_IN_PROGRESS"), "YBVoyager max colocated batches in progress should match the env var")
	assert.Equal(t, "6", os.Getenv("NUM_EVENT_CHANNELS"), "Num event channels for importing data to source replica should match the env var")
	assert.Equal(t, "6000", os.Getenv("EVENT_CHANNEL_SIZE"), "Event channel size for importing data to source replica should match the env var")
	assert.Equal(t, "3000", os.Getenv("MAX_EVENTS_PER_BATCH"), "Max events per batch for importing data to source replica should match the env var")
	assert.Equal(t, "3", os.Getenv("MAX_INTERVAL_BETWEEN_BATCHES"), "Max interval between batches for importing data to source replica should match the env var")
	assert.Equal(t, "10485760", os.Getenv("MAX_BATCH_SIZE_BYTES"), "Max batch size bytes for importing data to source replica should match the env var")
}

///////////////////////////// Initiate cutover to target Tests ////////////////////////////////

func setupIntitiateCutoverToTargetContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(cutoverToTargetCmd)
	t.Cleanup(func() {
		resetFlags(cutoverToTargetCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
initiate-cutover-to-target:
  prepare-for-fall-back: true
  use-yb-grpc-connector: false
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestIntitateCutoverToTargetConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupIntitiateCutoverToTargetContext(t)

	rootCmd.SetArgs([]string{
		"initiate", "cutover", "to", "target",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	// Assertions on initiate cutover to target config
	assert.Equal(t, utils.BoolStr(true), prepareForFallBack, "Prepare for fall back should match the config")
	assert.Equal(t, utils.BoolStr(false), useYBgRPCConnector, "Use YB GRPC connector should match the config")
}

func TestInitiateCutoverToTargetConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupIntitiateCutoverToTargetContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	rootCmd.SetArgs([]string{
		"initiate", "cutover", "to", "target",
		"--export-dir", tmpExportDir,
		"--config-file", ctx.configFile,
		"--prepare-for-fall-back", "false",
		"--use-yb-grpc-connector", "true",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	// Assertions on initiate cutover to target config
	assert.Equal(t, utils.BoolStr(false), prepareForFallBack, "Prepare for fall back should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), useYBgRPCConnector, "Use YB GRPC connector should be overridden by CLI")
}

///////////////////////////// Archive Changes Tests ////////////////////////////////

func setupArchiveChangesContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(archiveChangesCmd)
	t.Cleanup(func() {
		resetFlags(archiveChangesCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: false
profile: true
archive-changes:
  delete-changes-without-archiving: true
  fs-utilization-threshold: 20
  move-to: "path/to/dir"
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestArchiveChangesConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupArchiveChangesContext(t)

	rootCmd.SetArgs([]string{
		"archive", "changes",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	// Assertions on initiate cutover to target config
	assert.Equal(t, utils.BoolStr(true), deleteSegments, "Delete Segments should match the config")
	assert.Equal(t, 20, utilizationThreshold, "Utilizations threshold should match the config")
	assert.Equal(t, "path/to/dir", moveDestination, "Move destination should match the config")
}

func TestArchiveChangesConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupArchiveChangesContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	rootCmd.SetArgs([]string{
		"archive", "changes",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "true",
		"--profile", "false",
		"--delete-changes-without-archiving", "false",
		"--fs-utilization-threshold", "40",
		"--move-to", "path/to/dir2",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	// Assertions on initiate cutover to target config
	assert.Equal(t, utils.BoolStr(false), deleteSegments, "Delete Segments should be overridden by CLI")
	assert.Equal(t, 40, utilizationThreshold, "Utilizations threshold should be overridden by CLI")
	assert.Equal(t, "path/to/dir2", moveDestination, "Move destination should be overridden by CLI")
}

func TestArchiveChangesConfigBinding_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(archiveChangesCmd)
	defer resetFlags(archiveChangesCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
archive-changes:
  log-level: debug
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"archive", "changes",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}

///////////////////////////// End Migration Tests ////////////////////////////////

func setupEndMigrationContext(t *testing.T) *testContext {
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	resetCmdAndEnvVars(endMigrationCmd)
	t.Cleanup(func() {
		resetFlags(endMigrationCmd)
	})

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: false
profile: true
end-migration:
  backup-schema-files: false 
  backup-data-files: false
  save-migration-reports: false 
  backup-log-files: false
  backup-dir: path/to/dir
`, tmpExportDir)
	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	return &testContext{
		tmpExportDir: tmpExportDir,
		configFile:   configFile,
	}
}

func TestEndMigrationConfigBinding_ConfigFileBinding(t *testing.T) {
	ctx := setupEndMigrationContext(t)

	rootCmd.SetArgs([]string{
		"end", "migration",
		"--config-file", ctx.configFile,
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, ctx.tmpExportDir, exportDir, "Export directory should match the config")
	assert.Equal(t, "info", config.LogLevel, "Log level should match the config")
	assert.Equal(t, utils.BoolStr(false), callhome.SendDiagnostics, "Send diagnostics should match the config")
	assert.Equal(t, utils.BoolStr(true), perfProfile, "Profile should match the config")
	// Assertions on initiate cutover to target config
	assert.Equal(t, utils.BoolStr(false), backupSchemaFiles, "Backup schema files should match the config")
	assert.Equal(t, utils.BoolStr(false), backupDataFiles, "Backup data files should match the config")
	assert.Equal(t, utils.BoolStr(false), saveMigrationReports, "Save migration reports should match the config")
	assert.Equal(t, utils.BoolStr(false), backupLogFiles, "Backup log files should match the config")
	assert.Equal(t, "path/to/dir", backupDir, "Backup dir should match the config")
}

func TestEndMigrationConfigBinding_CLIOverridesConfig(t *testing.T) {
	ctx := setupEndMigrationContext(t)

	// Creating a new temporary export directory to test the cli override
	tmpExportDir := setupExportDir(t)
	t.Cleanup(func() { os.RemoveAll(tmpExportDir) })

	rootCmd.SetArgs([]string{
		"end", "migration",
		"--config-file", ctx.configFile,
		"--export-dir", tmpExportDir,
		"--log-level", "debug",
		"--send-diagnostics", "true",
		"--profile", "false",
		"--backup-schema-files", "true",
		"--backup-data-files", "true",
		"--save-migration-reports", "true",
		"--backup-log-files", "true",
		"--backup-dir", "path/to/dir2",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should be overridden by CLI")
	assert.Equal(t, "debug", config.LogLevel, "Log level should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(false), perfProfile, "Profile should be overridden by CLI")
	// Assertions on initiate cutover to target config
	assert.Equal(t, utils.BoolStr(true), backupSchemaFiles, "Backup schema files should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), backupDataFiles, "Backup data files should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), saveMigrationReports, "Save migration reports should be overridden by CLI")
	assert.Equal(t, utils.BoolStr(true), backupLogFiles, "Backup log files should be overridden by CLI")
	assert.Equal(t, "path/to/dir2", backupDir, "Backup dir should be overridden by CLI")
}

func TestEndMigrationConfigBinding_GlobalVsLocalConfig(t *testing.T) {
	tmpExportDir := setupExportDir(t)
	defer os.RemoveAll(tmpExportDir)

	resetCmdAndEnvVars(endMigrationCmd)
	defer resetFlags(endMigrationCmd)

	configContent := fmt.Sprintf(`
export-dir: %s
log-level: info
send-diagnostics: true
end-migration:
  log-level: debug
  backup-schema-files: false
  backup-data-files: false
  save-migration-reports: false
  backup-log-files: false
  `, tmpExportDir)

	configFile, configDir := setupConfigFile(t, configContent)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	rootCmd.SetArgs([]string{
		"end", "migration",
		"--config-file", configFile,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Assertions on global flags
	assert.Equal(t, tmpExportDir, exportDir, "Export directory should match the global config section")
	assert.Equal(t, "debug", config.LogLevel, "Log level should match the command level config section")
	assert.Equal(t, utils.BoolStr(true), callhome.SendDiagnostics, "Send diagnostics should match the global config section")
}
