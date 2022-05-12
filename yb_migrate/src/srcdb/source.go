package srcdb

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

const ORA2PG_EXPORT_DATA_LIMIT = 10000 // default value for DATA_LIMIT parameter in ora2pg.conf file

type Source struct {
	DBType         string
	Host           string
	Port           int
	User           string
	Password       string
	DBName         string
	DBSid          string
	OracleHome     string
	TNSAlias       string
	Schema         string
	SSLMode        string
	SSLCertPath    string
	SSLKey         string
	SSLRootCert    string
	SSLCRL         string
	SSLQueryString string
	Uri            string
	NumConnections int
	VerboseMode    bool
	TableList      string

	sourceDB SourceDB
}

func (s *Source) DB() SourceDB {
	if s.sourceDB == nil {
		s.sourceDB = newSourceDB(s)
	}
	return s.sourceDB
}

func (s *Source) ParseURI() {
	var err error
	switch s.DBType {
	case "oracle":
		//TODO: figure out correct uri format for oracle and implement
		oracleUriRegexpTNS := regexp.MustCompile(`([a-zA-Z0-9_.-]+)/([^@ ]+)@([a-zA-Z0-9_.-]+)`)
		oracleUriRegexpSID := regexp.MustCompile(`([a-zA-Z0-9_.-]+)/([^@ ]+)@//([a-zA-Z0-9_.-]+):([0-9]+):([a-zA-Z0-9_.-]+)`)
		oracleUriRegexpServiceName := regexp.MustCompile(`([a-zA-Z0-9_.-]+)/([^@ ]+)@//([a-zA-Z0-9_.-]+):([0-9]+)/([a-zA-Z0-9_.-]+)`)
		if uriParts := oracleUriRegexpTNS.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.TNSAlias = uriParts[3]
			s.Schema = s.User
		} else if uriParts := oracleUriRegexpSID.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.Host = uriParts[3]
			s.Port, err = strconv.Atoi(uriParts[4])
			if err != nil {
				panic(err)
			}
			s.DBSid = uriParts[5]
			s.Schema = s.User
		} else if uriParts := oracleUriRegexpServiceName.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.Host = uriParts[3]
			s.Port, err = strconv.Atoi(uriParts[4])
			if err != nil {
				panic(err)
			}
			s.DBName = uriParts[5]
			s.Schema = s.User
		} else {
			fmt.Printf("invalid connection uri for source db uri\n")
			os.Exit(1)
		}
	case "mysql":
		uriParts, err := url.Parse(s.Uri)
		if err != nil {
			panic(err)
		}

		s.User = uriParts.User.Username()
		if s.User == "" {
			fmt.Println("Username is mandatory!")
			os.Exit(1)
		}
		s.Password, _ = uriParts.User.Password()
		if s.Password == "" {
			fmt.Println("Password is mandatory!")
			os.Exit(1)
		}
		host_port := strings.Split(uriParts.Host, ":")
		if len(host_port) > 1 {
			s.Host = host_port[0]
			s.Port, err = strconv.Atoi(host_port[1])
			if err != nil {
				panic(err)
			}
		} else {
			s.Host = host_port[0]
			s.Port = 3306
		}
		if uriParts.Path != "" {
			s.DBName = uriParts.Path[1:]
		}
		s.SSLQueryString = uriParts.RawQuery

	}
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
