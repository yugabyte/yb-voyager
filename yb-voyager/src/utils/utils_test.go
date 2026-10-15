//go:build unit

package utils

import (
	"testing"

	"gotest.tools/assert"
)

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

func TestRegisteredDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"self-managed FQDN", "pg-prod-01.db.abc.com", "abc.com"},
		{"multi-label public suffix", "db1.abc.co.uk", "abc.co.uk"},
		{"wildcard SAN", "*.abc.com", "abc.com"},
		{"upper case and trailing dot", "DB.abc.COM.", "abc.com"},
		{"registered domain itself", "abc.com", "abc.com"},
		{"aws rds collapses to provider", "mydb.abc123xyz.us-east-1.rds.amazonaws.com", "amazonaws.com"},
		{"aws aurora wildcard collapses to provider", "*.cluster-abc123xyz.us-east-1.rds.amazonaws.com", "amazonaws.com"},
		{"azure flexible server collapses to provider", "myserver.postgres.database.azure.com", "azure.com"},
		{"gcp cloud sql", "1-abc.us-central1.sql.goog", "sql.goog"},
		{"localhost", "localhost", ""},
		{"single label", "postgres", ""},
		{"ipv4", "10.0.0.12", ""},
		{"ipv6", "::1", ""},
		{"non-ICANN internal tld", "db.corp.internal", ""},
		{"mdns local", "db.local", ""},
		{"bare public suffix", "co.uk", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, RegisteredDomain(tt.input))
		})
	}
}
