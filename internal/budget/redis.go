package budget

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client is a wrapper around the go-redis client.
type Client struct {
	rdb              *redis.Client
	batchMu          sync.Mutex
	reserveBatches   map[string][]reserveReq
	reconcileMu      sync.Mutex
	reconcileBatches map[string]reconcileReq
}

// NewClient initializes and returns a Client.
func NewClient(addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:            addr,
		Password:        password,
		DB:              db,
		PoolSize:        100,
		MinIdleConns:    10,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	return &Client{rdb: rdb}, nil
}

// Ping checks if the Redis server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// GetUnderlyingClient returns the raw go-redis client.
func (c *Client) GetUnderlyingClient() *redis.Client {
	return c.rdb
}
