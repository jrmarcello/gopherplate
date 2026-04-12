package redisclient

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jrmarcello/gopherplate/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: creates a RedisClient backed by miniredis
func newTestClient(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client, clientErr := NewRedisClient(RedisConfig{
		URL:     "redis://" + mr.Addr(),
		TTL:     "5m",
		Enabled: true,
	})
	require.NoError(t, clientErr)
	require.NotNil(t, client)
	return client, mr
}

// =============================================================================
// Constructor
// =============================================================================

func TestNewRedisClient_Disabled(t *testing.T) {
	client, clientErr := NewRedisClient(RedisConfig{Enabled: false})
	assert.NoError(t, clientErr)
	assert.Nil(t, client)
}

func TestNewRedisClient_InvalidURL(t *testing.T) {
	client, clientErr := NewRedisClient(RedisConfig{
		URL:     "not-a-valid-url",
		Enabled: true,
	})
	assert.Error(t, clientErr)
	assert.Nil(t, client)
}

func TestNewRedisClient_UnreachableServer(t *testing.T) {
	client, clientErr := NewRedisClient(RedisConfig{
		URL:         "redis://localhost:19999",
		Enabled:     true,
		DialTimeout: 100 * time.Millisecond,
	})
	assert.Error(t, clientErr)
	assert.Nil(t, client)
}

func TestNewRedisClient_ValidTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client, clientErr := NewRedisClient(RedisConfig{
		URL:     "redis://" + mr.Addr(),
		TTL:     "10m",
		Enabled: true,
	})
	require.NoError(t, clientErr)
	require.NotNil(t, client)
	assert.Equal(t, 10*time.Minute, client.ttl)
}

func TestNewRedisClient_InvalidTTL_FallbackTo5m(t *testing.T) {
	mr := miniredis.RunT(t)
	client, clientErr := NewRedisClient(RedisConfig{
		URL:     "redis://" + mr.Addr(),
		TTL:     "invalid",
		Enabled: true,
	})
	require.NoError(t, clientErr)
	require.NotNil(t, client)
	assert.Equal(t, 5*time.Minute, client.ttl)
}

func TestNewRedisClient_PoolConfig_Applied(t *testing.T) {
	mr := miniredis.RunT(t)
	client, clientErr := NewRedisClient(RedisConfig{
		URL:          "redis://" + mr.Addr(),
		TTL:          "1m",
		Enabled:      true,
		PoolSize:     50,
		MinIdleConns: 10,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	require.NoError(t, clientErr)
	require.NotNil(t, client)
	assert.Equal(t, 50, client.client.Options().PoolSize)
	assert.Equal(t, 10, client.client.Options().MinIdleConns)
}

// =============================================================================
// Nil safety (no-op pattern)
// =============================================================================

func TestNilClient_Get_ReturnsErrCacheMiss(t *testing.T) {
	var client *RedisClient
	var dest string
	getErr := client.Get(context.Background(), "key", &dest)
	assert.ErrorIs(t, getErr, cache.ErrCacheMiss)
}

func TestNilClient_Set_ReturnsNil(t *testing.T) {
	var client *RedisClient
	setErr := client.Set(context.Background(), "key", "value")
	assert.NoError(t, setErr)
}

func TestNilClient_Delete_ReturnsNil(t *testing.T) {
	var client *RedisClient
	delErr := client.Delete(context.Background(), "key")
	assert.NoError(t, delErr)
}

func TestNilClient_Close_ReturnsNil(t *testing.T) {
	var client *RedisClient
	closeErr := client.Close()
	assert.NoError(t, closeErr)
}

func TestNilClient_Ping_ReturnsNil(t *testing.T) {
	var client *RedisClient
	pingErr := client.Ping(context.Background())
	assert.NoError(t, pingErr)
}

func TestNilClient_UnderlyingClient_ReturnsNil(t *testing.T) {
	var client *RedisClient
	assert.Nil(t, client.UnderlyingClient())
}

// =============================================================================
// Get / Set / Delete with miniredis
// =============================================================================

func TestGet_CacheMiss(t *testing.T) {
	client, _ := newTestClient(t)
	var dest string
	getErr := client.Get(context.Background(), "nonexistent", &dest)
	assert.ErrorIs(t, getErr, cache.ErrCacheMiss)
}

func TestSetAndGet_Success(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	type testUser struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	// Set
	setErr := client.Set(ctx, "user:1", testUser{Name: "Test", Email: "t@test.com"})
	assert.NoError(t, setErr)

	// Get
	var dest testUser
	getErr := client.Get(ctx, "user:1", &dest)
	assert.NoError(t, getErr)
	assert.Equal(t, "Test", dest.Name)
	assert.Equal(t, "t@test.com", dest.Email)
}

func TestGet_InvalidJSON(t *testing.T) {
	client, mr := newTestClient(t)
	// Set raw invalid JSON directly in miniredis
	_ = mr.Set("bad-json", "not-valid-json{{{")
	var dest map[string]string
	getErr := client.Get(context.Background(), "bad-json", &dest)
	assert.Error(t, getErr)
	assert.Nil(t, dest)
}

func TestSet_MarshalError(t *testing.T) {
	client, _ := newTestClient(t)
	// channels can't be marshaled to JSON
	ch := make(chan int)
	setErr := client.Set(context.Background(), "key", ch)
	assert.Error(t, setErr)
}

func TestDelete_ExistingKey(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	setErr := client.Set(ctx, "to-delete", "value")
	require.NoError(t, setErr)

	delErr := client.Delete(ctx, "to-delete")
	assert.NoError(t, delErr)

	// Verify deleted
	var dest string
	getErr := client.Get(ctx, "to-delete", &dest)
	assert.ErrorIs(t, getErr, cache.ErrCacheMiss)
}

func TestDelete_NonExistentKey(t *testing.T) {
	client, _ := newTestClient(t)
	delErr := client.Delete(context.Background(), "doesnt-exist")
	assert.NoError(t, delErr)
}

// =============================================================================
// TTL
// =============================================================================

func TestSet_AppliesTTL(t *testing.T) {
	client, mr := newTestClient(t)
	ctx := context.Background()

	setErr := client.Set(ctx, "ttl-test", "value")
	require.NoError(t, setErr)

	// Verify TTL is set in miniredis
	ttl := mr.TTL("ttl-test")
	assert.Equal(t, 5*time.Minute, ttl)
}

func TestSet_KeyExpiresAfterTTL(t *testing.T) {
	client, mr := newTestClient(t)
	ctx := context.Background()

	setErr := client.Set(ctx, "expiring", "value")
	require.NoError(t, setErr)

	// Fast-forward past TTL
	mr.FastForward(6 * time.Minute)

	var dest string
	getErr := client.Get(ctx, "expiring", &dest)
	assert.ErrorIs(t, getErr, cache.ErrCacheMiss)
}

// =============================================================================
// Ping / Close / UnderlyingClient
// =============================================================================

func TestPing_Success(t *testing.T) {
	client, _ := newTestClient(t)
	pingErr := client.Ping(context.Background())
	assert.NoError(t, pingErr)
}

func TestClose_Success(t *testing.T) {
	client, _ := newTestClient(t)
	closeErr := client.Close()
	assert.NoError(t, closeErr)
}

func TestUnderlyingClient_ReturnsClient(t *testing.T) {
	client, _ := newTestClient(t)
	rc := client.UnderlyingClient()
	assert.NotNil(t, rc)
}

// =============================================================================
// Connection errors
// =============================================================================

func TestGet_AfterClose_ReturnsError(t *testing.T) {
	client, _ := newTestClient(t)
	_ = client.Close()

	var dest string
	getErr := client.Get(context.Background(), "key", &dest)
	assert.Error(t, getErr)
	assert.NotErrorIs(t, getErr, cache.ErrCacheMiss) // real error, not cache miss
}

func TestSet_AfterClose_ReturnsError(t *testing.T) {
	client, _ := newTestClient(t)
	_ = client.Close()

	setErr := client.Set(context.Background(), "key", "value")
	assert.Error(t, setErr)
}

func TestDelete_AfterClose_ReturnsError(t *testing.T) {
	client, _ := newTestClient(t)
	_ = client.Close()

	delErr := client.Delete(context.Background(), "key")
	assert.Error(t, delErr)
}

func TestPing_AfterClose_ReturnsError(t *testing.T) {
	client, _ := newTestClient(t)
	_ = client.Close()

	pingErr := client.Ping(context.Background())
	assert.Error(t, pingErr)
}
