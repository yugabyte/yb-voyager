package utils

import (
	"fmt"
	"sync"
)

type Source struct {
	DBType         string
	Host           string
	Port           string
	User           string
	Password       string
	DBName         string
	Schema         string
	SSLMode        string
	SSLCertPath    string
	NumConnections int
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

//the list elements order is same as the import objects order

var oracleSchemaObjectList = []string{"TYPE", "SEQUENCE", "TABLE", "PACKAGE", "VIEW",
	/*"GRANT",*/ "TRIGGER", "FUNCTION", "PROCEDURE", /*"TABLESPACE", "PARTITION",*/
	/*"MVIEW", "DBLINK",*/ "SYNONYM" /*, "DIRECTORY"*/}

var postgresSchemaObjectList = []string{"SCHEMA", "TYPE", "DOMAIN", "SEQUENCE",
	"TABLE", "RULE", "FUNCTION", "AGGREGATE", "PROCEDURE", "VIEW", "TRIGGER",
	/*Test/Read: MVIEW, PARTITION, TABLESPACES, GRANT, ROLE, RULE, AGGREGATE */}

var mysqlSchemaObjectList = []string{ /*"TYPE", "SEQUENCE",*/ "TABLE", "VIEW", /*"GRANT*/
	"TRIGGER", "FUNCTION", "PROCEDURE" /*"TABLESPACE", "PARTIITON"*/}

type ExportMetaInfo struct {
	SourceDBType   string
	ExportToolUsed string
}

var log = GetLogger()

var WaitGroup sync.WaitGroup
