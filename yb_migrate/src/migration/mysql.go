package migration

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"yb_migrate/src/utils"
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
			DSN += ";mysql_ssl_ca_file=" + source.SSLRootCert
		} else {
			fmt.Println("Root authority certificate needed for verify-ca mode.")
			os.Exit(1)
		}
		break
	case "verify-full":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += ";mysql_ssl_ca_file=" + source.SSLRootCert + ";mysql_ssl_verify_server_cert=1"
		} else {
			fmt.Println("Root authority certificate needed for verify-full mode.")
			os.Exit(1)
		}
		break
	default:
		fmt.Println("WARNING: Incorrect sslmode provided. Export will complete without SSL Encryption")
		return DSN
	}

	if source.SSLCertPath != "" {
		DSN += ";mysql_ssl_client_cert=" + source.SSLCertPath
	}
	if source.SSLKey != "" {
		DSN += ";mysql_ssl_client_key=" + source.SSLKey
	}

	return DSN
}
