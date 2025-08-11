package anon

import (
	"fmt"

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

func (a *SqlAnonymizer) identifierNodesProcessor(msg protoreflect.Message) (err error) {
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

	err = a.handleIndexObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling index object nodes: %w", err)
	}

	err = a.handlePolicyObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling policy object nodes: %w", err)
	}

	err = a.handleCommentObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling comment object nodes: %w", err)
	}

	err = a.handleConversionObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling conversion object nodes: %w", err)
	}

	err = a.handleForeignTableObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling foreign table object nodes: %w", err)
	}

	err = a.handleRuleObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling rule object nodes: %w", err)
	}

	err = a.handleAggregateObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling aggregate object nodes: %w", err)
	}

	err = a.handleOperatorObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling operator object nodes: %w", err)
	}

	err = a.handleOperatorClassAndFamilyObjectNodes(msg)
	if err != nil {
		return fmt.Errorf("error handling operator class and family object nodes: %w", err)
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
		rt.Name, err = a.registry.GetHash(COLUMN_KIND_PREFIX, rt.Name)
		if err != nil {
			return fmt.Errorf("anon alias: %w", err)
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

		cons.Conname, err = a.registry.GetHash(CONSTRAINT_KIND_PREFIX, cons.Conname)
		if err != nil {
			return fmt.Errorf("anon constraint: %w", err)
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
		alias.Aliasname, err = a.registry.GetHash(ALIAS_KIND_PREFIX, alias.Aliasname)
		if err != nil {
			return fmt.Errorf("anon aliasnode: %w", err)
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

		// Only anonymize if it's not a built-in type
		if !IsBuiltinType(tn) {
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

		if ac.Val != nil && ac.GetSval() != nil {
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

		if ao.ObjectType == pg_query.ObjectType_OBJECT_SCHEMA {
			if ao.Object != nil && ao.Object.GetString_() != nil {
				// anonymize the schema name
				ao.Object.GetString_().Sval, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, ao.Object.GetString_().Sval)
				if err != nil {
					return fmt.Errorf("anon schema alter owner: %w", err)
				}
			}
		} else if ao.ObjectType == pg_query.ObjectType_OBJECT_CONVERSION {
			// Handle CONVERSION objects
			if ao.Object != nil && ao.Object.GetList() != nil {
				items := ao.Object.GetList().Items
				err = a.anonymizeStringNodes(items, CONVERSION_KIND_PREFIX)
				if err != nil {
					return fmt.Errorf("anon conversion alter owner: %w", err)
				}
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
			if str := obj.GetString_(); str != nil {
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
		_, err = a.handleDefineStmtWithReturn(msg, pg_query.ObjectType_OBJECT_COLLATION, COLLATION_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon collation create: %w", err)
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

			list := def.Arg.GetList()
			if list == nil {
				continue
			}

			itemNodes := list.Items
			err = a.anonymizeColumnRefNode(itemNodes)
			if err != nil {
				return fmt.Errorf("anon alter sequence owned by: %w", err)
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
			str := val.GetString_()
			if str == nil {
				continue
			}

			str.Sval, err = a.registry.GetHash(ENUM_KIND_PREFIX, str.Sval)
			if err != nil {
				return err
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
		ae.NewVal, err = a.registry.GetHash(ENUM_KIND_PREFIX, ae.NewVal)
		if err != nil {
			return err
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

	/*
		SQL:		CREATE TYPE myrange AS RANGE (subtype = int4);
		ParseTree:	stmt:{create_range_stmt:{type_name:{string:{sval:"myrange"}} params:{def_elem:{defname:"subtype"
					arg:{type_name:{names:{string:{sval:"int4"}} }} defaction:DEFELEM_UNSPEC }}}}
	*/
	case queryparser.PG_QUERY_CREATE_RANGE_STMT_NODE:
		crs, ok := queryparser.ProtoAsCreateRangeStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateRangeStmt, got %T", msg.Interface())
		}

		// Anonymize the range type name
		err = a.anonymizeStringNodes(crs.GetTypeName(), TYPE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon range type name: %w", err)
		}

		// Anonymize subtype which can be a user defined type or a built-in type
		for _, defElem := range crs.Params {
			defElemNode := defElem.GetDefElem()
			if defElemNode == nil || defElemNode.Defname == "" {
				continue
			}
			switch defElemNode.Defname {
			case "subtype":
				// For range types, subtype should be anonymized as TYPE
				if defElemNode.Arg == nil {
					continue
				}
				typeName := defElemNode.Arg.GetTypeName()
				if typeName == nil || IsBuiltinType(typeName) {
					continue
				}
				if err := a.anonymizeStringNodes(typeName.Names, TYPE_KIND_PREFIX); err != nil {
					return fmt.Errorf("anon range subtype: %w", err)
				}
			}
		}

	/*
		CREATE TYPE base_type_examples.base_type (
				INTERNALLENGTH = variable, -- anonymized as constant
				INPUT = base_type_examples.base_fn_in, -- anonymized as function
				OUTPUT = base_type_examples.base_fn_out, -- anonymized as function
				ALIGNMENT = int4, -- anonymized as constant
				STORAGE = plain -- anonymized as constant
		);
		ParseTree: stmt:{define_stmt:{kind:OBJECT_TYPE defnames:{string:{sval:"base_type_examples"}} defnames:{string:{sval:"base_type"}}
		definition:{def_elem:{defname:"internallength" arg:{type_name:{names:{string:{sval:"variable"}} }} defaction:DEFELEM_UNSPEC }}
		definition:{def_elem:{defname:"input" arg:{type_name:{names:{string:{sval:"base_type_examples"}} names:{string:{sval:"base_fn_in"}} }}
		defaction:DEFELEM_UNSPEC }} definition:{def_elem:{defname:"output" arg:{type_name:{names:{string:{sval:"base_type_examples"}} names:{string:{sval:"base_fn_out"}} }}
		defaction:DEFELEM_UNSPEC }} definition:{def_elem:{defname:"alignment" arg:{type_name:{names:{string:{sval:"int4"}} }}
		defaction:DEFELEM_UNSPEC }} definition:{def_elem:{defname:"storage" arg:{type_name:{names:{string:{sval:"plain"}} }}
		defaction:DEFELEM_UNSPEC }}}}
	*/
	case queryparser.PG_QUERY_DEFINE_STMT_NODE:
		ds, err := a.handleDefineStmtWithReturn(msg, pg_query.ObjectType_OBJECT_TYPE, TYPE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon create type define: %w", err)
		}

		// If this is not a TYPE DefineStmt, skip
		if ds == nil {
			return nil
		}

		// Additional processing to anonymize function names in definitions
		// For base types: input/output functions, for range types: subtype references
		for _, defElem := range ds.Definition {
			defElemNode := defElem.GetDefElem()
			if defElemNode == nil || defElemNode.Defname == "" {
				continue
			}
			switch defElemNode.Defname {
			case "input", "output":
				// For base types, input/output functions are stored as TypeName nodes
				if defElemNode.Arg == nil {
					continue
				}
				typeName := defElemNode.Arg.GetTypeName()
				if typeName == nil {
					continue
				}
				if err := a.anonymizeStringNodes(typeName.Names, FUNCTION_KIND_PREFIX); err != nil {
					return fmt.Errorf("anon type %s function: %w", defElemNode.Defname, err)
				}
			case "subtype":
				tn := defElemNode.Arg.GetTypeName()
				if tn == nil || IsBuiltinType(tn) {
					continue
				}
				for i, node := range tn.Names {
					str := node.GetString_()
					if str == nil || str.Sval == "" {
						continue
					}
					hashed, err := a.registry.GetHash(TYPE_KIND_PREFIX, str.Sval)
					if err != nil {
						return fmt.Errorf("anon subtype[%d]=%q lookup: %w", i, str.Sval, err)
					}
					str.Sval = hashed
				}
			case "internallength", "alignment", "storage":
				// These values should be treated as constants, not types
				// Even though they're stored as TypeName nodes in the parse tree
				if defElemNode.Arg == nil {
					continue
				}
				typeName := defElemNode.Arg.GetTypeName()
				if typeName == nil {
					continue
				}
				if err := a.anonymizeStringNodes(typeName.Names, CONST_KIND_PREFIX); err != nil {
					return fmt.Errorf("anon type %s value: %w", defElemNode.Defname, err)
				}
			}
		}
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

// alterTablePrefixMap maps AlterTableType to the appropriate prefix for anonymization
var alterTablePrefixMap = map[pg_query.AlterTableType]string{
	// Column operations
	pg_query.AlterTableType_AT_DropColumn:                COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_AlterColumnType:           COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_ColumnDefault:             COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_CookedColumnDefault:       COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_DropNotNull:               COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetNotNull:                COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_CheckNotNull:              COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetExpression:             COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_DropExpression:            COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetStatistics:             COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetOptions:                COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_ResetOptions:              COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetStorage:                COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetCompression:            COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_AlterColumnGenericOptions: COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_AddIdentity:               COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_SetIdentity:               COLUMN_KIND_PREFIX,
	pg_query.AlterTableType_AT_DropIdentity:              COLUMN_KIND_PREFIX,

	// Constraint operations
	pg_query.AlterTableType_AT_AlterConstraint:    CONSTRAINT_KIND_PREFIX,
	pg_query.AlterTableType_AT_ValidateConstraint: CONSTRAINT_KIND_PREFIX,
	pg_query.AlterTableType_AT_DropConstraint:     CONSTRAINT_KIND_PREFIX,

	// Trigger/Rule operations
	pg_query.AlterTableType_AT_EnableTrig:        TRIGGER_KIND_PREFIX,
	pg_query.AlterTableType_AT_EnableAlwaysTrig:  TRIGGER_KIND_PREFIX,
	pg_query.AlterTableType_AT_EnableReplicaTrig: TRIGGER_KIND_PREFIX,
	pg_query.AlterTableType_AT_DisableTrig:       TRIGGER_KIND_PREFIX,
	pg_query.AlterTableType_AT_EnableRule:        RULE_KIND_PREFIX,
	pg_query.AlterTableType_AT_EnableAlwaysRule:  RULE_KIND_PREFIX,
	pg_query.AlterTableType_AT_EnableReplicaRule: RULE_KIND_PREFIX,
	pg_query.AlterTableType_AT_DisableRule:       RULE_KIND_PREFIX,

	// Index operations
	pg_query.AlterTableType_AT_ClusterOn: INDEX_KIND_PREFIX,
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

		// To Handle partitioning specification if present
		cs, ok := queryparser.ProtoAsCreateStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateStmt, got %T", msg.Interface())
		}

		// Handle partitioning if present
		if cs.Partspec != nil {
			err = a.handlePartitionSpec(cs.Partspec)
			if err != nil {
				return fmt.Errorf("anon partition spec: %w", err)
			}
		}

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

		// Handle ALTER TABLE command types that contain sensitive information
		// Check if this operation type is in our prefix map
		if prefix, exists := alterTablePrefixMap[atc.Subtype]; exists {
			atc.Name, err = a.registry.GetHash(prefix, atc.Name)
			if err != nil {
				return fmt.Errorf("anon alter table %s operation: %w", atc.Subtype, err)
			}
		}

		// Handle other operations that need custom processing
		switch atc.Subtype {
		// ─── REPLICA IDENTITY OPERATIONS (custom handling) ─────────────────────────────────────────────
		case pg_query.AlterTableType_AT_ReplicaIdentity:
			// REPLICA IDENTITY { DEFAULT | USING INDEX index_name | FULL | NOTHING }
			if atc.Def != nil {
				replicaIdentityNode := atc.Def.GetReplicaIdentityStmt()
				if replicaIdentityNode != nil {
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

		case pg_query.AlterTableType_AT_AddIdentity:
			// Handle sequence name in identity options if present
			if atc.Def == nil {
				break
			}

			constraint := atc.Def.GetConstraint()
			if constraint == nil || constraint.Options == nil {
				break
			}

			for _, opt := range constraint.Options {
				if opt == nil || opt.GetDefElem() == nil {
					continue
				}

				defElem := opt.GetDefElem()
				if defElem.Defname != "sequence_name" || defElem.Arg == nil || defElem.Arg.GetList() == nil {
					continue
				}

				err = a.anonymizeStringNodes(defElem.Arg.GetList().Items, SEQUENCE_KIND_PREFIX)
				if err != nil {
					return fmt.Errorf("anon identity sequence name: %w", err)
				}
			}

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

		/*
			SQL: ALTER TABLE ONLY sales.orders ATTACH PARTITION sales.orders_2024 FOR VALUES FROM (55) TO (56)
			ParseTree: stmt:{alter_table_stmt:{relation:{schemaname:"sales"  relname:"orders"  relpersistence:"p"  }
						cmds:{alter_table_cmd:{subtype:AT_AttachPartition  def:{partition_cmd:{name:{schemaname:"sales"  relname:"orders_2024"  inh:true  relpersistence:"p"  }
						bound:{strategy:"r"  lowerdatums:{a_const:{ival:{ival:55}  }}  upperdatums:{a_const:{ival:{ival:56}  }}  }}}  behavior:DROP_RESTRICT}}  objtype:OBJECT_TABLE}}
		*/
		case pg_query.AlterTableType_AT_AttachPartition:
			// ATTACH PARTITION partition_table FOR VALUES ...
			// Partition table name is in atc.Def as PartitionCmd, which contains RangeVar
			if atc.Def == nil {
				break
			}

			partitionCmd := atc.Def.GetPartitionCmd()
			if partitionCmd == nil {
				return fmt.Errorf("expected PartitionCmd for AT_AttachPartition")
			}

			// Anonymize the partition table name (RangeVar)
			if partitionCmd.Name != nil {
				err = a.anonymizeRangeVarNode(partitionCmd.Name, TABLE_KIND_PREFIX)
				if err != nil {
					return fmt.Errorf("anon attach partition table name: %w", err)
				}
			}

			// Anonymize the bound values as constants
			if partitionCmd.Bound == nil {
				break
			}

			// Handle lower bound
			if partitionCmd.Bound.Lowerdatums != nil {
				for _, datum := range partitionCmd.Bound.Lowerdatums {
					if datum == nil {
						continue
					}
					err = a.anonymizePartitionBoundValue(datum)
					if err != nil {
						return fmt.Errorf("anon partition lower bound: %w", err)
					}
				}
			}

			// Handle upper bound
			if partitionCmd.Bound.Upperdatums != nil {
				for _, datum := range partitionCmd.Bound.Upperdatums {
					if datum == nil {
						continue
					}
					err = a.anonymizePartitionBoundValue(datum)
					if err != nil {
						return fmt.Errorf("anon partition upper bound: %w", err)
					}
				}
			}

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

// handlePartitionSpec processes the partitioning specification in CREATE TABLE statements
/*
	SQL:		CREATE TABLE sales.orders (id int PRIMARY KEY, amt numeric) PARTITION BY LIST(region);
	ParseTree:	stmt:{create_stmt:{relation:{schemaname:"sales" relname:"orders" inh:true relpersistence:"p" location:13}
				table_elts:{column_def:{colname:"id" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"int4"}} ...} is_local:true constraints:{constraint:{contype:CONSTR_PRIMARY ...}} }}
				table_elts:{column_def:{colname:"amt" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"numeric"}} ...} is_local:true location:47}}
				partspec:{partparams:{partelem:{name:"region" ...}} ...}}} oncommit:ONCOMMIT_NOOP}}
*/
func (a *SqlAnonymizer) handlePartitionSpec(partspec *pg_query.PartitionSpec) error {
	if partspec == nil || partspec.PartParams == nil {
		return nil
	}

	for _, partParam := range partspec.PartParams {
		if partParam == nil {
			continue
		}

		// Extract the partition element
		partElem := partParam.GetPartitionElem()
		if partElem == nil || partElem.Name == "" {
			continue
		}

		// Anonymize the column name used in partitioning
		// Note: it will always be column name, not qualified name here
		hashedName, err := a.registry.GetHash(COLUMN_KIND_PREFIX, partElem.Name)
		if err != nil {
			return fmt.Errorf("anon partition column: %w", err)
		}
		partElem.Name = hashedName
	}

	return nil
}

// anonymizePartitionBoundValue anonymizes partition bound values (constants) in partition commands
func (a *SqlAnonymizer) anonymizePartitionBoundValue(datum *pg_query.Node) error {
	if datum == nil {
		return nil
	}

	// Handle constants
	aConst := datum.GetAConst()
	if aConst == nil {
		return nil
	}

	// Handle integer constants
	if ival := aConst.GetIval(); ival != nil {
		// Convert integer to string for hashing
		intVal := fmt.Sprintf("%d", ival.Ival)
		hashedVal, err := a.registry.GetHash(CONST_KIND_PREFIX, intVal)
		if err != nil {
			return fmt.Errorf("anon partition bound int value: %w", err)
		}
		// Replace the integer value node with the string node for anonymized value
		aConst.Val = &pg_query.A_Const_Sval{
			Sval: &pg_query.String{Sval: hashedVal},
		}
		return nil
	}

	// Handle string constants
	if sval := aConst.GetSval(); sval != nil {
		hashedVal, err := a.registry.GetHash(CONST_KIND_PREFIX, sval.Sval)
		if err != nil {
			return fmt.Errorf("anon partition bound string value: %w", err)
		}
		sval.Sval = hashedVal
		return nil
	}

	return nil
}

func (a *SqlAnonymizer) handleIndexObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
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

		idx.Idxname, err = a.registry.GetHash(INDEX_KIND_PREFIX, idx.Idxname)
		if err != nil {
			return fmt.Errorf("anon idxname: %w", err)
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
		ie.Indexcolname, err = a.registry.GetHash(ALIAS_KIND_PREFIX, ie.Indexcolname)
		if err != nil {
			return fmt.Errorf("anon index column alias: %w", err)
		}

		// 4) Operator class: (Opclass != nil)
		//    CREATE INDEX ... ON tbl(col opclass_schema.opclass_name(opts));
		if ie.Opclass != nil {
			err = a.anonymizeIndexOpclass(ie.Opclass)
			if err != nil {
				return fmt.Errorf("anon index opclass: %w", err)
			}
		}

		// 5) Operator class options: (Opclassopts != nil)
		//    CREATE INDEX ... ON tbl(col opclass_name(siglen='32'));
		if ie.Opclassopts != nil {
			err = a.anonymizeIndexOpclassOptions(ie.Opclassopts)
			if err != nil {
				return fmt.Errorf("anon index opclass options: %w", err)
			}
		}

	}

	return nil
}

// anonymizeIndexOpclass anonymizes operator class names in index definitions
func (a *SqlAnonymizer) anonymizeIndexOpclass(opclass []*pg_query.Node) error {
	if opclass == nil {
		return nil
	}

	// Handle operator class names (usually 1-2 elements: [schema, opclass_name])
	for _, op := range opclass {
		if op == nil {
			continue
		}

		// Extract string value from the operator class node
		if strNode := op.GetString_(); strNode != nil {
			hashedName, err := a.registry.GetHash(OPCLASS_KIND_PREFIX, strNode.Sval)
			if err != nil {
				return fmt.Errorf("anon opclass name: %w", err)
			}
			strNode.Sval = hashedName
		}
	}

	return nil
}

// anonymizeIndexOpclassOptions anonymizes operator class options in index definitions
func (a *SqlAnonymizer) anonymizeIndexOpclassOptions(opts []*pg_query.Node) error {
	if opts == nil {
		return nil
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		// Handle DefElem nodes (e.g., siglen='32')
		if defElem := opt.GetDefElem(); defElem != nil {
			// Anonymize the option name if needed
			if defElem.Defname != "" {
				hashedName, err := a.registry.GetHash(PARAMETER_KIND_PREFIX, defElem.Defname)
				if err != nil {
					return fmt.Errorf("anon opclass option name: %w", err)
				}
				defElem.Defname = hashedName
			}

			// Handle the argument value (e.g., '32')
			if defElem.Arg != nil {
				if strNode := defElem.Arg.GetString_(); strNode != nil {
					hashedVal, err := a.registry.GetHash(CONST_KIND_PREFIX, strNode.Sval)
					if err != nil {
						return fmt.Errorf("anon opclass option value: %w", err)
					}
					strNode.Sval = hashedVal
				}
			}
		}
	}

	return nil
}

func (a *SqlAnonymizer) handlePolicyObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	/*
		SQL:		CREATE POLICY user_policy ON sales.orders FOR ALL TO authenticated_users USING (user_id = current_user_id());
		ParseTree:	stmt:{create_policy_stmt:{policy_name:"user_policy"  table:{schemaname:"sales"  relname:"orders"  inh:true  relpersistence:"p"  location:29}  cmd_name:"all"  permissive:true  roles:{role_spec:{roletype:ROLESPEC_CSTRING  rolename:"authenticated_users"  location:53}}  qual:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"="}}  lexpr:{column_ref:{fields:{string:{sval:"user_id"}}  l
	*/
	case queryparser.PG_QUERY_CREATE_POLICY_STMT_NODE:
		ps, ok := queryparser.ProtoAsCreatePolicyStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreatePolicyStmt, got %T", msg.Interface())
		}

		// Policy name is always unqualified
		ps.PolicyName, err = a.registry.GetHash(POLICY_KIND_PREFIX, ps.PolicyName)
		if err != nil {
			return fmt.Errorf("anon policy name: %w", err)
		}
	}
	return nil
}

// handleGenericRenameStmt processes RENAME statements for any object type
/*
   SQL:		ALTER SCHEMA sales RENAME TO sales_new
   ParseTree:	stmt:{rename_stmt:{rename_type:OBJECT_SCHEMA  relation_type:OBJECT_ACCESS_METHOD  subname:"sales"  newname:"sales_new" ...}}

*/
func (a *SqlAnonymizer) handleGenericRenameStmt(rs *pg_query.RenameStmt) (err error) {
	if rs == nil {
		return nil
	}

	prefix := GetObjectTypePrefix(rs.RenameType)

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
		rs.Subname, err = a.registry.GetHash(prefix, rs.Subname)
		if err != nil {
			return fmt.Errorf("anon rename %s: %w", rs.RenameType, err)
		}

	case pg_query.ObjectType_OBJECT_COLLATION, pg_query.ObjectType_OBJECT_TYPE,
		pg_query.ObjectType_OBJECT_DOMAIN, pg_query.ObjectType_OBJECT_CONVERSION:
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
			str.Sval, err = a.registry.GetHash(prefix, str.Sval)
			if err != nil {
				return fmt.Errorf("anon rename object: %w", err)
			}
		}
	}

	// Always anonymize the new name
	rs.Newname, err = a.registry.GetHash(prefix, rs.Newname)
	if err != nil {
		return fmt.Errorf("anon rename newname: %w", err)
	}

	return nil
}

// handleGenericDropStmt processes DROP statements for any object type
func (a *SqlAnonymizer) handleGenericDropStmt(ds *pg_query.DropStmt) (err error) {
	if ds == nil || len(ds.Objects) == 0 {
		return nil
	}

	prefix := GetObjectTypePrefix(ds.RemoveType)

	// Process each object in the DROP statement
	for _, obj := range ds.Objects {
		if obj == nil {
			continue
		}

		// Handle different object name patterns based on object type
		switch ds.RemoveType {
		case pg_query.ObjectType_OBJECT_SCHEMA:
			// Schema names are simple strings
			if str := obj.GetString_(); str != nil {
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
			} else if str := obj.GetString_(); str != nil {
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

// handleGenericAlterObjectSchemaStmt processes ALTER ... SET SCHEMA statements for TABLE, EXTENSION, TYPE, SEQUENCE, and CONVERSION object types
func (a *SqlAnonymizer) handleGenericAlterObjectSchemaStmt(aos *pg_query.AlterObjectSchemaStmt) (err error) {
	if aos == nil {
		return nil
	}

	prefix := GetObjectTypePrefix(aos.ObjectType)

	// Handle TABLE, EXTENSION, TYPE, and SEQUENCE
	switch aos.ObjectType {
	case pg_query.ObjectType_OBJECT_TABLE, pg_query.ObjectType_OBJECT_SEQUENCE:
		// Tables, Sequences use RangeVar for their names
		if aos.Relation != nil {
			err = a.anonymizeRangeVarNode(aos.Relation, prefix)
			if err != nil {
				return fmt.Errorf("anon alter table schema: %w", err)
			}
		}

	case pg_query.ObjectType_OBJECT_EXTENSION:
		// Extensions have simple string names in Object field
		if aos.Object != nil && aos.Object.GetString_() != nil {
			str := aos.Object.GetString_()
			str.Sval, err = a.registry.GetHash(DEFAULT_KIND_PREFIX, str.Sval) // No specific prefix for extensions
			if err != nil {
				return fmt.Errorf("anon alter extension schema: %w", err)
			}
		}

	case pg_query.ObjectType_OBJECT_TYPE:
		// Types have qualified names in Object as a list
		if aos.Object != nil && aos.Object.GetList() != nil {
			items := aos.Object.GetList().Items
			err = a.anonymizeStringNodes(items, TYPE_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon alter type schema: %w", err)
			}
		}

	/*
		SQL:    ALTER CONVERSION sales.my_conversion SET SCHEMA public;
		ParseTree: stmt:{alter_object_schema_stmt:{object_type:OBJECT_CONVERSION object:{list:{items:{string:{sval:"sales"}} items:{string:{sval:"my_conversion"}}}} newschema:"public"}}
	*/
	case pg_query.ObjectType_OBJECT_CONVERSION:
		// Conversions have qualified names in Object as a list
		if aos.Object != nil && aos.Object.GetList() != nil {
			items := aos.Object.GetList().Items
			err = a.anonymizeStringNodes(items, CONVERSION_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon alter conversion schema: %w", err)
			}
		}
	}

	// Always anonymize the new schema name
	aos.Newschema, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, aos.Newschema)
	if err != nil {
		return fmt.Errorf("anon alter object schema newschema: %w", err)
	}

	return nil
}

func (a *SqlAnonymizer) handleCommentObjectNodes(msg protoreflect.Message) (err error) {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_COMMENT_STMT_NODE {
		return nil
	}
	cs, ok := queryparser.ProtoAsCommentStmtNode(msg)
	if !ok {
		return fmt.Errorf("expected CommentStmt, got %T", msg.Interface())
	}

	/*
		SQL: COMMENT ON TABLE sales.orders IS 'Customer order information';
		ParseTree: stmt:{comment_stmt:{objtype:OBJECT_TABLE object:{list:{items:{string:{sval:"sales"}}
			items:{string:{sval:"orders"}}}} comment:"Customer order information"}}
	*/

	// Handle different object types based on their parse tree structure
	switch cs.Objtype {
	case pg_query.ObjectType_OBJECT_FUNCTION, pg_query.ObjectType_OBJECT_PROCEDURE:
		// Functions/procedures use ObjectWithArgs structure
		if objWithArgs := cs.Object.GetObjectWithArgs(); objWithArgs != nil {
			prefix := GetObjectTypePrefix(cs.Objtype)
			err = a.anonymizeStringNodes(objWithArgs.Objname, prefix)
			if err != nil {
				return fmt.Errorf("anon comment function/procedure: %w", err)
			}
		}

	case pg_query.ObjectType_OBJECT_TYPE, pg_query.ObjectType_OBJECT_DOMAIN:
		// TYPE and DOMAIN use TypeName structure
		if typeName := cs.Object.GetTypeName(); typeName != nil {
			prefix := GetObjectTypePrefix(cs.Objtype)
			if err = a.anonymizeStringNodes(typeName.Names, prefix); err != nil {
				return fmt.Errorf("anon comment typename: %w", err)
			}
		}

	case pg_query.ObjectType_OBJECT_COLUMN:
		// COLUMN uses column reference logic
		if list := cs.Object.GetList(); list != nil {
			err = a.anonymizeColumnRefNode(list.Items)
			if err != nil {
				return fmt.Errorf("anon comment column: %w", err)
			}
		}

	case pg_query.ObjectType_OBJECT_TABCONSTRAINT, pg_query.ObjectType_OBJECT_TRIGGER, pg_query.ObjectType_OBJECT_POLICY:
		// Table-scoped objects: CONSTRAINT/TRIGGER/POLICY object_name ON schema.table
		if list := cs.Object.GetList(); list != nil && len(list.Items) >= 3 {
			// [object_name, schema, table]
			objectPrefix := GetObjectTypePrefix(cs.Objtype)
			err = a.anonymizeStringNodes(list.Items[:1], objectPrefix)
			if err != nil {
				return fmt.Errorf("anon comment %s name: %w", cs.Objtype, err)
			}
			err = a.anonymizeStringNodes(list.Items[1:], TABLE_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon comment %s table: %w", cs.Objtype, err)
			}
		}

	default:
		// Handle all other object types
		if list := cs.Object.GetList(); list != nil {
			// Most objects use qualified names: [schema, object] or [db, schema, object]
			prefix := GetObjectTypePrefix(cs.Objtype)
			if err := a.anonymizeStringNodes(list.Items, prefix); err != nil {
				return fmt.Errorf("anon comment object: %w", err)
			}
		} else if str := cs.Object.GetString_(); str != nil {
			// Simple unqualified names (SCHEMA, DATABASE, EXTENSION, ROLE)
			prefix := GetObjectTypePrefix(cs.Objtype)
			str.Sval, err = a.registry.GetHash(prefix, str.Sval)
			if err != nil {
				return fmt.Errorf("anon comment object: %w", err)
			}
		}
	}

	// Anonymize the comment text itself (if present)
	cs.Comment, err = a.registry.GetHash(CONST_KIND_PREFIX, cs.Comment)
	if err != nil {
		return fmt.Errorf("anon comment text: %w", err)
	}
	return nil
}

func (a *SqlAnonymizer) handleConversionObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	/*
		SQL:		CREATE CONVERSION conversion_example.myconv FOR 'LATIN1' TO 'UTF8' FROM iso8859_1_to_utf8;
		ParseTree:	stmt:{create_conversion_stmt:{conversion_name:{string:{sval:"conversion_example"}} conversion_name:{string:{sval:"myconv"}}
					for_encoding_name:{string:{sval:"LATIN1"}} to_encoding_name:{string:{sval:"UTF8"}} func_name:{string:{sval:"iso8859_1_to_utf8"}}}}

		SQL:		CREATE CONVERSION sales.my_conversion FOR 'LATIN1' TO 'UTF8' FROM latin1_to_utf8;
		ParseTree:	stmt:{create_conversion_stmt:{conversion_name:{string:{sval:"sales"}}  conversion_name:{string:{sval:"my_conversion"}}
					for_encoding_name:"LATIN1"  to_encoding_name:"UTF8"  func_name:{string:{sval:"latin1_to_utf8"}}}}
	*/
	case queryparser.PG_QUERY_CREATE_CONVERSION_STMT_NODE:
		ccs, ok := queryparser.ProtoAsCreateConversionStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateConversionStmt, got %T", msg.Interface())
		}

		// Anonymize the conversion name (fully qualified: schema.conversion_name)
		err = a.anonymizeStringNodes(ccs.GetConversionName(), CONVERSION_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon conversion name: %w", err)
		}

		// Anonymize the function name if present
		// The function name is typically a list of nodes like the conversion name
		if ccs.FuncName != nil {
			err = a.anonymizeStringNodes(ccs.FuncName, FUNCTION_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon conversion function name: %w", err)
			}
		}

	}
	return nil
}

// handleForeignTableObjectNodes processes ForeignTable nodes.
// Foreign tables have a base_stmt with a relation field, which is a RangeVar.
// We need to anonymize the relation (table name) with FOREIGN_TABLE_KIND_PREFIX instead of TABLE_KIND_PREFIX.
/*
	SQL:    CREATE FOREIGN TABLE sales.external_orders (id int, customer_id int, amount numeric) SERVER external_server OPTIONS (schema_name 'public', table_name 'orders');
			ParseTree: stmt:{create_foreign_table_stmt:{base_stmt:{relation:{schemaname:"sales" relname:"external_orders" }
			table_elts:{column_def:{colname:"id" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"int4"}}}}}
			table_elts:{column_def:{colname:"customer_id" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"int4"}}}}}
			table_elts:{column_def:{colname:"amount" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"numeric"}} } }} oncommit:ONCOMMIT_NOOP}
			servername:"external_server" options:{def_elem:{defname:"schema_name" arg:{string:{sval:"public"}} defaction:DEFELEM_UNSPEC location:117}}
			options:{def_elem:{defname:"table_name" arg:{string:{sval:"orders"}} defaction:DEFELEM_UNSPEC location:139}}}}
*/
func (a *SqlAnonymizer) handleForeignTableObjectNodes(msg protoreflect.Message) (err error) {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_CREATE_FOREIGN_TABLE_STMT_NODE {
		return nil
	}
	fts, ok := queryparser.ProtoAsCreateForeignTableStmt(msg)
	if !ok {
		return fmt.Errorf("expected CreateForeignTableStmt, got %T", msg.Interface())
	}

	// Anonymize the relation (table name) with FOREIGN_TABLE_KIND_PREFIX
	if fts.BaseStmt != nil && fts.BaseStmt.Relation != nil {
		err = a.anonymizeRangeVarNode(fts.BaseStmt.Relation, FOREIGN_TABLE_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon foreign table relation: %w", err)
		}
	}

	// Anonymize columns in BaseStmt.TableElts -> already covered by anonymizeColumnDefNode

	// Anonymize the server name if present
	fts.Servername, err = a.registry.GetHash(DEFAULT_KIND_PREFIX, fts.Servername)
	if err != nil {
		return fmt.Errorf("anon foreign table servername: %w", err)
	}

	// Anonymize the options if present
	if fts.Options == nil {
		return nil
	}

	for _, opt := range fts.Options {
		defElem := opt.GetDefElem()
		if defElem == nil {
			continue
		}

		var prefix string
		switch defElem.Defname {
		case "table_name":
			prefix = TABLE_KIND_PREFIX
		case "schema_name":
			prefix = SCHEMA_KIND_PREFIX
		default:
			prefix = DEFAULT_KIND_PREFIX
		}

		if defElem.Arg == nil || defElem.Arg.GetString_() == nil {
			continue
		}
		err = a.anonymizeStringNodes([]*pg_query.Node{defElem.Arg}, prefix)
		if err != nil {
			return fmt.Errorf("anon foreign table option %s: %w", defElem.Defname, err)
		}
	}

	return nil
}

// Note: we should disable anonymization for rules for now.
// because rule contains - SELECT, UPDATE, DELETE, INSERT DML in the DO clause.
// Once we support DML anonymization, we can enable RULE anonymization.
func (a *SqlAnonymizer) handleRuleObjectNodes(msg protoreflect.Message) (err error) {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_RULE_STMT_NODE {
		return nil
	}

	rs, ok := queryparser.ProtoAsRuleStmtNode(msg)
	if !ok {
		return fmt.Errorf("expected CreateRuleStmt, got %T", msg.Interface())
	}

	// Anonymize the rule name
	// Note: rule name is not a qualified name, its stored/attached to the table on which it is defined
	rs.Rulename, err = a.registry.GetHash(RULE_KIND_PREFIX, rs.Rulename)
	if err != nil {
		return fmt.Errorf("anon rule name: %w", err)
	}

	return nil
}

func (a *SqlAnonymizer) handleAggregateObjectNodes(msg protoreflect.Message) (err error) {
	// there is no CREATE AGGREGATE stmt node in pg_query. DefineStmt is used instead.
	// We need to anonymize the aggregate name and the function name.
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_DEFINE_STMT_NODE {
		return nil
	}

	ds, err := a.handleDefineStmtWithReturn(msg, pg_query.ObjectType_OBJECT_AGGREGATE, AGGREGATE_KIND_PREFIX)
	if err != nil {
		return fmt.Errorf("anon aggregate name: %w", err)
	}
	if ds == nil {
		return nil // not an aggregate, skip
	}

	/*
		SQL:		CREATE AGGREGATE sales.order_total(int) (SFUNC = sales.add_order, STYPE = int);
		ParseTree: 	stmt:{define_stmt:{kind:OBJECT_AGGREGATE  defnames:{string:{sval:"sales"}}  defnames:{string:{sval:"order_total"}}
					args:{list:{items:{function_parameter:{arg_type:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"int4"}} }
					mode:FUNC_PARAM_DEFAULT}}}}  args:{integer:{ival:-1}}  definition:{def_elem:{defname:"sfunc"
					arg:{type_name:{names:{string:{sval:"sales"}}  names:{string:{sval:"add_order"}}}}}}
					definition:{def_elem:{defname:"stype"  arg:{type_name:{names:{string:{sval:"pg_catalog"}}  names:{string:{sval:"int4"}}}} }}}}
	*/
	// stype can be a user-defined type, covered already in type nodes processors

	// Process function references in the definition
	if ds.Definition == nil {
		return nil
	}

	functionDefs := map[string]bool{
		"sfunc":      true,
		"finalfunc":  true,
		"msfunc":     true,
		"minvfunc":   true,
		"mfinalfunc": true,
	}

	for _, defElem := range ds.Definition {
		defElemNode := defElem.GetDefElem()
		if defElemNode == nil || !functionDefs[defElemNode.Defname] {
			continue
		}

		// Anonymize function references
		if defElemNode.Arg == nil {
			continue
		}

		typeName := defElemNode.Arg.GetTypeName()
		if typeName == nil {
			continue
		}

		err = a.anonymizeStringNodes(typeName.Names, FUNCTION_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon aggregate function %s: %w", defElemNode.Defname, err)
		}
	}

	return nil
}

func (a *SqlAnonymizer) handleOperatorObjectNodes(msg protoreflect.Message) (err error) {
	/*
		SQL:		CREATE OPERATOR sales.<# (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_lt);
		ParseTree:	stmt:{define_stmt:{kind:OBJECT_OPERATOR defnames:{string:{sval:"sales"}} defnames:{string:{sval:"<#"}}
					definition:{def_elem:{defname:"leftarg" arg:{type_name:{names:{string:{sval:"int4"}} typemod:-1 location:36}} defaction:DEFELEM_UNSPEC location:26}}
					definition:{def_elem:{defname:"rightarg" arg:{type_name:{names:{string:{sval:"int4"}} typemod:-1 location:53}} defaction:DEFELEM_UNSPEC location:42}}
					definition:{def_elem:{defname:"procedure" arg:{type_name:{names:{string:{sval:"int4_abs_lt"}} typemod:-1 location:71}} defaction:DEFELEM_UNSPEC location:59}}}}

		SQL:		CREATE OPERATOR sales.=# (LEFTARG = int4, RIGHTARG = int4, PROCEDURE = int4_abs_eq, COMMUTATOR = =#);
		ParseTree:	stmt:{define_stmt:{kind:OBJECT_OPERATOR defnames:{string:{sval:"sales"}} defnames:{string:{sval:"=#"}}
					definition:{def_elem:{defname:"leftarg" arg:{type_name:{names:{string:{sval:"int4"}} typemod:-1 location:36}} defaction:DEFELEM_UNSPEC location:26}}
					definition:{def_elem:{defname:"rightarg" argq:{type_name:{names:{string:{sval:"int4"}} typemod:-1 location:53}} defaction:DEFELEM_UNSPEC location:42}}
					definition:{def_elem:{defname:"procedure" arg:{type_name:{names:{string:{sval:"int4_abs_eq"}} typemod:-1 location:71}} defaction:DEFELEM_UNSPEC location:59}}
					definition:{def_elem:{defname:"commutator" arg:{type_name:{names:{string:{sval:"=#"}} typemod:-1 location:88}} defaction:DEFELEM_UNSPEC location:76}}}}
	*/

	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_DEFINE_STMT_NODE {
		return nil
	}

	ds, err := a.handleDefineStmtWithReturn(msg, pg_query.ObjectType_OBJECT_OPERATOR, OPERATOR_KIND_PREFIX)
	if err != nil {
		return fmt.Errorf("anon operator name: %w", err)
	}
	if ds == nil {
		return nil // not an operator, skip
	}

	// Process definition elements
	if ds.Definition == nil {
		return nil // no definition elements, skip
	}

	for _, defElem := range ds.Definition {
		defElemNode := defElem.GetDefElem()
		if defElemNode == nil {
			continue
		}

		// Handle different definition element types
		switch defElemNode.Defname {
		case "procedure":
			// Anonymize procedure name - it's a TypeName node containing the function name
			if defElemNode.Arg == nil {
				continue
			}

			typeNameNode := defElemNode.Arg.GetTypeName()
			if typeNameNode == nil {
				continue
			}

			err = a.anonymizeStringNodes(typeNameNode.Names, FUNCTION_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon operator procedure: %w", err)
			}

		case "commutator", "negator":
			// Anonymize operator references in lists
			if defElemNode.Arg == nil {
				continue
			}

			listNode := defElemNode.Arg.GetList()
			if listNode == nil {
				continue
			}

			err = a.anonymizeStringNodes(listNode.Items, OPERATOR_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon operator commutator/negator: %w", err)
			}

		case "restrict", "join":
			// Anonymize function names - they're also TypeName nodes
			if defElemNode.Arg == nil {
				continue
			}

			typeNameNode := defElemNode.Arg.GetTypeName()
			if typeNameNode == nil {
				continue
			}

			err = a.anonymizeStringNodes(typeNameNode.Names, FUNCTION_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon operator restrict/join function: %w", err)
			}
		}
	}

	return nil
}

func (a *SqlAnonymizer) handleOperatorClassAndFamilyObjectNodes(msg protoreflect.Message) (err error) {
	switch queryparser.GetMsgFullName(msg) {
	/*
		SQL:		CREATE OPERATOR CLASS sales.int4_abs_ops FOR TYPE int4 USING btree AS OPERATOR 1 <#, OPERATOR 2 <=#, OPERATOR 3 =#, OPERATOR 4 >=#, OPERATOR 5 >#, FUNCTION 1 int4_abs_cmp(int4,int4);
		ParseTree:	stmt:{create_op_class_stmt:{opclassname:{string:{sval:"sales"}} opclassname:{string:{sval:"int4_abs_ops"}} amname:"btree" datatype:{names:{string:{sval:"int4"}} typemod:-1 location:50}
					items:{create_op_class_item:{itemtype:1 name:{objname:{string:{sval:"<#"}}} number:1}} items:{create_op_class_item:{itemtype:1 name:{objname:{string:{sval:"<=#"}}} number:2}}
					items:{create_op_class_item:{itemtype:1 name:{objname:{string:{sval:"=#"}}} number:3}} items:{create_op_class_item:{itemtype:1 name:{objname:{string:{sval:">=#"}}} number:4}}
					items:{create_op_class_item:{itemtype:1 name:{objname:{string:{sval:">#"}}} number:5}} items:{create_op_class_item:{itemtype:2 name:{objname:{string:{sval:"int4_abs_cmp"}}
					objargs:{type_name:{names:{string:{sval:"int4"}} typemod:-1 location:171}} objargs:{type_name:{names:{string:{sval:"int4"}} typemod:-1 location:176}}
					objfuncargs:{function_parameter:{arg_type:{names:{string:{sval:"int4"}} typemod:-1 location:171} mode:FUNC_PARAM_DEFAULT}}
					objfuncargs:{function_parameter:{arg_type:{names:{string:{sval:"int4"}} typemod:-1 location:176} mode:FUNC_PARAM_DEFAULT}}} number:1}}}}

	*/
	case queryparser.PG_QUERY_CREATE_OP_CLASS_STMT_NODE:
		cocs, ok := queryparser.ProtoAsCreateOpClassStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateOpClassStmt, got %T", msg.Interface())
		}

		// Anonymize the operator class name (qualified: schema.opclass_name)
		err = a.anonymizeStringNodes(cocs.GetOpclassname(), OPCLASS_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon operator class name: %w", err)
		}

		// Anonymize the operator family name if present (qualified: schema.opfamily_name)
		if cocs.Opfamilyname != nil {
			err = a.anonymizeStringNodes(cocs.GetOpfamilyname(), OPFAMILY_KIND_PREFIX)
			if err != nil {
				return fmt.Errorf("anon operator class family name: %w", err)
			}
		}

		// Anonymize operators and functions in the items
		if cocs.Items == nil {
			return nil
		}

		for _, item := range cocs.Items {
			itemNode := item.GetCreateOpClassItem()
			if itemNode == nil {
				continue
			}

			if itemNode.Name == nil || itemNode.Name.Objname == nil {
				continue
			}

			// itemtype: 1 = operator, 2 = function
			var prefix string
			switch itemNode.Itemtype {
			case 1:
				prefix = OPERATOR_KIND_PREFIX
			case 2:
				prefix = FUNCTION_KIND_PREFIX
			default:
				prefix = DEFAULT_KIND_PREFIX
			}

			err = a.anonymizeStringNodes(itemNode.Name.Objname, prefix)
			if err != nil {
				return fmt.Errorf("anon operator class item: %w", err)
			}
		}

	/*
		SQL:		CREATE OPERATOR FAMILY sales.abs_numeric_ops USING btree;
		ParseTree:	stmt:{create_op_family_stmt:{opfamilyname:{string:{sval:"sales"}} opfamilyname:{string:{sval:"abs_numeric_ops"}} amname:"btree"}}
	*/
	case queryparser.PG_QUERY_CREATE_OP_FAMILY_STMT_NODE:
		cofs, ok := queryparser.ProtoAsCreateOpFamilyStmtNode(msg)
		if !ok {
			return fmt.Errorf("expected CreateOpFamilyStmt, got %T", msg.Interface())
		}

		// Anonymize the operator family name (qualified: schema.opfamily_name)
		err = a.anonymizeStringNodes(cofs.GetOpfamilyname(), OPFAMILY_KIND_PREFIX)
		if err != nil {
			return fmt.Errorf("anon operator family name: %w", err)
		}
	}

	return nil
}

// ========================= Anonymization Helpers =========================

// handleDefineStmtWithReturn is a common handler for DefineStmt nodes that can handle different object types
// based on the provided object type and prefix. Returns the DefineStmt for additional processing if needed.
func (a *SqlAnonymizer) handleDefineStmtWithReturn(msg protoreflect.Message, expectedObjectType pg_query.ObjectType, objectPrefix string) (*pg_query.DefineStmt, error) {
	// caller should check this but adding it here for safety
	ds, ok := queryparser.ProtoAsDefineStmtNode(msg)
	if !ok {
		return nil, fmt.Errorf("expected DefineStmt, got %T", msg.Interface())
	}

	if ds.Kind != expectedObjectType {
		return nil, nil // not the expected object type, skip
	}

	// Anonymize the object name
	err := a.anonymizeStringNodes(ds.Defnames, objectPrefix)
	if err != nil {
		return nil, fmt.Errorf("anon %s name: %w", expectedObjectType, err)
	}

	return ds, nil
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
		if s := n.GetString_(); s != nil {
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
	rv.Catalogname, err = a.registry.GetHash(DATABASE_KIND_PREFIX, rv.Catalogname)
	if err != nil {
		return err
	}
	rv.Schemaname, err = a.registry.GetHash(SCHEMA_KIND_PREFIX, rv.Schemaname)
	if err != nil {
		return err
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

	// Handle constraints in ColumnDef (like CONSTR_DEFAULT)
	// Examples of default values for columns:
	// 1. CREATE TABLE sales.products (id int DEFAULT 1)
	// 2. CREATE TABLE sales.products (id int DEFAULT 1, name text DEFAULT 'product')
	// 3. CREATE TABLE sales.products (id int DEFAULT 1, name text DEFAULT 'product', price numeric DEFAULT 0.00, active boolean DEFAULT true)
	if cd.Constraints == nil {
		return nil
	}

	for _, constraintNode := range cd.Constraints {
		if constraintNode == nil {
			continue
		}

		// Get the constraint from the node
		constraint := constraintNode.GetConstraint()
		if constraint == nil || constraint.Contype != pg_query.ConstrType_CONSTR_DEFAULT || constraint.RawExpr == nil {
			continue
		}

		// Handle A_Const nodes (literal values)
		aConst := constraint.RawExpr.GetAConst()
		if aConst == nil || aConst.Val == nil {
			continue
		}

		// Handle string literals in default values
		if sval := aConst.GetSval(); sval != nil {
			sval.Sval, err = a.registry.GetHash(CONST_KIND_PREFIX, sval.Sval)
			if err != nil {
				return fmt.Errorf("anon coldef default string value: %w", err)
			}
			continue
		}

		// Handle integer literals in default values
		if ival := aConst.GetIval(); ival != nil {
			// Convert integer to string for hashing
			intVal := fmt.Sprintf("%d", ival.Ival)
			hashedVal, err := a.registry.GetHash(CONST_KIND_PREFIX, intVal)
			if err != nil {
				return fmt.Errorf("anon coldef default int value: %w", err)
			}
			// Replace the integer value node with the string node for anonymized value
			aConst.Val = &pg_query.A_Const_Sval{
				Sval: &pg_query.String{Sval: hashedVal},
			}
			continue
		}

		// Handle float literals in default values
		if fval := aConst.GetFval(); fval != nil {
			// Get the float value as string and hash it
			floatVal := fval.Fval
			hashedVal, err := a.registry.GetHash(CONST_KIND_PREFIX, floatVal)
			if err != nil {
				return fmt.Errorf("anon coldef default float value: %w", err)
			}
			// Replace the float value node with the string node for anonymized value
			aConst.Val = &pg_query.A_Const_Sval{
				Sval: &pg_query.String{Sval: hashedVal},
			}
			continue
		}

		// Handle boolean literals in default values
		if boolval := aConst.GetBoolval(); boolval != nil {
			// Get the boolean value as string and hash it
			boolVal := boolval.String()
			hashedVal, err := a.registry.GetHash(CONST_KIND_PREFIX, boolVal)
			if err != nil {
				return fmt.Errorf("anon coldef default boolean value: %w", err)
			}
			// Replace the boolean value node with the string node for anonymized value
			aConst.Val = &pg_query.A_Const_Sval{
				Sval: &pg_query.String{Sval: hashedVal},
			}
			continue
		}
	}

	return nil
}
