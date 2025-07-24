package mcpservernew

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigValidator tests the config validation functionality
func TestConfigValidator(t *testing.T) {
	cv := NewConfigValidator()

	// Test with empty config path
	err := cv.ValidateConfig("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config path is required")
	fmt.Printf("Empty path error: %v\n", err)

	// Test with non-existent file
	err = cv.ValidateConfig("/path/to/nonexistent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file does not exist")
	fmt.Printf("Non-existent file error: %v\n", err)

	// Test with existing example config
	err = cv.ValidateConfig("example-config.yaml")
	assert.NoError(t, err)
	fmt.Printf("Valid config validation: SUCCESS\n")

	// Test GetConfigInfo with valid config
	configInfo, err := cv.GetConfigInfo("example-config.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, configInfo)

	// Check that path is absolute
	expectedPath, _ := filepath.Abs("example-config.yaml")
	assert.Equal(t, expectedPath, configInfo["path"])
	assert.True(t, configInfo["is_readable"].(bool))
	assert.False(t, configInfo["is_directory"].(bool))
	assert.Greater(t, configInfo["total_keys"].(int), 0)
	fmt.Printf("Config info: %+v\n", configInfo)
}

// TestConfigValidator_FileValidation tests file-level validation
func TestConfigValidator_FileValidation(t *testing.T) {
	cv := NewConfigValidator()

	// Test with empty file
	emptyFile := createTempFile(t, "")
	defer os.Remove(emptyFile)

	err := cv.ValidateConfig(emptyFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file is empty")
	fmt.Printf("Empty file error: %v\n", err)

	// Test with non-yaml file
	nonYamlFile := createTempFile(t, "this is not yaml content")
	defer os.Remove(nonYamlFile)

	err = cv.ValidateConfig(nonYamlFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
	fmt.Printf("Non-YAML file error: %v\n", err)

	// Test with invalid YAML
	invalidYamlFile := createTempFile(t, `
source:
  db-host: localhost
  invalid-key: value  # This should cause validation error
`)
	defer os.Remove(invalidYamlFile)

	err = cv.ValidateConfig(invalidYamlFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
	fmt.Printf("Invalid YAML error: %v\n", err)
}

// TestConfigValidator_GetConfigInfo tests the GetConfigInfo functionality
func TestConfigValidator_GetConfigInfo(t *testing.T) {
	cv := NewConfigValidator()

	// Test with valid config
	configInfo, err := cv.GetConfigInfo("example-config.yaml")
	require.NoError(t, err)

	// Validate all expected fields
	assert.NotEmpty(t, configInfo["path"])
	assert.IsType(t, int64(0), configInfo["size_bytes"])
	assert.True(t, configInfo["is_readable"].(bool))
	assert.False(t, configInfo["is_directory"].(bool))
	assert.NotEmpty(t, configInfo["mod_time"])
	assert.IsType(t, []string{}, configInfo["config_sections"])
	assert.IsType(t, []string{}, configInfo["config_keys"])
	assert.IsType(t, 0, configInfo["total_keys"])

	// Validate specific content for example-config.yaml
	sections := configInfo["config_sections"].([]string)
	assert.Contains(t, sections, "source")
	assert.Contains(t, sections, "target")

	keys := configInfo["config_keys"].([]string)
	assert.Greater(t, len(keys), 0)

	// Check that we have expected keys
	hasSourceKeys := false
	hasTargetKeys := false
	for _, key := range keys {
		if len(key) > 6 && key[:6] == "source" {
			hasSourceKeys = true
		}
		if len(key) > 6 && key[:6] == "target" {
			hasTargetKeys = true
		}
	}
	assert.True(t, hasSourceKeys, "Should have source keys")
	assert.True(t, hasTargetKeys, "Should have target keys")

	// Test with invalid config (should fail validation)
	invalidFile := createTempFile(t, `
source:
  invalid-key: value
`)
	defer os.Remove(invalidFile)

	configInfo, err = cv.GetConfigInfo(invalidFile)
	assert.Error(t, err)
	assert.Nil(t, configInfo)
}

// TestConfigValidator_ContentValidation tests content validation
func TestConfigValidator_ContentValidation(t *testing.T) {
	cv := NewConfigValidator()

	// Test with valid YAML but invalid config keys
	invalidKeysFile := createTempFile(t, `
source:
  db-host: localhost
  db-port: 5432
  invalid-key: value
target:
  db-host: localhost
  db-port: 5433
  another-invalid-key: value
`)
	defer os.Remove(invalidKeysFile)

	err := cv.ValidateConfig(invalidKeysFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
	assert.Contains(t, err.Error(), "Invalid keys in section")
	fmt.Printf("Invalid keys error: %v\n", err)

	// Test with conflicting sections
	conflictingFile := createTempFile(t, `
export-data:
  table-list: a,b
export-data-from-source:
  table-list: x,y
`)
	defer os.Remove(conflictingFile)

	err = cv.ValidateConfig(conflictingFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
	assert.Contains(t, err.Error(), "Only one of the following sections can be used")
	fmt.Printf("Conflicting sections error: %v\n", err)

	// Test with valid config
	validFile := createTempFile(t, `
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-name: testdb
  db-user: postgres
target:
  db-host: localhost
  db-port: 5433
  db-name: testdb
  db-user: yugabyte
export-dir: /tmp/migration-test
`)
	defer os.Remove(validFile)

	err = cv.ValidateConfig(validFile)
	assert.NoError(t, err)
	fmt.Printf("Valid config validation: SUCCESS\n")
}

// Helper function to create temporary files for testing
func createTempFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	return tmpFile.Name()
}
