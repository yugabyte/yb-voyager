package namereg

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

const (
	TARGET_DB_IMPORTER_ROLE         = "target_db_importer"
	SOURCE_DB_IMPORTER_ROLE         = "source_db_importer"         // Fallback.
	SOURCE_REPLICA_DB_IMPORTER_ROLE = "source_replica_db_importer" // Fall-forward.
	SOURCE_DB_EXPORTER_ROLE         = "source_db_exporter"
	SOURCE_DB_EXPORTER_STATUS_ROLE  = "source_db_exporter_status"
	TARGET_DB_EXPORTER_FF_ROLE      = "target_db_exporter_ff"
	TARGET_DB_EXPORTER_FB_ROLE      = "target_db_exporter_fb"
	IMPORT_FILE_ROLE                = "import_file"
)

var NameReg NameRegistry

type SourceDBInterface interface {
	GetAllTableNamesRaw(schemaName string) ([]string, error)
	GetAllSequencesRaw(schemaName string) ([]string, error)
	GetAllSchemaNamesIdentifiers() ([]sqlname.Identifier, error)
}

type YBDBInterface interface {
	GetAllSchemaNamesRaw() ([]string, error)
	GetAllTableNamesRaw(schemaName string) ([]string, error)
	GetAllSequencesRaw(schemaName string) ([]string, error)
} // Only implemented by TargetYugabyteDB and dummyTargetDB.

type NameRegistryParams struct {
	FilePath string
	Role     string

	SourceDBType   string
	SourceDBSchema string
	SourceDBName   string
	SDB            SourceDBInterface

	TargetDBSchema string
	YBDB           YBDBInterface
}

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

	params NameRegistryParams

	SourceDBSequenceNames map[string][]string
	YBSequenceNames       map[string][]string
}

func InitNameRegistry(params NameRegistryParams) error {
	log.Infof("Initializing name registry with params - %v", params)
	NameReg = *NewNameRegistry(params)
	return NameReg.Init()
}

func NewNameRegistry(params NameRegistryParams) *NameRegistry {

	return &NameRegistry{
		params: params,
	}
}

func (reg *NameRegistry) Init() error {
	log.Infof("initialising name registry: %s", reg.params.FilePath)
	jsonFile := jsonfile.NewJsonFile[NameRegistry](reg.params.FilePath)
	if utils.FileOrFolderExists(reg.params.FilePath) {
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
		err := reg.save()
		if err != nil {
			return fmt.Errorf("create/update name registry: %w", err)
		}
	}
	return nil
}

func (reg *NameRegistry) save() error {
	log.Infof("saving name registry: %s", reg.params.FilePath)
	jsonFile := jsonfile.NewJsonFile[NameRegistry](reg.params.FilePath)
	return jsonFile.Update(func(nr *NameRegistry) {
		*nr = *reg
	})
}

// TODO: only registry tables from tablelist, not all from schema.
func (reg *NameRegistry) registerNames() (bool, error) {
	switch true {
	case reg.params.Role == SOURCE_DB_EXPORTER_ROLE && reg.SourceDBTableNames == nil:
		log.Info("registering source names in the name registry")
		return reg.registerSourceNames()
	case (reg.params.Role == TARGET_DB_IMPORTER_ROLE || reg.params.Role == IMPORT_FILE_ROLE) && reg.YBTableNames == nil:
		log.Info("registering YB names in the name registry")
		return reg.registerYBNames()
	case reg.params.Role == SOURCE_REPLICA_DB_IMPORTER_ROLE && reg.DefaultSourceReplicaDBSchemaName == "":
		log.Infof("setting default source replica schema name in the name registry: %s", reg.DefaultSourceDBSchemaName)
		defaultSchema := lo.Ternary(reg.SourceDBType == constants.POSTGRESQL, reg.DefaultSourceDBSchemaName, reg.params.TargetDBSchema)
		reg.setDefaultSourceReplicaDBSchemaName(defaultSchema)
		return true, nil
	}
	log.Infof("no name registry update required: mode %q", reg.params.Role)
	return false, nil
}

func (reg *NameRegistry) UnRegisterYBNames() error {
	log.Info("unregistering YB names")
	reg.YBTableNames = nil
	reg.YBSchemaNames = nil
	reg.DefaultYBSchemaName = ""
	reg.save()
	return nil
}

func (reg *NameRegistry) registerSourceNames() (bool, error) {
	if reg.params.SDB == nil {
		return false, goerrors.Errorf("source db connection is not available")
	}
	reg.SourceDBType = reg.params.SourceDBType
	err := reg.initSourceDBSchemaNames()
	if err != nil {
		return false, fmt.Errorf("init source db schema names: %w", err)
	}
	tableMap := make(map[string][]string)
	sequenceMap := make(map[string][]string)
	for _, schemaName := range reg.SourceDBSchemaNames {
		tableNames, err := reg.params.SDB.GetAllTableNamesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all table names: %w", err)
		}
		tableMap[schemaName] = tableNames
		seqNames, err := reg.params.SDB.GetAllSequencesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all sequence names: %w", err)
		}
		sequenceMap[schemaName] = append(sequenceMap[schemaName], seqNames...)
	}
	reg.SourceDBTableNames = tableMap
	reg.SourceDBSequenceNames = sequenceMap
	return true, nil
}

// this function returns the tables names in the namereg which is required by the export data
// table-list code path for getting a full list of tables from first run i.e. registered table list
// returning the objectNames from here as we need to get nameTuple based on type of tables either leaf partition or normal..
func (reg *NameRegistry) GetRegisteredTableList(ignoreOtherSideOfMappingIfNotFound bool) ([]sqlname.NameTuple, error) {
	var res []sqlname.NameTuple
	var m map[string][]string            // Complete list of tables and sequences
	var sequencesMap map[string][]string // only sequence list
	ignoreIfTargetNotFound := false
	switch reg.params.Role {
	case SOURCE_DB_EXPORTER_ROLE, SOURCE_DB_IMPORTER_ROLE, SOURCE_REPLICA_DB_IMPORTER_ROLE:
		m = reg.SourceDBTableNames
		sequencesMap = reg.SourceDBSequenceNames
		ignoreIfTargetNotFound = true
	case TARGET_DB_EXPORTER_FB_ROLE, TARGET_DB_EXPORTER_FF_ROLE, TARGET_DB_IMPORTER_ROLE:
		m = reg.YBTableNames
		sequencesMap = reg.YBSequenceNames
	}
	for s, tables := range m {
		for _, t := range tables {
			if slices.Contains(sequencesMap[s], t) {
				//If its a sequence continue and not append in the registerd list
				continue
			}
			tableName := fmt.Sprintf("%v.%v", s, t)
			if ignoreOtherSideOfMappingIfNotFound {
				if ignoreIfTargetNotFound {
					tuple, err := reg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableName)
					if err != nil {
						return nil, goerrors.Errorf("error lookup for the table name [%v]: %v", tableName, err)
					}
					res = append(res, tuple)
				} else {
					tuple, err := reg.LookupTableNameAndIgnoreIfSourceNotFound(tableName)
					if err != nil {
						return nil, goerrors.Errorf("error lookup for the table name [%v]: %v", tableName, err)
					}
					res = append(res, tuple)
				}
			} else {
				tuple, err := reg.LookupTableName(tableName)
				if err != nil {
					return nil, goerrors.Errorf("error lookup for the table name [%v]: %v", tableName, err)
				}
				res = append(res, tuple)
			}
		}
	}
	return res, nil
}

func (reg *NameRegistry) initSourceDBSchemaNames() error {
	// source.Schema contains only one schema name for MySQL and Oracle; whereas
	// it contains a pipe separated list for postgres.
	switch reg.params.SourceDBType {
	case constants.ORACLE:
		schema := reg.params.SourceDBSchema
		var err error
		reg.SourceDBSchemaNames, err = reg.validateAndSetSchemaNames([]string{schema})
		if err != nil {
			return fmt.Errorf("failed to validate schema names: %w", err)
		}
	case constants.MYSQL:
		reg.SourceDBSchemaNames = []string{reg.params.SourceDBName}
	case constants.POSTGRESQL:
		schemaNames := strings.Split(reg.params.SourceDBSchema, "|")
		var err error
		reg.SourceDBSchemaNames, err = reg.validateAndSetSchemaNames(schemaNames)
		if err != nil {
			return fmt.Errorf("failed to validate schema names: %w", err)
		}
	}
	if len(reg.SourceDBSchemaNames) == 1 {
		reg.DefaultSourceDBSchemaName = reg.SourceDBSchemaNames[0]
	} else if lo.Contains(reg.SourceDBSchemaNames, "public") {
		reg.DefaultSourceDBSchemaName = "public"
	}
	return nil
}

func (reg *NameRegistry) validateAndSetSchemaNames(schemaNames []string) ([]string, error) {
	allSchemas, err := reg.params.SDB.GetAllSchemaNamesIdentifiers()
	if err != nil {
		return nil, fmt.Errorf("get all schema names: %w", err)
	}
	schemaIdenitifiers := sqlname.ParseIdentifiersFromStrings(reg.params.SourceDBType, schemaNames)
	var schemaNotPresent []sqlname.Identifier
	var finalSchemaList []sqlname.Identifier
	for _, schema := range schemaIdenitifiers {
		matchedSchema, matchedSchemaIdentifier := schema.FindBestMatchingIdenitifier(allSchemas)
		if !matchedSchema {
			schemaNotPresent = append(schemaNotPresent, schema)
			continue
		}
		finalSchemaList = append(finalSchemaList, matchedSchemaIdentifier)
	}

	if len(schemaNotPresent) > 0 {
		return nil, goerrors.Errorf("\nFollowing schemas are not present in source database: %v, please provide a valid schema list.\n", sqlname.JoinUnquoted(schemaNotPresent, ", "))
	}
	return sqlname.ExtractUnquoted(finalSchemaList), nil
}
func (reg *NameRegistry) registerYBNames() (bool, error) {
	if reg.params.YBDB == nil {
		return false, goerrors.Errorf("target db is nil")
	}
	yb := reg.params.YBDB

	tableMap := make(map[string][]string)
	sequenceMap := make(map[string][]string)
	reg.DefaultYBSchemaName = reg.params.TargetDBSchema
	if reg.SourceDBTableNames != nil && reg.SourceDBType == constants.POSTGRESQL {
		reg.DefaultYBSchemaName = reg.DefaultSourceDBSchemaName
	}
	switch reg.SourceDBType {
	case constants.POSTGRESQL:
		reg.YBSchemaNames = reg.SourceDBSchemaNames
	default:
		identifiers := sqlname.ParseIdentifiersFromString(constants.YUGABYTEDB, reg.params.TargetDBSchema, "|")
		reg.YBSchemaNames = sqlname.ExtractUnquoted(identifiers)
	}
	for _, schemaName := range reg.YBSchemaNames {
		tableNames, err := yb.GetAllTableNamesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all table names: %w", err)
		}
		tableMap[schemaName] = tableNames
		seqNames, err := yb.GetAllSequencesRaw(schemaName)
		if err != nil {
			return false, fmt.Errorf("get all sequence names: %w", err)
		}
		sequenceMap[schemaName] = append(sequenceMap[schemaName], seqNames...)
	}
	reg.YBTableNames = tableMap
	reg.YBSequenceNames = sequenceMap
	if reg.params.Role == IMPORT_FILE_ROLE {
		//In case of import data fiel we don't have source db type so we set it to yugabyte db as the type is required in lookup for schema identifier
		reg.SourceDBType = constants.YUGABYTEDB
	}
	return true, nil
}

func (reg *NameRegistry) setDefaultSourceReplicaDBSchemaName(defaultSourceReplicaDBSchemaName string) error {
	reg.DefaultSourceReplicaDBSchemaName = defaultSourceReplicaDBSchemaName
	reg.SourceDBTableNames[defaultSourceReplicaDBSchemaName] = reg.SourceDBTableNames[reg.DefaultSourceDBSchemaName]
	return nil
}

func (reg *NameRegistry) DefaultSourceSideSchemaName() string {
	if reg.params.Role == SOURCE_REPLICA_DB_IMPORTER_ROLE {
		return reg.DefaultSourceReplicaDBSchemaName
	} else {
		return reg.DefaultSourceDBSchemaName
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
//TODO: have a separate function for Sequence lookup LookupSequenceName
func (reg *NameRegistry) LookupTableName(tableNameArg string) (sqlname.NameTuple, error) {
	sourceName, targetName, err := reg.lookupSourceAndTargetTableNames(tableNameArg, false, false)
	if err != nil {
		return sqlname.NameTuple{}, err
	}
	ntup := NewNameTuple(reg.params.Role, sourceName, targetName)
	return ntup, nil
}

//================================================

/*
this function hels returning the nametuple in case the tableNameArg is only present in target,
so this will return a nametuple with one side populated only
In case both the source and target not present for the table this will return error.

Usecases:
0. export-data: Required to run guardrails to prevent changing table-list in resumption cases of export-data.
1. import-data: Table was exported in export-data, but table not created on target, and excluded in table-list of import-data. Tables which are not present on target are not allowed to be migrated.
2. import-data-status: as a result of above case of import-data, status commands also need to support tables that were exported, but not created/migrated on target. status for ignored tables is reported as NOT_STARTED.
3. export-data-status: run export-data tables (a,b,c), import-data(a,b). run export-data-status: at this point, we need to ignore if c is not found on target. (we still report it in the status output)
4. schema-registry: BETA_FAST_DATA_EXPORT case (debezium) where table is exported, but not created on target. Only tables present in the table-list (already validated to be present on target) will be loaded from the registry.
*/

func (reg *NameRegistry) LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole(tableNameArg string) (sqlname.NameTuple, error) {
	switch reg.params.Role {
	case TARGET_DB_IMPORTER_ROLE, SOURCE_DB_EXPORTER_STATUS_ROLE, SOURCE_DB_EXPORTER_ROLE:
		//For target db importer specific roles only we should use this lookup with ignore if target not found
		//using exporter_status for export data status command as this command can run after import data command so we need to ignore if target not found
		//using source db exporter role for export data command as it gets a registered table list from namereg and we need to ignore the ones that are not present in the target to handle subset of tables from source being migrated
		//not using this function TARGET_DB_EXPORTER ones as they are live migration specific and we shouldn't use it for them.
		sourceName, targetName, err := reg.lookupSourceAndTargetTableNames(tableNameArg, true, false)
		if err != nil {
			return sqlname.NameTuple{}, goerrors.Errorf("error lookup source and target names for table [%v]: %v", tableNameArg, err)
		}
		ntup := NewNameTuple(reg.params.Role, sourceName, targetName)
		return ntup, nil
	default:
		//In case the role is not target db import related we should use proper lookup
		return reg.LookupTableName(tableNameArg)
	}
}

/*
this function helps returning the nametuple in case the tableNameArg is only present in source,
so this will return a nametuple with one side populated only
In case both the source and target not present for the table this will return error.
*/

func (reg *NameRegistry) LookupTableNameAndIgnoreIfSourceNotFound(tableNameArg string) (sqlname.NameTuple, error) {
	sourceName, targetName, err := reg.lookupSourceAndTargetTableNames(tableNameArg, false, true)
	if err != nil {
		return sqlname.NameTuple{}, goerrors.Errorf("error lookup source and target names for table [%v]: %v", tableNameArg, err)
	}
	ntup := NewNameTuple(reg.params.Role, sourceName, targetName)
	return ntup, nil
}

func (reg *NameRegistry) lookupSourceAndTargetTableNames(tableNameArg string, ignoreIfTargetNotFound bool, ignoreIfSourceNotFound bool) (*sqlname.ObjectName, *sqlname.ObjectName, error) {
	if ignoreIfTargetNotFound && ignoreIfSourceNotFound {
		//can't use nametuple if both are ignored
		return nil, nil, goerrors.Errorf("ignoreIfTargetNotFound and ignoreIfSourceNotFound cannot be true at the same time")
	}
	createAllObjectNamesMap := func(m map[string][]string, m1 map[string][]string) map[string][]string {
		if m == nil && m1 == nil {
			return nil
		}
		res := make(map[string][]string)
		for s, objs := range m {
			res[s] = append(res[s], objs...)
		}

		for s, objs := range m1 {
			res[s] = append(res[s], objs...)
		}
		return res
	}
	sourceObjectNameMap := createAllObjectNamesMap(reg.SourceDBTableNames, reg.SourceDBSequenceNames)
	targetObjectNameMap := createAllObjectNamesMap(reg.YBTableNames, reg.YBSequenceNames)
	// TODO: REVISIT. Removing the check for reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE because it's possible that import-data-to-source-replica
	// starts before import-data-to-target and so , defaultYBSchemaName will not be set.
	// if (reg.role == TARGET_DB_IMPORTER_ROLE || reg.role == SOURCE_REPLICA_DB_IMPORTER_ROLE) &&
	if (reg.params.Role == TARGET_DB_IMPORTER_ROLE) &&
		(reg.DefaultSourceSideSchemaName() == "") != (reg.DefaultYBSchemaName == "") {

		msg := "either both or none of the default schema names should be set"
		return nil, nil, goerrors.Errorf("%s: [%s], [%s]", msg,
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
		return nil, nil, goerrors.Errorf("invalid table name: %s", tableNameArg)
	}
	// Consider a case of oracle data migration: source table name is SAKILA.TABLE1 and it is being imported in ybsakila.table1.
	// When a name lookup comes for ybsakila.table1 we have to pair it with SAKILA.TABLE1.
	// Consider the case of fall-forward import-data to source-replica (with different schema for source-replica SAKILA_REPLICA
	// During the snapshot and event data in the beginning before cutover, lookup will be for SAKILA.TABLE1,
	// but we want to get the SourceName to be SAKILA_REPLICA.TABLE1.
	// Therefore, we unqualify the input in case it is equal to the default.
	if reg.checkIfSchemaNameIsDefault(schemaName) {
		schemaName = ""
	}

	var sourceName *sqlname.ObjectName
	var targetName *sqlname.ObjectName
	if sourceObjectNameMap != nil { // nil for `import data file` mode.
		sourceName, err = reg.lookup(
			reg.SourceDBType, sourceObjectNameMap, reg.DefaultSourceSideSchemaName(), schemaName, tableName)
		if err != nil {
			errObj := &ErrMultipleMatchingNames{}
			if errors.As(err, &errObj) {
				// Case insensitive match.
				caseInsensitiveName := tableName
				if reg.SourceDBType == constants.POSTGRESQL || reg.SourceDBType == constants.YUGABYTEDB {
					caseInsensitiveName = strings.ToLower(tableName)
				} else if reg.SourceDBType == constants.ORACLE {
					caseInsensitiveName = strings.ToUpper(tableName)
				}
				if lo.Contains(errObj.Names, caseInsensitiveName) {
					sourceName, err = reg.lookup(
						reg.SourceDBType, sourceObjectNameMap, reg.DefaultSourceSideSchemaName(), schemaName, caseInsensitiveName)
				}
			}
			if err != nil {
				errNotFound := &ErrNameNotFound{}
				if ignoreIfSourceNotFound && errors.As(err, &errNotFound) {

					log.Debugf("lookup source table name [%s.%s]: %v", schemaName, tableName, err)
				} else {
					// `err` can be: no default schema, no matching name, multiple matching names.
					return nil, nil, fmt.Errorf("lookup source table name [%s.%s]: %w", schemaName, tableName, err)
				}
			}
		}
	}
	if targetObjectNameMap != nil { // nil in `export` mode.
		targetName, err = reg.lookup(
			constants.YUGABYTEDB, targetObjectNameMap, reg.DefaultYBSchemaName, schemaName, tableName)
		if err != nil {
			errObj := &ErrMultipleMatchingNames{}
			if errors.As(err, &errObj) {
				// A special case.
				if lo.Contains(errObj.Names, strings.ToLower(tableName)) {
					targetName, err = reg.lookup(
						constants.YUGABYTEDB, targetObjectNameMap, reg.DefaultYBSchemaName, schemaName, strings.ToLower(tableName))
				}
			}
			if err != nil {
				errNotFound := &ErrNameNotFound{}
				if ignoreIfTargetNotFound && errors.As(err, &errNotFound) {
					log.Debugf("lookup target table name [%s]: %v", tableNameArg, err)
				} else {
					// `err` can be: no default schema, no matching name, multiple matching names.
					return nil, nil, fmt.Errorf("lookup target table name [%s]: %w", tableNameArg, err)
				}
			}
		}
	}
	if sourceName == nil && targetName == nil {
		return nil, nil, &ErrNameNotFound{ObjectType: "table", Name: tableNameArg}
	}
	return sourceName, targetName, nil
}

func (reg *NameRegistry) LookupSchemaName(schemaName string) (sqlname.Identifier, error) {
	var schemaNames []string
	var dbType string
	switch reg.params.Role {
	case SOURCE_DB_EXPORTER_ROLE, SOURCE_DB_IMPORTER_ROLE, SOURCE_REPLICA_DB_IMPORTER_ROLE:
		schemaNames = reg.SourceDBSchemaNames
		dbType = reg.SourceDBType
		if reg.params.Role == SOURCE_REPLICA_DB_IMPORTER_ROLE && dbType == constants.ORACLE {
			//For oracle source replica , schema may or maynot be the sourceSchemaNames so taking the default source replica schema name
			schemaNames = []string{reg.DefaultSourceReplicaDBSchemaName}
		}
	case TARGET_DB_IMPORTER_ROLE, IMPORT_FILE_ROLE, TARGET_DB_EXPORTER_FF_ROLE, TARGET_DB_EXPORTER_FB_ROLE:
		schemaNames = reg.YBSchemaNames
		dbType = constants.YUGABYTEDB
	default:
		return sqlname.Identifier{}, goerrors.Errorf("invalid role: %s", reg.params.Role)
	}
	schemaIdenitifiers := sqlname.ParseIdentifiersFromStrings(dbType, schemaNames)
	schemaNameIdentifier := sqlname.NewIdentifier(dbType, schemaName)
	matchedSchema, matchedSchemaIdentifier := schemaNameIdentifier.FindBestMatchingIdenitifier(schemaIdenitifiers)
	if !matchedSchema {
		return sqlname.Identifier{}, goerrors.Errorf("schema name not found: %s", schemaName)
	}
	return matchedSchemaIdentifier, nil
}

func (reg *NameRegistry) checkIfSchemaNameIsDefault(schemaName string) bool {
	schemaNameIdentifier := sqlname.NewIdentifier(reg.SourceDBType, schemaName)
	defaultSchemaNameIdentifier := sqlname.NewIdentifier(reg.SourceDBType, reg.DefaultSourceDBSchemaName)
	defaultSourceReplicaSchemaNameIdentifier := sqlname.NewIdentifier(reg.SourceDBType, reg.DefaultSourceReplicaDBSchemaName)
	defaultYBSchemaNameIdentifier := sqlname.NewIdentifier(reg.SourceDBType, reg.DefaultYBSchemaName)
	return schemaNameIdentifier.Equals(defaultSchemaNameIdentifier) || schemaNameIdentifier.Equals(defaultSourceReplicaSchemaNameIdentifier) ||
		schemaNameIdentifier.Equals(defaultYBSchemaNameIdentifier)
}

func (reg *NameRegistry) lookup(
	dbType string, m map[string][]string, defaultSchemaName, schemaName, tableName string) (*sqlname.ObjectName, error) {

	if schemaName == "" {
		if defaultSchemaName == "" {
			return nil, goerrors.Errorf("no default schema name: qualify table name [%s]", tableName)
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
			return "", goerrors.Errorf("invalid quoted %s name: [%s]", objType, name)
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

func NewNameTuple(role string, sourceName *sqlname.ObjectName, targetName *sqlname.ObjectName) sqlname.NameTuple {
	t := sqlname.NameTuple{SourceName: sourceName, TargetName: targetName}
	switch role {
	case TARGET_DB_IMPORTER_ROLE:
		t.CurrentName = t.TargetName
	case SOURCE_DB_IMPORTER_ROLE:
		t.CurrentName = t.SourceName
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		t.CurrentName = t.SourceName
	case SOURCE_DB_EXPORTER_ROLE, SOURCE_DB_EXPORTER_STATUS_ROLE:
		t.CurrentName = t.SourceName
	case TARGET_DB_EXPORTER_FF_ROLE, TARGET_DB_EXPORTER_FB_ROLE:
		t.CurrentName = t.TargetName
	case IMPORT_FILE_ROLE:
		t.CurrentName = t.TargetName
	default:
		t.CurrentName = nil
	}
	return t
}
