package anon

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	// HASH_LENGTH defines the number of hex characters in the anonymized hash portion
	HASH_LENGTH = 16
)

type IdentifierHasher interface {
	// GetHash returns a deterministic anonymised string for identifier with kind prefix.
	// If identifier is empty, it returns empty string.
	// If the identifier is itself anonymized, it returns the same string.
	// If the identifier is not anonymized, it generates a new anonymized string and caches it for future use.
	GetHash(kind string, identifier string) (string, error)
}

// IdentifierHashRegistry is meant to be shared across voyager commands/processes under the migration_uuid
// also across various anonymizer in same run - sql anonymizer, metadata anonymizer etc.
// so that the same identifier always gets the same anonymized token irrespective of the command or anonymizer used.
type IdentifierHashRegistry struct {
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
	identifierHashMap map[string]string

	mu sync.RWMutex // Mutex to protect concurrent access to tokenMap
}

func NewIdentifierHashRegistry(salt string) (IdentifierHasher, error) {
	return &IdentifierHashRegistry{
		salt:              salt,
		identifierHashMap: make(map[string]string),
	}, nil
}

func (r *IdentifierHashRegistry) GetHash(kind string, identifier string) (string, error) {
	if identifier == "" {
		return "", nil // No identifier to anonymize
	}

	if IsAnonymized(identifier) {
		log.Infof("identifier %s is already anonymized", identifier)
		return identifier, nil
	}

	// combining 'kind' for uniqueness wrt the namespace (table, schema, database)
	// for eg: users as tablename(unqualified) and users as columnname
	key := kind + identifier
	r.mu.RLock()
	if token, exists := r.identifierHashMap[key]; exists {
		r.mu.RUnlock()
		return token, nil // Return cached token
	}
	r.mu.RUnlock()

	// Generate a new token
	h := sha256.New() // generates 32-byte hash
	h.Write([]byte(kind + r.salt + identifier))
	sum := h.Sum(nil)
	token := kind + hex.EncodeToString(sum)[:HASH_LENGTH] // 16 hex chars == 8 bytes

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
	r.identifierHashMap[key] = token
	r.mu.Unlock()
	return token, nil
}

// IsAnonymized checks if an identifier has already been anonymized
// An anonymized identifier has the format: <prefix><HASH_LENGTH_hex_chars>
// where prefix is one of the *_KIND_PREFIX constants defined in this package
// For qualified names (e.g., "schema.table"), it checks if any part is anonymized
func IsAnonymized(identifier string) bool {
	if identifier == "" {
		return false
	}

	if !strings.Contains(identifier, ".") {
		return isPartAnonymized(identifier)
	}

	// Handle qualified names by checking each part separately
	// Only case for this: sequence function arguments like nextval('schema.table.sequence')
	parts := strings.Split(identifier, ".")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if isPartAnonymized(part) {
			return true // If any part is anonymized, assuming the whole thing anonymized
		}
	}

	return false
}

// isPartAnonymized checks if a single identifier part is anonymized
func isPartAnonymized(part string) bool {
	for _, prefix := range AllKindPrefixes {
		if strings.HasPrefix(part, prefix) {
			remaining := part[len(prefix):]
			if len(remaining) == HASH_LENGTH && isHexString(remaining) {
				return true
			}
		}
	}
	return false
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	hexPattern := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	return hexPattern.MatchString(s)
}
