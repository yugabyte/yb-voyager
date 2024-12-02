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
package queryparser

import (
	"fmt"
	"slices"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/samber/lo"
)

// Base parser interface
type DDLParser interface {
	Parse(*pg_query.ParseResult) (DDLObject, error)
}

// Base DDL object interface
type DDLObject interface {
	GetObjectName() string
	GetSchemaName() string
}

// TableParser handles parsing CREATE TABLE statements
type TableParser struct{}

func NewTableParser() *TableParser {
	return &TableParser{}
}

/*
e.g. In case if PRIMARY KEY is included in column definition

	 CREATE TABLE example2 (
	 	id numeric NOT NULL PRIMARY KEY,
		country_code varchar(3),
		record_type varchar(5)

) PARTITION BY RANGE (country_code, record_type) ;
stmts:{stmt:{create_stmt:{relation:{relname:"example2"  inh:true  relpersistence:"p"  location:193}  table_elts:{column_def:{colname:"id"
type_name:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"numeric"}}  typemod:-1  location:208}  is_local:true
constraints:{constraint:{contype:CONSTR_NOTNULL  location:216}}  constraints:{constraint:{contype:CONSTR_PRIMARY  location:225}}
location:205}}  ...  partspec:{strategy:PARTITION_STRATEGY_RANGE
part_params:{partition_elem:{name:"country_code"  location:310}}  part_params:{partition_elem:{name:"record_type"  location:324}}
location:290}  oncommit:ONCOMMIT_NOOP}}  stmt_location:178  stmt_len:159}

In case if PRIMARY KEY in column list CREATE TABLE example1 (..., PRIMARY KEY(id,country_code) ) PARTITION BY RANGE (country_code, record_type);
stmts:{stmt:{create_stmt:{relation:{relname:"example1"  inh:true  relpersistence:"p"  location:15}  table_elts:{column_def:{colname:"id"
type_name:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"numeric"}}  ... table_elts:{constraint:{contype:CONSTR_PRIMARY
location:98  keys:{string:{sval:"id"}} keys:{string:{sval:"country_code"}}}}  partspec:{strategy:PARTITION_STRATEGY_RANGE
part_params:{partition_elem:{name:"country_code" location:150}}  part_params:{partition_elem:{name:"record_type"  ...
*/
func (p *TableParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	createTableNode, ok := getCreateTableStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE TABLE statement")
	}

	table := &Table{
		SchemaName:       createTableNode.CreateStmt.Relation.Schemaname,
		TableName:        createTableNode.CreateStmt.Relation.Relname,
		IsUnlogged:       createTableNode.CreateStmt.Relation.GetRelpersistence() == "u",
		IsPartitioned:    createTableNode.CreateStmt.GetPartspec() != nil,
		GeneratedColumns: make([]string, 0),
		Constraints:      make([]TableConstraint, 0),
		PartitionColumns: make([]string, 0),
	}

	// Parse columns and their properties
	for _, element := range createTableNode.CreateStmt.TableElts {
		if element.GetColumnDef() != nil {
			if p.isGeneratedColumn(element.GetColumnDef()) {
				table.GeneratedColumns = append(table.GeneratedColumns, element.GetColumnDef().Colname)
			}
			colName := element.GetColumnDef().GetColname()

			typeNames := element.GetColumnDef().GetTypeName().GetNames()
			typeName, typeSchemaName := getTypeNameAndSchema(typeNames)
			table.Columns = append(table.Columns, TableColumn{
				ColumnName:  colName,
				TypeName:    typeName,
				TypeSchema:  typeSchemaName,
				IsArrayType: len(element.GetColumnDef().GetTypeName().GetArrayBounds()) > 0,
			})

			constraints := element.GetColumnDef().GetConstraints()
			if constraints != nil {
				for idx, c := range constraints {
					constraint := c.GetConstraint()
					if slices.Contains(deferrableConstraintsList, constraint.Contype) {
						if idx > 0 {
							lastConstraint := table.Constraints[len(table.Constraints)-1]
							lastConstraint.IsDeferrable = true
							table.Constraints[len(table.Constraints)-1] = lastConstraint
						}
					} else {
						table.addConstraint(constraint.Contype, []string{colName}, constraint.Conname, false)
					}
				}
			}

		} else if element.GetConstraint() != nil {
			constraint := element.GetConstraint()
			conType := element.GetConstraint().Contype
			columns := parseColumnsFromKeys(constraint.GetKeys())
			if conType == EXCLUSION_CONSTR_TYPE {
				exclusions := constraint.GetExclusions()
				//exclusions:{list:{items:{index_elem:{name:"room_id" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
				//items:{list:{items:{string:{sval:"="}}}}}}
				columns = p.parseColumnsFromExclusions(exclusions)
			}
			table.addConstraint(conType, columns, constraint.Conname, constraint.Deferrable)

		}
	}

	if table.IsPartitioned {

		partitionElements := createTableNode.CreateStmt.GetPartspec().GetPartParams()
		table.PartitionStrategy = createTableNode.CreateStmt.GetPartspec().GetStrategy()

		for _, partElem := range partitionElements {
			if partElem.GetPartitionElem().GetExpr() != nil {
				table.IsExpressionPartition = true
			} else {
				table.PartitionColumns = append(table.PartitionColumns, partElem.GetPartitionElem().GetName())
			}
		}
	}

	return table, nil
}

func (p *TableParser) parseColumnsFromExclusions(list []*pg_query.Node) []string {
	var res []string
	for _, k := range list {
		res = append(res, k.GetList().GetItems()[0].GetIndexElem().Name) // every first element of items in exclusions will be col name
	}
	return res
}

func parseColumnsFromKeys(keys []*pg_query.Node) []string {
	var res []string
	for _, k := range keys {
		res = append(res, k.GetString_().Sval)
	}
	return res

}

func (p *TableParser) isGeneratedColumn(colDef *pg_query.ColumnDef) bool {
	for _, constraint := range colDef.Constraints {
		if constraint.GetConstraint().Contype == pg_query.ConstrType_CONSTR_GENERATED {
			return true
		}
	}
	return false
}

type ForeignTableParser struct{}

func NewForeignTableParser() *ForeignTableParser {
	return &ForeignTableParser{}
}

func (f *ForeignTableParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	foreignTableNode, ok := getForeignTableStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE FOREIGN TABLE statement")
	}
	baseStmt := foreignTableNode.CreateForeignTableStmt.BaseStmt
	relation := baseStmt.Relation
	table := Table{
		TableName:  relation.GetRelname(),
		SchemaName: relation.GetSchemaname(),
		//Not populating rest info
	}
	for _, element := range baseStmt.TableElts {
		if element.GetColumnDef() != nil {
			colName := element.GetColumnDef().GetColname()

			typeNames := element.GetColumnDef().GetTypeName().GetNames()
			typeName, typeSchemaName := getTypeNameAndSchema(typeNames)
			table.Columns = append(table.Columns, TableColumn{
				ColumnName:  colName,
				TypeName:    typeName,
				TypeSchema:  typeSchemaName,
				IsArrayType: len(element.GetColumnDef().GetTypeName().GetArrayBounds()) > 0,
			})
		}
	}
	return &ForeignTable{
		Table:      table,
		ServerName: foreignTableNode.CreateForeignTableStmt.GetServername(),
	}, nil

}

// IndexParser handles parsing CREATE INDEX statements
type IndexParser struct{}

func NewIndexParser() *IndexParser {
	return &IndexParser{}
}

func (p *IndexParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	indexNode, ok := getCreateIndexStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE INDEX statement")
	}

	index := &Index{
		SchemaName:        indexNode.IndexStmt.Relation.Schemaname,
		IndexName:         indexNode.IndexStmt.Idxname,
		TableName:         indexNode.IndexStmt.Relation.Relname,
		AccessMethod:      indexNode.IndexStmt.AccessMethod,
		NumStorageOptions: len(indexNode.IndexStmt.GetOptions()),
		Params:            p.parseIndexParams(indexNode.IndexStmt.IndexParams),
	}

	return index, nil
}

func (p *IndexParser) parseIndexParams(params []*pg_query.Node) []IndexParam {
	var indexParams []IndexParam
	for _, i := range params {
		ip := IndexParam{
			SortByOrder:  i.GetIndexElem().Ordering,
			ColName:      i.GetIndexElem().GetName(),
			IsExpression: i.GetIndexElem().GetExpr() != nil,
		}
		if ip.IsExpression {
			//For the expression index case to report in case casting to unsupported types #3
			typeNames := i.GetIndexElem().GetExpr().GetTypeCast().GetTypeName().GetNames()
			ip.ExprCastTypeName, ip.ExprCastTypeSchema = getTypeNameAndSchema(typeNames)
		}
		indexParams = append(indexParams, ip)
	}
	return indexParams
}

// AlterTableParser handles parsing ALTER TABLE statements
type AlterTableParser struct{}

func NewAlterTableParser() *AlterTableParser {
	return &AlterTableParser{}
}

func (p *AlterTableParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	alterNode, ok := getAlterStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not an ALTER TABLE statement")
	}

	alter := &AlterTable{
		SchemaName: alterNode.AlterTableStmt.Relation.Schemaname,
		TableName:  alterNode.AlterTableStmt.Relation.Relname,
		AlterType:  alterNode.AlterTableStmt.Cmds[0].GetAlterTableCmd().GetSubtype(),
	}

	// Parse specific alter command
	cmd := alterNode.AlterTableStmt.Cmds[0].GetAlterTableCmd()
	switch alter.AlterType {
	case pg_query.AlterTableType_AT_SetOptions:
		alter.NumSetAttributes = len(cmd.GetDef().GetList().GetItems())
	case pg_query.AlterTableType_AT_AddConstraint:
		alter.NumStorageOptions = len(cmd.GetDef().GetConstraint().GetOptions())
		/*
		   e.g. ALTER TABLE ONLY public.meeting ADD CONSTRAINT no_time_overlap EXCLUDE USING gist (room_id WITH =, time_range WITH &&);
		   cmds:{alter_table_cmd:{subtype:AT_AddConstraint def:{constraint:{contype:CONSTR_EXCLUSION conname:"no_time_overlap" location:41
		   here again same checking the definition of the alter stmt if it has constraint and checking its type
		*/
		constraint := cmd.GetDef().GetConstraint()
		alter.ConstraintType = constraint.Contype
		alter.ConstraintName = constraint.Conname
		alter.IsDeferrable = constraint.Deferrable
		alter.ConstraintColumns = parseColumnsFromKeys(constraint.GetKeys())

	case pg_query.AlterTableType_AT_DisableRule:
		alter.RuleName = cmd.Name
	}

	return alter, nil
}

// PolicyParser handles parsing CREATE POLICY statements
type PolicyParser struct{}

func NewPolicyParser() *PolicyParser {
	return &PolicyParser{}
}

func (p *PolicyParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	policyNode, ok := getPolicyStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE POLICY statement")
	}

	policy := &Policy{
		PolicyName: policyNode.CreatePolicyStmt.GetPolicyName(),
		SchemaName: policyNode.CreatePolicyStmt.GetTable().GetSchemaname(),
		TableName:  policyNode.CreatePolicyStmt.GetTable().GetRelname(),
		RoleNames:  make([]string, 0),
	}
	roles := policyNode.CreatePolicyStmt.GetRoles()
	/*
		e.g. CREATE POLICY P ON tbl1 TO regress_rls_eve, regress_rls_frank USING (true);
		stmt:{create_policy_stmt:{policy_name:"p" table:{relname:"tbl1" inh:true relpersistence:"p" location:20} cmd_name:"all"
		permissive:true roles:{role_spec:{roletype:ROLESPEC_CSTRING rolename:"regress_rls_eve" location:28}} roles:{role_spec:
		{roletype:ROLESPEC_CSTRING rolename:"regress_rls_frank" location:45}} qual:{a_const:{boolval:{boolval:true} location:70}}}}
		stmt_len:75

		here role_spec of each roles is managing the roles related information in a POLICY DDL if any, so we can just check if there is
		a role name available in it which means there is a role associated with this DDL. Hence report it.

	*/
	for _, role := range roles {
		roleName := role.GetRoleSpec().GetRolename() // only in case there is role associated with a policy it will error out in schema migration
		if roleName != "" {
			//this means there is some role or grants used in this Policy, so detecting it
			policy.RoleNames = append(policy.RoleNames, roleName)
		}
	}
	return policy, nil
}

// TriggerParser handles parsing CREATE Trigger statements
type TriggerParser struct{}

func NewTriggerParser() *TriggerParser {
	return &TriggerParser{}
}

/*
e.g.CREATE CONSTRAINT TRIGGER some_trig

	AFTER DELETE ON xyz_schema.abc
	DEFERRABLE INITIALLY DEFERRED
	FOR EACH ROW EXECUTE PROCEDURE xyz_schema.some_trig();

create_trig_stmt:{isconstraint:true trigname:"some_trig" relation:{schemaname:"xyz_schema" relname:"abc" inh:true relpersistence:"p"
location:56} funcname:{string:{sval:"xyz_schema"}} funcname:{string:{sval:"some_trig"}} row:true events:8 deferrable:true initdeferred:true}}
stmt_len:160}

e.g. CREATE TRIGGER projects_loose_fk_trigger

	AFTER DELETE ON public.projects
	REFERENCING OLD TABLE AS old_table
	FOR EACH STATEMENT EXECUTE FUNCTION xyz_schema.some_trig();

stmt:{create_trig_stmt:{trigname:"projects_loose_fk_trigger" relation:{schemaname:"public" relname:"projects" inh:true
relpersistence:"p" location:58} funcname:{string:{sval:"xyz_schema"}} funcname:{string:{sval:"some_trig"}} events:8
transition_rels:{trigger_transition:{name:"old_table" is_table:true}}}} stmt_len:167}
*/
func (t *TriggerParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	triggerNode, ok := getCreateTriggerStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE TRIGGER statement")
	}

	trigger := &Trigger{
		SchemaName:  triggerNode.CreateTrigStmt.Relation.Schemaname,
		TableName:   triggerNode.CreateTrigStmt.Relation.Relname,
		TriggerName: triggerNode.CreateTrigStmt.Trigname,

		IsConstraint:           triggerNode.CreateTrigStmt.Isconstraint,
		NumTransitionRelations: len(triggerNode.CreateTrigStmt.GetTransitionRels()),
		Timing:                 triggerNode.CreateTrigStmt.Timing,
		Events:                 triggerNode.CreateTrigStmt.Events,
		ForEachRow:             triggerNode.CreateTrigStmt.Row,
	}

	return trigger, nil
}

// DDL Objects
type Table struct {
	SchemaName            string
	TableName             string
	IsUnlogged            bool
	IsPartitioned         bool
	Columns               []TableColumn
	IsExpressionPartition bool
	PartitionStrategy     pg_query.PartitionStrategy
	PartitionColumns      []string
	GeneratedColumns      []string
	Constraints           []TableConstraint
}

type TableColumn struct {
	ColumnName  string
	TypeName    string
	TypeSchema  string
	IsArrayType bool
}

func (tc *TableColumn) GetFullTypeName() string {
	return lo.Ternary(tc.TypeSchema != "", tc.TypeSchema+"."+tc.TypeName, tc.TypeName)
}

type TableConstraint struct {
	ConstraintType pg_query.ConstrType
	ConstraintName string
	IsDeferrable   bool
	Columns        []string
}

func (c *TableConstraint) IsPrimaryKeyORUniqueConstraint() bool {
	return c.ConstraintType == PRIMARY_CONSTR_TYPE || c.ConstraintType == UNIQUE_CONSTR_TYPE
}

func (t *TableConstraint) generateConstraintName(tableName string) string {
	suffix := ""
	//Deferrable is only applicable to following constraint
	//https://www.postgresql.org/docs/current/sql-createtable.html#:~:text=Currently%2C%20only%20UNIQUE%2C%20PRIMARY%20KEY%2C%20EXCLUDE%2C%20and%20REFERENCES
	switch t.ConstraintType {
	case pg_query.ConstrType_CONSTR_UNIQUE:
		suffix = "_key"
	case pg_query.ConstrType_CONSTR_PRIMARY:
		suffix = "_pkey"
	case pg_query.ConstrType_CONSTR_EXCLUSION:
		suffix = "_excl"
	case pg_query.ConstrType_CONSTR_FOREIGN:
		suffix = "_fkey"
	}

	return fmt.Sprintf("%s_%s%s", tableName, strings.Join(t.Columns, "_"), suffix)
}

func (t *Table) GetObjectName() string {
	qualifiedTable := lo.Ternary(t.SchemaName != "", fmt.Sprintf("%s.%s", t.SchemaName, t.TableName), t.TableName)
	return qualifiedTable
}
func (t *Table) GetSchemaName() string { return t.SchemaName }

func (t *Table) PrimaryKeyColumns() []string {
	for _, c := range t.Constraints {
		if c.ConstraintType == PRIMARY_CONSTR_TYPE {
			return c.Columns
		}
	}
	return []string{}
}

func (t *Table) UniqueKeyColumns() []string {
	uniqueCols := make([]string, 0)
	for _, c := range t.Constraints {
		if c.ConstraintType == UNIQUE_CONSTR_TYPE {
			uniqueCols = append(uniqueCols, c.Columns...)
		}
	}
	return uniqueCols
}

func (t *Table) addConstraint(conType pg_query.ConstrType, columns []string, specifiedConName string, deferrable bool) {
	tc := TableConstraint{
		ConstraintType: conType,
		Columns:        columns,
		IsDeferrable:   deferrable,
	}
	generatedConName := tc.generateConstraintName(t.GetObjectName())
	conName := lo.Ternary(specifiedConName == "", generatedConName, specifiedConName)
	tc.ConstraintName = conName
	t.Constraints = append(t.Constraints, tc)
}

type ForeignTable struct {
	Table
	ServerName string
}

func (f *ForeignTable) GetObjectName() string {
	return lo.Ternary(f.SchemaName != "", f.SchemaName+"."+f.TableName, f.TableName)
}
func (f *ForeignTable) GetSchemaName() string { return f.SchemaName }

type Index struct {
	SchemaName        string
	IndexName         string
	TableName         string
	AccessMethod      string
	NumStorageOptions int
	Params            []IndexParam
}

type IndexParam struct {
	SortByOrder         pg_query.SortByDir
	ColName             string
	IsExpression        bool
	ExprCastTypeName    string //In case of expression and casting to a type
	ExprCastTypeSchema  string //In case of expression and casting to a type
	IsExprCastArrayType bool
	//Add more fields
}

func (ip *IndexParam) GetFullExprCastTypeName() string {
	return lo.Ternary(ip.ExprCastTypeSchema != "", ip.ExprCastTypeSchema+"."+ip.ExprCastTypeName, ip.ExprCastTypeName)
}

func (i *Index) GetObjectName() string {
	return fmt.Sprintf("%s ON %s", i.IndexName, i.GetTableName())
}
func (i *Index) GetSchemaName() string { return i.SchemaName }

func (i *Index) GetTableName() string {
	return lo.Ternary(i.SchemaName != "", fmt.Sprintf("%s.%s", i.SchemaName, i.TableName), i.TableName)
}

type AlterTable struct {
	Query             string
	SchemaName        string
	TableName         string
	AlterType         pg_query.AlterTableType
	RuleName          string
	NumSetAttributes  int
	NumStorageOptions int
	//In case AlterType - ADD_CONSTRAINT
	ConstraintType    pg_query.ConstrType
	ConstraintName    string
	IsDeferrable      bool
	ConstraintColumns []string
}

func (a *AlterTable) GetObjectName() string {
	qualifiedTable := lo.Ternary(a.SchemaName != "", fmt.Sprintf("%s.%s", a.SchemaName, a.TableName), a.TableName)
	return qualifiedTable
}
func (a *AlterTable) GetSchemaName() string { return a.SchemaName }

func (a *AlterTable) AddPrimaryKeyOrUniqueCons() bool {
	return a.ConstraintType == PRIMARY_CONSTR_TYPE || a.ConstraintType == UNIQUE_CONSTR_TYPE
}

type Policy struct {
	SchemaName string
	TableName  string
	PolicyName string
	RoleNames  []string
}

func (p *Policy) GetObjectName() string {
	qualifiedTable := lo.Ternary(p.SchemaName != "", fmt.Sprintf("%s.%s", p.SchemaName, p.TableName), p.TableName)
	return fmt.Sprintf("%s ON %s", p.PolicyName, qualifiedTable)
}
func (p *Policy) GetSchemaName() string { return p.SchemaName }

type Trigger struct {
	SchemaName             string
	TableName              string
	TriggerName            string
	IsConstraint           bool
	NumTransitionRelations int
	ForEachRow             bool
	Timing                 int32
	Events                 int32
}

func (t *Trigger) GetObjectName() string {
	return fmt.Sprintf("%s ON %s", t.TriggerName, t.GetTableName())
}

func (t *Trigger) GetTableName() string {
	return lo.Ternary(t.SchemaName != "", fmt.Sprintf("%s.%s", t.SchemaName, t.TableName), t.TableName)
}

func (t *Trigger) GetSchemaName() string { return t.SchemaName }

/*
e.g.CREATE TRIGGER after_insert_or_delete_trigger

	BEFORE INSERT OR DELETE ON main_table
	FOR EACH ROW
	EXECUTE FUNCTION handle_insert_or_delete();

stmt:{create_trig_stmt:{trigname:"after_insert_or_delete_trigger" relation:{relname:"main_table" inh:true relpersistence:"p"
location:111} funcname:{string:{sval:"handle_insert_or_delete"}} row:true timing:2 events:12}} stmt_len:177}

here,
timing - bits of BEFORE/AFTER/INSTEAD
events - bits of "OR" INSERT/UPDATE/DELETE/TRUNCATE
row - FOR EACH ROW (true), FOR EACH STATEMENT (false)
refer - https://github.com/pganalyze/pg_query_go/blob/c3a818d346a927c18469460bb18acb397f4f4301/parser/include/postgres/catalog/pg_trigger_d.h#L49

	TRIGGER_TYPE_BEFORE				(1 << 1)
	TRIGGER_TYPE_INSERT				(1 << 2)
	TRIGGER_TYPE_DELETE				(1 << 3)
	TRIGGER_TYPE_UPDATE				(1 << 4)
	TRIGGER_TYPE_TRUNCATE			(1 << 5)
	TRIGGER_TYPE_INSTEAD			(1 << 6)
*/
func (t *Trigger) IsBeforeRowTrigger() bool {
	isSecondBitSet := t.Timing&(1<<1) != 0
	return t.ForEachRow && isSecondBitSet
}

type TypeParser struct{}

func NewTypeParser() *TypeParser {
	return &TypeParser{}
}

func (t *TypeParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	compositeNode, isComposite := getCompositeTypeStmtNode(parseTree)
	enumNode, isEnum := getEnumTypeStmtNode(parseTree)

	switch {
	case isComposite:
		createType := &CreateType{
			TypeName:   compositeNode.CompositeTypeStmt.Typevar.GetRelname(),
			SchemaName: compositeNode.CompositeTypeStmt.Typevar.GetSchemaname(),
		}
		return createType, nil
	case isEnum:
		typeNames := enumNode.CreateEnumStmt.GetTypeName()
		typeName, typeSchemaName := getTypeNameAndSchema(typeNames)
		createType := &CreateType{
			TypeName:   typeName,
			SchemaName: typeSchemaName,
			IsEnum:     true,
		}
		return createType, nil

	default:
		return nil, fmt.Errorf("not CREATE TYPE statement")
	}

}

type CreateType struct {
	TypeName   string
	SchemaName string
	IsEnum     bool
}

func (c *CreateType) GetObjectName() string {
	return lo.Ternary(c.SchemaName != "", fmt.Sprintf("%s.%s", c.SchemaName, c.TypeName), c.TypeName)
}
func (c *CreateType) GetSchemaName() string { return c.SchemaName }

//No op parser for objects we don't have parser yet

type NoOpParser struct{}

func NewNoOpParser() *NoOpParser {
	return &NoOpParser{}
}

type Object struct {
	ObjectName string
	SchemaName string
}

func (o *Object) GetObjectName() string { return o.ObjectName }
func (o *Object) GetSchemaName() string { return o.SchemaName }

func (g *NoOpParser) Parse(parseTree *pg_query.ParseResult) (DDLObject, error) {
	return &Object{}, nil
}

func GetDDLParser(parseTree *pg_query.ParseResult) (DDLParser, error) {
	switch {
	case IsCreateTable(parseTree):
		return NewTableParser(), nil
	case IsCreateIndex(parseTree):
		return NewIndexParser(), nil
	case IsAlterTable(parseTree):
		return NewAlterTableParser(), nil
	case IsCreatePolicy(parseTree):
		return NewPolicyParser(), nil
	case IsCreateTrigger(parseTree):
		return NewTriggerParser(), nil
	case IsCreateType(parseTree):
		return NewTypeParser(), nil
	case IsCreateForeign(parseTree):
		return NewForeignTableParser(), nil
	default:
		return NewNoOpParser(), nil
	}
}

const (
	ADD_CONSTRAINT        = pg_query.AlterTableType_AT_AddConstraint
	SET_OPTIONS           = pg_query.AlterTableType_AT_SetOptions
	DISABLE_RULE          = pg_query.AlterTableType_AT_DisableRule
	CLUSTER_ON            = pg_query.AlterTableType_AT_ClusterOn
	EXCLUSION_CONSTR_TYPE = pg_query.ConstrType_CONSTR_EXCLUSION
	FOREIGN_CONSTR_TYPE   = pg_query.ConstrType_CONSTR_FOREIGN
	DEFAULT_SORTING_ORDER = pg_query.SortByDir_SORTBY_DEFAULT
	PRIMARY_CONSTR_TYPE   = pg_query.ConstrType_CONSTR_PRIMARY
	UNIQUE_CONSTR_TYPE    = pg_query.ConstrType_CONSTR_UNIQUE
	LIST_PARTITION        = pg_query.PartitionStrategy_PARTITION_STRATEGY_LIST
)

var deferrableConstraintsList = []pg_query.ConstrType{
	pg_query.ConstrType_CONSTR_ATTR_DEFERRABLE,
	pg_query.ConstrType_CONSTR_ATTR_DEFERRED,
	pg_query.ConstrType_CONSTR_ATTR_IMMEDIATE,
}
