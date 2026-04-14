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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const voyagerHomeDir = ".yb-voyager"

// validMigrationNameRegex allows alphanumeric, hyphens, and underscores
var validMigrationNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// getVoyagerHome returns the path to the voyager home directory ($HOME/.yb-voyager)
func getVoyagerHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		utils.ErrExit("failed to get user home directory: %v", err)
	}
	return filepath.Join(home, voyagerHomeDir)
}

// ensureVoyagerHome creates the voyager home directory if it doesn't exist
func ensureVoyagerHome() error {
	voyagerHome := getVoyagerHome()
	if err := os.MkdirAll(voyagerHome, 0755); err != nil {
		return fmt.Errorf("failed to create voyager home directory %q: %w", voyagerHome, err)
	}
	return nil
}

// isValidMigrationName checks if the migration name is filesystem-safe
func isValidMigrationName(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > 255 {
		return false
	}
	return validMigrationNameRegex.MatchString(name)
}

// getMigrationPath returns the actual path for a migration name.
// It checks for:
// 1. A directory at $HOME/.yb-voyager/<name>
// 2. A symlink at $HOME/.yb-voyager/<name>
// 3. A .path file at $HOME/.yb-voyager/<name>.path containing the path
func getMigrationPath(name string) (string, error) {
	if !isValidMigrationName(name) {
		return "", fmt.Errorf("invalid migration name %q: must be alphanumeric with hyphens or underscores", name)
	}

	voyagerHome := getVoyagerHome()
	migrationPath := filepath.Join(voyagerHome, name)

	// Check if directory or symlink exists
	info, err := os.Lstat(migrationPath)
	if err == nil {
		if info.IsDir() {
			return migrationPath, nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink, resolve it
			resolved, err := filepath.EvalSymlinks(migrationPath)
			if err != nil {
				return "", fmt.Errorf("migration %q symlink is broken: %w", name, err)
			}
			return resolved, nil
		}
	}

	// Check for .path file fallback
	pathFile := filepath.Join(voyagerHome, name+".path")
	if data, err := os.ReadFile(pathFile); err == nil {
		resolvedPath := strings.TrimSpace(string(data))
		if resolvedPath != "" {
			// Verify the path exists
			if _, err := os.Stat(resolvedPath); err != nil {
				return "", fmt.Errorf("migration %q path file points to non-existent directory: %s", name, resolvedPath)
			}
			return resolvedPath, nil
		}
	}

	return "", fmt.Errorf("migration %q not found", name)
}

// getMigrationConfigPath returns the config.yaml path for a migration name
func getMigrationConfigPath(name string) (string, error) {
	migrationPath, err := getMigrationPath(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(migrationPath, "config.yaml"), nil
}

// listMigrations returns all migration names in the registry
func listMigrations() ([]string, error) {
	voyagerHome := getVoyagerHome()

	// Check if voyager home exists
	if _, err := os.Stat(voyagerHome); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(voyagerHome)
	if err != nil {
		return nil, fmt.Errorf("failed to read voyager home directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip .path files when listing (they're handled separately)
		if strings.HasSuffix(name, ".path") {
			// Extract the migration name and check if it's valid
			migrationName := strings.TrimSuffix(name, ".path")
			if isValidMigrationName(migrationName) {
				// Check if the path file points to a valid directory
				if _, err := getMigrationPath(migrationName); err == nil {
					// Only add if not already in list (as directory/symlink)
					found := false
					for _, m := range migrations {
						if m == migrationName {
							found = true
							break
						}
					}
					if !found {
						migrations = append(migrations, migrationName)
					}
				}
			}
			continue
		}

		// Check if it's a directory or symlink
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			// Verify it's a valid migration (has config.yaml or is a valid symlink target)
			migrationPath := filepath.Join(voyagerHome, name)

			// For symlinks, check if target exists
			if info.Mode()&os.ModeSymlink != 0 {
				if _, err := filepath.EvalSymlinks(migrationPath); err != nil {
					// Broken symlink, skip with warning
					fmt.Fprintf(os.Stderr, "Warning: migration %q has broken symlink, skipping\n", name)
					continue
				}
			}

			migrations = append(migrations, name)
		}
	}

	return migrations, nil
}

// migrationExists checks if a migration with the given name already exists
func migrationExists(name string) bool {
	_, err := getMigrationPath(name)
	return err == nil
}

// registerMigration registers a migration in the voyager home directory.
// If the migration path is already inside voyager home, no action is needed.
// If it's a custom path, create a symlink (or .path file as fallback).
func registerMigration(name, path string) error {
	if !isValidMigrationName(name) {
		return fmt.Errorf("invalid migration name %q: must be alphanumeric with hyphens or underscores", name)
	}

	if err := ensureVoyagerHome(); err != nil {
		return err
	}

	voyagerHome := getVoyagerHome()
	registryPath := filepath.Join(voyagerHome, name)

	// Make path absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// If path is already inside voyager home, it's already registered
	// by virtue of being a directory there — no symlink needed.
	if strings.HasPrefix(absPath, voyagerHome+string(os.PathSeparator)) {
		return nil
	}

	// Check if registration already exists
	if _, err := os.Lstat(registryPath); err == nil {
		return fmt.Errorf("migration %q already registered", name)
	}

	// Try to create symlink
	err = os.Symlink(absPath, registryPath)
	if err == nil {
		return nil
	}

	// Symlink failed, use .path file fallback
	pathFile := filepath.Join(voyagerHome, name+".path")
	if err := os.WriteFile(pathFile, []byte(absPath), 0644); err != nil {
		return fmt.Errorf("failed to register migration (symlink and path file both failed): %w", err)
	}

	return nil
}

// ErrNoMigrations is returned when no migrations exist in the registry.
type ErrNoMigrations struct{}

func (e ErrNoMigrations) Error() string {
	return "no migrations found"
}

// ErrMultipleMigrations is returned when multiple migrations exist and none was specified.
type ErrMultipleMigrations struct {
	Names []string
}

func (e ErrMultipleMigrations) Error() string {
	return fmt.Sprintf("multiple migrations found: %s", strings.Join(e.Names, ", "))
}

// resolveMigration auto-detects the migration to use.
// Returns (migrationName, configPath, error)
// If exactly one migration exists, returns it.
// If zero migrations, returns ErrNoMigrations.
// If multiple migrations, returns ErrMultipleMigrations with the list.
func resolveMigration() (string, string, error) {
	migrations, err := listMigrations()
	if err != nil {
		return "", "", err
	}

	switch len(migrations) {
	case 0:
		return "", "", ErrNoMigrations{}
	case 1:
		name := migrations[0]
		configPath, err := getMigrationConfigPath(name)
		if err != nil {
			return "", "", err
		}
		return name, configPath, nil
	default:
		return "", "", ErrMultipleMigrations{Names: migrations}
	}
}

// getDefaultMigrationPath returns the default path for a new migration
func getDefaultMigrationPath(name string) string {
	return filepath.Join(getVoyagerHome(), name)
}

// shouldShowMigrationError returns true if the error is a migration resolution error
// that should be shown to the user with formatted output.
func shouldShowMigrationError(err error) bool {
	switch err.(type) {
	case ErrNoMigrations, ErrMultipleMigrations:
		return true
	default:
		return false
	}
}

// printMigrationResolutionError prints a formatted error message for migration resolution errors.
func printMigrationResolutionError(err error, cmdPath string) {
	switch e := err.(type) {
	case ErrMultipleMigrations:
		fmt.Println(color.RedString("Error:") + " multiple migrations found. Specify which one to use:\n")
		for _, name := range e.Names {
			fmt.Printf("  %s\n", name)
		}
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Printf("  %s --migration-name <name>\n", cmdPath)
	case ErrNoMigrations:
		fmt.Println(color.RedString("Error:") + " no migrations found.")
		fmt.Println()
		fmt.Println("Create one with:")
		fmt.Println("  yb-voyager new")
	default:
		fmt.Println(color.RedString("Error:") + " " + err.Error())
	}
}

// buildMigrationNameFlag returns " --migration-name <name>" if multiple migrations exist
// in the registry, or empty string if auto-detection is sufficient (0 or 1 migration).
// The name is resolved from migrationName flag, migrationDir, cfgFile path, or exportDir.
func buildMigrationNameFlag() string {
	migrations, err := listMigrations()
	if err != nil || len(migrations) <= 1 {
		return ""
	}

	// Multiple migrations: determine which one is active
	name := migrationName

	// Try to infer from migrationDir (set during 'yb-voyager new')
	if name == "" && migrationDir != "" {
		voyagerHome := getVoyagerHome()
		if strings.HasPrefix(migrationDir, voyagerHome+string(os.PathSeparator)) {
			name = filepath.Base(migrationDir)
		}
	}

	// Try to infer from config file path
	if name == "" && cfgFile != "" {
		voyagerHome := getVoyagerHome()
		dir := filepath.Dir(cfgFile)
		if strings.HasPrefix(dir, voyagerHome+string(os.PathSeparator)) {
			name = filepath.Base(dir)
		}
	}

	// Try to infer from exportDir (set by config for most commands)
	if name == "" && exportDir != "" {
		voyagerHome := getVoyagerHome()
		dir := filepath.Dir(exportDir)
		if strings.HasPrefix(dir, voyagerHome+string(os.PathSeparator)) {
			name = filepath.Base(dir)
		}
	}

	if name == "" {
		return ""
	}
	return fmt.Sprintf(" --migration-name %s", name)
}
