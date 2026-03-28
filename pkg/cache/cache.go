package cache

import (
	"context"
	"errors"
)

// ErrCacheMiss indicates that the key was not found in cache.
var ErrCacheMiss = errors.New("cache miss")

// Cache defines the interface for cache operations.
// Implementations should be nil-safe (operations on nil client are no-ops).
type Cache interface {
	// Get retrieves a value from cache and deserializes into dest.
	// Returns error if key doesn't exist or deserialization fails.
	Get(ctx context.Context, key string, dest interface{}) error

	// Set stores a value in the cache with default TTL.
	Set(ctx context.Context, key string, value interface{}) error

	// Delete removes a key from the cache.
	Delete(ctx context.Context, key string) error

	// Ping checks if the cache connection is healthy.
	Ping(ctx context.Context) error

	// Close closes the cache connection.
	Close() error
}
