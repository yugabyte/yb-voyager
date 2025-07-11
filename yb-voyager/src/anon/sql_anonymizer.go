package anon

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SqlAnonymizer struct {
	registry IdentifierHasher
}

func NewSqlAnonymizer(registry IdentifierHasher) Anonymizer {
	return &SqlAnonymizer{
		registry: registry,
	}
}

func (a *SqlAnonymizer) Anonymize(inputSql string) (string, error) {
	parseResult, err := queryparser.Parse(inputSql) // Parse the input SQL to ensure it's valid
	if err != nil {
		return "", fmt.Errorf("error parsing input SQL: %w", err)
	}

	visited := make(map[protoreflect.Message]bool)
	parseTreeMsg := queryparser.GetProtoMessageFromParseTree(parseResult)
	err = queryparser.TraverseParseTree(parseTreeMsg, visited, a.anonymizationProcessor)
	if err != nil {
		return "", fmt.Errorf("error traversing parse tree: %w", err)
	}

	anonymizedSql, err := queryparser.DeparseParseTree(parseResult)
	if err != nil {
		return "", fmt.Errorf("error deparsing parse tree: %w", err)
	}

	return anonymizedSql, nil
}

func (a *SqlAnonymizer) anonymizationProcessor(msg protoreflect.Message) (err error) {
	// three categories of nodes: Identifier nodes, literal nodes, Misc nodes for rest
	err = a.identifierNodesProcessor(msg)
	if err != nil {
		return fmt.Errorf("error processing identifier nodes: %w", err)
	}

	err = a.literalNodesProcessor(msg)
	if err != nil {
		return fmt.Errorf("error processing literal nodes: %w", err)
	}

	err = a.miscellaneousNodesProcessor(msg)
	if err != nil {
		return fmt.Errorf("error processing miscellaneous nodes: %w", err)
	}
	return nil
}

func (a *SqlAnonymizer) identifierNodesProcessor(msg protoreflect.Message) error {
	var err error
	switch queryparser.GetMsgFullName(msg) {
	/*
		RangeVar node is for tablename in FROM clause of a query
			SQL:		SELECT * FROM sales.orders;
			ParseTree:
	*/
	case queryparser.PG_QUERY_RANGEVAR_NODE:
		rv, ok := queryparser.ProtoAsRangeVarNode(msg)
		if !ok {
			return fmt.Errorf("expected RangeVar, got %T", msg.Interface())
		}

		if rv.Catalogname != "" {
			rv.Catalogname, err = a.registry.GetHash(DATABASE_KIND_PREFIX, rv.Catalogname)
			if err != nil {
				return fmt.Errorf("anon catalog: %w", err)
			}
		}

		if rv.Schemaname != "" {
			rv.Schemaname, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, rv.Schemaname)
			if err != nil {
				return fmt.Errorf("anon schema: %w", err)
			}
		}
		rv.Relname, err = a.registry.GetHash(TABLE_KIND_PREFIX, rv.Relname)
		if err != nil {
			return fmt.Errorf("anon table: %w", err)
		}

	/*
		Column Definition node in CREATE TABLE statements
			SQL:		CREATE TABLE foo(column1 INT);
			ParseTree:	{create_stmt:{relation:{relname:"foo" ...}  table_elts:{column_def:{colname:"column1"  type_name:{names:{string:{sval:"pg_catalog"}}
							names:{string:{sval:"int4"}}  ...}  }}  }}
	*/
	case queryparser.PG_QUERY_COLUMNDEF_NODE:
		cd, ok := queryparser.ProtoAsColumnDef(msg)
		if !ok {
			return fmt.Errorf("expected ColumnDef, got %T", msg.Interface())
		}
		cd.Colname, err = a.registry.GetHash(COLUMN_KIND_PREFIX, cd.Colname)
		if err != nil {
			return fmt.Errorf("anon coldef: %w", err)
		}

	/*
		ColumnRef node is for column names in SELECT, WHERE, etc.
	*/
	case queryparser.PG_QUERY_COLUMNREF_NODE:
		err = a.columnRefNodeProcessor(msg)
		if err != nil {
			return fmt.Errorf("error processing ColumnRef node: %w", err)
		}

	/*
		This node generally stores the column name or alias for column name
		SQL:		SELECT column1 FROM users;
		SQL: 		SELECT column1 AS alias1 FROM users;

		ParseTree:	stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"column1"}} }} }}
					from_clause:{range_var:{relname:"users"  ...}} ...}}

		ParseTree:	stmt:{select_stmt:{target_list:{res_target:{name:"alias1"  val:{column_ref:{fields:{string:{sval:"column1"}} }}  }}
					from_clause:{range_var:{relname:"users" ...}}  }}

	*/
	case queryparser.PG_QUERY_RESTARGET_NODE:
		rt, ok := queryparser.ProtoAsResTargetNode(msg)
		if !ok {
			return fmt.Errorf("expected ResTarget, got %T", msg.Interface())
		}
		if rt.Name != "" {
			rt.Name, err = a.registry.GetHash(COLUMN_KIND_PREFIX, rt.Name)
			if err != nil {
				return fmt.Errorf("anon alias: %w", err)
			}
		}

	/*
		IndexStmtNode check is for anonymizing the index name
		IndexElemNode check is for anonymizing the column names in index definition

		SQL:		CREATE INDEX idx_emp_name_date ON hr.employee(last_name, first_name, hire_date);
		ParseTree:	stmt:{index_stmt:{idxname:"idx_emp_name_date" relation:{schemaname:"hr" relname:"employee" inh:true relpersistence:"p" location:34} access_method:"btree"
					index_params:{index_elem:{name:"last_name" ...}} index_params:{index_elem:{name:"first_name" ...}} index_params:{index_elem:{name:"hire_date" ...}}}}
	*/
	case queryparser.PG_QUERY_INDEX_STMT_NODE:
		idx, ok := queryparser.ProtoAsIndexStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected IndexStmt, got %T", msg.Interface())
		}

		if idx.Idxname != "" {
			idx.Idxname, err = a.registry.GetHash(INDEX_KIND_PREFIX, idx.Idxname)
			if err != nil {
				return fmt.Errorf("anon idxname: %w", err)
			}
		}

	case queryparser.PG_QUERY_INDEXELEM_NODE:
		ie, ok := queryparser.ProtoAsIndexElemNode(msg)
		if !ok {
			return fmt.Errorf("expected IndexElem, got %T", msg.Interface())
		}

		/*
			SQL:		CREATE INDEX idx_emp_name_date ON hr.employee(last_name, first_name, hire_date);
			ParseTree:	{index_stmt:{idxname:"idx_emp_name_date" relation:{schemaname:"hr" relname:"employee" inh:true relpersistence:"p" location:34} access_method:"btree"
						index_params:{index_elem:{name:"last_name" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
						index_params:{index_elem:{name:"first_name" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}
						index_params:{index_elem:{name:"hire_date" ordering:SORTBY_DEFAULT nulls_ordering:SORTBY_NULLS_DEFAULT}}}} stmt_len:79
		*/
		// 1) Plain column index: (Name != "")
		//    CREATE INDEX … ON tbl(col1);
		if ie.Name != "" {
			ie.Name, err = a.registry.GetHash(COLUMN_KIND_PREFIX, ie.Name)
			if err != nil {
				return fmt.Errorf("anon index column name: %w", err)
			}
		}

		/*
			SQL:		CREATE INDEX idx_emp_name_date ON hr.employee((last_name + first_name));
			ParseTree:	stmt:{index_stmt:{idxname:"idx_emp_name_date" relation:{schemaname:"hr" relname:"employee" inh:true relpersistence:"p" location:34} ...
						index_params:{index_elem:{expr:{a_expr:{kind:AEXPR_OP name:{string:{sval:"+"}} lexpr:{column_ref:{fields:{string:{sval:"last_name"}} location:47}}
						rexpr:{column_ref:{fields:{string:{sval:"first_name"}} ...}} ...}} ...}}}}
		*/
		// 2) Expression index: (Expr != nil, Name == "")
		//    CREATE INDEX … ON tbl((col1+col2));
		// this case will be handled already by ColumnRefNode processor

		// 3) Expression alias: (Indexcolname != "")
		//    CREATE INDEX … ON tbl((col1+col2) AS sum_col);
		//	 above this sql syntax is invalid, couldn't find an example of Indexcolname but still keeping the anonymization logic here
		if ie.Indexcolname != "" {
			ie.Indexcolname, err = a.registry.GetHash(ALIAS_KIND_PREFIX, ie.Indexcolname)
			if err != nil {
				return fmt.Errorf("anon index column alias: %w", err)
			}
		}

	/*
		SQL:		ALTER TABLE ONLY public.foo ADD CONSTRAINT unique_1 UNIQUE (column1, column2) DEFERRABLE;
		ParseTree:	stmt:{alter_table_stmt:{relation:{schemaname:"public" relname:"foo" relpersistence:"p" } cmds:{alter_table_cmd:{subtype:AT_AddConstraint
					def:{constraint:{contype:CONSTR_UNIQUE conname:"unique_1" deferrable:true location:32 keys:{string:{sval:"column1"}}
					keys:{string:{sval:"column2"}}}} behavior:DROP_RESTRICT}} ...}}
	*/
	case queryparser.PG_QUERY_CONSTRAINT_NODE:
		cons, ok := queryparser.ProtoAsTableConstraintNode(msg)
		if !ok {
			return fmt.Errorf("expected TableConstraint, got %T", msg.Interface())
		}

		if cons.Conname != "" {
			cons.Conname, err = a.registry.GetHash(CONSTRAINT_KIND_PREFIX, cons.Conname)
			if err != nil {
				return fmt.Errorf("anon constraint: %w", err)
			}
		}

		if len(cons.Keys) > 0 {
			// For each key in the constraint, anonymize the column names
			for i, key := range cons.Keys {
				if key.GetString_() == nil {
					continue
				}
				colName := key.GetString_().Sval
				if colName == "" {
					continue // skip empty names
				}
				key.GetString_().Sval, err = a.registry.GetHash(COLUMN_KIND_PREFIX, colName)
				if err != nil {
					return fmt.Errorf("anon constraint key[%d]=%q: %w", i, colName, err)
				}
			}
		}

	/*
		ALTER TABLE humanresources.department CLUSTER ON \"PK_Department_DepartmentID\";
		stmt: {alter_table_stmt:{relation:{schemaname:"humanresources" relname:"department" ...}
			cmds:{alter_table_cmd:{subtype:AT_ClusterOn name:"PK_Department_DepartmentID" behavior:...}} objtype:OBJECT_TABLE}}
	*/
	case queryparser.PG_QUERY_ALTER_TABLE_STMT:
		ats, ok := queryparser.ProtoAsAlterTableStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterTableStmt, got %T", msg.Interface())
		}

		for _, cmd := range ats.Cmds {
			alterTableCmdNode := cmd.GetAlterTableCmd()
			if alterTableCmdNode == nil {
				continue // skip if not an AlterTableCmd
			}

			if alterTableCmdNode.GetSubtype() == pg_query.AlterTableType_AT_ClusterOn {
				// AT_ClusterOn has a name field that needs anonymization
				name := alterTableCmdNode.GetName()
				if name != "" {
					alterTableCmdNode.Name, err = a.registry.GetHash(CONSTRAINT_KIND_PREFIX, name)
					if err != nil {
						return fmt.Errorf("anon alter table cluster on index: %w", err)
					}
				}
			}
		}

	case queryparser.PG_QUERY_ALIAS_NODE:
		alias, ok := queryparser.ProtoAsAliasNode(msg)
		if !ok {
			return fmt.Errorf("expected Alias, got %T", msg.Interface())
		}
		if alias.Aliasname != "" {
			alias.Aliasname, err = a.registry.GetHash(ALIAS_KIND_PREFIX, alias.Aliasname)
			if err != nil {
				return fmt.Errorf("anon aliasnode: %w", err)
			}
		}

	/*
		SQL:		CREATE TABLE foo(id my_custom_type);
		ParseTree:	stmt:{create_stmt:{relation:{relname:"foo"  inh:true  ...}
					table_elts:{column_def:{colname:"id"  type_name:{names:{string:{sval:"my_custom_type"}}  ....}
					....}}  oncommit:ONCOMMIT_NOOP}}
	*/
	case queryparser.PG_QUERY_TYPENAME_NODE:
		tn, ok := queryparser.ProtoAsTypeNameNode(msg)
		if !ok {
			return fmt.Errorf("expected TypeName, got %T", msg.Interface())
		}

		// get string node sval from TypeName
		for i, node := range tn.Names {
			str := node.GetString_()
			if str == nil || str.Sval == "" {
				continue
			}
			str.Sval, err = a.registry.GetHash(TYPE_KIND_PREFIX, str.Sval)
			if err != nil {
				return fmt.Errorf("anon typename[%d]=%q lookup: %w", i, str.Sval, err)
			}
		}

	/*
		SQL: 		GRANT SELECT ON foo TO reporting_user;
		ParseTree:	stmt:{grant_stmt:{is_grant:true ... objects:{range_var:{relname:"foo"  ...}}
					privileges:{access_priv:{priv_name:"select"}}  grantees:{role_spec:{roletype:ROLESPEC_CSTRING  rolename:"reporting_user" ...}}  ...}}
	*/
	case queryparser.PG_QUERY_ROLESPEC_NODE:
		rs, ok := queryparser.ProtoAsRoleSpecNode(msg)
		if !ok {
			return fmt.Errorf("expected RoleSpec, got %T", msg.Interface())
		}

		rs.Rolename, err = a.registry.GetHash(ROLE_KIND_PREFIX, rs.Rolename)
		if err != nil {
			return fmt.Errorf("anon rolespec: %w", err)
		}
	}
	return nil
}

func (a *SqlAnonymizer) literalNodesProcessor(msg protoreflect.Message) error {
	switch queryparser.GetMsgFullName(msg) {

	/*
		SQL:		INSERT INTO foo VALUES ('superSecret');
		ParseTree:	{insert_stmt:{relation:{relname:"foo"  inh:true  relpersistence:"p"  ...}
		select_stmt:{select_stmt:{values_lists:{list:{items:{a_const:{sval:{sval:"superSecret"}  ...}}}}  ...}}  ...}}
	*/
	case queryparser.PG_QUERY_ACONST_NODE:
		ac, ok := queryparser.ProtoAsAConstNode(msg)
		if !ok {
			return fmt.Errorf("expected A_Const, got %T", msg.Interface())
		}

		if ac.Val != nil && ac.GetSval() != nil && ac.GetSval().Sval != "" {
			// Anonymize the string literal
			tok, err := a.registry.GetHash(CONST_KIND_PREFIX, ac.GetSval().Sval)
			if err != nil {
				return fmt.Errorf("anon A_Const: %w", err)
			}
			ac.GetSval().Sval = tok
		}

	}

	return nil
}

func (a *SqlAnonymizer) miscellaneousNodesProcessor(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {

	/*
		SQL: 		CREATE EXTENSION IF NOT EXISTS postgis SCHEMA public;
		ParseTree: 	stmt:{create_extension_stmt:{extname:"postgis"  if_not_exists:true
					options:{def_elem:{defname:"schema"  arg:{string:{sval:"public"}}  defaction:DEFELEM_UNSPEC ...}}}}

		SQL: 		CREATE FOREIGN TABLE f_t(i serial, ts timestamptz(0) default now(), j json, t text, e myenum, c mycomposit) server p10 options (TABLE_name 't');
		ParseTree: 	stmt:{create_foreign_table_stmt:{base_stmt:{relation:{relname:"f_t" ...} table_elts:{column_def:{colname:"i" type_name:{names:{string:{sval:"serial"}} ...} ...}}
					table_elts:{column_def:{colname:"ts" type_name:{names:{string:{sval:"timestamptz"}} ...}} typemod:-1 location:38}
					constraints:{constraint:{contype:CONSTR_DEFAULT location:53 raw_expr:{func_call:{funcname:{string:{sval:"now"}} ....}}}} location:35}}
					table_elts:{column_def:{colname:"j" type_name:{names:{string:{sval:"json"}} typemod:-1 location:70} is_local:true location:68}}
					table_elts:{column_def:{colname:"t" type_name:{names:{string:{sval:"text"}} typemod:-1 location:78} is_local:true location:76}}
					table_elts:{column_def:{colname:"e" type_name:{names:{string:{sval:"myenum"}} typemod:-1 location:86} is_local:true location:84}}
					table_elts:{column_def:{colname:"c" type_name:{names:{string:{sval:"mycomposit"}} ....}} oncommit:ONCOMMIT_NOOP}
					servername:"p10" options:{def_elem:{defname:"table_name" arg:{string:{sval:"t"}} defaction:DEFELEM_UNSPEC location:128}}}} stmt_len:143

		Note: FOREIGN TABLE name part will be anonymized in the identifierNodesProcessor, here handling the remote table name


	*/
	case queryparser.PG_QUERY_DEFELEM_NODE:
		// DEFELEM node used in CREATE EXTENSION, CREATE FOREIGN TABLE have table_name and schema
		defElem, ok := queryparser.ProtoAsDefElemNode(msg)
		if !ok {
			return fmt.Errorf("expected DefElem, got %T", msg.Interface())
		}

		defName := defElem.GetDefname()
		if defName == "table_name" || defName == "schema" {
			kind := lo.Ternary(defName == "table_name", TABLE_KIND_PREFIX, SCHEMA_KIND_PREFIX)
			if defElem.Arg != nil && defElem.Arg.GetString_() != nil {
				defElem.Arg.GetString_().Sval, err = a.registry.GetHash(kind, defElem.Arg.GetString_().Sval)
				if err != nil {
					return fmt.Errorf("anon DefElem %q: %w", defName, err)
				}
			}
		}
	}

	return nil
}

/*
SQL:		SELECT sales.orders.column1 FROM sales.orders WHERE column2 = 'value';
ParseTree:	stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:

		{fields:{string:{sval:"sales"}}
		fields:{string:{sval:"orders"}}
		fields:{string:{sval:"column1"}} }} }}
	from_clause:{range_var:{schemaname:"sales" relname:"orders" ...}}
	where_clause:{a_expr:{kind:AEXPR_OP name:{string:{sval:"="}} lexpr:{column_ref:{fields:{string:{sval:"column2"}} }} rexpr:{a_const:{sval:{sval:"value"} }} }} ...}}
*/
func (a *SqlAnonymizer) columnRefNodeProcessor(msg protoreflect.Message) (err error) {
	/*
		ColumnRef node is for column names in SELECT, WHERE, etc.
		SQL:		SELECT column1 FROM sales.orders WHERE column2 = 'value';

	*/
	cr, ok := queryparser.ProtoAsColumnRef(msg)
	if !ok {
		return fmt.Errorf("expected ColumnRef, got %T", msg.Interface())
	}

	log.Infof("processing ColumnRef node: %s\n", cr.String())
	log.Infof("total number of fields in ColumnRef node: %d\n", len(cr.Fields))

	// Fully qualified column name syntax: DB.Schema.Table.Column
	// so we need to anonymize each part of the column reference node as per its kind

	/*
		Note: need to operate on string fields only in ColumnRef
		SQL: 		SELECT * from sales.orders;
		ParseTree:	{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
					from_clause:{range_var:{schemaname:"sales"  relname:"orders" }}  }}

		Other example:	SELECT *, mydb.sales.users.password FROM mydb.sales.users;
		ParseTree:		stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
						target_list:{res_target:{val:{ column_ref:
						{fields:{string:{sval:"mydb"}}  fields:{string:{sval:"sales"}}  fields:{string:{sval:"users"}}  fields:{string:{sval:"password"}}  }}  }}
						from_clause:{range_var:{catalogname:"mydb"  schemaname:"sales"  relname:"users"  inh:true  relpersistence:"p"  location:41}}  }}
	*/
	var strFields []*pg_query.String
	for _, field := range cr.Fields {
		if field.GetString_() != nil {
			strFields = append(strFields, field.GetString_())
		}
	}

	switch len(strFields) {
	case 1: // col
		str := strFields[0]
		str.Sval, err = a.registry.GetHash(COLUMN_KIND_PREFIX, str.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=1, column field: %w", err)
		}
	case 2: // tbl/alias . col
		// corner case not handled: it can be a table name or table alias
		tbl := strFields[0]
		col := strFields[1]
		tbl.Sval, err = a.registry.GetHash(TABLE_KIND_PREFIX, tbl.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=2, table field: %w", err)
		}
		col.Sval, err = a.registry.GetHash(COLUMN_KIND_PREFIX, col.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=2, column field: %w", err)
		}
	case 3: // schema . table . col
		sch := strFields[0]
		tbl := strFields[1]
		col := strFields[2]
		sch.Sval, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, sch.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=3, schema field: %w", err)
		}
		tbl.Sval, err = a.registry.GetHash(TABLE_KIND_PREFIX, tbl.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=3, table field: %w", err)
		}
		col.Sval, err = a.registry.GetHash(COLUMN_KIND_PREFIX, col.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=3, column field: %w", err)
		}
	case 4: // db . schema . table . col
		db := strFields[0]
		sch := strFields[1]
		tbl := strFields[2]
		col := strFields[3]
		db.Sval, err = a.registry.GetHash(DATABASE_KIND_PREFIX, db.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=4, database field: %w", err)
		}
		sch.Sval, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, sch.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=4, schema field: %w", err)
		}
		tbl.Sval, err = a.registry.GetHash(TABLE_KIND_PREFIX, tbl.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=4, table field: %w", err)
		}
		col.Sval, err = a.registry.GetHash(COLUMN_KIND_PREFIX, col.Sval)
		if err != nil {
			return fmt.Errorf("anon ColumnRef, fieldLen=4, column field: %w", err)
		}

	default: // zero fields
		return nil
	}
	return nil
}
