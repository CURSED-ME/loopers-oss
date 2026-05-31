package cache

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// TTLCache is a thread-safe cache with TTL.
type TTLCache struct {
	m   sync.Map
	ttl time.Duration
}

// NewTTLCache creates a new TTLCache with the specified TTL.
func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{
		ttl: ttl,
	}
}

// Set stores a value in the cache.
func (c *TTLCache) Set(key string, value interface{}) {
	c.m.Store(key, cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Get retrieves a value from the cache if it hasn't expired.
func (c *TTLCache) Get(key string) (interface{}, bool) {
	val, ok := c.m.Load(key)
	if !ok {
		return nil, false
	}
	entry := val.(cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.m.Delete(key)
		return nil, false
	}
	return entry.value, true
}

// Delete removes a key from the cache.
func (c *TTLCache) Delete(key string) {
	c.m.Delete(key)
}
