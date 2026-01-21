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
package sqlname

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

var (
	SourceDBType string
	PreserveCase bool
)

type Identifier struct {
	Quoted    string
	Unquoted  string
	MinQuoted string
}

func NewIdentifier(dbType, name string) Identifier {
	if IsQuoted(name) {
		name = unquote(name, dbType)
	}
	return Identifier{
		Quoted:    quote2(dbType, name),
		Unquoted:  name,
		MinQuoted: minQuote2(name, dbType),
	}
}

func (i Identifier) Equals(other Identifier) bool {
	return i.Quoted == other.Quoted && i.Unquoted == other.Unquoted && i.MinQuoted == other.MinQuoted
}

type SourceName struct {
	ObjectName Identifier
	SchemaName Identifier
	Qualified  Identifier
}

type TargetName struct {
	ObjectName Identifier
	SchemaName Identifier
	Qualified  Identifier
}

// ASsumption is to pass quoted name with  case sensitivity preserved
func NewSourceName(schemaName, objectName string) *SourceName {
	if schemaName == "" {
		panic("schema name cannot be empty")
	}
	if !IsQuoted(schemaName) || !IsQuoted(objectName) {
		panic(fmt.Sprintf("schema or object name should be quoted: %s or %s", schemaName, objectName))
	}
	return &SourceName{
		ObjectName: Identifier{
			Quoted:    quote(objectName, SourceDBType),
			Unquoted:  unquote(objectName, SourceDBType),
			MinQuoted: minQuote(objectName, SourceDBType),
		},
		// We do not support quoted schema names yet.
		SchemaName: Identifier{
			Quoted:    quote(schemaName, SourceDBType),
			Unquoted:  unquote(schemaName, SourceDBType),
			MinQuoted: minQuote(schemaName, SourceDBType),
		},
		Qualified: Identifier{
			Quoted:    quote(schemaName, SourceDBType) + "." + quote(objectName, SourceDBType),
			Unquoted:  unquote(schemaName, SourceDBType) + "." + unquote(objectName, SourceDBType),
			MinQuoted: minQuote(schemaName, SourceDBType) + "." + minQuote(objectName, SourceDBType),
		},
	}
}

// ASsumption is to pass quoted name with  case sensitivity preserved
func NewSourceNameFromQualifiedName(qualifiedName string) *SourceName {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid qualified name: %s", qualifiedName))
	}
	return NewSourceName(parts[0], parts[1])
}

// ASsumption is to pass quoted name with  case sensitivity preserved
func NewSourceNameFromMaybeQualifiedName(qualifiedName string, defaultSchemaName string) *SourceName {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) == 2 {
		return NewSourceName(parts[0], parts[1])
	} else if len(parts) == 1 {
		return NewSourceName(defaultSchemaName, parts[0])
	} else {
		panic(fmt.Sprintf("invalid qualified name: %s", qualifiedName))
	}
}

func (s *SourceName) String() string {
	return s.Qualified.MinQuoted
}

func (s *SourceName) ToTargetName() *TargetName {
	if PreserveCase {
		return NewTargetName(s.SchemaName.Quoted, s.ObjectName.Quoted)
	}
	return NewTargetName(s.SchemaName.Unquoted, s.ObjectName.Unquoted)
}

// ASsumption is to pass quoted name with  case sensitivity preserved
func NewTargetName(schemaName, objectName string) *TargetName {
	if schemaName == "" {
		panic("schema name cannot be empty")
	}
	if !IsQuoted(schemaName) || !IsQuoted(objectName) {
		panic(fmt.Sprintf("schema or object name should be quoted: %s or %s", schemaName, objectName))
	}
	return &TargetName{
		ObjectName: Identifier{
			Quoted:    quote(objectName, constants.YUGABYTEDB),
			Unquoted:  unquote(objectName, constants.YUGABYTEDB),
			MinQuoted: minQuote(objectName, constants.YUGABYTEDB),
		},
		SchemaName: Identifier{
			Quoted:    quote(schemaName, constants.YUGABYTEDB),
			Unquoted:  unquote(schemaName, constants.YUGABYTEDB),
			MinQuoted: minQuote(schemaName, constants.YUGABYTEDB),
		},
		Qualified: Identifier{
			Quoted:    quote(schemaName, constants.YUGABYTEDB) + "." + quote(objectName, constants.YUGABYTEDB),
			Unquoted:  unquote(schemaName, constants.YUGABYTEDB) + "." + unquote(objectName, constants.YUGABYTEDB),
			MinQuoted: minQuote(schemaName, constants.YUGABYTEDB) + "." + minQuote(objectName, constants.YUGABYTEDB),
		},
	}
}

//ASsumption is to pass quoted name with  case sensitivity preserved
func NewTargetNameFromQualifiedName(qualifiedName string) *TargetName {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid qualified name: %s", qualifiedName))
	}
	return NewTargetName(parts[0], parts[1])
}

//ASsumption is to pass quoted name with  case sensitivity preserved
func NewTargetNameFromMaybeQualifiedName(qualifiedName string, defaultSchemaName string) *TargetName {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) == 2 {
		return NewTargetName(parts[0], parts[1])
	} else if len(parts) == 1 {
		return NewTargetName(defaultSchemaName, parts[0])
	} else {
		panic(fmt.Sprintf("invalid qualified name: %s", qualifiedName))
	}
}

func (t *TargetName) String() string {
	return t.Qualified.Quoted
}

func IsQuoted(s string) bool {
	if len(s) == 0 {
		return false
	}
	// TODO: Learn the semantics of backticks in MySQL and Oracle.
	return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`')
}

func quote(s string, dbType string) string {
	if IsQuoted(s) {
		if s[0] == '`' && dbType == constants.YUGABYTEDB {
			return `"` + unquote(s, dbType) + `"` // `Foo` -> "Foo"
		}
		return s
	}
	switch dbType {
	case constants.POSTGRESQL, constants.YUGABYTEDB:
		return `"` + strings.ToLower(s) + `"`
	case constants.MYSQL:
		return s // TODO - learn the semantics of quoting in MySQL.
	case constants.ORACLE:
		return `"` + strings.ToUpper(s) + `"`
	default:
		panic("unknown source db type " + dbType)
	}
}

func unquote(s string, dbType string) string {
	if IsQuoted(s) {
		return s[1 : len(s)-1]
	}
	switch dbType {
	case constants.POSTGRESQL, constants.YUGABYTEDB:
		return strings.ToLower(s)
	case constants.MYSQL:
		return s
	case constants.ORACLE:
		return strings.ToUpper(s)
	default:
		panic("unknown source db type")
	}
}

func SetDifference(a, b []*SourceName) []*SourceName {
	m := make(map[string]bool)
	for _, x := range b {
		m[x.String()] = true
	}
	var res []*SourceName
	for _, x := range a {
		if !m[x.String()] {
			res = append(res, x)
		}
	}
	return res
}

func minQuote(objectName, sourceDBType string) string {
	objectName = unquote(objectName, sourceDBType)
	switch sourceDBType {
	case constants.YUGABYTEDB, constants.POSTGRESQL:
		if IsAllLowercase(objectName) && !IsReservedKeywordPG(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	case constants.MYSQL:
		return objectName
	case constants.ORACLE:
		if IsAllUppercase(objectName) && !IsReservedKeywordOracle(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	default:
		panic("invalid source db type")
	}
}

func IsAllUppercase(s string) bool {
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			return false
		}
	}
	return true
}

func IsAllLowercase(s string) bool {
	for _, c := range s {
		if c == '_' {
			continue
		}
		if !(unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c))) { // check for special chars
			return false
		}
		if unicode.IsUpper(rune(c)) {
			return false
		}
	}
	return true
}

func IsCaseSensitive(s string, sourceDbType string) bool {
	switch sourceDbType {
	case constants.ORACLE:
		return !IsAllUppercase(s)
	case constants.POSTGRESQL:
		return !IsAllLowercase(s)
	case constants.MYSQL:
		return false
	}
	panic("invalid source db type")
}

var PgReservedKeywords = []string{"all", "analyse", "analyze", "and", "any", "array",
	"as", "asc", "asymmetric", "both", "case", "cast", "check", "collate", "column",
	"constraint", "create", "current_catalog", "current_date", "current_role",
	"current_time", "current_timestamp", "current_user", "default", "deferrable",
	"desc", "distinct", "do", "else", "end", "except", "false", "fetch", "for", "foreign",
	"from", "grant", "group", "having", "in", "initially", "intersect", "into", "lateral",
	"leading", "limit", "localtime", "localtimestamp", "not", "null", "offset", "on",
	"only", "or", "order", "placing", "primary", "references", "returning", "select",
	"session_user", "some", "symmetric", "table", "then", "to", "trailing", "true",
	"union", "unique", "user", "using", "variadic", "when", "where", "window", "with"}

var OracleReservedKeywords = []string{
	"ACCESS", "ADD", "ADMIN", "AFTER", "ALL", "ALTER", "ANALYZE", "AND", "ANY", "ARCHIVE",
	"ARCHIVELOG", "ARRAYLEN", "AS", "ASC", "AUTHORIZATION", "AVG", "BACKUP", "BECOME",
	"BEFORE", "BEGIN", "BETWEEN", "BLOCK", "BODY", "BY", "CACHE", "CANCEL", "CASCADE",
	"CHANGE", "CHAR", "CHECK", "CHECKPOINT", "CLOSE", "CLUSTER", "COBOL", "COLUMN",
	"COMMENT", "COMMIT", "COMPILE", "COMPRESS", "CONNECT", "CONSTRAINT", "CONSTRAINTS",
	"CONTENTS", "CONTINUE", "CONTROLFILE", "COUNT", "CREATE", "CURRENT", "CYCLE", "DATAFILE",
	"DATABASE", "DATE", "DECIMAL", "DECLARE", "DEFAULT", "DEFERRABLE", "DELETE", "DESC",
	"DISABLE", "DISTINCT", "DISTRIBUTE", "DROP", "DUMP", "EACH", "ELSE", "ENABLE",
	"END", "ESCAPE", "EXCEPT", "EXCEPTIONS", "EXCLUSIVE", "EXEC", "EXECUTE", "EXISTS",
	"EXIT", "EXTERNALLY", "EXTRACT", "FAILED", "FETCH", "FLOAT", "FOR", "FORCE",
	"FOREIGN", "FOUND", "FROM", "FUNCTION", "GOTO", "GRANT", "GROUP", "GROUPS", "HAVING",
	"IDENTIFIED", "IMMEDIATE", "INCLUDING", "INCREMENT", "INDEX", "INDICATOR", "INITIAL",
	"INITRANS", "INSERT", "INSTANCE", "INT", "INTEGER", "INTERSECT", "INTO", "IS", "KEY",
	"LAYER", "LEVEL", "LIKE", "LINK", "LISTS", "LOCK", "LOGFILE", "LONG", "MANAGE", "MANUAL",
	"MAX", "MAXDATAFILES", "MAXEXTENTS", "MAXINSTANCES", "MAXLOGFILES", "MAXLOGHISTORY",
	"MAXLOGMEMBERS", "MAXTRANS", "MAXVALUE", "MIN", "MINEXTENTS", "MINUS", "MINVALUE",
	"MODE", "MODIFY", "MODULE", "MOUNT", "NEXT", "NOARCHIVELOG", "NOCACHE", "NOCOMPRESS",
	"NOCYCLE", "NOORDER", "NORESETLOGS", "NORMAL", "NOT", "NOTFOUND", "NOWAIT", "NULL",
	"NUMBER", "NUMERIC", "OFF", "OFFLINE", "ON", "ONLINE", "ONLY", "OPEN", "OPTION",
	"OR", "ORDER", "OWN", "PACKAGE", "PARALLEL", "PCTFREE", "PCTINCREASE", "PCTUSED",
	"PLAN", "PLI", "POOL", "PRIOR", "PRIVILEGES", "PROCEDURE", "PUBLIC", "RAW", "RBA",
	"READ", "REAL", "RECOVER", "REFERENCES", "REFERENCING", "RELEASE", "RENAME",
	"RESOURCE", "RESTRICTED", "RETURN", "REUSE", "REVOKE", "ROLE", "ROLES", "ROLLBACK",
	"ROW", "ROWID", "ROWLABEL", "ROWNUM", "ROWS", "SCHEMA", "SCN", "SECTION", "SEGMENT",
	"SELECT", "SEQUENCE", "SESSION", "SET", "SHARE", "SHARED", "SIZE", "SMALLINT", "SNAPSHOT",
	"SOME", "SORT", "SQL", "SQLBUF", "SQLCODE", "SQLERROR", "SQLSTATE", "START", "STATEMENT_ID",
	"STATISTICS", "STOP", "STORAGE", "SUCCESSFUL", "SWITCH", "SYNONYM", "SYSDATE", "SYSTEM",
	"TABLE", "TABLES", "TABLESPACE", "TEMPORARY", "THEN", "THREAD", "TIME", "TO", "TRACING",
	"TRIGGER", "TRIGGERS", "TRUNCATE", "UID", "UNION", "UNIQUE", "UNTIL", "UPDATE", "USE", "USER",
	"VALIDATE", "VALUES", "VARCHAR", "VARCHAR2", "VIEW", "WHEN", "WHENEVER", "WHERE", "WHILE",
	"WITH", "WORK", "WRITE",
}

func IsReservedKeywordPG(word string) bool {
	return slices.Contains(PgReservedKeywords, word)
}

func IsReservedKeywordOracle(word string) bool {
	return slices.Contains(OracleReservedKeywords, word)
}

type ObjectNameQualifiedWithTableName struct {
	SchemaName        Identifier
	TableName         *ObjectName
	ObjectName        string
	FromDefaultSchema bool

	Qualified    Identifier
	Unqualified  Identifier
	MinQualified Identifier
}

/*
no assumption if the schema,object, and table name is quoted or unquoted or minquoted, just that casing is preserved, all handled and properly ObjectNameQualifiedWithTableName is returned
*/
func NewObjectNameQualifiedWithTableName(dbType, defaultSchemaName, objectName string, schemaName, tableName string) *ObjectNameQualifiedWithTableName {
	if IsQuoted(schemaName) {
		schemaName = schemaName[1 : len(schemaName)-1]
	}
	if IsQuoted(tableName) {
		tableName = tableName[1 : len(tableName)-1]
	}
	if IsQuoted(objectName) {
		objectName = objectName[1 : len(objectName)-1]
	}
	result := &ObjectNameQualifiedWithTableName{
		SchemaName: Identifier{
			Quoted:    quote2(dbType, schemaName),
			Unquoted:  schemaName,
			MinQuoted: minQuote2(schemaName, dbType),
		},
		FromDefaultSchema: schemaName == defaultSchemaName,
		TableName:         NewObjectName(dbType, defaultSchemaName, schemaName, tableName),
		ObjectName:        objectName,
		Qualified: Identifier{
			Quoted:    quote2(dbType, schemaName) + "." + quote2(dbType, tableName) + "." + quote2(dbType, objectName),
			Unquoted:  schemaName + "." + tableName + "." + objectName,
			MinQuoted: minQuote2(schemaName, dbType) + "." + minQuote2(tableName, dbType) + "." + minQuote2(objectName, dbType),
		},
		Unqualified: Identifier{
			Quoted:    quote2(dbType, schemaName) + "." + quote2(dbType, tableName) + "." + quote2(dbType, objectName),
			Unquoted:  schemaName + "." + tableName + "." + objectName,
			MinQuoted: minQuote2(schemaName, dbType) + "." + minQuote2(tableName, dbType) + "." + minQuote2(objectName, dbType),
		},
	}
	result.MinQualified = lo.Ternary(result.FromDefaultSchema, result.Unqualified, result.Qualified)
	return result
}

func (o *ObjectNameQualifiedWithTableName) CatalogName() string {
	return o.Qualified.Unquoted
}

func (o *ObjectNameQualifiedWithTableName) GetObjectONTableFormatString() string {
	return fmt.Sprintf("%s ON %s", o.ObjectName, o.TableName.Qualified.MinQuoted)
}

func (o *ObjectNameQualifiedWithTableName) Key() string {
	return o.CatalogName()
}

func (o *ObjectNameQualifiedWithTableName) GetQualifiedTableName() string {
	return o.TableName.Qualified.MinQuoted
}

func (o *ObjectNameQualifiedWithTableName) GetObjectName() string {
	return o.ObjectName
}
