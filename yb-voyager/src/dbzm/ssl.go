package dbzm

import (
	"encoding/pem"
	"fmt"
	"os"
)

// pgjdbc (internally used by debezium) only supports keys in DER format.
// https://github.com/pgjdbc/pgjdbc/issues/1364#issuecomment-447441182
func WritePKCS8PrivateKeyPEMasDER(sslKeyPath string, targetSslKeyPath string) error {
	privateKeyBytes, err := convertPKCS8PrivateKeyPEMtoDER(sslKeyPath)
	if err != nil {
		return fmt.Errorf("could not convert private key from PEM to DER: %w", err)
	}

	err = os.WriteFile(targetSslKeyPath, privateKeyBytes, 0600)
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
		return nil, fmt.Errorf("could not decode pem key file")
	}

	// downstream pgjdbc (used by debezium) expects PKCS8 DER format
	if b.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("could not decode pem key file. Expected PKCS8 standard. (type=PRIVATE KEY), received type=%s", b.Type)
	}
	return b.Bytes, nil
}
