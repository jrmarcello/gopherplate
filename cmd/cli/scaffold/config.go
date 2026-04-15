package scaffold

// Protocol defines the API protocol choice.
type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolGRPC Protocol = "grpc"
	ProtocolBoth Protocol = "both"
)

// DIStrategy defines the dependency injection strategy.
type DIStrategy string

const (
	DIManual DIStrategy = "manual"
)

// DBDriver defines the database driver choice.
type DBDriver string

const (
	DBPostgres DBDriver = "postgres"
	DBMySQL    DBDriver = "mysql"
	DBSQLite   DBDriver = "sqlite"
	DBOther    DBDriver = "other"
)

// Config holds all scaffold configuration.
type Config struct {
	// ServiceName is the short name (e.g., "payment-service")
	ServiceName string

	// ModulePath is the full Go module path (e.g., "github.com/org/payment-service")
	ModulePath string

	// TemplateModulePath is the original template module path to replace
	TemplateModulePath string

	// OutputDir is the target directory for generated files
	OutputDir string

	// DB is the database driver choice
	DB DBDriver

	// Protocol is the API protocol (http, grpc, both). Currently only "http" is supported.
	Protocol Protocol

	// DI is the dependency injection strategy (manual, fx). Currently only "manual" is supported.
	DI DIStrategy

	// Redis enables Redis cache support
	Redis bool

	// Idempotency enables idempotency middleware (requires Redis)
	Idempotency bool

	// Auth enables service key authentication
	Auth bool

	// KeepExamples keeps the example domains (user/role)
	KeepExamples bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		TemplateModulePath: "github.com/jrmarcello/gopherplate",
		DB:                 DBPostgres,
		Protocol:           ProtocolHTTP,
		DI:                 DIManual,
		Redis:              true,
		Idempotency:        true,
		Auth:               true,
		KeepExamples:       true,
	}
}
