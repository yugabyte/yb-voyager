package anon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

type TokenRegistry interface {
	// Token returns a deterministic anonymised string for identifier with kind prefix.
	Token(kind string, identifier string) (string, error)
}

// SchemaTokenRegistry is meant to be shared across voyager commands/processes under the migration_uuid
// also across various anonymizer in same run - sql anonymizer, metadata anonymizer etc.
// so that the same identifier always gets the same anonymized token irrespective of the command or anonymizer used.
type SchemaTokenRegistry struct {
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

func NewSchemaTokenRegistry(metaDB *metadb.MetaDB) (TokenRegistry, error) {
	salt, err := loadOrCreateSalt(metaDB)
	if err != nil {
		return nil, fmt.Errorf("error loading or creating salt: %w", err)
	}

	return &SchemaTokenRegistry{
		salt:     salt,
		tokenMap: make(map[string]string),
	}, nil
}

func (r *SchemaTokenRegistry) Token(kind string, identifier string) (string, error) {
	if identifier == "" {
		return "", nil // No identifier to anonymize
	}

	// combining 'kind' for uniqueness wrt the namespace (table, schema, database)
	// for eg: users as tablename(unqualified) and users as columnname
	key := kind + identifier
	r.mu.RLock()
	if token, exists := r.tokenMap[key]; exists {
		r.mu.RUnlock()
		return token, nil // Return cached token
	}
	r.mu.RUnlock()

	// Generate a new token
	h := sha256.New() // generates 32-byte hash
	h.Write([]byte(kind + r.salt + identifier))
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
	r.mu.Lock()
	r.tokenMap[key] = token
	r.mu.Unlock()
	return token, nil
}

func loadOrCreateSalt(metaDB *metadb.MetaDB) (string, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return "", fmt.Errorf("error getting migration status record: %w", err)
	}

	var salt string
	if msr != nil && msr.AnonymizerSalt != "" {
		salt = msr.AnonymizerSalt
	} else {
		salt, err = GenerateSalt(SALT_SIZE)
		if err != nil {
			return "", fmt.Errorf("error generating salt: %w", err)
		}

		// Store the generated salt in the migration status record
		err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			if record == nil { // should not happen, but just in case
				record = &metadb.MigrationStatusRecord{}
			}
			record.AnonymizerSalt = salt
		})
		if err != nil {
			return "", fmt.Errorf("error updating migration status record with salt: %w", err)
		}
	}
	return salt, nil
}
