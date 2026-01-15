package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Setup env vars for test - os.Getenv reads these!
	// Struct structure: Server.Port -> SERVER_PORT
	os.Setenv("SERVER_PORT", "9090")
	// DB.DSN -> DB_DSN
	os.Setenv("DB_DSN", "postgres://test:test@localhost:5432/test_db?sslmode=disable")
	// Redis.Enabled -> REDIS_ENABLED
	os.Setenv("REDIS_ENABLED", "true")
	defer os.Clearenv()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify overrides
	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Contains(t, cfg.DB.DSN, "test_db")
	assert.True(t, cfg.Redis.Enabled)
	assert.Equal(t, 5*time.Minute, cfg.GetRedisTTL())
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()

	cfg, err := Load()
	assert.NoError(t, err)

	// Verify defaults
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Contains(t, cfg.DB.DSN, "entities")
	assert.False(t, cfg.Redis.Enabled)
}
