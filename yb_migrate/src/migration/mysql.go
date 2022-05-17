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
	_ "embed"
	"fmt"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/srcdb"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func parseSSLString(source *srcdb.Source) {
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

func extrapolateDSNfromSSLParams(source *srcdb.Source, DSN string) string {
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
