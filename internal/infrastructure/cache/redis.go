package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

// RedisClient wraps the Redis client with application-specific methods.
type RedisClient struct {
	client *redis.Client
	ttl    time.Duration
}

// Config holds Redis connection settings.
type Config struct {
	URL     string
	TTL     string
	Enabled bool
}

// NewRedisClient creates a new Redis client from URL.
// URL format: redis://[user:password@]host:port[/db]
func NewRedisClient(cfg Config) (*RedisClient, error) {
	if !cfg.Enabled {
		slog.Info("Redis cache disabled")
		return nil, nil
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if pingErr := client.Ping(ctx).Err(); pingErr != nil {
		return nil, pingErr
	}

	// Parse TTL
	ttl, err := time.ParseDuration(cfg.TTL)
	if err != nil {
		ttl = 5 * time.Minute // default
	}

	slog.Info("Redis cache initialized",
		"url", cfg.URL,
		"ttl", ttl,
	)

	return &RedisClient{
		client: client,
		ttl:    ttl,
	}, nil
}

// Get retrieves a value from cache and unmarshals into dest.
func (r *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	if r == nil {
		return ErrCacheMiss
	}

	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// Set stores a value in cache with the configured TTL.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}) error {
	if r == nil {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, r.ttl).Err()
}

// Delete removes a key from cache.
func (r *RedisClient) Delete(ctx context.Context, key string) error {
	if r == nil {
		return nil
	}

	return r.client.Del(ctx, key).Err()
}

// Close closes the Redis connection.
func (r *RedisClient) Close() error {
	if r == nil {
		return nil
	}
	return r.client.Close()
}

// Ping checks the Redis connection health.
func (r *RedisClient) Ping(ctx context.Context) error {
	if r == nil {
		return nil // nil client means cache is disabled, not an error
	}
	return r.client.Ping(ctx).Err()
}
