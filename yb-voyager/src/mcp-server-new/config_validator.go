package mcpservernew

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/config"
)

// ConfigValidator handles config file validation
type ConfigValidator struct{}

// NewConfigValidator creates a new config validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// ValidateConfig checks if a config file exists, is readable, and has valid content
func (cv *ConfigValidator) ValidateConfig(configPath string) error {
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}

	// Get absolute path
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", configPath, err)
	}

	// Use existing YB Voyager utility to check if file exists
	if !utils.FileOrFolderExists(absPath) {
		return fmt.Errorf("config file does not exist: %s", absPath)
	}

	// Check if file is readable
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("config file is not readable: %s - %w", absPath, err)
	}
	defer file.Close()

	// Check if file is not empty using existing utility
	if utils.IsFileEmpty(absPath) {
		return fmt.Errorf("config file is empty: %s", absPath)
	}

	// Use existing YB Voyager config validation logic
	if err := cv.validateConfigContent(absPath); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

// validateConfigContent uses the existing YB Voyager config validation logic
func (cv *ConfigValidator) validateConfigContent(configPath string) error {
	// Create a new viper instance for this validation
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(configPath)

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Use the shared validation function from utils/config package
	if err := config.ValidateConfigFile(v); err != nil {
		return fmt.Errorf("config content validation failed: %w", err)
	}

	return nil
}

// GetConfigInfo returns information about the config file
func (cv *ConfigValidator) GetConfigInfo(configPath string) (map[string]interface{}, error) {
	if err := cv.ValidateConfig(configPath); err != nil {
		return nil, err
	}

	absPath, _ := filepath.Abs(configPath)
	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Read config content for additional info
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(absPath)

	var configSections []string
	var configKeys []string

	if err := v.ReadInConfig(); err == nil {
		// Get all keys from the config
		configKeys = v.AllKeys()

		// Extract unique sections
		sections := make(map[string]bool)
		for _, key := range configKeys {
			if parts := strings.Split(key, "."); len(parts) > 0 {
				sections[parts[0]] = true
			}
		}

		for section := range sections {
			configSections = append(configSections, section)
		}
	}

	return map[string]interface{}{
		"path":            absPath,
		"size_bytes":      stat.Size(),
		"is_readable":     true,
		"is_directory":    stat.IsDir(),
		"mod_time":        stat.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		"config_sections": configSections,
		"config_keys":     configKeys,
		"total_keys":      len(configKeys),
	}, nil
}
