package srcdb

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Source struct {
	DBType            string
	Host              string
	Port              int
	User              string
	Password          string
	DBName            string
	DBSid             string
	OracleHome        string
	TNSAlias          string
	Schema            string
	SSLMode           string
	SSLCertPath       string
	SSLKey            string
	SSLRootCert       string
	SSLCRL            string
	SSLQueryString    string
	Uri               string
	NumConnections    int
	VerboseMode       bool
	TableList         string
	ExcludeTableList  string
	UseOrafce         bool
	CommentsOnObjects bool

	sourceDB SourceDB
}

func (s *Source) DB() SourceDB {
	if s.sourceDB == nil {
		s.sourceDB = newSourceDB(s)
	}
	return s.sourceDB
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
		pem, err := ioutil.ReadFile(source.SSLRootCert)
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

func (source *Source) PopulateOra2pgConfigFile(configFilePath string) {
	sourceDSN := source.getSourceDSN()
	baseConfigFilePath := filepath.Join("/", "etc", "yb-voyager", "base-ora2pg.conf")
	if utils.FileOrFolderExists(baseConfigFilePath) {
		BaseOra2pgConfigFile, err := ioutil.ReadFile(baseConfigFilePath)
		if err != nil {
			utils.ErrExit("Error while reading base ora2pg configuration file: %v", err)
		}
		Ora2pgConfigFile = string(BaseOra2pgConfigFile)
	}

	lines := strings.Split(Ora2pgConfigFile, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "ORACLE_DSN") {
			lines[i] = "ORACLE_DSN	" + sourceDSN
		} else if strings.HasPrefix(line, "ORACLE_USER") {
			lines[i] = "ORACLE_USER	" + source.User
		} else if strings.HasPrefix(line, "ORACLE_HOME") && source.OracleHome != "" {
			lines[i] = "ORACLE_HOME	" + source.OracleHome
		} else if strings.HasPrefix(line, "ORACLE_PWD") {
			lines[i] = "ORACLE_PWD	" + source.Password
		} else if source.DBType == "oracle" && strings.HasPrefix(line, "SCHEMA") {
			if source.Schema != "" { // in oracle USER and SCHEMA are essentially the same thing
				lines[i] = "SCHEMA	" + source.Schema
			} else if source.User != "" {
				lines[i] = "SCHEMA	" + source.User
			}
		} else if strings.HasPrefix(line, "PARALLEL_TABLES") {
			lines[i] = "PARALLEL_TABLES " + strconv.Itoa(source.NumConnections)
		} else if strings.HasPrefix(line, "PG_VERSION") {
			lines[i] = "PG_VERSION " + strconv.Itoa(11)
		} else if strings.HasPrefix(line, "INDEXES_RENAMING") && source.DBType == "mysql" {
			lines[i] = "INDEXES_RENAMING 1"
		} else if strings.HasPrefix(line, "PREFIX_PARTITION") {
			lines[i] = "PREFIX_PARTITION 0"
		} else if strings.HasPrefix(line, "USE_ORAFCE") {
			if source.UseOrafce && strings.EqualFold(source.DBType, "oracle") {
				lines[i] = "USE_ORAFCE 1"
			}
		} else if strings.HasPrefix(line, "#DATA_TYPE") || strings.HasPrefix(line, "DATA_TYPE") {
			lines[i] = "DATA_TYPE      VARCHAR2:varchar,NVARCHAR2:varchar,DATE:timestamp,LONG:text,LONG RAW:bytea,CLOB:text,NCLOB:text,BLOB:bytea,BFILE:bytea,RAW(16):uuid,RAW(32):uuid,RAW:bytea,UROWID:oid,ROWID:oid,FLOAT:double precision,DEC:decimal,DECIMAL:decimal,DOUBLE PRECISION:double precision,INT:integer,INTEGER:integer,REAL:real,SMALLINT:smallint,BINARY_FLOAT:double precision,BINARY_DOUBLE:double precision,TIMESTAMP:timestamp,XMLTYPE:xml,BINARY_INTEGER:integer,PLS_INTEGER:integer,TIMESTAMP WITH TIME ZONE:timestamp with time zone,TIMESTAMP WITH LOCAL TIME ZONE:timestamp with time zone"
		} else if strings.HasPrefix(line, "DISABLE_COMMENT") && !source.CommentsOnObjects {
			lines[i] = "DISABLE_COMMENT 1"
		} else if strings.HasPrefix(line, "PG_INTEGER_TYPE") {
			lines[i] = "PG_INTEGER_TYPE 1" // Required otherwise MySQL autoincrement sequences don't export
		} else if strings.HasPrefix(line, "DEFAULT_NUMERIC") {
			lines[i] = "DEFAULT_NUMERIC numeric"
		}
	}

	output := strings.Join(lines, "\n")
	err := ioutil.WriteFile(configFilePath, []byte(output), 0644)
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
