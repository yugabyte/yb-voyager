package testutils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

const testPKIValidity = 24 * time.Hour

// TestPKI is a throwaway certificate authority for TLS integration tests. Nothing it issues is
// meant to outlive the test process.
type TestPKI struct {
	CACertPEM []byte
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
}

func NewTestPKI() (*TestPKI, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "yb-voyager test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(testPKIValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("self-sign CA certificate: %w", err)
	}
	caCert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}
	return &TestPKI{
		CACertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		caCert:    caCert,
		caKey:     caKey,
	}, nil
}

// IssueServerCert returns a PEM certificate and PKCS#8 PEM key valid for the given host names
// and IP addresses, so that clients using sslmode=verify-full accept it.
func (p *TestPKI) IssueServerCert(commonName string, dnsNames []string, ips []net.IP) (certPEM []byte, keyPEM []byte, err error) {
	return p.issue(&x509.Certificate{
		Subject:     pkix.Name{CommonName: commonName},
		DNSNames:    dnsNames,
		IPAddresses: ips,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
}

// IssueClientCert returns a PEM certificate and PKCS#8 PEM key whose CN is dbUser, which is
// what `cert` authentication in pg_hba.conf matches against the connecting role.
func (p *TestPKI) IssueClientCert(dbUser string) (certPEM []byte, keyPEM []byte, err error) {
	return p.issue(&x509.Certificate{
		Subject:     pkix.Name{CommonName: dbUser},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
}

func (p *TestPKI) issue(template *x509.Certificate) ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key for %q: %w", template.Subject.CommonName, err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial for %q: %w", template.Subject.CommonName, err)
	}
	template.SerialNumber = serial
	template.NotBefore = time.Now().Add(-time.Hour)
	template.NotAfter = time.Now().Add(testPKIValidity)
	template.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	der, err := x509.CreateCertificate(rand.Reader, template, p.caCert, &key.PublicKey, p.caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("sign certificate for %q: %w", template.Subject.CommonName, err)
	}
	// voyager converts the source key to DER for Debezium and accepts only PKCS#8 PEM there.
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal key for %q: %w", template.Subject.CommonName, err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}
