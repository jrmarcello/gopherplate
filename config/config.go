package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"db"`
	Otel   OtelConfig   `mapstructure:"otel"`
	Redis  RedisConfig  `mapstructure:"redis"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type DBConfig struct {
	// Formato Postgres: postgres://user:password@host:port/database?sslmode=disable
	DSN string `mapstructure:"dsn"`
}

type OtelConfig struct {
	ServiceName  string `mapstructure:"service_name"`
	CollectorURL string `mapstructure:"collector_url"`
}

type RedisConfig struct {
	URL     string `mapstructure:"url"`
	TTL     string `mapstructure:"ttl"` // ex: "5m", "1h"
	Enabled bool   `mapstructure:"enabled"`
}

// Load configurations using Viper.
// Priority:
// 1. Environment Variables
// 2. Config File (config.yaml)
// 3. Defaults
func Load() (*Config, error) {
	v := viper.New()

	// 1. Set Defaults
	setDefaults(v)

	// 2. Load from file (optional)
	v.SetConfigFile(".env") // explicit file path
	v.SetConfigType("env")  // proper format
	_ = v.ReadInConfig()    // ignore error if config file not found

	// 3. Load from Environment Variables
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.port", "8080")

	// DB
	v.SetDefault("db.dsn", "postgres://user:password@localhost:5432/entities?sslmode=disable")

	// Otel
	v.SetDefault("otel.service_name", "entity-service")
	v.SetDefault("otel.collector_url", "")

	// Redis
	v.SetDefault("redis.url", "redis://localhost:6379")
	v.SetDefault("redis.ttl", "5m")
	v.SetDefault("redis.enabled", false)
}

func (c *Config) GetRedisTTL() time.Duration {
	d, err := time.ParseDuration(c.Redis.TTL)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
