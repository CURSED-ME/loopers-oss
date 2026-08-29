package keyring

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/try-loopers/loopers-oss/internal/cache"
	"golang.org/x/sync/singleflight"
)

var keyMetaCache = cache.NewTTLCache[string, *KeyMetadata](10 * time.Second)
var keyMetaGroup singleflight.Group

// KeyMetadata represents the fields stored in Redis for a loopers API key.
type KeyMetadata struct {
	Name             string `redis:"name" json:"name"`
	Provider         string `redis:"provider" json:"provider"`
	CreatedAt        string `redis:"created_at" json:"created_at"`
	Active           string `redis:"active" json:"active"`
	AgentName        string `redis:"agent_name" json:"agent_name,omitempty"`
	Owner            string `redis:"owner" json:"owner,omitempty"`
	AllowedTools     string `redis:"allowed_tools" json:"allowed_tools,omitempty"`
	AllowedProviders string `redis:"allowed_providers" json:"allowed_providers,omitempty"`
	Tags             string `redis:"tags" json:"tags,omitempty"`
	Jkt              string `redis:"jkt" json:"jkt,omitempty"`
}

// ParseAllowedTools parses the comma-separated allowed tools into a set.
func (m *KeyMetadata) ParseAllowedTools() map[string]bool {
	return parseCommaSeparatedSet(m.AllowedTools)
}

// ParseAllowedProviders parses the comma-separated allowed providers into a set.
func (m *KeyMetadata) ParseAllowedProviders() map[string]bool {
	return parseCommaSeparatedSet(m.AllowedProviders)
}

// ParseTags parses the comma-separated key=value tags into a map.
func (m *KeyMetadata) ParseTags() map[string]string {
	tags := make(map[string]string)
	if m.Tags == "" {
		return tags
	}
	pairs := strings.Split(m.Tags, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			tags[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		} else {
			tags[strings.TrimSpace(parts[0])] = ""
		}
	}
	return tags
}

func parseCommaSeparatedSet(input string) map[string]bool {
	set := make(map[string]bool)
	if input == "" {
		return set
	}
	items := strings.Split(input, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = true
		}
	}
	return set
}

// GetKeyMetadata retrieves key metadata from Redis using the SHA-256 hash of the raw key.
func GetKeyMetadata(ctx context.Context, rdb *redis.Client, keyHash string) (*KeyMetadata, error) {
	if val, ok := keyMetaCache.Get(keyHash); ok {
		return val, nil
	}

	key := fmt.Sprintf("loopers:key:%s", keyHash)

	v, err, _ := keyMetaGroup.Do(keyHash, func() (interface{}, error) {
		var meta KeyMetadata
		if err := rdb.HGetAll(ctx, key).Scan(&meta); err != nil {
			return nil, fmt.Errorf("failed to scan key metadata: %w", err)
		}

		// HGetAll returns empty struct if key does not exist
		if meta.Name == "" {
			return nil, errors.New("key does not exist")
		}

		keyMetaCache.Set(keyHash, &meta)
		return &meta, nil
	})

	if err != nil {
		return nil, err
	}

	return v.(*KeyMetadata), nil
}

// EncryptValue encrypts a plaintext string using AES-256-GCM and returns a base64-encoded string.
func EncryptValue(plaintext string, secret []byte) (string, error) {
	if len(secret) == 0 {
		if viper.GetString("env") != "development" {
			return "", errors.New("server_secret is required in production environments")
		}
		return plaintext, nil
	}
	if plaintext == "" {
		return plaintext, nil
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:v1:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptValue decrypts a base64 string using AES-256-GCM. If decryption fails, it returns the string as-is for backwards compatibility.
func DecryptValue(ciphertextB64 string, secret []byte) (string, error) {
	if len(secret) == 0 {
		if viper.GetString("env") != "development" {
			return "", errors.New("server_secret is required in production environments")
		}
		return ciphertextB64, nil
	}
	if ciphertextB64 == "" {
		return ciphertextB64, nil
	}
	if !strings.HasPrefix(ciphertextB64, "enc:v1:") {
		// VULN-035: Prevent silent fallback to plaintext if a secret is configured.
		// If the server has a secret, we enforce that all stored keys are encrypted.
		return "", fmt.Errorf("decryption failed: missing enc:v1: prefix (legacy plaintext fallback disabled)")
	}

	b64Data := strings.TrimPrefix(ciphertextB64, "enc:v1:")
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", fmt.Errorf("decryption failed: invalid base64")
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("decryption failed: ciphertext too short")
	}
	nonce, cipherBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	return string(plaintext), nil
}
