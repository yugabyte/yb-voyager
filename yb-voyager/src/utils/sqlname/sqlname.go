package sqlname

import (
	"fmt"
	"strings"
)

const (
	YUGABYTE = "yugabyte"
)

var (
	SourceDBType string
	PreserveCase bool
)

type sqlName struct {
	Quoted   string
	Unquoted string
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
			Quoted:   quote(objectName, SourceDBType),
			Unquoted: unquote(objectName, SourceDBType),
		},
		SchemaName: sqlName{
			Quoted:   quote(schemaName, SourceDBType),
			Unquoted: unquote(schemaName, SourceDBType),
		},
		Qualified: sqlName{
			Quoted:   quote(schemaName, SourceDBType) + "." + quote(objectName, SourceDBType),
			Unquoted: unquote(schemaName, SourceDBType) + "." + unquote(objectName, SourceDBType),
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
			Quoted:   quote(objectName, YUGABYTE),
			Unquoted: unquote(objectName, YUGABYTE),
		},
		SchemaName: sqlName{
			Quoted:   quote(schemaName, YUGABYTE),
			Unquoted: unquote(schemaName, YUGABYTE),
		},
		Qualified: sqlName{
			Quoted:   quote(schemaName, YUGABYTE) + "." + quote(objectName, YUGABYTE),
			Unquoted: unquote(schemaName, YUGABYTE) + "." + unquote(objectName, YUGABYTE),
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
	case "postgres", YUGABYTE:
		return `"` + strings.ToLower(s) + `"`
	case "mysql":
		return "`" + s + "`"
	case "oracle":
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
	case "postgres", YUGABYTE:
		return strings.ToLower(s)
	case "mysql":
		return s
	case "oracle":
		return strings.ToUpper(s)
	default:
		panic("unknown source db type")
	}
}
