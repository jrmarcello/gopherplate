package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jrmarcello/gopherplate/cmd/cli/scaffold"
	gopherplatetmpl "github.com/jrmarcello/gopherplate/cmd/cli/templates/gopherplate"
	"github.com/spf13/cobra"
)

// goModTidyTimeout limits how long `go mod tidy` can run before we give up.
// Private modules without GOPRIVATE/auth configured can make tidy hang
// indefinitely while Go tries to fetch from the public proxy.
const goModTidyTimeout = 30 * time.Second

var newCmd = &cobra.Command{
	Use:   "new [service-name]",
	Short: "Cria um novo microserviço a partir do template",
	Long: `Cria um novo microserviço Go com Clean Architecture a partir do template.

Exemplos:
  gopherplate new my-service
  gopherplate new my-service --module github.com/org/my-service --db postgres
  gopherplate new my-service --module github.com/org/my-service --no-redis --no-auth`,
	Args: cobra.MaximumNArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().String("module", "", "Go module path (ex: github.com/org/service)")
	newCmd.Flags().String("db", "", "Database driver: postgres, mysql, sqlite3, other")
	newCmd.Flags().String("template", "", "Path to the template project root (auto-detected from $GOPHERPLATE_TEMPLATE, cwd, or the CLI binary location when omitted)")
	newCmd.Flags().Bool("no-redis", false, "Disable Redis cache")
	newCmd.Flags().Bool("no-idempotency", false, "Disable idempotency middleware")
	newCmd.Flags().Bool("no-auth", false, "Disable service key authentication")
	newCmd.Flags().Bool("no-examples", false, "Remove example domains (user/role)")
	newCmd.Flags().Bool("keep-examples", false, "Keep example domains (user/role)")
	newCmd.Flags().BoolP("yes", "y", false, "Accept defaults for all unspecified options (non-interactive)")
}

// templateEnvVar lets users pin the template path explicitly, overriding all
// auto-detection strategies below it in the resolution chain.
const templateEnvVar = "GOPHERPLATE_TEMPLATE"

func runNew(cmd *cobra.Command, args []string) error {
	cfg := scaffold.DefaultConfig()
	reader := bufio.NewReader(os.Stdin)
	acceptDefaults, _ := cmd.Flags().GetBool("yes")
	interactive := IsInteractive() && !acceptDefaults

	// --- Collect configuration from args/flags/prompts ---

	// Service name
	if len(args) > 0 {
		cfg.ServiceName = args[0]
	}
	if cfg.ServiceName == "" && interactive {
		promptVal, promptErr := promptInput(reader, "Nome do serviço", "")
		if promptErr != nil {
			return promptErr
		}
		cfg.ServiceName = promptVal
	}
	if cfg.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	// Module path
	modulePath, _ := cmd.Flags().GetString("module")
	switch {
	case modulePath != "":
		cfg.ModulePath = modulePath
	case interactive:
		defaultModule := fmt.Sprintf("github.com/appmax/%s", cfg.ServiceName)
		promptVal, promptErr := promptInput(reader, "Module path", defaultModule)
		if promptErr != nil {
			return promptErr
		}
		cfg.ModulePath = promptVal
	default:
		cfg.ModulePath = fmt.Sprintf("github.com/appmax/%s", cfg.ServiceName)
	}

	// Database
	dbFlag, _ := cmd.Flags().GetString("db")
	if dbFlag != "" {
		cfg.DB = scaffold.DBDriver(dbFlag)
	} else if interactive {
		dbVal, dbPromptErr := promptSelect(reader, "Banco de dados", []string{"postgres", "mysql", "sqlite3", "other"}, "postgres")
		if dbPromptErr != nil {
			return dbPromptErr
		}
		cfg.DB = scaffold.DBDriver(dbVal)
	}
	// else: keep default (postgres)

	// Protocol (currently HTTP only, show "coming soon" for others)
	cfg.Protocol = scaffold.ProtocolHTTP
	fmt.Println("\n  Protocolo: HTTP/REST (Gin) [gRPC: em breve]")

	// DI (currently manual only, show "coming soon" for others)
	cfg.DI = scaffold.DIManual
	fmt.Println("  Injeção de dependência: Manual [Uber Fx: em breve]")

	// Redis
	noRedis, _ := cmd.Flags().GetBool("no-redis")
	if noRedis {
		cfg.Redis = false
	} else if interactive && !cmd.Flags().Changed("no-redis") {
		redisVal, redisErr := PromptConfirm(reader, "Incluir cache Redis?", true)
		if redisErr != nil {
			return redisErr
		}
		cfg.Redis = redisVal
	}
	// else: keep default (true)

	// Idempotency (only if Redis is enabled)
	noIdempotency, _ := cmd.Flags().GetBool("no-idempotency")
	switch {
	case !cfg.Redis:
		cfg.Idempotency = false
	case noIdempotency:
		cfg.Idempotency = false
	case interactive && !cmd.Flags().Changed("no-idempotency"):
		idempVal, idempErr := PromptConfirm(reader, "Incluir idempotência?", true)
		if idempErr != nil {
			return idempErr
		}
		cfg.Idempotency = idempVal
	}

	// Auth
	noAuth, _ := cmd.Flags().GetBool("no-auth")
	if noAuth {
		cfg.Auth = false
	} else if interactive && !cmd.Flags().Changed("no-auth") {
		authVal, authErr := PromptConfirm(reader, "Incluir Service Key Auth?", true)
		if authErr != nil {
			return authErr
		}
		cfg.Auth = authVal
	}
	// else: keep default (true)

	// Keep examples
	noExamples, _ := cmd.Flags().GetBool("no-examples")
	keepExamples, _ := cmd.Flags().GetBool("keep-examples")
	switch {
	case noExamples:
		cfg.KeepExamples = false
	case keepExamples:
		cfg.KeepExamples = true
	case interactive && !cmd.Flags().Changed("no-examples") && !cmd.Flags().Changed("keep-examples"):
		exVal, exErr := PromptConfirm(reader, "Manter domínios de exemplo (user/role)?", true)
		if exErr != nil {
			return exErr
		}
		cfg.KeepExamples = exVal
	}

	// --- Summary ---
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Resumo")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Serviço:      %s\n", cfg.ServiceName)
	fmt.Printf("  Module:       %s\n", cfg.ModulePath)
	fmt.Printf("  Banco:        %s\n", cfg.DB)
	fmt.Printf("  Protocolo:    %s\n", cfg.Protocol)
	fmt.Printf("  DI:           %s\n", cfg.DI)
	fmt.Printf("  Redis:        %s\n", boolToYesNo(cfg.Redis))
	fmt.Printf("  Idempotência: %s\n", boolToYesNo(cfg.Idempotency))
	fmt.Printf("  Auth:         %s\n", boolToYesNo(cfg.Auth))
	fmt.Printf("  Exemplos:     %s\n", boolToYesNo(cfg.KeepExamples))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// --- Execution flow ---

	// 1. Determine source directory (template root)
	templateFlag, _ := cmd.Flags().GetString("template")
	templateDir, resolveErr := resolveTemplateRoot(templateFlag)
	if resolveErr != nil {
		return resolveErr
	}

	// 2. Validate output directory doesn't exist
	outputDir := filepath.Join(".", cfg.ServiceName)
	if _, statErr := os.Stat(outputDir); statErr == nil {
		return fmt.Errorf("directory '%s' already exists", cfg.ServiceName)
	}

	// 3. Copy project
	fmt.Printf("\nCriando %s...\n\n", cfg.ServiceName)
	copyErr := gopherplatetmpl.CopyProject(templateDir, outputDir)
	if copyErr != nil {
		return fmt.Errorf("copying project: %w", copyErr)
	}

	// 4. Rewrite module path
	fmt.Println("  Rewriting module path...")
	rewriteErr := scaffold.RewriteModulePath(outputDir, scaffold.TemplateModulePath, cfg.ModulePath)
	if rewriteErr != nil {
		return fmt.Errorf("rewriting module path: %w", rewriteErr)
	}

	// 5. Replace service name in configs
	fmt.Println("  Replacing service name...")
	renameErr := gopherplatetmpl.ReplaceServiceName(outputDir, cfg.ServiceName)
	if renameErr != nil {
		return fmt.Errorf("replacing service name: %w", renameErr)
	}

	// 6. Switch DB driver if needed
	if cfg.DB != scaffold.DBPostgres {
		fmt.Printf("  Switching DB driver to %s...\n", cfg.DB)
		dbErr := gopherplatetmpl.SwitchDBDriver(outputDir, string(cfg.DB))
		if dbErr != nil {
			return fmt.Errorf("switching DB driver: %w", dbErr)
		}
	}

	// 7. Remove disabled features
	fmt.Println("  Removing disabled features...")
	removeErr := scaffold.RemoveDisabledFeatures(outputDir, cfg)
	if removeErr != nil {
		return fmt.Errorf("removing disabled features: %w", removeErr)
	}

	// 8. Clean up wiring in server.go and router.go
	fmt.Println("  Cleaning up wiring...")
	wiringErr := scaffold.CleanupWiring(outputDir, cfg)
	if wiringErr != nil {
		return fmt.Errorf("cleaning up wiring: %w", wiringErr)
	}

	// 9. Clean up template-specific files
	fmt.Println("  Cleaning up template files...")
	// Remove template specs (keep only TEMPLATE.md, .gitkeep, .gitignore)
	specsDir := filepath.Join(outputDir, ".specs")
	if entries, readErr := os.ReadDir(specsDir); readErr == nil {
		specsAllowList := map[string]bool{
			"TEMPLATE.md": true,
			".gitkeep":    true,
			".gitignore":  true,
		}
		for _, entry := range entries {
			if !specsAllowList[entry.Name()] {
				_ = os.Remove(filepath.Join(specsDir, entry.Name()))
			}
		}
	}

	// 10. Reset CHANGELOG.md (template history replaced with empty starter)
	changelogPath := filepath.Join(outputDir, "CHANGELOG.md")
	changelogContent := "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\nFormat based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).\n"
	_ = os.WriteFile(changelogPath, []byte(changelogContent), 0o644) //nolint:gosec // CLI scaffold writes project files

	// 11. Clean up generated files that shouldn't be in new projects
	cleanupFiles := []string{
		"docs/swagger.json",
		"docs/swagger.yaml",
	}
	if !cfg.KeepExamples {
		// docs.go is only needed when swagger annotations exist (examples kept)
		cleanupFiles = append(cleanupFiles, "docs/docs.go")
	}
	for _, f := range cleanupFiles {
		_ = os.Remove(filepath.Join(outputDir, f))
	}

	// 11. Initialize fresh git
	fmt.Println("  Initializing git...")
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = outputDir
	if gitInitErr := gitCmd.Run(); gitInitErr != nil {
		fmt.Fprintf(os.Stderr, "  Warning: git init failed: %v\n", gitInitErr)
	}

	// 12. Run go mod tidy (with timeout: private modules without GOPRIVATE/auth
	// configured can make tidy hang indefinitely while Go tries the public proxy).
	fmt.Printf("  Running go mod tidy (timeout %s)...\n", goModTidyTimeout)
	tidyCtx, tidyCancel := context.WithTimeout(context.Background(), goModTidyTimeout)
	defer tidyCancel()
	tidyCmd := exec.CommandContext(tidyCtx, "go", "mod", "tidy")
	tidyCmd.Dir = outputDir
	tidyOut, tidyErr := tidyCmd.CombinedOutput()
	switch {
	case errors.Is(tidyCtx.Err(), context.DeadlineExceeded):
		fmt.Fprintf(os.Stderr, "  Warning: go mod tidy timed out after %s.\n", goModTidyTimeout)
		fmt.Fprintln(os.Stderr, "  This usually means the module path points to a private repo without auth.")
		fmt.Fprintln(os.Stderr, "  Configure GOPRIVATE and credentials, then run 'go mod tidy' manually.")
	case tidyErr != nil:
		fmt.Fprintf(os.Stderr, "  Warning: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
	}

	// 13. Print success summary
	fmt.Printf("\nProjeto '%s' criado com sucesso!\n\n", cfg.ServiceName)
	fmt.Println("Próximos passos:")
	fmt.Printf("  cd %s\n", cfg.ServiceName)
	fmt.Println("  make setup     # Instala tools + sobe Docker + roda migrations")
	fmt.Println("  make dev       # Inicia servidor com hot reload")
	fmt.Println("")

	return nil
}

// --- Prompt helpers (simple stdin-based, no bubbletea dependency) ---

func promptInput(reader *bufio.Reader, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("\n  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("\n  %s: ", label)
	}

	line, readErr := reader.ReadString('\n')
	if readErr != nil {
		return "", fmt.Errorf("reading input: %w", readErr)
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}

	return line, nil
}

func promptSelect(reader *bufio.Reader, label string, options []string, defaultVal string) (string, error) {
	fmt.Printf("\n  %s (%s) [%s]: ", label, strings.Join(options, "/"), defaultVal)

	line, readErr := reader.ReadString('\n')
	if readErr != nil {
		return "", fmt.Errorf("reading input: %w", readErr)
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}

	// Validate the selection
	for _, opt := range options {
		if line == opt {
			return line, nil
		}
	}

	return "", fmt.Errorf("invalid option '%s'; choose from: %s", line, strings.Join(options, ", "))
}

func boolToYesNo(v bool) string {
	if v {
		return "sim"
	}
	return "não"
}

// resolveTemplateRoot returns the absolute path to the gopherplate template
// directory, trying (in order):
//  1. the --template flag (if non-empty and not the legacy ".")
//  2. the $GOPHERPLATE_TEMPLATE env var
//  3. the current working directory
//  4. the directory tree around the running CLI binary (os.Executable())
//
// A candidate is accepted only if its go.mod declares the template module
// path. When all strategies fail, it returns an error that explains how to
// fix the situation.
func resolveTemplateRoot(flagValue string) (string, error) {
	var candidates []string

	// 1. Explicit --template flag (accept "." for backwards compatibility,
	//    but still validate it like any other candidate).
	if flagValue != "" {
		abs, absErr := filepath.Abs(flagValue)
		if absErr != nil {
			return "", fmt.Errorf("resolving --template path: %w", absErr)
		}
		if isGopherplateRoot(abs) {
			return abs, nil
		}
		return "", fmt.Errorf(
			"--template %q does not point to a gopherplate checkout (no go.mod with module %s)",
			flagValue, scaffold.TemplateModulePath,
		)
	}

	// 2. Env var override (useful when the CLI is installed via `go install`
	//    and the user wants a fixed template location).
	if envPath := os.Getenv(templateEnvVar); envPath != "" {
		abs, absErr := filepath.Abs(envPath)
		if absErr == nil && isGopherplateRoot(abs) {
			return abs, nil
		}
		return "", fmt.Errorf(
			"$%s=%q does not point to a gopherplate checkout (no go.mod with module %s)",
			templateEnvVar, envPath, scaffold.TemplateModulePath,
		)
	}

	// 3. Current working directory (and its ancestors) — covers the common
	//    case of running the CLI from inside the gopherplate repo.
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		candidates = append(candidates, cwd)
	}

	// 4. Directory tree around the running binary — covers `go run ./cmd/cli`
	//    and `./bin/gopherplate` invocations from within the source tree.
	if exe, exeErr := os.Executable(); exeErr == nil {
		if resolved, symErr := filepath.EvalSymlinks(exe); symErr == nil {
			exe = resolved
		}
		candidates = append(candidates, filepath.Dir(exe))
	}

	for _, start := range candidates {
		if found, ok := walkUpForTemplate(start); ok {
			return found, nil
		}
	}

	return "", fmt.Errorf(
		"could not locate the gopherplate template automatically; "+
			"run from inside a gopherplate checkout, set $%s to the checkout path, "+
			"or pass --template <path>",
		templateEnvVar,
	)
}

// walkUpForTemplate walks from start up to the filesystem root looking for a
// directory whose go.mod declares the gopherplate template module.
func walkUpForTemplate(start string) (string, bool) {
	dir := start
	for {
		if isGopherplateRoot(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// isGopherplateRoot reports whether dir contains a go.mod whose module
// directive matches the gopherplate template module path.
func isGopherplateRoot(dir string) bool {
	data, readErr := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec // CLI reads well-known project manifest
	if readErr != nil {
		return false
	}
	needle := []byte("module " + scaffold.TemplateModulePath)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			return strings.TrimSpace(trimmed) == string(needle)
		}
	}
	return false
}
