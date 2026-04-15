package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ServiceName = "my-service"
	cfg.ModulePath = "github.com/org/my-service"
	cfg.OutputDir = "/tmp/my-service"

	s := New(cfg)
	assert.NotNil(t, s)
}

func TestScaffold_Validate(t *testing.T) {
	validConfig := func() Config {
		cfg := DefaultConfig()
		cfg.ServiceName = "my-service"
		cfg.ModulePath = "github.com/org/my-service"
		cfg.OutputDir = "/tmp/my-service"
		return cfg
	}

	tests := []struct {
		name    string
		modify  func(cfg *Config)
		wantErr string
	}{
		{
			name:    "valid config passes validation",
			modify:  func(_ *Config) {},
			wantErr: "",
		},
		{
			name:    "missing service name",
			modify:  func(cfg *Config) { cfg.ServiceName = "" },
			wantErr: "service name is required",
		},
		{
			name:    "missing module path",
			modify:  func(cfg *Config) { cfg.ModulePath = "" },
			wantErr: "module path is required",
		},
		{
			name:    "missing output dir",
			modify:  func(cfg *Config) { cfg.OutputDir = "" },
			wantErr: "output directory is required",
		},
		{
			name: "idempotency without redis",
			modify: func(cfg *Config) {
				cfg.Idempotency = true
				cfg.Redis = false
			},
			wantErr: "idempotency requires Redis to be enabled",
		},
		{
			name:    "unsupported protocol - grpc",
			modify:  func(cfg *Config) { cfg.Protocol = ProtocolGRPC },
			wantErr: `protocol "grpc" is not yet supported`,
		},
		{
			name:    "unsupported protocol - both",
			modify:  func(cfg *Config) { cfg.Protocol = ProtocolBoth },
			wantErr: `protocol "both" is not yet supported`,
		},
		{
			name:    "unsupported DI strategy",
			modify:  func(cfg *Config) { cfg.DI = "fx" },
			wantErr: `DI strategy "fx" is not yet supported`,
		},
		{
			name: "redis disabled with idempotency also disabled is valid",
			modify: func(cfg *Config) {
				cfg.Redis = false
				cfg.Idempotency = false
			},
			wantErr: "",
		},
		{
			name: "auth disabled is valid",
			modify: func(cfg *Config) {
				cfg.Auth = false
			},
			wantErr: "",
		},
		{
			name: "keep examples disabled is valid",
			modify: func(cfg *Config) {
				cfg.KeepExamples = false
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)
			s := New(cfg)

			validateErr := s.Validate()

			if tt.wantErr == "" {
				require.NoError(t, validateErr)
			} else {
				require.Error(t, validateErr)
				assert.Contains(t, validateErr.Error(), tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "github.com/jrmarcello/gopherplate", cfg.TemplateModulePath)
	assert.Equal(t, DBPostgres, cfg.DB)
	assert.Equal(t, ProtocolHTTP, cfg.Protocol)
	assert.Equal(t, DIManual, cfg.DI)
	assert.True(t, cfg.Redis)
	assert.True(t, cfg.Idempotency)
	assert.True(t, cfg.Auth)
	assert.True(t, cfg.KeepExamples)
	assert.Empty(t, cfg.ServiceName)
	assert.Empty(t, cfg.ModulePath)
	assert.Empty(t, cfg.OutputDir)
}

func TestScaffold_Validate_EdgeCases(t *testing.T) {
	validConfig := func() Config {
		cfg := DefaultConfig()
		cfg.ServiceName = "my-service"
		cfg.ModulePath = "github.com/org/my-service"
		cfg.OutputDir = "/tmp/my-service"
		return cfg
	}

	tests := []struct {
		name    string
		modify  func(cfg *Config)
		wantErr string
	}{
		{
			name: "empty DB driver is valid (no validation on DB field)",
			modify: func(cfg *Config) {
				cfg.DB = ""
			},
			wantErr: "",
		},
		{
			name: "all features disabled simultaneously is valid",
			modify: func(cfg *Config) {
				cfg.Redis = false
				cfg.Idempotency = false
				cfg.Auth = false
				cfg.KeepExamples = false
			},
			wantErr: "",
		},
		{
			name: "very long service name is valid",
			modify: func(cfg *Config) {
				cfg.ServiceName = "a-very-long-microservice-name-that-exceeds-normal-conventions-but-should-still-be-accepted-by-the-scaffold"
			},
			wantErr: "",
		},
		{
			name: "service name with special characters is valid",
			modify: func(cfg *Config) {
				cfg.ServiceName = "my_service.v2"
			},
			wantErr: "",
		},
		{
			name: "unknown protocol string",
			modify: func(cfg *Config) {
				cfg.Protocol = Protocol("websocket")
			},
			wantErr: `protocol "websocket" is not yet supported`,
		},
		{
			name: "unknown DI strategy string",
			modify: func(cfg *Config) {
				cfg.DI = DIStrategy("wire")
			},
			wantErr: `DI strategy "wire" is not yet supported`,
		},
		{
			name: "whitespace-only service name treated as empty",
			modify: func(cfg *Config) {
				cfg.ServiceName = "   "
			},
			// Current Validate() checks for empty string, not whitespace-only.
			// "   " is not empty, so this should pass validation.
			wantErr: "",
		},
		{
			name: "module path without slash is valid (scaffold does not validate path format)",
			modify: func(cfg *Config) {
				cfg.ModulePath = "my-service"
			},
			wantErr: "",
		},
		{
			name: "custom DB driver string is valid",
			modify: func(cfg *Config) {
				cfg.DB = DBDriver("cockroachdb")
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)
			s := New(cfg)

			validateErr := s.Validate()

			if tt.wantErr == "" {
				require.NoError(t, validateErr)
			} else {
				require.Error(t, validateErr)
				assert.Contains(t, validateErr.Error(), tt.wantErr)
			}
		})
	}
}
