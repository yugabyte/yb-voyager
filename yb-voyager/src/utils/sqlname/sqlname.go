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

	"golang.org/x/exp/slices"
)

const (
	YUGABYTE   = "yugabyte"
	POSTGRESQL = "postgresql"
	ORACLE     = "oracle"
	MYSQL      = "mysql"
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

func NewSourceName(schemaName, objectName string) *SourceName {
	if schemaName == "" {
		panic("schema name cannot be empty")
	}
	return &SourceName{
		ObjectName: Identifier{
			Quoted:    quote(objectName, SourceDBType),
			Unquoted:  unquote(objectName, SourceDBType),
			MinQuoted: minQuote(objectName, SourceDBType),
		},
		// We do not support quoted schema names yet.
		SchemaName: Identifier{
			Quoted:    `"` + schemaName + `"`,
			Unquoted:  schemaName,
			MinQuoted: schemaName,
		},
		Qualified: Identifier{
			Quoted:    schemaName + "." + quote(objectName, SourceDBType),
			Unquoted:  schemaName + "." + unquote(objectName, SourceDBType),
			MinQuoted: schemaName + "." + minQuote(objectName, SourceDBType),
		},
	}
}

func NewSourceNameFromQualifiedName(qualifiedName string) *SourceName {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid qualified name: %s", qualifiedName))
	}
	return NewSourceName(parts[0], parts[1])
}

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

func NewTargetName(schemaName, objectName string) *TargetName {
	if schemaName == "" {
		panic("schema name cannot be empty")
	}
	return &TargetName{
		ObjectName: Identifier{
			Quoted:    quote(objectName, YUGABYTE),
			Unquoted:  unquote(objectName, YUGABYTE),
			MinQuoted: minQuote(objectName, YUGABYTE),
		},
		SchemaName: Identifier{
			Quoted:    `"` + schemaName + `"`,
			Unquoted:  schemaName,
			MinQuoted: schemaName,
		},
		Qualified: Identifier{
			Quoted:    schemaName + "." + quote(objectName, YUGABYTE),
			Unquoted:  schemaName + "." + unquote(objectName, YUGABYTE),
			MinQuoted: schemaName + "." + minQuote(objectName, YUGABYTE),
		},
	}
}

func NewTargetNameFromQualifiedName(qualifiedName string) *TargetName {
	parts := strings.Split(qualifiedName, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid qualified name: %s", qualifiedName))
	}
	return NewTargetName(parts[0], parts[1])
}

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
	// TODO: Learn the semantics of backticks in MySQL and Oracle.
	return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`')
}

func quote(s string, dbType string) string {
	if IsQuoted(s) {
		if s[0] == '`' && dbType == YUGABYTE {
			return `"` + unquote(s, dbType) + `"` // `Foo` -> "Foo"
		}
		return s
	}
	switch dbType {
	case POSTGRESQL, YUGABYTE:
		return `"` + strings.ToLower(s) + `"`
	case MYSQL:
		return s // TODO - learn the semantics of quoting in MySQL.
	case ORACLE:
		return `"` + strings.ToUpper(s) + `"`
	default:
		panic("unknown source db type")
	}
}

func unquote(s string, dbType string) string {
	if IsQuoted(s) {
		return s[1 : len(s)-1]
	}
	switch dbType {
	case POSTGRESQL, YUGABYTE:
		return strings.ToLower(s)
	case MYSQL:
		return s
	case ORACLE:
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
	case YUGABYTE, POSTGRESQL:
		if IsAllLowercase(objectName) && !IsReservedKeywordPG(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	case MYSQL:
		return objectName
	case ORACLE:
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
		if c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
}

func IsCaseSensitive(s string, sourceDbType string) bool {
	switch sourceDbType {
	case ORACLE:
		return !IsAllUppercase(s)
	case POSTGRESQL:
		return !IsAllLowercase(s)
	case MYSQL:
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
