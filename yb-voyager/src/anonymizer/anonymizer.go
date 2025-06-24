package anonymizer

import (
	"fmt"

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
	DEFAULT_KIND_PREFIX    = "anon_" // fallback for any other identifiers
)

type Anonymizer interface {
	Anonymize(input string) (string, error)
}

type SqlAnonymizer struct {
	store     *AnonymizerStore
	exportDir string
}

func NewSqlAnonymizer(exportDir string) (*SqlAnonymizer, error) {
	store, err := NewAnonymizerStore(exportDir)
	if err != nil {
		return nil, fmt.Errorf("error creating anonymizer store: %w", err)
	}

	err = store.Init()
	if err != nil {
		return nil, fmt.Errorf("error initializing anonymizer store: %w", err)
	}

	return &SqlAnonymizer{
		store:     store,
		exportDir: exportDir,
	}, nil
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

func (a *SqlAnonymizer) Close() error {
	if a.store != nil {
		return a.store.Close()
	}
	return nil
}

func (a *SqlAnonymizer) anonymizationProcessor(msg protoreflect.Message) error {
	var err error
	switch queryparser.GetMsgFullName(msg) {
	case queryparser.PG_QUERY_RANGEVAR_NODE:
		rv, err := queryparser.ProtoAsRangeVarNode(msg)
		if err != nil {
			return fmt.Errorf("cast to RangeVar: %w", err)
		}
		if rv.Schemaname != "" {
			rv.Schemaname, err = a.store.LookupOrCreate(SCHEMA_KIND_PREFIX, rv.Schemaname)
			if err != nil {
				return fmt.Errorf("anon schema: %w", err)
			}
		}
		rv.Relname, err = a.store.LookupOrCreate(TABLE_KIND_PREFIX, rv.Relname)
		if err != nil {
			return fmt.Errorf("anon table: %w", err)
		}

	case queryparser.PG_QUERY_COLUMNDEF_NODE:
		cd, ok := queryparser.ProtoAsColumnDef(msg)
		if !ok {
			return fmt.Errorf("expected ColumnDef, got %T", msg.Interface())
		}
		cd.Colname, err = a.store.LookupOrCreate(COLUMN_KIND_PREFIX, cd.Colname)
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
			str := node.GetString_() // returns *pg_query.String or nil
			if str == nil {
				continue
			}
			orig := str.Sval // the original identifier
			if orig == "" {
				continue
			}
			// Lookup (or create) the token
			tok, err := a.store.LookupOrCreate(COLUMN_KIND_PREFIX, orig)
			if err != nil {
				return fmt.Errorf("anon colref[%d]=%q lookup: %w", i, orig, err)
			}
			// Overwrite in place
			str.Sval = tok
		}
	case queryparser.PG_QUERY_RESTARGET_NODE:
		rt, ok := queryparser.ProtoAsResTargetNode(msg)
		if !ok {
			return fmt.Errorf("expected ResTarget, got %T", msg.Interface())
		}
		if rt.Name != "" {
			rt.Name, err = a.store.LookupOrCreate(ALIAS_KIND_PREFIX, rt.Name)
			if err != nil {
				return fmt.Errorf("anon alias: %w", err)
			}
		}

	case queryparser.PG_QUERY_INDEX_STMT_NODE:
		idx, err := queryparser.ProtoAsIndexStmtNode(msg)
		if err != nil {
			return err
		}
		// index name
		if idx.Idxname != "" {
			idx.Idxname, err = a.store.LookupOrCreate(INDEX_KIND_PREFIX, idx.Idxname)
			if err != nil {
				return fmt.Errorf("anon idxname: %w", err)
			}
		}
		// table name
		idx.Relation.Relname, err = a.store.LookupOrCreate(TABLE_KIND_PREFIX, idx.Relation.Relname)
		if err != nil {
			return fmt.Errorf("anon idx table: %w", err)
		}

	case queryparser.PG_QUERY_INDEXELEM_NODE:
		// Each Node whose one-of is IndexElem comes here
		ie, err := queryparser.ProtoAsIndexElemNode(msg)
		if err != nil {
			return err
		}
		if ie.Name != "" {
			ie.Name, err = a.store.LookupOrCreate(COLUMN_KIND_PREFIX, ie.Name)
			if err != nil {
				return fmt.Errorf("anon index column %q: %w", ie.Name, err)
			}
		}

	case queryparser.PG_QUERY_CONSTRAINT_NODE:
		cons, err := queryparser.ProtoAsTableConstraintNode(msg)
		if err != nil {
			return err
		}
		if cons.Conname != "" {
			cons.Conname, err = a.store.LookupOrCreate(CONSTRAINT_KIND_PREFIX, cons.Conname)
			if err != nil {
				return fmt.Errorf("anon constraint: %w", err)
			}
		}

	case queryparser.PG_QUERY_ALIAS_NODE:
		alias, ok := queryparser.ProtoAsAliasNode(msg)
		if !ok {
			return fmt.Errorf("expected Alias, got %T", msg.Interface())
		}
		if alias.Aliasname != "" {
			alias.Aliasname, err = a.store.LookupOrCreate(ALIAS_KIND_PREFIX, alias.Aliasname)
			if err != nil {
				return fmt.Errorf("anon aliasnode: %w", err)
			}
		}
	}

	return nil
}
