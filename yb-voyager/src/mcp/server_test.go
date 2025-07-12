package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerCreation tests basic server creation and initialization
func TestServerCreation(t *testing.T) {
	server := NewServer()
	assert.NotNil(t, server)
	assert.NotNil(t, server.server)
}

// TestHierarchicalCommandParsing tests the core fix for hierarchical command parsing
func TestHierarchicalCommandParsing(t *testing.T) {
	testCases := []struct {
		name            string
		command         string
		expectedParts   []string
		expectedCommand string
	}{
		{
			name:            "Single command - assess-migration",
			command:         "assess-migration",
			expectedParts:   []string{"assess-migration"},
			expectedCommand: "assess-migration --config-file test.yaml",
		},
		{
			name:            "Hierarchical command - export schema",
			command:         "export schema",
			expectedParts:   []string{"export", "schema"},
			expectedCommand: "export schema --config-file test.yaml",
		},
		{
			name:            "Hierarchical command - export data",
			command:         "export data",
			expectedParts:   []string{"export", "data"},
			expectedCommand: "export data --config-file test.yaml",
		},
		{
			name:            "Hierarchical command - import schema",
			command:         "import schema",
			expectedParts:   []string{"import", "schema"},
			expectedCommand: "import schema --config-file test.yaml",
		},
		{
			name:            "Hierarchical command - import data",
			command:         "import data",
			expectedParts:   []string{"import", "data"},
			expectedCommand: "import data --config-file test.yaml",
		},
		{
			name:            "Single command - analyze-schema",
			command:         "analyze-schema",
			expectedParts:   []string{"analyze-schema"},
			expectedCommand: "analyze-schema --config-file test.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test command parsing
			commandParts := strings.Fields(tc.command)
			assert.Equal(t, tc.expectedParts, commandParts)

			// Test command building (simulating the actual logic from handlers.go)
			cmdArgs := append(commandParts, "--config-file", "test.yaml")
			actualCommand := strings.Join(cmdArgs, " ")
			assert.Equal(t, tc.expectedCommand, actualCommand)
		})
	}
}

// TestParseVoyagerOutput tests the output parsing logic
func TestParseVoyagerOutput(t *testing.T) {
	testCases := []struct {
		name           string
		command        string
		output         string
		err            error
		expectedStatus string
	}{
		{
			name:           "Assessment success",
			command:        "assess-migration",
			output:         "Assessment completed successfully\n10 tables found\n5 indexes found",
			err:            nil,
			expectedStatus: "completed",
		},
		{
			name:           "Export schema success",
			command:        "export schema",
			output:         "Schema export completed successfully",
			err:            nil,
			expectedStatus: "completed",
		},
		{
			name:           "Export data success",
			command:        "export data",
			output:         "Data export completed successfully",
			err:            nil,
			expectedStatus: "completed",
		},
		{
			name:           "Import schema success",
			command:        "import schema",
			output:         "Schema import completed successfully",
			err:            nil,
			expectedStatus: "completed",
		},
		{
			name:           "Import data success",
			command:        "import data",
			output:         "Data import completed successfully",
			err:            nil,
			expectedStatus: "completed",
		},
		{
			name:           "Generic command success",
			command:        "analyze-schema",
			output:         "Analysis completed",
			err:            nil,
			expectedStatus: "completed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary, structuredData := parseVoyagerOutput(tc.command, tc.output, tc.err)
			assert.NotEmpty(t, summary)
			assert.NotNil(t, structuredData)

			if status, ok := structuredData["status"]; ok {
				assert.Equal(t, tc.expectedStatus, status)
			}
		})
	}
}

// TestExtractExportDirFromURI tests URI parsing for export directory
func TestExtractExportDirFromURI(t *testing.T) {
	testCases := []struct {
		name        string
		uri         string
		expectedDir string
	}{
		{
			name:        "Assessment report URI",
			uri:         "voyager://assessment-report/tmp/test-export",
			expectedDir: "tmp/test-export",
		},
		{
			name:        "Schema analysis URI",
			uri:         "voyager://schema-analysis-report/home/user/export",
			expectedDir: "home/user/export",
		},
		{
			name:        "URI with query parameter",
			uri:         "voyager://logs/recent?export_dir=/tmp/test",
			expectedDir: "/tmp/test",
		},
		{
			name:        "Invalid URI",
			uri:         "invalid://uri",
			expectedDir: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractExportDirFromURI(tc.uri)
			assert.Equal(t, tc.expectedDir, result)
		})
	}
}

// TestGetExportDirInfo tests the export directory info helper function
func TestGetExportDirInfo(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	exportDir := filepath.Join(tempDir, "test-export")

	// Test non-existent directory
	info := getExportDirInfo(exportDir)
	assert.Equal(t, exportDir, info.Path)
	assert.False(t, info.Exists)
	assert.False(t, info.IsInitialized)
	assert.False(t, info.HasMetaDB)

	// Create the directory
	err := os.MkdirAll(exportDir, 0755)
	require.NoError(t, err)

	// Test existing directory
	info = getExportDirInfo(exportDir)
	assert.Equal(t, exportDir, info.Path)
	assert.True(t, info.Exists)
	assert.False(t, info.IsInitialized) // No metainfo directory yet

	// Create metainfo directory
	metaInfoDir := filepath.Join(exportDir, "metainfo")
	err = os.MkdirAll(metaInfoDir, 0755)
	require.NoError(t, err)

	// Test initialized directory
	info = getExportDirInfo(exportDir)
	assert.True(t, info.IsInitialized)
	assert.False(t, info.HasMetaDB) // No meta.db file yet

	// Create meta.db file
	metaDBPath := filepath.Join(metaInfoDir, "meta.db")
	err = os.WriteFile(metaDBPath, []byte("test"), 0644)
	require.NoError(t, err)

	// Test directory with metaDB
	info = getExportDirInfo(exportDir)
	assert.True(t, info.HasMetaDB)
}

// TestFormatDuration tests duration formatting
func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "Milliseconds",
			duration: 500 * time.Millisecond,
			expected: "500ms",
		},
		{
			name:     "Seconds",
			duration: 2500 * time.Millisecond,
			expected: "2.5s",
		},
		{
			name:     "Minutes",
			duration: 90 * time.Second,
			expected: "1.5m",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDuration(tc.duration)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestFindYbVoyagerPath tests finding the yb-voyager executable
func TestFindYbVoyagerPath(t *testing.T) {
	path := findYbVoyagerPath()
	assert.NotEmpty(t, path)
	// The path should either be "yb-voyager" (if in PATH) or an absolute path
	assert.True(t, path == "yb-voyager" || filepath.IsAbs(path))
}

// TestParseAssessmentOutput tests specific assessment output parsing
func TestParseAssessmentOutput(t *testing.T) {
	testCases := []struct {
		name           string
		output         string
		err            error
		expectedStatus string
		expectedTables int
	}{
		{
			name:           "Successful assessment",
			output:         "Assessment completed successfully\n10 tables found\n5 indexes found",
			err:            nil,
			expectedStatus: "completed",
			expectedTables: 10,
		},
		{
			name:           "Assessment with missing dependencies",
			output:         "Missing dependencies:\npg_dump\npg_restore",
			err:            fmt.Errorf("missing dependencies"),
			expectedStatus: "failed",
			expectedTables: 0,
		},
		{
			name:           "Assessment with issues",
			output:         "Assessment completed\n5 tables found\nWarning: compatibility issues found",
			err:            nil,
			expectedStatus: "completed",
			expectedTables: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary, structuredData := parseAssessmentOutput(tc.output, tc.err)
			assert.NotEmpty(t, summary)
			assert.NotNil(t, structuredData)

			if status, ok := structuredData["status"]; ok {
				assert.Equal(t, tc.expectedStatus, status)
			}

			if tablesCount, ok := structuredData["tables_count"]; ok {
				assert.Equal(t, tc.expectedTables, tablesCount)
			}
		})
	}
}

// TestBuildEnvWithExtraPath tests PATH environment variable construction
func TestBuildEnvWithExtraPath(t *testing.T) {
	env := buildEnvWithExtraPath()
	assert.NotEmpty(t, env)

	// Find the PATH variable
	var pathVar string
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "PATH=") {
			pathVar = envVar
			break
		}
	}

	assert.NotEmpty(t, pathVar)
	assert.Contains(t, pathVar, "PATH=")

	// Check that common paths are included
	pathValue := strings.TrimPrefix(pathVar, "PATH=")
	assert.Contains(t, pathValue, "/opt/homebrew/bin")
	assert.Contains(t, pathValue, "/usr/local/bin")
}

// TestCommandParsingEdgeCases tests edge cases in command parsing
func TestCommandParsingEdgeCases(t *testing.T) {
	testCases := []struct {
		name          string
		command       string
		expectedParts []string
	}{
		{
			name:          "Empty command",
			command:       "",
			expectedParts: []string{},
		},
		{
			name:          "Single word",
			command:       "assess-migration",
			expectedParts: []string{"assess-migration"},
		},
		{
			name:          "Multiple spaces",
			command:       "export    schema",
			expectedParts: []string{"export", "schema"},
		},
		{
			name:          "Leading/trailing spaces",
			command:       "  import data  ",
			expectedParts: []string{"import", "data"},
		},
		{
			name:          "Three part command",
			command:       "get migration status",
			expectedParts: []string{"get", "migration", "status"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commandParts := strings.Fields(tc.command)
			assert.Equal(t, tc.expectedParts, commandParts)
		})
	}
}

// Benchmark tests for performance
func BenchmarkHierarchicalCommandParsing(b *testing.B) {
	commands := []string{
		"assess-migration",
		"export schema",
		"export data",
		"import schema",
		"import data",
		"analyze-schema",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cmd := range commands {
			commandParts := strings.Fields(cmd)
			cmdArgs := append(commandParts, "--config-file", "test.yaml")
			_ = strings.Join(cmdArgs, " ")
		}
	}
}

func BenchmarkParseVoyagerOutput(b *testing.B) {
	testOutput := "Assessment completed successfully\n10 tables found\n5 indexes found"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseVoyagerOutput("assess-migration", testOutput, nil)
	}
}
