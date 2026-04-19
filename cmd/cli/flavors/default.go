package flavors

// Default returns the package-level registry pre-populated with every flavor
// the CLI currently knows about. This is the single entrypoint consumers
// (cmd/cli/commands/new.go) use to resolve --flavor values.
//
// Extending: to add a new flavor, append its constructor to the slice below
// and add the constructor file in this package. The registry constructs fresh
// on every call — no global mutable state between CLI invocations.
func Default() *Registry {
	reg := NewRegistry()
	flavors := []Flavor{
		Crud(),
		// Future (deferred to .specs/flavors-event-data.md):
		//   EventProcessor(),
		//   DataPipeline(),
	}
	for _, f := range flavors {
		// Register never fails on fresh registry with unique ids hard-coded here;
		// panic on this would be a programming error (duplicate constructor).
		if regErr := reg.Register(f); regErr != nil {
			panic("flavors: internal error: " + regErr.Error())
		}
	}
	return reg
}
