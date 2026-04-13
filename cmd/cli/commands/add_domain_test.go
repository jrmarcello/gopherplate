package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDomainName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		// Valid names
		{
			name:    "simple lowercase",
			input:   "order",
			wantErr: false,
		},
		{
			name:    "snake_case",
			input:   "order_item",
			wantErr: false,
		},
		{
			name:    "payment",
			input:   "payment",
			wantErr: false,
		},
		{
			name:    "single letter",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "name with digits",
			input:   "order2",
			wantErr: false,
		},
		{
			name:    "PascalCase is normalized to snake_case",
			input:   "OrderItem",
			wantErr: false,
		},
		{
			name:    "camelCase is normalized to snake_case",
			input:   "orderItem",
			wantErr: false,
		},
		{
			name:    "hyphenated name is normalized to snake_case",
			input:   "order-item",
			wantErr: false,
		},
		// Invalid names
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "domain name cannot be empty",
		},
		{
			name:    "starts with digit",
			input:   "123abc",
			wantErr: true,
			errMsg:  "invalid domain name",
		},
		{
			name:    "contains spaces normalizes to valid snake_case",
			input:   "order item",
			wantErr: false,
		},
		{
			name:    "starts with underscore normalizes to valid name",
			input:   "_order",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateErr := validateDomainName(tt.input)

			if tt.wantErr {
				require.Error(t, validateErr)
				assert.Contains(t, validateErr.Error(), tt.errMsg)
			} else {
				assert.NoError(t, validateErr)
			}
		})
	}
}

func TestDetectModulePath(t *testing.T) {
	t.Run("detects module path from valid go.mod", func(t *testing.T) {
		dir := t.TempDir()
		goModContent := "module github.com/test/my-service\n\ngo 1.25.0\n"
		writeErr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o600)
		require.NoError(t, writeErr)

		// detectModulePath reads from current directory, so we must chdir
		origDir, getErr := os.Getwd()
		require.NoError(t, getErr)
		chdirErr := os.Chdir(dir)
		require.NoError(t, chdirErr)
		t.Cleanup(func() {
			_ = os.Chdir(origDir)
		})

		modulePath, detectErr := detectModulePath()
		require.NoError(t, detectErr)
		assert.Equal(t, "github.com/test/my-service", modulePath)
	})

	t.Run("detects module path with extra whitespace", func(t *testing.T) {
		dir := t.TempDir()
		goModContent := "  module   github.com/org/svc  \n\ngo 1.25.0\n"
		writeErr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o600)
		require.NoError(t, writeErr)

		origDir, getErr := os.Getwd()
		require.NoError(t, getErr)
		chdirErr := os.Chdir(dir)
		require.NoError(t, chdirErr)
		t.Cleanup(func() {
			_ = os.Chdir(origDir)
		})

		modulePath, detectErr := detectModulePath()
		require.NoError(t, detectErr)
		assert.Equal(t, "github.com/org/svc", modulePath)
	})

	t.Run("returns error when go.mod is missing", func(t *testing.T) {
		dir := t.TempDir()

		origDir, getErr := os.Getwd()
		require.NoError(t, getErr)
		chdirErr := os.Chdir(dir)
		require.NoError(t, chdirErr)
		t.Cleanup(func() {
			_ = os.Chdir(origDir)
		})

		_, detectErr := detectModulePath()
		require.Error(t, detectErr)
	})

	t.Run("returns error when go.mod has no module directive", func(t *testing.T) {
		dir := t.TempDir()
		goModContent := "go 1.25.0\n\nrequire (\n)\n"
		writeErr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o600)
		require.NoError(t, writeErr)

		origDir, getErr := os.Getwd()
		require.NoError(t, getErr)
		chdirErr := os.Chdir(dir)
		require.NoError(t, chdirErr)
		t.Cleanup(func() {
			_ = os.Chdir(origDir)
		})

		_, detectErr := detectModulePath()
		require.Error(t, detectErr)
		assert.Contains(t, detectErr.Error(), "module directive not found")
	})

	t.Run("returns error when module line has no path after keyword", func(t *testing.T) {
		// "module " followed by only whitespace: TrimSpace produces "module",
		// which does not match HasPrefix("module "), so it falls through to
		// "module directive not found".
		dir := t.TempDir()
		goModContent := "module \n\ngo 1.25.0\n"
		writeErr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o644)
		require.NoError(t, writeErr)

		origDir, getErr := os.Getwd()
		require.NoError(t, getErr)
		chdirErr := os.Chdir(dir)
		require.NoError(t, chdirErr)
		t.Cleanup(func() {
			_ = os.Chdir(origDir)
		})

		_, detectErr := detectModulePath()
		require.Error(t, detectErr)
		assert.Contains(t, detectErr.Error(), "module directive not found")
	})
}

func TestBuildTemplateMappings(t *testing.T) {
	t.Run("maps all templates to correct output paths", func(t *testing.T) {
		mappings := buildTemplateMappings("order", "20260329120000")

		// Domain layer
		assert.Equal(t,
			filepath.Join("internal", "domain", "order", "entity.go"),
			mappings["entity.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "domain", "order", "entity_test.go"),
			mappings["entity_test.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "domain", "order", "errors.go"),
			mappings["errors.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "domain", "order", "filter.go"),
			mappings["filter.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "domain", "order", "filter_test.go"),
			mappings["filter_test.go.tmpl"],
		)

		// Use cases
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "create.go"),
			mappings["create_usecase.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "get.go"),
			mappings["get_usecase.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "list.go"),
			mappings["list_usecase.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "update.go"),
			mappings["update_usecase.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "delete.go"),
			mappings["delete_usecase.go.tmpl"],
		)

		// Interfaces
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "interfaces", "repository.go"),
			mappings["repository_interface.go.tmpl"],
		)

		// DTOs
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "dto", "create.go"),
			mappings["dto_create.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "usecases", "order", "dto", "get.go"),
			mappings["dto_get.go.tmpl"],
		)

		// Infrastructure
		assert.Equal(t,
			filepath.Join("internal", "infrastructure", "db", "postgres", "repository", "order.go"),
			mappings["repository_postgres.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "infrastructure", "web", "handler", "order.go"),
			mappings["handler.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "infrastructure", "web", "router", "order.go"),
			mappings["router.go.tmpl"],
		)

		// Migration
		assert.Equal(t,
			filepath.Join("internal", "infrastructure", "db", "postgres", "migration", "20260329120000_create_orders.sql"),
			mappings["migration.sql.tmpl"],
		)
	})

	t.Run("handles multi-word snake_case domain name", func(t *testing.T) {
		mappings := buildTemplateMappings("order_item", "20260329120000")

		assert.Equal(t,
			filepath.Join("internal", "domain", "order_item", "entity.go"),
			mappings["entity.go.tmpl"],
		)
		assert.Equal(t,
			filepath.Join("internal", "infrastructure", "db", "postgres", "migration", "20260329120000_create_order_items.sql"),
			mappings["migration.sql.tmpl"],
		)
	})

	t.Run("total template count matches expected", func(t *testing.T) {
		mappings := buildTemplateMappings("product", "20260329120000")
		// 5 domain (3 source + 2 tests) + 12 usecases (5 UC + 5 UC tests + errors + mocks) + 1 interface + 5 DTOs + 4 infra (repo + repo_test + handler + router) + 1 migration = 28
		assert.Len(t, mappings, 28)
	})
}
