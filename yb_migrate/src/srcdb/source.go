package srcdb

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Source struct {
	DBType             string
	Host               string
	Port               int
	User               string
	Password           string
	DBName             string
	DBSid              string
	OracleHome         string
	TNSAlias           string
	Schema             string
	SSLMode            string
	SSLCertPath        string
	SSLKey             string
	SSLRootCert        string
	SSLCRL             string
	SSLQueryString     string
	Uri                string
	NumConnections     int
	GenerateReportMode bool
	VerboseMode        bool
	TableList          string
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
