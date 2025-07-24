package mcpservernew

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// Server represents the MCP server
type Server struct {
	server          *server.MCPServer
	runningCommands map[string]*CommandResult
	mu              sync.RWMutex
	commandExecutor *CommandExecutor
	configValidator *ConfigValidator
	sqlParser       *SQLParser
	configSchema    *ConfigSchema
	// Add context map for command cancellation
	commandContexts map[string]context.CancelFunc
}

// NewServer creates a new MCP server instance
func NewServer() *Server {
	mcpServer := server.NewMCPServer(
		"migration-mcp-server",
		"1.0.0",
		server.WithLogging(),
	)

	s := &Server{
		server:          mcpServer,
		configValidator: NewConfigValidator(),
		sqlParser:       NewSQLParser(),
		configSchema:    NewConfigSchema(),
		commandExecutor: NewCommandExecutor(),
		runningCommands: make(map[string]*CommandResult),
		commandContexts: make(map[string]context.CancelFunc),
	}

	s.registerTools()
	return s
}

// Run starts the MCP server with stdio transport
func (s *Server) Run() error {
	log.Info("Starting MCP server...")
	return server.ServeStdio(s.server)
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// Config validation tool
	s.server.AddTool(
		mcp.NewTool("validate_config",
			mcp.WithDescription("Validate YB Voyager configuration files for existence, readability, YAML format, and content compliance with YB Voyager schema rules"),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file to validate")),
		),
		s.validateConfigHandler,
	)

	// SQL parsing tool
	s.server.AddTool(
		mcp.NewTool("parse_sql",
			mcp.WithDescription("Parse and validate a SQL statement using PostgreSQL parser"),
			mcp.WithString("sql_statement", mcp.Required(), mcp.Description("SQL statement to parse and validate")),
		),
		s.parseSQLHandler,
	)

	// Config schema tool
	s.server.AddTool(
		mcp.NewTool("get_config_schema",
			mcp.WithDescription("Get available configuration keys and schema information for YB Voyager config sections. If no section is provided, returns all available sections."),
			mcp.WithString("section", mcp.Description("Configuration section name (e.g., source, target, export-data). If not provided, returns all available sections.")),
		),
		s.getConfigSchemaHandler,
	)

	// Assess migration tool (synchronous - may block for long-running commands)
	// TODO: Add this back in when we have a way to handle interactive prompts
	// s.server.AddTool(
	// 	mcp.NewTool("assess_migration",
	// 		mcp.WithDescription("Execute YB Voyager assess-migration command synchronously with automatic --yes flag. WARNING: This may block for long-running commands. Use assess_migration_async for better experience."),
	// 		mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and assess-migration sections")),
	// 		mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional, --yes is automatically added)")),
	// 	),
	// 	s.assessMigrationHandler,
	// )

	// Assess migration async tool (RECOMMENDED for long-running commands)
	s.server.AddTool(
		mcp.NewTool("assess_migration_async",
			mcp.WithDescription("Execute YB Voyager assess-migration command asynchronously with real-time output streaming. RECOMMENDED for long-running commands. Returns execution ID for tracking progress. Automatically adds --yes flag to avoid interactive prompts."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and assess-migration sections")),
			mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional)")),
		),
		s.assessMigrationAsyncHandler,
	)

	// Export schema tool (synchronous)
	// TODO: Add this back in when we have a way to handle interactive prompts
	// s.server.AddTool(
	// 	mcp.NewTool("export_schema",
	// 		mcp.WithDescription("Execute YB Voyager export schema command synchronously. Automatically adds --yes flag to avoid interactive prompts."),
	// 		mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and export schema sections")),
	// 		mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional, --yes is automatically added)")),
	// 	),
	// 	s.exportSchemaHandler,
	// )

	// Export schema async tool (RECOMMENDED for long-running commands)
	s.server.AddTool(
		mcp.NewTool("export_schema_async",
			mcp.WithDescription("Execute YB Voyager export schema command asynchronously with real-time output streaming. RECOMMENDED for long-running commands. Returns execution ID for tracking progress. Automatically adds --yes flag to avoid interactive prompts."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and export schema sections")),
			mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional)")),
		),
		s.exportSchemaAsyncHandler,
	)

	// TODO: Add this back in when we have a way to handle interactive prompts
	// Analyze schema tool (synchronous)
	// s.server.AddTool(
	// 	mcp.NewTool("analyze_schema",
	// 		mcp.WithDescription("Execute YB Voyager analyze-schema command synchronously. Automatically adds --yes flag to avoid interactive prompts."),
	// 		mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and analyze-schema sections")),
	// 		mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional, --yes is automatically added)")),
	// 	),
	// 	s.analyzeSchemaHandler,
	// )

	// Analyze schema async tool (RECOMMENDED for long-running commands)
	s.server.AddTool(
		mcp.NewTool("analyze_schema_async",
			mcp.WithDescription("Execute YB Voyager analyze-schema command asynchronously with real-time output streaming. RECOMMENDED for long-running commands. Returns execution ID for tracking progress. Automatically adds --yes flag to avoid interactive prompts."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and analyze-schema sections")),
			mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional)")),
		),
		s.analyzeSchemaAsyncHandler,
	)

	// Get assessment report tool
	s.server.AddTool(
		mcp.NewTool("get_assessment_report",
			mcp.WithDescription("Get the complete YB Voyager migration assessment report as structured JSON. This includes all issues, sizing recommendations, schema summary, and performance statistics."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing export-dir information")),
		),
		s.getAssessmentReportHandler,
	)

	// Get schema analysis report tool
	s.server.AddTool(
		mcp.NewTool("get_schema_analysis_report",
			mcp.WithDescription("Get the complete YB Voyager schema analysis report as structured JSON. This includes all compatibility issues, object summaries, and migration readiness information."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing export-dir information")),
		),
		s.getSchemaAnalysisReportHandler,
	)

	// Get command status tool
	s.server.AddTool(
		mcp.NewTool("get_command_status",
			mcp.WithDescription("Get the status and progress of a running command execution"),
			mcp.WithString("execution_id", mcp.Required(), mcp.Description("Execution ID returned by async command")),
		),
		s.getCommandStatusHandler,
	)

	// Stop command tool
	s.server.AddTool(
		mcp.NewTool("stop_command",
			mcp.WithDescription("Stop/cancel a running command execution"),
			mcp.WithString("execution_id", mcp.Required(), mcp.Description("Execution ID of the running command to stop")),
		),
		s.stopCommandHandler,
	)
}

// validateConfigHandler handles config validation requests
func (s *Server) validateConfigHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	if err := s.configValidator.ValidateConfig(configPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Config validation failed: %v", err)), nil
	}

	configInfo, err := s.configValidator.GetConfigInfo(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get config info: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"message":     "Config file is valid",
		"config_info": configInfo,
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// parseSQLHandler handles SQL parsing requests
func (s *Server) parseSQLHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sqlStatement, err := req.RequireString("sql_statement")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'sql_statement': %v", err)), nil
	}

	parseResult, err := s.sqlParser.ParseSQL(sqlStatement)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("SQL parsing failed: %v", err)), nil
	}

	jsonResult, _ := json.MarshalIndent(parseResult, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getConfigSchemaHandler handles config schema requests
func (s *Server) getConfigSchemaHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	section := req.GetString("section", "")
	if section == "" {
		// No section provided, return all available sections
		allSections, err := s.configSchema.GetAllSections()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get all sections: %v", err)), nil
		}

		jsonResult, _ := json.MarshalIndent(allSections, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
		}, nil
	}

	schemaInfo, err := s.configSchema.GetSchemaInfo(section)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get schema info: %v", err)), nil
	}

	jsonResult, _ := json.MarshalIndent(schemaInfo, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// assessMigrationHandler handles assess-migration command requests (synchronous)
func (s *Server) assessMigrationHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Execute the assess-migration command synchronously
	result, err := s.commandExecutor.ExecuteCommandAsync(ctx, "assess-migration", configPath, additionalArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start assess migration: %v", err)), nil
	}

	// Wait for completion (this may block for long-running commands)
	time.Sleep(2 * time.Second) // Give it some time to start

	response := map[string]interface{}{
		"message":  "Command executed synchronously with --yes flag to avoid interactive prompts. For better experience with long-running commands, consider using assess_migration_async instead.",
		"status":   result.Status,
		"progress": result.Progress,
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// assessMigrationAsyncHandler handles async assess-migration command requests
func (s *Server) assessMigrationAsyncHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Create a cancellable context for this command
	cmdCtx, cancel := context.WithCancel(ctx)

	// Execute the assess-migration command asynchronously
	result, err := s.commandExecutor.ExecuteCommandAsync(cmdCtx, "assess-migration", configPath, additionalArgs)
	if err != nil {
		cancel() // Clean up the context
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start assess migration: %v", err)), nil
	}

	// Store the running command and cancel function
	s.mu.Lock()
	s.runningCommands[result.ExecutionID] = result
	s.commandContexts[result.ExecutionID] = cancel
	s.mu.Unlock()

	response := map[string]interface{}{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
		"message":      "Command started asynchronously. Use get_command_status to track progress.",
		"start_time":   result.StartTime.Format(time.RFC3339),
		"instructions": []string{
			"1. Use 'get_command_status' with the execution_id to check progress",
			"2. Monitor the 'progress' array for real-time output",
			"3. Continue monitoring until status becomes 'completed' or 'failed'",
			"4. Commands automatically include --yes flag to avoid interactive prompts",
		},
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// exportSchemaHandler handles export-schema requests
func (s *Server) exportSchemaHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Execute the export-schema command synchronously
	result, err := s.commandExecutor.ExecuteCommandAsync(ctx, "export schema", configPath, additionalArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start export schema: %v", err)), nil
	}

	// For synchronous execution, we just return the initial result
	// The command will complete in the background

	response := map[string]interface{}{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
		"progress":     result.Progress,
		"start_time":   result.StartTime.Format(time.RFC3339),
		"duration":     result.Duration,
		"exit_code":    result.ExitCode,
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// exportSchemaAsyncHandler handles async export schema requests
func (s *Server) exportSchemaAsyncHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Create a cancellable context for this command
	cmdCtx, cancel := context.WithCancel(ctx)

	// Execute the export schema command asynchronously
	result, err := s.commandExecutor.ExecuteCommandAsync(cmdCtx, "export schema", configPath, additionalArgs)
	if err != nil {
		cancel() // Clean up the context
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start export schema: %v", err)), nil
	}

	// Store the running command and cancel function
	s.mu.Lock()
	s.runningCommands[result.ExecutionID] = result
	s.commandContexts[result.ExecutionID] = cancel
	s.mu.Unlock()

	response := map[string]interface{}{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
		"message":      "Export schema command started asynchronously. Use get_command_status to track progress.",
		"start_time":   result.StartTime.Format(time.RFC3339),
		"instructions": []string{
			"1. Use 'get_command_status' with the execution_id to check progress",
			"2. Monitor the 'progress' array for real-time output",
			"3. Continue monitoring until status becomes 'completed' or 'failed'",
			"4. Commands automatically include --yes flag to avoid interactive prompts",
		},
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getCommandStatusHandler handles command status requests
func (s *Server) getCommandStatusHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	executionID, err := req.RequireString("execution_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'execution_id': %v", err)), nil
	}

	s.mu.RLock()
	result, exists := s.runningCommands[executionID]
	s.mu.RUnlock()

	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Execution ID '%s' not found", executionID)), nil
	}

	// Create response with current status
	response := map[string]interface{}{
		"execution_id": executionID,
		"status":       result.Status,
		"progress":     result.Progress,
		"start_time":   result.StartTime.Format(time.RFC3339),
	}

	if result.EndTime != nil {
		response["end_time"] = result.EndTime.Format(time.RFC3339)
		response["duration"] = result.Duration
		response["exit_code"] = result.ExitCode
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	// If command is completed, remove it from running commands and clean up context
	if result.Status == "completed" || result.Status == "failed" || result.Status == "timeout" {
		s.mu.Lock()
		delete(s.runningCommands, executionID)
		delete(s.commandContexts, executionID)
		s.mu.Unlock()
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// stopCommandHandler handles stopping/canceling running commands
func (s *Server) stopCommandHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	executionID, err := req.RequireString("execution_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'execution_id': %v", err)), nil
	}

	// Get the running command
	s.mu.RLock()
	commandResult, exists := s.runningCommands[executionID]
	cancelFunc, hasCancel := s.commandContexts[executionID]
	s.mu.RUnlock()

	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("No running command found with execution ID: %s", executionID)), nil
	}

	if commandResult.Status != "running" {
		return mcp.NewToolResultError(fmt.Sprintf("Command is not running (status: %s)", commandResult.Status)), nil
	}

	// Cancel the command context
	if hasCancel {
		cancelFunc()
	}

	// Update the command status
	s.mu.Lock()
	commandResult.Status = "cancelled"
	commandResult.Error = "Command was cancelled by user"
	endTime := time.Now()
	commandResult.EndTime = &endTime
	commandResult.Duration = s.commandExecutor.formatDuration(endTime.Sub(commandResult.StartTime))
	commandResult.Progress = append(commandResult.Progress, fmt.Sprintf("[%s] COMMAND_CANCELLED: Cancelled by user", time.Now().Format("15:04:05")))

	// Clean up the context
	delete(s.commandContexts, executionID)
	s.mu.Unlock()

	responseData := map[string]interface{}{
		"execution_id": executionID,
		"status":       "cancelled",
		"message":      "Command was successfully cancelled",
		"timestamp":    time.Now().Format("15:04:05"),
	}

	jsonResult, _ := json.MarshalIndent(responseData, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// analyzeSchemaHandler handles analyze-schema requests
func (s *Server) analyzeSchemaHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Execute the analyze-schema command synchronously
	result, err := s.commandExecutor.ExecuteCommandAsync(ctx, "analyze-schema", configPath, additionalArgs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start analyze schema: %v", err)), nil
	}

	// For synchronous execution, we just return the initial result
	// The command will complete in the background

	response := map[string]interface{}{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
		"progress":     result.Progress,
		"start_time":   result.StartTime.Format(time.RFC3339),
		"duration":     result.Duration,
		"exit_code":    result.ExitCode,
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// analyzeSchemaAsyncHandler handles async analyze-schema requests
func (s *Server) analyzeSchemaAsyncHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Create a cancellable context for this command
	cmdCtx, cancel := context.WithCancel(ctx)

	// Execute the analyze-schema command asynchronously
	result, err := s.commandExecutor.ExecuteCommandAsync(cmdCtx, "analyze-schema", configPath, additionalArgs)
	if err != nil {
		cancel() // Clean up the context
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start analyze schema: %v", err)), nil
	}

	// Store the running command and cancel function
	s.mu.Lock()
	s.runningCommands[result.ExecutionID] = result
	s.commandContexts[result.ExecutionID] = cancel
	s.mu.Unlock()

	response := map[string]interface{}{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
		"message":      "Analyze schema command started asynchronously. Use get_command_status to track progress.",
		"start_time":   result.StartTime.Format(time.RFC3339),
		"instructions": []string{
			"1. Use 'get_command_status' with the execution_id to check progress",
			"2. Monitor the 'progress' array for real-time output",
			"3. Continue monitoring until status becomes 'completed' or 'failed'",
			"4. Commands automatically include --yes flag to avoid interactive prompts",
		},
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getAssessmentReportHandler handles assessment report requests
func (s *Server) getAssessmentReportHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	// Extract export-dir from config file
	exportDir, err := s.extractExportDirFromConfig(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to extract export-dir from config: %v", err)), nil
	}

	// Build the path to the assessment report
	assessmentReportDir := filepath.Join(exportDir, "assessment", "reports")
	jsonReportPath := filepath.Join(assessmentReportDir, "migration_assessment_report.json")

	if !utils.FileOrFolderExists(jsonReportPath) {
		return mcp.NewToolResultError(fmt.Sprintf("Assessment report not found at %s. Run 'assess-migration' command first.", jsonReportPath)), nil
	}

	// Read and parse the assessment report
	reportData, err := s.parseAssessmentReport(jsonReportPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse assessment report: %v", err)), nil
	}

	// Return the complete structured report as JSON
	result := map[string]interface{}{
		"export_dir":  exportDir,
		"report_path": jsonReportPath,
		"timestamp":   time.Now().Format(time.RFC3339),
		"report":      reportData,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getSchemaAnalysisReportHandler handles schema analysis report requests
func (s *Server) getSchemaAnalysisReportHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	// Extract export-dir from config file
	exportDir, err := s.extractExportDirFromConfig(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to extract export-dir from config: %v", err)), nil
	}

	// Build the path to the schema analysis report
	reportsDir := filepath.Join(exportDir, "reports")
	jsonReportPath := filepath.Join(reportsDir, "schema_analysis_report.json")

	if !utils.FileOrFolderExists(jsonReportPath) {
		return mcp.NewToolResultError(fmt.Sprintf("Schema analysis report not found at %s. Run 'analyze-schema' command first.", jsonReportPath)), nil
	}

	// Read and parse the schema analysis report
	reportData, err := s.parseSchemaAnalysisReport(jsonReportPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse schema analysis report: %v", err)), nil
	}

	// Return the complete structured report as JSON
	result := map[string]interface{}{
		"export_dir":  exportDir,
		"report_path": jsonReportPath,
		"timestamp":   time.Now().Format(time.RFC3339),
		"report":      reportData,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// extractExportDirFromConfig extracts the export-dir from a config file
func (s *Server) extractExportDirFromConfig(configPath string) (string, error) {
	// Check if config file exists
	if !utils.FileOrFolderExists(configPath) {
		return "", fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Load config using viper
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	// Get export-dir from config
	exportDir := v.GetString("export-dir")
	if exportDir == "" {
		return "", fmt.Errorf("export-dir not found in config file")
	}

	return exportDir, nil
}

// parseAssessmentReport reads and parses an assessment report JSON file
func (s *Server) parseAssessmentReport(reportPath string) (map[string]interface{}, error) {
	if !utils.FileOrFolderExists(reportPath) {
		return nil, fmt.Errorf("report file %q does not exist", reportPath)
	}

	// Read the JSON file
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read report file %q: %w", reportPath, err)
	}

	// Parse the JSON
	var report map[string]interface{}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse JSON report file %q: %w", reportPath, err)
	}

	return report, nil
}

// parseSchemaAnalysisReport reads and parses a schema analysis report JSON file
func (s *Server) parseSchemaAnalysisReport(reportPath string) (map[string]interface{}, error) {
	if !utils.FileOrFolderExists(reportPath) {
		return nil, fmt.Errorf("report file %q does not exist", reportPath)
	}

	// Read the JSON file
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read report file %q: %w", reportPath, err)
	}

	// Parse the JSON
	var report map[string]interface{}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse JSON report file %q: %w", reportPath, err)
	}

	return report, nil
}
