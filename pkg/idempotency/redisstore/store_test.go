package redisstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jrmarcello/gopherplate/pkg/idempotency"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- helpers ----------

func setupRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewRedisStore(client, 24*time.Hour, 30*time.Second)
	return store, mr
}

// ---------- Constructor tests ----------

func TestNewRedisStore_ReturnsStore(t *testing.T) {
	store, _ := setupRedisStore(t)

	require.NotNil(t, store)
	assert.Equal(t, 24*time.Hour, store.ttl)
	assert.Equal(t, 30*time.Second, store.lockTTL)
}

func TestNewRedisStore_TTLValues(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	store := NewRedisStore(client, 1*time.Hour, 10*time.Second)

	assert.Equal(t, 1*time.Hour, store.ttl)
	assert.Equal(t, 10*time.Second, store.lockTTL)
}

func TestNewRedisStore_ImplementsStoreInterface(t *testing.T) {
	store, _ := setupRedisStore(t)
	var _ idempotency.Store = store // compile-time check
	assert.NotNil(t, store)
}

// ---------- Lock tests ----------

func TestRedisStore_Lock_AcquiresOnFirstCall(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	acquired, lockErr := store.Lock(ctx, "idem:key-1", "fp-abc")

	require.NoError(t, lockErr)
	assert.True(t, acquired)
}

func TestRedisStore_Lock_ReturnsFalseOnDuplicate(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-abc")
	acquired, lockErr := store.Lock(ctx, "idem:key-1", "fp-abc")

	require.NoError(t, lockErr)
	assert.False(t, acquired)
}

func TestRedisStore_Lock_SetsCorrectTTL(t *testing.T) {
	store, mr := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-ttl", "fp-abc")

	ttl := mr.TTL("idem:key-ttl")
	assert.Equal(t, 30*time.Second, ttl)
}

func TestRedisStore_Lock_StoresProcessingStatus(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-abc")

	entry, getErr := store.Get(ctx, "idem:key-1")
	require.NoError(t, getErr)
	require.NotNil(t, entry)
	assert.Equal(t, idempotency.StatusProcessing, entry.Status)
	assert.Equal(t, "fp-abc", entry.Fingerprint)
}

func TestRedisStore_Lock_ErrorOnClosedConnection(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisStore(client, 24*time.Hour, 30*time.Second)

	mr.Close()

	_, lockErr := store.Lock(context.Background(), "key", "fp")
	assert.Error(t, lockErr)
}

// ---------- Get tests ----------

func TestRedisStore_Get_ReturnsNilForNonExistentKey(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	entry, getErr := store.Get(ctx, "idem:missing")

	require.NoError(t, getErr)
	assert.Nil(t, entry)
}

func TestRedisStore_Get_ReturnsEntryAfterLock(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-123")

	entry, getErr := store.Get(ctx, "idem:key-1")

	require.NoError(t, getErr)
	require.NotNil(t, entry)
	assert.Equal(t, idempotency.StatusProcessing, entry.Status)
	assert.Equal(t, "fp-123", entry.Fingerprint)
	assert.Equal(t, 0, entry.StatusCode, "StatusCode must be zero for a locked-but-not-completed entry")
	assert.Nil(t, entry.Body, "Body must be nil for a locked-but-not-completed entry")
}

func TestRedisStore_Get_ReturnsCompletedEntry(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-123")
	_ = store.Complete(ctx, "idem:key-1", &idempotency.Entry{
		StatusCode:  201,
		Body:        []byte(`{"created":true}`),
		Fingerprint: "fp-123",
	})

	entry, getErr := store.Get(ctx, "idem:key-1")

	require.NoError(t, getErr)
	require.NotNil(t, entry)
	assert.Equal(t, idempotency.StatusCompleted, entry.Status)
	assert.Equal(t, 201, entry.StatusCode)
	assert.Equal(t, []byte(`{"created":true}`), entry.Body)
	assert.Equal(t, "fp-123", entry.Fingerprint)
}

func TestRedisStore_Get_ErrorOnClosedConnection(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisStore(client, 24*time.Hour, 30*time.Second)

	mr.Close()

	entry, getErr := store.Get(context.Background(), "key")
	assert.Error(t, getErr)
	assert.Nil(t, entry)
}

// ---------- Complete tests ----------

func TestRedisStore_Complete_SetsStatusToCompleted(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-abc")

	entry := &idempotency.Entry{
		StatusCode:  200,
		Body:        []byte(`{"ok":true}`),
		Fingerprint: "fp-abc",
	}
	completeErr := store.Complete(ctx, "idem:key-1", entry)

	require.NoError(t, completeErr)
	assert.Equal(t, idempotency.StatusCompleted, entry.Status, "Complete must mutate the entry Status")
}

func TestRedisStore_Complete_ExtendsTTL(t *testing.T) {
	store, mr := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-ttl", "fp-abc")

	// Before Complete, TTL is lockTTL (30s)
	ttlBefore := mr.TTL("idem:key-ttl")
	assert.Equal(t, 30*time.Second, ttlBefore)

	_ = store.Complete(ctx, "idem:key-ttl", &idempotency.Entry{
		StatusCode:  200,
		Body:        []byte(`{}`),
		Fingerprint: "fp-abc",
	})

	// After Complete, TTL is store.ttl (24h)
	ttlAfter := mr.TTL("idem:key-ttl")
	assert.Equal(t, 24*time.Hour, ttlAfter)
}

func TestRedisStore_Complete_OverwritesExistingEntry(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-abc")

	_ = store.Complete(ctx, "idem:key-1", &idempotency.Entry{
		StatusCode:  201,
		Body:        []byte(`{"id":"new-user"}`),
		Fingerprint: "fp-abc",
	})

	entry, _ := store.Get(ctx, "idem:key-1")
	require.NotNil(t, entry)
	assert.Equal(t, 201, entry.StatusCode)
	assert.Equal(t, []byte(`{"id":"new-user"}`), entry.Body)
}

func TestRedisStore_Complete_ErrorOnClosedConnection(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisStore(client, 24*time.Hour, 30*time.Second)

	mr.Close()

	completeErr := store.Complete(context.Background(), "key", &idempotency.Entry{
		StatusCode:  200,
		Body:        []byte(`{}`),
		Fingerprint: "fp",
	})
	assert.Error(t, completeErr)
}

// ---------- Unlock tests ----------

func TestRedisStore_Unlock_RemovesKey(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:key-1", "fp-abc")

	unlockErr := store.Unlock(ctx, "idem:key-1")
	require.NoError(t, unlockErr)

	entry, getErr := store.Get(ctx, "idem:key-1")
	require.NoError(t, getErr)
	assert.Nil(t, entry)
}

func TestRedisStore_Unlock_NoErrorOnNonExistentKey(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	unlockErr := store.Unlock(ctx, "idem:non-existent")
	assert.NoError(t, unlockErr)
}

func TestRedisStore_Unlock_ErrorOnClosedConnection(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisStore(client, 24*time.Hour, 30*time.Second)

	mr.Close()

	unlockErr := store.Unlock(context.Background(), "key")
	assert.Error(t, unlockErr)
}

// ---------- Full lifecycle tests ----------

func TestRedisStore_FullLifecycle_LockCompleteGet(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	// 1. Lock
	acquired, lockErr := store.Lock(ctx, "idem:lifecycle", "fp-xyz")
	require.NoError(t, lockErr)
	require.True(t, acquired)

	// 2. Verify processing status
	entry, _ := store.Get(ctx, "idem:lifecycle")
	require.NotNil(t, entry)
	assert.Equal(t, idempotency.StatusProcessing, entry.Status)

	// 3. Complete
	completeErr := store.Complete(ctx, "idem:lifecycle", &idempotency.Entry{
		StatusCode:  201,
		Body:        []byte(`{"id":"user-1"}`),
		Fingerprint: "fp-xyz",
	})
	require.NoError(t, completeErr)

	// 4. Verify completed status
	entry, _ = store.Get(ctx, "idem:lifecycle")
	require.NotNil(t, entry)
	assert.Equal(t, idempotency.StatusCompleted, entry.Status)
	assert.Equal(t, 201, entry.StatusCode)
	assert.Equal(t, []byte(`{"id":"user-1"}`), entry.Body)

	// 5. Retry lock must fail
	acquired, _ = store.Lock(ctx, "idem:lifecycle", "fp-xyz")
	assert.False(t, acquired, "Lock must fail for a completed key")
}

func TestRedisStore_RetryLifecycle_LockUnlockRelock(t *testing.T) {
	store, _ := setupRedisStore(t)
	ctx := context.Background()

	// 1. Lock
	acquired, _ := store.Lock(ctx, "idem:retry", "fp-1")
	require.True(t, acquired)

	// 2. Simulated 5xx -- unlock
	unlockErr := store.Unlock(ctx, "idem:retry")
	require.NoError(t, unlockErr)

	// 3. Client retries
	acquired, lockErr := store.Lock(ctx, "idem:retry", "fp-2")
	require.NoError(t, lockErr)
	assert.True(t, acquired)

	// 4. Verify new fingerprint
	entry, _ := store.Get(ctx, "idem:retry")
	require.NotNil(t, entry)
	assert.Equal(t, "fp-2", entry.Fingerprint)
}

func TestRedisStore_LockExpiry(t *testing.T) {
	store, mr := setupRedisStore(t)
	ctx := context.Background()

	_, _ = store.Lock(ctx, "idem:expiry", "fp-abc")

	// Fast-forward past lockTTL
	mr.FastForward(31 * time.Second)

	// Key should have expired; Get returns nil
	entry, getErr := store.Get(ctx, "idem:expiry")
	require.NoError(t, getErr)
	assert.Nil(t, entry, "entry must be nil after lock TTL expires")

	// Lock should succeed again
	acquired, _ := store.Lock(ctx, "idem:expiry", "fp-def")
	assert.True(t, acquired)
}
