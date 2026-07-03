package keyring

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateRawKey generates a random Base62 key prefixed with lp-.
func GenerateRawKey() (string, error) {
	b62 := make([]byte, 43)
	for i := range b62 {
		num, err := rand.Int(rand.Reader, big.NewInt(62))
		if err != nil {
			return "", err
		}
		b62[i] = base62Chars[num.Int64()]
	}
	return "lp-" + string(b62), nil
}

// HashKey computes the SHA-256 hex string of the loopers proxy key (used for public logging and headers).
func HashKey(rawKey string) string {
	hasher := sha256.New()
	hasher.Write([]byte(rawKey))
	return hex.EncodeToString(hasher.Sum(nil))
}

// StorageHash computes the HMAC-SHA256 hex string of the loopers proxy key using a server secret.
// This decouples the internal Redis storage namespace from the observable public hash.
func StorageHash(rawKey, serverSecret string) string {
	if serverSecret == "" {
		return HashKey(rawKey)
	}
	mac := hmac.New(sha256.New, []byte(serverSecret))
	mac.Write([]byte(rawKey))
	return hex.EncodeToString(mac.Sum(nil))
}
