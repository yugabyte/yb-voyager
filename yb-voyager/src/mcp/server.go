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
		mcp.NewTool("get_export_directory_info",
			mcp.WithDescription("Get information about an export directory"),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Path to the export directory to analyze")),
		),
		getExportDirectoryInfo,
	)

	// Config file management tools
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

	// Command execution tools - 3-tiered approach for optimal LLM experience
	s.server.AddTool(
		mcp.NewTool("execute_voyager_command",
			mcp.WithDescription(`**RECOMMENDED** Execute YB Voyager commands with smart normalization and automatic parameter routing.

VALID COMMANDS (accepts variations):
✅ assess-migration     - Analyze source database compatibility
✅ export schema        - Export database schema (also: export-schema, schema export)
✅ export data          - Export table data (also: export-data, data export)
✅ analyze-schema       - Analyze exported schema for issues
✅ import schema        - Import schema to target (also: import-schema, schema import)
✅ import data          - Import data to target (also: import-data, data import)
✅ get data-migration-report - Check migration status (also: status, migration status)
✅ end migration        - Complete migration (also: end-migration, finish migration)
✅ version              - Show version info

EXAMPLES:
• "export schema" → Exports database schema
• "export-data" → Normalized to "export data"
• "data import" → Normalized to "import data"
• "status" → Normalized to "get data-migration-report"

The tool automatically normalizes command variations and chooses the best execution method.`),
			mcp.WithString("command", mcp.Required(), mcp.Description("YB Voyager command to execute. Accepts variations like 'export-schema', 'data export', etc.")),
			mcp.WithString("config_path", mcp.Description("Path to configuration file (preferred method)")),
			mcp.WithString("export_dir", mcp.Description("Export directory path (used if config_path not provided)")),
			mcp.WithString("additional_args", mcp.Description("Additional command-line arguments")),
		),
		executeVoyagerCommandSmart,
	)

	s.server.AddTool(
		mcp.NewTool("execute_voyager_with_config",
			mcp.WithDescription("Execute YB Voyager commands using a configuration file. This is the preferred method when you have a config file available and want explicit config-file execution."),
			mcp.WithString("command", mcp.Required(), mcp.Description("YB Voyager command to execute (e.g., 'assess-migration', 'export schema', 'export data', 'import schema', 'import data')")),
			mcp.WithString("config_path", mcp.Required(), mcp.Description("Path to the configuration file")),
			mcp.WithString("additional_args", mcp.Description("Additional command-line arguments")),
		),
		executeVoyagerWithConfig,
	)

	s.server.AddTool(
		mcp.NewTool("execute_voyager_legacy",
			mcp.WithDescription("**LEGACY** Execute YB Voyager commands with individual parameters. Only use this if config files are not available or specifically requested. The smart execute_voyager_command tool is preferred."),
			mcp.WithString("command", mcp.Required(), mcp.Description("YB Voyager command to execute (e.g., 'assess-migration', 'export schema', 'export data', 'import schema', 'import data')")),
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

	// Complete report access tools
	s.server.AddTool(
		mcp.NewTool("get_assessment_report",
			mcp.WithDescription("Get the complete YB Voyager migration assessment report as structured JSON. This includes all issues, sizing recommendations, schema summary, and performance statistics."),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Export directory path containing the assessment report")),
		),
		getAssessmentReport,
	)

	s.server.AddTool(
		mcp.NewTool("get_schema_analysis_report",
			mcp.WithDescription("Get the complete YB Voyager schema analysis report as structured JSON. This includes all compatibility issues, object summaries, and migration readiness information."),
			mcp.WithString("export_dir", mcp.Required(), mcp.Description("Export directory path containing the schema analysis report")),
		),
		getSchemaAnalysisReport,
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
