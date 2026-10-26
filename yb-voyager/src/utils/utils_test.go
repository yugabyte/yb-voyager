//go:build unit

package utils

import (
	"net/url"
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

func TestInstanceQualifiedDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"aws rds instance", "mydb.abc123xyz.us-east-1.rds.amazonaws.com", "mydb.*.amazonaws.com"},
		{"aws aurora cluster", "mycluster.cluster-abc123xyz.us-east-1.rds.amazonaws.com", "mycluster.*.amazonaws.com"},
		{"azure flexible server", "myserver.postgres.database.azure.com", "myserver.*.azure.com"},
		{"gcp cloud sql", "1-abc.us-central1.sql.goog", "1-abc.*.sql.goog"},
		{"self-managed FQDN", "pg01.db.abc.com", "pg01.*.abc.com"},
		{"instance directly under domain", "pg01.abc.com", "pg01.abc.com"},
		{"multi-label public suffix", "db1.abc.co.uk", "db1.abc.co.uk"},
		{"multi-label public suffix with middle labels", "db1.eu.abc.co.uk", "db1.*.abc.co.uk"},
		{"upper case and trailing dot", "PG01.DB.abc.COM.", "pg01.*.abc.com"},
		{"registered domain itself", "abc.com", "abc.com"},
		{"wildcard has no instance", "*.db.abc.com", "abc.com"},
		{"localhost", "localhost", ""},
		{"single label", "postgres", ""},
		{"ipv4", "10.0.0.12", ""},
		{"ipv6", "::1", ""},
		{"non-ICANN internal tld", "db.corp.internal", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, InstanceQualifiedDomain(tt.input))
		})
	}
}

func TestEscapeConnUriQueryValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "plain path", input: "/certs/root.crt", expected: "%2Fcerts%2Froot.crt"},
		{name: "space", input: "/my certs/root.crt", expected: "%2Fmy%20certs%2Froot.crt"},
		{name: "plus", input: "/c++/root.crt", expected: "%2Fc%2B%2B%2Froot.crt"},
		{name: "percent", input: "/100%/root.crt", expected: "%2F100%25%2Froot.crt"},
		{name: "query delimiters", input: "/a&b=c/root.crt", expected: "%2Fa%26b%3Dc%2Froot.crt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escaped := EscapeConnUriQueryValue(tt.input)
			assert.Equal(t, tt.expected, escaped)

			// libpq and pgx (>= v5.11) decode URI query values with "+" as a literal plus, as url.PathUnescape does.
			decoded, err := url.PathUnescape(escaped)
			assert.NilError(t, err)
			assert.Equal(t, tt.input, decoded)
		})
	}
}
