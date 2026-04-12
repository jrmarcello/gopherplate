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

	// Claude Code — only exclude runtime/local artifacts
	".claude/worktrees/",
	".claude/settings.local.json",

	// Git directory
	".git/",

	// Build artifacts
	"bin/",

	// Test artifacts
	"tests/coverage/",
	"tests/load/results/",

	// CI pipeline (template-specific, teams add their own)
	".github/",

	// Changelog (template history excluded, but cliff.toml + empty CHANGELOG kept)
	// cliff.toml is project-generic config; CHANGELOG.md is reset in post-processing

	// Template-specific files
	"roadmap.md",
	"docs/guides/template-cli.md",

	// Docs modules (auto-generated)
	"docs/modules/",

	// Environment file with real values
	".env",
}
