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
package srcdb

import (
	"crypto/x509"
	"crypto/x509/pkix"
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
			assert.Equal(t, tt.expected, registeredDomain(tt.input))
		})
	}
}

func TestExtractSourceDomain(t *testing.T) {
	tests := []struct {
		name           string
		cert           *x509.Certificate
		host           string
		expectedDomain string
	}{
		{
			name:           "SAN preferred over host",
			cert:           &x509.Certificate{DNSNames: []string{"pg.abc.com"}},
			host:           "pg.other.com",
			expectedDomain: "abc.com",
		},
		{
			name:           "skips unusable SANs",
			cert:           &x509.Certificate{DNSNames: []string{"localhost", "postgres", "pg.abc.com"}},
			host:           "10.0.0.1",
			expectedDomain: "abc.com",
		},
		{
			name:           "common name used when no SANs",
			cert:           &x509.Certificate{Subject: pkix.Name{CommonName: "pg.abc.com"}},
			host:           "10.0.0.1",
			expectedDomain: "abc.com",
		},
		{
			name:           "unusable cert falls back to host",
			cert:           &x509.Certificate{DNSNames: []string{"localhost"}, Subject: pkix.Name{CommonName: "postgres"}},
			host:           "pg.abc.com",
			expectedDomain: "abc.com",
		},
		{
			name:           "no cert falls back to host",
			cert:           nil,
			host:           "pg.abc.com",
			expectedDomain: "abc.com",
		},
		{
			name:           "nothing usable",
			cert:           nil,
			host:           "127.0.0.1",
			expectedDomain: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedDomain, extractSourceDomain(tt.cert, tt.host))
		})
	}
}
