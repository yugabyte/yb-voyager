/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var migrationDir string
var sourceConnString string

// paths for installed gather-assessment-metadata scripts
const (
	pgGatherScriptsInstalledDir     = "/etc/yb-voyager/gather-assessment-metadata/postgresql"
	oracleGatherScriptsInstalledDir = "/etc/yb-voyager/gather-assessment-metadata/oracle"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new migration project",
	Long: `Initialize a new YB Voyager migration project.

Creates a migration directory with the necessary structure and configuration
to begin migrating your database to YugabyteDB.`,

	Run: func(cmd *cobra.Command, args []string) {
		runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&migrationDir, "migration-dir", "",
		"path to the migration directory (will be created if it doesn't exist)")
	initCmd.MarkFlagRequired("migration-dir")
	initCmd.Flags().StringVar(&sourceConnString, "source-db-connection-string", "",
		"source database connection string (e.g. postgresql://user:password@host:5432/dbname). If provided, skips the interactive prompt.")
}

func runInit() {
	// Resolve to absolute path
	var err error
	migrationDir, err = filepath.Abs(migrationDir)
	if err != nil {
		utils.ErrExit("failed to resolve migration-dir path: %v", err)
	}

	configFilePath := filepath.Join(migrationDir, "config.yaml")
	exportDirPath := filepath.Join(migrationDir, "export-dir")

	// Idempotency guard: fail if config.yaml already exists
	if utils.FileOrFolderExists(configFilePath) {
		utils.ErrExit("migration directory %q already contains a config.yaml. "+
			"Use the existing config or choose a different directory.", migrationDir)
	}

	printWelcomeBanner()

	// If connection string was provided via flag, skip the interactive prompt
	if sourceConnString != "" {
		handleConnectionStringDirect(configFilePath, exportDirPath, sourceConnString)
		return
	}

	// Prompt for source DB details
	var sourceOption string
	err = huh.NewSelect[string]().
		Title("How would you like to provide source database details?").
		Description("It is recommended to run assessment against your production database for an accurate assessment.").
		Options(
			huh.NewOption("Enter a connection string", "connection_string"),
			huh.NewOption("I don't have access to source database - generate scripts to gather metadata", "generate_scripts"),
			huh.NewOption("Skip and configure later", "skip"),
		).
		Value(&sourceOption).
		Run()
	if err != nil {
		utils.ErrExit("prompt failed: %v", err)
	}

	switch sourceOption {
	case "connection_string":
		handleConnectionString(configFilePath, exportDirPath)
	case "generate_scripts":
		handleGenerateScripts(configFilePath, exportDirPath)
	case "skip":
		handleSkip(configFilePath, exportDirPath)
	}
}

func printWelcomeBanner() {
	banner := `
══════════════════════════════════════════════════════════════
                    Welcome to YB Voyager
══════════════════════════════════════════════════════════════

YB Voyager is a database migration tool that helps you migrate
to YugabyteDB.

What YB Voyager can do:
  • Assess your source database for migration complexity and sizing
  • Export and import schema with automatic YugabyteDB optimizations
  • Migrate data via offline snapshot or live change data capture (CDC)
  • Validate data consistency between source and target

Supported source databases: PostgreSQL, Oracle, MySQL
Documentation: https://docs.yugabyte.com/preview/yugabyte-voyager/

══════════════════════════════════════════════════════════════

The first step is to assess your source database.
`
	fmt.Println(banner)
}

// handleConnectionString prompts for a connection string, validates it, and generates config
func handleConnectionString(configFilePath, exportDirPath string) {
	var connString string
	var parsed *parsedConnInfo

	for {
		err := huh.NewInput().
			Title("Enter your PostgreSQL connection string").
			Description("Format: postgresql://user:password@host:port/dbname").
			Placeholder("postgresql://user:password@host:5432/mydb").
			Value(&connString).
			Run()
		if err != nil {
			utils.ErrExit("prompt failed: %v", err)
		}

		connString = strings.TrimSpace(connString)
		if connString == "" {
			fmt.Println(color.YellowString("  No connection string provided. Skipping connection setup."))
			handleSkip(configFilePath, exportDirPath)
			return
		}

		var parseErr error
		parsed, parseErr = parsePostgresConnString(connString)
		if parseErr != nil {
			fmt.Println(color.RedString("  ✗ Invalid connection string: %v", parseErr))
			fmt.Println()
			continue
		}

		// Validate connectivity
		fmt.Printf("  Connecting to %s:%d...\n", parsed.Host, parsed.Port)
		if err := validatePostgresConnection(connString); err != nil {
			fmt.Println(color.RedString("  ✗ Connection failed: %v", err))
			fmt.Println()

			var retry bool
			huh.NewConfirm().
				Title("Would you like to try again?").
				Value(&retry).
				Run()
			if !retry {
				fmt.Println(color.YellowString("  Skipping connection setup. You can configure the connection later in the config file."))
				handleSkip(configFilePath, exportDirPath)
				return
			}
			continue
		}

		fmt.Println(color.GreenString("  ✓ Connected to %s:%d", parsed.Host, parsed.Port))
		break
	}

	// Create export-dir
	createExportDir(exportDirPath)

	// Fetch schemas if not specified in connection string
	schemas := parsed.Schema
	if schemas == "" {
		schemas = "public"
	}

	// Generate config
	generateConfigFile(configFilePath, exportDirPath, &sourceConfig{
		DBType:   "postgresql",
		Host:     parsed.Host,
		Port:     parsed.Port,
		DBName:   parsed.DBName,
		User:     parsed.User,
		Password: parsed.Password,
		Schema:   schemas,
	}, nil)

	fmt.Println(color.GreenString("  ✓ Created export directory: %s", exportDirPath))
	fmt.Println(color.GreenString("  ✓ Generated config: %s", configFilePath))

	printInitNextSteps(configFilePath, true, false)
}

// handleConnectionStringDirect handles the case where the connection string was provided via flag (non-interactive).
func handleConnectionStringDirect(configFilePath, exportDirPath, connStr string) {
	connStr = strings.TrimSpace(connStr)

	parsed, err := parsePostgresConnString(connStr)
	if err != nil {
		utils.ErrExit("Invalid connection string: %v", err)
	}

	// Validate connectivity
	fmt.Printf("  Connecting to %s:%d...\n", parsed.Host, parsed.Port)
	if err := validatePostgresConnection(connStr); err != nil {
		utils.ErrExit("Connection failed: %v", err)
	}
	fmt.Println(color.GreenString("  ✓ Connected to %s:%d", parsed.Host, parsed.Port))

	// Create export-dir
	createExportDir(exportDirPath)

	schemas := parsed.Schema
	if schemas == "" {
		schemas = "public"
	}

	// Generate config from template with source details filled in
	generateConfigFile(configFilePath, exportDirPath, &sourceConfig{
		DBType:   "postgresql",
		Host:     parsed.Host,
		Port:     parsed.Port,
		DBName:   parsed.DBName,
		User:     parsed.User,
		Password: parsed.Password,
		Schema:   schemas,
	}, nil)

	fmt.Println(color.GreenString("  ✓ Created export directory: %s", exportDirPath))
	fmt.Println(color.GreenString("  ✓ Generated config: %s", configFilePath))

	printInitNextSteps(configFilePath, true, false)
}

// handleGenerateScripts creates export-dir and copies gather-assessment-metadata scripts
func handleGenerateScripts(configFilePath, exportDirPath string) {
	createExportDir(exportDirPath)

	scriptsDir := filepath.Join(exportDirPath, "scripts")
	os.MkdirAll(scriptsDir, 0755)

	// Copy PostgreSQL scripts
	pgScriptsDir := filepath.Join(scriptsDir, "postgresql")
	copyGatherScripts(pgGatherScriptsInstalledDir, pgScriptsDir)

	// Copy Oracle scripts
	oracleScriptsDir := filepath.Join(scriptsDir, "oracle")
	copyGatherScripts(oracleGatherScriptsInstalledDir, oracleScriptsDir)

	// Generate a minimal config (no source connection)
	generateConfigFile(configFilePath, exportDirPath, nil, nil)

	fmt.Println(color.GreenString("  ✓ Created export directory: %s", exportDirPath))
	fmt.Println(color.GreenString("  ✓ Generated config: %s", configFilePath))
	fmt.Println(color.GreenString("  ✓ Copied metadata-gathering scripts to:"))
	fmt.Printf("    %s\n", scriptsDir)

	printInitNextSteps(configFilePath, false, true)
}

// handleSkip creates export-dir and generates a config template
func handleSkip(configFilePath, exportDirPath string) {
	createExportDir(exportDirPath)

	// Generate config with empty source
	generateConfigFile(configFilePath, exportDirPath, nil, nil)

	fmt.Println(color.GreenString("  ✓ Created export directory: %s", exportDirPath))
	fmt.Println(color.GreenString("  ✓ Generated config: %s", configFilePath))

	printInitNextSteps(configFilePath, false, false)
}

func createExportDir(exportDirPath string) {
	if err := os.MkdirAll(exportDirPath, 0755); err != nil {
		utils.ErrExit("failed to create export directory %q: %v", exportDirPath, err)
	}

	// Create standard subdirectories
	subdirs := []string{
		"schema", "data", "reports",
		"assessment", "assessment/metadata", "assessment/dbs",
		"assessment/metadata/schema", "assessment/reports",
		"metainfo", "metainfo/data", "metainfo/conf", "metainfo/ssl",
		"temp", "temp/ora2pg_temp_dir", "temp/schema",
		"scripts",
	}

	for _, subdir := range subdirs {
		if err := os.MkdirAll(filepath.Join(exportDirPath, subdir), 0755); err != nil {
			utils.ErrExit("failed to create subdirectory %q: %v", subdir, err)
		}
	}
}

type parsedConnInfo struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Schema   string
}

func parsePostgresConnString(connStr string) (*parsedConnInfo, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse connection string: %w", err)
	}

	if u.Scheme != "postgresql" && u.Scheme != "postgres" {
		return nil, fmt.Errorf("expected postgresql:// or postgres:// scheme, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}

	port := 5432
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	user := ""
	password := ""
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return nil, fmt.Errorf("database name is required in the connection string")
	}

	// Check for schema in search_path option
	schema := ""
	if options := u.Query().Get("options"); options != "" {
		for _, opt := range strings.Split(options, " ") {
			if strings.HasPrefix(opt, "-csearch_path=") || strings.HasPrefix(opt, "-csearch_path%3D") {
				schema = strings.TrimPrefix(opt, "-csearch_path=")
				schema = strings.TrimPrefix(schema, "-csearch_path%3D")
			}
		}
	}

	return &parsedConnInfo{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbName,
		Schema:   schema,
	}, nil
}

func validatePostgresConnection(connStr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

type sourceConfig struct {
	DBType   string
	Host     string
	Port     int
	DBName   string
	User     string
	Password string
	Schema   string
}

type targetConfig struct {
	Host     string
	Port     int
	DBName   string
	User     string
	Password string
}

// configTemplatePaths lists the locations to search for config templates, in priority order.
var configTemplatePaths = []string{
	"/opt/yb-voyager/config-templates",
}

// readConfigTemplate reads a config template file from installed or repo locations.
func readConfigTemplate(templateName string) (string, error) {
	// Try the installed path first
	for _, dir := range configTemplatePaths {
		path := filepath.Join(dir, templateName)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("config template %q not found in any of: %v", templateName, configTemplatePaths)
}

// generateConfigFile reads the offline-migration.yaml template, fills in the source
// connection details via string replacement, and writes it as the project config file.
func generateConfigFile(configFilePath, exportDirPath string, src *sourceConfig, tgt *targetConfig) {
	content, err := readConfigTemplate("offline-migration.yaml")
	if err != nil {
		utils.ErrExit("failed to read config template: %v\n"+
			"Ensure yb-voyager is installed or config-templates are available at /opt/yb-voyager/config-templates/", err)
	}

	// --- Global replacements ---
	content = replaceConfigValue(content, "export-dir:", "<export-dir-path>", exportDirPath)

	// --- Remove control plane section (not needed for init POC) ---
	content = removeSection(content, "Control Plane Configuration", "Source Database Configuration")

	// --- Source replacements ---
	if src != nil {
		content = replaceConfigValue(content, "db-type:", "postgresql", src.DBType)
		// Replace source host (only in the source section, before Target section)
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"db-host: localhost", fmt.Sprintf("db-host: %s", src.Host))
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"db-port: 5432", fmt.Sprintf("db-port: %d", src.Port))
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"db-name: test_db", fmt.Sprintf("db-name: %s", src.DBName))
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"db-schema: public", fmt.Sprintf("db-schema: %s", src.Schema))
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"db-user: test_user", fmt.Sprintf("db-user: %s", src.User))
		if src.Password != "" {
			content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
				"db-password: test_password", fmt.Sprintf("db-password: '%s'", strings.ReplaceAll(src.Password, "'", "''")))
		} else {
			content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
				"  db-password: test_password", "  # db-password: <password>  # Or set SOURCE_DB_PASSWORD env var")
		}
	}

	// --- Target section: leave template defaults (will be filled by start-migration) ---
	// No changes needed; the template already has placeholder values for target.

	if err := os.WriteFile(configFilePath, []byte(content), 0644); err != nil {
		utils.ErrExit("failed to write config file %q: %v", configFilePath, err)
	}
}

// replaceConfigValue replaces a value on a line matching the given key prefix.
func replaceConfigValue(content, keyPrefix, oldValue, newValue string) string {
	old := keyPrefix + " " + oldValue
	new := keyPrefix + " " + newValue
	return strings.Replace(content, old, new, 1)
}

// removeSection removes everything between the line containing startMarker and
// the line containing endMarker (exclusive — the endMarker line is kept).
func removeSection(content, startMarker, endMarker string) string {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return content
	}
	// Walk back to the start of the line (or a preceding blank/comment line)
	lineStart := strings.LastIndex(content[:startIdx], "\n")
	if lineStart == -1 {
		lineStart = 0
	}
	// Walk further back to include the "### ***" header line before the marker
	// by finding the start of the comment block
	for lineStart > 0 {
		prevNewline := strings.LastIndex(content[:lineStart], "\n")
		if prevNewline == -1 {
			break
		}
		line := strings.TrimSpace(content[prevNewline+1 : lineStart])
		if line == "" || strings.HasPrefix(line, "###") || strings.HasPrefix(line, "#") {
			lineStart = prevNewline
		} else {
			break
		}
	}

	endIdx := strings.Index(content, endMarker)
	if endIdx == -1 {
		return content
	}
	// Walk back to include the "# ---" separator line for the end section
	sectionSep := strings.LastIndex(content[:endIdx], "# ---------")
	if sectionSep != -1 && sectionSep > startIdx {
		endIdx = sectionSep
		// Walk back further to capture the leading newline
		prevNl := strings.LastIndex(content[:endIdx], "\n")
		if prevNl != -1 && prevNl > lineStart {
			endIdx = prevNl
		}
	}

	return content[:lineStart] + "\n" + content[endIdx:]
}

// replaceInSection replaces oldStr with newStr only within the text between startMarker and endMarker.
func replaceInSection(content, startMarker, endMarker, oldStr, newStr string) string {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return content
	}
	endIdx := strings.Index(content[startIdx:], endMarker)
	if endIdx == -1 {
		// Marker not found; replace in the rest of the content
		before := content[:startIdx]
		section := content[startIdx:]
		section = strings.Replace(section, oldStr, newStr, 1)
		return before + section
	}
	before := content[:startIdx]
	section := content[startIdx : startIdx+endIdx]
	after := content[startIdx+endIdx:]
	section = strings.Replace(section, oldStr, newStr, 1)
	return before + section + after
}

func copyGatherScripts(srcDir, destDir string) {
	if !utils.FileOrFolderExists(srcDir) {
		fmt.Println(color.YellowString("  Warning: gather-assessment-metadata scripts not found at %s", srcDir))
		fmt.Println(color.YellowString("  You may need to install yb-voyager to get the scripts, or copy them manually."))
		return
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		utils.ErrExit("failed to create scripts directory %q: %v", destDir, err)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		utils.ErrExit("failed to read scripts directory %q: %v", srcDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcFile := filepath.Join(srcDir, entry.Name())
		destFile := filepath.Join(destDir, entry.Name())
		data, err := os.ReadFile(srcFile)
		if err != nil {
			utils.ErrExit("failed to read script %q: %v", srcFile, err)
		}
		if err := os.WriteFile(destFile, data, 0755); err != nil {
			utils.ErrExit("failed to write script %q: %v", destFile, err)
		}
	}
}

func printInitNextSteps(configFilePath string, connected bool, scripts bool) {
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println()

	step := 1
	if !connected {
		fmt.Printf("  %d. Add your source connection details to the config file:\n", step)
		fmt.Printf("     vi %s\n\n", configFilePath)
		step++
	}

	if scripts {
		fmt.Printf("  %d. Copy the appropriate scripts directory to a machine that can reach\n", step)
		fmt.Println("     your source database:")
		fmt.Println()
		exportDirPath := filepath.Dir(configFilePath)
		exportDirPath = filepath.Join(exportDirPath, "export-dir")
		fmt.Printf("     PostgreSQL:  %s/scripts/postgresql\n", exportDirPath)
		fmt.Printf("     Oracle:      %s/scripts/oracle\n", exportDirPath)
		fmt.Println()
		fmt.Println("     Run the yb-voyager-<db_type>-gather-assessment-metadata.sh script with --help to see usage.")
		fmt.Println()
		step++

		fmt.Printf("  %d. Copy the resulting metadata directory back to this machine.\n\n", step)
		step++

		fmt.Printf("  %d. Run assessment:\n", step)
		fmt.Printf("     yb-voyager assess-migration --config-file %s \\\n", configFilePath)
		fmt.Println("       --assessment-metadata-dir /path/to/assessment-metadata")
	} else {
		fmt.Printf("  %d. Run assessment:\n", step)
		fmt.Printf("     yb-voyager assess-migration --config-file %s\n", configFilePath)
	}

	fmt.Println()
	printTip(configFilePath)
}

func printTip(configFilePath string) {
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Printf("Tip: Run yb-voyager status -c %s\n", configFilePath)
	fmt.Println("     anytime to check migration progress and see what to do next.")
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println()
}
