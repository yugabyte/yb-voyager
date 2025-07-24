package mcpservernew

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfigSchema tests the config schema functionality
func TestConfigSchema(t *testing.T) {
	cs := NewConfigSchema()

	// Test with empty section
	result, err := cs.GetSchemaInfo("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "section name is required")
	fmt.Printf("Empty section error: %v\n", err)

	// Test with valid section
	result, err = cs.GetSchemaInfo("source")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "source", result["section"])
	assert.Equal(t, "Source database configuration", result["description"])
	assert.IsType(t, []string{}, result["all_keys"])
	fmt.Printf("Source schema result: %+v\n", result)

	// Test with invalid section
	result, err = cs.GetSchemaInfo("invalid-section")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result["error"].(string), "Invalid section")
	assert.Contains(t, result["error"].(string), "Available sections are")
	fmt.Printf("Invalid section error: %+v\n", result)

	// Test with case-insensitive section name
	result, err = cs.GetSchemaInfo("TARGET")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "target", result["section"])
	fmt.Printf("Case-insensitive target result: %+v\n", result)

	// Test with spaces in section name
	result, err = cs.GetSchemaInfo("export data")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "export-data", result["section"])
	fmt.Printf("Spaces in section name result: %+v\n", result)
}

// TestConfigSchema_GetAllSections tests the GetAllSections functionality
func TestConfigSchema_GetAllSections(t *testing.T) {
	cs := NewConfigSchema()

	result, err := cs.GetAllSections()
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.IsType(t, 0, result["total_sections"])
	assert.IsType(t, []string{}, result["sections"])
	assert.IsType(t, map[string]string{}, result["section_descriptions"])

	totalSections := result["total_sections"].(int)
	assert.Greater(t, totalSections, 0)

	sections := result["sections"].([]string)
	assert.Equal(t, totalSections, len(sections))

	sectionDescriptions := result["section_descriptions"].(map[string]string)
	assert.Equal(t, totalSections, len(sectionDescriptions))

	fmt.Printf("All sections result: %+v\n", result)
}

// TestConfigSchema_SpecificSections tests specific sections
func TestConfigSchema_SpecificSections(t *testing.T) {
	cs := NewConfigSchema()

	testCases := []struct {
		name         string
		section      string
		expectedKeys int
		description  string
	}{
		{
			name:         "Source section",
			section:      "source",
			expectedKeys: 19,
			description:  "Source database configuration",
		},
		{
			name:         "Target section",
			section:      "target",
			expectedKeys: 12,
			description:  "Target database configuration",
		},
		{
			name:         "Export data section",
			section:      "export-data",
			expectedKeys: 12,
			description:  "Data export configuration",
		},
		{
			name:         "Import data section",
			section:      "import-data",
			expectedKeys: 34,
			description:  "Data import configuration",
		},
		{
			name:         "Global section",
			section:      "global",
			expectedKeys: 11,
			description:  "Global configuration keys (not in any section)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cs.GetSchemaInfo(tc.section)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tc.section, result["section"])
			assert.Equal(t, tc.description, result["description"])
			assert.Equal(t, tc.expectedKeys, result["total_keys"])

			allKeys := result["all_keys"].([]string)
			assert.Equal(t, tc.expectedKeys, len(allKeys))

			fmt.Printf("%s: %+v\n", tc.name, result)
		})
	}
}

// TestConfigSchema_EdgeCases tests edge cases for config schema
func TestConfigSchema_EdgeCases(t *testing.T) {
	cs := NewConfigSchema()

	testCases := []struct {
		name        string
		section     string
		expectError bool
		description string
	}{
		{
			name:        "Empty string",
			section:     "",
			expectError: true,
			description: "Should fail with empty section",
		},
		{
			name:        "Whitespace only",
			section:     "   ",
			expectError: false,
			description: "Should normalize whitespace",
		},
		{
			name:        "Mixed case",
			section:     "ExPoRt-DaTa",
			expectError: false,
			description: "Should handle mixed case",
		},
		{
			name:        "With underscores",
			section:     "export_data",
			expectError: false,
			description: "Should handle underscores",
		},
		{
			name:        "Non-existent section",
			section:     "non-existent",
			expectError: false,
			description: "Should return error with available sections",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cs.GetSchemaInfo(tc.section)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "section name is required")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
			fmt.Printf("%s: %+v\n", tc.name, result)
		})
	}
}
