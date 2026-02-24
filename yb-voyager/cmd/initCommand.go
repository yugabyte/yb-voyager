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
var migrationDirExplicit bool
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
		"path to the migration directory (will be created if it doesn't exist). "+
			"If not provided, defaults to ./migration-<dbname> or ./migration")
	initCmd.Flags().StringVar(&sourceConnString, "source-db-connection-string", "",
		"source database connection string (e.g. postgresql://user:password@host:5432/dbname). If provided, skips the interactive prompt.")
}

func runInit() {
	migrationDirExplicit = migrationDir != ""

	// If migration-dir was explicitly provided, resolve to absolute path now
	if migrationDirExplicit {
		var err error
		migrationDir, err = filepath.Abs(migrationDir)
		if err != nil {
			utils.ErrExit("failed to resolve migration-dir path: %v", err)
		}
	}

	printWelcomeBanner()

	// If connection string was provided via flag, skip the interactive prompt
	if sourceConnString != "" {
		handleConnectionStringDirect(sourceConnString)
		return
	}

	// Prompt for source DB details
	var sourceOption string
	err := huh.NewSelect[string]().
		Title("First, let's assess your source database. How would you like to connect?\n"+
			"Recommendation: Connect to your production database for accurate results.").
		Options(
			huh.NewOption("Enter a connection string", "connection_string"),
			huh.NewOption("I don't have access to source database from this machine - Get scripts that can be run on another machine.", "generate_scripts"),
			huh.NewOption("Skip and configure later", "skip"),
		).
		Value(&sourceOption).
		Run()
	if err != nil {
		utils.ErrExit("prompt failed: %v", err)
	}

	switch sourceOption {
	case "connection_string":
		handleConnectionString()
	case "generate_scripts":
		handleGenerateScripts()
	case "skip":
		handleSkip()
	}
}

// ensureMigrationDir sets migrationDir to a sensible default if it was not
// provided via the --migration-dir flag. The directory is named
// "voyager-migration-<dbname>" (or "voyager-migration" when no dbname is
// available). If that directory already exists, a numeric suffix is appended
// (-2, -3, ...) until a non-existing path is found. Existing directories are
// never reused.
func ensureMigrationDir(dbName string) {
	if migrationDir != "" {
		return
	}
	baseName := "voyager-migration"
	if dbName != "" {
		baseName = fmt.Sprintf("voyager-migration-%s", dbName)
	}

	candidate := baseName
	for i := 2; ; i++ {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			utils.ErrExit("failed to resolve migration-dir path: %v", err)
		}
		if !utils.FileOrFolderExists(absPath) {
			migrationDir = absPath
			return
		}
		candidate = fmt.Sprintf("%s-%d", baseName, i)
	}
}

// getMigrationPaths returns the config-file and export-dir paths derived from
// the current migrationDir.
func getMigrationPaths() (configFilePath, exportDirPath string) {
	return filepath.Join(migrationDir, "config.yaml"), filepath.Join(migrationDir, "export-dir")
}

// checkConfigIdempotency exits with a helpful message if a config.yaml already
// exists inside migrationDir.
func checkConfigIdempotency() {
	configFilePath, _ := getMigrationPaths()
	if utils.FileOrFolderExists(configFilePath) {
		utils.ErrExit("migration directory %q already contains a config.yaml. "+
			"Use the existing config or choose a different directory.", migrationDir)
	}
}

func printWelcomeBanner() {
	printSection("Welcome to YB Voyager",
		"Migrate your database to YugabyteDB.",
		"  - Assessment : Assess source database for complexity and sizing",
		"  - Schema     : Export and import schema with auto optimizations",
		"  - Data       : Migrate data offline or live with CDC",
		"  - Validation : Validate data consistency and performance between source and target",
		"",
		dimStyle.Render("Docs: https://docs.yugabyte.com/preview/yugabyte-voyager/"),
	)
	fmt.Println()
}

// handleConnectionString prompts for a connection string, validates it, and generates config
func handleConnectionString() {
	var connString string
	var parsed *parsedConnInfo

	for {
		err := huh.NewInput().
			Title("Enter your PostgreSQL connection string").
			Description("Format: postgresql://user:password@host:port/dbname").
			// Placeholder("postgresql://user:password@host:5432/mydb").
			Value(&connString).
			Run()
		if err != nil {
			utils.ErrExit("prompt failed: %v", err)
		}

		connString = strings.TrimSpace(connString)
		if connString == "" {
			fmt.Println(color.YellowString("  No connection string provided. Skipping connection setup."))
			handleSkip()
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
				ensureMigrationDir(parsed.DBName)
				handleSkip()
				return
			}
			continue
		}
		break
	}

	ensureMigrationDir(parsed.DBName)
	if migrationDirExplicit {
		checkConfigIdempotency()
	}
	configFilePath, exportDirPath := getMigrationPaths()

	// Create export-dir
	createExportDir(exportDirPath)

	schemas := parsed.Schema
	if schemas == "" {
		schemas = "public"
	}

	// Check SELECT permissions
	selectPermsMissing := checkAndReportSelectPermissions(connString, schemas)

	// Prompt for fleet control plane
	cpConnStr := promptFleetControlPlane()

	// Generate config
	generateConfigFile(configFilePath, exportDirPath, &sourceConfig{
		DBType:   "postgresql",
		Host:     parsed.Host,
		Port:     parsed.Port,
		DBName:   parsed.DBName,
		User:     parsed.User,
		Password: parsed.Password,
		Schema:   schemas,
	}, nil, cpConnStr)

	printInitResultBox(parsed, configFilePath, exportDirPath, selectPermsMissing)
	printInitNextSteps(configFilePath, true, false, selectPermsMissing, parsed)
}

// handleConnectionStringDirect handles the case where the connection string was provided via flag (non-interactive).
func handleConnectionStringDirect(connStr string) {
	connStr = strings.TrimSpace(connStr)

	parsed, err := parsePostgresConnString(connStr)
	if err != nil {
		utils.ErrExit("Invalid connection string: %v", err)
	}

	// Validate connectivity
	if err := validatePostgresConnection(connStr); err != nil {
		utils.ErrExit("Connection failed: %v", err)
	}

	ensureMigrationDir(parsed.DBName)
	if migrationDirExplicit {
		checkConfigIdempotency()
	}
	configFilePath, exportDirPath := getMigrationPaths()

	// Create export-dir
	createExportDir(exportDirPath)

	schemas := parsed.Schema
	if schemas == "" {
		schemas = "public"
	}

	// Check SELECT permissions
	selectPermsMissing := checkAndReportSelectPermissions(connStr, schemas)

	// Generate config from template with source details filled in
	// Non-interactive: no fleet prompt; use default local control plane
	generateConfigFile(configFilePath, exportDirPath, &sourceConfig{
		DBType:   "postgresql",
		Host:     parsed.Host,
		Port:     parsed.Port,
		DBName:   parsed.DBName,
		User:     parsed.User,
		Password: parsed.Password,
		Schema:   schemas,
	}, nil)

	printInitResultBox(parsed, configFilePath, exportDirPath, selectPermsMissing)
	printInitNextSteps(configFilePath, true, false, selectPermsMissing, parsed)
}

// handleGenerateScripts creates export-dir and copies gather-assessment-metadata scripts
func handleGenerateScripts() {
	ensureMigrationDir("")
	if migrationDirExplicit {
		checkConfigIdempotency()
	}
	configFilePath, exportDirPath := getMigrationPaths()

	createExportDir(exportDirPath)

	scriptsDir := filepath.Join(migrationDir, "scripts")
	os.MkdirAll(scriptsDir, 0755)

	// Copy PostgreSQL scripts
	pgScriptsDir := filepath.Join(scriptsDir, "postgresql")
	copyGatherScripts(pgGatherScriptsInstalledDir, pgScriptsDir)

	// Copy Oracle scripts
	oracleScriptsDir := filepath.Join(scriptsDir, "oracle")
	copyGatherScripts(oracleGatherScriptsInstalledDir, oracleScriptsDir)

	// Prompt for fleet control plane
	cpConnStr := promptFleetControlPlane()

	// Generate a minimal config (no source connection)
	generateConfigFile(configFilePath, exportDirPath, nil, nil, cpConnStr)

	fmt.Println()
	fmt.Println("  " + titleStyle.Render("Initializing Migration Project"))
	fmt.Println("  " + ruleStyle.Render(strings.Repeat("─", ruleWidth)))

	steps := []string{
		successLine("Created migration workspace  " + dimStyle.Render(displayPath(exportDirPath))),
		successLine("Generated config             " + dimStyle.Render(displayPath(configFilePath))),
		successLine("Copied metadata scripts      " + dimStyle.Render(displayPath(scriptsDir))),
	}
	for _, step := range steps {
		time.Sleep(1 * time.Second)
		fmt.Println("  " + step)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println()
	fmt.Println("  " + successStyle.Render("Done!"))
	fmt.Println()

	printInitNextSteps(configFilePath, false, true, false, nil)
}

// handleSkip creates export-dir and generates a config template
func handleSkip() {
	ensureMigrationDir("")
	if migrationDirExplicit {
		checkConfigIdempotency()
	}
	configFilePath, exportDirPath := getMigrationPaths()

	createExportDir(exportDirPath)

	// Prompt for fleet control plane
	cpConnStr := promptFleetControlPlane()

	// Generate config with empty source
	generateConfigFile(configFilePath, exportDirPath, nil, nil, cpConnStr)

	fmt.Println()
	fmt.Println("  " + titleStyle.Render("Initializing Migration Project"))
	fmt.Println("  " + ruleStyle.Render(strings.Repeat("─", ruleWidth)))

	steps := []string{
		successLine("Created migration workspace  " + dimStyle.Render(displayPath(exportDirPath))),
		successLine("Generated config             " + dimStyle.Render(displayPath(configFilePath))),
	}
	for _, step := range steps {
		time.Sleep(1 * time.Second)
		fmt.Println("  " + step)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println()
	fmt.Println("  " + successStyle.Render("Done!"))
	fmt.Println()

	printInitNextSteps(configFilePath, false, false, false, nil)
}

// promptFleetControlPlane asks the user whether they have a fleet of databases to assess
// and, if so, prompts for a shared control plane connection string. Returns the connection
// string (empty if the user chose local UI).
func promptFleetControlPlane() string {
	var fleetOption string
	err := huh.NewSelect[string]().
		Title("Choose your assessment control plane.").
		Description("Voyager stores assessment results in a YugabyteDB instance with a built-in dashboard.\nTip: To view assessments for multiple databases in one place, set up a shared instance: https://docs.yugabyte.com/stable/quick-start/linux/#install-yugabytedb").
		Options(
			huh.NewOption("Local instance (default — no setup needed)", "local"),
			huh.NewOption("Shared YugabyteDB instance (I'll provide a connection string)", "fleet"),
		).
		Value(&fleetOption).
		Run()
	if err != nil {
		utils.ErrExit("prompt failed: %v", err)
	}

	if fleetOption == "fleet" {
		var cpConnString string
		err := huh.NewInput().
			Title("Enter the connection string for your assessment control plane").
			Description("Format: postgresql://user:password@host:port").
			Placeholder("postgresql://yugabyte:yugabyte@yb-fleet.example.com:5433").
			Value(&cpConnString).
			Run()
		if err != nil {
			utils.ErrExit("prompt failed: %v", err)
		}
		cpConnString = strings.TrimSpace(cpConnString)
		if cpConnString != "" {
			return cpConnString
		}
		fmt.Println(color.YellowString("  No connection string provided. Using local UI."))
	}

	return ""
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

// checkBasicSelectPermissions connects to the source DB and checks whether the
// user has SELECT permission on all tables in the given schemas (comma-separated).
// Returns true if any table is missing SELECT permission (i.e. permissions need
// to be granted), and the count of affected tables.
func checkBasicSelectPermissions(connStr string, schemas string) (missingPerms bool, missingCount int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return false, 0, fmt.Errorf("failed to connect for permission check: %w", err)
	}
	defer conn.Close(ctx)

	schemaList := strings.Split(schemas, ",")
	for i, s := range schemaList {
		schemaList[i] = strings.TrimSpace(s)
	}

	quoted := make([]string, len(schemaList))
	for i, s := range schemaList {
		quoted[i] = fmt.Sprintf("'%s'", s)
	}
	schemaFilter := strings.Join(quoted, ",")

	query := fmt.Sprintf(`
		SELECT count(*)
		FROM pg_tables t
		WHERE t.schemaname IN (%s)
		  AND NOT has_table_privilege(current_user,
		          quote_ident(t.schemaname) || '.' || quote_ident(t.tablename), 'SELECT')
	`, schemaFilter)

	var count int
	if err := conn.QueryRow(ctx, query).Scan(&count); err != nil {
		return false, 0, fmt.Errorf("failed to check SELECT permissions: %w", err)
	}

	return count > 0, count, nil
}

// checkAndReportSelectPermissions runs the SELECT permission check and prints
// an inline warning if permissions are missing. Returns true when permissions
// are missing.
func checkAndReportSelectPermissions(connStr, schemas string) bool {
	missing, count, err := checkBasicSelectPermissions(connStr, schemas)
	if err != nil {
		fmt.Printf("  %s\n", warnStyle.Render(fmt.Sprintf("⚠ Could not verify SELECT permissions: %v", err)))
		return false
	}
	if missing {
		fmt.Printf("  %s\n", warnStyle.Render(fmt.Sprintf("⚠ SELECT permission missing on %d table(s)", count)))
	}
	return missing
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
// assessmentCP is an optional assessment control plane connection string; if non-empty,
// the assess-migration.assessment-control-plane and yugabyted-control-plane.db-conn-string
// fields are populated with this value.
func generateConfigFile(configFilePath, exportDirPath string, src *sourceConfig, tgt *targetConfig, assessmentCP ...string) {
	content, err := readConfigTemplate("offline-migration.yaml")
	if err != nil {
		utils.ErrExit("failed to read config template: %v\n"+
			"Ensure yb-voyager is installed or config-templates are available at /opt/yb-voyager/config-templates/", err)
	}

	// --- Global replacements ---
	content = replaceConfigValue(content, "export-dir:", "<export-dir-path>", exportDirPath)

	// --- Control plane section ---
	// If an assessment control plane connection string was provided, populate the
	// assess-migration.assessment-control-plane field (not the global control plane config).
	cpConnStr := ""
	if len(assessmentCP) > 0 && assessmentCP[0] != "" {
		cpConnStr = assessmentCP[0]
	}
	if cpConnStr != "" {
		// Try replacing the commented-out placeholder first (new template)
		newVal := fmt.Sprintf("assessment-control-plane: %s", cpConnStr)
		replaced := strings.Replace(content,
			"# assessment-control-plane: postgresql://yugabyte:yugabyte@127.0.0.1:5433",
			newVal, 1)
		if replaced != content {
			content = replaced
		} else {
			// Fallback for older templates that don't have the placeholder:
			// inject the field right after the "assess-migration:" section header.
			marker := "assess-migration:\n"
			idx := strings.Index(content, marker)
			if idx != -1 {
				insertAt := idx + len(marker)
				content = content[:insertAt] +
					fmt.Sprintf("\n  ### Connection string for the assessment control plane\n  %s\n", newVal) +
					content[insertAt:]
			}
		}
	}

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
	} else {
		// Comment out source connection fields so they don't get picked up as
		// explicitly-set flags (e.g., by assess-migration --assessment-metadata-dir).
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"  db-host: localhost", "  # db-host: localhost")
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"  db-port: 5432", "  # db-port: 5432")
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"  db-name: test_db", "  # db-name: <database-name>")
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"  db-schema: public", "  # db-schema: public")
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"  db-user: test_user", "  # db-user: <username>")
		content = replaceInSection(content, "Source Database Configuration", "Target Database Configuration",
			"  db-password: test_password", "  # db-password: <password>  # Or set SOURCE_DB_PASSWORD env var")
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

// printInitResultBox prints the "Initializing Migration Project" section with
// progressive checkmarks (each appearing with a short delay).
func printInitResultBox(parsed *parsedConnInfo, configFilePath, exportDirPath string, selectPermsMissing bool) {
	fmt.Println()
	fmt.Println("  " + titleStyle.Render("Initializing Migration Project"))
	fmt.Println("  " + ruleStyle.Render(strings.Repeat("─", ruleWidth)))

	sourceLine := fmt.Sprintf("PostgreSQL @ %s:%d/%s (%s)", parsed.Host, parsed.Port, parsed.DBName, parsed.User)
	fmt.Println("  " + formatKeyValue("Source:", sourceLine, 8))
	fmt.Println()

	steps := []string{
		successLine("Connected to source database"),
		successLine("Created migration workspace  " + dimStyle.Render(displayPath(exportDirPath))),
		successLine("Generated config             " + dimStyle.Render(displayPath(configFilePath))),
	}

	for _, step := range steps {
		time.Sleep(1 * time.Second)
		fmt.Println("  " + step)
	}
	if selectPermsMissing {
		time.Sleep(500 * time.Millisecond)
		fmt.Println("  " + warnStyle.Render("⚠") + " SELECT permissions missing — grant before running assessment")
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println()
	fmt.Println("  " + successStyle.Render("Done!"))
	fmt.Println()
}

func printInitNextSteps(configFilePath string, connected bool, scripts bool, selectPermsMissing bool, parsed *parsedConnInfo) {
	var lines []string

	step := 1
	if !connected && !scripts {
		lines = append(lines, fmt.Sprintf("%d. Add your source connection details to the config file:", step))
		lines = append(lines, fmt.Sprintf("   vi %s", displayPath(configFilePath)))
		lines = append(lines, "")
		step++
	}

	if selectPermsMissing && parsed != nil {
		lines = append(lines, fmt.Sprintf("%d. Grant required permissions on your source database:", step))
		lines = append(lines, "")
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("  psql -h %s -d %s -U %s \\\n    -f /opt/yb-voyager/guardrails-scripts/yb-voyager-pg-grant-migration-permissions.sql",
			parsed.Host, parsed.DBName, parsed.User)))
		lines = append(lines, "")
		step++
	}

	if scripts {
		migrationDirPath := filepath.Dir(configFilePath)
		scriptsDir := filepath.Join(migrationDirPath, "scripts")

		lines = append(lines, fmt.Sprintf("%d. Copy the scripts to a machine that can reach your source database:", step))
		lines = append(lines, fmt.Sprintf("   %s", dimStyle.Render(scriptsDir)))
		step++
		lines = append(lines, fmt.Sprintf("%d. Run the script on that machine (PostgreSQL example):", step))
		lines = append(lines, cmdStyle.Render("   bash yb-voyager-pg-gather-assessment-metadata.sh 'postgresql://user@host:5432/dbname' 'public' /path/to/output false"))
		step++
		lines = append(lines, fmt.Sprintf("%d. Copy the resulting metadata directory back to this machine.", step))
		step++
		lines = append(lines, nextStepLabelStyle.Render(fmt.Sprintf("%d. Run assessment:", step)))
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("   yb-voyager assess run --config-file %s --assessment-metadata-dir /path/to/metadata",
			displayPath(configFilePath))))
	} else {
		lines = append(lines, nextStepLabelStyle.Render(fmt.Sprintf("%d. Assess your source database for migration:", step)))
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("  yb-voyager assess run --config-file %s",
			displayPath(configFilePath))))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(fmt.Sprintf("Tip: yb-voyager status -c %s", displayPath(configFilePath))))

	printSection("What's Next", lines...)
	fmt.Println()
}
