package anonymizer

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

const (
	ANONYMIZED_TOKENS_TABLE = "anonymized_tokens"
)

// a sqlite store for anonymized data
type AnonymizerStore struct {
	db *sql.DB
}

func GetAnonymizerDBPath(exportDir string) string {
	return filepath.Join(exportDir, "metainfo", "anonymizer.db")
}

const SQLITE_OPTIONS = "?_txlock=exclusive&_timeout=30000"

func NewAnonymizerStore(exportDir string) (*AnonymizerStore, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s%s", GetAnonymizerDBPath(exportDir), SQLITE_OPTIONS))
	if err != nil {
		return nil, fmt.Errorf("error while opening meta db: %w", err)
	}
	return &AnonymizerStore{db: db}, nil
}

func (s *AnonymizerStore) Init() error {
	createTableSQL := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_token TEXT NOT NULL UNIQUE,
		anonymized_token TEXT NOT NULL
	);`, ANONYMIZED_TOKENS_TABLE)

	if _, err := s.db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("error creating %s table: %w", ANONYMIZED_TOKENS_TABLE, err)
	}
	return nil
}

func (s *AnonymizerStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// LookupOrCreate returns the anonymized token for 'orig',
// creating and persisting a new one if necessary.
func (s *AnonymizerStore) LookupOrCreate(orig string) (string, error) {
	if orig == "" {
		return "", nil
	}

	query := fmt.Sprintf(`SELECT anonymized_token FROM %s WHERE original_token = '%s'`, ANONYMIZED_TOKENS_TABLE, orig)

	// check if mapping exists
	var existing string
	err := s.db.QueryRow(query).Scan(&existing)
	if err == nil {
		return existing, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("lookup failed: %w", err)
	}

	// If not then generate a fresh unique token
	newToken, err := generateAnonToken()
	if err != nil {
		return "", fmt.Errorf("generating anon token: %w", err)
	}

	insertStmt := fmt.Sprintf(`INSERT INTO %s (original_token, anonymized_token) VALUES ('%s', '%s')`, ANONYMIZED_TOKENS_TABLE, orig, newToken)
	// Insert-or-ignore in case of race
	_, err = s.db.Exec(insertStmt)
	if err != nil {
		return "", fmt.Errorf("insert anon mapping: %w", err)
	}

	return newToken, nil
}

// generateAnonToken makes an unpredictable 16-hex-char string, prefixed anon_
// e.g. "anon_a3b9f1c2d4e5f678"
func generateAnonToken() (string, error) {
	const numBytes = 8
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "anon_" + hex.EncodeToString(b), nil
}
