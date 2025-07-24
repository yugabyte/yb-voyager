package mcpservernew

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAssessMigrationSync_AutoYesFlag tests that the synchronous assess_migration automatically adds --yes flag
func TestAssessMigrationSync_AutoYesFlag(t *testing.T) {
	server := NewServer()

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

	// Create a mock request with additional args
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "assess_migration",
			Arguments: map[string]interface{}{
				"config_path":     configPath,
				"additional_args": "--log-level debug",
			},
		},
	}

	// Call the handler
	result, err := server.assessMigrationHandler(context.Background(), req)

	// The handler should not return an error immediately (it will fail later due to database connection)
	// But we can verify that the --yes flag was automatically added by checking the response message
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check that the response mentions the --yes flag
	content := result.Content[0]
	if textContent, ok := content.(mcp.TextContent); ok {
		assert.Contains(t, textContent.Text, "--yes flag")
		assert.Contains(t, textContent.Text, "synchronously with --yes flag")
	}

	fmt.Println("Synchronous assess_migration automatically adds --yes flag as expected")
}
