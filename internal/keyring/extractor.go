package keyring

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// KeyMetadata represents the fields stored in Redis for a loopers API key.
type KeyMetadata struct {
	Name      string `redis:"name"`
	Provider  string `redis:"provider"`
	CreatedAt string `redis:"created_at"`
	Active    string `redis:"active"`
}

// GetKeyMetadata retrieves key metadata from Redis using the SHA-256 hash of the raw key.
func GetKeyMetadata(ctx context.Context, rdb *redis.Client, keyHash string) (*KeyMetadata, error) {
	key := fmt.Sprintf("loopers:key:%s", keyHash)

	// Check if key exists
	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query key existence: %w", err)
	}
	if exists == 0 {
		return nil, errors.New("key does not exist")
	}

	var meta KeyMetadata
	if err := rdb.HGetAll(ctx, key).Scan(&meta); err != nil {
		return nil, fmt.Errorf("failed to scan key metadata: %w", err)
	}

	return &meta, nil
}
