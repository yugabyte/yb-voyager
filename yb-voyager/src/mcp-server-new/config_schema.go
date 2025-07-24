package mcpservernew

import (
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/config"
)

// ConfigSchema provides schema information for YB Voyager configuration
type ConfigSchema struct{}

// NewConfigSchema creates a new config schema provider
func NewConfigSchema() *ConfigSchema {
	return &ConfigSchema{}
}

// GetSchemaInfo returns schema information for a specific section
func (cs *ConfigSchema) GetSchemaInfo(section string) (map[string]interface{}, error) {
	if section == "" {
		return nil, fmt.Errorf("section name is required")
	}

	// Normalize section name (convert to lowercase and replace spaces with hyphens)
	normalizedSection := strings.ToLower(strings.ReplaceAll(section, " ", "-"))

	// Handle global section specially
	if normalizedSection == "global" {
		keysSlice := config.AllowedGlobalConfigKeys.ToSlice()
		return map[string]interface{}{
			"section":     normalizedSection,
			"description": getSectionDescription(normalizedSection),
			"all_keys":    keysSlice,
			"total_keys":  len(keysSlice),
		}, nil
	}

	// Get the allowed keys from the validation logic
	allowedKeys, exists := config.AllowedConfigSections[normalizedSection]
	if !exists {
		// Provide helpful error message with available sections
		availableSections := make([]string, 0, len(config.AllowedConfigSections)+1) // +1 for global
		availableSections = append(availableSections, "global")
		for section := range config.AllowedConfigSections {
			availableSections = append(availableSections, section)
		}

		return map[string]interface{}{
			"error":              fmt.Sprintf("Invalid section '%s'. Available sections are: %s", section, strings.Join(availableSections, ", ")),
			"available_sections": availableSections,
			"requested_section":  section,
		}, nil
	}

	// Convert the set to a slice for easier handling
	keysSlice := allowedKeys.ToSlice()

	// Return schema information
	return map[string]interface{}{
		"section":     normalizedSection,
		"description": getSectionDescription(normalizedSection),
		"all_keys":    keysSlice,
		"total_keys":  len(keysSlice),
	}, nil
}

// getSectionDescription returns a description for a given section
func getSectionDescription(section string) string {
	descriptions := map[string]string{
		"source":                           "Source database configuration",
		"source-replica":                   "Source replica database configuration",
		"target":                           "Target database configuration",
		"assess-migration":                 "Migration assessment configuration",
		"analyze-schema":                   "Schema analysis configuration",
		"export-schema":                    "Schema export configuration",
		"export-data":                      "Data export configuration",
		"export-data-from-source":          "Data export from source configuration (alias for export-data)",
		"export-data-from-target":          "Data export from target configuration",
		"import-schema":                    "Schema import configuration",
		"finalize-schema-post-data-import": "Schema finalization after data import configuration",
		"import-data":                      "Data import configuration",
		"import-data-to-target":            "Data import to target configuration (alias for import-data)",
		"import-data-to-source":            "Data import to source configuration",
		"import-data-to-source-replica":    "Data import to source replica configuration",
		"import-data-file":                 "Data import from file configuration",
		"initiate-cutover-to-target":       "Cutover to target configuration",
		"archive-changes":                  "Archive changes configuration",
		"end-migration":                    "End migration configuration",
		"global":                           "Global configuration keys (not in any section)",
	}

	if desc, exists := descriptions[section]; exists {
		return desc
	}
	return "Configuration section"
}

// GetAllSections returns information about all available sections
func (cs *ConfigSchema) GetAllSections() (map[string]interface{}, error) {
	sections := make([]string, 0, len(config.AllowedConfigSections)+1) // +1 for global
	sectionDescriptions := make(map[string]string)

	// Add global section first
	sections = append(sections, "global")
	sectionDescriptions["global"] = getSectionDescription("global")

	// Add all other sections
	for section := range config.AllowedConfigSections {
		sections = append(sections, section)
		sectionDescriptions[section] = getSectionDescription(section)
	}

	return map[string]interface{}{
		"total_sections":       len(sections),
		"sections":             sections,
		"section_descriptions": sectionDescriptions,
	}, nil
}
