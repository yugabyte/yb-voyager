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

	log "github.com/sirupsen/logrus"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

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

	// Parse command - handle hierarchical commands like "export data" or "import schema"
	commandParts := strings.Fields(command)

	// Build the command arguments starting with parsed command parts
	cmdArgs := append(commandParts, "--config-file", configPath)

	// Add additional arguments if provided
	if additionalArgs != "" {
		args := strings.Fields(additionalArgs)
		cmdArgs = append(cmdArgs, args...)
	}

	// Execute the command using full path to yb-voyager
	startTime := time.Now()
	voyagerPath := findYbVoyagerPath()
	cmd := exec.CommandContext(ctx, voyagerPath, cmdArgs...)
	cmd.Env = buildEnvWithExtraPath()
	cmd.Dir = filepath.Dir(configPath) // Set working directory to config file directory

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Create standardized response
	rawOutput := string(output)
	summary, status, _ := createCommandResponse(command, rawOutput, err)

	result := map[string]interface{}{
		"execution": map[string]interface{}{
			"command":     "yb-voyager " + strings.Join(cmdArgs, " "),
			"config_path": configPath,
			"success":     err == nil,
			"duration":    formatDuration(duration),
			"timestamp":   time.Now().Format(time.RFC3339),
		},
		"summary":    summary,
		"status":     status,
		"raw_output": rawOutput,
	}

	if err != nil {
		result["execution"].(map[string]interface{})["error"] = err.Error()
		result["execution"].(map[string]interface{})["exit_code"] = getExitCode(err)
	}

	// Return clean JSON response
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
		IsError: err != nil,
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

	// Parse command - handle hierarchical commands like "export data" or "import schema"
	commandParts := strings.Fields(command)

	// Build the command arguments starting with parsed command parts
	cmdArgs := commandParts

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
	cmd.Env = buildEnvWithExtraPath()
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Create standardized response
	rawOutput := string(output)
	summary, status, _ := createCommandResponse(command, rawOutput, err)

	result := map[string]interface{}{
		"execution": map[string]interface{}{
			"command":   "yb-voyager " + strings.Join(cmdArgs, " "),
			"success":   err == nil,
			"duration":  formatDuration(duration),
			"timestamp": time.Now().Format(time.RFC3339),
		},
		"summary":    summary,
		"status":     status,
		"raw_output": rawOutput,
	}

	if err != nil {
		result["execution"].(map[string]interface{})["error"] = err.Error()
		result["execution"].(map[string]interface{})["exit_code"] = getExitCode(err)
	}

	// Return clean JSON response
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
		IsError: err != nil,
	}, nil
}

// executeVoyagerCommandSmart intelligently executes YB Voyager commands
// It automatically chooses between config-file and individual parameter methods
func executeVoyagerCommandSmart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rawCommand, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'command': %v", err)), nil
	}

	// Normalize the command to handle common variations
	command := normalizeVoyagerCommand(rawCommand)

	configPath := req.GetString("config_path", "")
	exportDir := req.GetString("export_dir", "")
	additionalArgs := req.GetString("additional_args", "")

	var methodUsed string
	var result *mcp.CallToolResult

	// CASE 1: Both config_path and export_dir provided
	if configPath != "" && exportDir != "" {
		// Prioritize config_path but validate it exists
		if utils.FileOrFolderExists(configPath) {
			// Use config file method
			configReq := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "execute_voyager_with_config",
					Arguments: map[string]interface{}{
						"command":         command,
						"config_path":     configPath,
						"additional_args": additionalArgs,
					},
				},
			}
			result, err = executeVoyagerWithConfig(ctx, configReq)
			methodUsed = "config_file"
		} else {
			// Config file doesn't exist, fall back to export_dir method
			legacyReq := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "execute_voyager_command",
					Arguments: map[string]interface{}{
						"command":    command,
						"export_dir": exportDir,
						"args":       additionalArgs,
					},
				},
			}
			result, err = executeVoyagerCommand(ctx, legacyReq)
			methodUsed = "export_dir_fallback"
		}
	} else if configPath != "" {
		// CASE 2: Only config_path provided
		configReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "execute_voyager_with_config",
				Arguments: map[string]interface{}{
					"command":         command,
					"config_path":     configPath,
					"additional_args": additionalArgs,
				},
			},
		}
		result, err = executeVoyagerWithConfig(ctx, configReq)
		methodUsed = "config_file"
	} else if exportDir != "" {
		// CASE 3: Only export_dir provided
		legacyReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "execute_voyager_command",
				Arguments: map[string]interface{}{
					"command":    command,
					"export_dir": exportDir,
					"args":       additionalArgs,
				},
			},
		}
		result, err = executeVoyagerCommand(ctx, legacyReq)
		methodUsed = "export_dir"
	} else {
		// CASE 4: Neither provided
		return mcp.NewToolResultError("Either 'config_path' or 'export_dir' must be provided. Config file method is preferred."), nil
	}

	if err != nil {
		return result, err
	}

	// Enhance the result with method information and guidance
	if result != nil && !result.IsError && len(result.Content) > 0 {
		// Parse the existing JSON response to add method information
		content := result.Content[0]
		if textContent, ok := content.(mcp.TextContent); ok {
			var responseData map[string]interface{}
			if json.Unmarshal([]byte(textContent.Text), &responseData) == nil {
				// Add method information to the execution section
				if execution, ok := responseData["execution"].(map[string]interface{}); ok {
					execution["method_used"] = methodUsed
					execution["command_executed"] = command
					if rawCommand != command {
						execution["command_normalized"] = fmt.Sprintf("'%s' → '%s'", rawCommand, command)
					}
					switch methodUsed {
					case "config_file":
						execution["recommendation"] = "✅ Used preferred config file method"
					case "export_dir":
						execution["recommendation"] = "💡 Consider using config file method for better consistency"
					case "export_dir_fallback":
						execution["recommendation"] = "⚠️ Config file not found, fell back to export_dir method"
					}
				}

				// Re-encode the enhanced response
				if enhancedJSON, err := json.MarshalIndent(responseData, "", "  "); err == nil {
					result.Content[0] = mcp.NewTextContent(string(enhancedJSON))
				}
			}
		}
	}

	return result, nil
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

			// Extract key metrics for easy LLM access using struct fields
			result["voyager_version"] = assessmentReport.VoyagerVersion
			result["migration_complexity"] = assessmentReport.MigrationComplexity
			result["migration_complexity_explanation"] = assessmentReport.MigrationComplexityExplanation
			result["total_issues"] = len(assessmentReport.Issues)

			// Categorize issues by impact and category using struct fields
			impactCounts := make(map[string]int)
			categoryCounts := make(map[string]int)
			for _, issue := range assessmentReport.Issues {
				impactCounts[issue.Impact]++
				categoryCounts[issue.Category]++
			}
			result["issues_by_impact"] = impactCounts
			result["issues_by_category"] = categoryCounts

			// Extract schema summary using struct fields
			result["schema_summary"] = assessmentReport.SchemaSummary

			// Extract sizing recommendations if available using struct methods
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

			// Extract table and index statistics using struct methods
			if assessmentReport.TableIndexStats != nil {
				result["total_table_row_count"] = assessmentReport.GetTotalTableRowCount()
				result["total_table_size"] = assessmentReport.GetTotalTableSize()
				result["total_index_size"] = assessmentReport.GetTotalIndexSize()
				result["table_index_stats"] = assessmentReport.TableIndexStats
			}

			// Extract notes using struct fields
			result["notes"] = assessmentReport.Notes

			// Extract target DB version using struct fields
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

			// Extract key metrics for easy LLM access using struct fields
			result["voyager_version"] = schemaReport.VoyagerVersion
			result["total_issues"] = len(schemaReport.Issues)

			// Extract target DB version using struct fields
			if schemaReport.TargetDBVersion != nil {
				result["target_db_version"] = schemaReport.TargetDBVersion.String()
			}

			// Categorize issues by type and impact using struct fields
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

			// Extract schema summary using struct fields
			result["schema_summary"] = schemaReport.SchemaSummary

			// Extract database objects summary with detailed breakdown using struct fields
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

			// Extract database information using struct fields
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

// getAssessmentReport returns the complete assessment report as structured JSON
func getAssessmentReport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exportDir, err := req.RequireString("export_dir")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'export_dir': %v", err)), nil
	}

	// Try to parse the assessment report using YB Voyager's native parser
	assessmentReportDir := filepath.Join(exportDir, "assessment", "reports")
	jsonReportPath := filepath.Join(assessmentReportDir, "migration_assessment_report.json")

	if !utils.FileOrFolderExists(jsonReportPath) {
		return mcp.NewToolResultError(fmt.Sprintf("Assessment report not found at %s. Run 'assess-migration' command first.", jsonReportPath)), nil
	}

	// Parse the assessment report using the native parser
	assessmentReport, err := parseAssessmentReport(jsonReportPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse assessment report: %v", err)), nil
	}

	// Return the complete structured report as JSON
	result := map[string]interface{}{
		"export_dir":  exportDir,
		"report_path": jsonReportPath,
		"timestamp":   time.Now().Format(time.RFC3339),
		"report":      assessmentReport,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
	}, nil
}

// getSchemaAnalysisReport returns the complete schema analysis report as structured JSON
func getSchemaAnalysisReport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exportDir, err := req.RequireString("export_dir")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'export_dir': %v", err)), nil
	}

	// Try to parse the schema analysis report using YB Voyager's native parser
	reportsDir := filepath.Join(exportDir, "reports")
	jsonReportPath := filepath.Join(reportsDir, "schema_analysis_report.json")

	if !utils.FileOrFolderExists(jsonReportPath) {
		return mcp.NewToolResultError(fmt.Sprintf("Schema analysis report not found at %s. Run 'analyze-schema' command first.", jsonReportPath)), nil
	}

	// Parse the schema analysis report using the native parser
	schemaReport, err := parseSchemaAnalysisReport(jsonReportPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse schema analysis report: %v", err)), nil
	}

	// Return the complete structured report as JSON
	result := map[string]interface{}{
		"export_dir":  exportDir,
		"report_path": jsonReportPath,
		"timestamp":   time.Now().Format(time.RFC3339),
		"report":      schemaReport,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonResult))},
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
			// Join all parts after the resource type to get the full export directory path
			return strings.Join(parts[1:], "/")
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

// Helper functions for command execution

// createCommandResponse creates a standardized response for YB Voyager command execution
func createCommandResponse(command string, rawOutput string, execErr error) (string, string, map[string]interface{}) {
	status := "completed"
	summary := "Command completed successfully"

	if execErr != nil {
		status = "failed"
		summary = fmt.Sprintf("Command failed: %v", execErr)
	}

	data := map[string]interface{}{
		"command":    command,
		"status":     status,
		"raw_output": rawOutput,
	}

	if execErr != nil {
		data["error"] = execErr.Error()
		data["exit_code"] = getExitCode(execErr)
	}

	return summary, status, data
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

// buildEnvWithExtraPath constructs an environment slice where PATH is extended so
// that helper binaries (pg_dump, pg_restore, psql) are discoverable even when
// MCP is started by a GUI process that inherits a minimal PATH.
//
// Precedence order (left-to-right):
//  1. VOYAGER_EXTRA_PATH env var supplied by the user.
//  2. Hard-coded common locations.
//  3. Original PATH from the parent process.
func buildEnvWithExtraPath() []string {
	basePath := os.Getenv("PATH")

	// User-supplied override (may contain multiple ':'-separated dirs)
	extra := os.Getenv("VOYAGER_EXTRA_PATH")

	defaults := []string{
		"/opt/homebrew/bin",                   // Homebrew (Apple Silicon)
		"/opt/homebrew/opt/postgresql@16/bin", // Brew keg for pg16 utilities (Apple Silicon)
		"/usr/local/opt/postgresql@16/bin",    // Brew keg for pg16 utilities (Intel)
		"/usr/local/bin",                      // Common fallback
	}

	var parts []string
	if extra != "" {
		parts = append(parts, extra)
	}
	parts = append(parts, defaults...)
	parts = append(parts, basePath)

	finalPath := strings.Join(parts, ":")

	// For debugging: find the log file at ~/Library/Logs/Claude/mcp-server-yb-voyager.log
	log.Infof("Original PATH from MCP server environment: %s", basePath)
	log.Infof("Constructed PATH for yb-voyager command: %s", finalPath)

	var newEnv []string
	// Copy original environment, but replace PATH.
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
		newEnv = append(newEnv, e)
	}
	newEnv = append(newEnv, "PATH="+finalPath)

	return newEnv
}

// normalizeVoyagerCommand maps common command variations to correct YB Voyager commands
func normalizeVoyagerCommand(input string) string {
	// Normalize input: lowercase, trim, replace separators with spaces
	normalized := strings.ToLower(strings.TrimSpace(input))
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")

	// Command mapping for common variations
	commandMap := map[string]string{
		// Export commands
		"export schema": "export schema",
		"exportschema":  "export schema",
		"schema export": "export schema",
		"export data":   "export data",
		"exportdata":    "export data",
		"data export":   "export data",

		// Import commands
		"import schema": "import schema",
		"importschema":  "import schema",
		"schema import": "import schema",
		"import data":   "import data",
		"importdata":    "import data",
		"data import":   "import data",

		// Assessment commands
		"assess migration": "assess-migration",
		"assessmigration":  "assess-migration",
		"assessment":       "assess-migration",

		// Analysis commands
		"analyze schema":  "analyze-schema",
		"analyseschema":   "analyze-schema",
		"schema analysis": "analyze-schema",

		// Status commands
		"get status":       "get data-migration-report",
		"migration status": "get data-migration-report",
		"status":           "get data-migration-report",

		// End commands
		"end migration":    "end migration",
		"endmigration":     "end migration",
		"finish migration": "end migration",

		// Version
		"version":     "version",
		"get version": "version",
	}

	// Return mapped command or original if no mapping found
	if mapped, exists := commandMap[normalized]; exists {
		return mapped
	}
	return input // Return original if no mapping found
}
