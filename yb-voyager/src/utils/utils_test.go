//go:build unit

package utils

import "testing"

func TestCamelCaseToTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"thisIsCamelCase", "This Is Camel Case"},
		{"simpleTest", "Simple Test"},
		{"JSONData", "JSON Data"},
		{"getHTTPResponseCode", "Get HTTP Response Code"},
		{"parseXMLFile", "Parse XML File"},
		{"parseXmlFile", "Parse Xml File"}, // Note: non-acronym variant
		{"convertToIDFormat", "Convert To ID Format"},
		{"userID", "User ID"},
		{"IPAddress", "IP Address"},
	}

	for _, tt := range tests {
		result := CamelCaseToTitleCase(tt.input)
		if result != tt.expected {
			t.Errorf("CamelCaseToTitleCase(%q) = %q; expected %q", tt.input, result, tt.expected)
		}
	}
}
