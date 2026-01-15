package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server ServerConfig
	DB     DBConfig
	Otel   OtelConfig
	Redis  RedisConfig
}

type ServerConfig struct {
	Port string
}

type DBConfig struct {
	// Formato Postgres: postgres://user:password@host:port/database?sslmode=disable
	DSN string
}

type OtelConfig struct {
	ServiceName  string
	CollectorURL string
}

type RedisConfig struct {
	URL     string
	TTL     string // ex: "5m", "1h"
	Enabled bool
}

// Load configura a aplicação lendo do ambiente.
// Prioridade:
// 1. Variáveis de Ambiente (maior prioridade)
// 2. Arquivo .env (desenvolvimento local)
// 3. Defaults (fallback seguro)
func Load() (*Config, error) {
	// Carrega .env se existir (ignora erro se não existir)
	_ = godotenv.Load()

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		DB: DBConfig{
			DSN: getEnv("DB_DSN", "postgres://user:password@localhost:5432/entities?sslmode=disable"),
		},
		Otel: OtelConfig{
			ServiceName:  getEnv("OTEL_SERVICE_NAME", "entity-service"),
			CollectorURL: getEnv("OTEL_COLLECTOR_URL", ""),
		},
		Redis: RedisConfig{
			URL:     getEnv("REDIS_URL", "redis://localhost:6379"),
			TTL:     getEnv("REDIS_TTL", "5m"),
			Enabled: getEnvBool("REDIS_ENABLED", false),
		},
	}, nil
}

// getEnv retorna o valor da variável de ambiente ou o fallback se não existir.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvBool retorna o valor booleano da variável de ambiente ou o fallback.
func getEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fallback
		}
		return parsed
	}
	return fallback
}

// GetRedisTTL retorna o TTL do Redis como time.Duration.
func (c *Config) GetRedisTTL() time.Duration {
	d, err := time.ParseDuration(c.Redis.TTL)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
