/*
Copyright (c) YugaByte, Inc.

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
package migration

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func parseSSLString(source *utils.Source) {
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
					break
				case 1:
					source.SSLCertPath = slicedparam[1]
					break
				case 2:
					source.SSLRootCert = slicedparam[1]
					break
				case 3:
					source.SSLKey = slicedparam[1]
					break

				}
				break
			}
		}
	}
}

func createTLSConf(source *utils.Source) tls.Config {
	rootCertPool := x509.NewCertPool()
	if source.SSLRootCert != "" {
		pem, err := ioutil.ReadFile(source.SSLRootCert)
		if err != nil {
			errMsg := fmt.Sprintf("error in reading SSL Root Certificate: %v\n", err)
			utils.ErrExit(errMsg)
		}

		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			errMsg := "Failed to append PEM.\n"
			utils.ErrExit(errMsg)
		}
	} else {
		errMsg := "Root Certificate Needed for verify-ca and verify-full SSL Modes\n"
		utils.ErrExit(errMsg)
	}
	clientCert := make([]tls.Certificate, 0, 1)

	if source.SSLCertPath != "" && source.SSLKey != "" {
		certs, err := tls.LoadX509KeyPair(source.SSLCertPath, source.SSLKey)
		if err != nil {
			errMsg := fmt.Sprintf("error in reading and parsing SSL KeyPair: %v\n", err)
			utils.ErrExit(errMsg)
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

func extrapolateDSNfromSSLParams(source *utils.Source, DSN string) string {
	switch source.SSLMode {
	case "disable":
		DSN += ";mysql_ssl=0;mysql_ssl_optional=0"
		break
	case "prefer":
		DSN += ";mysql_ssl_optional=1"
		break
	case "require":
		DSN += ";mysql_ssl=1"
		break
	case "verify-ca":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += fmt.Sprintf(";mysql_ssl_ca_file=%s", source.SSLRootCert)
		} else {
			fmt.Println("Root authority certificate needed for verify-ca mode.")
			os.Exit(1)
		}
		break
	case "verify-full":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += fmt.Sprintf(";mysql_ssl_ca_file=%s;mysql_ssl_verify_server_cert=1", source.SSLRootCert)
		} else {
			fmt.Println("Root authority certificate needed for verify-full mode.")
			os.Exit(1)
		}
		break
	default:
		fmt.Println("WARNING: Incorrect sslmode provided. Please provide a correct value for sslmode and try again.")
		os.Exit(1)
	}

	if source.SSLCertPath != "" {
		DSN += fmt.Sprintf(";mysql_ssl_client_cert=%s", source.SSLCertPath)
	}
	if source.SSLKey != "" {
		DSN += fmt.Sprintf(";mysql_ssl_client_key=%s", source.SSLKey)
	}

	return DSN
}
