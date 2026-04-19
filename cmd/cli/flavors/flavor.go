// Package flavors models the "harness template per service topology" concept
// from Fowler's Harness Engineering article: different service shapes (CRUD,
// event processor, data pipeline) each ship with a pre-wired set of guides
// and sensors appropriate for their topology.
//
// A Flavor is a named bundle of overlays applied on top of the base scaffold
// template. A Registry resolves flavor ids to Flavors at CLI parse time.
//
// See .specs/cli-harness-flavors.md for the full contract.
package flavors

// Flavor is a named variant of the `gopherplate new` scaffold.
//
// The ID is the value passed to `--flavor` and must be a slug (lowercase
// letters, digits, and hyphens). Description feeds the `--help` output.
// Overlays are applied in order on top of the base scaffold.
type Flavor struct {
	ID          string
	Description string
	Overlays    []Overlay
}

// Action names the kind of mutation an Overlay performs on the scaffold tree.
// See cmd/cli/flavors/overlay.go for the full semantics.
type Action string

const (
	// ActionCreate writes a new file at Path. Fails if the file already exists.
	ActionCreate Action = "create"
	// ActionAppend appends Template to the end of an existing file.
	ActionAppend Action = "append"
	// ActionInsertMarker inserts Template at a comment-marker line in the base
	// template (ex.: `# @flavor-makefile-targets`).
	ActionInsertMarker Action = "insert-marker"
	// ActionOverwrite replaces the file's contents entirely. Reserved for
	// cases where no additive form is possible; callers should prefer
	// insert-marker or append.
	ActionOverwrite Action = "overwrite"
	// ActionGoModRequire adds a `require` line to go.mod via golang.org/x/mod/modfile,
	// resolving version conflicts by picking the higher one.
	ActionGoModRequire Action = "go-mod-require"
)

// Overlay is one step in a Flavor's transformation of the scaffold.
//
// Path is relative to the scaffold root. For ActionInsertMarker, Marker names
// the comment-line the template is injected below. For ActionGoModRequire,
// Module is "path version" (e.g. "github.com/redis/go-redis/v9 v9.11.0") and
// Template must be empty.
type Overlay struct {
	Action   Action
	Path     string
	Template string
	Marker   string
	Module   string
}
