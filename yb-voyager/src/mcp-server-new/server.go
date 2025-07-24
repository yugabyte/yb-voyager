package mcpservernew

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
)

// Server represents the MCP server
type Server struct {
	server          *server.MCPServer
	configValidator *ConfigValidator
	sqlParser       *SQLParser
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
			mcp.WithDescription("Validate that a config file exists and is readable"),
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
