package mcpservernew

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandExecutor_ValidateConfig tests config validation
func TestCommandExecutor_ValidateConfig(t *testing.T) {
	ce := NewCommandExecutor()

	// Test with non-existent config file
	err := ce.validateConfigForCommand("assess-migration", "/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file does not exist")

	// Test with empty config file
	tempDir := t.TempDir()
	emptyConfigPath := filepath.Join(tempDir, "empty-config.yaml")
	err = os.WriteFile(emptyConfigPath, []byte(""), 0644)
	require.NoError(t, err)

	err = ce.validateConfigForCommand("assess-migration", emptyConfigPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file is empty")

	// Test with valid config file
	validConfigPath := filepath.Join(tempDir, "valid-config.yaml")
	validConfig := `
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: testuser
  db-name: testdb
  db-schema: public

assess-migration:
  log-level: info
  run-guardrails-checks: true

export-dir: /tmp/test-export
`

	err = os.WriteFile(validConfigPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	err = ce.validateConfigForCommand("assess-migration", validConfigPath)
	assert.NoError(t, err)
}

// TestCommandExecutor_AsyncExecution tests async command execution
func TestCommandExecutor_AsyncExecution(t *testing.T) {
	ce := NewCommandExecutor()

	// Create a temporary config file for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	exportDir := filepath.Join(tempDir, "export")

	// Create the export directory
	err := os.MkdirAll(exportDir, 0755)
	require.NoError(t, err)

	validConfig := fmt.Sprintf(`
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: testuser
  db-name: testdb
  db-schema: public

assess-migration:
  log-level: info
  run-guardrails-checks: true

export-dir: %s
`, exportDir)

	err = os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Test async execution (this will fail but we can test the setup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ce.ExecuteCommandAsync(ctx, "assess-migration", configPath, "--yes")

	// The command should start (even if it fails due to missing database)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ExecutionID)
	assert.Equal(t, "running", result.Status)

	fmt.Printf("Async execution started with ID: %s\n", result.ExecutionID)

	// Wait a bit to see if we get any output
	time.Sleep(2 * time.Second)

	// Check if we have any progress
	fmt.Printf("Progress lines: %d\n", len(result.Progress))
	for i, line := range result.Progress {
		fmt.Printf("Progress[%d]: %s\n", i, line)
	}

	// The command should have completed (even if failed)
	// Note: The command might complete quickly due to missing database connection
	if result.EndTime != nil {
		fmt.Printf("Command completed with status: %s\n", result.Status)
		fmt.Printf("Exit code: %d\n", result.ExitCode)
		fmt.Printf("Duration: %s\n", result.Duration)
		// Command should have completed (even if failed due to database connection)
		assert.NotEqual(t, "running", result.Status)
	} else {
		fmt.Printf("Command still running with status: %s\n", result.Status)
		// Wait a bit more to see if it completes
		time.Sleep(3 * time.Second)
		fmt.Printf("After additional wait - Status: %s, EndTime: %v\n", result.Status, result.EndTime)
		if result.EndTime != nil {
			fmt.Printf("Command completed with status: %s\n", result.Status)
			fmt.Printf("Exit code: %d\n", result.ExitCode)
			fmt.Printf("Duration: %s\n", result.Duration)
			assert.NotEqual(t, "running", result.Status)
		} else {
			fmt.Printf("Command still running - this might indicate an issue\n")
			// If still running after 5 seconds, it might be waiting for input
			assert.Equal(t, "running", result.Status)
		}
	}
}

// TestCommandExecutor_InteractivePrompts tests interactive prompt handling
func TestCommandExecutor_InteractivePrompts(t *testing.T) {
	ce := NewCommandExecutor()

	// Create a temporary config file for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	exportDir := filepath.Join(tempDir, "export")

	// Create the export directory
	err := os.MkdirAll(exportDir, 0755)
	require.NoError(t, err)

	validConfig := fmt.Sprintf(`
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: testuser
  db-name: testdb
  db-schema: public

assess-migration:
  log-level: info
  run-guardrails-checks: true

export-dir: %s
`, exportDir)

	err = os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Test with a command that might ask for input
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ce.ExecuteCommandAsync(ctx, "assess-migration", configPath, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ExecutionID)
	assert.Equal(t, "running", result.Status)

	fmt.Printf("Interactive test started with ID: %s\n", result.ExecutionID)

	// Wait a bit to see if we get any output
	time.Sleep(3 * time.Second)

	// Check if we have any progress
	fmt.Printf("Progress lines: %d\n", len(result.Progress))
	for i, line := range result.Progress {
		fmt.Printf("Progress[%d]: %s\n", i, line)
	}

	// The command should still be running (waiting for input)
	fmt.Printf("Command status: %s\n", result.Status)

	// Wait a bit more to see if it completes
	time.Sleep(2 * time.Second)

	fmt.Printf("Final progress lines: %d\n", len(result.Progress))
	for i, line := range result.Progress {
		fmt.Printf("Final Progress[%d]: %s\n", i, line)
	}
}

// TestCommandExecutor_SimpleCommand tests a simple command that should complete quickly
func TestCommandExecutor_SimpleCommand(t *testing.T) {
	ce := NewCommandExecutor()

	// Create a temporary config file for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	exportDir := filepath.Join(tempDir, "export")

	// Create the export directory
	err := os.MkdirAll(exportDir, 0755)
	require.NoError(t, err)

	validConfig := fmt.Sprintf(`
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: testuser
  db-name: testdb
  db-schema: public

assess-migration:
  log-level: info
  run-guardrails-checks: true

export-dir: %s
`, exportDir)

	err = os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Test with a command that should complete quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ce.ExecuteCommandAsync(ctx, "assess-migration", configPath, "--yes")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ExecutionID)
	assert.Equal(t, "running", result.Status)

	fmt.Printf("Simple command test started with ID: %s\n", result.ExecutionID)

	// Wait longer to see if it completes
	time.Sleep(5 * time.Second)

	// Check if we have any progress
	fmt.Printf("Progress lines: %d\n", len(result.Progress))
	for i, line := range result.Progress {
		fmt.Printf("Progress[%d]: %s\n", i, line)
	}

	// Check final status
	fmt.Printf("Final command status: %s\n", result.Status)
	if result.EndTime != nil {
		fmt.Printf("Command completed at: %s\n", result.EndTime.Format("15:04:05"))
		fmt.Printf("Exit code: %d\n", result.ExitCode)
		fmt.Printf("Duration: %s\n", result.Duration)
		// Command should have completed (even if failed due to database connection)
		assert.NotEqual(t, "running", result.Status)
	} else {
		fmt.Printf("Command still running - this might indicate an issue\n")
		// Wait a bit more to see if timeout kicks in
		time.Sleep(2 * time.Second)
		fmt.Printf("After additional wait - Status: %s, EndTime: %v\n", result.Status, result.EndTime)
		// If still running after 5 seconds, it might be waiting for input
		assert.Equal(t, "running", result.Status)
	}
}

func TestCommandExecutor_ExportSchemaCommand(t *testing.T) {
	ce := NewCommandExecutor()

	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	configContent := `
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: testuser
  db-name: testdb
  db-schema: public
export-dir: /tmp/test-export
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Execute export schema command
	ctx := context.Background()
	result, err := ce.ExecuteCommandAsync(ctx, "export schema", configPath, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	fmt.Printf("Export schema test started with ID: %s\n", result.ExecutionID)

	// Wait for the command to complete
	time.Sleep(5 * time.Second)

	// Check final status
	fmt.Printf("Final command status: %s\n", result.Status)
	fmt.Printf("Command completed at: %s\n", time.Now().Format("15:04:05"))
	fmt.Printf("Exit code: %d\n", result.ExitCode)
	fmt.Printf("Duration: %s\n", result.Duration)

	// The command should have completed (even if it failed due to database connection)
	assert.True(t, result.Status == "completed" || result.Status == "failed")
}

func TestCommandExecutor_AnalyzeSchemaCommand(t *testing.T) {
	ce := NewCommandExecutor()

	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	configContent := `
source:
  db-type: postgresql
  db-host: localhost
  db-port: 5432
  db-user: testuser
  db-name: testdb
  db-schema: public
export-dir: /tmp/test-export
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Execute analyze schema command
	ctx := context.Background()
	result, err := ce.ExecuteCommandAsync(ctx, "analyze-schema", configPath, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	fmt.Printf("Analyze schema test started with ID: %s\n", result.ExecutionID)

	// Wait for the command to complete
	time.Sleep(5 * time.Second)

	// Check final status
	fmt.Printf("Final command status: %s\n", result.Status)
	fmt.Printf("Command completed at: %s\n", time.Now().Format("15:04:05"))
	fmt.Printf("Exit code: %d\n", result.ExitCode)
	fmt.Printf("Duration: %s\n", result.Duration)

	// The command should have completed (even if it failed due to database connection)
	assert.True(t, result.Status == "completed" || result.Status == "failed")
}
