package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Setup env vars for test
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("POSTGRES_DB", "test_db")
	os.Setenv("REDIS_ENABLED", "true")
	defer os.Clearenv()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify overrides
	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Contains(t, cfg.DB.DSN, "test_db")
	assert.True(t, cfg.Redis.Enabled)
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
