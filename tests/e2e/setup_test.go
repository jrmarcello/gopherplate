package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"bitbucket.org/appmax-space/go-boilerplate/pkg/cache/redisclient"
)

var testDB *sqlx.DB
var testCache *redisclient.RedisClient

// PostgresContainer encapsula o container do Postgres para testes
type PostgresContainer struct {
	*postgres.PostgresContainer
	ConnectionString string
}

// CreatePostgresContainer cria e inicia um container Postgres para testes
func CreatePostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	return &PostgresContainer{
		PostgresContainer: container,
		ConnectionString:  connStr,
	}, nil
}

// getMigrationsDir retorna o caminho absoluto para o diretório de migrations
func getPostgresMigrationsDir() string {
	_, currentFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(currentFile)
	// Navega de tests/e2e/ para a raiz do projeto e depois para o diretório de migrations
	return filepath.Join(testDir, "..", "..", "internal", "infrastructure", "db", "postgres", "migration")
}

// RunMigrations executa as migrações no banco de teste usando goose
func RunPostgresMigrations(db *sql.DB) error {
	// Configurar goose
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Executar todas as migrations
	if err := goose.Up(db, getPostgresMigrationsDir()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// CreateRedisContainer cria e inicia um container Redis para testes
func CreateRedisContainer(ctx context.Context) (testcontainers.Container, string, error) {
	container, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start redis container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get redis connection string: %w", err)
	}

	return container, connStr, nil
}

// GetTestCache retorna o cache Redis de teste
func GetTestCache() *redisclient.RedisClient {
	return testCache
}

// TestMain configura o ambiente de teste e2e
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Iniciar container Postgres
	pgContainer, err := CreatePostgresContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to create postgres container: %v", err)
	}

	// Conectar ao banco
	testDB, err = sqlx.Connect("postgres", pgContainer.ConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to test database: %v", err)
	}

	// Executar migrações usando goose
	if migrateErr := RunPostgresMigrations(testDB.DB); migrateErr != nil {
		log.Fatalf("Failed to run migrations: %v", migrateErr)
	}

	// Iniciar container Redis
	redisContainer, redisConnStr, err := CreateRedisContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to create redis container: %v", err)
	}

	// Criar cliente Redis para testes
	testCache, err = redisclient.NewRedisClient(redisclient.RedisConfig{
		URL:     redisConnStr,
		TTL:     "5m",
		Enabled: true,
	})
	if err != nil {
		log.Fatalf("Failed to create redis client: %v", err)
	}

	// Definir variáveis de ambiente para a aplicação
	os.Setenv("DB_DSN", pgContainer.ConnectionString)
	os.Setenv("REDIS_URL", redisConnStr)
	os.Setenv("REDIS_ENABLED", "true")

	// Executar testes
	code := m.Run()

	// Cleanup
	if testCache != nil {
		testCache.Close()
	}
	testDB.Close()
	if err := redisContainer.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate redis container: %v", err)
	}
	if err := pgContainer.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate postgres container: %v", err)
	}

	os.Exit(code)
}

// HTTPClient retorna um http.Client configurado para testes
func HTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

// GetTestDB retorna a conexão do banco de teste
func GetTestDB() *sqlx.DB {
	return testDB
}

// CleanupEntities remove todas as entities do banco de teste
func CleanupEntities() error {
	_, err := testDB.Exec("DELETE FROM entities")
	return err
}
