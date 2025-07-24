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
	configSchema    *ConfigSchema
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
