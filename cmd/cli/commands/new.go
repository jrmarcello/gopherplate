package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jrmarcello/go-boilerplate/cmd/cli/scaffold"
	boilerplatetmpl "github.com/jrmarcello/go-boilerplate/cmd/cli/templates/boilerplate"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [service-name]",
	Short: "Cria um novo microserviço a partir do template",
	Long: `Cria um novo microserviço Go com Clean Architecture a partir do template.

Exemplos:
  boilerplate new my-service
  boilerplate new my-service --module github.com/org/my-service --db postgres
  boilerplate new my-service --module github.com/org/my-service --no-redis --no-auth`,
	Args: cobra.MaximumNArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().String("module", "", "Go module path (ex: github.com/org/service)")
	newCmd.Flags().String("db", "", "Database driver: postgres, mysql, sqlite3, other")
	newCmd.Flags().String("template", ".", "Path to the template project root")
	newCmd.Flags().Bool("no-redis", false, "Disable Redis cache")
	newCmd.Flags().Bool("no-idempotency", false, "Disable idempotency middleware")
	newCmd.Flags().Bool("no-auth", false, "Disable service key authentication")
	newCmd.Flags().Bool("no-examples", false, "Remove example domains (user/role)")
	newCmd.Flags().Bool("keep-examples", false, "Keep example domains (user/role)")
	newCmd.Flags().BoolP("yes", "y", false, "Accept defaults for all unspecified options (non-interactive)")
}

const templateModulePath = "github.com/jrmarcello/go-boilerplate"

func runNew(cmd *cobra.Command, args []string) error {
	cfg := scaffold.DefaultConfig()
	reader := bufio.NewReader(os.Stdin)
	acceptDefaults, _ := cmd.Flags().GetBool("yes")
	interactive := isInteractive() && !acceptDefaults

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
		redisVal, redisErr := promptConfirm(reader, "Incluir cache Redis?", true)
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
		idempVal, idempErr := promptConfirm(reader, "Incluir idempotência?", true)
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
		authVal, authErr := promptConfirm(reader, "Incluir Service Key Auth?", true)
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
		exVal, exErr := promptConfirm(reader, "Manter domínios de exemplo (user/role)?", true)
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
	templateDir, _ := cmd.Flags().GetString("template")
	templateDir, absErr := filepath.Abs(templateDir)
	if absErr != nil {
		return fmt.Errorf("resolving template path: %w", absErr)
	}

	// 2. Validate output directory doesn't exist
	outputDir := filepath.Join(".", cfg.ServiceName)
	if _, statErr := os.Stat(outputDir); statErr == nil {
		return fmt.Errorf("directory '%s' already exists", cfg.ServiceName)
	}

	// 3. Copy project
	fmt.Printf("\nCriando %s...\n\n", cfg.ServiceName)
	copyErr := boilerplatetmpl.CopyProject(templateDir, outputDir)
	if copyErr != nil {
		return fmt.Errorf("copying project: %w", copyErr)
	}

	// 4. Rewrite module path
	fmt.Println("  Rewriting module path...")
	rewriteErr := scaffold.RewriteModulePath(outputDir, templateModulePath, cfg.ModulePath)
	if rewriteErr != nil {
		return fmt.Errorf("rewriting module path: %w", rewriteErr)
	}

	// 5. Replace service name in configs
	fmt.Println("  Replacing service name...")
	renameErr := boilerplatetmpl.ReplaceServiceName(outputDir, cfg.ServiceName)
	if renameErr != nil {
		return fmt.Errorf("replacing service name: %w", renameErr)
	}

	// 6. Switch DB driver if needed
	if cfg.DB != scaffold.DBPostgres {
		fmt.Printf("  Switching DB driver to %s...\n", cfg.DB)
		dbErr := boilerplatetmpl.SwitchDBDriver(outputDir, string(cfg.DB))
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

	// 9. Clean up template-specific files that shouldn't be in new projects
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

	// 10. Initialize fresh git
	fmt.Println("  Initializing git...")
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = outputDir
	if gitInitErr := gitCmd.Run(); gitInitErr != nil {
		fmt.Fprintf(os.Stderr, "  Warning: git init failed: %v\n", gitInitErr)
	}

	// 11. Run go mod tidy
	fmt.Println("  Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = outputDir
	tidyOut, tidyErr := tidyCmd.CombinedOutput()
	if tidyErr != nil {
		fmt.Fprintf(os.Stderr, "  Warning: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
	}

	// 12. Print success summary
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

func promptConfirm(reader *bufio.Reader, label string, defaultVal bool) (bool, error) { //nolint:unparam // defaultVal kept for API flexibility
	defaultHint := "Y/n"
	if !defaultVal {
		defaultHint = "y/N"
	}

	fmt.Printf("  %s [%s]: ", label, defaultHint)

	line, readErr := reader.ReadString('\n')
	if readErr != nil {
		return false, fmt.Errorf("reading input: %w", readErr)
	}

	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultVal, nil
	}

	switch line {
	case "y", "yes", "s", "sim":
		return true, nil
	case "n", "no", "não", "nao":
		return false, nil
	default:
		return defaultVal, nil
	}
}

func boolToYesNo(v bool) string {
	if v {
		return "sim"
	}
	return "não"
}

// isInteractive returns true if stdin is a terminal (TTY).
// When not interactive, prompts are skipped and defaults are used.
func isInteractive() bool {
	fi, statErr := os.Stdin.Stat()
	if statErr != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
