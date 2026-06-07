package tokenhash

import (
	"crypto/sha256"
	"encoding/hex"
)

func Hash(plainToken string) string {
	sum := sha256.Sum256([]byte(plainToken))
	return hex.EncodeToString(sum[:])
}
