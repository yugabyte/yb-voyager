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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Source struct {
	DBType                string
	Host                  string
	Port                  int
	User                  string
	Password              string
	DBName                string
	CDBName               string
	DBSid                 string
	CDBSid                string
	OracleHome            string
	TNSAlias              string
	CDBTNSAlias           string
	Schema                string
	SSLMode               string
	SSLCertPath           string
	SSLKey                string
	SSLRootCert           string
	SSLCRL                string
	SSLQueryString        string
	SSLKeyStore           string
	SSLKeyStorePassword   string
	SSLTrustStore         string
	SSLTrustStorePassword string
	Uri                   string
	NumConnections        int
	VerboseMode           bool
	TableList             string
	ExcludeTableList      string
	UseOrafce             bool
	CommentsOnObjects     bool

	sourceDB SourceDB
}

func (s *Source) DB() SourceDB {
	if s.sourceDB == nil {
		s.sourceDB = newSourceDB(s)
	}
	return s.sourceDB
}

func (s *Source) GetOracleHome() string {
	if s.OracleHome != "" {
		return s.OracleHome
	} else {
		return "/usr/lib/oracle/21/client64"
	}
}

func (s *Source) IsOracleCDBSetup() bool {
	return (s.CDBName != "" || s.CDBTNSAlias != "" || s.CDBSid != "")
}

func parseSSLString(source *Source) {
	if source.SSLQueryString == "" {
		return
	}

	validParams := []string{"sslmode", "sslcert", "sslrootcert", "sslkey"}

	sslParams := strings.Split(source.SSLQueryString, "&")
	for _, param := range sslParams {
		slicedparam := strings.Split(param, "=")
		for i, checkParam := range validParams {
			if checkParam == slicedparam[0] {
				switch i {
				case 0:
					source.SSLMode = slicedparam[1]
				case 1:
					source.SSLCertPath = slicedparam[1]
				case 2:
					source.SSLRootCert = slicedparam[1]
				case 3:
					source.SSLKey = slicedparam[1]
				}
				break
			}
		}
	}
}

func (source *Source) PrepareSSLParamsForDebezium(exportDir string) error {
	switch source.DBType {
	case "postgresql", "yugabytedb": //TODO test for yugabytedb
		if source.SSLKey != "" {
			targetSslKeyPath := filepath.Join(exportDir, "metainfo", "ssl", ".key.der")
			err := dbzm.WritePKCS8PrivateKeyPEMasDER(source.SSLKey, targetSslKeyPath)
			if err != nil {
				return fmt.Errorf("could not write private key PEM as DER: %w", err)
			}
			utils.PrintAndLog("Converted SSL key from PEM to DER format. File saved at %s", targetSslKeyPath)
			source.SSLKey = targetSslKeyPath
		}
	case "mysql":
		switch source.SSLMode {
		case "disable":
			source.SSLMode = "disabled"
		case "prefer":
			source.SSLMode = "preferred"
		case "require":
			source.SSLMode = "required"
		case "verify-ca":
			source.SSLMode = "verify_ca"
		case "verify-full":
			source.SSLMode = "verify_identity"
		}
		if source.SSLKey != "" {
			keyStorePath := filepath.Join(exportDir, "metainfo", "ssl", ".keystore.jks")
			keyStorePassword := utils.GenerateRandomString(8)
			err := dbzm.WritePKCS8PrivateKeyCertAsJavaKeystore(source.SSLKey, source.SSLCertPath, "mysqlclient", keyStorePassword, keyStorePath)
			if err != nil {
				return fmt.Errorf("failed to write java keystore for debezium: %w", err)
			}
			utils.PrintAndLog("Converted SSL key, cert to java keystore. File saved at %s", keyStorePath)
			source.SSLKeyStore = keyStorePath
			source.SSLKeyStorePassword = keyStorePassword
		}
		if source.SSLRootCert != "" {
			trustStorePath := filepath.Join(exportDir, "metainfo", "ssl", ".truststore.jks")
			trustStorePassword := utils.GenerateRandomString(8)
			err := dbzm.WriteRootCertAsJavaTrustStore(source.SSLRootCert, "MySQLCACert", trustStorePassword, trustStorePath)
			if err != nil {
				return fmt.Errorf("failed to write java truststore for debezium: %w", err)
			}
			utils.PrintAndLog("Converted SSL root cert to java truststore. File saved at %s", trustStorePath)
			source.SSLTrustStore = trustStorePath
			source.SSLTrustStorePassword = trustStorePassword
		}
	case "oracle":
	}
	return nil
}
