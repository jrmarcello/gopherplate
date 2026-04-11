package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Setup env vars for test
	require.NoError(t, os.Setenv("SERVER_PORT", "9090"))
	require.NoError(t, os.Setenv("DB_HOST", "testhost"))
	require.NoError(t, os.Setenv("DB_PORT", "5433"))
	require.NoError(t, os.Setenv("DB_USER", "testuser"))
	require.NoError(t, os.Setenv("DB_PASSWORD", "testpass"))
	require.NoError(t, os.Setenv("DB_NAME", "test_db"))
	require.NoError(t, os.Setenv("DB_SSLMODE", "require"))
	require.NoError(t, os.Setenv("DB_MAX_OPEN_CONNS", "50"))
	require.NoError(t, os.Setenv("DB_MAX_IDLE_CONNS", "15"))
	require.NoError(t, os.Setenv("DB_CONN_MAX_LIFETIME", "10m"))
	require.NoError(t, os.Setenv("DB_CONN_MAX_IDLE_TIME", "2m"))
	require.NoError(t, os.Setenv("DB_REPLICA_ENABLED", "true"))
	require.NoError(t, os.Setenv("DB_REPLICA_HOST", "replicahost"))
	require.NoError(t, os.Setenv("DB_REPLICA_PORT", "5434"))
	require.NoError(t, os.Setenv("REDIS_ENABLED", "true"))
	require.NoError(t, os.Setenv("SWAGGER_ENABLED", "false"))
	defer os.Clearenv()

	cfg, loadErr := Load()
	assert.NoError(t, loadErr)
	assert.NotNil(t, cfg)

	// Verify overrides
	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, "testhost", cfg.DB.Host)
	assert.Equal(t, "5433", cfg.DB.Port)
	assert.Equal(t, "testuser", cfg.DB.User)
	assert.Equal(t, "testpass", cfg.DB.Password)
	assert.Equal(t, "test_db", cfg.DB.Name)
	assert.Equal(t, "require", cfg.DB.SSLMode)
	assert.Equal(t, 50, cfg.DB.MaxOpenConns)
	assert.Equal(t, 15, cfg.DB.MaxIdleConns)
	assert.Equal(t, 10*time.Minute, cfg.DB.ConnMaxLifetime)
	assert.Equal(t, 2*time.Minute, cfg.DB.ConnMaxIdleTime)
	assert.True(t, cfg.DB.ReplicaEnabled)
	assert.Equal(t, "replicahost", cfg.DB.ReplicaHost)
	assert.Equal(t, "5434", cfg.DB.ReplicaPort)
	assert.True(t, cfg.Redis.Enabled)
	assert.False(t, cfg.Swagger.Enabled)

	// Verify Redis TTL is parseable (previously tested via GetRedisTTL)
	redisTTL, parseErr := time.ParseDuration(cfg.Redis.TTL)
	assert.NoError(t, parseErr)
	assert.Equal(t, 5*time.Minute, redisTTL)

	// Verify DSN methods
	writerDSN := cfg.DB.GetWriterDSN()
	assert.Contains(t, writerDSN, "host=testhost")
	assert.Contains(t, writerDSN, "port=5433")
	assert.Contains(t, writerDSN, "dbname=test_db")

	readerDSN := cfg.DB.GetReaderDSN()
	assert.Contains(t, readerDSN, "host=replicahost")
	assert.Contains(t, readerDSN, "port=5434")
	// Falls back to writer user/password/name
	assert.Contains(t, readerDSN, "user=testuser")
	assert.Contains(t, readerDSN, "dbname=test_db")
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()

	cfg, loadErr := Load()
	assert.NoError(t, loadErr)

	// Verify defaults
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.DB.Host)
	assert.Equal(t, "5432", cfg.DB.Port)
	assert.Equal(t, "user", cfg.DB.User)
	assert.Equal(t, "password", cfg.DB.Password)
	assert.Equal(t, "users", cfg.DB.Name)
	assert.Equal(t, "disable", cfg.DB.SSLMode)
	assert.Equal(t, 25, cfg.DB.MaxOpenConns)
	assert.Equal(t, 10, cfg.DB.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.DB.ConnMaxLifetime)
	assert.Equal(t, 90*time.Second, cfg.DB.ConnMaxIdleTime)
	assert.False(t, cfg.DB.ReplicaEnabled)
	assert.False(t, cfg.Redis.Enabled)
	assert.False(t, cfg.Swagger.Enabled)

	// Writer DSN with defaults
	writerDSN := cfg.DB.GetWriterDSN()
	assert.Equal(t, "host=localhost port=5432 user=user password=password dbname=users sslmode=disable", writerDSN)

	// Reader DSN falls back entirely to writer when no replica fields set
	readerDSN := cfg.DB.GetReaderDSN()
	assert.Equal(t, writerDSN, readerDSN)
}
