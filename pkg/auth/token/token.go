// Package token generates and hashes the secrets Atellar hands out: node API
// keys, join tokens and the admin token. Only SHA-256 hashes are stored.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"
)

const (
	NodeAPIKeyTTL         = 90 * 24 * time.Hour
	NodeAPIKeyRenewBefore = 7 * 24 * time.Hour
)

// Generate returns a random 256-bit secret as hex.
func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Equal compares secrets in constant time.
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ShouldRenewNodeAPIKey reports whether a key is close enough to expiry.
func ShouldRenewNodeAPIKey(expiresAt time.Time) bool {
	return time.Until(expiresAt) <= NodeAPIKeyRenewBefore
}
