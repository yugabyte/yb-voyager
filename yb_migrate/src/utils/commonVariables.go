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
}

type Target struct {
	Host                   string
	Port                   string
	User                   string
	Password               string
	DBName                 string
	SSLMode                string
	SSLCertPath            string
	SSLKey                 string
	SSLRootCert            string
	SSLCRL                 string
	SSLQueryString         string
	Uri                    string
	ImportIndexesAfterData bool
	ContinueOnError        bool
	YsqlshPath             string
	IgnoreIfExists         bool
	VerboseMode            bool
}

type Format interface {
	PrintFormat(cnt int)
}

type TableProgressMetadata struct {
	TableSchema          string
	TableName            string
	InProgressFilePath   string
	FinalFilePath        string
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
		oracleUriRegexpTNS := regexp.MustCompile(`oracle://([a-zA-Z0-9_.-]+)/([^@ ]+)@([a-zA-Z0-9_.-]+)`)
		oracleUriRegexpSID := regexp.MustCompile(`oracle://([a-zA-Z0-9_.-]+)/([^@ ]+)@//([a-zA-Z0-9_.-]+):([0-9]+):([a-zA-Z0-9_.-]+)`)
		oracleUriRegexpServiceName := regexp.MustCompile(`oracle://([a-zA-Z0-9_.-]+)/([^@ ]+)@//([a-zA-Z0-9_.-]+):([0-9]+)/([a-zA-Z0-9_.-]+)`)
		if uriParts := oracleUriRegexpTNS.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.TNSAlias = uriParts[3]
			s.Schema = s.User
		} else if uriParts := oracleUriRegexpSID.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[1]
			s.Password = uriParts[2]
			s.Host = uriParts[3]
			s.Port = uriParts[4]
			s.DBSid = uriParts[5]
			s.Schema = s.User
		} else if uriParts := oracleUriRegexpServiceName.FindStringSubmatch(s.Uri); uriParts != nil {
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
		fmt.Printf(s.Uri)
		postgresUriRegexp := regexp.MustCompile(`(postgresql|postgres)://([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)@([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)\?([a-zA-Z0-9=&/_.-]+)`)
		if uriParts := postgresUriRegexp.FindStringSubmatch(s.Uri); uriParts != nil {
			s.User = uriParts[2]
			s.Password = uriParts[3]
			s.Host = uriParts[4]
			s.Port = uriParts[5]
			s.DBName = uriParts[6]
			s.SSLQueryString = uriParts[7]
		} else {
			fmt.Printf("invalid connection uri for source db uri\n")
			os.Exit(1)
		}
	}
}

func (t *Target) ParseURI() {
	yugabyteDBUriRegexp := regexp.MustCompile(`(postgresql|postgres)://([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)@([a-zA-Z0-9_.-]+):([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)\?([a-zA-Z0-9&/_.-]+)`)
	if uriParts := yugabyteDBUriRegexp.FindStringSubmatch(t.Uri); uriParts != nil {
		t.User = uriParts[2]
		t.Password = uriParts[3]
		t.Host = uriParts[4]
		t.Port = uriParts[5]
		t.DBName = uriParts[6]
		t.SSLQueryString = uriParts[7]
	} else {
		fmt.Printf("invalid connection uri for source db uri\n")
		os.Exit(1)
	}
}

func (t *Target) GetConnectionUri() string {
	if t.Uri == "" {
		t.Uri = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?%s",
			t.User, t.Password, t.Host, t.Port, t.DBName, generateSSLQueryStringIfNotExists(t))
	}
	//TODO: else do a regex match for the correct Uri pattern of user input

	return t.Uri
}

func generateSSLQueryStringIfNotExists(t *Target) string {
	SSLQueryString := ""
	if t.SSLQueryString == "" {

		if t.SSLMode == "disable" || t.SSLMode == "allow" || t.SSLMode == "prefer" || t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
			SSLQueryString = "sslmode=" + t.SSLMode
			if t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
				SSLQueryString = fmt.Sprintf("sslmode=%s", t.SSLMode)
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

//the list elements order is same as the import objects order
//TODO: Need to make each of the list comprehensive, not missing any database object category
var oracleSchemaObjectList = []string{"TYPE", "SEQUENCE", "TABLE", "INDEX", "PACKAGE", "VIEW",
	/*"GRANT",*/ "TRIGGER", "FUNCTION", "PROCEDURE", /*"TABLESPACE", "PARTITION",*/
	"MVIEW" /*"DBLINK",*/, "SYNONYM" /*, "DIRECTORY"*/}

var postgresSchemaObjectList = []string{"SCHEMA", "TYPE", "DOMAIN", "SEQUENCE",
	"TABLE", "INDEX", "RULE", "FUNCTION", "AGGREGATE", "PROCEDURE", "VIEW", "TRIGGER",
	"MVIEW", "EXTENSION" /*Test/Read: , PARTITION, TABLESPACES, GRANT, ROLE, */}

var mysqlSchemaObjectList = []string{ /*"TYPE", "SEQUENCE",*/ "TABLE", "INDEX", "VIEW", /*"GRANT*/
	"TRIGGER", "FUNCTION", "PROCEDURE" /*"TABLESPACE", "PARTIITON"*/}

type ExportMetaInfo struct {
	SourceDBType   string
	ExportToolUsed string
}

var log = GetLogger()

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
