package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"gopkg.in/yaml.v2"
)

// ConfigFile represents a yb-voyager configuration file structure
type ConfigFile struct {
	ExportDir string `yaml:"export-dir"`
	LogLevel  string `yaml:"log-level"`
	Source    struct {
		DBType     string `yaml:"db-type"`
		DBHost     string `yaml:"db-host"`
		DBPort     int    `yaml:"db-port"`
		DBName     string `yaml:"db-name"`
		DBSchema   string `yaml:"db-schema"`
		DBUser     string `yaml:"db-user"`
		DBPassword string `yaml:"db-password"`
	} `yaml:"source"`
	Target struct {
		DBHost     string `yaml:"db-host"`
		DBPort     int    `yaml:"db-port"`
		DBName     string `yaml:"db-name"`
		DBUser     string `yaml:"db-user"`
		DBPassword string `yaml:"db-password"`
	} `yaml:"target"`
	AssessMigration struct {
		TargetDBVersion string `yaml:"target-db-version"`
	} `yaml:"assess-migration"`
	ExportSchema struct {
		ObjectTypeList string `yaml:"object-type-list"`
	} `yaml:"export-schema"`
	ExportData struct {
		ParallelJobs int `yaml:"parallel-jobs"`
	} `yaml:"export-data"`
	ImportData struct {
		ParallelJobs int `yaml:"parallel-jobs"`
	} `yaml:"import-data"`
}

// ExportDirInfo represents information about an export directory
type ExportDirInfo struct {
	Path          string   `json:"path"`
	Exists        bool     `json:"exists"`
	IsInitialized bool     `json:"is_initialized"`
	HasMetaDB     bool     `json:"has_metadb"`
	Size          int64    `json:"size_bytes"`
	Contents      []string `json:"contents"`
}

// Tool handlers

// createExportDirectory creates an export directory if it doesn't exist
func createExportDirectory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exportDir, err := req.RequireString("export_dir")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'export_dir': %v", err)), nil
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(exportDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}

	// Check if directory already exists
	if utils.FileOrFolderExists(absPath) {
		info := getExportDirInfo(absPath)
		result := map[string]interface{}{
			"status": "already_exists",
			"path":   absPath,
			"info":   info,
		}
		jsonResult, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
		}, nil
	}

	// Create directory
	err = os.MkdirAll(absPath, 0755)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create directory: %v", err)), nil
	}

	info := getExportDirInfo(absPath)
	result := map[string]interface{}{
		"status": "created",
		"path":   absPath,
		"info":   info,
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getExportDirectoryInfo returns information about an export directory
func getExportDirectoryInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exportDir, err := req.RequireString("export_dir")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'export_dir': %v", err)), nil
	}

	absPath, err := filepath.Abs(exportDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}

	info := getExportDirInfo(absPath)
	jsonResult, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// createConfigFile creates a yb-voyager configuration file from parameters
func createConfigFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	templateType := req.GetString("template_type", "live-migration")
	exportDir := req.GetString("export_dir", "")

	// Check directory permissions before attempting to write
	configDir := filepath.Dir(configPath)
	if err := checkDirectoryWritable(configDir); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Cannot write to directory %s: %v\n\nSuggestion: Use a directory like ~/yb-voyager-workspace/ or ask the user to create the config file manually.", configDir, err)), nil
	}

	// Load template
	templatePath := filepath.Join("config-templates", templateType+".yaml")
	templateContent, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read template %s: %v", templateType, err)), nil
	}

	// Replace placeholders with actual values if provided
	content := string(templateContent)
	if exportDir != "" {
		content = strings.ReplaceAll(content, "<export-dir-path>", exportDir)
	}

	// Apply other parameters from request
	if sourceDBType := req.GetString("source_db_type", ""); sourceDBType != "" {
		content = strings.ReplaceAll(content, "db-type: postgresql", "db-type: "+sourceDBType)
	}
	if sourceDBHost := req.GetString("source_db_host", ""); sourceDBHost != "" {
		content = strings.ReplaceAll(content, "db-host: localhost", "db-host: "+sourceDBHost)
	}
	if sourceDBName := req.GetString("source_db_name", ""); sourceDBName != "" {
		content = strings.ReplaceAll(content, "db-name: test_db", "db-name: "+sourceDBName)
	}
	if sourceDBUser := req.GetString("source_db_user", ""); sourceDBUser != "" {
		content = strings.ReplaceAll(content, "db-user: test_user", "db-user: "+sourceDBUser)
	}

	// Write config file
	err = ioutil.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write config file: %v\n\nSuggestion: Try using a directory like ~/yb-voyager-workspace/ or ask the user to create the file manually.", err)), nil
	}

	result := map[string]interface{}{
		"status":        "created",
		"config_path":   configPath,
		"template_type": templateType,
		"export_dir":    exportDir,
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// executeVoyagerWithConfig executes yb-voyager commands using a config file
func executeVoyagerWithConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'command': %v", err)), nil
	}

	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	// Validate config file exists
	if !utils.FileOrFolderExists(configPath) {
		return mcp.NewToolResultError(fmt.Sprintf("Config file does not exist: %s", configPath)), nil
	}

	additionalArgs := req.GetString("additional_args", "")

	// Build the command arguments
	cmdArgs := []string{command, "--config-file", configPath}

	// Add additional arguments if provided
	if additionalArgs != "" {
		args := strings.Fields(additionalArgs)
		cmdArgs = append(cmdArgs, args...)
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, "yb-voyager", cmdArgs...)
	cmd.Dir = filepath.Dir(configPath) // Set working directory to config file directory

	output, err := cmd.CombinedOutput()

	result := map[string]interface{}{
		"command":     "yb-voyager " + strings.Join(cmdArgs, " "),
		"config_path": configPath,
		"output":      string(output),
		"success":     err == nil,
	}

	if err != nil {
		result["error"] = err.Error()
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	textContent := mcp.NewTextContent(string(jsonResult))

	return &mcp.CallToolResult{
		Content: []mcp.Content{textContent},
		IsError: err != nil,
	}, nil
}

// validateConfigFile validates a yb-voyager configuration file
func validateConfigFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configPath, err := req.RequireString("config_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'config_path': %v", err)), nil
	}

	// Check if file exists
	if !utils.FileOrFolderExists(configPath) {
		return mcp.NewToolResultError(fmt.Sprintf("Config file does not exist: %s", configPath)), nil
	}

	// Read and parse config file
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read config file: %v", err)), nil
	}

	var config ConfigFile
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse YAML config: %v", err)), nil
	}

	// Validate required fields
	issues := []string{}

	if config.ExportDir == "" {
		issues = append(issues, "export-dir is required")
	} else if !utils.FileOrFolderExists(config.ExportDir) {
		issues = append(issues, fmt.Sprintf("export-dir does not exist: %s", config.ExportDir))
	}

	if config.Source.DBType == "" {
		issues = append(issues, "source.db-type is required")
	} else if config.Source.DBType != "postgresql" && config.Source.DBType != "oracle" {
		issues = append(issues, "source.db-type must be 'postgresql' or 'oracle'")
	}

	if config.Source.DBHost == "" {
		issues = append(issues, "source.db-host is required")
	}

	if config.Source.DBName == "" {
		issues = append(issues, "source.db-name is required")
	}

	result := map[string]interface{}{
		"config_path": configPath,
		"valid":       len(issues) == 0,
		"issues":      issues,
		"config":      config,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// executeVoyagerCommand executes yb-voyager commands as a subprocess (legacy method)
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

	// Add additional arguments if provided
	if args != "" {
		additionalArgs := strings.Fields(args)
		cmdArgs = append(cmdArgs, additionalArgs...)
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, "yb-voyager", cmdArgs...)
	output, err := cmd.CombinedOutput()

	result := map[string]interface{}{
		"command": "yb-voyager " + strings.Join(cmdArgs, " "),
		"output":  string(output),
		"success": err == nil,
	}

	if err != nil {
		result["error"] = err.Error()
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	textContent := mcp.NewTextContent(string(jsonResult))

	return &mcp.CallToolResult{
		Content: []mcp.Content{textContent},
		IsError: err != nil,
	}, nil
}

// queryMigrationStatus queries the migration status from metaDB
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

	result := map[string]interface{}{
		"export_dir": exportDir,
		"status":     status,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getMetaDBStats retrieves statistics from the metaDB
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

	// Get various statistics
	stats := map[string]interface{}{
		"export_dir": exportDir,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	// Try to get different stats (some may not be available depending on migration phase)
	if migrationStatus, err := metaDB.GetMigrationStatusRecord(); err == nil {
		stats["migration_status"] = migrationStatus
	}

	jsonResult, _ := json.MarshalIndent(stats, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// Resource handlers

// getMigrationStatusResource returns migration status as a resource
func getMigrationStatusResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	// Extract export_dir from URI if provided
	exportDir := extractExportDirFromURI(uri)
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir not found in URI")
	}

	metaDB, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metaDB: %v", err)
	}

	status, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("failed to get migration status: %v", err)
	}

	result := map[string]interface{}{
		"export_dir": exportDir,
		"status":     status,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// getLogsResource returns recent logs as a resource
func getLogsResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	exportDir := extractExportDirFromURI(uri)
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir not found in URI")
	}

	logDir := filepath.Join(exportDir, "logs")
	if !utils.FileOrFolderExists(logDir) {
		return &mcp.ReadResourceResult{
			Contents: []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      uri,
					MIMEType: "text/plain",
					Text:     "No logs directory found",
				},
			},
		}, nil
	}

	// Get recent log files
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs directory: %v", err)
	}

	var recentLogs []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
			recentLogs = append(recentLogs, file.Name())
		}
	}

	result := map[string]interface{}{
		"export_dir":  exportDir,
		"logs_dir":    logDir,
		"recent_logs": recentLogs,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// getSchemaAnalysisResource returns schema analysis as a resource
func getSchemaAnalysisResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	exportDir := extractExportDirFromURI(uri)
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir not found in URI")
	}

	// Check for schema analysis files
	schemaDir := filepath.Join(exportDir, "schema")
	assessmentDir := filepath.Join(exportDir, "assessment")

	result := map[string]interface{}{
		"export_dir": exportDir,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if utils.FileOrFolderExists(schemaDir) {
		files, _ := ioutil.ReadDir(schemaDir)
		var schemaFiles []string
		for _, file := range files {
			if !file.IsDir() {
				schemaFiles = append(schemaFiles, file.Name())
			}
		}
		result["schema_files"] = schemaFiles
	}

	if utils.FileOrFolderExists(assessmentDir) {
		files, _ := ioutil.ReadDir(assessmentDir)
		var assessmentFiles []string
		for _, file := range files {
			if !file.IsDir() {
				assessmentFiles = append(assessmentFiles, file.Name())
			}
		}
		result["assessment_files"] = assessmentFiles
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// getConfigTemplatesResource returns available config templates
func getConfigTemplatesResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	templatesDir := "config-templates"

	if !utils.FileOrFolderExists(templatesDir) {
		return nil, fmt.Errorf("config templates directory not found")
	}

	files, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %v", err)
	}

	var templates []map[string]interface{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			templateName := strings.TrimSuffix(file.Name(), ".yaml")
			templates = append(templates, map[string]interface{}{
				"name":     templateName,
				"filename": file.Name(),
				"size":     file.Size(),
				"modified": file.ModTime().Format(time.RFC3339),
			})
		}
	}

	result := map[string]interface{}{
		"templates_dir": templatesDir,
		"templates":     templates,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// Prompt handlers

// troubleshootMigrationPrompt provides troubleshooting guidance
func troubleshootMigrationPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	exportDir := req.Params.Arguments["export_dir"]
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir argument is required")
	}

	// Get current migration status
	var statusInfo string
	if metaDB, err := metadb.NewMetaDB(exportDir); err == nil {
		if status, err := metaDB.GetMigrationStatusRecord(); err == nil {
			statusData, _ := json.MarshalIndent(status, "", "  ")
			statusInfo = string(statusData)
		}
	}

	prompt := fmt.Sprintf(`You are troubleshooting a YB Voyager migration. Here's the current status:

Export Directory: %s
Current Status: %s

Please analyze the migration status and provide specific troubleshooting steps for any issues found.
Consider common migration problems like:
- Schema compatibility issues
- Data type mismatches
- Connection problems
- Performance bottlenecks
- Incomplete migrations

Provide actionable recommendations with specific yb-voyager commands to resolve issues.`, exportDir, statusInfo)

	return &mcp.GetPromptResult{
		Description: "Troubleshoot YB Voyager migration issues",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: prompt,
				},
			},
		},
	}, nil
}

// optimizePerformancePrompt provides performance optimization guidance
func optimizePerformancePrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	exportDir := req.Params.Arguments["export_dir"]
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir argument is required")
	}

	// Get migration stats
	var statsInfo string
	if metaDB, err := metadb.NewMetaDB(exportDir); err == nil {
		stats := map[string]interface{}{
			"export_dir": exportDir,
			"timestamp":  time.Now().Format(time.RFC3339),
		}

		if migrationStatus, err := metaDB.GetMigrationStatusRecord(); err == nil {
			stats["migration_status"] = migrationStatus
		}

		statsData, _ := json.MarshalIndent(stats, "", "  ")
		statsInfo = string(statsData)
	}

	prompt := fmt.Sprintf(`You are optimizing the performance of a YB Voyager migration. Here are the current statistics:

Export Directory: %s
Migration Statistics: %s

Please provide specific performance optimization recommendations including:
- Parallel job configuration
- Batch size tuning
- Resource allocation
- Network optimization
- Database-specific optimizations

Suggest specific yb-voyager command modifications and configuration changes to improve migration performance.`, exportDir, statsInfo)

	return &mcp.GetPromptResult{
		Description: "Optimize YB Voyager migration performance",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: prompt,
				},
			},
		},
	}, nil
}

// configGenerationPrompt helps generate config files
func configGenerationPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	migrationType := req.Params.Arguments["migration_type"]
	if migrationType == "" {
		migrationType = "live-migration"
	}

	sourceType := req.Params.Arguments["source_type"]
	if sourceType == "" {
		sourceType = "postgresql"
	}

	prompt := fmt.Sprintf(`You are helping to generate a YB Voyager configuration file for a %s migration from %s to YugabyteDB.

Available templates:
- live-migration: For live migrations with CDC
- live-migration-with-fall-back: Live migration with fallback capability
- live-migration-with-fall-forward: Live migration with fall-forward capability
- offline-migration: For offline migrations
- bulk-data-load: For bulk data loading scenarios

Please help the user create a configuration file by:
1. Asking for the required connection parameters (source and target database details)
2. Suggesting appropriate migration settings based on their use case
3. Providing the create_config_file tool call with the collected parameters

Key parameters to collect:
- export_dir: Where to store migration artifacts
- source database: host, port, database name, user, password
- target database: host, port, database name, user, password
- migration-specific settings: parallel jobs, batch sizes, etc.

Use the create_config_file tool to generate the configuration file once you have the necessary information.`, migrationType, sourceType)

	return &mcp.GetPromptResult{
		Description: "Generate YB Voyager configuration file",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: prompt,
				},
			},
		},
	}, nil
}

// Helper functions

// getExportDirInfo returns detailed information about an export directory
func getExportDirInfo(exportDir string) ExportDirInfo {
	info := ExportDirInfo{
		Path:   exportDir,
		Exists: utils.FileOrFolderExists(exportDir),
	}

	if !info.Exists {
		return info
	}

	// Check if initialized (has metainfo directory)
	metaInfoDir := filepath.Join(exportDir, "metainfo")
	info.IsInitialized = utils.FileOrFolderExists(metaInfoDir)

	// Check if has metaDB
	metaDBPath := filepath.Join(exportDir, "metainfo", "meta.db")
	info.HasMetaDB = utils.FileOrFolderExists(metaDBPath)

	// Get directory size and contents
	if files, err := ioutil.ReadDir(exportDir); err == nil {
		for _, file := range files {
			info.Contents = append(info.Contents, file.Name())
			info.Size += file.Size()
		}
	}

	return info
}

// extractExportDirFromURI extracts export directory from voyager:// URI
func extractExportDirFromURI(uri string) string {
	// Expected format: voyager://resource-type/export-dir/...
	// or voyager://resource-type?export_dir=path

	if strings.Contains(uri, "?export_dir=") {
		parts := strings.Split(uri, "?export_dir=")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	// Try to extract from path
	if strings.HasPrefix(uri, "voyager://") {
		path := strings.TrimPrefix(uri, "voyager://")
		parts := strings.Split(path, "/")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	return ""
}

// checkDirectoryWritable checks if a directory is writable
func checkDirectoryWritable(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist")
	}

	// Try to create a temporary file to test write permissions
	tempFile := filepath.Join(dir, ".mcp_write_test")
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("no write permission")
	}
	file.Close()
	os.Remove(tempFile)
	return nil
}

// generateConfigContent generates config file content without writing to disk
func generateConfigContent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templateType := req.GetString("template_type", "live-migration")
	exportDir := req.GetString("export_dir", "")

	// Load template
	templatePath := filepath.Join("config-templates", templateType+".yaml")
	templateContent, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read template %s: %v", templateType, err)), nil
	}

	// Replace placeholders with actual values if provided
	content := string(templateContent)
	if exportDir != "" {
		content = strings.ReplaceAll(content, "<export-dir-path>", exportDir)
	}

	// Apply other parameters from request
	if sourceDBType := req.GetString("source_db_type", ""); sourceDBType != "" {
		content = strings.ReplaceAll(content, "db-type: postgresql", "db-type: "+sourceDBType)
	}
	if sourceDBHost := req.GetString("source_db_host", ""); sourceDBHost != "" {
		content = strings.ReplaceAll(content, "db-host: localhost", "db-host: "+sourceDBHost)
	}
	if sourceDBName := req.GetString("source_db_name", ""); sourceDBName != "" {
		content = strings.ReplaceAll(content, "db-name: test_db", "db-name: "+sourceDBName)
	}
	if sourceDBUser := req.GetString("source_db_user", ""); sourceDBUser != "" {
		content = strings.ReplaceAll(content, "db-user: test_user", "db-user: "+sourceDBUser)
	}

	result := map[string]interface{}{
		"template_type": templateType,
		"export_dir":    exportDir,
		"content":       content,
		"instructions":  "Copy this content to a file with .yaml extension, then use the execute_voyager_with_config tool to run commands.",
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}
