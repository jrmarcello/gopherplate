// Package boilerplate provides the project snapshot and copy logic for the
// `gopherplate new` command. Instead of maintaining Go templates for every file,
// we copy the actual project tree and run post-processing (module rewrite,
// service name replacement, feature removal, DB driver switch).
package boilerplate

// ExcludePaths lists path prefixes that must never be copied to new projects.
// Paths ending with "/" are treated as directory prefixes; paths without "/" are
// matched exactly against the relative path.
var ExcludePaths = []string{
	// CLI itself (new projects don't need the scaffold tool)
	"cmd/cli/",

	// Claude/AI configuration
	".claude/",
	"CLAUDE.md",
	"AGENTS.md",

	// Specs (template development artifacts)
	".specs/",

	// DevContainer (template-specific)
	".devcontainer/",

	// Git directory
	".git/",

	// Build artifacts
	"bin/",

	// Test artifacts
	"tests/coverage/",
	"tests/load/results/",

	// CI pipeline (template-specific, teams add their own)
	".github/",

	// Changelog tooling (template-specific)
	"cliff.toml",
	"CHANGELOG.md",

	// Contributing guide (template-specific)
	"CONTRIBUTING.md",

	// Docs modules (auto-generated)
	"docs/modules/",

	// Markdownlint config (template-specific)
	".markdownlint.json",

	// Environment file with real values
	".env",
}
