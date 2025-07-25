package anon

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/samber/lo"
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

	err = a.handleSchemaObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling schema object nodes: %w", err)
	}

	err = a.handleCollationObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling collation object nodes: %w", err)
	}

	err = a.handleExtensionObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling extension object nodes: %w", err)
	}

	err = a.handleSequenceObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling sequence object nodes: %w", err)
	}

	err = a.handleUserDefinedTypeObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling user defined type object nodes: %w", err)
	}

	err = a.handleDomainObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling domain object nodes: %w", err)
	}

	err = a.handleTableObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling table object nodes: %w", err)
	}

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

		// ISSUE: RangeVar.relname is not always a TABLE - it could be a SEQUENCE, VIEW, MATERIALIZED VIEW, etc.
		// This creates two problems:
		// 1. The same identifier might be anonymized twice by different processors
		// 2. An identifier might be anonymized with the wrong prefix (e.g., table_ instead of seq_)
		//
		// IDEAL SOLUTION: Process each identifier exactly once across all processors
		// However, this would require complex coordination logic and isn't worth the implementation effort.
		//
		// CURRENT SOLUTION: Check if an identifier is already anonymized before processing
		// This allows processors to be written independently without worrying about double-anonymization.
		// Implemented in identifier_hash_registry.go GetHash() method
		//
		// IMPORTANT: Fallback/catchall processors like this one should run last to avoid conflicts.
		err = a.anonymizeRangeVarNode(rv, TABLE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon rangevar: %w", err)
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
		err = a.anonymizeColumnDefNode(cd)
		if err != nil {
			return fmt.Errorf("error anonymizing column def: %w", err)
		}

	/*
		ColumnRef node is for column names in SELECT, WHERE, etc.
		SQL: 		SELECT * from sales.orders;
		ParseTree:	{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
					from_clause:{range_var:{schemaname:"sales"  relname:"orders" }}  }}

		Other example:	SELECT *, mydb.sales.users.password FROM mydb.sales.users;
		ParseTree:		stmt:{select_stmt:{target_list:{res_target:{val:{ column_ref:
						{fields:{string:{sval:"mydb"}}  fields:{string:{sval:"sales"}}  fields:{string:{sval:"users"}}  fields:{string:{sval:"password"}}  }}  }}
						target_list:{res_target:{val:{ column_ref:
						{fields:{string:{sval:"mydb"}}  fields:{string:{sval:"sales"}}  fields:{string:{sval:"users"}}  fields:{string:{sval:"password"}}  }}  }}
						from_clause:{range_var:{catalogname:"mydb"  schemaname:"sales"  relname:"users"  inh:true  relpersistence:"p"  location:41}}  }}
	*/
	case queryparser.PG_QUERY_COLUMNREF_NODE:
		cr, ok := queryparser.ProtoAsColumnRefNode(msg)
		if !ok {
			return fmt.Errorf("expected ColumnRef, got %T", msg.Interface())
		}
		err = a.anonymizeColumnRefNode(cr.Fields)
		if err != nil {
			return fmt.Errorf("error anonymizing column ref: %w", err)
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

		SQL:		ALTER TABLE sales.orders ADD CONSTRAINT fk_customer FOREIGN KEY (customer_id) REFERENCES customers(id);
		ParseTree:	stmt:{alter_table_stmt:{relation:{schemaname:"sales" relname:"orders" inh:true relpersistence:"p" location:12}
					cmds:{alter_table_cmd:{subtype:AT_AddConstraint def:{
					constraint:{contype:CONSTR_FOREIGN conname:"fk_customer"
					pktable:{relname:"customers" inh:true relpersistence:"p" location:89} fk_attrs:{string:{sval:"customer_id"}}
					pk_attrs:{string:{sval:"id"}} fk_matchtype:"s" fk_upd_action:"a" fk_del_action:"a" initially_valid:true}} behavior:DROP_RESTRICT}}
					objtype:OBJECT_TABLE}}

		SQL:		ALTER TABLE sales.orders ADD CONSTRAINT uq_order_number UNIQUE USING INDEX idx_unique_order_num;
		ParseTree:	stmt:{alter_table_stmt:{relation:{schemaname:"sales" relname:"orders" inh:true relpersistence:"p" location:12}
					cmds:{alter_table_cmd:{subtype:AT_AddConstraint def:{
					constraint:{contype:CONSTR_UNIQUE conname:"uq_order_number"
					indexname:"idx_unique_order_num"}} behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}} stmt_len:95

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

		err = a.anonymizeStringNodes(cons.PkAttrs, COLUMN_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon constraint pkattrs: %w", err)
		}
		err = a.anonymizeStringNodes(cons.FkAttrs, COLUMN_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon constraint fkattrs: %w", err)
		}

		cons.Indexname, err = a.registry.GetHash(INDEX_KIND_PREFIX, cons.Indexname)
		if err != nil {
			return fmt.Errorf("anon constraint indexname: %w", err)
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

	// ─── GENERIC RENAME STATEMENT PROCESSOR ─────────────────────────────────────────────
	case queryparser.PG_QUERY_RENAME_STMT_NODE:
		rs, ok := queryparser.ProtoAsRenameStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected RenameStmt, got %T", msg.Interface())
		}
		err = a.handleGenericRenameStmt(rs)
		if err != nil {
			return fmt.Errorf("anon rename stmt: %w", err)
		}

	// ─── GENERIC DROP STATEMENT PROCESSOR ─────────────────────────────────────────────
	case queryparser.PG_QUERY_DROP_STMT_NODE:
		ds, ok := queryparser.ProtoAsDropStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected DropStmt, got %T", msg.Interface())
		}
		err = a.handleGenericDropStmt(ds)
		if err != nil {
			return fmt.Errorf("anon drop stmt: %w", err)
		}

	// ─── GENERIC ALTER OBJECT SCHEMA STATEMENT PROCESSOR ─────────────────────────────────────────────
	case queryparser.PG_QUERY_ALTER_OBJECT_SCHEMA_STMT_NODE:
		aos, ok := queryparser.ProtoAsAlterObjectSchemaStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterObjectSchemaStmt, got %T", msg.Interface())
		}
		err = a.handleGenericAlterObjectSchemaStmt(aos)
		if err != nil {
			return fmt.Errorf("anon alter object schema stmt: %w", err)
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
		// DEFELEM node is used in CREATE EXTENSION, CREATE FOREIGN TABLE have table_name and schema
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

// ----- smaller helper function for each object type processing ----

func (a *SqlAnonymizer) handleSchemaObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	// ─── SCHEMA: CREATE ───────────────────────────────────────────
	case queryparser.PG_QUERY_CREATE_SCHEMA_STMT_NODE:
		cs, ok := queryparser.ProtoAsCreateSchemaStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateSchemaStmt, got %T", msg.Interface())
		}
		// anonymize the schema name
		cs.Schemaname, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, cs.Schemaname)
		if err != nil {
			return fmt.Errorf("anon schema create: %w", err)
		}

	// ─── SCHEMA: ALTER(OWNER) ────────────────────────────────────────────
	// SQL: ALTER SCHEMA sales_new OWNER TO sales_owner;
	case queryparser.PG_QUERY_ALTER_OWNER_STMT_NODE:
		ao, ok := queryparser.ProtoAsAlterOwnerStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterOwnerStmt, got %T", msg.Interface())
		}

		if ao.ObjectType != pg_query.ObjectType_OBJECT_SCHEMA {
			return nil // not a schema DDL, skip
		}

		if ao.Object != nil && ao.Object.GetString_() != nil {
			// anonymize the schema name
			ao.Object.GetString_().Sval, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, ao.Object.GetString_().Sval)
			if err != nil {
				return fmt.Errorf("anon schema alter owner: %w", err)
			}
		}

	// ─── SCHEMA: GRANT ────────────────────────────────────────────
	// SQL: GRANT USAGE ON SCHEMA sales TO sales_user;
	case queryparser.PG_QUERY_GRANT_STMT_NODE:
		gs, ok := queryparser.ProtoAsGrantStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected GrantStmt, got %T", msg.Interface())
		}

		if gs.Objtype != pg_query.ObjectType_OBJECT_SCHEMA {
			return nil // not a schema DDL, skip
		}

		// Anonymize each schema name in the GRANT list
		for _, obj := range gs.Objects {
			if obj == nil {
				continue
			}
			if str := obj.GetString_(); str != nil && str.Sval != "" {
				str.Sval, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, str.Sval)
				if err != nil {
					return fmt.Errorf("anon schema grant: %w", err)
				}
			}
		}
	}

	return nil
}

func (a *SqlAnonymizer) handleCollationObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	// ─── COLLATION: CREATE ────────────────────────────────────────────
	/*
		SQL:  		CREATE COLLATION sales.nocase (provider = icu, locale = 'und');
		ParseTree: 	stmt:{define_stmt:{kind:OBJECT_COLLATION defnames:{string:{sval:"sales"}} defnames:{string:{sval:"nocase"}} definition:{def_elem:{defname:"provider"
					arg:{type_name:{names:{string:{sval:"icu"}} typemod:-1 location:43}} defaction:DEFELEM_UNSPEC location:32}} definition:{def_elem:{defname:"locale" arg:{string:{sval:"und"}} ...}}}
	*/
	case queryparser.PG_QUERY_DEFINE_STMT_NODE:
		ds, ok := queryparser.ProtoAsDefineStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected DefineStmt, got %T", msg.Interface())
		}

		if ds.Kind != pg_query.ObjectType_OBJECT_COLLATION {
			return nil // not a collation DDL, skip
		}

		// Anonymize the qualified collation name
		// defnames may be:
		//   [name]
		//   [schema, name]
		//   [dbname, schema, name]
		// parser won't fail if defnames has more than 3 elements, but thats not a valid SQL
		for i, nameNode := range ds.Defnames {
			if str := nameNode.GetString_(); str != nil && str.Sval != "" {
				switch {
				case len(ds.Defnames) == 3 && i == 0:
					// first element is database
					tok, err := a.registry.GetHash(DATABASE_KIND_PREFIX, str.Sval)
					if err != nil {
						return fmt.Errorf("anon collation create db: %w", err)
					}
					str.Sval = tok

				case (len(ds.Defnames) == 3 || len(ds.Defnames) == 2) && i == len(ds.Defnames)-2:
					// second‐to‐last element is schema (when 2 or 3 parts)
					tok, err := a.registry.GetHash(SCHEMA_KIND_PREFIX, str.Sval)
					if err != nil {
						return fmt.Errorf("anon collation create schema: %w", err)
					}
					str.Sval = tok

				case i == len(ds.Defnames)-1:
					// last element is the collation name
					tok, err := a.registry.GetHash(COLLATION_KIND_PREFIX, str.Sval)
					if err != nil {
						return fmt.Errorf("anon collation create name: %w", err)
					}
					str.Sval = tok
				}
			}
		}
	}
	return nil
}

func (a *SqlAnonymizer) handleExtensionObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	// ─── EXTENSION: CREATE ────────────────────────────────────────────
	// already covered in miscellaneousNodesProcessor as DefElemNode

	// ─── EXTENSION: ALTER (change schema) ────────────────────────────
	/*
		SQL: 		ALTER EXTENSION postgis SET SCHEMA archive;
		ParseTree: 	stmt:{alter_object_schema_stmt:{object_type:OBJECT_EXTENSION object:{string:{sval:"postgis"}} newschema:"archive"}} stmt_len:42
	*/
	// Covered in handleGenericAlterObjectSchemaStmt

	// ─── EXTENSION: ALTER (OTHER) ────────────────────────────
	// there are other alter extension statement where you add or drop a member_object [skipping]
	// for eg: ALTER EXTENSION hstore ADD FUNCTION populate_record(anyelement, hstore);
	// but this is too much of a corner case and not sure if pg_dump will generate such statements

	}
	return nil
}

func (a *SqlAnonymizer) handleSequenceObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {

	// SQL: CREATE SEQUENCE sales.ord_id_seq;
	case queryparser.PG_QUERY_CREATE_SEQ_STMT_NODE:
		cs, ok := queryparser.ProtoAsCreateSeqStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateSeqStmt, got %T", msg.Interface())
		}
		rv := cs.Sequence
		if rv == nil {
			return nil
		}
		err = a.anonymizeRangeVarNode(rv, SEQUENCE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon create sequence: %w", err)
		}

	// ALTER SEQUENCE  (incl. OWNED BY)
	// SQL: ALTER SEQUENCE sales.ord_id_seq OWNED BY sales.orders.id;
	case queryparser.PG_QUERY_ALTER_SEQ_STMT_NODE:
		as, ok := queryparser.ProtoAsAlterSeqStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterSeqStmt, got %T", msg.Interface())
		}
		err = a.anonymizeRangeVarNode(as.Sequence, SEQUENCE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon alter sequence: %w", err)
		}

		// handle OWNED BY clause
		for _, opt := range as.Options {
			if opt == nil {
				continue
			}
			def := opt.GetDefElem()
			if def == nil || def.Defname != "owned_by" || def.Arg == nil {
				continue
			}
			if list := def.Arg.GetList(); list != nil {
				itemNodes := list.Items
				err = a.anonymizeColumnRefNode(itemNodes)
				if err != nil {
					return fmt.Errorf("anon alter sequence owned by: %w", err)
				}
			}
		}

		// ALTER SEQUENCE … SET SCHEMA
		// SQL: ALTER SEQUENCE sales.ord_id_seq SET SCHEMA archive;
		// Covered in handleGenericAlterObjectSchemaStmt

	}
	return nil
}

// this has processors for TYPE(user defined types i.e. COMPOSITE), ENUM, and DOMAIN
func (a *SqlAnonymizer) handleUserDefinedTypeObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	/*
		SQL:		CREATE TYPE db.schema1.status AS ENUM ('new','proc','done');
		ParseTree:	stmt:{create_enum_stmt:{type_name:{string:{sval:"db"}} type_name:{string:{sval:"schema1"}} type_name:{string:{sval:"status"}}
					vals:{string:{sval:"new"}} vals:{string:{sval:"proc"}} vals:{string:{sval:"done"}}}}
	*/
	case queryparser.PG_QUERY_CREATE_ENUM_TYPE_STMT_NODE:
		ces, ok := queryparser.ProtoAsCreateEnumStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateEnumStmt, got %T", msg.Interface())
		}

		// Anonymize the type name (fully qualified: database.schema.typename)
		err = a.anonymizeStringNodes(ces.GetTypeName(), TYPE_KIND_PREFIX) // object type for enum is TYPE
		if err != nil {
			return fmt.Errorf("anon enum typename: %w", err)
		}

		// anonymize the enum values
		for _, val := range ces.Vals {
			if str := val.GetString_(); str != nil && str.Sval != "" {
				str.Sval, err = a.registry.GetHash(ENUM_KIND_PREFIX, str.Sval)
				if err != nil {
					return err
				}
			}
		}

	/*
		SQL:		ALTER TYPE status ADD VALUE 'archived';
		ParseTree:	stmt:{alter_enum_stmt:{type_name:{string:{sval:"status"}} new_val:"archived" new_val_is_after:true}} stmt_len:3
	*/
	case queryparser.PG_QUERY_ALTER_ENUM_TYPE_STMT_NODE:
		ae, ok := queryparser.ProtoAsAlterEnumStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterEnumStmt, got %T", msg.Interface())
		}

		// Anonymize the type name (fully qualified: database.schema.typename)
		err = a.anonymizeStringNodes(ae.GetTypeName(), TYPE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon alter enum type: %w", err)
		}

		// Anonymize the new value
		if ae.NewVal != "" {
			ae.NewVal, err = a.registry.GetHash(ENUM_KIND_PREFIX, ae.NewVal)
			if err != nil {
				return err
			}
		}

	/*
		SQL: 		CREATE TYPE mycomposit AS (a int, b text);
		ParseTree:	stmt:{composite_type_stmt:{typevar:{relname:"mycomposit" ...} coldeflist:{column_def:{colname:"a" type_name:{names:{string:{sval:"pg_catalog"}}
					names:{string:{sval:"int4"}} typemod:-1 location:29} is_local:true location:27}}
					coldeflist:{column_def:{colname:"b" type_name:{names:{string:{sval:"text"}} ... } }}}}
	*/
	case queryparser.PG_QUERY_CREATE_COMPOSITE_TYPE_STMT_NODE:
		ct, ok := queryparser.ProtoAsCompositeTypeStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateTypeStmt, got %T", msg.Interface())
		}

		// Anonymize the type name present as RangeVar node
		err = a.anonymizeRangeVarNode(ct.Typevar, TYPE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon composite type: %w", err)
		}

		// Anonymize the column names in columndeflist in column_def node
		// Already covered by ColumnDef processor

	}
	return nil
}

func (a *SqlAnonymizer) handleDomainObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {

	/*
		SQL:		CREATE DOMAIN us_postal AS text CHECK (VALUE ~ '^[0-9]{5}$');
		ParseTree:	stmt:{create_domain_stmt:{domainname:{string:{sval:"us_postal"}}  type_name:{names:{string:{sval:"text"}}  typemod:-1  location:27}
					constraints:{constraint:{contype:CONSTR_CHECK  location:32  raw_expr:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"~"}}
					 lexpr:{column_ref:{fields:{string:{sval:"value"}}  location:39}}  rexpr:{a_const:{sval:{sval:"^[0-9]{5}$"}  }} }}  }}}}  stmt_len:60
	*/
	case queryparser.PG_QUERY_CREATE_DOMAIN_STMT_NODE:
		cd, ok := queryparser.ProtoAsCreateDomainStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateDomainStmt, got %T", msg.Interface())
		}
		// Anonymize the domain name
		err = a.anonymizeStringNodes(cd.Domainname, DOMAIN_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon create domain: %w", err)
		}

	}
	return nil
}

func (a *SqlAnonymizer) handleTableObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	/*
		SQL:		CREATE TABLE sales.orders (id int PRIMARY KEY, amt numeric);
		ParseTree:	stmt:{create_stmt:{relation:{schemaname:"sales"  relname:"orders"  inh:true  relpersistence:"p"  location:13}
					table_elts:{column_def:{colname:"id"  type_name:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"int4"}}  ...}  is_local:true  constraints:{constraint:{contype:CONSTR_PRIMARY  }}  }}
					table_elts:{column_def:{colname:"amt"  type_name:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"numeric"}} ...}  is_local:true  location:47}}  oncommit:ONCOMMIT_NOOP}}
	*/
	case queryparser.PG_QUERY_CREATE_STMT_NODE: //|| queryparser.PG_QUERY_CREATE_TABLE_AS_STMT || :
		// RangeVar node in CREATE TABLE is handled by common anonymizeRangeVarNode() processor
		// columnDef nodes in CREATE TABLE are handled by common anonymizeColumnDefNode() processor

	/*
		SQL:		ALTER TABLE sales.orders ADD COLUMN note text;
		ParseTree:	stmt:{alter_table_stmt:{relation:{schemaname:"sales"  relname:"orders"  inh:true  relpersistence:"p"  location:12}
					cmds:{alter_table_cmd:{subtype:AT_AddColumn  def:{column_def:{colname:"note"  type_name:{names:{string:{sval:"text"}} }  }} }}  objtype:OBJECT_TABLE}}

		SQL: 		ALTER TABLE sales.orders ALTER COLUMN amount TYPE decimal(10,2);
		ParseTree: 	stmt:{alter_table_stmt:{relation:{schemaname:"sales" relname:"orders" inh:true relpersistence:"p" location:12}
					cmds:{alter_table_cmd:{subtype:AT_AlterColumnType name:"amount" def:{column_def:{type_name:{names:{string:{sval:"pg_catalog"}}
					names:{string:{sval:"numeric"}} typmods:{a_const:{ival:{ival:10} location:58}} typmods:{a_const:{ival:{ival:2} }} } }} behavior:DROP_RESTRICT}} objtype:OBJECT_TABLE}}

		SQL:		ALTER TABLE sales.orders RENAME COLUMN amt TO amount;
		ParseTree:	stmt:{rename_stmt:{rename_type:OBJECT_COLUMN  relation_type:OBJECT_TABLE  relation:{schemaname:"sales"  relname:"orders"  }
					subname:"amt"  newname:"amount"  behavior:DROP_RESTRICT}}

	*/
	case queryparser.PG_QUERY_ALTER_TABLE_STMT_NODE:
		at, ok := queryparser.ProtoAsAlterTableStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterTableStmt, got %T", msg.Interface())
		}
		err = a.anonymizeRangeVarNode(at.Relation, TABLE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon alter table: %w", err)
		}

	case queryparser.PG_QUERY_ALTER_TABLE_CMD_NODE:
		atc, ok := queryparser.ProtoAsAlterTableCmdNode(msg)
		if !ok {
			return fmt.Errorf("expected AlterTableCmd, got %T", msg.Interface())
		}

		// Handle only ALTER TABLE command types that contain sensitive information
		switch atc.Subtype {

		// ─── COLUMN OPERATIONS (using COLUMN_KIND_PREFIX) ─────────────────────────────────────────────
		case pg_query.AlterTableType_AT_DropColumn,
			pg_query.AlterTableType_AT_AlterColumnType,
			pg_query.AlterTableType_AT_ColumnDefault,
			pg_query.AlterTableType_AT_CookedColumnDefault,
			pg_query.AlterTableType_AT_DropNotNull,
			pg_query.AlterTableType_AT_SetNotNull,
			pg_query.AlterTableType_AT_CheckNotNull,
			pg_query.AlterTableType_AT_SetExpression,
			pg_query.AlterTableType_AT_DropExpression,
			pg_query.AlterTableType_AT_SetStatistics,
			pg_query.AlterTableType_AT_SetOptions,
			pg_query.AlterTableType_AT_ResetOptions,
			pg_query.AlterTableType_AT_SetStorage,
			pg_query.AlterTableType_AT_SetCompression,
			pg_query.AlterTableType_AT_AlterColumnGenericOptions,
			pg_query.AlterTableType_AT_AddIdentity,
			pg_query.AlterTableType_AT_SetIdentity,
			pg_query.AlterTableType_AT_DropIdentity:
			// All these operations work on column names
			if atc.Name != "" {
				atc.Name, err = a.registry.GetHash(COLUMN_KIND_PREFIX, atc.Name)
				if err != nil {
					return fmt.Errorf("anon alter table column operation: %w", err)
				}
			}

		// ─── CONSTRAINT OPERATIONS (using CONSTRAINT_KIND_PREFIX) ─────────────────────────────────────────────
		case pg_query.AlterTableType_AT_AlterConstraint,
			pg_query.AlterTableType_AT_ValidateConstraint,
			pg_query.AlterTableType_AT_DropConstraint:
			// All these operations work on constraint names
			if atc.Name != "" {
				atc.Name, err = a.registry.GetHash(CONSTRAINT_KIND_PREFIX, atc.Name)
				if err != nil {
					return fmt.Errorf("anon alter table constraint operation: %w", err)
				}
			}

		// ─── TRIGGER/RULE OPERATIONS (using TRIGGER_KIND_PREFIX) ─────────────────────────────────────────────
		case pg_query.AlterTableType_AT_EnableTrig,
			pg_query.AlterTableType_AT_EnableAlwaysTrig,
			pg_query.AlterTableType_AT_EnableReplicaTrig,
			pg_query.AlterTableType_AT_DisableTrig,
			pg_query.AlterTableType_AT_EnableRule,
			pg_query.AlterTableType_AT_EnableAlwaysRule,
			pg_query.AlterTableType_AT_EnableReplicaRule,
			pg_query.AlterTableType_AT_DisableRule:
			// All these operations work on trigger/rule names
			if atc.Name != "" {
				atc.Name, err = a.registry.GetHash(TRIGGER_KIND_PREFIX, atc.Name)
				if err != nil {
					return fmt.Errorf("anon alter table trigger/rule operation: %w", err)
				}
			}

		// ─── INDEX OPERATIONS (using INDEX_KIND_PREFIX) ─────────────────────────────────────────────
		// caveat: Here after "ON" it can be INDEX or Primary key both
		// because PRIMARY KEY is also an INDEX, but at other places we use CONSTRAINT_KIND_PREFIX for it
		// but here falling back to INDEX_KIND_PREFIX instead of going and determing it is index or primary key
		case pg_query.AlterTableType_AT_ClusterOn:
			// CLUSTER ON index_name
			if atc.Name != "" {
				atc.Name, err = a.registry.GetHash(INDEX_KIND_PREFIX, atc.Name)
				if err != nil {
					return fmt.Errorf("anon alter table cluster on: %w", err)
				}
			}

		// ─── REPLICA IDENTITY OPERATIONS (custom handling) ─────────────────────────────────────────────
		case pg_query.AlterTableType_AT_ReplicaIdentity:
			// REPLICA IDENTITY { DEFAULT | USING INDEX index_name | FULL | NOTHING }
			if atc.Def != nil {
				replicaIdentityNode := atc.Def.GetReplicaIdentityStmt()
				if replicaIdentityNode != nil && replicaIdentityNode.Name != "" {
					replicaIdentityNode.Name, err = a.registry.GetHash(INDEX_KIND_PREFIX, replicaIdentityNode.Name)
					if err != nil {
						return fmt.Errorf("anon alter table replica identity: %w", err)
					}
				}
			}

		// ─── OPERATIONS HANDLED BY OTHER PROCESSORS ─────────────────────────────────────────────
		case pg_query.AlterTableType_AT_AddConstraint,
			pg_query.AlterTableType_AT_ReAddConstraint,
			pg_query.AlterTableType_AT_ReAddDomainConstraint,
			pg_query.AlterTableType_AT_AddIndex,
			pg_query.AlterTableType_AT_ReAddIndex,
			pg_query.AlterTableType_AT_AddIndexConstraint:
			// These operations have their definitions handled by other processors:
			// - Constraint definitions handled by CONSTRAINT_NODE processor
			// - Index definitions handled by IndexStmt processor

		case pg_query.AlterTableType_AT_ChangeOwner:
			// OWNER TO role_name
			// The role is in atc.Newowner, handled by RoleSpec processor

		case pg_query.AlterTableType_AT_AddInherit,
			pg_query.AlterTableType_AT_DropInherit,
			pg_query.AlterTableType_AT_DetachPartition,
			pg_query.AlterTableType_AT_DetachPartitionFinalize:
			// Parent/partition table names in atc.Def as RangeVar, handled by RangeVar processor

		case pg_query.AlterTableType_AT_AddOf:
			// OF type_name
			// Type name is in atc.Def as TypeName, handled by TypeName processor

		case pg_query.AlterTableType_AT_AttachPartition:
			// ATTACH PARTITION partition_table FOR VALUES ...
			// Partition table name is in atc.Def as PartitionCmd, which contains RangeVar

			// All other cases that don't contain sensitive information are intentionally omitted:
			// - AT_DropCluster, AT_SetLogged, AT_SetUnLogged, AT_DropOids
			// - AT_EnableRowSecurity, AT_DisableRowSecurity, AT_ForceRowSecurity, AT_NoForceRowSecurity
			// - AT_EnableTrigAll, AT_DisableTrigAll, AT_EnableTrigUser, AT_DisableTrigUser
			// - AT_SetRelOptions, AT_ResetRelOptions, AT_ReplaceRelOptions, AT_GenericOptions
			// - AT_DropOf, AT_ReAddComment, AT_SetAccessMethod
		}
	}
	return nil
}

// anonymizeStringNodes walks a slice of *pg_query.Node whose concrete
// value is expected to be *pg_query.String and replaces every Sval
// with the deterministic token from the IdentifierHashRegistry.
//
// parts may be 1-3 items: [obj] | [schema,obj] | [db,schema,obj].
// Pass the PREFIX for the *last* element (obj).
// The helper will automatically use DATABASE_KIND_PREFIX and SCHEMA_KIND_PREFIX
// for the first and second element when present.
func (a *SqlAnonymizer) anonymizeStringNodes(nodes []*pg_query.Node, finalPrefix string) error {
	if len(nodes) == 0 {
		return nil
	}

	// pick the actual *pg_query.String values
	var strs []*pg_query.String
	for _, n := range nodes {
		if s := n.GetString_(); s != nil && s.Sval != "" {
			strs = append(strs, s)
		}
	}
	if len(strs) == 0 {
		return nil // nothing to do
	}

	// build prefix slice based on number of parts
	var prefixes []string
	switch len(strs) {
	case 1:
		prefixes = []string{finalPrefix} // [obj]
	case 2:
		prefixes = []string{SCHEMA_KIND_PREFIX, finalPrefix} // [schema,obj]
	case 3:
		prefixes = []string{DATABASE_KIND_PREFIX, SCHEMA_KIND_PREFIX, finalPrefix} // [db,schema,obj]
	default:
		return fmt.Errorf("qualified name with %d parts not supported", len(strs))
	}

	// apply prefixes to each string based on the index
	for i, s := range strs {
		tok, err := a.registry.GetHash(prefixes[i], s.Sval)
		if err != nil {
			return err
		}
		s.Sval = tok
	}
	return nil
}

// anonymizeColumnRefNode rewrites the String parts of a ColumnRef
// The slice may have 1–4 parts but the *last* one is always a column.
func (a *SqlAnonymizer) anonymizeColumnRefNode(strNodes []*pg_query.Node) error {
	n := len(strNodes)
	if n == 0 {
		return nil
	}

	// 1. Everything *left* of the column (0,1 or 2 items)
	if n > 1 {
		// left part could be [tbl] or [sch,tbl] or [db,sch,tbl]
		if err := a.anonymizeStringNodes(strNodes[:n-1], TABLE_KIND_PREFIX); err != nil {
			return err
		}
	}

	// 2. The right-most item – always a column
	return a.anonymizeStringNodes(strNodes[n-1:], COLUMN_KIND_PREFIX)
}

// sample rangevar node: typevar:{catalogname:"db"  schemaname:"schema1"  relname:"mycomposit"  relpersistence:"p"  location:12}
func (a *SqlAnonymizer) anonymizeRangeVarNode(rv *pg_query.RangeVar, finalPrefix string) (err error) {
	if rv == nil {
		return nil
	}
	if rv.Catalogname != "" {
		rv.Catalogname, err = a.registry.GetHash(DATABASE_KIND_PREFIX, rv.Catalogname)
		if err != nil {
			return err
		}
	}
	if rv.Schemaname != "" {
		rv.Schemaname, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, rv.Schemaname)
		if err != nil {
			return err
		}
	}
	rv.Relname, err = a.registry.GetHash(finalPrefix, rv.Relname)
	if err != nil {
		return err
	}
	return nil
}

func (a *SqlAnonymizer) anonymizeColumnDefNode(cd *pg_query.ColumnDef) (err error) {
	if cd == nil {
		return nil
	}
	cd.Colname, err = a.registry.GetHash(COLUMN_KIND_PREFIX, cd.Colname)
	if err != nil {
		return fmt.Errorf("anon coldef: %w", err)
	}
	return nil
}

// handleGenericRenameStmt processes RENAME statements for any object type
/*
	SQL:			ALTER SCHEMA sales RENAME TO sales_new
    ParseTree:		stmt:{rename_stmt:{rename_type:OBJECT_SCHEMA  relation_type:OBJECT_ACCESS_METHOD  subname:"sales"  newname:"sales_new" ...}}
*/
func (a *SqlAnonymizer) handleGenericRenameStmt(rs *pg_query.RenameStmt) error {
	if rs == nil {
		return nil
	}

	prefix := a.getObjectTypePrefix(rs.RenameType)

	// Handle different object name patterns based on object type
	switch rs.RenameType {

	/*
		SQL:		ALTER SCHEMA sales RENAME TO sales_new;
		ParseTree:	stmt:{rename_stmt:{rename_type:OBJECT_SCHEMA  relation_type:OBJECT_ACCESS_METHOD  subname:"sales"  newname:"sales_new" ...}}

		SQL:		ALTER TABLE sales.orders RENAME COLUMN amt TO amount;
		ParseTree:	stmt:{rename_stmt:{rename_type:OBJECT_COLUMN  relation_type:OBJECT_TABLE  relation:{schemaname:"sales"  relname:"orders" ...}
				subname:"amt"  newname:"amount"  behavior:DROP_RESTRICT}}
	*/
	case pg_query.ObjectType_OBJECT_SCHEMA, pg_query.ObjectType_OBJECT_COLUMN:
		// Both schema and column names are in rs.Subname
		if rs.Subname != "" {
			var err error
			rs.Subname, err = a.registry.GetHash(prefix, rs.Subname)
			if err != nil {
				return fmt.Errorf("anon rename %s: %w", rs.RenameType, err)
			}
		}

	case pg_query.ObjectType_OBJECT_COLLATION, pg_query.ObjectType_OBJECT_TYPE,
		pg_query.ObjectType_OBJECT_DOMAIN:
		// These objects have qualified names in rs.Object as a list
		if rs.Object != nil && rs.Object.GetList() != nil {
			items := rs.Object.GetList().Items
			err := a.anonymizeStringNodes(items, prefix)
			if err != nil {
				return fmt.Errorf("anon rename qualified object: %w", err)
			}
		}

	default:
		// For other object types, try rs.Object as a simple string
		if rs.Object != nil && rs.Object.GetString_() != nil {
			str := rs.Object.GetString_()
			if str.Sval != "" {
				var err error
				str.Sval, err = a.registry.GetHash(prefix, str.Sval)
				if err != nil {
					return fmt.Errorf("anon rename object: %w", err)
				}
			}
		}
	}

	// Always anonymize the new name
	if rs.Newname != "" {
		var err error
		rs.Newname, err = a.registry.GetHash(prefix, rs.Newname)
		if err != nil {
			return fmt.Errorf("anon rename newname: %w", err)
		}
	}

	return nil
}

// handleGenericDropStmt processes DROP statements for any object type
func (a *SqlAnonymizer) handleGenericDropStmt(ds *pg_query.DropStmt) error {
	if ds == nil || len(ds.Objects) == 0 {
		return nil
	}

	prefix := a.getObjectTypePrefix(ds.RemoveType)

	// Process each object in the DROP statement
	for _, obj := range ds.Objects {
		if obj == nil {
			continue
		}

		var err error
		// Handle different object name patterns based on object type
		switch ds.RemoveType {
		case pg_query.ObjectType_OBJECT_SCHEMA:
			// Schema names are simple strings
			if str := obj.GetString_(); str != nil && str.Sval != "" {
				str.Sval, err = a.registry.GetHash(prefix, str.Sval)
			}

		case pg_query.ObjectType_OBJECT_DOMAIN:
			// Domain names can be in TypeName format
			if tn := obj.GetTypeName(); tn != nil {
				err = a.anonymizeStringNodes(tn.Names, prefix)
			}

		default:
			// For all other object types, try as qualified name list first, then as simple string
			if list := obj.GetList(); list != nil {
				// Use existing anonymizeStringNodes for qualified names
				err = a.anonymizeStringNodes(list.Items, prefix)
			} else if str := obj.GetString_(); str != nil && str.Sval != "" {
				// Fallback to simple string
				str.Sval, err = a.registry.GetHash(prefix, str.Sval)
			}
		}

		if err != nil {
			return fmt.Errorf("anon drop object: %w", err)
		}
	}

	return nil
}

// handleGenericAlterObjectSchemaStmt processes ALTER ... SET SCHEMA statements for TABLE, EXTENSION, TYPE, and SEQUENCE object types
func (a *SqlAnonymizer) handleGenericAlterObjectSchemaStmt(aos *pg_query.AlterObjectSchemaStmt) error {
	if aos == nil {
		return nil
	}

	prefix := a.getObjectTypePrefix(aos.ObjectType)

	// Handle TABLE, EXTENSION, TYPE, and SEQUENCE
	switch aos.ObjectType {
	case pg_query.ObjectType_OBJECT_TABLE, pg_query.ObjectType_OBJECT_SEQUENCE:
		// Tables, Sequences use RangeVar for their names
		if aos.Relation != nil {
			err := a.anonymizeRangeVarNode(aos.Relation, prefix)
			if err != nil {
				return fmt.Errorf("anon alter table schema: %w", err)
			}
		}

	case pg_query.ObjectType_OBJECT_EXTENSION:
		// Extensions have simple string names in Object field
		if aos.Object != nil && aos.Object.GetString_() != nil {
			str := aos.Object.GetString_()
			if str.Sval != "" {
				var err error
				str.Sval, err = a.registry.GetHash(DEFAULT_KIND_PREFIX, str.Sval) // No specific prefix for extensions
				if err != nil {
					return fmt.Errorf("anon alter extension schema: %w", err)
				}
			}
		}

	case pg_query.ObjectType_OBJECT_TYPE:
		// Types have qualified names in Object as a list
		if aos.Object != nil && aos.Object.GetList() != nil {
			items := aos.Object.GetList().Items
			err := a.anonymizeStringNodes(items, TYPE_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon alter type schema: %w", err)
			}
		}
	}

	// Always anonymize the new schema name
	if aos.Newschema != "" {
		var err error
		aos.Newschema, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, aos.Newschema)
		if err != nil {
			return fmt.Errorf("anon alter object schema newschema: %w", err)
		}
	}

	return nil
}

// getObjectTypePrefix returns the appropriate prefix for anonymization based on the PostgreSQL object type
func (a *SqlAnonymizer) getObjectTypePrefix(objectType pg_query.ObjectType) string {
	switch objectType {
	case pg_query.ObjectType_OBJECT_SCHEMA:
		return SCHEMA_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TABLE:
		return TABLE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_VIEW:
		return VIEW_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_MATVIEW:
		return MVIEW_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		return SEQUENCE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_INDEX:
		return INDEX_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TYPE:
		return TYPE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_DOMAIN:
		return DOMAIN_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_COLLATION:
		return COLLATION_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_TRIGGER:
		return TRIGGER_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_ROLE:
		return ROLE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_DATABASE:
		return DATABASE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_FUNCTION:
		return FUNCTION_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_PROCEDURE:
		return PROCEDURE_KIND_PREFIX
	case pg_query.ObjectType_OBJECT_COLUMN:
		return COLUMN_KIND_PREFIX
	default:
		log.Printf("Unknown object type: %s", objectType.String())
		return DEFAULT_KIND_PREFIX // Fallback to default prefix
	}
}
