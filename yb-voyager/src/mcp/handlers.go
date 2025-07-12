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
	"syscall"
	"time"

	"regexp"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	"gopkg.in/yaml.v2"
)

// ConfigFile represents a YB Voyager configuration file structure
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

// AssessmentReport represents the migration assessment report structure
// This mirrors the structure from cmd/common.go to avoid import cycles
type AssessmentReport struct {
	VoyagerVersion                 string                                `json:"VoyagerVersion"`
	TargetDBVersion                *ybversion.YBVersion                  `json:"TargetDBVersion"`
	MigrationComplexity            string                                `json:"MigrationComplexity"`
	MigrationComplexityExplanation string                                `json:"MigrationComplexityExplanation"`
	SchemaSummary                  utils.SchemaSummary                   `json:"SchemaSummary"`
	Sizing                         *migassessment.SizingAssessmentReport `json:"Sizing"`
	Issues                         []AssessmentIssue                     `json:"AssessmentIssues"`
	TableIndexStats                *[]migassessment.TableIndexStats      `json:"TableIndexStats"`
	Notes                          []string                              `json:"Notes"`
}

// AssessmentIssue represents an individual assessment issue
type AssessmentIssue struct {
	Category               string                          `json:"Category"`
	CategoryDescription    string                          `json:"CategoryDescription"`
	Type                   string                          `json:"Type"`
	Name                   string                          `json:"Name"`
	Description            string                          `json:"Description"`
	Impact                 string                          `json:"Impact"`
	ObjectType             string                          `json:"ObjectType"`
	ObjectName             string                          `json:"ObjectName"`
	SqlStatement           string                          `json:"SqlStatement"`
	DocsLink               string                          `json:"DocsLink"`
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion `json:"MinimumVersionsFixedIn"`
	Details                map[string]interface{}          `json:"Details,omitempty"`
}

// Helper functions for parsing reports using YB Voyager's jsonfile utility
func parseAssessmentReport(reportPath string) (*AssessmentReport, error) {
	if !utils.FileOrFolderExists(reportPath) {
		return nil, fmt.Errorf("report file %q does not exist", reportPath)
	}

	var report AssessmentReport
	err := jsonfile.NewJsonFile[AssessmentReport](reportPath).Load(&report)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json report file %q: %w", reportPath, err)
	}

	return &report, nil
}

func parseSchemaAnalysisReport(reportPath string) (*utils.SchemaReport, error) {
	if !utils.FileOrFolderExists(reportPath) {
		return nil, fmt.Errorf("report file %q does not exist", reportPath)
	}

	var report utils.SchemaReport
	err := jsonfile.NewJsonFile[utils.SchemaReport](reportPath).Load(&report)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json report file %q: %w", reportPath, err)
	}

	return &report, nil
}

// Helper methods for AssessmentReport
func (ar *AssessmentReport) GetShardedTablesRecommendation() ([]string, error) {
	if ar.Sizing == nil {
		return nil, fmt.Errorf("sizing report is null, can't fetch sharded tables")
	}
	return ar.Sizing.SizingRecommendation.ShardedTables, nil
}

func (ar *AssessmentReport) GetColocatedTablesRecommendation() ([]string, error) {
	if ar.Sizing == nil {
		return nil, fmt.Errorf("sizing report is null, can't fetch colocated tables")
	}
	return ar.Sizing.SizingRecommendation.ColocatedTables, nil
}

func (ar *AssessmentReport) GetClusterSizingRecommendation() string {
	if ar.Sizing == nil {
		return ""
	}
	if ar.Sizing.FailureReasoning != "" {
		return ar.Sizing.FailureReasoning
	}
	return fmt.Sprintf("Num Nodes: %f, vCPU per instance: %d, Memory per instance: %d, Estimated Import Time: %f minutes",
		ar.Sizing.SizingRecommendation.NumNodes, ar.Sizing.SizingRecommendation.VCPUsPerInstance,
		ar.Sizing.SizingRecommendation.MemoryPerInstance, ar.Sizing.SizingRecommendation.EstimatedTimeInMinForImport)
}

func (ar *AssessmentReport) GetTotalTableRowCount() int64 {
	if ar.TableIndexStats == nil {
		return -1
	}
	var totalTableRowCount int64
	for _, stat := range *ar.TableIndexStats {
		if !stat.IsIndex {
			totalTableRowCount += utils.SafeDereferenceInt64(stat.RowCount)
		}
	}
	return totalTableRowCount
}

func (ar *AssessmentReport) GetTotalTableSize() int64 {
	if ar.TableIndexStats == nil {
		return -1
	}
	var totalTableSize int64
	for _, stat := range *ar.TableIndexStats {
		if !stat.IsIndex {
			totalTableSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
		}
	}
	return totalTableSize
}

func (ar *AssessmentReport) GetTotalIndexSize() int64 {
	if ar.TableIndexStats == nil {
		return -1
	}
	var totalIndexSize int64
	for _, stat := range *ar.TableIndexStats {
		if stat.IsIndex {
			totalIndexSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
		}
	}
	return totalIndexSize
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

	// Execute the command using full path to yb-voyager
	startTime := time.Now()
	voyagerPath := findYbVoyagerPath()
	cmd := exec.CommandContext(ctx, voyagerPath, cmdArgs...)
	cmd.Dir = filepath.Dir(configPath) // Set working directory to config file directory

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Parse and analyze the output
	rawOutput := string(output)
	summary, structuredData := parseVoyagerOutput(command, rawOutput, err)

	// Create enhanced response
	result := map[string]interface{}{
		"execution": map[string]interface{}{
			"command":     "yb-voyager " + strings.Join(cmdArgs, " "),
			"config_path": configPath,
			"success":     err == nil,
			"duration":    formatDuration(duration),
			"timestamp":   time.Now().Format(time.RFC3339),
		},
		"summary":         summary,
		"structured_data": structuredData,
		"raw_output":      rawOutput,
	}

	if err != nil {
		result["execution"].(map[string]interface{})["error"] = err.Error()
		result["execution"].(map[string]interface{})["exit_code"] = getExitCode(err)
	}

	// Create rich markdown content for display
	var markdownContent strings.Builder
	markdownContent.WriteString("# YB Voyager Command Execution\n\n")
	markdownContent.WriteString("## Summary\n")
	markdownContent.WriteString(summary + "\n\n")
	markdownContent.WriteString("## Command Details\n")
	markdownContent.WriteString(fmt.Sprintf("- **Command**: %s\n", "yb-voyager "+strings.Join(cmdArgs, " ")))
	markdownContent.WriteString(fmt.Sprintf("- **Duration**: %s\n", formatDuration(duration)))

	status := "✅ Success"
	if err != nil {
		status = "❌ Failed"
	}
	markdownContent.WriteString(fmt.Sprintf("- **Status**: %s\n", status))
	markdownContent.WriteString(fmt.Sprintf("- **Timestamp**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	markdownContent.WriteString("## Raw Output\n```\n")
	markdownContent.WriteString(rawOutput)
	markdownContent.WriteString("\n```\n\n")

	markdownContent.WriteString("## Structured Data\n```json\n")
	jsonData, _ := json.MarshalIndent(structuredData, "", "  ")
	markdownContent.WriteString(string(jsonData))
	markdownContent.WriteString("\n```\n")

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(markdownContent.String())},
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

	// Execute the command using full path to yb-voyager
	startTime := time.Now()
	voyagerPath := findYbVoyagerPath()
	cmd := exec.CommandContext(ctx, voyagerPath, cmdArgs...)
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Parse and analyze the output
	rawOutput := string(output)
	summary, structuredData := parseVoyagerOutput(command, rawOutput, err)

	// Create enhanced response
	result := map[string]interface{}{
		"execution": map[string]interface{}{
			"command":   "yb-voyager " + strings.Join(cmdArgs, " "),
			"success":   err == nil,
			"duration":  formatDuration(duration),
			"timestamp": time.Now().Format(time.RFC3339),
		},
		"summary":         summary,
		"structured_data": structuredData,
		"raw_output":      rawOutput,
	}

	if err != nil {
		result["execution"].(map[string]interface{})["error"] = err.Error()
		result["execution"].(map[string]interface{})["exit_code"] = getExitCode(err)
	}

	// Create rich markdown content for display
	var markdownContent strings.Builder
	markdownContent.WriteString("# YB Voyager Command Execution\n\n")
	markdownContent.WriteString("## Summary\n")
	markdownContent.WriteString(summary + "\n\n")
	markdownContent.WriteString("## Command Details\n")
	markdownContent.WriteString(fmt.Sprintf("- **Command**: %s\n", "yb-voyager "+strings.Join(cmdArgs, " ")))
	markdownContent.WriteString(fmt.Sprintf("- **Duration**: %s\n", formatDuration(duration)))

	status := "✅ Success"
	if err != nil {
		status = "❌ Failed"
	}
	markdownContent.WriteString(fmt.Sprintf("- **Status**: %s\n", status))
	markdownContent.WriteString(fmt.Sprintf("- **Timestamp**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	markdownContent.WriteString("## Raw Output\n```\n")
	markdownContent.WriteString(rawOutput)
	markdownContent.WriteString("\n```\n\n")

	markdownContent.WriteString("## Structured Data\n```json\n")
	jsonData, _ := json.MarshalIndent(structuredData, "", "  ")
	markdownContent.WriteString(string(jsonData))
	markdownContent.WriteString("\n```\n")

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(markdownContent.String())},
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

// getAssessmentReportResource returns the migration assessment report as a resource
func getAssessmentReportResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	exportDir := extractExportDirFromURI(uri)
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir not found in URI")
	}

	// Check for assessment report files
	assessmentReportDir := filepath.Join(exportDir, "assessment", "reports")
	jsonReportPath := filepath.Join(assessmentReportDir, "migration_assessment_report.json")
	htmlReportPath := filepath.Join(assessmentReportDir, "migration_assessment_report.html")

	result := map[string]interface{}{
		"export_dir":       exportDir,
		"assessment_dir":   assessmentReportDir,
		"timestamp":        time.Now().Format(time.RFC3339),
		"json_report_path": jsonReportPath,
		"html_report_path": htmlReportPath,
		"json_exists":      utils.FileOrFolderExists(jsonReportPath),
		"html_exists":      utils.FileOrFolderExists(htmlReportPath),
	}

	// Try to read and parse the JSON assessment report using YB Voyager's parser
	if utils.FileOrFolderExists(jsonReportPath) {
		assessmentReport, err := parseAssessmentReport(jsonReportPath)
		if err != nil {
			result["parse_error"] = err.Error()
			// Fallback to raw content if parsing fails
			if jsonData, readErr := ioutil.ReadFile(jsonReportPath); readErr == nil {
				result["raw_content"] = string(jsonData)
			}
		} else {
			// Use the properly parsed AssessmentReport struct
			result["assessment_report"] = assessmentReport

			// Extract key metrics for easy LLM access
			result["voyager_version"] = assessmentReport.VoyagerVersion
			result["migration_complexity"] = assessmentReport.MigrationComplexity
			result["migration_complexity_explanation"] = assessmentReport.MigrationComplexityExplanation
			result["total_issues"] = len(assessmentReport.Issues)

			// Categorize issues by impact and category
			impactCounts := make(map[string]int)
			categoryCounts := make(map[string]int)
			for _, issue := range assessmentReport.Issues {
				impactCounts[issue.Impact]++
				categoryCounts[issue.Category]++
			}
			result["issues_by_impact"] = impactCounts
			result["issues_by_category"] = categoryCounts

			// Extract schema summary
			result["schema_summary"] = assessmentReport.SchemaSummary

			// Extract sizing recommendations if available
			if assessmentReport.Sizing != nil {
				result["sizing_recommendations"] = assessmentReport.Sizing
				result["cluster_sizing_recommendation"] = assessmentReport.GetClusterSizingRecommendation()

				shardedTables, err := assessmentReport.GetShardedTablesRecommendation()
				if err == nil {
					result["sharded_tables"] = shardedTables
				}

				colocatedTables, err := assessmentReport.GetColocatedTablesRecommendation()
				if err == nil {
					result["colocated_tables"] = colocatedTables
				}
			}

			// Extract table and index statistics
			if assessmentReport.TableIndexStats != nil {
				result["total_table_row_count"] = assessmentReport.GetTotalTableRowCount()
				result["total_table_size"] = assessmentReport.GetTotalTableSize()
				result["total_index_size"] = assessmentReport.GetTotalIndexSize()
				result["table_index_stats"] = assessmentReport.TableIndexStats
			}

			// Extract notes
			result["notes"] = assessmentReport.Notes

			// Extract target DB version
			if assessmentReport.TargetDBVersion != nil {
				result["target_db_version"] = assessmentReport.TargetDBVersion.String()
			}
		}
	} else {
		result["status"] = "Assessment report not found. Run 'assess-migration' command first."
	}

	// Try to read and parse the JSON schema analysis report using YB Voyager's parser
	if utils.FileOrFolderExists(jsonReportPath) {
		schemaReport, err := parseSchemaAnalysisReport(jsonReportPath)
		if err != nil {
			result["parse_error"] = err.Error()
			// Fallback to raw content if parsing fails
			if jsonData, readErr := ioutil.ReadFile(jsonReportPath); readErr == nil {
				result["raw_content"] = string(jsonData)
			}
		} else {
			// Use the properly parsed SchemaReport struct
			result["schema_analysis_report"] = schemaReport

			// Extract key metrics for easy LLM access
			result["voyager_version"] = schemaReport.VoyagerVersion
			result["total_issues"] = len(schemaReport.Issues)

			// Extract target DB version
			if schemaReport.TargetDBVersion != nil {
				result["target_db_version"] = schemaReport.TargetDBVersion.String()
			}

			// Categorize issues by type and impact
			typeCounts := make(map[string]int)
			impactCounts := make(map[string]int)
			objectTypeCounts := make(map[string]int)

			for _, issue := range schemaReport.Issues {
				typeCounts[issue.IssueType]++
				impactCounts[issue.Impact]++
				objectTypeCounts[issue.ObjectType]++
			}
			result["issues_by_type"] = typeCounts
			result["issues_by_impact"] = impactCounts
			result["issues_by_object_type"] = objectTypeCounts

			// Extract schema summary
			result["schema_summary"] = schemaReport.SchemaSummary

			// Extract database objects summary with detailed breakdown
			if len(schemaReport.SchemaSummary.DBObjects) > 0 {
				result["total_db_objects"] = len(schemaReport.SchemaSummary.DBObjects)

				objectCounts := make(map[string]map[string]interface{})
				totalObjects := 0
				totalInvalidObjects := 0

				for _, obj := range schemaReport.SchemaSummary.DBObjects {
					objectCounts[obj.ObjectType] = map[string]interface{}{
						"total_count":   obj.TotalCount,
						"invalid_count": obj.InvalidCount,
						"valid_count":   obj.TotalCount - obj.InvalidCount,
						"object_names":  obj.ObjectNames,
						"details":       obj.Details,
					}
					totalObjects += obj.TotalCount
					totalInvalidObjects += obj.InvalidCount
				}

				result["objects_by_type"] = objectCounts
				result["total_objects_count"] = totalObjects
				result["total_invalid_objects"] = totalInvalidObjects
				result["total_valid_objects"] = totalObjects - totalInvalidObjects
			}

			// Extract database information
			result["database_name"] = schemaReport.SchemaSummary.DBName
			result["database_version"] = schemaReport.SchemaSummary.DBVersion
			result["schema_names"] = schemaReport.SchemaSummary.SchemaNames
			result["notes"] = schemaReport.SchemaSummary.Notes
		}
	} else {
		result["status"] = "Schema analysis report not found. Run 'analyze-schema' command first."
	}

	// Get file metadata if reports exist
	if utils.FileOrFolderExists(jsonReportPath) {
		if stat, err := os.Stat(jsonReportPath); err == nil {
			result["json_file_size"] = stat.Size()
			result["json_modified"] = stat.ModTime().Format(time.RFC3339)
		}
	}
	if utils.FileOrFolderExists(htmlReportPath) {
		if stat, err := os.Stat(htmlReportPath); err == nil {
			result["html_file_size"] = stat.Size()
			result["html_modified"] = stat.ModTime().Format(time.RFC3339)
		}
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

// getSchemaAnalysisReportResource returns the schema analysis report as a resource
func getSchemaAnalysisReportResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	exportDir := extractExportDirFromURI(uri)
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir not found in URI")
	}

	// Check for schema analysis report files
	reportsDir := filepath.Join(exportDir, "reports")
	jsonReportPath := filepath.Join(reportsDir, "schema_analysis_report.json")
	htmlReportPath := filepath.Join(reportsDir, "schema_analysis_report.html")

	result := map[string]interface{}{
		"export_dir":       exportDir,
		"reports_dir":      reportsDir,
		"timestamp":        time.Now().Format(time.RFC3339),
		"json_report_path": jsonReportPath,
		"html_report_path": htmlReportPath,
		"json_exists":      utils.FileOrFolderExists(jsonReportPath),
		"html_exists":      utils.FileOrFolderExists(htmlReportPath),
	}

	// Try to read and parse the JSON schema analysis report using YB Voyager's parser
	if utils.FileOrFolderExists(jsonReportPath) {
		schemaReport, err := parseSchemaAnalysisReport(jsonReportPath)
		if err != nil {
			result["parse_error"] = err.Error()
			// Fallback to raw content if parsing fails
			if jsonData, readErr := ioutil.ReadFile(jsonReportPath); readErr == nil {
				result["raw_content"] = string(jsonData)
			}
		} else {
			// Use the properly parsed SchemaReport struct
			result["schema_analysis_report"] = schemaReport

			// Extract key metrics for easy LLM access
			result["voyager_version"] = schemaReport.VoyagerVersion
			result["total_issues"] = len(schemaReport.Issues)

			// Extract target DB version
			if schemaReport.TargetDBVersion != nil {
				result["target_db_version"] = schemaReport.TargetDBVersion.String()
			}

			// Categorize issues by type and impact
			typeCounts := make(map[string]int)
			impactCounts := make(map[string]int)
			objectTypeCounts := make(map[string]int)

			for _, issue := range schemaReport.Issues {
				typeCounts[issue.IssueType]++
				impactCounts[issue.Impact]++
				objectTypeCounts[issue.ObjectType]++
			}
			result["issues_by_type"] = typeCounts
			result["issues_by_impact"] = impactCounts
			result["issues_by_object_type"] = objectTypeCounts

			// Extract schema summary
			result["schema_summary"] = schemaReport.SchemaSummary

			// Extract database objects summary with detailed breakdown
			if len(schemaReport.SchemaSummary.DBObjects) > 0 {
				result["total_db_objects"] = len(schemaReport.SchemaSummary.DBObjects)

				objectCounts := make(map[string]map[string]interface{})
				totalObjects := 0
				totalInvalidObjects := 0

				for _, obj := range schemaReport.SchemaSummary.DBObjects {
					objectCounts[obj.ObjectType] = map[string]interface{}{
						"total_count":   obj.TotalCount,
						"invalid_count": obj.InvalidCount,
						"valid_count":   obj.TotalCount - obj.InvalidCount,
						"object_names":  obj.ObjectNames,
						"details":       obj.Details,
					}
					totalObjects += obj.TotalCount
					totalInvalidObjects += obj.InvalidCount
				}

				result["objects_by_type"] = objectCounts
				result["total_objects_count"] = totalObjects
				result["total_invalid_objects"] = totalInvalidObjects
				result["total_valid_objects"] = totalObjects - totalInvalidObjects
			}

			// Extract database information
			result["database_name"] = schemaReport.SchemaSummary.DBName
			result["database_version"] = schemaReport.SchemaSummary.DBVersion
			result["schema_names"] = schemaReport.SchemaSummary.SchemaNames
			result["notes"] = schemaReport.SchemaSummary.Notes
		}
	} else {
		result["status"] = "Schema analysis report not found. Run 'analyze-schema' command first."
	}

	// Get file metadata if reports exist
	if utils.FileOrFolderExists(jsonReportPath) {
		if stat, err := os.Stat(jsonReportPath); err == nil {
			result["json_file_size"] = stat.Size()
			result["json_modified"] = stat.ModTime().Format(time.RFC3339)
		}
	}
	if utils.FileOrFolderExists(htmlReportPath) {
		if stat, err := os.Stat(htmlReportPath); err == nil {
			result["html_file_size"] = stat.Size()
			result["html_modified"] = stat.ModTime().Format(time.RFC3339)
		}
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

// getSchemaFilesResource returns information about exported schema files
func getSchemaFilesResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	exportDir := extractExportDirFromURI(uri)
	if exportDir == "" {
		return nil, fmt.Errorf("export_dir not found in URI")
	}

	// Check for schema files
	schemaDir := filepath.Join(exportDir, "schema")

	result := map[string]interface{}{
		"export_dir": exportDir,
		"schema_dir": schemaDir,
		"timestamp":  time.Now().Format(time.RFC3339),
		"dir_exists": utils.FileOrFolderExists(schemaDir),
	}

	if utils.FileOrFolderExists(schemaDir) {
		files, err := ioutil.ReadDir(schemaDir)
		if err != nil {
			result["read_error"] = err.Error()
		} else {
			var schemaFiles []map[string]interface{}
			var totalSize int64

			for _, file := range files {
				if !file.IsDir() {
					fileInfo := map[string]interface{}{
						"name":     file.Name(),
						"size":     file.Size(),
						"modified": file.ModTime().Format(time.RFC3339),
					}

					// Determine file type based on extension and name
					fileName := file.Name()
					if strings.HasSuffix(fileName, ".sql") {
						fileInfo["type"] = "sql"

						// Categorize SQL files by object type
						if strings.Contains(fileName, "table") {
							fileInfo["object_type"] = "table"
						} else if strings.Contains(fileName, "view") {
							fileInfo["object_type"] = "view"
						} else if strings.Contains(fileName, "function") {
							fileInfo["object_type"] = "function"
						} else if strings.Contains(fileName, "procedure") {
							fileInfo["object_type"] = "procedure"
						} else if strings.Contains(fileName, "trigger") {
							fileInfo["object_type"] = "trigger"
						} else if strings.Contains(fileName, "index") {
							fileInfo["object_type"] = "index"
						} else if strings.Contains(fileName, "sequence") {
							fileInfo["object_type"] = "sequence"
						} else {
							fileInfo["object_type"] = "other"
						}
					} else {
						fileInfo["type"] = "other"
					}

					schemaFiles = append(schemaFiles, fileInfo)
					totalSize += file.Size()
				}
			}

			result["schema_files"] = schemaFiles
			result["total_files"] = len(schemaFiles)
			result["total_size"] = totalSize

			// Group files by object type
			objectTypes := make(map[string][]string)
			for _, file := range schemaFiles {
				if objType, ok := file["object_type"].(string); ok {
					if fileName, ok := file["name"].(string); ok {
						objectTypes[objType] = append(objectTypes[objType], fileName)
					}
				}
			}
			result["files_by_object_type"] = objectTypes
		}
	} else {
		result["status"] = "Schema directory not found. Run 'export-schema' command first."
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

// findYbVoyagerPath finds the path to yb-voyager executable
func findYbVoyagerPath() string {
	// First try to find in PATH
	if path, err := exec.LookPath("yb-voyager"); err == nil {
		return path
	}

	// Fallback to common locations
	commonPaths := []string{
		"/Users/sanyamsinghal/go/bin/yb-voyager",
		"/usr/local/bin/yb-voyager",
		"/usr/bin/yb-voyager",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If not found, return default and let it fail with proper error
	return "yb-voyager"
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

// Enhanced output parsing and formatting functions

// parseVoyagerOutput analyzes yb-voyager command output and extracts structured information
func parseVoyagerOutput(command, rawOutput string, execErr error) (summary string, structuredData map[string]interface{}) {
	structuredData = make(map[string]interface{})

	// Parse based on command type
	switch command {
	case "assess-migration":
		return parseAssessmentOutput(rawOutput, execErr)
	case "export", "export-schema":
		return parseExportOutput(rawOutput, execErr)
	case "import", "import-schema":
		return parseImportOutput(rawOutput, execErr)
	default:
		return parseGenericOutput(rawOutput, execErr)
	}
}

// parseAssessmentOutput parses assess-migration command output
func parseAssessmentOutput(output string, execErr error) (string, map[string]interface{}) {
	data := make(map[string]interface{})

	if execErr != nil {
		summary := fmt.Sprintf("❌ Assessment failed: %v", execErr)
		data["status"] = "failed"
		data["error"] = execErr.Error()
		return summary, data
	}

	// Extract key information from assessment output
	lines := strings.Split(output, "\n")
	var totalTables, totalIndexes, totalConstraints int
	var issues []string
	var missingDeps []string

	// Detect 'Missing dependencies' block
	parsingDeps := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)

		if strings.HasPrefix(lower, "missing dependencies") {
			parsingDeps = true
			continue
		}
		if parsingDeps {
			// blank line or end stops parsing deps
			if trim == "" {
				parsingDeps = false
				continue
			}
			missingDeps = append(missingDeps, trim)
			continue
		}

		// Parse table count
		if matched, _ := regexp.MatchString(`(\d+)\s+tables`, trim); matched {
			re := regexp.MustCompile(`(\d+)\s+tables`)
			if matches := re.FindStringSubmatch(trim); len(matches) > 1 {
				totalTables, _ = strconv.Atoi(matches[1])
			}
		}
		// Parse index count
		if matched, _ := regexp.MatchString(`(\d+)\s+indexes`, trim); matched {
			re := regexp.MustCompile(`(\d+)\s+indexes`)
			if matches := re.FindStringSubmatch(trim); len(matches) > 1 {
				totalIndexes, _ = strconv.Atoi(matches[1])
			}
		}
		if strings.Contains(lower, "issue") || strings.Contains(lower, "warning") {
			issues = append(issues, trim)
		}
	}

	data["tables_count"] = totalTables
	data["indexes_count"] = totalIndexes
	data["constraints_count"] = totalConstraints
	data["issues"] = issues
	if len(missingDeps) > 0 {
		data["missing_dependencies"] = missingDeps
		data["status"] = "failed"
		summary := "❌ Assessment failed due to missing dependencies:\n- " + strings.Join(missingDeps, "\n- ")
		return summary, data
	}

	data["status"] = "completed"
	summary := fmt.Sprintf("✅ Assessment completed successfully\n📊 Found %d tables, %d indexes", totalTables, totalIndexes)
	if len(issues) > 0 {
		summary += fmt.Sprintf("\n⚠️  %d issues found", len(issues))
	}
	return summary, data
}

// parseExportOutput parses export command output
func parseExportOutput(output string, execErr error) (string, map[string]interface{}) {
	data := make(map[string]interface{})

	if execErr != nil {
		summary := fmt.Sprintf("❌ Export failed: %v", execErr)
		data["status"] = "failed"
		data["error"] = execErr.Error()
		return summary, data
	}

	// Look for completion indicators
	var summary string
	if strings.Contains(output, "export completed") || strings.Contains(output, "successfully") {
		data["status"] = "completed"
		summary = "✅ Export completed successfully"
	} else {
		data["status"] = "in_progress"
		summary = "🔄 Export in progress..."
	}

	return summary, data
}

// parseImportOutput parses import command output
func parseImportOutput(output string, execErr error) (string, map[string]interface{}) {
	data := make(map[string]interface{})

	if execErr != nil {
		summary := fmt.Sprintf("❌ Import failed: %v", execErr)
		data["status"] = "failed"
		data["error"] = execErr.Error()
		return summary, data
	}

	var summary string
	if strings.Contains(output, "import completed") || strings.Contains(output, "successfully") {
		data["status"] = "completed"
		summary = "✅ Import completed successfully"
	} else {
		data["status"] = "in_progress"
		summary = "🔄 Import in progress..."
	}

	return summary, data
}

// parseGenericOutput parses generic command output
func parseGenericOutput(output string, execErr error) (string, map[string]interface{}) {
	data := make(map[string]interface{})

	if execErr != nil {
		summary := fmt.Sprintf("❌ Command failed: %v", execErr)
		data["status"] = "failed"
		data["error"] = execErr.Error()
		return summary, data
	}

	data["status"] = "completed"
	summary := "✅ Command completed successfully"

	return summary, data
}

// getExitCode extracts exit code from error
func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}

	return 1
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
}

// getSchemaAnalysisResource returns schema analysis as a resource (kept for backward compatibility)
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
		"note":       "This resource is deprecated. Use voyager://assessment-report/{export_dir} or voyager://schema-analysis-report/{export_dir} instead.",
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
