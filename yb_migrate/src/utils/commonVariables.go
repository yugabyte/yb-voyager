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
package utils

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

type Target struct {
	Host                   string
	Port                   int
	User                   string
	Password               string
	DBName                 string
	Schema                 string
	SSLMode                string
	SSLCertPath            string
	SSLKey                 string
	SSLRootCert            string
	SSLCRL                 string
	SSLQueryString         string
	Uri                    string
	ImportIndexesAfterData bool
	ContinueOnError        bool
	IgnoreIfExists         bool
	VerboseMode            bool
	TableList              string
	ImportMode             bool
}

type Format interface {
	PrintFormat(cnt int)
}

const (
	TABLE_MIGRATION_NOT_STARTED = iota
	TABLE_MIGRATION_IN_PROGRESS
	TABLE_MIGRATION_DONE
	TABLE_MIGRATION_COMPLETED
)

type TableProgressMetadata struct {
	TableSchema          string
	TableName            string
	FullTableName        string
	InProgressFilePath   string
	FinalFilePath        string
	Status               int //(0: NOT-STARTED, 1: IN-PROGRESS, 2: DONE, 3: COMPLETED)
	CountLiveRows        int64
	CountTotalRows       int64
	FileOffsetToContinue int64 // This might be removed later
	IsPartition          bool
	ParentTable          string
	//timeTakenByLast1000Rows int64; TODO: for ESTIMATED time calculation
}

func (s *Source) PrintFormat(cnt int) {
	fmt.Printf("On type Source\n")
}

func (s *Target) PrintFormat(cnt int) {
	fmt.Printf("On type Target\n")
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

func (t *Target) GetConnectionUri() string {
	if t.Uri == "" {
		t.Uri = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s",
			t.User, t.Password, t.Host, t.Port, t.DBName, generateSSLQueryStringIfNotExists(t))
	}
	//TODO: else do a regex match for the correct Uri pattern of user input

	return t.Uri
}

//this function is only triggered when t.Uri==""
func generateSSLQueryStringIfNotExists(t *Target) string {
	SSLQueryString := ""
	if t.SSLMode == "" {
		t.SSLMode = "prefer"
	}
	if t.SSLQueryString == "" {

		if t.SSLMode == "disable" || t.SSLMode == "allow" || t.SSLMode == "prefer" || t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
			SSLQueryString = "sslmode=" + t.SSLMode
			if t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {

				if t.SSLCertPath != "" {
					SSLQueryString += "&sslcert=" + t.SSLCertPath
				}
				if t.SSLKey != "" {
					SSLQueryString += "&sslkey=" + t.SSLKey
				}
				if t.SSLRootCert != "" {
					SSLQueryString += "&sslrootcert=" + t.SSLRootCert
				}
				if t.SSLCRL != "" {
					SSLQueryString += "&sslcrl=" + t.SSLCRL
				}
			}
		} else {
			fmt.Println("Invalid sslmode entered")
		}
	} else {
		SSLQueryString = t.SSLQueryString
	}
	return SSLQueryString
}

// the list elements order is same as the import objects order
// TODO: Need to make each of the list comprehensive, not missing any database object category
var oracleSchemaObjectList = []string{"TYPE", "SEQUENCE", "TABLE", "INDEX", "PACKAGE", "VIEW",
	/*"GRANT",*/ "TRIGGER", "FUNCTION", "PROCEDURE", "PARTITION", /*"TABLESPACE",*/
	"MVIEW" /*"DBLINK",*/, "SYNONYM" /*, "DIRECTORY"*/}

// In PG, PARTITION are exported along with TABLE
var postgresSchemaObjectList = []string{"SCHEMA", "TYPE", "DOMAIN", "SEQUENCE",
	"TABLE", "INDEX", "RULE", "FUNCTION", "AGGREGATE", "PROCEDURE", "VIEW", "TRIGGER",
	"MVIEW", "EXTENSION" /*TABLESPACES, GRANT, ROLE*/}

// In MYSQL, TYPE and SEQUENCE are not supported
var mysqlSchemaObjectList = []string{"TABLE", "INDEX", "VIEW", /*"GRANT*/
	"TRIGGER", "FUNCTION", "PROCEDURE" /* "TABLESPACE, PARTITION"*/}

type ExportMetaInfo struct {
	SourceDBType   string
	ExportToolUsed string
}

var WaitGroup sync.WaitGroup
var WaitChannel = make(chan int)

//report.json format
type Report struct {
	Summary Summary `json:"summary"`
	Issues  []Issue `json:"issues"`
}

type Summary struct {
	DBName     string     `json:"dbName"`
	SchemaName string     `json:"schemaName"`
	DBVersion  string     `json:"dbVersion"`
	DBObjects  []DBObject `json:"databaseObjects"`
}

type DBObject struct {
	ObjectType   string `json:"objectType"`
	TotalCount   int    `json:"totalCount"`
	InvalidCount int    `json:"invalidCount"`
	ObjectNames  string `json:"objectNames"`
	Details      string `json:"details"`
}

type Issue struct {
	ObjectType   string `json:"objectType"`
	ObjectName   string `json:"objectName"`
	Reason       string `json:"reason"`
	SqlStatement string `json:"sqlStatement"`
	FilePath     string `json:"filePath"`
	Suggestion   string `json:"suggestion"`
	GH           string `json:"GH"`
}
