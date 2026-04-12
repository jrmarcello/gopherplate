package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jrmarcello/gopherplate/cmd/cli/scaffold"
	domaintmpl "github.com/jrmarcello/gopherplate/cmd/cli/templates/domain"
)

var addDomainCmd = &cobra.Command{
	Use:   "domain [name]",
	Short: "Add a new domain to the project",
	Long: `Scaffolds a complete Clean Architecture domain including:
  - Domain layer (entity, errors, value objects)
  - Use cases (CRUD operations with interfaces and DTOs)
  - Infrastructure (repository, handler, router)
  - Database migration`,
	Args: cobra.ExactArgs(1),
	RunE: runAddDomain,
}

// domainNamePattern validates domain names: lowercase letters, digits, underscores; must start with a letter.
var domainNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func runAddDomain(_ *cobra.Command, args []string) error {
	domainName := args[0]

	// 1. Detect module path from go.mod in current directory
	modulePath, detectErr := detectModulePath()
	if detectErr != nil {
		return fmt.Errorf("detecting module path: %w (are you in a Go project directory?)", detectErr)
	}

	// 2. Validate domain name (lowercase, no spaces, alphanumeric + underscores)
	if validateErr := validateDomainName(domainName); validateErr != nil {
		return validateErr
	}

	// 3. Normalize to snake_case for path resolution
	snakeName := scaffold.ToSnakeCase(domainName)

	// 4. Check if domain already exists
	domainDir := filepath.Join("internal", "domain", snakeName)
	if _, statErr := os.Stat(domainDir); statErr == nil {
		return fmt.Errorf("domain '%s' already exists at %s", snakeName, domainDir)
	}

	// 5. Build TemplateData
	cfg := scaffold.Config{ModulePath: modulePath}
	data := scaffold.NewTemplateData(domainName, cfg)

	// 6. Generate migration timestamp (Goose-compatible: YYYYMMDDHHMMSS)
	migrationTimestamp := time.Now().Format("20060102150405")

	// 7. Build template-to-output-path mappings
	templateMappings := buildTemplateMappings(snakeName, migrationTimestamp)

	// 8. Render and write each template
	fmt.Println()
	fmt.Printf("Scaffolding domain '%s'...\n\n", snakeName)

	createdFiles := make([]string, 0, len(templateMappings))

	for tmplName, outputPath := range templateMappings {
		tmplContent, readErr := fs.ReadFile(domaintmpl.Templates, tmplName)
		if readErr != nil {
			return fmt.Errorf("reading template %s: %w", tmplName, readErr)
		}

		rendered, renderErr := scaffold.RenderTemplate(string(tmplContent), data)
		if renderErr != nil {
			return fmt.Errorf("rendering template %s: %w", tmplName, renderErr)
		}

		dirPath := filepath.Dir(outputPath)
		if mkdirErr := os.MkdirAll(dirPath, 0o750); mkdirErr != nil {
			return fmt.Errorf("creating directory %s: %w", dirPath, mkdirErr)
		}

		if writeErr := os.WriteFile(outputPath, []byte(rendered), 0o600); writeErr != nil {
			return fmt.Errorf("writing file %s: %w", outputPath, writeErr)
		}

		createdFiles = append(createdFiles, outputPath)
		fmt.Printf("  \u2713 %s\n", outputPath)
	}

	// 9. Print summary
	fmt.Printf("\n%d files created.\n", len(createdFiles))

	// 10. Print wiring instructions
	printWiringInstructions(data)

	return nil
}

// detectModulePath reads go.mod from the current directory and extracts the module path.
func detectModulePath() (string, error) {
	content, readErr := os.ReadFile("go.mod")
	if readErr != nil {
		return "", readErr
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			modulePath := strings.TrimPrefix(trimmed, "module ")
			modulePath = strings.TrimSpace(modulePath)
			if modulePath == "" {
				return "", fmt.Errorf("empty module path in go.mod")
			}
			return modulePath, nil
		}
	}

	return "", fmt.Errorf("module directive not found in go.mod")
}

// validateDomainName checks that the name is valid for a Go package.
// Must be lowercase, start with a letter, and contain only a-z, 0-9, underscore.
func validateDomainName(name string) error {
	if name == "" {
		return fmt.Errorf("domain name cannot be empty")
	}

	// Normalize: convert hyphens/camelCase to snake_case for validation
	normalized := scaffold.ToSnakeCase(name)

	if !domainNamePattern.MatchString(normalized) {
		return fmt.Errorf(
			"invalid domain name '%s' (normalized: '%s'): must start with a letter and contain only lowercase letters, digits, and underscores",
			name, normalized,
		)
	}

	return nil
}

// buildTemplateMappings creates the map from template filename to output path.
func buildTemplateMappings(snakeName, migrationTimestamp string) map[string]string {
	pluralName := scaffold.ToPlural(snakeName)

	return map[string]string{
		// Domain layer
		"entity.go.tmpl": filepath.Join("internal", "domain", snakeName, "entity.go"),
		"errors.go.tmpl": filepath.Join("internal", "domain", snakeName, "errors.go"),
		"filter.go.tmpl": filepath.Join("internal", "domain", snakeName, "filter.go"),

		// Use cases
		"create_usecase.go.tmpl": filepath.Join("internal", "usecases", snakeName, "create.go"),
		"get_usecase.go.tmpl":    filepath.Join("internal", "usecases", snakeName, "get.go"),
		"list_usecase.go.tmpl":   filepath.Join("internal", "usecases", snakeName, "list.go"),
		"update_usecase.go.tmpl": filepath.Join("internal", "usecases", snakeName, "update.go"),
		"delete_usecase.go.tmpl": filepath.Join("internal", "usecases", snakeName, "delete.go"),

		// Use case interfaces
		"repository_interface.go.tmpl": filepath.Join("internal", "usecases", snakeName, "interfaces", "repository.go"),

		// DTOs
		"dto_create.go.tmpl": filepath.Join("internal", "usecases", snakeName, "dto", "create.go"),
		"dto_get.go.tmpl":    filepath.Join("internal", "usecases", snakeName, "dto", "get.go"),
		"dto_list.go.tmpl":   filepath.Join("internal", "usecases", snakeName, "dto", "list.go"),
		"dto_update.go.tmpl": filepath.Join("internal", "usecases", snakeName, "dto", "update.go"),
		"dto_delete.go.tmpl": filepath.Join("internal", "usecases", snakeName, "dto", "delete.go"),

		// Infrastructure - Repository
		"repository_postgres.go.tmpl": filepath.Join("internal", "infrastructure", "db", "postgres", "repository", snakeName+".go"),

		// Infrastructure - Handler
		"handler.go.tmpl": filepath.Join("internal", "infrastructure", "web", "handler", snakeName+".go"),

		// Infrastructure - Router
		"router.go.tmpl": filepath.Join("internal", "infrastructure", "web", "router", snakeName+".go"),

		// Migration
		"migration.sql.tmpl": filepath.Join("internal", "infrastructure", "db", "postgres", "migration", migrationTimestamp+"_create_"+pluralName+".sql"),
	}
}

// printWiringInstructions prints the manual wiring steps the developer needs to perform.
func printWiringInstructions(data scaffold.TemplateData) {
	fmt.Println("\n\U0001f4cb Pr\u00f3ximos passos (wiring manual):")

	// Step 1: server.go wiring
	fmt.Println("\n1. Adicione em cmd/api/server.go (buildDependencies):")
	fmt.Println()
	fmt.Printf("   // --- %s Domain ---\n", data.DomainNamePascal)
	fmt.Printf("   %sRepo := repository.New%sRepository(sqlxWriter, sqlxReader)\n", data.DomainNameCamel, data.DomainNamePascal)
	fmt.Printf("   %sCreateUC := %suc.NewCreateUseCase(%sRepo)\n", data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel)
	fmt.Printf("   %sGetUC := %suc.NewGetUseCase(%sRepo)\n", data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel)
	fmt.Printf("   %sListUC := %suc.NewListUseCase(%sRepo)\n", data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel)
	fmt.Printf("   %sUpdateUC := %suc.NewUpdateUseCase(%sRepo)\n", data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel)
	fmt.Printf("   %sDeleteUC := %suc.NewDeleteUseCase(%sRepo)\n", data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel)
	fmt.Printf("   %sHandler := handler.New%sHandler(%sCreateUC, %sGetUC, %sListUC, %sUpdateUC, %sDeleteUC)\n",
		data.DomainNameCamel, data.DomainNamePascal,
		data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel, data.DomainNameCamel)
	fmt.Println()
	fmt.Println("   Adicione o import:")
	fmt.Printf("   %suc \"%s/internal/usecases/%s\"\n", data.DomainNameCamel, data.ModulePath, data.DomainNameSnake)
	fmt.Println()
	fmt.Printf("   Adicione ao retorno de Dependencies:\n")
	fmt.Printf("   %sHandler: %sHandler,\n", data.DomainNamePascal, data.DomainNameCamel)

	// Step 2: router.go wiring
	fmt.Println("\n2. Adicione em internal/infrastructure/web/router/router.go:")
	fmt.Printf("   router.Register%sRoutes(protected, deps.%sHandler)\n", data.DomainNamePascal, data.DomainNamePascal)

	// Step 3: Dependencies struct
	fmt.Println("\n3. Adicione o campo na struct Dependencies (router/router.go):")
	fmt.Printf("   %sHandler *handler.%sHandler\n", data.DomainNamePascal, data.DomainNamePascal)

	// Step 4: Run migration
	fmt.Println("\n4. Execute a migration:")
	fmt.Println("   make migrate-up")
}
