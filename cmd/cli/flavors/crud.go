package flavors

// Crud returns the `crud` flavor — HTTP+gRPC CRUD with Postgres, the project's
// default topology.
//
// In the MVP scope (see .specs/cli-harness-flavors.md § "Scope revision"), CRUD
// is a named pointer to the base scaffold behavior: the existing `gopherplate new`
// copy+rewrite flow already produces a working CRUD service, and the harness
// artifacts CRUD would nominally contribute (tests/load/baselines, .semgrep/,
// tests/testutil/golden) are already copied because they are NOT in
// cmd/cli/templates/gopherplate.ExcludePaths.
//
// .github/workflows/ (including perf-regression.yml and mutation-nightly.yml) IS
// excluded today — intentionally, per the exclude-list comment "teams add their
// own". Adding those back via CRUD overlays is left for a follow-up refinement
// once we have real user feedback on whether teams want them out of the box.
//
// For the CLI this is enough to prove the flavor plumbing end-to-end: lookup
// works, --flavor crud succeeds, scaffold builds and lints. The overlay engine
// is exercised by the unit-test fixtures; adding overlays to this flavor is a
// one-liner when a clear need emerges.
func Crud() Flavor {
	return Flavor{
		ID:          "crud",
		Description: "HTTP+gRPC CRUD with Postgres (default)",
		Overlays:    nil,
	}
}
