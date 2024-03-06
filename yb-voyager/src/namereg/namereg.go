package namereg

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/samber/lo"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	YUGABYTEDB = sqlname.YUGABYTEDB
	POSTGRESQL = sqlname.POSTGRESQL
	ORACLE     = sqlname.ORACLE
	MYSQL      = sqlname.MYSQL
)

const (
	TARGET_DB_IMPORTER_ROLE         = "target_db_importer"
	SOURCE_DB_IMPORTER_ROLE         = "source_db_importer"         // Fallback.
	SOURCE_REPLICA_DB_IMPORTER_ROLE = "source_replica_db_importer" // Fall-forward.
	SOURCE_DB_EXPORTER_ROLE         = "source_db_exporter"
	TARGET_DB_EXPORTER_FF_ROLE      = "target_db_exporter_ff"
	TARGET_DB_EXPORTER_FB_ROLE      = "target_db_exporter_fb"
	IMPORT_FILE_ROLE                = "import_file"
)

type NameRegistry struct {
	SourceDBType string

	// All schema and table names are in the format as stored in DB catalog.
	SourceDBSchemaNames       []string
	DefaultSourceDBSchemaName string
	// SourceDBTableNames has one entry for `DefaultSourceReplicaDBSchemaName` if `Mode` is `IMPORT_TO_SOURCE_REPLICA_MODE`.
	SourceDBTableNames map[string][]string // nil for `import data file` mode.

	YBSchemaNames       []string
	DefaultYBSchemaName string
	YBTableNames        map[string][]string

	DefaultSourceReplicaDBSchemaName string
	// Source replica has same table name list as original source.

	// Private members are not saved in the JSON file.
	filePath string
	role     string
	sconf    *srcdb.Source
	sdb      srcdb.SourceDB
	tconf    *tgtdb.TargetConf
	tdb      tgtdb.TargetDB
}

func NewNameRegistry(
	exportDir string, role string,
	sconf *srcdb.Source, sdb srcdb.SourceDB,
	tconf *tgtdb.TargetConf, tdb tgtdb.TargetDB) *NameRegistry {

	return &NameRegistry{
		filePath: fmt.Sprintf("%s/metainfo/name_registry.json", exportDir),
		role:     role,
		sconf:    sconf,
		sdb:      sdb,
		tconf:    tconf,
		tdb:      tdb,
	}
}

func (reg *NameRegistry) Init() error {
	log.Infof("initialising name registry: %s", reg.filePath)
	jsonFile := jsonfile.NewJsonFile[NameRegistry](reg.filePath)
	if utils.FileOrFolderExists(reg.filePath) {
		err := jsonFile.Load(reg)
		if err != nil {
			return fmt.Errorf("load name registry: %w", err)
		}
	}
	registryUpdated, err := reg.registerNames()
	if err != nil {
		return fmt.Errorf("register names: %w", err)
	}
	if registryUpdated {
		err = jsonFile.Update(func(nr *NameRegistry) {
			*nr = *reg
		})
		if err != nil {
			return fmt.Errorf("create/update name registry: %w", err)
		}
	}
	return nil
}

func (reg *NameRegistry) registerNames() (bool, error) {
	switch true {
	case reg.role == SOURCE_DB_EXPORTER_ROLE && reg.SourceDBTableNames == nil:
		log.Info("registering source names in the name registry")
		return reg.registerSourceNames()
	case (reg.role == TARGET_DB_IMPORTER_ROLE || reg.role == IMPORT_FILE_ROLE) && reg.YBTableNames == nil:
		log.Info("registering YB names in the name registry")
		return reg.registerYBNames()
	case reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE && reg.DefaultSourceReplicaDBSchemaName == "":
		log.Infof("setting default source replica schema name in the name registry: %s", reg.DefaultSourceDBSchemaName)
		defaultSchema := lo.Ternary(reg.SourceDBType == POSTGRESQL, reg.DefaultSourceDBSchemaName, reg.tconf.Schema)
		reg.setDefaultSourceReplicaDBSchemaName(defaultSchema)
		return true, nil
	}
	log.Infof("no name registry update required: mode %q", reg.role)
	return false, nil
}

func (reg *NameRegistry) registerSourceNames() (bool, error) {
	reg.SourceDBType = reg.sconf.DBType
	reg.initSourceDBSchemaNames()
	m := make(map[string][]string)
	for _, schemaName := range reg.SourceDBSchemaNames {
		tableNames, err := reg.sdb.GetAllTableNamesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all table names: %w", err)
		}
		m[schemaName] = tableNames
	}
	reg.SourceDBTableNames = m
	return true, nil
}

func (reg *NameRegistry) initSourceDBSchemaNames() {
	// source.Schema contains only one schema name for MySQL and Oracle; whereas
	// it contains a pipe separated list for postgres.
	switch reg.sconf.DBType {
	case ORACLE:
		reg.SourceDBSchemaNames = []string{strings.ToUpper(reg.sconf.Schema)}
	case MYSQL:
		reg.SourceDBSchemaNames = []string{reg.sconf.DBName}
	case POSTGRESQL:
		reg.SourceDBSchemaNames = lo.Map(strings.Split(reg.sconf.Schema, "|"), func(s string, _ int) string {
			return strings.ToLower(s)
		})
	}
	if len(reg.SourceDBSchemaNames) == 1 {
		reg.DefaultSourceDBSchemaName = reg.SourceDBSchemaNames[0]
	} else if lo.Contains(reg.SourceDBSchemaNames, "public") {
		reg.DefaultSourceDBSchemaName = "public"
	}
}

func (reg *NameRegistry) registerYBNames() (bool, error) {
	type YBDB interface {
		GetAllSchemaNamesRaw() ([]string, error)
		GetAllTableNamesRaw(schemaName string) ([]string, error)
	} // Only implemented by TargetYugabyteDB and dummyTargetDB.
	if reg.tdb == nil {
		return false, fmt.Errorf("target db is nil")
	}
	yb, ok := reg.tdb.(YBDB)
	if !ok {
		return false, fmt.Errorf("target db is not YugabyteDB")
	}

	m := make(map[string][]string)
	schemaNames, err := yb.GetAllSchemaNamesRaw()
	if err != nil {
		return false, fmt.Errorf("get all schema names: %w", err)
	}
	reg.DefaultYBSchemaName = reg.tconf.Schema
	if reg.SourceDBTableNames != nil && reg.SourceDBType == POSTGRESQL {
		reg.DefaultYBSchemaName = reg.DefaultSourceDBSchemaName
	}
	for _, schemaName := range schemaNames {
		tableNames, err := yb.GetAllTableNamesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all table names: %w", err)
		}
		m[schemaName] = tableNames
	}
	reg.YBTableNames = m
	reg.YBSchemaNames = schemaNames
	return true, nil
}

func (reg *NameRegistry) setDefaultSourceReplicaDBSchemaName(defaultSourceReplicaDBSchemaName string) error {
	reg.DefaultSourceReplicaDBSchemaName = defaultSourceReplicaDBSchemaName
	reg.SourceDBTableNames[defaultSourceReplicaDBSchemaName] = reg.SourceDBTableNames[reg.DefaultSourceDBSchemaName]
	return nil
}

func (reg *NameRegistry) DefaultSourceSideSchemaName() string {
	originalSourceModes := []string{
		SOURCE_DB_EXPORTER_ROLE,
		SOURCE_DB_IMPORTER_ROLE,
		TARGET_DB_IMPORTER_ROLE,
		TARGET_DB_EXPORTER_FF_ROLE,
		TARGET_DB_EXPORTER_FB_ROLE,
	}
	if lo.Contains(originalSourceModes, reg.role) {
		return reg.DefaultSourceDBSchemaName
	} else if reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE {
		return reg.DefaultSourceReplicaDBSchemaName
	} else {
		return ""
	}
}

//================================================

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
	if (reg.role == TARGET_DB_IMPORTER_ROLE || reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE) &&
		(reg.DefaultSourceSideSchemaName() == "") != (reg.DefaultYBSchemaName == "") {

		msg := "either both or none of the default schema names should be set"
		return nil, fmt.Errorf("%s: [%s], [%s]", msg,
			reg.DefaultSourceSideSchemaName(), reg.DefaultYBSchemaName)
	}
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
	if schemaName == reg.DefaultSourceSideSchemaName() || schemaName == reg.DefaultYBSchemaName {
		schemaName = ""
	}
	ntup := &NameTuple{}
	if reg.SourceDBTableNames != nil { // nil for `import data file` mode.
		ntup.SourceName, err = reg.lookup(
			reg.SourceDBType, reg.SourceDBTableNames, reg.DefaultSourceSideSchemaName(), schemaName, tableName)
		if err != nil {
			errObj := &ErrMultipleMatchingNames{}
			if errors.As(err, &errObj) {
				// Case insensitive match.
				caseInsensitiveName := tableName
				if reg.SourceDBType == POSTGRESQL || reg.SourceDBType == YUGABYTEDB {
					caseInsensitiveName = strings.ToLower(tableName)
				} else if reg.SourceDBType == ORACLE {
					caseInsensitiveName = strings.ToUpper(tableName)
				}
				if lo.Contains(errObj.Names, caseInsensitiveName) {
					ntup.SourceName, err = reg.lookup(
						reg.SourceDBType, reg.SourceDBTableNames, reg.DefaultSourceSideSchemaName(), schemaName, caseInsensitiveName)
				}
			}
			if err != nil {
				// `err` can be: no default schema, no matching name, multiple matching names.
				return nil, fmt.Errorf("lookup source table name [%s.%s]: %w", schemaName, tableName, err)
			}
		}
	}
	if reg.YBTableNames != nil { // nil in `export` mode.
		ntup.TargetName, err = reg.lookup(
			YUGABYTEDB, reg.YBTableNames, reg.DefaultYBSchemaName, schemaName, tableName)
		if err != nil {
			errObj := &ErrMultipleMatchingNames{}
			if errors.As(err, &errObj) {
				// A special case.
				if lo.Contains(errObj.Names, strings.ToLower(tableName)) {
					ntup.TargetName, err = reg.lookup(
						YUGABYTEDB, reg.YBTableNames, reg.DefaultYBSchemaName, schemaName, strings.ToLower(tableName))
				}
			}
			if err != nil {
				return nil, fmt.Errorf("lookup target table name [%s]: %w", tableNameArg, err)
			}
		}
	}
	// Set the current table name based on the mode.
	ntup.SetMode(reg.role)
	if ntup.SourceName == nil && ntup.TargetName == nil {
		return nil, &ErrNameNotFound{ObjectType: "table", Name: tableNameArg}
	}
	return ntup, nil
}

func (reg *NameRegistry) lookup(
	dbType string, m map[string][]string, defaultSchemaName, schemaName, tableName string) (*ObjectName, error) {

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
	return newObjectName(dbType, defaultSchemaName, schemaName, tableName), nil
}

type ErrMultipleMatchingNames struct {
	ObjectType string
	Names      []string
}

func (e *ErrMultipleMatchingNames) Error() string {
	return fmt.Sprintf("multiple matching %s names: %s", e.ObjectType, strings.Join(e.Names, ", "))
}

type ErrNameNotFound struct {
	ObjectType string
	Name       string
}

func (e *ErrNameNotFound) Error() string {
	return fmt.Sprintf("%s name not found: %s", e.ObjectType, e.Name)
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
		return "", &ErrMultipleMatchingNames{ObjectType: objType, Names: candidateNames}
	}
	return "", &ErrNameNotFound{ObjectType: objType, Name: name}
}

//================================================

type identifier struct {
	Quoted, Unquoted, MinQuoted string
}

// Can be a name of a table, sequence, materialised view, etc.
type ObjectName struct {
	SchemaName        string
	FromDefaultSchema bool

	Qualified    identifier
	Unqualified  identifier
	MinQualified identifier
}

func newObjectName(dbType, defaultSchemaName, schemaName, tableName string) *ObjectName {
	result := &ObjectName{
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

func (nv *ObjectName) String() string {
	return nv.MinQualified.MinQuoted
}

func (nv *ObjectName) MatchesPattern(pattern string) (bool, error) {
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
	Mode        string
	CurrentName *ObjectName
	SourceName  *ObjectName
	TargetName  *ObjectName
}

func (t1 *NameTuple) Equals(t2 *NameTuple) bool {
	return reflect.DeepEqual(t1, t2)
}

func (t *NameTuple) SetMode(mode string) {
	t.Mode = mode
	switch mode {
	case TARGET_DB_IMPORTER_ROLE:
		t.CurrentName = t.TargetName
	case SOURCE_DB_IMPORTER_ROLE:
		t.CurrentName = t.SourceName
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		t.CurrentName = t.SourceName
	case SOURCE_DB_EXPORTER_ROLE:
		t.CurrentName = t.SourceName
	case TARGET_DB_EXPORTER_FF_ROLE, TARGET_DB_EXPORTER_FB_ROLE:
		t.CurrentName = t.TargetName
	default:
		t.CurrentName = nil
	}
}

func (t *NameTuple) String() string {
	return t.CurrentName.String()
}

func (t *NameTuple) MatchesPattern(pattern string) (bool, error) {
	for _, tableName := range []*ObjectName{t.SourceName, t.TargetName} {
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
	return t.CurrentName.Qualified.Quoted
}

func (t *NameTuple) ForCatalogQuery() (string, string) {
	return t.CurrentName.SchemaName, t.CurrentName.Unqualified.Unquoted
}

func (t *NameTuple) ForKey() string {
	if t.SourceName != nil {
		return t.SourceName.Qualified.Quoted
	}
	return t.TargetName.Qualified.Quoted
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
