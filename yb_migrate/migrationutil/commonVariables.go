package migrationutil

import "fmt"

type Source struct {
	DBType         string
	Host           string
	Port           string
	User           string
	Password       string
	DBName         string
	Schema         string
	SSLMode        string
	SSLCert        string
	NumConnections int
}

type Target struct {
	Host       string
	Port       string
	User       string
	Password   string
	DBName     string
	SSLMode    string
	SSLCert    string
	StartClean bool
}

type Format interface {
	PrintFormat(cnt int)
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

var mysqlSchemaObjectList = []string{"TO-BE-ADDED"}

type MetaInfo struct {
	SourceDBType      string
	ExportToolUsed    string
	NumberOfDataFiles int //dummy variable
}
