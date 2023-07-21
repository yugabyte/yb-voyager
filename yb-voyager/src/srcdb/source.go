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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"

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

func createTLSConf(source *Source) tls.Config {
	rootCertPool := x509.NewCertPool()
	if source.SSLRootCert != "" {
		pem, err := os.ReadFile(source.SSLRootCert)
		if err != nil {
			utils.ErrExit("error in reading SSL Root Certificate: %v", err)
		}

		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			utils.ErrExit("Failed to append PEM.")
		}
	} else {
		utils.ErrExit("Root Certificate Needed for verify-ca and verify-full SSL Modes")
	}
	clientCert := make([]tls.Certificate, 0, 1)

	if source.SSLCertPath != "" && source.SSLKey != "" {
		certs, err := tls.LoadX509KeyPair(source.SSLCertPath, source.SSLKey)
		if err != nil {
			utils.ErrExit("error in reading and parsing SSL KeyPair: %v", err)
		}

		clientCert = append(clientCert, certs)
	}

	if source.SSLMode == "verify-ca" {
		return tls.Config{
			RootCAs:            rootCertPool,
			Certificates:       clientCert,
			InsecureSkipVerify: true,
		}
	} else { //if verify-full

		return tls.Config{
			RootCAs:            rootCertPool,
			Certificates:       clientCert,
			InsecureSkipVerify: false,
			ServerName:         source.Host,
		}
	}
}

func generateSSLQueryStringIfNotExists(s *Source) string {

	if s.Uri == "" {
		SSLQueryString := ""
		if s.SSLQueryString == "" {

			if s.SSLMode == "disable" || s.SSLMode == "allow" || s.SSLMode == "prefer" || s.SSLMode == "require" || s.SSLMode == "verify-ca" || s.SSLMode == "verify-full" {
				SSLQueryString = "sslmode=" + s.SSLMode
				if s.SSLMode == "require" || s.SSLMode == "verify-ca" || s.SSLMode == "verify-full" {
					SSLQueryString = fmt.Sprintf("sslmode=%s", s.SSLMode)
					if s.SSLCertPath != "" {
						SSLQueryString += "&sslcert=" + s.SSLCertPath
					}
					if s.SSLKey != "" {
						SSLQueryString += "&sslkey=" + s.SSLKey
					}
					if s.SSLRootCert != "" {
						SSLQueryString += "&sslrootcert=" + s.SSLRootCert
					}
					if s.SSLCRL != "" {
						SSLQueryString += "&sslcrl=" + s.SSLCRL
					}
				}
			} else {
				utils.ErrExit("Invalid sslmode: %q", s.SSLMode)
			}
		} else {
			SSLQueryString = s.SSLQueryString
		}
		return SSLQueryString
	} else {
		return ""
	}
}

//go:embed data/sample-ora2pg.conf
var Ora2pgConfigFile string

type Ora2pgConfig struct {
	OracleDSN        string
	OracleUser       string
	OracleHome       string
	OraclePWD        string
	Schema           string
	ParallelTables   string
	UseOrafce        string
	DisablePartition string
	DisableComment   string
	Allow            string
	ModifyStruct     string
}

func (source *Source) getDefaultOra2pgConfig() *Ora2pgConfig {
	conf := &Ora2pgConfig{}
	conf.OracleDSN = source.getSourceDSN()
	conf.OracleUser = source.User
	conf.ParallelTables = strconv.Itoa(source.NumConnections)
	conf.OraclePWD = source.Password
	conf.DisablePartition = "0"

	conf.OracleHome = source.GetOracleHome()
	if source.Schema != "" {
		conf.Schema = source.Schema
	} else {
		conf.Schema = source.User
	}
	if source.UseOrafce {
		conf.UseOrafce = "1"
	} else {
		conf.UseOrafce = "0"
	}
	if source.CommentsOnObjects {
		conf.DisableComment = "0"
	} else {
		conf.DisableComment = "1"
	}
	return conf
}

func (source *Source) PopulateOra2pgConfigFile(configFilePath string, conf *Ora2pgConfig) {
	baseConfigFilePath := filepath.Join("/", "etc", "yb-voyager", "base-ora2pg.conf")
	if utils.FileOrFolderExists(baseConfigFilePath) {
		BaseOra2pgConfigFile, err := os.ReadFile(baseConfigFilePath)
		if err != nil {
			utils.ErrExit("Error while reading base ora2pg configuration file: %v", err)
		}
		Ora2pgConfigFile = string(BaseOra2pgConfigFile)
	}

	tmpl, err := template.New("ora2pg").Parse(Ora2pgConfigFile)
	if err != nil {
		utils.ErrExit("Error while parsing ora2pg configuration file: %v", err)
	}

	var output bytes.Buffer
	err = tmpl.Execute(&output, conf)
	if err != nil {
		utils.ErrExit("Error while preparing ora2pg configuration file: %v", err)
	}

	err = os.WriteFile(configFilePath, output.Bytes(), 0644)
	if err != nil {
		utils.ErrExit("unable to update config file %q: %v\n", configFilePath, err)
	}
}

func (source *Source) getSourceDSN() string {
	var sourceDSN string

	if source.DBType == "oracle" {
		if source.DBName != "" {
			sourceDSN = fmt.Sprintf("dbi:Oracle:host=%s;service_name=%s;port=%d", source.Host, source.DBName, source.Port)
		} else if source.DBSid != "" {
			sourceDSN = fmt.Sprintf("dbi:Oracle:host=%s;sid=%s;port=%d", source.Host, source.DBSid, source.Port)
		} else {
			sourceDSN = fmt.Sprintf("dbi:Oracle:%s", source.TNSAlias) //this option is ideal for ssl connectivity, provide in documentation if needed
		}
	} else if source.DBType == "mysql" {
		parseSSLString(source)
		sourceDSN = fmt.Sprintf("dbi:mysql:host=%s;database=%s;port=%d", source.Host, source.DBName, source.Port)
		sourceDSN = source.extrapolateDSNfromSSLParams(sourceDSN)
	} else {
		utils.ErrExit("Invalid Source DB Type.")
	}

	log.Infof("Source DSN used for export: %s", sourceDSN)
	return sourceDSN
}

func (source *Source) extrapolateDSNfromSSLParams(DSN string) string {
	switch source.SSLMode {
	case "disable":
		DSN += ";mysql_ssl=0;mysql_ssl_optional=0"
	case "prefer":
		DSN += ";mysql_ssl_optional=1"
	case "require":
		DSN += ";mysql_ssl=1"
	case "verify-ca":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += fmt.Sprintf(";mysql_ssl_ca_file=%s", source.SSLRootCert)
		} else {
			utils.ErrExit("Root authority certificate needed for verify-ca mode.")
		}
	case "verify-full":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += fmt.Sprintf(";mysql_ssl_ca_file=%s;mysql_ssl_verify_server_cert=1", source.SSLRootCert)
		} else {
			utils.ErrExit("Root authority certificate needed for verify-full mode.")
		}
	default:
		utils.ErrExit("Incorrect sslmode provided. Please provide a correct value for sslmode and try again.")
	}

	if source.SSLCertPath != "" {
		DSN += fmt.Sprintf(";mysql_ssl_client_cert=%s", source.SSLCertPath)
	}
	if source.SSLKey != "" {
		DSN += fmt.Sprintf(";mysql_ssl_client_key=%s", source.SSLKey)
	}

	return DSN
}

func (source *Source) PrepareSSLParamsForDebezium(exportDir string) error {
	switch source.DBType {
	case "postgresql":
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
