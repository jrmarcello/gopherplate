package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jrmarcello/go-boilerplate/pkg/idempotency"
	"github.com/redis/go-redis/v9"
)

// RedisStore implements Store using Redis.
type RedisStore struct {
	client  *redis.Client
	ttl     time.Duration // TTL for completed entries (e.g. 24h)
	lockTTL time.Duration // TTL for processing lock (e.g. 30s)
}

// NewRedisStore creates a new Redis-backed idempotency store.
func NewRedisStore(client *redis.Client, ttl, lockTTL time.Duration) *RedisStore {
	return &RedisStore{
		client:  client,
		ttl:     ttl,
		lockTTL: lockTTL,
	}
}

// Lock attempts to acquire a lock for the key via SET NX (atomic operation).
// Returns true if acquired (first request), false if already existed.
// The fingerprint is stored in the entry so it can be checked later on replay.
func (s *RedisStore) Lock(ctx context.Context, key, fingerprint string) (bool, error) {
	entry := idempotency.Entry{
		Status:      idempotency.StatusProcessing,
		Fingerprint: fingerprint,
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return false, marshalErr
	}

	// SET key value NX EX lockTTL — atomic, prevents race conditions
	ok, setErr := s.client.SetNX(ctx, key, data, s.lockTTL).Result()
	if setErr != nil {
		return false, setErr
	}

	return ok, nil
}

// Get returns the entry stored for the key.
// Returns nil, nil if the key does not exist.
func (s *RedisStore) Get(ctx context.Context, key string) (*idempotency.Entry, error) {
	val, getErr := s.client.Get(ctx, key).Result()
	if errors.Is(getErr, redis.Nil) {
		return nil, nil
	}
	if getErr != nil {
		return nil, getErr
	}

	var entry idempotency.Entry
	if unmarshalErr := json.Unmarshal([]byte(val), &entry); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return &entry, nil
}

// Complete saves the final result with a long TTL.
func (s *RedisStore) Complete(ctx context.Context, key string, entry *idempotency.Entry) error {
	entry.Status = idempotency.StatusCompleted

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return marshalErr
	}

	return s.client.Set(ctx, key, data, s.ttl).Err()
}

// Unlock removes the key (used when the handler fails with 5xx).
func (s *RedisStore) Unlock(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
