package namereg

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/samber/lo"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	YUGABYTEDB = sqlname.YUGABYTEDB
	POSTGRESQL = sqlname.POSTGRESQL
	ORACLE     = sqlname.ORACLE
	MYSQL      = sqlname.MYSQL
)

const (
	IMPORT_TO_TARGET_MODE         = "import_to_target"
	IMPORT_TO_SOURCE_REPLICA_MODE = "import_to_source_replica"
	EXPORT_FROM_SOURCE_MODE       = "export_from_source"
	EXPORT_FROM_TARGET_MODE       = "export_from_target"

	UNSPECIFIED_MODE = "unspecified"
)

type NameRegistry struct {
	metaDB *metadb.MetaDB

	Mode         string // "import" or "export"
	SourceDBType string

	SourceDBSchemaNames       []string
	TargetDBSchemaNames       []string
	DefaultSourceDBSchemaName string
	DefaultTargetDBSchemaName string

	// All schema and table names are in the format as stored in DB catalog.
	SourceDBTableNames map[string][]string // nil for `import data file` mode.
	TargetDBTableNames map[string][]string
}

func NewNameRegistry(metaDB *metadb.MetaDB) *NameRegistry {
	return &NameRegistry{
		Mode:   UNSPECIFIED_MODE,
		metaDB: metaDB,
	}
}

func (reg *NameRegistry) Init() error {
	return nil
}

func (reg *NameRegistry) loadFromMetaDB() error {
	return nil
}

func (reg *NameRegistry) saveToMetaDB() error {
	return nil
}

func (reg *NameRegistry) SetSourceDBType(sourceDBType string) error {
	reg.SourceDBType = sourceDBType
	return reg.saveToMetaDB()
}

func (reg *NameRegistry) SetSourceDBSchemaNames(sdb srcdb.SourceDB, sourceDBSchemaNamesFromCLI []string) error {
	reg.SourceDBSchemaNames = sourceDBSchemaNamesFromCLI
	return reg.saveToMetaDB()
}

func (reg *NameRegistry) SetTargetDBSchemaNames(tdb tgtdb.TargetDB, targetDBSchemaNamesFromCLI []string) error {
	reg.TargetDBSchemaNames = targetDBSchemaNamesFromCLI
	return reg.saveToMetaDB()
}

func (reg *NameRegistry) RegisterNamesFromSourceDB(sdb srcdb.SourceDB) error {
	return nil
}

func (reg *NameRegistry) RegisterNamesFromTargetDB(tdb tgtdb.TargetDB) error {
	return nil
}

/*
`tableName` can be qualified/unqualified, quoted/unquoted.
If case-sensitive match exits, the lookup is successful irrespective of other variants.
If case-insensitive match exits, the lookup is successful only if there is exactly one match.

foobar, "foobar", FooBar, "FooBar", FOOBAR, "FOOBAR",
(fuzzy-case-match) fooBar, "fooBar"
schema1.foobar, schema1."foobar", schema1.FooBar, schema1."FooBar", schema1.FOOBAR, schema1."FOOBAR",
(fuzzy-case-match) schema1.fooBar, schema1."fooBar"
*/
func (reg *NameRegistry) LookupTableName(tableNameArg string) (*NameTuple, error) {
	var err error
	parts := strings.Split(tableNameArg, ".")

	var schemaName, tableName string
	switch true {
	case len(parts) == 1:
		schemaName = ""
		tableName = tableNameArg
	case len(parts) == 2:
		schemaName = parts[0]
		tableName = parts[1]
	default:
		return nil, fmt.Errorf("invalid table name: %s", tableNameArg)
	}
	objTableName := &NameTuple{}
	if reg.SourceDBTableNames != nil { // nil for `import data file` mode.
		objTableName.SourceTableName, err = reg.lookup(
			reg.SourceDBType, reg.SourceDBTableNames, reg.DefaultSourceDBSchemaName, schemaName, tableName)
		if err != nil {
			// `err` can be: no default schema, no matching name, multiple matching names.
			return nil, fmt.Errorf("lookup source table name [%s]: %w", tableNameArg, err)
		}
	}
	if reg.TargetDBTableNames != nil { // nil in `export` mode.
		objTableName.TargetTableName, err = reg.lookup(
			YUGABYTEDB, reg.TargetDBTableNames, reg.DefaultTargetDBSchemaName, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("lookup target table name [%s]: %w", tableNameArg, err)
		}
	}
	// Set the current table name based on the mode.
	objTableName.SetMode(reg.Mode)
	return objTableName, nil
}

func (reg *NameRegistry) lookup(
	dbType string, m map[string][]string, defaultSchemaName, schemaName, tableName string) (*TableName, error) {

	if schemaName == "" {
		if defaultSchemaName == "" {
			return nil, fmt.Errorf("no default schema name: qualify table name [%s]", tableName)
		}
		schemaName = defaultSchemaName
	}
	schemaName, err := matchName("schema", lo.Keys(m), schemaName)
	if err != nil {
		return nil, err
	}
	tableName, err = matchName("table", m[schemaName], tableName)
	if err != nil {
		return nil, err
	}
	return newTableName(dbType, defaultSchemaName, schemaName, tableName), nil
}

func matchName(objType string, names []string, name string) (string, error) {
	if name[0] == '"' {
		if name[len(name)-1] != '"' {
			return "", fmt.Errorf("invalid quoted %s name: [%s]", objType, name)
		}
		name = name[1 : len(name)-1]
	}
	var candidateNames []string
	for _, n := range names {
		if n == name { // Exact match.
			return n, nil
		}
		if strings.EqualFold(n, name) {
			candidateNames = append(candidateNames, n)
		}
	}
	if len(candidateNames) == 1 {
		return candidateNames[0], nil
	}
	if len(candidateNames) > 1 {
		return "", fmt.Errorf("multiple matching %s names for [%s]: %s", objType, name, strings.Join(candidateNames, ", "))
	}
	return "", fmt.Errorf("no matching %s name: [%s]", objType, name)
}

//================================================

type identifier struct {
	Quoted, Unquoted, MinQuoted string
}

type TableName struct {
	SchemaName        string
	FromDefaultSchema bool

	Qualified    identifier
	Unqualified  identifier
	MinQualified identifier
}

func newTableName(dbType, defaultSchemaName, schemaName, tableName string) *TableName {
	result := &TableName{
		SchemaName:        schemaName,
		FromDefaultSchema: schemaName == defaultSchemaName,
		Qualified: identifier{
			Quoted:    schemaName + "." + quote(dbType, tableName),
			Unquoted:  schemaName + "." + tableName,
			MinQuoted: schemaName + "." + minQuote(tableName, dbType),
		},
		Unqualified: identifier{
			Quoted:    quote(dbType, tableName),
			Unquoted:  tableName,
			MinQuoted: minQuote(tableName, dbType),
		},
	}
	result.MinQualified = lo.Ternary(result.FromDefaultSchema, result.Unqualified, result.Qualified)
	return result
}

func (nv *TableName) String() string {
	return nv.MinQualified.MinQuoted
}

func (nv *TableName) MatchesPattern(pattern string) (bool, error) {
	parts := strings.Split(pattern, ".")
	switch true {
	case len(parts) == 2:
		if !strings.EqualFold(parts[0], nv.SchemaName) {
			return false, nil
		}
		pattern = parts[1]
	case len(parts) == 1:
		if !nv.FromDefaultSchema {
			return false, nil
		}
		pattern = parts[0]
	default:
		return false, fmt.Errorf("invalid pattern: %s", pattern)
	}
	match1, err := filepath.Match(pattern, nv.Unqualified.Unquoted)
	if err != nil {
		return false, fmt.Errorf("invalid pattern: %s", pattern)
	}
	if match1 {
		return true, nil
	}
	match2, err := filepath.Match(pattern, nv.Unqualified.Quoted)
	if err != nil {
		return false, fmt.Errorf("invalid pattern: %s", pattern)
	}
	return match2, nil
}

// <SourceTableName, TargetTableName>
type NameTuple struct {
	Mode             string
	CurrentTableName *TableName
	SourceTableName  *TableName
	TargetTableName  *TableName
}

func (t1 *NameTuple) Equals(t2 *NameTuple) bool {
	return reflect.DeepEqual(t1, t2)
}

func (t *NameTuple) SetMode(mode string) {
	t.Mode = mode
	switch mode {
	case IMPORT_TO_TARGET_MODE:
		t.CurrentTableName = t.TargetTableName
	case IMPORT_TO_SOURCE_REPLICA_MODE:
		t.CurrentTableName = t.SourceTableName
	case EXPORT_FROM_SOURCE_MODE:
		t.CurrentTableName = t.SourceTableName
	case EXPORT_FROM_TARGET_MODE:
		t.CurrentTableName = t.TargetTableName
	default:
		t.CurrentTableName = nil
	}
}

func (t *NameTuple) String() string {
	return t.CurrentTableName.String()
}

func (t *NameTuple) MatchesPattern(pattern string) (bool, error) {
	for _, tableName := range []*TableName{t.SourceTableName, t.TargetTableName} {
		if tableName == nil {
			continue
		}
		match, err := tableName.MatchesPattern(pattern)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func (t *NameTuple) ForUserQuery() string {
	return t.CurrentTableName.Qualified.Quoted
}

func (t *NameTuple) ForCatalogQuery() (string, string) {
	return t.CurrentTableName.SchemaName, t.CurrentTableName.Unqualified.Unquoted
}

//================================================

func quote(dbType, name string) string {
	switch dbType {
	case POSTGRESQL, YUGABYTEDB, ORACLE:
		return `"` + name + `"`
	case MYSQL:
		return name
	default:
		panic("unknown source db type")
	}
}

func minQuote(objectName, sourceDBType string) string {
	switch sourceDBType {
	case YUGABYTEDB, POSTGRESQL:
		if sqlname.IsAllLowercase(objectName) && !sqlname.IsReservedKeywordPG(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	case MYSQL:
		return objectName
	case ORACLE:
		if sqlname.IsAllUppercase(objectName) && !sqlname.IsReservedKeywordOracle(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	default:
		panic("invalid source db type")
	}
}
