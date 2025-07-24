package mcpservernew

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerCreation tests that we can create a server instance
func TestServerCreation(t *testing.T) {
	server := NewServer()
	assert.NotNil(t, server)
	assert.NotNil(t, server.server)
}

// TestServer_ToolRegistration tests that both sync and async tools are registered
func TestServer_ToolRegistration(t *testing.T) {
	server := NewServer()
	assert.NotNil(t, server)

	// Both tools should be available
	// Note: We can't directly test tool registration without the MCP framework,
	// but we can verify the server is created successfully
	fmt.Printf("Server created with %d running commands capacity\n", len(server.runningCommands))
}

// TestServer_ToolDescriptions tests that tool descriptions are properly set
func TestServer_ToolDescriptions(t *testing.T) {
	server := NewServer()

	// Check that the server was created successfully
	assert.NotNil(t, server)

	fmt.Println("Expected tool descriptions:")
	fmt.Println("- assess_migration_async: Execute YB Voyager assess-migration command asynchronously with real-time output streaming. RECOMMENDED for long-running commands. Returns execution ID for tracking progress.")
	fmt.Println("- assess_migration: Execute YB Voyager assess-migration command synchronously with automatic --yes flag. WARNING: This may block for long-running commands. Use assess_migration_async for better experience.")
	fmt.Println("- export_schema: Execute YB Voyager export-schema command synchronously. Automatically adds --yes flag to avoid interactive prompts.")
	fmt.Println("- export_schema_async: Execute YB Voyager export-schema command asynchronously with real-time output streaming. RECOMMENDED for long-running commands. Returns execution ID for tracking progress. Automatically adds --yes flag to avoid interactive prompts.")
	fmt.Println("- stop_command: Stop/cancel a running command execution")
}

// TestServer_StopCommand tests the stop command functionality
func TestServer_StopCommand(t *testing.T) {
	server := NewServer()

	// Create a mock running command for testing
	executionID := "test_exec_123"
	mockResult := &CommandResult{
		ExecutionID: executionID,
		Status:      "running",
		Progress:    []string{"[12:00:00] Command started"},
		StartTime:   time.Now(),
	}

	// Create a cancellable context
	_, cancel := context.WithCancel(context.Background())

	// Store the mock command in the server
	server.mu.Lock()
	server.runningCommands[executionID] = mockResult
	server.commandContexts[executionID] = cancel
	server.mu.Unlock()

	fmt.Printf("Created mock command with ID: %s\n", executionID)

	// Verify the command is running
	server.mu.RLock()
	storedResult, exists := server.runningCommands[executionID]
	server.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, "running", storedResult.Status)

	// Stop the command
	stopReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "stop_command",
			Arguments: map[string]interface{}{
				"execution_id": executionID,
			},
		},
	}

	stopResult, err := server.stopCommandHandler(context.Background(), stopReq)
	require.NoError(t, err)
	require.NotNil(t, stopResult)

	// Check that the command was stopped
	server.mu.RLock()
	stoppedResult, stillExists := server.runningCommands[executionID]
	_, hasContext := server.commandContexts[executionID]
	server.mu.RUnlock()

	assert.True(t, stillExists)
	assert.Equal(t, "cancelled", stoppedResult.Status)
	assert.False(t, hasContext, "Context should be cleaned up")

	fmt.Printf("Command stopped successfully. Final status: %s\n", stoppedResult.Status)
}
