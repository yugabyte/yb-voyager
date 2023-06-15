package dbzm

import (
	"encoding/pem"
	"fmt"
	"os"
)

func ConvertPKCS8PrivateKeyPEMtoDER(pemFilePath string) ([]byte, error) {
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
