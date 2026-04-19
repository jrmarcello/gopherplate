package flavors

// Base returns the implicit base flavor.
//
// The base scaffold is NOT a set of template files — it is the live project
// tree (everything outside cmd/cli/templates/gopherplate.ExcludePaths), rewritten
// for the new module path and service name by cmd/cli/scaffold. That copy+rewrite
// flow is what every flavor starts from; flavors add overlays on top of the
// resulting tree.
//
// Consequently the Base flavor carries ZERO overlays: it exists purely as a
// named placeholder so registry lookups and --help listings are uniform, and so
// a future flavor author has a clear answer to "what if I want vanilla behavior
// with one tiny tweak — do I duplicate the whole template tree?" (No — just
// register a Flavor with a single overlay.)
//
// Insert-marker strategy: the spec designs ActionInsertMarker for extending
// base files (Makefile, server.go, ci.yml) from flavors. For the MVP scope
// (crud flavor only), CRUD's overlays are all additive-create (new semgrep
// rules, new k6 baseline, new workflow file). No base-template markers are
// required yet. Markers will be added to base files by the follow-up spec
// .specs/flavors-event-data.md when event-processor and data-pipeline need to
// inject into the Makefile and DI wiring.
func Base() Flavor {
	return Flavor{
		ID:          "base",
		Description: "Implicit base scaffold (not directly selectable via --flavor)",
	}
}
