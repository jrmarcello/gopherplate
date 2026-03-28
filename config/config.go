package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server  ServerConfig
	DB      DBConfig
	Otel    OtelConfig
	Redis   RedisConfig
	Auth    AuthConfig
	Swagger SwaggerConfig
}

type AuthConfig struct {
	// Enabled indica se a autenticação está habilitada.
	// Em HML/PRD deve ser true. Se true e ServiceKeys vazio → fail-closed (503).
	Enabled bool
	// ServiceKeys no formato "service1:key1,service2:key2"
	ServiceKeys string
}

type ServerConfig struct {
	Port string
	Env  string
}

// DBConfig contém a configuração do banco de dados.
// Writer (primary) usa DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE.
// Replica (read) usa DB_REPLICA_* com fallback para os valores do writer.
type DBConfig struct {
	// Writer (primary)
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string

	// Writer pool
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Replica (read)
	ReplicaEnabled         bool
	ReplicaHost            string
	ReplicaPort            string
	ReplicaUser            string
	ReplicaPassword        string
	ReplicaName            string
	ReplicaSSLMode         string
	ReplicaMaxOpenConns    int
	ReplicaMaxIdleConns    int
	ReplicaConnMaxLifetime time.Duration
	ReplicaConnMaxIdleTime time.Duration
}

// GetWriterDSN builds the writer (primary) connection DSN in key=value format.
func (c *DBConfig) GetWriterDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// GetReaderDSN builds the reader (replica) connection DSN.
// Empty fields fall back to writer values.
func (c *DBConfig) GetReaderDSN() string {
	host := c.ReplicaHost
	if host == "" {
		host = c.Host
	}
	port := c.ReplicaPort
	if port == "" {
		port = c.Port
	}
	user := c.ReplicaUser
	if user == "" {
		user = c.User
	}
	password := c.ReplicaPassword
	if password == "" {
		password = c.Password
	}
	name := c.ReplicaName
	if name == "" {
		name = c.Name
	}
	sslMode := c.ReplicaSSLMode
	if sslMode == "" {
		sslMode = c.SSLMode
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, name, sslMode)
}

type OtelConfig struct {
	ServiceName  string
	CollectorURL string
	Insecure     bool
}

type RedisConfig struct {
	URL          string
	TTL          string // ex: "5m", "1h"
	Enabled      bool
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type SwaggerConfig struct {
	Enabled bool
	Host    string
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
			Env:  getEnv("APP_ENV", "development"),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "user"),
			Password: getEnv("DB_PASSWORD", "password"),
			Name:     getEnv("DB_NAME", "entities"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),

			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 90*time.Second),

			ReplicaEnabled:         getEnvBool("DB_REPLICA_ENABLED", false),
			ReplicaHost:            os.Getenv("DB_REPLICA_HOST"),
			ReplicaPort:            os.Getenv("DB_REPLICA_PORT"),
			ReplicaUser:            os.Getenv("DB_REPLICA_USER"),
			ReplicaPassword:        os.Getenv("DB_REPLICA_PASSWORD"),
			ReplicaName:            os.Getenv("DB_REPLICA_NAME"),
			ReplicaSSLMode:         os.Getenv("DB_REPLICA_SSLMODE"),
			ReplicaMaxOpenConns:    getEnvInt("DB_REPLICA_MAX_OPEN_CONNS", 40),
			ReplicaMaxIdleConns:    getEnvInt("DB_REPLICA_MAX_IDLE_CONNS", 20),
			ReplicaConnMaxLifetime: getEnvDuration("DB_REPLICA_CONN_MAX_LIFETIME", 5*time.Minute),
			ReplicaConnMaxIdleTime: getEnvDuration("DB_REPLICA_CONN_MAX_IDLE_TIME", 90*time.Second),
		},
		Otel: OtelConfig{
			ServiceName:  getEnv("OTEL_SERVICE_NAME", "entity-service"),
			CollectorURL: getEnv("OTEL_COLLECTOR_URL", ""),
			Insecure:     getEnvBool("OTEL_INSECURE", true),
		},
		Redis: RedisConfig{
			URL:          getEnv("REDIS_URL", "redis://localhost:6379"),
			TTL:          getEnv("REDIS_TTL", "5m"),
			Enabled:      getEnvBool("REDIS_ENABLED", false),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 30),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 500*time.Millisecond),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 200*time.Millisecond),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 200*time.Millisecond),
		},
		Auth: AuthConfig{
			Enabled:     getEnvBool("SERVICE_KEYS_ENABLED", false),
			ServiceKeys: getEnv("SERVICE_KEYS", ""),
		},
		Swagger: SwaggerConfig{
			Enabled: getEnvBool("SWAGGER_ENABLED", false),
			Host:    getEnv("SWAGGER_HOST", ""),
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
		parsed, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			return fallback
		}
		return parsed
	}
	return fallback
}

// getEnvInt retorna o valor inteiro da variável de ambiente ou o fallback.
func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return fallback
		}
		return parsed
	}
	return fallback
}

// getEnvDuration retorna o valor de duração da variável de ambiente ou o fallback.
// Aceita formatos como "5m", "1h", "30s".
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		parsed, parseErr := time.ParseDuration(value)
		if parseErr != nil {
			return fallback
		}
		return parsed
	}
	return fallback
}
