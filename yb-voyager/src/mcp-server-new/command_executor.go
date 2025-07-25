package mcpservernew

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/config"
)

type CommandExecutor struct{}

type CommandResult struct {
	ExecutionID string                 `json:"execution_id"`
	Status      string                 `json:"status"` // "running", "completed", "failed", "timeout"
	Progress    []string               `json:"progress"`
	Error       string                 `json:"error,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	ExitCode    int                    `json:"exit_code"`
	ConfigKeys  map[string]interface{} `json:"config_keys,omitempty"` // Config keys used by this command
}

func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{}
}

// ExecuteCommandAsync executes a YB Voyager command asynchronously
func (ce *CommandExecutor) ExecuteCommandAsync(ctx context.Context, command string, configPath string, additionalArgs string) (*CommandResult, error) {
	// Validate config
	if err := ce.validateConfigForCommand(command, configPath); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Extract config keys used by this command
	configKeys, err := ce.extractConfigKeysForCommand(command, configPath)
	if err != nil {
		// Log the error but don't fail the command
		log.Warnf("Failed to extract config keys: %v", err)
	}
	// Create result
	result := &CommandResult{
		ExecutionID: ce.generateExecutionID(),
		Status:      "running",
		Progress:    make([]string, 0),
		StartTime:   time.Now(),
		ConfigKeys:  configKeys,
	}

	// Start command execution in background
	go ce.runCommand(ctx, command, configPath, additionalArgs, result)
	return result, nil
}

// runCommand executes the command and updates the result
func (ce *CommandExecutor) runCommand(ctx context.Context, command string, configPath string, additionalArgs string, result *CommandResult) {
	defer func() {
		endTime := time.Now()
		result.EndTime = &endTime
		result.Duration = ce.formatDuration(endTime.Sub(result.StartTime))
	}()

	// Build command
	cmdArgs := ce.buildCommandArgs(command, configPath, additionalArgs)
	voyagerPath := ce.findYbVoyagerPath(configPath)

	cmd := exec.CommandContext(ctx, voyagerPath, cmdArgs...)
	cmd.Env = ce.buildEnvironment(additionalArgs)

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("Failed to create stdout pipe: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("Failed to create stderr pipe: %v", err)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("Failed to start command: %v", err)
		return
	}

	// Stream output in real-time
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ce.streamOutput(stdout, result, "stdout")
	}()

	go func() {
		defer wg.Done()
		ce.streamOutput(stderr, result, "stderr")
	}()

	// Wait for command to complete
	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
	} else {
		result.Status = "completed"
	}
	result.ExitCode = ce.getExitCode(err)
	result.Progress = append(result.Progress, fmt.Sprintf("[%s] COMMAND_COMPLETED: Status=%s, ExitCode=%d", time.Now().Format("15:04:05"), result.Status, result.ExitCode))
}

// buildCommandArgs builds the command arguments
func (ce *CommandExecutor) buildCommandArgs(command string, configPath string, additionalArgs string) []string {
	// Split the command into separate arguments (e.g., "export schema" -> ["export", "schema"])
	commandArgs := strings.Fields(command)
	cmdArgs := append(commandArgs, "--config-file", configPath)

	if additionalArgs != "" {
		args := strings.Fields(additionalArgs)
		cmdArgs = append(cmdArgs, args...)
	}

	// Automatically add --yes flag for async commands to avoid interactive prompts
	cmdArgs = append(cmdArgs, "--yes")

	return cmdArgs
}

// buildEnvironment builds the environment variables
func (ce *CommandExecutor) buildEnvironment(additionalArgs string) []string {
	env := os.Environ()

	// Add current directory to PATH
	currentDir, _ := os.Getwd()
	pathVar := "PATH=" + currentDir + ":" + os.Getenv("PATH")
	env = append(env, pathVar)

	// Add password since we're always using --yes for async commands
	env = append(env, "SOURCE_DB_PASSWORD=testpassword")

	return env
}

// streamOutput streams output from a pipe
func (ce *CommandExecutor) streamOutput(pipe io.ReadCloser, result *CommandResult, streamType string) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		timestamp := time.Now().Format("15:04:05")
		formattedLine := fmt.Sprintf("[%s] %s: %s", timestamp, streamType, line)
		result.Progress = append(result.Progress, formattedLine)

		// Check for interactive prompts
		if ce.isInteractivePrompt(line) {
			promptLine := fmt.Sprintf("[%s] INTERACTIVE_PROMPT: %s", timestamp, line)
			result.Progress = append(result.Progress, promptLine)
		}
	}
}

// isInteractivePrompt checks if a line contains an interactive prompt
func (ce *CommandExecutor) isInteractivePrompt(line string) bool {
	promptPatterns := []string{
		"[Y/N]:",
		"[y/n]:",
		"(Y/N):",
		"(y/n):",
		"Do you want to continue",
		"Password:",
		"password:",
		"Enter password:",
		"Please confirm:",
	}

	line = strings.ToLower(line)
	for _, pattern := range promptPatterns {
		if strings.Contains(line, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// generateExecutionID generates a unique execution ID
func (ce *CommandExecutor) generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}

// validateConfigForCommand validates the config for the given command
func (ce *CommandExecutor) validateConfigForCommand(command string, configPath string) error {
	// Check if config file exists
	if !utils.FileOrFolderExists(configPath) {
		return fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Check if config file is empty
	if utils.IsFileEmpty(configPath) {
		return fmt.Errorf("config file is empty: %s", configPath)
	}

	// Validate config content
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	if err := config.ValidateConfigFile(v); err != nil {
		return fmt.Errorf("config content validation failed: %w", err)
	}

	// Validate required sections for the command
	requiredSections := ce.getRequiredSectionsForCommand(command)
	configInfo, err := ce.getConfigInfo(configPath)
	if err != nil {
		return fmt.Errorf("failed to get config info: %w", err)
	}

	for _, section := range requiredSections {
		if !ce.sectionExists(configInfo, section) {
			return fmt.Errorf("required section '%s' not found in config file", section)
		}
	}

	return nil
}

// getRequiredSectionsForCommand returns the required config sections for a command
func (ce *CommandExecutor) getRequiredSectionsForCommand(command string) []string {
	normalizedCommand := ce.normalizeCommand(command)

	switch normalizedCommand {
	case "assess-migration":
		return []string{"source"} // assess-migration section is optional
	case "export schema":
		return []string{"source"} // export-schema section is optional
	case "export data":
		return []string{"source"} // export-data section is optional
	case "import schema":
		return []string{"target"} // import-schema section is optional
	case "import data":
		return []string{"target"} // import-data section is optional
	case "analyze-schema":
		return []string{"source"} // analyze-schema section is optional
	default:
		return []string{"source"}
	}
}

// sectionExists checks if a section exists in the config
func (ce *CommandExecutor) sectionExists(configInfo map[string]interface{}, section string) bool {
	// If sections is a map, check if the section exists
	if _, exists := configInfo[section].(map[string]interface{}); exists {
		return exists
	}
	// If sections is not a map, return false
	return false
}

// normalizeCommand normalizes the command name
func (ce *CommandExecutor) normalizeCommand(command string) string {
	// Remove any leading/trailing whitespace
	command = strings.TrimSpace(command)

	// Handle common variations
	switch strings.ToLower(command) {
	case "assess", "assess-migration", "assess_migration":
		return "assess-migration"
	case "export-schema", "export_schema", "export schema":
		return "export schema"
	case "export-data", "export_data", "export data":
		return "export data"
	case "import-schema", "import_schema", "import schema":
		return "import schema"
	case "import-data", "import_data", "import data":
		return "import data"
	case "analyze-schema", "analyze_schema", "analyze schema":
		return "analyze-schema"
	default:
		return command
	}
}

// getConfigInfo gets information about the config file
func (ce *CommandExecutor) getConfigInfo(configPath string) (map[string]interface{}, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	// Get all keys to determine sections
	allKeys := v.AllKeys()
	sections := make(map[string]interface{})

	for _, key := range allKeys {
		// If the key is in a section
		parts := strings.Split(key, ".")
		if len(parts) > 0 {
			section := parts[0]

			// Initialize section if it doesn't exist
			if sections[section] == nil {
				sections[section] = make(map[string]interface{})
			}

			// Get the value for this key
			value := v.Get(key)
			sections[section].(map[string]interface{})[key] = value
		}
		// if key is not in a section, include it as a global key
		if len(parts) == 1 {
			// Add it to the global section
			// If the global section doesn't exist, create it
			if sections["global"] == nil {
				sections["global"] = make(map[string]interface{})
			}
			sections["global"].(map[string]interface{})[key] = v.Get(key)
		}
	}

	return sections, nil
}

// findYbVoyagerPath finds the path to the yb-voyager executable
func (ce *CommandExecutor) findYbVoyagerPath(configPath string) string {
	// Try to find yb-voyager in PATH
	if path, err := exec.LookPath("yb-voyager"); err == nil {
		return path
	}

	// Try to find yb-voyager in the project root
	projectRoot := "/home/ubuntu/yb-voyager/yb-voyager"
	binaryPath := filepath.Join(projectRoot, "yb-voyager")
	if utils.FileOrFolderExists(binaryPath) {
		return binaryPath
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

	// Try to find yb-voyager in current directory
	currentDir, _ := os.Getwd()
	localPath := filepath.Join(currentDir, "yb-voyager")
	if utils.FileOrFolderExists(localPath) {
		return localPath
	}

	// Try to find yb-voyager in the same directory as the config file
	configDir := filepath.Dir(configPath)
	binaryPath = filepath.Join(configDir, "yb-voyager")
	if utils.FileOrFolderExists(binaryPath) {
		return binaryPath
	}

	// Fallback to just "yb-voyager"
	return "yb-voyager"
}

// getExitCode gets the exit code from an error
func (ce *CommandExecutor) getExitCode(err error) int {
	if err == nil {
		return 0
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}

	return -1
}

// formatDuration formats a duration
func (ce *CommandExecutor) formatDuration(d time.Duration) string {
	return d.String()
}

// extractConfigKeysForCommand extracts config keys relevant to a specific command
func (ce *CommandExecutor) extractConfigKeysForCommand(command string, configPath string) (map[string]interface{}, error) {
	// Load config
	configInfo, err := ce.getConfigInfo(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	normalizedCommand := ce.normalizeCommand(command)
	relevantKeys := make(map[string]interface{})

	// Always include global keys (top-level keys like export-dir, log-level, etc.)
	// Get all the keys in the global section
	globalKeys := configInfo["global"].(map[string]interface{})
	for key, value := range globalKeys {
		relevantKeys[key] = value
	}

	// Add source keys for commands that need source database
	switch normalizedCommand {
	case "assess-migration", "export schema", "export data", "analyze-schema":
		if sourceKeys, exists := configInfo["source"]; exists {
			relevantKeys["source"] = sourceKeys
		}
		// Add command-specific keys if they exist (using hyphenated section names)
		hyphenatedCommand := strings.ReplaceAll(normalizedCommand, " ", "-")
		if cmdKeys, exists := configInfo[hyphenatedCommand]; exists {
			relevantKeys[hyphenatedCommand] = cmdKeys
		}
	case "import schema", "import data":
		if targetKeys, exists := configInfo["target"]; exists {
			relevantKeys["target"] = targetKeys
		}
		// Add command-specific keys if they exist (using hyphenated section names)
		hyphenatedCommand := strings.ReplaceAll(normalizedCommand, " ", "-")
		if cmdKeys, exists := configInfo[hyphenatedCommand]; exists {
			relevantKeys[hyphenatedCommand] = cmdKeys
		}
	default:
		// For unknown commands, include all sections
		for section, keys := range configInfo {
			relevantKeys[section] = keys
		}
	}

	return relevantKeys, nil
}
