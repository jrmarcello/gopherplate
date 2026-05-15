// record-decision subcommand (REQ-5, REQ-9). Inserts one row into the
// `decisions` table and, for `new-skill`/`new-memory`/`update`, materializes
// the corresponding target file (frontmatter + body) so the learning-loop's
// decision is observable on disk.
//
// For `pending-approval` the file mutation is deferred: the row carries the
// proposed diff and a follow-up `apply-decision <id>` (see apply_decision.go)
// performs the actual move. For `discard` no file mutation happens.
package learn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validRecordActions is the subset of `decisions.action` values accepted by
// `record-decision`. The `applied` value is reserved for `apply-decision`.
var validRecordActions = map[string]struct{}{
	"new-skill":        {},
	"new-memory":       {},
	"update":           {},
	"discard":          {},
	"pending-approval": {},
}

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newRecordDecisionCmd())
	})
}

// newRecordDecisionCmd builds the `learn record-decision` cobra command.
func newRecordDecisionCmd() *cobra.Command {
	var (
		candidateSig string
		action       string
		targetPath   string
		rationale    string
		diffFile     string
		bodyFile     string
		description  string
		mode         string
		dbPath       string
	)

	c := &cobra.Command{
		Use:   "record-decision",
		Short: "Record a learning-loop decision and materialize the target file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordDecision(cmd.Context(), recordDecisionOpts{
				CandidateSignature: candidateSig,
				Action:             action,
				TargetPath:         targetPath,
				Rationale:          rationale,
				DiffFile:           diffFile,
				BodyFile:           bodyFile,
				Description:        description,
				Mode:               mode,
				DBPath:             dbPath,
			})
		},
	}
	c.Flags().StringVar(&candidateSig, "candidate-signature", "", "Candidate signature being decided on (required)")
	c.Flags().StringVar(&action, "action", "", "Decision action: new-skill|new-memory|update|discard|pending-approval")
	c.Flags().StringVar(&targetPath, "target-path", "", "Path of the skill/memory file affected")
	c.Flags().StringVar(&rationale, "rationale", "", "Free-form rationale for the decision")
	c.Flags().StringVar(&diffFile, "diff-file", "", "Path to a file containing the proposed diff (pending-approval)")
	c.Flags().StringVar(&bodyFile, "body-file", "", "Path to a file containing the body for new-skill/new-memory/update")
	c.Flags().StringVar(&description, "description", "", "Description for the frontmatter of a new file")
	c.Flags().StringVar(&mode, "mode", "append", "Update mode for action=update: append|replace")
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store")
	return c
}

// recordDecisionOpts is the typed input to runRecordDecision.
type recordDecisionOpts struct {
	CandidateSignature string
	Action             string
	TargetPath         string
	Rationale          string
	DiffFile           string
	BodyFile           string
	Description        string
	Mode               string
	DBPath             string
	// Now is injected by tests; defaults to time.Now().UTC().
	Now func() time.Time
}

// runRecordDecision performs validation, inserts the decision row, and
// materializes the target file according to the action.
func runRecordDecision(ctx context.Context, o recordDecisionOpts) error {
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.CandidateSignature == "" {
		return usagef("record-decision: --candidate-signature is required")
	}
	if _, ok := validRecordActions[o.Action]; !ok {
		return usagef("record-decision: invalid --action %q (want one of new-skill|new-memory|update|discard|pending-approval)", o.Action)
	}
	if o.DBPath == "" {
		return usagef("record-decision: --db-path must be non-empty")
	}

	// Per-action validation of the target-path requirement: only `discard`
	// is allowed to omit it because no file is touched.
	if o.Action != "discard" && o.TargetPath == "" {
		return usagef("record-decision: --target-path is required for action %q", o.Action)
	}

	var diffContent string
	if o.DiffFile != "" {
		raw, readErr := os.ReadFile(o.DiffFile)
		if readErr != nil {
			return runtimef("record-decision: read --diff-file %q: %w", o.DiffFile, readErr)
		}
		diffContent = string(raw)
	}

	// Atomicity: file mutation runs FIRST, but we register a defer that
	// rolls back the file write on DB-insert failure. This avoids both
	// failure modes:
	//   - DB inserts succeeds but file write fails → no row, no file (clean)
	//   - File write succeeds but DB insert fails → file rolled back, no
	//     orphan that `reindex` would pick up as a phantom skill.
	// For `update` we capture the original bytes before write and restore
	// them on rollback; for `new-skill`/`new-memory` we simply remove the
	// freshly-created file.
	var fileRollback func()
	switch o.Action {
	case "new-skill", "new-memory":
		if writeErr := writeNewLearningFile(o); writeErr != nil {
			return writeErr
		}
		target := o.TargetPath
		fileRollback = func() { _ = os.Remove(target) }
	case "update":
		original, captureErr := os.ReadFile(o.TargetPath)
		if captureErr != nil && !errors.Is(captureErr, os.ErrNotExist) {
			return runtimef("record-decision: capture pre-update %q: %w", o.TargetPath, captureErr)
		}
		if updErr := applyUpdateToLearningFile(o); updErr != nil {
			return updErr
		}
		target := o.TargetPath
		hadOriginal := captureErr == nil
		orig := original
		fileRollback = func() {
			if hadOriginal {
				_ = os.WriteFile(target, orig, 0o600) //nolint:gosec // target captured pre-update; same provenance
			} else {
				_ = os.Remove(target)
			}
		}
	case "discard", "pending-approval":
		// No file mutation. pending-approval keeps the diff in the row only.
	}

	st, openErr := openStore(o.DBPath)
	if openErr != nil {
		if fileRollback != nil {
			fileRollback()
		}
		return runtimef("record-decision: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	const insertDecision = `
INSERT INTO decisions (candidate_signature, action, target_path, rationale, diff, decided_at)
VALUES (?, ?, ?, ?, ?, ?)`
	if _, execErr := st.DB().ExecContext(ctx, insertDecision,
		o.CandidateSignature,
		o.Action,
		nullIfEmpty(o.TargetPath),
		o.Rationale,
		nullIfEmpty(diffContent),
		o.Now().UTC().Format(time.RFC3339Nano),
	); execErr != nil {
		if fileRollback != nil {
			fileRollback()
		}
		return runtimef("record-decision: insert decision: %w", execErr)
	}
	return nil
}

// writeNewLearningFile creates a new skill or memory file with the minimal
// frontmatter described in the spec (TASK-19) and the body sourced from
// --body-file. If the target file already exists the call is a usage error —
// the caller should pick `update` instead.
func writeNewLearningFile(o recordDecisionOpts) error {
	if o.BodyFile == "" {
		return usagef("record-decision: --body-file is required for action %q", o.Action)
	}
	body, readErr := os.ReadFile(o.BodyFile)
	if readErr != nil {
		return runtimef("record-decision: read --body-file %q: %w", o.BodyFile, readErr)
	}
	if _, statErr := os.Stat(o.TargetPath); statErr == nil {
		return usagef("record-decision: target %q already exists (use --action=update)", o.TargetPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return runtimef("record-decision: stat %q: %w", o.TargetPath, statErr)
	}

	name := deriveNameFromPath(o.TargetPath)
	now := o.Now().UTC().Format(time.RFC3339)

	header := buildFrontmatter(name, o.Description, o.CandidateSignature, now)
	if mkErr := os.MkdirAll(filepath.Dir(o.TargetPath), 0o750); mkErr != nil {
		return runtimef("record-decision: mkdir parent of %q: %w", o.TargetPath, mkErr)
	}

	final := header + ensureTrailingNewline(string(body))
	// o.TargetPath comes from --target-path CLI flag — caller-controlled by
	// design (purpose of new-skill/new-memory subcommand).
	if writeErr := os.WriteFile(o.TargetPath, []byte(final), 0o600); writeErr != nil { //nolint:gosec
		return runtimef("record-decision: write %q: %w", o.TargetPath, writeErr)
	}
	return nil
}

// applyUpdateToLearningFile appends (or replaces) the body of an existing
// learning file and bumps `last_reviewed_at` in its frontmatter. Other
// frontmatter fields are preserved verbatim.
func applyUpdateToLearningFile(o recordDecisionOpts) error {
	if o.BodyFile == "" {
		return usagef("record-decision: --body-file is required for action %q", o.Action)
	}
	body, readErr := os.ReadFile(o.BodyFile)
	if readErr != nil {
		return runtimef("record-decision: read --body-file %q: %w", o.BodyFile, readErr)
	}
	existing, statErr := os.ReadFile(o.TargetPath)
	if statErr != nil {
		return runtimef("record-decision: read target %q: %w", o.TargetPath, statErr)
	}

	fmMap, bodyText, splitErr := splitFrontmatterRaw(existing)
	if splitErr != nil {
		return usagef("record-decision: parse target %q: %w", o.TargetPath, splitErr)
	}
	fmMap["last_reviewed_at"] = o.Now().UTC().Format(time.RFC3339)

	mode := o.Mode
	if mode == "" {
		mode = "append"
	}
	var newBody string
	switch mode {
	case "append":
		newBody = ensureTrailingNewline(bodyText) + ensureTrailingNewline(string(body))
	case "replace":
		newBody = ensureTrailingNewline(string(body))
	default:
		return usagef("record-decision: invalid --mode %q (want append|replace)", mode)
	}

	header, marshalErr := marshalFrontmatterMap(fmMap)
	if marshalErr != nil {
		return runtimef("record-decision: marshal frontmatter: %w", marshalErr)
	}
	final := header + newBody
	// o.TargetPath validated above; comes from --target-path CLI flag
	// (caller-controlled by design — purpose of the subcommand).
	if writeErr := os.WriteFile(o.TargetPath, []byte(final), 0o600); writeErr != nil { //nolint:gosec
		return runtimef("record-decision: write %q: %w", o.TargetPath, writeErr)
	}
	return nil
}

// buildFrontmatter produces the canonical minimal frontmatter for a freshly
// created learning file (TASK-19 spec wording). The `learning_provenance`
// list is seeded with the candidate signature that prompted creation.
func buildFrontmatter(name, description, provenance, ts string) string {
	type fm struct {
		Name               string   `yaml:"name"`
		Description        string   `yaml:"description"`
		LearningProvenance []string `yaml:"learning_provenance"`
		CreatedAt          string   `yaml:"created_at"`
		LastReviewedAt     string   `yaml:"last_reviewed_at"`
	}
	v := fm{
		Name:               name,
		Description:        description,
		LearningProvenance: []string{provenance},
		CreatedAt:          ts,
		LastReviewedAt:     ts,
	}
	body, marshalErr := yaml.Marshal(&v)
	if marshalErr != nil {
		// yaml.Marshal of a fixed struct cannot realistically fail; fall back to
		// a hand-built fragment to keep the call infallible from the caller's
		// perspective.
		return fmt.Sprintf("---\nname: %s\ndescription: %s\nlearning_provenance:\n  - %s\ncreated_at: %s\nlast_reviewed_at: %s\n---\n", name, description, provenance, ts, ts)
	}
	return "---\n" + string(body) + "---\n"
}

// deriveNameFromPath returns the basename of path minus its extension.
// `path/to/foo.md` -> `foo`; `SKILL.md` -> `SKILL`. The result is used as the
// `name` frontmatter field when the caller doesn't supply one explicitly.
func deriveNameFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

// splitFrontmatterRaw parses a markdown file that begins with `---\n...\n---\n`
// and returns the frontmatter as an unstructured map (preserving unknown
// fields) plus the remaining body. Files without frontmatter return an error.
func splitFrontmatterRaw(raw []byte) (frontmatter map[string]any, body string, err error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---") {
		return nil, "", errors.New("missing frontmatter: file does not start with '---'")
	}
	rest := text[3:]
	if !strings.HasPrefix(rest, "\n") {
		return nil, "", errors.New("missing frontmatter: opening delimiter not followed by newline")
	}
	rest = rest[1:]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return nil, "", errors.New("missing frontmatter: closing '---' not found")
	}
	yamlBlock := rest[:closeIdx]
	body = strings.TrimPrefix(rest[closeIdx+len("\n---"):], "\n")
	m := make(map[string]any)
	if unmarshalErr := yaml.Unmarshal([]byte(yamlBlock), &m); unmarshalErr != nil {
		return nil, "", fmt.Errorf("invalid YAML frontmatter: %w", unmarshalErr)
	}
	return m, body, nil
}

// marshalFrontmatterMap serializes an unstructured frontmatter map back into
// a `---\n<yaml>\n---\n` block. yaml.v3 sorts map keys alphabetically which
// is the desired stable output for this project.
func marshalFrontmatterMap(m map[string]any) (string, error) {
	body, marshalErr := yaml.Marshal(m)
	if marshalErr != nil {
		return "", marshalErr
	}
	return "---\n" + string(body) + "---\n", nil
}

// ensureTrailingNewline appends '\n' to s when missing. A no-op on empty
// strings (we don't want to inject a stray blank line into an empty body).
func ensureTrailingNewline(s string) string {
	if s == "" {
		return s
	}
	if s[len(s)-1] == '\n' {
		return s
	}
	return s + "\n"
}

// nullIfEmpty turns "" into a sql.NullString sentinel so the column is NULL
// rather than the empty string — this matters for the `target_path` and
// `diff` columns which are nullable by schema.
func nullIfEmpty(s string) any {
	if s == "" {
		return sql.NullString{}
	}
	return s
}
