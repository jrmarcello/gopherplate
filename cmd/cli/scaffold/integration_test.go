package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaintmpl "github.com/jrmarcello/go-boilerplate/cmd/cli/templates/domain"
)

func TestAddDomainIntegration(t *testing.T) {
	t.Run("scaffolds complete domain for product", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create a minimal Go project structure
		goModContent := "module github.com/test/my-service\n\ngo 1.25.0\n"
		writeErr := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0o644)
		require.NoError(t, writeErr)

		// Build TemplateData for domain "product"
		cfg := Config{ModulePath: "github.com/test/my-service"}
		data := NewTemplateData("product", cfg)

		// Read each template from the embedded domain templates and render to project
		entries, readDirErr := domaintmpl.Templates.ReadDir(".")
		require.NoError(t, readDirErr)

		migrationTimestamp := "20260329120000"

		templateMappings := buildTestTemplateMappings(data, migrationTimestamp)

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".tmpl" {
				continue
			}

			tmplContent, readErr := fs.ReadFile(domaintmpl.Templates, entry.Name())
			require.NoError(t, readErr, "reading template %s", entry.Name())

			outputRelPath, found := templateMappings[entry.Name()]
			require.True(t, found, "no output mapping for template %s", entry.Name())

			outputPath := filepath.Join(projectDir, outputRelPath)

			renderErr := RenderTemplateFile(string(tmplContent), data, outputPath)
			require.NoError(t, renderErr, "rendering template %s", entry.Name())

		}

		// Verify all expected directories exist
		expectedDirs := []string{
			filepath.Join("internal", "domain", "product"),
			filepath.Join("internal", "usecases", "product"),
			filepath.Join("internal", "usecases", "product", "interfaces"),
			filepath.Join("internal", "usecases", "product", "dto"),
			filepath.Join("internal", "infrastructure", "db", "postgres", "repository"),
			filepath.Join("internal", "infrastructure", "web", "handler"),
			filepath.Join("internal", "infrastructure", "web", "router"),
			filepath.Join("internal", "infrastructure", "db", "postgres", "migration"),
		}
		for _, dir := range expectedDirs {
			absDir := filepath.Join(projectDir, dir)
			info, statErr := os.Stat(absDir)
			require.NoError(t, statErr, "expected directory %s to exist", dir)
			assert.True(t, info.IsDir(), "expected %s to be a directory", dir)
		}

		// Verify all expected files exist
		expectedFiles := []string{
			filepath.Join("internal", "domain", "product", "entity.go"),
			filepath.Join("internal", "domain", "product", "errors.go"),
			filepath.Join("internal", "domain", "product", "filter.go"),
			filepath.Join("internal", "usecases", "product", "create.go"),
			filepath.Join("internal", "usecases", "product", "get.go"),
			filepath.Join("internal", "usecases", "product", "list.go"),
			filepath.Join("internal", "usecases", "product", "update.go"),
			filepath.Join("internal", "usecases", "product", "delete.go"),
			filepath.Join("internal", "usecases", "product", "interfaces", "repository.go"),
			filepath.Join("internal", "usecases", "product", "dto", "create.go"),
			filepath.Join("internal", "usecases", "product", "dto", "get.go"),
			filepath.Join("internal", "usecases", "product", "dto", "list.go"),
			filepath.Join("internal", "usecases", "product", "dto", "update.go"),
			filepath.Join("internal", "usecases", "product", "dto", "delete.go"),
			filepath.Join("internal", "infrastructure", "db", "postgres", "repository", "product.go"),
			filepath.Join("internal", "infrastructure", "web", "handler", "product.go"),
			filepath.Join("internal", "infrastructure", "web", "router", "product.go"),
			filepath.Join("internal", "infrastructure", "db", "postgres", "migration", "20260329120000_create_products.sql"),
		}
		for _, file := range expectedFiles {
			absFile := filepath.Join(projectDir, file)
			_, statErr := os.Stat(absFile)
			assert.NoError(t, statErr, "expected file %s to exist", file)
		}

		// Verify rendered content has correct domain substitutions
		entityContent, readErr := os.ReadFile(filepath.Join(projectDir, "internal", "domain", "product", "entity.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(entityContent), "package product")
		assert.Contains(t, string(entityContent), "type Product struct")
		assert.Contains(t, string(entityContent), "func NewProduct(")

		errorsContent, readErr2 := os.ReadFile(filepath.Join(projectDir, "internal", "domain", "product", "errors.go"))
		require.NoError(t, readErr2)
		assert.Contains(t, string(errorsContent), "package product")
		assert.Contains(t, string(errorsContent), "ErrProductNotFound")

		filterContent, readErr3 := os.ReadFile(filepath.Join(projectDir, "internal", "domain", "product", "filter.go"))
		require.NoError(t, readErr3)
		assert.Contains(t, string(filterContent), "package product")
		assert.Contains(t, string(filterContent), "ListResult")

		// Verify use case content
		createContent, readErr4 := os.ReadFile(filepath.Join(projectDir, "internal", "usecases", "product", "create.go"))
		require.NoError(t, readErr4)
		assert.Contains(t, string(createContent), "package product")
		assert.Contains(t, string(createContent), "CreateUseCase")
		assert.Contains(t, string(createContent), "github.com/test/my-service/internal/domain/product")

		// Verify handler content
		handlerContent, readErr5 := os.ReadFile(filepath.Join(projectDir, "internal", "infrastructure", "web", "handler", "product.go"))
		require.NoError(t, readErr5)
		assert.Contains(t, string(handlerContent), "package handler")
		assert.Contains(t, string(handlerContent), "ProductHandler")
		assert.Contains(t, string(handlerContent), "github.com/test/my-service/internal/usecases/product")

		// Verify router content
		routerContent, readErr6 := os.ReadFile(filepath.Join(projectDir, "internal", "infrastructure", "web", "router", "product.go"))
		require.NoError(t, readErr6)
		assert.Contains(t, string(routerContent), "package router")
		assert.Contains(t, string(routerContent), "RegisterProductRoutes")
		assert.Contains(t, string(routerContent), "/products")

		// Verify migration content
		migrationContent, readErr7 := os.ReadFile(filepath.Join(projectDir, "internal", "infrastructure", "db", "postgres", "migration", "20260329120000_create_products.sql"))
		require.NoError(t, readErr7)
		assert.Contains(t, string(migrationContent), "-- +goose Up")
		assert.Contains(t, string(migrationContent), "CREATE TABLE products")
		assert.Contains(t, string(migrationContent), "-- +goose Down")
		assert.Contains(t, string(migrationContent), "DROP TABLE IF EXISTS products")
	})

	t.Run("scaffolds multi-word domain correctly", func(t *testing.T) {
		projectDir := t.TempDir()

		goModContent := "module github.com/test/my-service\n\ngo 1.25.0\n"
		writeErr := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0o644)
		require.NoError(t, writeErr)

		cfg := Config{ModulePath: "github.com/test/my-service"}
		data := NewTemplateData("order_item", cfg)

		// Render just the entity and errors templates
		entityTmpl, readErr := fs.ReadFile(domaintmpl.Templates, "entity.go.tmpl")
		require.NoError(t, readErr)

		entityPath := filepath.Join(projectDir, "internal", "domain", "order_item", "entity.go")
		renderErr := RenderTemplateFile(string(entityTmpl), data, entityPath)
		require.NoError(t, renderErr)

		content, readErr2 := os.ReadFile(entityPath)
		require.NoError(t, readErr2)
		assert.Contains(t, string(content), "package order_item")
		assert.Contains(t, string(content), "type OrderItem struct")
		assert.Contains(t, string(content), "func NewOrderItem(")
	})
}

func TestDomainTemplatesRender(t *testing.T) {
	data := NewTemplateData("order_item", Config{
		ModulePath: "github.com/test/my-service",
	})

	entries, readDirErr := domaintmpl.Templates.ReadDir(".")
	require.NoError(t, readDirErr)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tmpl" {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			tmplContent, readErr := fs.ReadFile(domaintmpl.Templates, entry.Name())
			require.NoError(t, readErr)

			rendered, renderErr := RenderTemplate(string(tmplContent), data)
			require.NoError(t, renderErr)
			assert.NotEmpty(t, rendered)

			// Verify Go files have a valid package declaration
			if strings.HasSuffix(entry.Name(), ".go.tmpl") {
				assert.Contains(t, rendered, "package ",
					"Go template %s should contain a package declaration", entry.Name())
			}

			// Verify SQL files have goose directives
			if strings.HasSuffix(entry.Name(), ".sql.tmpl") {
				assert.Contains(t, rendered, "-- +goose Up",
					"SQL template %s should contain goose Up directive", entry.Name())
				assert.Contains(t, rendered, "-- +goose Down",
					"SQL template %s should contain goose Down directive", entry.Name())
			}

			// Verify no raw template placeholders remain
			assert.NotContains(t, rendered, "{{.",
				"rendered output for %s should not contain unresolved template placeholders", entry.Name())
			assert.NotContains(t, rendered, "{{plural",
				"rendered output for %s should not contain unresolved template functions", entry.Name())
		})
	}
}

func TestDomainTemplatesRender_DomainNameVariants(t *testing.T) {
	tests := []struct {
		name              string
		domainName        string
		wantPascal        string
		wantCamel         string
		wantSnake         string
		wantPlural        string
		wantPluralInRoute string
	}{
		{
			name:              "simple domain",
			domainName:        "product",
			wantPascal:        "Product",
			wantCamel:         "product",
			wantSnake:         "product",
			wantPlural:        "products",
			wantPluralInRoute: "/products",
		},
		{
			name:              "multi-word snake_case",
			domainName:        "order_item",
			wantPascal:        "OrderItem",
			wantCamel:         "orderItem",
			wantSnake:         "order_item",
			wantPlural:        "order_items",
			wantPluralInRoute: "/order_items",
		},
		{
			name:              "PascalCase input",
			domainName:        "PaymentMethod",
			wantPascal:        "PaymentMethod",
			wantCamel:         "paymentMethod",
			wantSnake:         "payment_method",
			wantPlural:        "payment_methods",
			wantPluralInRoute: "/payment_methods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{ModulePath: "github.com/test/svc"}
			data := NewTemplateData(tt.domainName, cfg)

			// Verify TemplateData is computed correctly
			assert.Equal(t, tt.wantPascal, data.DomainNamePascal)
			assert.Equal(t, tt.wantCamel, data.DomainNameCamel)
			assert.Equal(t, tt.wantSnake, data.DomainNameSnake)
			assert.Equal(t, tt.wantPlural, data.DomainNamePlural)

			// Render the entity template and verify
			entityTmpl, readErr := fs.ReadFile(domaintmpl.Templates, "entity.go.tmpl")
			require.NoError(t, readErr)

			rendered, renderErr := RenderTemplate(string(entityTmpl), data)
			require.NoError(t, renderErr)
			assert.Contains(t, rendered, "type "+tt.wantPascal+" struct")
			assert.Contains(t, rendered, "func New"+tt.wantPascal+"(")

			// Render the router template and verify route paths
			routerTmpl, readErr2 := fs.ReadFile(domaintmpl.Templates, "router.go.tmpl")
			require.NoError(t, readErr2)

			routerRendered, renderErr2 := RenderTemplate(string(routerTmpl), data)
			require.NoError(t, renderErr2)
			assert.Contains(t, routerRendered, tt.wantPluralInRoute)
			assert.Contains(t, routerRendered, "Register"+tt.wantPascal+"Routes")
		})
	}
}

func TestDomainTemplatesEmbedFS(t *testing.T) {
	t.Run("embedded FS contains expected template files", func(t *testing.T) {
		entries, readDirErr := domaintmpl.Templates.ReadDir(".")
		require.NoError(t, readDirErr)

		expectedTemplates := []string{
			"entity.go.tmpl",
			"errors.go.tmpl",
			"filter.go.tmpl",
			"create_usecase.go.tmpl",
			"get_usecase.go.tmpl",
			"list_usecase.go.tmpl",
			"update_usecase.go.tmpl",
			"delete_usecase.go.tmpl",
			"repository_interface.go.tmpl",
			"repository_postgres.go.tmpl",
			"dto_create.go.tmpl",
			"dto_get.go.tmpl",
			"dto_list.go.tmpl",
			"dto_update.go.tmpl",
			"dto_delete.go.tmpl",
			"handler.go.tmpl",
			"router.go.tmpl",
			"migration.sql.tmpl",
		}

		fileNames := make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tmpl" {
				fileNames = append(fileNames, entry.Name())
			}
		}

		for _, expected := range expectedTemplates {
			assert.Contains(t, fileNames, expected, "embedded FS should contain %s", expected)
		}
	})

	t.Run("all template files are non-empty", func(t *testing.T) {
		entries, readDirErr := domaintmpl.Templates.ReadDir(".")
		require.NoError(t, readDirErr)

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".tmpl" {
				continue
			}
			content, readErr := fs.ReadFile(domaintmpl.Templates, entry.Name())
			require.NoError(t, readErr)
			assert.NotEmpty(t, content, "template %s should not be empty", entry.Name())
		}
	})
}

// buildTestTemplateMappings creates the template-to-output-path map for tests.
// This mirrors buildTemplateMappings from add_domain.go but takes TemplateData.
func buildTestTemplateMappings(data TemplateData, migrationTimestamp string) map[string]string {
	snakeName := data.DomainNameSnake
	pluralName := data.DomainNamePlural

	return map[string]string{
		"entity.go.tmpl":               filepath.Join("internal", "domain", snakeName, "entity.go"),
		"errors.go.tmpl":               filepath.Join("internal", "domain", snakeName, "errors.go"),
		"filter.go.tmpl":               filepath.Join("internal", "domain", snakeName, "filter.go"),
		"create_usecase.go.tmpl":       filepath.Join("internal", "usecases", snakeName, "create.go"),
		"get_usecase.go.tmpl":          filepath.Join("internal", "usecases", snakeName, "get.go"),
		"list_usecase.go.tmpl":         filepath.Join("internal", "usecases", snakeName, "list.go"),
		"update_usecase.go.tmpl":       filepath.Join("internal", "usecases", snakeName, "update.go"),
		"delete_usecase.go.tmpl":       filepath.Join("internal", "usecases", snakeName, "delete.go"),
		"repository_interface.go.tmpl": filepath.Join("internal", "usecases", snakeName, "interfaces", "repository.go"),
		"dto_create.go.tmpl":           filepath.Join("internal", "usecases", snakeName, "dto", "create.go"),
		"dto_get.go.tmpl":              filepath.Join("internal", "usecases", snakeName, "dto", "get.go"),
		"dto_list.go.tmpl":             filepath.Join("internal", "usecases", snakeName, "dto", "list.go"),
		"dto_update.go.tmpl":           filepath.Join("internal", "usecases", snakeName, "dto", "update.go"),
		"dto_delete.go.tmpl":           filepath.Join("internal", "usecases", snakeName, "dto", "delete.go"),
		"repository_postgres.go.tmpl":  filepath.Join("internal", "infrastructure", "db", "postgres", "repository", snakeName+".go"),
		"handler.go.tmpl":              filepath.Join("internal", "infrastructure", "web", "handler", snakeName+".go"),
		"router.go.tmpl":               filepath.Join("internal", "infrastructure", "web", "router", snakeName+".go"),
		"migration.sql.tmpl":           filepath.Join("internal", "infrastructure", "db", "postgres", "migration", migrationTimestamp+"_create_"+pluralName+".sql"),
	}
}
