package mcpservernew

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
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
	s.server.AddTool(
		mcp.NewTool("assess_migration",
			mcp.WithDescription("Execute YB Voyager assess-migration command synchronously with automatic --yes flag. WARNING: This may block for long-running commands. Use assess_migration_async for better experience."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and assess-migration sections")),
			mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional, --yes is automatically added)")),
		),
		s.assessMigrationHandler,
	)

	// Async assess migration tool (RECOMMENDED for long-running commands)
	s.server.AddTool(
		mcp.NewTool("assess_migration_async",
			mcp.WithDescription("Execute YB Voyager assess-migration command asynchronously with real-time output streaming. RECOMMENDED for long-running commands. Returns execution ID for tracking progress. Automatically adds --yes flag to avoid interactive prompts."),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file containing source and assess-migration sections")),
			mcp.WithString("additional_args", mcp.Description("Additional command line arguments (optional)")),
		),
		s.assessMigrationAsyncHandler,
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
			"3. Look for 'INTERACTIVE_PROMPT:' lines for user input needed",
			"4. Use 'respond_to_prompt' if interactive prompts are detected",
			"5. Continue monitoring until status becomes 'completed' or 'failed'",
			"6. Interactive prompts will be detected but NOT automatically responded to",
			"7. You must manually use respond_to_prompt to provide user input",
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
