package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewRedisClient_Disabled verifica que retorna nil quando cache está desabilitado.
func TestNewRedisClient_Disabled(t *testing.T) {
	cfg := Config{
		URL:     "redis://localhost:6379",
		TTL:     "5m",
		Enabled: false,
	}

	client, err := NewRedisClient(cfg)

	assert.NoError(t, err)
	assert.Nil(t, client)
}

// TestNewRedisClient_InvalidURL verifica erro com URL inválida.
func TestNewRedisClient_InvalidURL(t *testing.T) {
	cfg := Config{
		URL:     "not-a-valid-url",
		TTL:     "5m",
		Enabled: true,
	}

	client, err := NewRedisClient(cfg)

	assert.Error(t, err)
	assert.Nil(t, client)
}

// TestRedisClient_NilSafe verifica que métodos são seguros com nil receiver.
func TestRedisClient_NilSafe(t *testing.T) {
	var client *RedisClient = nil
	ctx := context.Background()

	t.Run("Get returns ErrCacheMiss", func(t *testing.T) {
		var dest string
		err := client.Get(ctx, "key", &dest)
		assert.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("Set returns nil", func(t *testing.T) {
		err := client.Set(ctx, "key", "value")
		assert.NoError(t, err)
	})

	t.Run("Delete returns nil", func(t *testing.T) {
		err := client.Delete(ctx, "key")
		assert.NoError(t, err)
	})

	t.Run("Close returns nil", func(t *testing.T) {
		err := client.Close()
		assert.NoError(t, err)
	})

	t.Run("Ping returns nil", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err)
	})
}

// TestConfig_TTLDefaults verifica defaults de TTL.
func TestConfig_TTLDefault(t *testing.T) {
	// Este teste apenas verifica a estrutura Config
	cfg := Config{
		URL:     "redis://localhost:6379",
		TTL:     "invalid-duration",
		Enabled: false,
	}

	// TTL inválido não deve causar erro na criação quando disabled
	client, err := NewRedisClient(cfg)
	assert.NoError(t, err)
	assert.Nil(t, client)
}
