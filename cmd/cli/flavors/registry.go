package flavors

import (
	"fmt"
	"sort"
	"strings"
)

// Registry resolves flavor ids to Flavor definitions.
//
// Zero value is NOT usable — always construct via NewRegistry. The CLI
// process holds a single global registry populated at init time by each
// flavor's package.
type Registry struct {
	byID map[string]Flavor
}

// NewRegistry returns an empty registry ready for Register.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[string]Flavor)}
}

// Register adds a flavor. Returns an error if a flavor with the same ID
// was already registered — protects against silent shadowing when a new
// flavor package forgets to pick a unique name.
func (r *Registry) Register(f Flavor) error {
	if _, exists := r.byID[f.ID]; exists {
		return fmt.Errorf("flavors: duplicate id %q", f.ID)
	}
	r.byID[f.ID] = f
	return nil
}

// Get resolves an id to its Flavor. On miss, the error message lists the
// currently-registered flavors so `--flavor foo` gives a useful CLI hint.
func (r *Registry) Get(id string) (Flavor, error) {
	f, ok := r.byID[id]
	if !ok {
		return Flavor{}, fmt.Errorf("flavors: unknown flavor %q; available: %s",
			id, strings.Join(r.List(), ", "))
	}
	return f, nil
}

// List returns the registered flavor ids in stable alphabetical order so
// `--help` output is deterministic across process runs.
func (r *Registry) List() []string {
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
