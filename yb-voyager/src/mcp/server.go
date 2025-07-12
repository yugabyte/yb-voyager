package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server represents the MCP server for yb-voyager
type Server struct {
	server *server.MCPServer
}

// NewServer creates a new MCP server instance
func NewServer() *Server {
	mcpServer := server.NewMCPServer(
		"yb-voyager",
		"1.0.0",
		server.WithLogging(),
	)

	s := &Server{
		server: mcpServer,
	}

	// Register MCP capabilities
	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

// Run starts the MCP server with stdio transport
func (s *Server) Run() error {
	return server.ServeStdio(s.server)
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// Tool 1: Execute yb-voyager commands
	s.server.AddTool(mcp.NewTool("execute_voyager_command",
		mcp.WithDescription("Execute yb-voyager commands with arguments"),
		mcp.WithString("command", mcp.Required(), mcp.Description("The yb-voyager command to execute (e.g., 'export schema', 'import data')")),
		mcp.WithString("export_dir", mcp.Description("Export directory path (required for most commands)")),
		mcp.WithString("args", mcp.Description("Additional command arguments as JSON array")),
	), s.handleExecuteVoyagerCommand)

	// Tool 2: Query migration status
	s.server.AddTool(mcp.NewTool("query_migration_status",
		mcp.WithDescription("Query current migration status from metaDB"),
		mcp.WithString("export_dir", mcp.Required(), mcp.Description("Export directory path containing metaDB")),
	), s.handleQueryMigrationStatus)

	// Tool 3: Get metaDB statistics
	s.server.AddTool(mcp.NewTool("get_metadb_stats",
		mcp.WithDescription("Get statistics from metaDB including table counts, data sizes, etc."),
		mcp.WithString("export_dir", mcp.Required(), mcp.Description("Export directory path containing metaDB")),
	), s.handleGetMetaDBStats)
}

// registerResources registers all MCP resources
func (s *Server) registerResources() {
	// Resource 1: Migration status
	s.server.AddResource(
		mcp.NewResource("voyager://migration-status", "Current migration status and progress"),
		s.handleMigrationStatusResource,
	)

	// Resource 2: Recent logs
	s.server.AddResourceTemplate(
		mcp.NewResourceTemplate("voyager://logs/{export_dir}/recent", "Recent migration logs"),
		s.handleLogsResource,
	)

	// Resource 3: Schema analysis
	s.server.AddResourceTemplate(
		mcp.NewResourceTemplate("voyager://schema-analysis/{export_dir}", "Schema analysis results and compatibility report"),
		s.handleSchemaAnalysisResource,
	)
}

// registerPrompts registers all MCP prompts
func (s *Server) registerPrompts() {
	// Prompt 1: Troubleshoot migration
	s.server.AddPrompt(
		mcp.NewPrompt("troubleshoot_migration",
			mcp.WithPromptDescription("Help troubleshoot migration issues"),
			mcp.WithArgument("export_dir", mcp.ArgumentDescription("Export directory path"), mcp.RequiredArgument()),
			mcp.WithArgument("error_context", mcp.ArgumentDescription("Error message or context")),
		),
		s.handleTroubleshootPrompt,
	)

	// Prompt 2: Optimize performance
	s.server.AddPrompt(
		mcp.NewPrompt("optimize_performance",
			mcp.WithPromptDescription("Provide performance optimization suggestions"),
			mcp.WithArgument("export_dir", mcp.ArgumentDescription("Export directory path"), mcp.RequiredArgument()),
			mcp.WithArgument("migration_phase", mcp.ArgumentDescription("Current migration phase (export/import/cutover)")),
		),
		s.handleOptimizePrompt,
	)
}

// Tool handlers
func (s *Server) handleExecuteVoyagerCommand(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return executeVoyagerCommand(ctx, req)
}

func (s *Server) handleQueryMigrationStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return queryMigrationStatus(ctx, req)
}

func (s *Server) handleGetMetaDBStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return getMetaDBStats(ctx, req)
}

// Resource handlers
func (s *Server) handleMigrationStatusResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return readMigrationStatusResource(ctx, req)
}

func (s *Server) handleLogsResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return readLogsResource(ctx, req)
}

func (s *Server) handleSchemaAnalysisResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return readSchemaAnalysisResource(ctx, req)
}

// Prompt handlers
func (s *Server) handleTroubleshootPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return getTroubleshootPrompt(ctx, req)
}

func (s *Server) handleOptimizePrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return getOptimizePrompt(ctx, req)
}

// Helper function to convert interface{} to JSON string
func toJSONString(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling to JSON: %v", err)
	}
	return string(b)
}

// Helper function to read file contents
func readFileContents(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filepath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	return string(content), nil
}
