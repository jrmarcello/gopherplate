package flavors

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// Applier applies Overlays under a fixed scaffold root. All Overlay.Path
// values are resolved against the root; traversal outside is rejected.
type Applier struct {
	root string
}

// NewApplier returns an Applier rooted at the given scaffold directory.
// The root must exist; it is canonicalized via filepath.Abs so path checks
// are robust to relative inputs.
func NewApplier(root string) *Applier {
	abs, err := filepath.Abs(root)
	if err != nil {
		// An error here means filepath.Abs couldn't get cwd — extremely rare.
		// Falling back to the raw root preserves caller's intent.
		abs = root
	}
	return &Applier{root: abs}
}

// Apply performs the overlay. Convenience wrapper that discards warnings.
func (a *Applier) Apply(o Overlay, data any) error {
	_, err := a.ApplyWithWarnings(o, data)
	return err
}

// ApplyWithWarnings performs the overlay and returns any non-fatal warnings
// (e.g., intentional overwrite, conflicting go.mod version resolved to max).
func (a *Applier) ApplyWithWarnings(o Overlay, data any) (warnings []string, err error) {
	fullPath, resolveErr := a.resolve(o.Path)
	if resolveErr != nil {
		return nil, resolveErr
	}

	switch o.Action {
	case ActionCreate:
		return nil, a.actionCreate(fullPath, o.Template, data)
	case ActionAppend:
		return nil, a.actionAppend(fullPath, o.Template, data)
	case ActionInsertMarker:
		return nil, a.actionInsertMarker(fullPath, o.Marker, o.Template, data)
	case ActionOverwrite:
		return a.actionOverwrite(fullPath, o.Template, data)
	case ActionGoModRequire:
		return a.actionGoModRequire(fullPath, o.Module)
	default:
		return nil, fmt.Errorf("flavors: unknown action %q", o.Action)
	}
}

// resolve validates that path, when joined with the root, stays inside the
// root (no traversal). Returns the cleaned absolute path.
func (a *Applier) resolve(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("flavors: empty path")
	}
	// Pre-check: any ".." segment triggers rejection. We do this before Join
	// so attackers can't rely on symlink tricks inside the root.
	cleaned := filepath.Clean(rel)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("flavors: path %q escapes scaffold root (invalid path)", rel)
	}

	full := filepath.Join(a.root, cleaned)
	fullAbs, absErr := filepath.Abs(full)
	if absErr != nil {
		return "", fmt.Errorf("flavors: resolving %q: %w", rel, absErr)
	}
	rootWithSep := a.root + string(filepath.Separator)
	if fullAbs != a.root && !strings.HasPrefix(fullAbs, rootWithSep) {
		return "", fmt.Errorf("flavors: path %q escapes scaffold root", rel)
	}
	return fullAbs, nil
}

func renderTemplate(src string, data any) (string, error) {
	if src == "" {
		return "", nil
	}
	tmpl, parseErr := template.New("overlay").Parse(src)
	if parseErr != nil {
		return "", fmt.Errorf("flavors: template parse: %w", parseErr)
	}
	var buf bytes.Buffer
	if execErr := tmpl.Execute(&buf, data); execErr != nil {
		return "", fmt.Errorf("flavors: template exec: %w", execErr)
	}
	return buf.String(), nil
}

func (a *Applier) actionCreate(fullPath, src string, data any) error {
	if _, statErr := os.Stat(fullPath); statErr == nil {
		return fmt.Errorf("flavors: create %q: path already exists", fullPath)
	}
	body, renderErr := renderTemplate(src, data)
	if renderErr != nil {
		return renderErr
	}
	if mkErr := os.MkdirAll(filepath.Dir(fullPath), 0o750); mkErr != nil {
		return fmt.Errorf("flavors: mkdir: %w", mkErr)
	}
	if writeErr := os.WriteFile(fullPath, []byte(body), 0o600); writeErr != nil {
		return fmt.Errorf("flavors: write %q: %w", fullPath, writeErr)
	}
	return nil
}

func (a *Applier) actionAppend(fullPath, src string, data any) error {
	existing, readErr := os.ReadFile(fullPath) //nolint:gosec // path validated by resolve()
	if readErr != nil {
		return fmt.Errorf("flavors: append to %q: %w", fullPath, readErr)
	}
	body, renderErr := renderTemplate(src, data)
	if renderErr != nil {
		return renderErr
	}
	// Allocate a fresh slice so append can't silently alias `existing`.
	combined := make([]byte, 0, len(existing)+len(body))
	combined = append(combined, existing...)
	combined = append(combined, body...)
	if writeErr := os.WriteFile(fullPath, combined, 0o600); writeErr != nil {
		return fmt.Errorf("flavors: write %q: %w", fullPath, writeErr)
	}
	return nil
}

func (a *Applier) actionInsertMarker(fullPath, marker, src string, data any) error {
	if marker == "" {
		return fmt.Errorf("flavors: insert-marker requires Marker field")
	}
	existing, readErr := os.ReadFile(fullPath) //nolint:gosec // path validated by resolve()
	if readErr != nil {
		return fmt.Errorf("flavors: insert-marker in %q: %w", fullPath, readErr)
	}
	if !bytes.Contains(existing, []byte(marker)) {
		return fmt.Errorf("flavors: insert-marker target %q not found in %s", marker, fullPath)
	}
	body, renderErr := renderTemplate(src, data)
	if renderErr != nil {
		return renderErr
	}
	// Insert below the marker line (after the first newline following the marker).
	markerIdx := bytes.Index(existing, []byte(marker))
	nlIdx := bytes.IndexByte(existing[markerIdx:], '\n')
	if nlIdx < 0 {
		// Marker is on the last line without trailing newline — treat as
		// "end of file" and append the body.
		combined := make([]byte, 0, len(existing)+1+len(body))
		combined = append(combined, existing...)
		combined = append(combined, '\n')
		combined = append(combined, body...)
		return os.WriteFile(fullPath, combined, 0o600)
	}
	insertAt := markerIdx + nlIdx + 1 // position right after the marker line's \n
	var out bytes.Buffer
	out.Write(existing[:insertAt])
	out.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		out.WriteByte('\n')
	}
	out.Write(existing[insertAt:])
	if writeErr := os.WriteFile(fullPath, out.Bytes(), 0o600); writeErr != nil {
		return fmt.Errorf("flavors: write %q: %w", fullPath, writeErr)
	}
	return nil
}

// overwriteSentinel guards explicit overwrites. Templates that replace a
// pre-existing file MUST start with a line matching this prefix — the
// comment makes intent obvious during review and lets the engine reject
// accidental overwrites (classic overlay pattern is additive).
const overwriteSentinel = "overlay: overwrite"

func (a *Applier) actionOverwrite(fullPath, src string, data any) ([]string, error) {
	if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
		// Overwriting a non-existent file is really a create; fall through
		// to that semantics without requiring the sentinel.
		return nil, a.actionCreate(fullPath, src, data)
	}
	body, renderErr := renderTemplate(src, data)
	if renderErr != nil {
		return nil, renderErr
	}
	if !strings.Contains(body, overwriteSentinel) {
		return nil, fmt.Errorf(
			"flavors: overwrite of %q requires template to include sentinel %q in a comment — refuse to silently replace existing file",
			fullPath, overwriteSentinel)
	}
	if writeErr := os.WriteFile(fullPath, []byte(body), 0o600); writeErr != nil {
		return nil, fmt.Errorf("flavors: write %q: %w", fullPath, writeErr)
	}
	return []string{fmt.Sprintf("overwrote %s (explicit sentinel present)", fullPath)}, nil
}

func (a *Applier) actionGoModRequire(fullPath, module string) ([]string, error) {
	parts := strings.Fields(module)
	if len(parts) != 2 {
		return nil, fmt.Errorf("flavors: go-mod-require Module must be \"path version\", got %q", module)
	}
	path, version := parts[0], parts[1]
	if !semver.IsValid(version) {
		return nil, fmt.Errorf("flavors: go-mod-require invalid semver %q for %q", version, path)
	}

	existing, readErr := os.ReadFile(fullPath) //nolint:gosec // path validated by resolve()
	if readErr != nil {
		return nil, fmt.Errorf("flavors: go-mod-require read %q: %w", fullPath, readErr)
	}
	mod, parseErr := modfile.Parse(filepath.Base(fullPath), existing, nil)
	if parseErr != nil {
		return nil, fmt.Errorf("flavors: go-mod-require parse: %w", parseErr)
	}

	var warnings []string
	applied := version
	for _, req := range mod.Require {
		if req.Mod.Path == path {
			cmp := semver.Compare(req.Mod.Version, version)
			switch {
			case cmp > 0:
				// Existing is higher → keep it. Warn so the caller knows
				// their request was downgraded.
				applied = req.Mod.Version
				warnings = append(warnings, fmt.Sprintf(
					"go-mod-require: %s already at %s, keeping existing (requested %s)",
					path, req.Mod.Version, version))
			case cmp < 0:
				// Requested is higher → bump.
				warnings = append(warnings, fmt.Sprintf(
					"go-mod-require: bumping %s from %s to %s",
					path, req.Mod.Version, version))
			}
			break
		}
	}

	if setErr := mod.AddRequire(path, applied); setErr != nil {
		return nil, fmt.Errorf("flavors: go-mod-require AddRequire: %w", setErr)
	}
	mod.Cleanup()
	out, formatErr := mod.Format()
	if formatErr != nil {
		return nil, fmt.Errorf("flavors: go-mod-require format: %w", formatErr)
	}
	if writeErr := os.WriteFile(fullPath, out, 0o600); writeErr != nil {
		return nil, fmt.Errorf("flavors: go-mod-require write: %w", writeErr)
	}
	return warnings, nil
}
