package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// Tool handlers

// executeVoyagerCommand executes yb-voyager commands as a subprocess
func executeVoyagerCommand(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'command': %v", err)), nil
	}

	exportDir := req.GetString("export_dir", "")
	args := req.GetString("args", "")

	// Build the command arguments
	cmdArgs := []string{command}

	// Add export-dir if provided
	if exportDir != "" {
		cmdArgs = append(cmdArgs, "--export-dir", exportDir)
	}

	// Parse additional arguments if provided
	if args != "" {
		var additionalArgs []string
		if err := json.Unmarshal([]byte(args), &additionalArgs); err != nil {
			// If not JSON, treat as space-separated string
			additionalArgs = strings.Fields(args)
		}
		cmdArgs = append(cmdArgs, additionalArgs...)
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, "yb-voyager", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		textContent := mcp.NewTextContent(fmt.Sprintf("Command failed: %v\nOutput: %s", err, string(output)))
		return &mcp.CallToolResult{
			Content: []mcp.Content{textContent},
			IsError: true,
		}, nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

// queryMigrationStatus queries the current migration status from metaDB
func queryMigrationStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exportDir, err := req.RequireString("export_dir")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'export_dir': %v", err)), nil
	}

	// Initialize metaDB
	metaDB, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to initialize metaDB: %v", err)), nil
	}

	// Get migration status
	status, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get migration status: %v", err)), nil
	}

	// Convert to JSON for response
	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal status: %v", err)), nil
	}

	return mcp.NewToolResultText(string(statusJSON)), nil
}

// getMetaDBStats gets statistics from metaDB
func getMetaDBStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exportDir, err := req.RequireString("export_dir")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'export_dir': %v", err)), nil
	}

	// Initialize metaDB
	metaDB, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to initialize metaDB: %v", err)), nil
	}

	// Collect various statistics
	stats := make(map[string]interface{})

	// Get total exported events (using empty string as runId to get all)
	if totalEvents, totalEventsRun, err := metaDB.GetTotalExportedEvents(""); err == nil {
		stats["total_exported_events"] = totalEvents
		stats["total_exported_events_current_run"] = totalEventsRun
	}

	// Get exported events stats for table (example with empty schema/table)
	if eventStats, err := metaDB.GetExportedEventsStatsForTable("", ""); err == nil {
		stats["event_stats_sample"] = eventStats
	}

	// Convert to JSON for response
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal stats: %v", err)), nil
	}

	return mcp.NewToolResultText(string(statsJSON)), nil
}

// Resource handlers

// readMigrationStatusResource reads migration status as a resource
func readMigrationStatusResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// This is a static resource, so we can use any export directory from the request
	// In a real implementation, you might want to make this configurable
	exportDir := "/tmp" // Default fallback

	// Try to extract export_dir from URI parameters if available
	if req.Params.URI == "voyager://migration-status" {
		// For now, use a default or get from environment
		if envDir := os.Getenv("VOYAGER_EXPORT_DIR"); envDir != "" {
			exportDir = envDir
		}
	}

	metaDB, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metaDB: %w", err)
	}

	status, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("failed to get migration status: %w", err)
	}

	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal status: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(statusJSON),
		},
	}, nil
}

// readLogsResource reads recent migration logs
func readLogsResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract export_dir from URI like "voyager://logs/{export_dir}/recent"
	uri := req.Params.URI
	parts := strings.Split(uri, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid logs URI format: %s", uri)
	}

	exportDir := parts[3] // Extract {export_dir} from URI

	// Look for log files in the export directory
	logFiles := []string{
		filepath.Join(exportDir, "logs", "yb-voyager.log"),
		filepath.Join(exportDir, "yb-voyager.log"),
		"yb-voyager.log",
	}

	var logContent strings.Builder
	logContent.WriteString("Recent YB Voyager Logs:\n\n")

	for _, logFile := range logFiles {
		if content, err := readFileContents(logFile); err == nil {
			logContent.WriteString(fmt.Sprintf("=== %s ===\n", logFile))
			// Get last 50 lines for "recent" logs
			lines := strings.Split(content, "\n")
			start := len(lines) - 50
			if start < 0 {
				start = 0
			}
			logContent.WriteString(strings.Join(lines[start:], "\n"))
			logContent.WriteString("\n\n")
			break // Use first available log file
		}
	}

	if logContent.Len() == 0 {
		logContent.WriteString("No log files found in the specified export directory.")
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     logContent.String(),
		},
	}, nil
}

// readSchemaAnalysisResource reads schema analysis results
func readSchemaAnalysisResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract export_dir from URI like "voyager://schema-analysis/{export_dir}"
	uri := req.Params.URI
	parts := strings.Split(uri, "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid schema analysis URI format: %s", uri)
	}

	exportDir := parts[2] // Extract {export_dir} from URI

	// Look for schema analysis files
	analysisFiles := []string{
		filepath.Join(exportDir, "assessment", "assessment.json"),
		filepath.Join(exportDir, "schema", "schema.sql"),
		filepath.Join(exportDir, "reports", "report.json"),
	}

	analysis := make(map[string]interface{})
	analysis["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	analysis["export_dir"] = exportDir

	for _, file := range analysisFiles {
		if content, err := readFileContents(file); err == nil {
			filename := filepath.Base(file)
			analysis[filename] = content
		}
	}

	if len(analysis) == 2 { // Only timestamp and export_dir
		analysis["error"] = "No schema analysis files found"
	}

	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(analysisJSON),
		},
	}, nil
}

// Prompt handlers

// getTroubleshootPrompt provides troubleshooting assistance
func getTroubleshootPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	exportDir := req.Params.Arguments["export_dir"]
	errorContext := req.Params.Arguments["error_context"]

	var promptText strings.Builder
	promptText.WriteString("I need help troubleshooting a YB Voyager migration issue.\n\n")

	if exportDir != "" {
		promptText.WriteString(fmt.Sprintf("Export Directory: %s\n", exportDir))

		// Try to get current migration status
		metaDB, err := metadb.NewMetaDB(exportDir)
		if err == nil {
			if status, err := metaDB.GetMigrationStatusRecord(); err == nil {
				statusJSON, _ := json.MarshalIndent(status, "", "  ")
				promptText.WriteString(fmt.Sprintf("Current Migration Status:\n```json\n%s\n```\n\n", statusJSON))
			}
		}
	}

	if errorContext != "" {
		promptText.WriteString(fmt.Sprintf("Error Context: %s\n\n", errorContext))
	}

	promptText.WriteString("Please analyze the migration status and error context to provide specific troubleshooting steps and recommendations.")

	return mcp.NewGetPromptResult(
		"YB Voyager Migration Troubleshooting",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(promptText.String()),
			),
		},
	), nil
}

// getOptimizePrompt provides performance optimization suggestions
func getOptimizePrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	exportDir := req.Params.Arguments["export_dir"]
	migrationPhase := req.Params.Arguments["migration_phase"]

	var promptText strings.Builder
	promptText.WriteString("I need suggestions for optimizing YB Voyager migration performance.\n\n")

	if exportDir != "" {
		promptText.WriteString(fmt.Sprintf("Export Directory: %s\n", exportDir))

		// Try to get migration statistics
		metaDB, err := metadb.NewMetaDB(exportDir)
		if err == nil {
			stats := make(map[string]interface{})

			if totalEvents, totalEventsRun, err := metaDB.GetTotalExportedEvents(""); err == nil {
				stats["total_exported_events"] = totalEvents
				stats["total_exported_events_current_run"] = totalEventsRun
			}

			if len(stats) > 0 {
				statsJSON, _ := json.MarshalIndent(stats, "", "  ")
				promptText.WriteString(fmt.Sprintf("Migration Statistics:\n```json\n%s\n```\n\n", statsJSON))
			}
		}
	}

	if migrationPhase != "" {
		promptText.WriteString(fmt.Sprintf("Current Migration Phase: %s\n\n", migrationPhase))
	}

	promptText.WriteString("Based on the migration statistics and current phase, please provide specific performance optimization recommendations including:\n")
	promptText.WriteString("1. Parallelism settings\n")
	promptText.WriteString("2. Batch size optimizations\n")
	promptText.WriteString("3. Resource allocation suggestions\n")
	promptText.WriteString("4. Network and I/O optimizations\n")
	promptText.WriteString("5. Database-specific tuning recommendations")

	return mcp.NewGetPromptResult(
		"YB Voyager Performance Optimization",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(promptText.String()),
			),
		},
	), nil
}
