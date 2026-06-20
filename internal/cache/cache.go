package cache

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTLCache is a thread-safe cache with TTL.
type TTLCache[K comparable, V any] struct {
	m   sync.Map
	ttl time.Duration
}

// NewTTLCache creates a new TTLCache with the specified TTL and starts a background sweep.
func NewTTLCache[K comparable, V any](ttl time.Duration) *TTLCache[K, V] {
	c := &TTLCache[K, V]{
		ttl: ttl,
	}
	go c.startSweep()
	return c
}

func (c *TTLCache[K, V]) startSweep() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		now := time.Now()
		c.m.Range(func(key, value interface{}) bool {
			entry := value.(cacheEntry[V])
			if now.After(entry.expiresAt) {
				c.m.Delete(key)
			}
			return true
		})
	}
}

// Set stores a value in the cache.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.m.Store(key, cacheEntry[V]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Get retrieves a value from the cache if it hasn't expired.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	var zero V
	val, ok := c.m.Load(key)
	if !ok {
		return zero, false
	}
	entry := val.(cacheEntry[V])
	if time.Now().After(entry.expiresAt) {
		c.m.Delete(key)
		return zero, false
	}
	return entry.value, true
}

// Delete removes a key from the cache.
func (c *TTLCache[K, V]) Delete(key K) {
	c.m.Delete(key)
}
