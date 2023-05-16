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
		return s // TODO - learn the semantics of quoting in MySQL.
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

func minQuote(objectName, sourceDBType string) string {
	objectName = unquote(objectName, sourceDBType)
	switch sourceDBType {
	case YUGABYTE, POSTGRESQL:
		if IsAllLowercase(objectName) && !IsReservedKeyword(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	case MYSQL:
		return objectName
	case ORACLE:
		if IsAllUppercase(objectName) && !IsReservedKeyword(objectName) {
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

func IsReservedKeyword(word string) bool {
	return slices.Contains(PgReservedKeywords, word)
}
