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
package dbzm

import (
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/pavlo-v-chernykh/keystore-go/v4"
)

// pgjdbc (internally used by debezium) only supports keys in DER format.
// https://github.com/pgjdbc/pgjdbc/issues/1364#issuecomment-447441182
func WritePKCS8PrivateKeyPEMasDER(sslKeyPath string, targetSslKeyPath string) error {
	privateKeyBytes, err := convertPKCS8PrivateKeyPEMtoDER(sslKeyPath)
	if err != nil {
		return fmt.Errorf("could not convert private key from PEM to DER: %w", err)
	}

	err = os.WriteFile(targetSslKeyPath, privateKeyBytes, 0400)
	if err != nil {
		return fmt.Errorf("could not write DER key: %w", err)
	}
	return nil
}

func convertPKCS8PrivateKeyPEMtoDER(pemFilePath string) ([]byte, error) {
	pkPEM, err := os.ReadFile(pemFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not read key file: %w", err)
	}

	b, _ := pem.Decode(pkPEM)
	if b == nil {
		return nil, fmt.Errorf("could not decode pem key file. Only PEM encoded keys are supported")
	}

	// downstream pgjdbc (used by debezium) expects PKCS8 DER format
	if b.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("could not decode pem key file. Expected PKCS8 standard(type=PRIVATE KEY), received type=%s.\n"+
			"You can use the following command to convert your key to PKCS8 standard - `openssl pkcs8 -topk8 -inform PEM -outform PEM -in <filename> -out <filename> -nocrypt`", b.Type)
	}
	return b.Bytes, nil
}

func WritePKCS8PrivateKeyCertAsJavaKeystore(sslKeyPath string, sslCertPath string, alias string, password string, filepath string) error {
	ks := keystore.New()

	pkBytes, err := convertPKCS8PrivateKeyPEMtoDER(sslKeyPath)
	if err != nil {
		return fmt.Errorf("could not convert private key from PEM to DER: %w", err)
	}
	certBytes, err := readCertificate(sslCertPath)
	if err != nil {
		return fmt.Errorf("could not read cert file: %w", err)
	}
	pke := keystore.PrivateKeyEntry{
		CreationTime: time.Now(),
		PrivateKey:   pkBytes,
		CertificateChain: []keystore.Certificate{
			{
				Type:    "X509",
				Content: certBytes,
			},
		},
	}

	err = ks.SetPrivateKeyEntry(alias, pke, []byte(password))
	if err != nil {
		return fmt.Errorf("failed to form keystore: %w", err)
	}
	err = writeKeyStore(ks, filepath, []byte(password))
	if err != nil {
		return fmt.Errorf("failed to write keystore: %w", err)
	}
	return nil
}

func WriteRootCertAsJavaTrustStore(sslRootCertPath string, alias string, password string, filepath string) error {
	ks := keystore.New()
	certBytes, err := readCertificate(sslRootCertPath)
	if err != nil {
		return fmt.Errorf("could not read cert file: %w", err)
	}
	tce := keystore.TrustedCertificateEntry{
		CreationTime: time.Now(),
		Certificate: keystore.Certificate{
			Type:    "X509",
			Content: certBytes,
		},
	}

	err = ks.SetTrustedCertificateEntry(alias, tce)
	if err != nil {
		return fmt.Errorf("failed to form truststore: %w", err)
	}
	err = writeKeyStore(ks, filepath, []byte(password))
	if err != nil {
		return fmt.Errorf("failed to write truststore: %w", err)
	}
	return nil
}

func readCertificate(filepath string) ([]byte, error) {
	pkPEM, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not read cert file: %w", err)
	}

	b, _ := pem.Decode(pkPEM)
	if b == nil {
		return nil, fmt.Errorf("invalid pem file. should have at least one pem block: %w", err)
	}

	if b.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid type. should be CERTIFICATE: %w", err)
	}
	return b.Bytes, nil
}

func writeKeyStore(ks keystore.KeyStore, filename string, password []byte) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("could not create keystore file: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	err = ks.Store(f, password)
	if err != nil {
		return err
	}
	return nil
}
