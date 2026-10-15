//go:build integration

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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

type tlsTestCertSpec struct {
	commonName string
	dnsNames   []string
	withIPSAN  bool
}

type tlsTestCheck struct {
	name           string
	sslMode        string
	expectedDomain string
}

func TestFetchTLSCertDomainName(t *testing.T) {
	ctx := context.Background()
	pgContainer, ok := testPostgresSource.TestContainer.(*testcontainers.PostgresContainer)
	require.True(t, ok, "unexpected postgres test container type %T", testPostgresSource.TestContainer)
	// The container is shared with the other srcdb integration tests.
	t.Cleanup(func() {
		if err := pgContainer.DisableTLS(context.Background()); err != nil {
			t.Errorf("disable TLS on shared postgres container: %v", err)
		}
	})

	ca := newTLSTestCA(t)
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(caFile, ca.certPEM, 0o600))

	tests := []struct {
		name   string
		cert   tlsTestCertSpec
		checks []tlsTestCheck
	}{
		{
			name: "company certificate",
			cert: tlsTestCertSpec{commonName: "pg01.db.acme.com", dnsNames: []string{"localhost", "pg01.db.acme.com", "*.acme.com"}, withIPSAN: true},
			checks: []tlsTestCheck{
				{name: "require", sslMode: "require", expectedDomain: "acme.com"},
				{name: "prefer negotiates TLS", sslMode: "prefer", expectedDomain: "acme.com"},
				{name: "verify-full is unaffected", sslMode: "verify-full", expectedDomain: "acme.com"},
				{name: "no TLS gives nothing", sslMode: "disable", expectedDomain: ""},
			},
		},
		{
			name:   "multi-label public suffix",
			cert:   tlsTestCertSpec{commonName: "db1.globex.co.uk", dnsNames: []string{"db1.globex.co.uk"}},
			checks: []tlsTestCheck{{name: "require", sslMode: "require", expectedDomain: "globex.co.uk"}},
		},
		{
			name:   "aws rds collapses to provider",
			cert:   tlsTestCertSpec{commonName: "mydb.abc123xyz.us-east-1.rds.amazonaws.com", dnsNames: []string{"mydb.abc123xyz.us-east-1.rds.amazonaws.com"}},
			checks: []tlsTestCheck{{name: "require", sslMode: "require", expectedDomain: "amazonaws.com"}},
		},
		{
			name:   "azure collapses to provider",
			cert:   tlsTestCertSpec{commonName: "myserver.postgres.database.azure.com", dnsNames: []string{"myserver.postgres.database.azure.com"}},
			checks: []tlsTestCheck{{name: "require", sslMode: "require", expectedDomain: "azure.com"}},
		},
		{
			name:   "gcp cloud sql",
			cert:   tlsTestCertSpec{commonName: "1-abc.us-central1.sql.goog", dnsNames: []string{"1-abc.us-central1.sql.goog"}},
			checks: []tlsTestCheck{{name: "require", sslMode: "require", expectedDomain: "sql.goog"}},
		},
		{
			name:   "CN used when certificate has no SAN",
			cert:   tlsTestCertSpec{commonName: "pg.initech.com"},
			checks: []tlsTestCheck{{name: "require", sslMode: "require", expectedDomain: "initech.com"}},
		},
		{
			name:   "certificate without a usable name",
			cert:   tlsTestCertSpec{commonName: "localhost", dnsNames: []string{"localhost", "postgres"}, withIPSAN: true},
			checks: []tlsTestCheck{{name: "require", sslMode: "require", expectedDomain: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPEM, keyPEM := ca.issueServerCert(t, tt.cert)
			require.NoError(t, pgContainer.EnableTLS(ctx, certPEM, keyPEM))

			for _, check := range tt.checks {
				t.Run(check.name, func(t *testing.T) {
					source := &Source{
						DBType:   testPostgresSource.DBType,
						Host:     testPostgresSource.Host,
						Port:     testPostgresSource.Port,
						User:     testPostgresSource.User,
						Password: testPostgresSource.Password,
						DBName:   testPostgresSource.DBName,
						SSLMode:  check.sslMode,
					}
					if check.sslMode == "verify-full" {
						source.SSLRootCert = caFile
					}
					require.NoError(t, source.DB().Connect())
					defer source.DB().Disconnect()

					require.NoError(t, source.DB().FetchTLSCertDomainName())
					assert.Equal(t, check.expectedDomain, source.TLSCertDomain)
				})
			}
		})
	}
}

type tlsTestCA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
}

func newTLSTestCA(t *testing.T) tlsTestCA {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          newTLSTestSerial(t),
		Subject:               pkix.Name{CommonName: "voyager-tls-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return tlsTestCA{
		cert:    cert,
		key:     key,
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}
}

func (ca tlsTestCA) issueServerCert(t *testing.T, spec tlsTestCertSpec) (certPEM []byte, keyPEM []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: newTLSTestSerial(t),
		Subject:      pkix.Name{CommonName: spec.commonName},
		DNSNames:     spec.dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if spec.withIPSAN {
		template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func newTLSTestSerial(t *testing.T) *big.Int {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	require.NoError(t, err)
	return serial
}
