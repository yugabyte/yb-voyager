/*
Copyright (c) YugabyteDB, Inc.

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
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

const (
	TABLE_MIGRATION_NOT_STARTED = iota
	TABLE_MIGRATION_IN_PROGRESS
	TABLE_MIGRATION_DONE
	TABLE_MIGRATION_COMPLETED
	YB_VOYAGER_NULL_STRING = "__YBV_NULL__"
)

type TableProgressMetadata struct {
	TableName            sqlname.NameTuple
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

var TableMetadataStatusMap = map[int]string{
	0: "NOT-STARTED",
	1: "EXPORTING",
	2: "DONE",
	3: "DONE",
}

// the list elements order is same as the import objects order
// TODO: Need to make each of the list comprehensive, not missing any database object category
var oracleSchemaObjectList = []string{"TYPE", "SEQUENCE", "TABLE", "PARTITION", "INDEX", "PACKAGE", "VIEW",
	/*"GRANT",*/ "TRIGGER", "FUNCTION", "PROCEDURE",
	"MVIEW" /*"DBLINK",*/, "SYNONYM" /*, "DIRECTORY"*/}
var oracleSchemaObjectListForExport = []string{"TYPE", "SEQUENCE", "TABLE", "PACKAGE", "TRIGGER", "FUNCTION", "PROCEDURE", "SYNONYM", "VIEW", "MVIEW"}

// In PG, PARTITION are exported along with TABLE
var postgresSchemaObjectList = []string{"SCHEMA", "COLLATION", "EXTENSION", "TYPE", "DOMAIN", "SEQUENCE",
	"TABLE", "INDEX", "FUNCTION", "AGGREGATE", "PROCEDURE", "VIEW", "TRIGGER",
	"MVIEW", "RULE", "COMMENT" /* GRANT, ROLE*/, "CONVERSION", "FOREIGN TABLE", "POLICY", "OPERATOR"}
var postgresSchemaObjectListForExport = []string{"TYPE", "DOMAIN", "SEQUENCE", "TABLE", "FUNCTION", "PROCEDURE", "AGGREGATE", "VIEW", "MVIEW", "TRIGGER", "COMMENT", "CONVERSION", "FOREIGN TABLE", "ROW SECURITY", "POLICY", "OPERATOR", "OPERATOR FAMILY", "OPERATOR CLASS"}

// In MYSQL, TYPE and SEQUENCE are not supported
var mysqlSchemaObjectList = []string{"TABLE", "PARTITION", "INDEX", "VIEW", /*"GRANT*/
	"TRIGGER", "FUNCTION", "PROCEDURE"}
var mysqlSchemaObjectListForExport = []string{"TABLE", "VIEW", "TRIGGER", "FUNCTION", "PROCEDURE"}

var WaitGroup sync.WaitGroup
var WaitChannel = make(chan int)

// ================== Schema Report ==============================

type SchemaReport struct {
	VoyagerVersion  string               `json:"VoyagerVersion"`
	TargetDBVersion *ybversion.YBVersion `json:"TargetDBVersion"`
	SchemaSummary   SchemaSummary        `json:"Summary"`
	Issues          []AnalyzeSchemaIssue `json:"Issues"`
}

type SchemaSummary struct {
	Description string     `json:"Description"`
	DBName      string     `json:"DbName"`
	SchemaNames []string   `json:"SchemaNames"`
	DBVersion   string     `json:"DbVersion"`
	Notes       []string   `json:"Notes,omitempty"`
	DBObjects   []DBObject `json:"DatabaseObjects"`
}

// TODO: Rename the variables of TotalCount and InvalidCount -> TotalObjects and ObjectsWithIssues
type DBObject struct {
	ObjectType   string `json:"ObjectType"`
	TotalCount   int    `json:"TotalCount"`
	InvalidCount int    `json:"InvalidCount"`
	ObjectNames  string `json:"ObjectNames"`
	Details      string `json:"Details,omitempty"`
}

// TODO: support MinimumVersionsFixedIn in xml
type AnalyzeSchemaIssue struct {
	// TODO: deprecate this and rename to Category
	IssueType              string                          `json:"IssueType"` //category: unsupported_features, unsupported_plpgsql_objects, etc
	ObjectType             string                          `json:"ObjectType"`
	ObjectName             string                          `json:"ObjectName"`
	Reason                 string                          `json:"Reason"`
	Type                   string                          `json:"-" xml:"-"` // identifier for issue type ADVISORY_LOCKS, SYSTEM_COLUMNS, etc
	Name                   string                          `json:"-" xml:"-"` // to use for AssessmentIssue
	Impact                 string                          `json:"-" xml:"-"` // temporary field; since currently we generate assessment issue from analyze issue
	SqlStatement           string                          `json:"SqlStatement,omitempty"`
	FilePath               string                          `json:"FilePath"`
	Suggestion             string                          `json:"Suggestion"`
	GH                     string                          `json:"GH"`
	DocsLink               string                          `json:"DocsLink,omitempty"`
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion `json:"MinimumVersionsFixedIn" xml:"-"` // key: series (2024.1, 2.21, etc)
}

func (i AnalyzeSchemaIssue) IsFixedIn(v *ybversion.YBVersion) (bool, error) {
	if i.MinimumVersionsFixedIn == nil {
		return false, nil
	}
	minVersionFixedInSeries, ok := i.MinimumVersionsFixedIn[v.Series()]
	if !ok {
		return false, nil
	}
	return v.GreaterThanOrEqual(minVersionFixedInSeries), nil
}

type IndexInfo struct {
	// TODO: ADD SchemaName string `json:"SchemaName"`
	IndexName string   `json:"IndexName"`
	IndexType string   `json:"IndexType"`
	TableName string   `json:"TableName"`
	Columns   []string `json:"Columns"`
}

type TableColumnsDataTypes struct {
	SchemaName        string `json:"SchemaName"`
	TableName         string `json:"TableName"`
	ColumnName        string `json:"ColumnName"`
	DataType          string `json:"DataType"`
	IsArrayOfEnumType bool   `json:"IsArrayOfEnumType"`
	IsUDTType         bool   `json:"IsUDTType"`
}

type UnsupportedQueryConstruct struct {
	ConstructTypeName      string
	Query                  string
	DocsLink               string
	MinimumVersionsFixedIn map[string]*ybversion.YBVersion // key: series (2024.1, 2.21, etc)
}

// ================== Segment ==============================
type Segment struct {
	Num      int
	FilePath string
}

const (
	SNAPSHOT_ONLY        = "snapshot-only"
	SNAPSHOT_AND_CHANGES = "snapshot-and-changes"
	CHANGES_ONLY         = "changes-only"
)
