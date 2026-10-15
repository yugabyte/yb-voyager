//go:build unit

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
package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
