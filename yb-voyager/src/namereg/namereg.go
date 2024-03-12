package namereg

import (
	"errors"
	"fmt"
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

var NameReg NameRegistry

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

func InitNameRegistry(
	exportDir string, role string,
	sconf *srcdb.Source, sdb srcdb.SourceDB,
	tconf *tgtdb.TargetConf, tdb tgtdb.TargetDB) error {
	NameReg = *NewNameRegistry(exportDir, role, sconf, sdb, tconf, tdb)
	return NameReg.Init()
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

// TODO: only registry tables from tablelist, not all from schema.
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
	reg.DefaultYBSchemaName = reg.tconf.Schema
	if reg.SourceDBTableNames != nil && reg.SourceDBType == POSTGRESQL {
		reg.DefaultYBSchemaName = reg.DefaultSourceDBSchemaName
	}
	switch reg.SourceDBType {
	case POSTGRESQL:
		reg.YBSchemaNames = reg.SourceDBSchemaNames
	default:
		reg.YBSchemaNames = []string{reg.tconf.Schema}
	}
	for _, schemaName := range reg.YBSchemaNames {
		tableNames, err := yb.GetAllTableNamesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all table names: %w", err)
		}
		m[schemaName] = tableNames
	}
	reg.YBTableNames = m
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
func (reg *NameRegistry) LookupTableName(tableNameArg string) (*sqlname.NameTuple, error) {
	// TODO: REVISIT. Removing the check for reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE because it's possible that import-data-to-source-replica
	// starts before import-data-to-target and so , defaultYBSchemaName will not be set.
	// if (reg.role == TARGET_DB_IMPORTER_ROLE || reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE) &&
	if (reg.role == TARGET_DB_IMPORTER_ROLE) &&
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

	var sourceName *sqlname.ObjectName
	var targetName *sqlname.ObjectName
	if reg.SourceDBTableNames != nil { // nil for `import data file` mode.
		sourceName, err = reg.lookup(
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
					sourceName, err = reg.lookup(
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
		targetName, err = reg.lookup(
			YUGABYTEDB, reg.YBTableNames, reg.DefaultYBSchemaName, schemaName, tableName)
		if err != nil {
			errObj := &ErrMultipleMatchingNames{}
			if errors.As(err, &errObj) {
				// A special case.
				if lo.Contains(errObj.Names, strings.ToLower(tableName)) {
					targetName, err = reg.lookup(
						YUGABYTEDB, reg.YBTableNames, reg.DefaultYBSchemaName, schemaName, strings.ToLower(tableName))
				}
			}
			if err != nil {
				return nil, fmt.Errorf("lookup target table name [%s]: %w", tableNameArg, err)
			}
		}
	}
	if sourceName == nil && targetName == nil {
		return nil, &ErrNameNotFound{ObjectType: "table", Name: tableNameArg}
	}

	ntup := NewNameTuple(reg.role, sourceName, targetName)
	return ntup, nil
}

func (reg *NameRegistry) lookup(
	dbType string, m map[string][]string, defaultSchemaName, schemaName, tableName string) (*sqlname.ObjectName, error) {

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
	return sqlname.NewObjectName(dbType, defaultSchemaName, schemaName, tableName), nil
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

func NewNameTuple(role string, sourceName *sqlname.ObjectName, targetName *sqlname.ObjectName) *sqlname.NameTuple {
	t := sqlname.NameTuple{SourceName: sourceName, TargetName: targetName}
	switch role {
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
	return &t
}
