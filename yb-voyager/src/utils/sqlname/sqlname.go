package sqlname

import (
	"fmt"
	"strings"
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

type sqlName struct {
	Quoted    string
	Unquoted  string
	MinQuoted string
}

type SourceName struct {
	sqlName    // ObjectName
	SchemaName sqlName
	Qualified  sqlName
}

type TargetName struct {
	sqlName    // ObjectName
	SchemaName sqlName
	Qualified  sqlName
}

func NewSourceName(schemaName, objectName string) *SourceName {
	if schemaName == "" {
		panic("schema name cannot be empty")
	}
	return &SourceName{
		sqlName: sqlName{
			Quoted:    quote(objectName, SourceDBType),
			Unquoted:  unquote(objectName, SourceDBType),
			MinQuoted: minQuote(objectName, SourceDBType),
		},
		// We do not support quoted schema names yet.
		SchemaName: sqlName{
			Quoted:    `"` + schemaName + `"`,
			Unquoted:  schemaName,
			MinQuoted: schemaName,
		},
		Qualified: sqlName{
			Quoted:    schemaName + "." + quote(objectName, SourceDBType),
			Unquoted:  schemaName + "." + unquote(objectName, SourceDBType),
			MinQuoted: schemaName + "." + minQuote(objectName, SourceDBType),
		},
	}
}

func minQuote(objectName, sourceDBType string) string {
	switch sourceDBType {
	case YUGABYTE, POSTGRESQL:
		if isAllLowercase(objectName) {
			return unquote(objectName, sourceDBType)
		} else {
			return `"` + objectName + `"`
		}
	case MYSQL:
		return objectName
	case ORACLE:
		if isAllUppercase(objectName) {
			return unquote(objectName, sourceDBType)
		} else {
			return `"` + objectName + `"`
		}
	default:
		panic("invalid source db type")
	}
}

func isAllUppercase(s string) bool {
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			return false
		}
	}
	return true
}

func isAllLowercase(s string) bool {
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
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
	return s.Qualified.Quoted
}

func (s *SourceName) ToTargetName() *TargetName {
	if PreserveCase {
		return NewTargetName(s.SchemaName.Quoted, s.Quoted)
	}
	return NewTargetName(s.SchemaName.Unquoted, s.Unquoted)
}

func NewTargetName(schemaName, objectName string) *TargetName {
	if schemaName == "" {
		panic("schema name cannot be empty")
	}
	return &TargetName{
		sqlName: sqlName{
			Quoted:    quote(objectName, YUGABYTE),
			Unquoted:  unquote(objectName, YUGABYTE),
			MinQuoted: minQuote(objectName, YUGABYTE),
		},
		SchemaName: sqlName{
			Quoted:    `"` + schemaName + `"`,
			Unquoted:  schemaName,
			MinQuoted: schemaName,
		},
		Qualified: sqlName{
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

func isQuoted(s string) bool {
	// TODO: Learn the semantics of backticks in MySQL and Oracle.
	return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`')
}

func quote(s string, dbType string) string {
	if isQuoted(s) {
		if s[0] == '`' && dbType == YUGABYTE {
			return `"` + unquote(s, dbType) + `"` // `Foo` -> "Foo"
		}
		return s
	}
	switch dbType {
	case POSTGRESQL, YUGABYTE:
		return `"` + strings.ToLower(s) + `"`
	case MYSQL:
		return "`" + s + "`"
	case ORACLE:
		return `"` + strings.ToUpper(s) + `"`
	default:
		panic("unknown source db type")
	}
}

func unquote(s string, dbType string) string {
	if isQuoted(s) {
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
