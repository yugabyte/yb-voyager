package anon

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateSalt returns a hex-encoded salt string of length 2*n characters,
// where n is the number of random bytes in salt.
// For example, GenerateSalt(8) gives you a 16-char hex string.
func GenerateSalt(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("invalid salt length %d; must be > 0", n)
	}

	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// convert bytes to hex to make it printable ASCII string
	return hex.EncodeToString(b), nil
}
