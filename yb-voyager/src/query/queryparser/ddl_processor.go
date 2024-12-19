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
/*
Whenever adding a new DDL type to prasing for detecting issues, need to extend this DDLProcessor
with the Process() function to adding logic to get the required information out from the parseTree of that DDL
and store it in a DDLObject struct
*/
type DDLProcessor interface {
	Process(*pg_query.ParseResult) (DDLObject, error)
}

// Base DDL object interface
/*
Whenever adding a new DDL type, You need to extend this DDLObject struct to be extended for that object type
with the required for storing the information which should have these required function also extended for the objeect Name and schema name
*/
type DDLObject interface {
	GetObjectName() string
	GetObjectType() string
	GetSchemaName() string
}

//=========== TABLE PROCESSOR ================================

// TableProcessor handles parsing CREATE TABLE statements
type TableProcessor struct{}

func NewTableProcessor() *TableProcessor {
	return &TableProcessor{}
}

/*
e.g. CREATE TABLE "Test"(

		id int,
		room_id int,
		time_range tsrange,
		room_id1 int,
		time_range1 tsrange
		EXCLUDE USING gist (room_id WITH =, time_range WITH &&),
		EXCLUDE USING gist (room_id1 WITH =, time_range1 WITH &&)
	);

create_stmt:{relation:{relname:"Test" inh:true relpersistence:"p" location:14} table_elts:...table_elts:{constraint:{contype:CONSTR_EXCLUSION
location:226 exclusions:{list:{items:{index_elem:{name:"room_id" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
items:{list:{items:{string:{sval:"="}}}}}} exclusions:{list:{items:{index_elem:{name:"time_range" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
items:{list:{items:{string:{sval:"&&"}}}}}} access_method:"gist"}} table_elts:{constraint:{contype:CONSTR_EXCLUSION location:282 exclusions:{list:
{items:{index_elem:{name:"room_id1" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}} items:{list:{items:{string:{sval:"="}}}}}}
exclusions:{list:{items:{index_elem:{name:"time_range1" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}} items:{list:{items:{string:{sval:"&&"}}}}}}
access_method:"gist"}} oncommit:ONCOMMIT_NOOP}} stmt_len:365}

here we are iterating over all the table_elts - table elements and which are comma separated column info in
the DDL so each column has column_def(column definition) in the parse tree but in case it is a constraint, the column_def
is nil.

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
func (tableProcessor *TableProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
	createTableNode, ok := getCreateTableStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE TABLE statement")
	}

	table := &Table{
		SchemaName: createTableNode.CreateStmt.Relation.Schemaname,
		TableName:  createTableNode.CreateStmt.Relation.Relname,
		/*
			e.g CREATE UNLOGGED TABLE tbl_unlogged (id int, val text);
			stmt:{create_stmt:{relation:{schemaname:"public" relname:"tbl_unlogged" inh:true relpersistence:"u" location:19}
		*/
		IsUnlogged:       createTableNode.CreateStmt.Relation.GetRelpersistence() == "u",
		IsPartitioned:    createTableNode.CreateStmt.GetPartspec() != nil,
		IsInherited:      tableProcessor.checkInheritance(createTableNode),
		GeneratedColumns: make([]string, 0),
		Constraints:      make([]TableConstraint, 0),
		PartitionColumns: make([]string, 0),
	}

	// Parse columns and their properties
	for _, element := range createTableNode.CreateStmt.TableElts {
		if element.GetColumnDef() != nil {
			if tableProcessor.isGeneratedColumn(element.GetColumnDef()) {
				table.GeneratedColumns = append(table.GeneratedColumns, element.GetColumnDef().Colname)
			}
			colName := element.GetColumnDef().GetColname()

			typeNames := element.GetColumnDef().GetTypeName().GetNames()
			typeName, typeSchemaName := getTypeNameAndSchema(typeNames)
			/*
				e.g. CREATE TABLE test_xml_type(id int, data xml);
				relation:{relname:"test_xml_type" inh:true relpersistence:"p" location:15} table_elts:{column_def:{colname:"id"
				type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"int4"}} typemod:-1 location:32}
				is_local:true location:29}} table_elts:{column_def:{colname:"data" type_name:{names:{string:{sval:"xml"}}
				typemod:-1 location:42} is_local:true location:37}} oncommit:ONCOMMIT_NOOP}}

				here checking the type of each column as type definition can be a list names for types which are native e.g. int
				it has type names - [pg_catalog, int4] both to determine but for complex types like text,json or xml etc. if doesn't have
				info about pg_catalog. so checking the 0th only in case XML/XID to determine the type and report
			*/
			table.Columns = append(table.Columns, TableColumn{
				ColumnName:  colName,
				TypeName:    typeName,
				TypeSchema:  typeSchemaName,
				IsArrayType: isArrayType(element.GetColumnDef().GetTypeName()),
			})

			constraints := element.GetColumnDef().GetConstraints()
			if constraints != nil {
				for idx, c := range constraints {
					constraint := c.GetConstraint()
					if slices.Contains(deferrableConstraintsList, constraint.Contype) {
						/*
								e.g. create table unique_def_test(id int UNIQUE DEFERRABLE, c1 int);

								create_stmt:{relation:{relname:"unique_def_test"  inh:true  relpersistence:"p"  location:15}
								table_elts:{column_def:{colname:"id"  type_name:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"int4"}}
								typemod:-1  location:34}  is_local:true  constraints:{constraint:{contype:CONSTR_UNIQUE  location:38}}
								constraints:{constraint:{contype:CONSTR_ATTR_DEFERRABLE  location:45}}  location:31}}  ....

							here checking the case where this clause is in column definition so iterating over each column_def and in that
							constraint type has deferrable or not and also it should not be a foreign constraint as Deferrable on FKs are
							supported.
						*/
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
			/*
				e.g. create table uniquen_def_test1(id int, c1 int, UNIQUE(id) DEFERRABLE INITIALLY DEFERRED);
					{create_stmt:{relation:{relname:"unique_def_test1"  inh:true  relpersistence:"p"  location:80}  table_elts:{column_def:{colname:"id"
					type_name:{....  names:{string:{sval:"int4"}}  typemod:-1  location:108}  is_local:true  location:105}}
					table_elts:{constraint:{contype:CONSTR_UNIQUE  deferrable:true  initdeferred:true location:113  keys:{string:{sval:"id"}}}} ..

					here checking the case where this constraint is at the at the end as a constraint only, so checking deferrable field in constraint
					in case of its not a FK.
			*/
			constraint := element.GetConstraint()
			conType := element.GetConstraint().Contype
			columns := parseColumnsFromKeys(constraint.GetKeys())
			if conType == EXCLUSION_CONSTR_TYPE {
				//In case CREATE DDL has EXCLUDE USING gist(room_id '=', time_range WITH &&) - it will be included in columns but won't have columnDef as its a constraint
				exclusions := constraint.GetExclusions()
				//exclusions:{list:{items:{index_elem:{name:"room_id" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
				//items:{list:{items:{string:{sval:"="}}}}}}
				columns = tableProcessor.parseColumnsFromExclusions(exclusions)
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

func (tableProcessor *TableProcessor) checkInheritance(createTableNode *pg_query.Node_CreateStmt) bool {
	/*
		CREATE TABLE Test(id int, name text) inherits(test_parent);
		stmts:{stmt:{create_stmt:{relation:{relname:"test" inh:true relpersistence:"p" location:13} table_elts:{column_def:{colname:"id" ....
		inh_relations:{range_var:{relname:"test_parent" inh:true relpersistence:"p" location:46}} oncommit:ONCOMMIT_NOOP}} stmt_len:58}

		CREATE TABLE accounts_list_partitioned_p_northwest PARTITION OF accounts_list_partitioned FOR VALUES IN ('OR', 'WA');
		version:160001 stmts:{stmt:{create_stmt:{relation:{relname:"accounts_list_partitioned_p_northwest" inh:true relpersistence:"p" location:14}
		inh_relations:{range_var:{relname:"accounts_list_partitioned" inh:true relpersistence:"p" location:65}} partbound:{strategy:"l" listdatums:{a_const:{sval:{sval:"OR"} location:106}}
		listdatums:{a_const:{sval:{sval:"WA"} location:112}} location:102} oncommit:ONCOMMIT_NOOP}}
	*/
	inheritsRel := createTableNode.CreateStmt.GetInhRelations()
	if inheritsRel != nil {
		isPartitionOf := createTableNode.CreateStmt.GetPartbound() != nil
		return !isPartitionOf
	}
	return false
}

func (tableProcessor *TableProcessor) parseColumnsFromExclusions(list []*pg_query.Node) []string {
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

func (tableProcessor *TableProcessor) isGeneratedColumn(colDef *pg_query.ColumnDef) bool {
	for _, constraint := range colDef.Constraints {
		if constraint.GetConstraint().Contype == pg_query.ConstrType_CONSTR_GENERATED {
			return true
		}
	}
	return false
}

type Table struct {
	SchemaName            string
	TableName             string
	IsUnlogged            bool
	IsInherited           bool
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

func (c *TableConstraint) generateConstraintName(tableName string) string {
	suffix := ""
	//Deferrable is only applicable to following constraint
	//https://www.postgresql.org/docs/current/sql-createtable.html#:~:text=Currently%2C%20only%20UNIQUE%2C%20PRIMARY%20KEY%2C%20EXCLUDE%2C%20and%20REFERENCES
	switch c.ConstraintType {
	case pg_query.ConstrType_CONSTR_UNIQUE:
		suffix = "_key"
	case pg_query.ConstrType_CONSTR_PRIMARY:
		suffix = "_pkey"
	case pg_query.ConstrType_CONSTR_EXCLUSION:
		suffix = "_excl"
	case pg_query.ConstrType_CONSTR_FOREIGN:
		suffix = "_fkey"
	}

	return fmt.Sprintf("%s_%s%s", tableName, strings.Join(c.Columns, "_"), suffix)
}

func (t *Table) GetObjectName() string {
	qualifiedTable := lo.Ternary(t.SchemaName != "", fmt.Sprintf("%s.%s", t.SchemaName, t.TableName), t.TableName)
	return qualifiedTable
}
func (t *Table) GetSchemaName() string { return t.SchemaName }

func (t *Table) GetObjectType() string { return TABLE_OBJECT_TYPE }

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
	generatedConName := tc.generateConstraintName(t.TableName)
	conName := lo.Ternary(specifiedConName == "", generatedConName, specifiedConName)
	tc.ConstraintName = conName
	t.Constraints = append(t.Constraints, tc)
}

//===========FOREIGN TABLE PROCESSOR ================================

type ForeignTableProcessor struct{}

func NewForeignTableProcessor() *ForeignTableProcessor {
	return &ForeignTableProcessor{}
}

func (ftProcessor *ForeignTableProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
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
				IsArrayType: isArrayType(element.GetColumnDef().GetTypeName()),
			})
		}
	}
	return &ForeignTable{
		Table:      table,
		ServerName: foreignTableNode.CreateForeignTableStmt.GetServername(),
	}, nil

}

type ForeignTable struct {
	Table
	ServerName string
}

func (f *ForeignTable) GetObjectName() string {
	return lo.Ternary(f.SchemaName != "", f.SchemaName+"."+f.TableName, f.TableName)
}
func (f *ForeignTable) GetSchemaName() string { return f.SchemaName }

func (t *ForeignTable) GetObjectType() string { return FOREIGN_TABLE_OBJECT_TYPE }

//===========INDEX PROCESSOR ================================

// IndexProcessor handles parsing CREATE INDEX statements
type IndexProcessor struct{}

func NewIndexProcessor() *IndexProcessor {
	return &IndexProcessor{}
}

func (indexProcessor *IndexProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
	indexNode, ok := getCreateIndexStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE INDEX statement")
	}

	index := &Index{
		SchemaName:   indexNode.IndexStmt.Relation.Schemaname,
		IndexName:    indexNode.IndexStmt.Idxname,
		TableName:    indexNode.IndexStmt.Relation.Relname,
		AccessMethod: indexNode.IndexStmt.AccessMethod,
		/*
			e.g. CREATE INDEX idx on table_name(id) with (fillfactor='70');
			index_stmt:{idxname:"idx" relation:{relname:"table_name" inh:true relpersistence:"p" location:21} access_method:"btree"
			index_params:{index_elem:{name:"id" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
			options:{def_elem:{defname:"fillfactor" arg:{string:{sval:"70"}} ...
			here again similar to ALTER table Storage parameters options is the high level field in for WITH options.
		*/
		NumStorageOptions: len(indexNode.IndexStmt.GetOptions()),
		Params:            indexProcessor.parseIndexParams(indexNode.IndexStmt.IndexParams),
	}

	return index, nil
}

func (indexProcessor *IndexProcessor) parseIndexParams(params []*pg_query.Node) []IndexParam {
	/*
		e.g.
		1. CREATE INDEX tsvector_idx ON public.documents  (title_tsvector, id);
		stmt:{index_stmt:{idxname:"tsvector_idx"  relation:{schemaname:"public"  relname:"documents"  inh:true  relpersistence:"p"  location:510}  access_method:"btree"
		index_params:{index_elem:{name:"title_tsvector"  ordering:SORTBY_DEFAULT  nulls_ordering:SORTBY_NULLS_DEFAULT}}  index_params:{index_elem:{name:"id"
		ordering:SORTBY_DEFAULT  nulls_ordering:SORTBY_NULLS_DEFAULT}}}}  stmt_location:479  stmt_len:69

		2. CREATE INDEX idx_json ON public.test_json ((data::jsonb));
		stmt:{index_stmt:{idxname:"idx_json"  relation:{schemaname:"public"  relname:"test_json"  inh:true  relpersistence:"p"  location:703}  access_method:"btree"
		index_params:{index_elem:{expr:{type_cast:{arg:{column_ref:{fields:{string:{sval:"data"}}  location:722}}  type_name:{names:{string:{sval:"jsonb"}}  typemod:-1
		location:728}  location:726}}  ordering:SORTBY_DEFAULT  nulls_ordering:SORTBY_NULLS_DEFAULT}}}}  stmt_location:676  stmt_len:59
	*/
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
			ip.IsExprCastArrayType = isArrayType(i.GetIndexElem().GetExpr().GetTypeCast().GetTypeName())
		}
		indexParams = append(indexParams, ip)
	}
	return indexParams
}

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

func (indexParam *IndexParam) GetFullExprCastTypeName() string {
	return lo.Ternary(indexParam.ExprCastTypeSchema != "", indexParam.ExprCastTypeSchema+"."+indexParam.ExprCastTypeName, indexParam.ExprCastTypeName)
}

func (i *Index) GetObjectName() string {
	return fmt.Sprintf("%s ON %s", i.IndexName, i.GetTableName())
}
func (i *Index) GetSchemaName() string { return i.SchemaName }

func (i *Index) GetTableName() string {
	return lo.Ternary(i.SchemaName != "", fmt.Sprintf("%s.%s", i.SchemaName, i.TableName), i.TableName)
}

func (i *Index) GetObjectType() string { return INDEX_OBJECT_TYPE }

//===========ALTER TABLE PROCESSOR ================================

// AlterTableProcessor handles parsing ALTER TABLE statements
type AlterTableProcessor struct{}

func NewAlterTableProcessor() *AlterTableProcessor {
	return &AlterTableProcessor{}
}

func (atProcessor *AlterTableProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
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
		/*
			e.g. alter table test_1 alter column col1 set (attribute_option=value);
			cmds:{alter_table_cmd:{subtype:AT_SetOptions name:"col1" def:{list:{items:{def_elem:{defname:"attribute_option"
			arg:{type_name:{names:{string:{sval:"value"}} typemod:-1 location:263}} defaction:DEFELEM_UNSPEC location:246}}}}...

			for set attribute issue we will the type of alter setting the options and in the 'def' definition field which has the
			information of the type, we will check if there is any list which will only present in case there is syntax like <SubTYPE> (...)
		*/
		alter.NumSetAttributes = len(cmd.GetDef().GetList().GetItems())
	case pg_query.AlterTableType_AT_AddConstraint:
		alter.NumStorageOptions = len(cmd.GetDef().GetConstraint().GetOptions())
		/*
			e.g.
				ALTER TABLE example2
					ADD CONSTRAINT example2_pkey PRIMARY KEY (id);
				tmts:{stmt:{alter_table_stmt:{relation:{relname:"example2"  inh:true  relpersistence:"p"  location:693}
				cmds:{alter_table_cmd:{subtype:AT_AddConstraint  def:{constraint:{contype:CONSTR_PRIMARY  conname:"example2_pkey"
				location:710  keys:{string:{sval:"id"}}}}  behavior:DROP_RESTRICT}}  objtype:OBJECT_TABLE}}  stmt_location:679  stmt_len:72}

			e.g. ALTER TABLE ONLY public.meeting ADD CONSTRAINT no_time_overlap EXCLUDE USING gist (room_id WITH =, time_range WITH &&);
			cmds:{alter_table_cmd:{subtype:AT_AddConstraint def:{constraint:{contype:CONSTR_EXCLUSION conname:"no_time_overlap" location:41
			here again same checking the definition of the alter stmt if it has constraint and checking its type

			e.g. ALTER TABLE ONLY public.users ADD CONSTRAINT users_email_key UNIQUE (email) DEFERRABLE;
			alter_table_cmd:{subtype:AT_AddConstraint  def:{constraint:{contype:CONSTR_UNIQUE  conname:"users_email_key"
			deferrable:true  location:196  keys:{string:{sval:"email"}}}}  behavior:DROP_RESTRICT}}  objtype:OBJECT_TABLE}}

			similar to CREATE table 2nd case where constraint is at the end of column definitions mentioning the constraint only
			so here as well while adding constraint checking the type of constraint and the deferrable field of it.

			ALTER TABLE test ADD CONSTRAINT chk check (id<>'') NOT VALID;
			stmts:{stmt:...subtype:AT_AddConstraint def:{constraint:{contype:CONSTR_CHECK conname:"chk" location:22
			raw_expr:{a_expr:{kind:AEXPR_OP name:{string:{sval:"<>"}} lexpr:{column_ref:{fields:{string:{sval:"id"}} location:43}} rexpr:{a_const:{sval:{}
			location:47}} location:45}} skip_validation:true}} behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}} stmt_len:60}
		*/
		constraint := cmd.GetDef().GetConstraint()
		alter.ConstraintType = constraint.Contype
		alter.ConstraintName = constraint.Conname
		alter.IsDeferrable = constraint.Deferrable
		alter.ConstraintNotValid = constraint.SkipValidation // this is set for the NOT VALID clause
		alter.ConstraintColumns = parseColumnsFromKeys(constraint.GetKeys())

	case pg_query.AlterTableType_AT_DisableRule:
		/*
			e.g. ALTER TABLE example DISABLE example_rule;
			cmds:{alter_table_cmd:{subtype:AT_DisableRule name:"example_rule" behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}}
			checking the subType is sufficient in this case
		*/
		alter.RuleName = cmd.Name
		//case CLUSTER ON
		/*
			e.g. ALTER TABLE example CLUSTER ON idx;
			stmt:{alter_table_stmt:{relation:{relname:"example" inh:true relpersistence:"p" location:13}
			cmds:{alter_table_cmd:{subtype:AT_ClusterOn name:"idx" behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}} stmt_len:32

		*/
	}

	return alter, nil
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
	ConstraintType     pg_query.ConstrType
	ConstraintName     string
	ConstraintNotValid bool
	IsDeferrable       bool
	ConstraintColumns  []string
}

func (a *AlterTable) GetObjectName() string {
	qualifiedTable := lo.Ternary(a.SchemaName != "", fmt.Sprintf("%s.%s", a.SchemaName, a.TableName), a.TableName)
	return qualifiedTable
}
func (a *AlterTable) GetSchemaName() string { return a.SchemaName }

func (a *AlterTable) GetObjectType() string { return TABLE_OBJECT_TYPE }

func (a *AlterTable) AddPrimaryKeyOrUniqueCons() bool {
	return a.ConstraintType == PRIMARY_CONSTR_TYPE || a.ConstraintType == UNIQUE_CONSTR_TYPE
}

func (a *AlterTable) IsAddConstraintType() bool {
	return a.AlterType == pg_query.AlterTableType_AT_AddConstraint
}

//===========POLICY PROCESSOR ================================

// PolicyProcessor handles parsing CREATE POLICY statements
type PolicyProcessor struct{}

func NewPolicyProcessor() *PolicyProcessor {
	return &PolicyProcessor{}
}

func (policyProcessor *PolicyProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
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

func (p *Policy) GetObjectType() string { return POLICY_OBJECT_TYPE }

//=====================TRIGGER PROCESSOR ==================

// TriggerProcessor handles parsing CREATE Trigger statements
type TriggerProcessor struct{}

func NewTriggerProcessor() *TriggerProcessor {
	return &TriggerProcessor{}
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
func (triggerProcessor *TriggerProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
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
	_, trigger.FuncName = getFunctionObjectName(triggerNode.CreateTrigStmt.Funcname)

	return trigger, nil
}

type Trigger struct {
	SchemaName             string
	TableName              string
	TriggerName            string
	IsConstraint           bool
	NumTransitionRelations int
	ForEachRow             bool
	Timing                 int32
	Events                 int32
	FuncName               string //Unqualified function name
}

func (t *Trigger) GetObjectName() string {
	return fmt.Sprintf("%s ON %s", t.TriggerName, t.GetTableName())
}

func (t *Trigger) GetTableName() string {
	return lo.Ternary(t.SchemaName != "", fmt.Sprintf("%s.%s", t.SchemaName, t.TableName), t.TableName)
}

func (t *Trigger) GetSchemaName() string { return t.SchemaName }

func (t *Trigger) GetObjectType() string { return TRIGGER_OBJECT_TYPE }

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

// ========================TYPE PROCESSOR======================

type TypeProcessor struct{}

func NewTypeProcessor() *TypeProcessor {
	return &TypeProcessor{}
}

func (typeProcessor *TypeProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
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

func (c *CreateType) GetObjectType() string { return TYPE_OBJECT_TYPE }

//===========================VIEW PROCESSOR===================

type ViewProcessor struct{}

func NewViewProcessor() *ViewProcessor {
	return &ViewProcessor{}
}

func (v *ViewProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
	viewNode, ok := getCreateViewNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE VIEW statement")
	}
	view := View{
		SchemaName: viewNode.ViewStmt.View.Schemaname,
		ViewName:   viewNode.ViewStmt.View.Relname,
	}
	return &view, nil
}

type View struct {
	SchemaName string
	ViewName   string
}

func (v *View) GetObjectName() string {
	return lo.Ternary(v.SchemaName != "", fmt.Sprintf("%s.%s", v.SchemaName, v.ViewName), v.ViewName)
}
func (v *View) GetSchemaName() string { return v.SchemaName }

func (v *View) GetObjectType() string { return VIEW_OBJECT_TYPE }

//===========================MVIEW PROCESSOR===================

type MViewProcessor struct{}

func NewMViewProcessor() *MViewProcessor {
	return &MViewProcessor{}
}

func (mv *MViewProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
	mviewNode, ok := getCreateTableAsStmtNode(parseTree)
	if !ok {
		return nil, fmt.Errorf("not a CREATE VIEW statement")
	}
	mview := MView{
		SchemaName: mviewNode.CreateTableAsStmt.Into.Rel.Schemaname,
		ViewName:   mviewNode.CreateTableAsStmt.Into.Rel.Relname,
	}
	return &mview, nil
}

type MView struct {
	SchemaName string
	ViewName   string
}

func (mv *MView) GetObjectName() string {
	return lo.Ternary(mv.SchemaName != "", fmt.Sprintf("%s.%s", mv.SchemaName, mv.ViewName), mv.ViewName)
}
func (mv *MView) GetSchemaName() string { return mv.SchemaName }

func (mv *MView) GetObjectType() string { return MVIEW_OBJECT_TYPE }

//=============================No-Op PROCESSOR ==================

//No op Processor for objects we don't have Processor yet

type NoOpProcessor struct{}

func NewNoOpProcessor() *NoOpProcessor {
	return &NoOpProcessor{}
}

type Object struct {
	ObjectName string
	SchemaName string
}

func (o *Object) GetObjectName() string { return o.ObjectName }
func (o *Object) GetSchemaName() string { return o.SchemaName }
func (o *Object) GetObjectType() string { return "OBJECT" }

func (n *NoOpProcessor) Process(parseTree *pg_query.ParseResult) (DDLObject, error) {
	return &Object{}, nil
}

func GetDDLProcessor(parseTree *pg_query.ParseResult) (DDLProcessor, error) {
	stmtType := GetStatementType(parseTree.Stmts[0].Stmt.ProtoReflect())
	switch stmtType {
	case PG_QUERY_CREATE_STMT:
		return NewTableProcessor(), nil
	case PG_QUERY_INDEX_STMT:
		return NewIndexProcessor(), nil
	case PG_QUERY_ALTER_TABLE_STMT:
		return NewAlterTableProcessor(), nil
	case PG_QUERY_POLICY_STMT:
		return NewPolicyProcessor(), nil
	case PG_QUERY_CREATE_TRIG_STMT:
		return NewTriggerProcessor(), nil
	case PG_QUERY_COMPOSITE_TYPE_STMT, PG_QUERY_ENUM_TYPE_STMT:
		return NewTypeProcessor(), nil
	case PG_QUERY_FOREIGN_TABLE_STMT:
		return NewForeignTableProcessor(), nil
	case PG_QUERY_VIEW_STMT:
		return NewViewProcessor(), nil
	case PG_QUERY_CREATE_TABLE_AS_STMT:
		if IsMviewObject(parseTree) {
			return NewMViewProcessor(), nil
		} else {
			return NewNoOpProcessor(), nil
		}
	default:
		return NewNoOpProcessor(), nil
	}
}

const (
	TABLE_OBJECT_TYPE             = "TABLE"
	TYPE_OBJECT_TYPE              = "TYPE"
	VIEW_OBJECT_TYPE              = "VIEW"
	MVIEW_OBJECT_TYPE             = "MVIEW"
	FOREIGN_TABLE_OBJECT_TYPE     = "FOREIGN TABLE"
	FUNCTION_OBJECT_TYPE          = "FUNCTION"
	PROCEDURE_OBJECT_TYPE         = "PROCEDURE"
	INDEX_OBJECT_TYPE             = "INDEX"
	POLICY_OBJECT_TYPE            = "POLICY"
	TRIGGER_OBJECT_TYPE           = "TRIGGER"
	ADD_CONSTRAINT                = pg_query.AlterTableType_AT_AddConstraint
	SET_OPTIONS                   = pg_query.AlterTableType_AT_SetOptions
	DISABLE_RULE                  = pg_query.AlterTableType_AT_DisableRule
	CLUSTER_ON                    = pg_query.AlterTableType_AT_ClusterOn
	EXCLUSION_CONSTR_TYPE         = pg_query.ConstrType_CONSTR_EXCLUSION
	FOREIGN_CONSTR_TYPE           = pg_query.ConstrType_CONSTR_FOREIGN
	DEFAULT_SORTING_ORDER         = pg_query.SortByDir_SORTBY_DEFAULT
	PRIMARY_CONSTR_TYPE           = pg_query.ConstrType_CONSTR_PRIMARY
	UNIQUE_CONSTR_TYPE            = pg_query.ConstrType_CONSTR_UNIQUE
	LIST_PARTITION                = pg_query.PartitionStrategy_PARTITION_STRATEGY_LIST
	PG_QUERY_CREATE_STMT          = "pg_query.CreateStmt"
	PG_QUERY_INDEX_STMT           = "pg_query.IndexStmt"
	PG_QUERY_ALTER_TABLE_STMT     = "pg_query.AlterTableStmt"
	PG_QUERY_POLICY_STMT          = "pg_query.CreatePolicyStmt"
	PG_QUERY_CREATE_TRIG_STMT     = "pg_query.CreateTrigStmt"
	PG_QUERY_COMPOSITE_TYPE_STMT  = "pg_query.CompositeTypeStmt"
	PG_QUERY_ENUM_TYPE_STMT       = "pg_query.CreateEnumStmt"
	PG_QUERY_FOREIGN_TABLE_STMT   = "pg_query.CreateForeignTableStmt"
	PG_QUERY_VIEW_STMT            = "pg_query.ViewStmt"
	PG_QUERY_CREATE_TABLE_AS_STMT = "pg_query.CreateTableAsStmt"
)

var deferrableConstraintsList = []pg_query.ConstrType{
	pg_query.ConstrType_CONSTR_ATTR_DEFERRABLE,
	pg_query.ConstrType_CONSTR_ATTR_DEFERRED,
	pg_query.ConstrType_CONSTR_ATTR_IMMEDIATE,
}
