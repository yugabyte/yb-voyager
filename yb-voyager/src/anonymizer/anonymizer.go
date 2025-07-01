package anonymizer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
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
	DEFAULT_KIND_PREFIX    = "anon_"           // fallback for any other identifiers
	SALT_KEY_METADB        = "anonymizer_salt" // Key to store salt in MetaDB MSR for consistent anonymization across runs
	SALT_SIZE              = 16                // Size of salt in bytes, can be adjusted as needed
)

type Anonymizer interface {
	Anonymize(input string) (string, error)
}

type SqlAnonymizer struct {
	/*
		Salt for anonymization, used to ensure consistent anonymization across runs
		Importance: If not used, the generated token will be globally unique not unique per run.
		Consider generic table names like users, employees, orders etc which are common across many databases.
		So using salt makes it much more safer and making it more difficult to reverse engineer the anonymized SQL.
	*/
	salt string

	// In-memory cache to avoid repeated generation for same identifier
	// Worst‐case memory analysis:
	//   - Key string: len(kind)+len(identifier) = ~8 + 16 = 24 bytes
	//   - Value string: len(kind)+16 hex chars  = ~8 + 16 = 24 bytes
	//   - Go map overhead: ~48 bytes per entry
	// Total per entry = 24 + 24 + 48 = 104 bytes
	// For N = 10^5 entries, ~10.4 MB
	tokenMap map[string]string

	mu sync.RWMutex // Mutex to protect concurrent access to tokenMap
}

func NewSqlAnonymizer(metaDB *metadb.MetaDB) (*SqlAnonymizer, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("error getting migration status record: %w", err)
	}

	var salt string
	if msr != nil && msr.AnonymizerSalt != "" {
		salt = msr.AnonymizerSalt
	} else {
		salt, err = GenerateSalt(SALT_SIZE)
		if err != nil {
			return nil, fmt.Errorf("error generating salt: %w", err)
		}

		// Store the generated salt in the migration status record
		err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			if record == nil { // should not happen, but just in case
				record = &metadb.MigrationStatusRecord{}
			}
			record.AnonymizerSalt = salt
		})
		if err != nil {
			return nil, fmt.Errorf("error updating migration status record with salt: %w", err)
		}
	}

	return &SqlAnonymizer{
		salt:     salt,
		tokenMap: make(map[string]string),
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

func (a *SqlAnonymizer) anonymizationProcessor(msg protoreflect.Message) error {
	var err error
	switch queryparser.GetMsgFullName(msg) {
	case queryparser.PG_QUERY_RANGEVAR_NODE:
		rv, err := queryparser.ProtoAsRangeVarNode(msg)
		if err != nil {
			return fmt.Errorf("cast to RangeVar: %w", err)
		}
		if rv.Schemaname != "" {
			rv.Schemaname, err = a.lookupOrCreate(SCHEMA_KIND_PREFIX, rv.Schemaname)
			if err != nil {
				return fmt.Errorf("anon schema: %w", err)
			}
		}
		rv.Relname, err = a.lookupOrCreate(TABLE_KIND_PREFIX, rv.Relname)
		if err != nil {
			return fmt.Errorf("anon table: %w", err)
		}

	case queryparser.PG_QUERY_COLUMNDEF_NODE:
		cd, ok := queryparser.ProtoAsColumnDef(msg)
		if !ok {
			return fmt.Errorf("expected ColumnDef, got %T", msg.Interface())
		}
		cd.Colname, err = a.lookupOrCreate(COLUMN_KIND_PREFIX, cd.Colname)
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
			tok, err := a.lookupOrCreate(COLUMN_KIND_PREFIX, orig)
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
			rt.Name, err = a.lookupOrCreate(ALIAS_KIND_PREFIX, rt.Name)
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
			idx.Idxname, err = a.lookupOrCreate(INDEX_KIND_PREFIX, idx.Idxname)
			if err != nil {
				return fmt.Errorf("anon idxname: %w", err)
			}
		}
		// table name
		idx.Relation.Relname, err = a.lookupOrCreate(TABLE_KIND_PREFIX, idx.Relation.Relname)
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
			ie.Name, err = a.lookupOrCreate(COLUMN_KIND_PREFIX, ie.Name)
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
			cons.Conname, err = a.lookupOrCreate(CONSTRAINT_KIND_PREFIX, cons.Conname)
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
			alias.Aliasname, err = a.lookupOrCreate(ALIAS_KIND_PREFIX, alias.Aliasname)
			if err != nil {
				return fmt.Errorf("anon aliasnode: %w", err)
			}
		}
	}

	return nil
}

func (a *SqlAnonymizer) lookupOrCreate(kind string, identifier string) (string, error) {
	if identifier == "" {
		return "", nil // No identifier to anonymize
	}

	key := kind + identifier // for map lookup
	a.mu.RLock()
	if token, exists := a.tokenMap[key]; exists {
		a.mu.RUnlock()
		return token, nil // Return cached token
	}
	a.mu.RUnlock()

	// Generate a new token
	h := sha256.New()
	h.Write([]byte(kind + a.salt + identifier))
	sum := h.Sum(nil)
	token := kind + hex.EncodeToString(sum)[:16] // 16 hex chars == 8 bytes

	/*
		Note: For SHA-256, collision probablity mathematically is (N^2)/(2M)
		where N is the number of unique identifiers and M is the size of the hash space.

		For eg:
		M is 8bytes/16hex/32bits and N is 1000, the collision chances in % are 2.7 * 10^-12
		M is 8bytes/16hex/32bits and N is 10^6, the collision chances in % are 2.7 * 10^-6

		Hence even for 1M unique objects, the chances of collision are extremely low.
	*/

	// Cache the token
	a.mu.Lock()
	a.tokenMap[key] = token
	a.mu.Unlock()
	return token, nil
}
