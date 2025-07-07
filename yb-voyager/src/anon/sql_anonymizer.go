package anon

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	SCHEMA_KIND_PREFIX     = "schema_"
	TABLE_KIND_PREFIX      = "table_"
	COLUMN_KIND_PREFIX     = "col_"
	INDEX_KIND_PREFIX      = "index_"
	CONSTRAINT_KIND_PREFIX = "constraint_"
	ALIAS_KIND_PREFIX      = "alias_"
	TYPE_KIND_PREFIX       = "type_"
	ROLE_KIND_PREFIX       = "role_"
	CONST_KIND_PREFIX      = "const_"
	DEFAULT_KIND_PREFIX    = "anon_"           // fallback for any other identifiers
	SALT_KEY_METADB        = "anonymizer_salt" // Key to store salt in MetaDB MSR for consistent anonymization across runs
	SALT_SIZE              = 16                // Size of salt in bytes, can be adjusted as needed
)

type Anonymizer interface {
	Anonymize(input string) (string, error)
}

type SqlAnonymizer struct {
	registry TokenRegistry
}

func NewSqlAnonymizer(registry TokenRegistry) Anonymizer {
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

func (a *SqlAnonymizer) anonymizationProcessor(msg protoreflect.Message) error {
	// three categories of nodes: Identifier nodes, literal nodes, Misc nodes for rest
	a.identifierNodesProcessor(msg)
	a.literalNodesProcessor(msg)
	a.miscellaneousNodesProcessor(msg)
	return nil
}

func (a *SqlAnonymizer) identifierNodesProcessor(msg protoreflect.Message) error {
	var err error
	switch queryparser.GetMsgFullName(msg) {
	case queryparser.PG_QUERY_RANGEVAR_NODE:
		rv, err := queryparser.ProtoAsRangeVarNode(msg)
		if err != nil {
			return fmt.Errorf("cast to RangeVar: %w", err)
		}
		if rv.Schemaname != "" {
			rv.Schemaname, err = a.registry.Token(SCHEMA_KIND_PREFIX, rv.Schemaname)
			if err != nil {
				return fmt.Errorf("anon schema: %w", err)
			}
		}
		rv.Relname, err = a.registry.Token(TABLE_KIND_PREFIX, rv.Relname)
		if err != nil {
			return fmt.Errorf("anon table: %w", err)
		}

	case queryparser.PG_QUERY_COLUMNDEF_NODE:
		cd, ok := queryparser.ProtoAsColumnDef(msg)
		if !ok {
			return fmt.Errorf("expected ColumnDef, got %T", msg.Interface())
		}
		cd.Colname, err = a.registry.Token(COLUMN_KIND_PREFIX, cd.Colname)
		if err != nil {
			return fmt.Errorf("anon coldef: %w", err)
		}

	case queryparser.PG_QUERY_COLUMNREF_NODE:
		cr, ok := queryparser.ProtoAsColumnRef(msg)
		if !ok {
			return fmt.Errorf("expected ColumnRef, got %T", msg.Interface())
		}

		// For each field (could be schema, table, or column name), see if it has a String node
		for i, node := range cr.Fields {
			str := node.GetString_()
			if str == nil || str.Sval == "" {
				continue
			}

			str.Sval, err = a.registry.Token(COLUMN_KIND_PREFIX, str.Sval)
			if err != nil {
				return fmt.Errorf("anon colref[%d]=%q lookup: %w", i, str.Sval, err)
			}
		}

	case queryparser.PG_QUERY_RESTARGET_NODE:
		rt, ok := queryparser.ProtoAsResTargetNode(msg)
		if !ok {
			return fmt.Errorf("expected ResTarget, got %T", msg.Interface())
		}
		if rt.Name != "" {
			rt.Name, err = a.registry.Token(ALIAS_KIND_PREFIX, rt.Name)
			if err != nil {
				return fmt.Errorf("anon alias: %w", err)
			}
		}

	case queryparser.PG_QUERY_INDEX_STMT_NODE:
		idx, err := queryparser.ProtoAsIndexStmtNode(msg)
		if err != nil {
			return err
		}

		if idx.Idxname != "" {
			idx.Idxname, err = a.registry.Token(INDEX_KIND_PREFIX, idx.Idxname)
			if err != nil {
				return fmt.Errorf("anon idxname: %w", err)
			}
		}
		idx.Relation.Relname, err = a.registry.Token(TABLE_KIND_PREFIX, idx.Relation.Relname)
		if err != nil {
			return fmt.Errorf("anon idx table: %w", err)
		}

	case queryparser.PG_QUERY_INDEXELEM_NODE:
		ie, err := queryparser.ProtoAsIndexElemNode(msg)
		if err != nil {
			return err
		}
		if ie.Name != "" {
			ie.Name, err = a.registry.Token(COLUMN_KIND_PREFIX, ie.Name)
			if err != nil {
				return fmt.Errorf("anon index column %q: %w", ie.Name, err)
			}
		}

	/*
		SQL:		ALTER TABLE ONLY public.foo ADD CONSTRAINT unique_1 UNIQUE (column1, column2) DEFERRABLE;
		ParseTree:	stmt:{alter_table_stmt:{relation:{schemaname:"public" relname:"foo" relpersistence:"p" } cmds:{alter_table_cmd:{subtype:AT_AddConstraint
					def:{constraint:{contype:CONSTR_UNIQUE conname:"unique_1" deferrable:true location:32 keys:{string:{sval:"column1"}}
					keys:{string:{sval:"column2"}}}} behavior:DROP_RESTRICT}} ...}}
	*/
	case queryparser.PG_QUERY_CONSTRAINT_NODE:
		cons, err := queryparser.ProtoAsTableConstraintNode(msg)
		if err != nil {
			return err
		}
		if cons.Conname != "" {
			cons.Conname, err = a.registry.Token(CONSTRAINT_KIND_PREFIX, cons.Conname)
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
				key.GetString_().Sval, err = a.registry.Token(COLUMN_KIND_PREFIX, colName)
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
					alterTableCmdNode.Name, err = a.registry.Token(CONSTRAINT_KIND_PREFIX, name)
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
			alias.Aliasname, err = a.registry.Token(ALIAS_KIND_PREFIX, alias.Aliasname)
			if err != nil {
				return fmt.Errorf("anon aliasnode: %w", err)
			}
		}

	/*
		SQL: CREATE TABLE foo(id my_custom_type);
		ParseTree: stmt:{create_stmt:{relation:{relname:"foo"  inh:true  ...}
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
			str := node.GetString_() // returns *pg_query.String or nil
			if str == nil || str.Sval == "" {
				continue
			}
			str.Sval, err = a.registry.Token(TYPE_KIND_PREFIX, str.Sval)
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

		rs.Rolename, err = a.registry.Token(ROLE_KIND_PREFIX, rs.Rolename)
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
			tok, err := a.registry.Token(CONST_KIND_PREFIX, ac.GetSval().Sval)
			if err != nil {
				return fmt.Errorf("anon A_Const: %w", err)
			}
			ac.GetSval().Sval = tok
		}

	}

	return nil
}

func (a *SqlAnonymizer) miscellaneousNodesProcessor(msg protoreflect.Message) error {
	switch queryparser.GetMsgFullName(msg) {

	/*
		SQL: 		CREATE EXTENSION IF NOT EXISTS postgis SCHEMA public;
		ParseTree: 	stmt:{create_extension_stmt:{extname:"postgis"  if_not_exists:true
					options:{def_elem:{defname:"schema"  arg:{string:{sval:"public"}}  defaction:DEFELEM_UNSPEC ...}}}}
	*/
	case queryparser.PG_QUERY_CREATE_EXTENSION_STMT:
		ces, ok := queryparser.ProtoAsCreateExtensionStmt(msg)
		if !ok {
			return fmt.Errorf("expected CreateExtensionStmt, got %T", msg.Interface())
		}

		// remove schema name from extension def
		for _, opt := range ces.Options {
			if opt.GetDefElem() != nil && opt.GetDefElem().GetDefname() == "schema" {
				// Anonymize the schema name
				opt.GetDefElem().Arg.GetString_().Sval, _ = a.registry.Token(SCHEMA_KIND_PREFIX, opt.GetDefElem().Arg.GetString_().Sval)
			}
		}
	}

	return nil
}
