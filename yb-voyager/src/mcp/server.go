package mcp

import (
	"context"

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
	// Export directory management tools
	s.server.AddTool(
		mcp.NewTool("create_export_directory",
			mcp.WithDescription("Create an export directory for YB Voyager migration"),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Path to the export directory to create")),
		),
		createExportDirectory,
	)

	s.server.AddTool(
		mcp.NewTool("get_export_directory_info",
			mcp.WithDescription("Get information about an export directory"),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Path to the export directory to analyze")),
		),
		getExportDirectoryInfo,
	)

	// Config file management tools
	s.server.AddTool(
		mcp.NewTool("create_config_file",
			mcp.WithDescription("Create a YB Voyager configuration file from template"),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path where the config file should be created")),
			mcp.WithString("template_type", mcp.Description("Template type (live-migration, offline-migration, etc.)")),
			mcp.WithString("export_dir", mcp.Description("Export directory path")),
			mcp.WithString("source_db_type", mcp.Description("Source database type (postgresql, oracle)")),
			mcp.WithString("source_db_host", mcp.Description("Source database host")),
			mcp.WithString("source_db_name", mcp.Description("Source database name")),
			mcp.WithString("source_db_user", mcp.Description("Source database user")),
		),
		createConfigFile,
	)

	s.server.AddTool(
		mcp.NewTool("validate_config_file",
			mcp.WithDescription("Validate a YB Voyager configuration file"),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the config file to validate")),
		),
		validateConfigFile,
	)

	s.server.AddTool(
		mcp.NewTool("generate_config_content",
			mcp.WithDescription("Generate YB Voyager configuration file content without writing to disk (useful when file system permissions are restricted)"),
			mcp.WithString("template_type", mcp.Description("Template type (live-migration, offline-migration, etc.)")),
			mcp.WithString("export_dir", mcp.Description("Export directory path")),
			mcp.WithString("source_db_type", mcp.Description("Source database type (postgresql, oracle)")),
			mcp.WithString("source_db_host", mcp.Description("Source database host")),
			mcp.WithString("source_db_name", mcp.Description("Source database name")),
			mcp.WithString("source_db_user", mcp.Description("Source database user")),
		),
		generateConfigContent,
	)

	// Command execution tools
	s.server.AddTool(
		mcp.NewTool("execute_voyager_with_config",
			mcp.WithDescription("Execute YB Voyager commands using a configuration file"),
			mcp.WithString("command", mcp.Required(), mcp.Description("YB Voyager command to execute (e.g., 'assess-migration', 'export-schema')")),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the configuration file")),
			mcp.WithString("additional_args", mcp.Description("Additional command-line arguments")),
		),
		executeVoyagerWithConfig,
	)

	s.server.AddTool(
		mcp.NewTool("execute_voyager_command",
			mcp.WithDescription("Execute YB Voyager commands with individual parameters (legacy method)"),
			mcp.WithString("command", mcp.Required(), mcp.Description("YB Voyager command to execute")),
			mcp.WithString("export_dir", mcp.Description("Export directory path")),
			mcp.WithString("args", mcp.Description("Additional command arguments")),
		),
		executeVoyagerCommand,
	)

	// Migration status and metadata tools
	s.server.AddTool(
		mcp.NewTool("query_migration_status",
			mcp.WithDescription("Query the current migration status from metaDB"),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Export directory containing the metaDB")),
		),
		queryMigrationStatus,
	)

	s.server.AddTool(
		mcp.NewTool("get_metadb_stats",
			mcp.WithDescription("Get statistics and metadata from the migration database"),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Export directory containing the metaDB")),
		),
		getMetaDBStats,
	)
}

// registerResources registers all MCP resources
func (s *Server) registerResources() {
	// Migration status resource
	s.server.AddResource(
		mcp.Resource{
			URI:         "voyager://migration-status",
			Name:        "Migration Status",
			Description: "Current migration status and progress information",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := getMigrationStatusResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		},
	)

	// Assessment report resource template
	s.server.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"voyager://assessment-report/{export_dir}",
			"Assessment Report",
			mcp.WithTemplateDescription("Migration assessment report with complexity analysis, issues, and recommendations"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := getAssessmentReportResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		},
	)

	// Schema analysis report resource template
	s.server.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"voyager://schema-analysis-report/{export_dir}",
			"Schema Analysis Report",
			mcp.WithTemplateDescription("Detailed schema analysis report with compatibility issues and object summaries"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := getSchemaAnalysisReportResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		},
	)

	// Logs resource template
	s.server.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"voyager://logs/{export_dir}/recent",
			"Recent Migration Logs",
			mcp.WithTemplateDescription("Recent log files from the migration process"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := getLogsResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		},
	)

	// Schema files resource template (for exported schema files)
	s.server.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"voyager://schema-files/{export_dir}",
			"Schema Files",
			mcp.WithTemplateDescription("Exported schema files and structure information"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := getSchemaFilesResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		},
	)

	// Config templates resource: exposes available config template files to the LLM
	s.server.AddResource(
		mcp.Resource{
			URI:         "voyager://config-templates",
			Name:        "Configuration Templates",
			Description: "Available YB Voyager configuration templates. This resource provides the LLM with access to the template files used for generating configuration files.",
			MIMEType:    "application/json",
		},
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			result, err := getConfigTemplatesResource(ctx, req.Params.URI)
			if err != nil {
				return nil, err
			}
			return result.Contents, nil
		},
	)
}

// registerPrompts registers all MCP prompts
func (s *Server) registerPrompts() {
	// Troubleshooting prompt
	s.server.AddPrompt(
		mcp.NewPrompt("troubleshoot_migration",
			mcp.WithPromptDescription("Get troubleshooting guidance for YB Voyager migration issues"),
			mcp.WithArgument("export_dir", mcp.RequiredArgument(), mcp.ArgumentDescription("Export directory path for the migration")),
		),
		troubleshootMigrationPrompt,
	)

	// Performance optimization prompt
	s.server.AddPrompt(
		mcp.NewPrompt("optimize_performance",
			mcp.WithPromptDescription("Get performance optimization recommendations for YB Voyager migration"),
			mcp.WithArgument("export_dir", mcp.RequiredArgument(), mcp.ArgumentDescription("Export directory path for the migration")),
		),
		optimizePerformancePrompt,
	)

	// Config generation prompt
	s.server.AddPrompt(
		mcp.NewPrompt("generate_config",
			mcp.WithPromptDescription("Get help generating YB Voyager configuration files"),
			mcp.WithArgument("migration_type", mcp.ArgumentDescription("Type of migration (live-migration, offline-migration, etc.)")),
			mcp.WithArgument("source_type", mcp.ArgumentDescription("Source database type (postgresql, oracle)")),
		),
		configGenerationPrompt,
	)
}
