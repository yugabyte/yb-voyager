package utils

import (
	"fmt"
	"os"
	"regexp"
	"sync"
)

type Source struct {
	DBType             string
	Host               string
	Port               string
	User               string
	Password           string
	DBName             string
	Schema             string
	SSLMode            string
	SSLCertPath        string
	Uri                string
	NumConnections     int
	GenerateReportMode bool
	VerboseMode        bool
}

type Target struct {
	Host        string
	Port        string
	User        string
	Password    string
	DBName      string
	SSLMode     string
	SSLCertPath string
	Uri         string
}

type Format interface {
	PrintFormat(cnt int)
}

type TableProgressMetadata struct {
	TableSchema          string
	TableName            string
	DataFilePath         string
	Status               int //(0: NOT-STARTED, 1: IN-PROGRESS, 2: DONE, 3: COMPLETED)
	CountLiveRows        int64
	CountTotalRows       int64
	FileOffsetToContinue int64 // This might be removed later
	//timeTakenByLast1000Rows int64; TODO: for ESTIMATED time calculation
}

func (s *Source) PrintFormat(cnt int) {
	fmt.Printf("On type Source\n")
}

func (s *Target) PrintFormat(cnt int) {
	fmt.Printf("On type Target\n")
}

func (s *Source) ParseURI() {
	// fmt.Printf("Parsing Uri for source=%s\n", s.DBType)
	switch s.DBType {
	case "oracle":
		//TODO: figure out correct uri format for oracle and implement
		oracleUriRegexp := regexp.MustCompile(`oracle://User=([a-zA-Z0-9_.-]+);Password=([a-zA-Z0-9_.-]+)@([a-zA-Z0-9_.-]+):([0-9]+)/([a-zA-Z0-9_.-]+)`)
		if uriParts := oracleUriRegexp.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.Host = uriParts[3]
			s.Port = uriParts[4]
			s.DBName = uriParts[5]
			s.Schema = s.User
		} else {
			fmt.Printf("invalid connection uri for source db uri\n")
			os.Exit(1)
		}
	case "mysql":
		mysqlUriRegexp := regexp.MustCompile(`mysql://([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)@([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)`)
		if uriParts := mysqlUriRegexp.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.Host = uriParts[3]
			s.Port = uriParts[4]
			s.DBName = uriParts[5]
		} else {
			fmt.Printf("invalid connection uri for source db uri\n")
			os.Exit(1)
		}
	case "postgresql":
		postgresUriRegexp := regexp.MustCompile(`(postgresql|postgres)://([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)@([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)`)
		if uriParts := postgresUriRegexp.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[2]
			s.Password = uriParts[3]
			s.Host = uriParts[4]
			s.Port = uriParts[5]
			s.DBName = uriParts[6]
		} else {
			fmt.Printf("invalid connection uri for source db uri\n")
			os.Exit(1)
		}
	}
}

//the list elements order is same as the import objects order
//TODO: Need to make each of the list comprehensive, not missing any database object category
var oracleSchemaObjectList = []string{"TYPE", "SEQUENCE", "TABLE", "PACKAGE", "VIEW",
	/*"GRANT",*/ "TRIGGER", "FUNCTION", "PROCEDURE", /*"TABLESPACE", "PARTITION",*/
	"MVIEW" /*"DBLINK",*/, "SYNONYM" /*, "DIRECTORY"*/}

var postgresSchemaObjectList = []string{"SCHEMA", "TYPE", "DOMAIN", "SEQUENCE",
	"TABLE", "RULE", "FUNCTION", "AGGREGATE", "PROCEDURE", "VIEW", "TRIGGER",
	"MVIEW", "EXTENSION" /*Test/Read: , PARTITION, TABLESPACES, GRANT, ROLE, */}

var mysqlSchemaObjectList = []string{ /*"TYPE", "SEQUENCE",*/ "TABLE", "VIEW", /*"GRANT*/
	"TRIGGER", "FUNCTION", "PROCEDURE" /*"TABLESPACE", "PARTIITON"*/}

type ExportMetaInfo struct {
	SourceDBType   string
	ExportToolUsed string
}

var log = GetLogger()

var WaitGroup sync.WaitGroup
