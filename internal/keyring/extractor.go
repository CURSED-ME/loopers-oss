package keyring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/loopers-oss/loopers/internal/cache"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var keyMetaCache = cache.NewTTLCache(30 * time.Second)
var keyMetaGroup singleflight.Group

// KeyMetadata represents the fields stored in Redis for a loopers API key.
type KeyMetadata struct {
	Name      string `redis:"name"`
	Provider  string `redis:"provider"`
	CreatedAt string `redis:"created_at"`
	Active    string `redis:"active"`
}

// GetKeyMetadata retrieves key metadata from Redis using the SHA-256 hash of the raw key.
func GetKeyMetadata(ctx context.Context, rdb *redis.Client, keyHash string) (*KeyMetadata, error) {
	if val, ok := keyMetaCache.Get(keyHash); ok {
		return val.(*KeyMetadata), nil
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
