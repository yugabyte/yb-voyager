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

func TestExtractTLSCertDomain(t *testing.T) {
	tests := []struct {
		name           string
		cert           *x509.Certificate
		expectedDomain string
	}{
		{
			name:           "SAN",
			cert:           &x509.Certificate{DNSNames: []string{"pg.abc.com"}},
			expectedDomain: "abc.com",
		},
		{
			name:           "SAN preferred over CN",
			cert:           &x509.Certificate{DNSNames: []string{"pg.abc.com"}, Subject: pkix.Name{CommonName: "pg.other.com"}},
			expectedDomain: "abc.com",
		},
		{
			name:           "skips unusable SANs",
			cert:           &x509.Certificate{DNSNames: []string{"localhost", "postgres", "pg.abc.com"}},
			expectedDomain: "abc.com",
		},
		{
			name:           "CN used when no SANs",
			cert:           &x509.Certificate{Subject: pkix.Name{CommonName: "pg.abc.com"}},
			expectedDomain: "abc.com",
		},
		{
			name:           "CN used when no SAN is usable",
			cert:           &x509.Certificate{DNSNames: []string{"localhost"}, Subject: pkix.Name{CommonName: "pg.abc.com"}},
			expectedDomain: "abc.com",
		},
		{
			name:           "managed service collapses to provider",
			cert:           &x509.Certificate{DNSNames: []string{"mydb.abc123xyz.us-east-1.rds.amazonaws.com"}},
			expectedDomain: "amazonaws.com",
		},
		{
			name:           "no usable name",
			cert:           &x509.Certificate{DNSNames: []string{"localhost"}, Subject: pkix.Name{CommonName: "postgres"}},
			expectedDomain: "",
		},
		{
			name:           "no certificate",
			cert:           nil,
			expectedDomain: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedDomain, extractTLSCertDomain(tt.cert))
		})
	}
}
