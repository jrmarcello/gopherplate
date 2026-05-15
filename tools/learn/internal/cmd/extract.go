// extract subcommand implementation. Mines repeated n-gram patterns out of
// the four ingest sources (spec, git, transcript, memory) and writes one
// JSONL Candidate per detected pattern to candidates.jsonl (REQ-2, REQ-2a).
// Sanitization (REQ-4) is enforced by every ingest package upstream — extract
// itself only operates on already-sanitized strings.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/jrmarcello/gopherplate/tools/learn/internal/config"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/ingest/git"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/ingest/memory"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/ingest/spec"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/ingest/transcript"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/learnerr"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/logging"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/pattern"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/sanitize"
	"github.com/jrmarcello/gopherplate/tools/learn/internal/store"
)

// extractNGramSizes are the n-gram window sizes the extractor sweeps for
// every kind. 2 covers tool pairs (Read→Edit), 3 covers triples
// (Read→Edit→Bash), 4 catches longer canonical sequences. Going wider has
// diminishing returns and explodes the candidate count, so we cap at 4.
var extractNGramSizes = []int{2, 3, 4}

// defaultOutFile is the path written when --out is omitted.
const defaultOutFile = "candidates.jsonl"

func init() {
	register(func(root *cobra.Command) {
		root.AddCommand(newExtractCmd())
	})
}

// newExtractCmd builds the `learn extract` cobra command.
func newExtractCmd() *cobra.Command {
	var (
		dbPath         string
		configPath     string
		specsDir       string
		repoDir        string
		transcriptsDir string
		memoryDir      string
		since          time.Duration
		out            string
		minFreq        int
	)

	c := &cobra.Command{
		Use:   "extract",
		Short: "Mine repeated patterns from ingest sources into candidates.jsonl",
		Long: `extract walks .specs/, the git log, transcripts, and memory entries,
extracts contiguous n-gram patterns above the configured frequency
threshold, and writes one Candidate per pattern to candidates.jsonl
(REQ-2). Output is deterministic: sorted lexicographically by (Kind,
Signature) so two runs on the same input yield byte-identical files.

Per-source failures (missing dir, parse warn) are logged via slog and the
extraction continues; only output-write and config errors hard-fail.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtract(cmd.Context(), extractOpts{
				DBPath:          dbPath,
				ConfigPath:      configPath,
				SpecsDir:        specsDir,
				RepoDir:         repoDir,
				TranscriptsDir:  transcriptsDir,
				MemoryDir:       memoryDir,
				Since:           since,
				Out:             out,
				MinFreqOverride: minFreq,
				Stderr:          cmd.ErrOrStderr(),
			})
		},
	}
	c.Flags().StringVar(&dbPath, "db-path", ".claude/learning/db.sqlite", "Path to the SQLite store")
	c.Flags().StringVar(&configPath, "config", ".claude/learning/config.yml", "Path to learning config")
	c.Flags().StringVar(&specsDir, "specs-dir", ".specs", "Directory containing SDD spec files")
	c.Flags().StringVar(&repoDir, "repo-dir", ".", "Git repository directory")
	c.Flags().StringVar(&transcriptsDir, "transcripts-dir", "", "Claude Code transcripts directory (skipped when empty)")
	c.Flags().StringVar(&memoryDir, "memory-dir", "memory", "Memory entries directory")
	c.Flags().DurationVar(&since, "since", 0, "Only consider records within this duration of now (0 = no filter)")
	c.Flags().StringVar(&out, "out", defaultOutFile, "Output candidates.jsonl path")
	c.Flags().IntVar(&minFreq, "min-freq", 0, "Override config.min_pattern_freq (0 = use config)")
	return c
}

// extractOpts is the typed input for runExtract. Tests inject Config and
// Now directly; the cobra command builds the struct from flags.
type extractOpts struct {
	DBPath          string
	ConfigPath      string
	SpecsDir        string
	RepoDir         string
	TranscriptsDir  string
	MemoryDir       string
	Since           time.Duration
	Out             string
	MinFreqOverride int
	Config          *config.Config
	Stderr          io.Writer
	Now             func() time.Time
}

// runExtract is the entry exercised by tests. Pipeline (REQ-2):
//  1. Load config + build sanitizer.
//  2. Ingest each source soft-fail per source.
//  3. Build per-kind token sequences grouped by Source.
//  4. ExtractNGrams over sizes 2,3,4 with min-freq filter.
//  5. Convert Counted → Candidate, score, gather contexts and time bounds.
//  6. Sort by (Kind, Signature) and write to Out.
//  7. Persist rows in patterns/candidates tables for the audit trail.
func runExtract(ctx context.Context, o extractOpts) error {
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC() }
	}
	if o.Out == "" {
		o.Out = defaultOutFile
	}

	logger := logging.New(o.Stderr, "extract")

	cfg := o.Config
	if cfg == nil {
		loaded, loadErr := config.Load(o.ConfigPath)
		if loadErr != nil {
			return learnerr.Runtimef("extract: load config: %w", loadErr)
		}
		cfg = loaded
	}

	minFreq := cfg.MinPatternFreq
	if o.MinFreqOverride > 0 {
		minFreq = o.MinFreqOverride
	}
	if minFreq < 1 {
		return learnerr.Usagef("extract: min_pattern_freq must be >= 1, got %d", minFreq)
	}

	san, sanErr := sanitize.NewFromConfig(cfg.SecretPatterns)
	if sanErr != nil {
		return learnerr.Runtimef("extract: build sanitizer: %w", sanErr)
	}

	now := o.Now().UTC()
	var windowStart time.Time
	if o.Since > 0 {
		windowStart = now.Add(-o.Since)
	}

	// === Ingest each source (soft-fail) ===
	specRecs := ingestSpecs(ctx, logger, o.SpecsDir, san, windowStart)
	gitRecs := ingestGit(ctx, logger, o.RepoDir, o.Since, san)
	transcriptRecs := ingestTranscripts(ctx, logger, o.TranscriptsDir, san, windowStart)
	memoryRecs := ingestMemory(ctx, logger, o.MemoryDir, san)
	// memoryRecs not currently used as a sequence source — memory entries
	// are curated singletons (one Record per file) and do not lend themselves
	// to n-gram mining the way ordered tool/file/commit sequences do.
	// Keeping the ingest call wired so soft-fail logging surfaces issues and
	// the source is reachable for future kinds.
	_ = memoryRecs

	// === Build per-kind sequences grouped by Source ===
	toolSeqs := buildToolSequences(transcriptRecs)
	fileSeqs := buildFileSequences(specRecs)
	commitSeqs := buildCommitSequences(gitRecs)

	// Per-source SeenAt map (min/max) keyed by (kind, sourceID).
	seenAt := newSeenAtIndex()
	seenAt.recordSourcesFromTranscripts(transcriptRecs)
	seenAt.recordSourcesFromSpecs(specRecs)
	seenAt.recordSourcesFromGit(gitRecs)

	// === Mine candidates per kind ===
	candidates := make([]pattern.Candidate, 0, 64)
	candidates = append(candidates, mineKind(pattern.KindToolSequence, toolSeqs, minFreq, seenAt)...)
	candidates = append(candidates, mineKind(pattern.KindFileSequence, fileSeqs, minFreq, seenAt)...)
	candidates = append(candidates, mineKind(pattern.KindCommitPattern, commitSeqs, minFreq, seenAt)...)
	// KindErrorFix: no source yet (REQ-2 deferred). Documented zero-emit.

	// === Sort deterministically (REQ-2) ===
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind < candidates[j].Kind
		}
		return candidates[i].Signature < candidates[j].Signature
	})

	// === Write output file ===
	if writeErr := writeCandidatesFile(o.Out, candidates); writeErr != nil {
		return writeErr
	}

	// === Persist into store (audit trail) ===
	if persistErr := persistCandidates(ctx, o.DBPath, candidates, now); persistErr != nil {
		return persistErr
	}

	logger.InfoContext(ctx, "extract complete",
		"candidates", len(candidates),
		"out", o.Out,
	)
	return nil
}

// ingestSpecs walks specsDir and returns spec Records, soft-failing each
// per-file error. windowStart, when non-zero, filters Records by SeenAt.
func ingestSpecs(ctx context.Context, logger *slog.Logger, specsDir string, san *sanitize.Sanitizer, windowStart time.Time) []spec.Record {
	if specsDir == "" {
		return nil
	}
	entries, readErr := os.ReadDir(specsDir)
	if readErr != nil {
		logger.WarnContext(ctx, "ingest source error", "source", "spec", "error", readErr.Error())
		return nil
	}
	var out []spec.Record
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 4 || name[len(name)-3:] != ".md" {
			continue
		}
		full := specsDir + "/" + name
		recs, parseErr := spec.Parse(ctx, full, san)
		if parseErr != nil {
			logger.WarnContext(ctx, "ingest source error", "source", "spec", "path", full, "error", parseErr.Error())
			continue
		}
		for _, r := range recs {
			if !windowStart.IsZero() && r.SeenAt.Before(windowStart) {
				continue
			}
			out = append(out, r)
		}
	}
	return out
}

// ingestGit runs ParseLog with the --since translated from a duration to the
// "<N>s" relative form git accepts. Soft-fails on parse error. An empty
// repoDir disables git ingestion entirely — required so test environments
// that do not bring their own repo do not silently inherit the surrounding
// repo's commits via git.ParseLog's "fall back to cwd" behavior.
func ingestGit(ctx context.Context, logger *slog.Logger, repoDir string, since time.Duration, san *sanitize.Sanitizer) []git.Record {
	if repoDir == "" {
		return nil
	}
	opts := git.Options{}
	if since > 0 {
		opts.Since = fmt.Sprintf("%d seconds ago", int64(since.Seconds()))
	}
	recs, parseErr := git.ParseLog(ctx, repoDir, opts, san)
	if parseErr != nil {
		logger.WarnContext(ctx, "ingest source error", "source", "git", "error", parseErr.Error())
		return nil
	}
	return recs
}

// ingestTranscripts walks the transcripts dir, soft-failing on any per-dir
// error. Records outside the time window are filtered.
func ingestTranscripts(ctx context.Context, logger *slog.Logger, transcriptsDir string, san *sanitize.Sanitizer, windowStart time.Time) []transcript.Record {
	if transcriptsDir == "" {
		return nil
	}
	recs, parseErr := transcript.ParseDir(ctx, transcriptsDir, san)
	if parseErr != nil {
		logger.WarnContext(ctx, "ingest source error", "source", "transcript", "error", parseErr.Error())
		return nil
	}
	if windowStart.IsZero() {
		return recs
	}
	filtered := make([]transcript.Record, 0, len(recs))
	for _, r := range recs {
		if r.SeenAt.Before(windowStart) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// ingestMemory parses memory entries. Memory is NOT time-filtered (curated
// content). Soft-fails on directory error.
func ingestMemory(ctx context.Context, logger *slog.Logger, memoryDir string, san *sanitize.Sanitizer) []memory.Record {
	if memoryDir == "" {
		return nil
	}
	recs, parseErr := memory.ParseDir(ctx, memoryDir, nil, san)
	if parseErr != nil {
		logger.WarnContext(ctx, "ingest source error", "source", "memory", "error", parseErr.Error())
		return nil
	}
	return recs
}

// sourceTokens is the intermediate shape used during sequence assembly:
// a single source's chronologically-ordered tokens, plus its identifier.
type sourceTokens struct {
	Source string
	Tokens []string
	First  time.Time
	Last   time.Time
}

// buildToolSequences groups transcript-tool-use records by Source (session
// id) and emits one token sequence per source in chronological order.
func buildToolSequences(recs []transcript.Record) []sourceTokens {
	type entry struct {
		ts    time.Time
		token string
	}
	bySource := make(map[string][]entry)
	for _, r := range recs {
		if r.Kind != "transcript-tool-use" {
			continue
		}
		if len(r.Tokens) == 0 {
			continue
		}
		bySource[r.Source] = append(bySource[r.Source], entry{ts: r.SeenAt, token: r.Tokens[0]})
	}
	out := make([]sourceTokens, 0, len(bySource))
	for src, es := range bySource {
		sort.Slice(es, func(i, j int) bool { return es[i].ts.Before(es[j].ts) })
		toks := make([]string, len(es))
		for i, e := range es {
			toks[i] = e.token
		}
		out = append(out, sourceTokens{
			Source: src,
			Tokens: toks,
			First:  es[0].ts,
			Last:   es[len(es)-1].ts,
		})
	}
	// Sort by Source for determinism of downstream context aggregation.
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// buildFileSequences groups spec-task records by Source (spec path) and
// emits the file/dir token sequence per spec in the order the parser saw
// them.
func buildFileSequences(recs []spec.Record) []sourceTokens {
	type entry struct {
		ts     time.Time
		tokens []string
	}
	bySource := make(map[string][]entry)
	for _, r := range recs {
		if r.Kind != spec.KindTask {
			continue
		}
		if len(r.Tokens) == 0 {
			continue
		}
		// Drop the leading TASK-N token; we want file/dir tokens only.
		toks := r.Tokens
		if len(toks) > 0 && len(toks[0]) >= 5 && toks[0][:5] == "TASK-" {
			toks = toks[1:]
		}
		if len(toks) == 0 {
			continue
		}
		bySource[r.Source] = append(bySource[r.Source], entry{ts: r.SeenAt, tokens: toks})
	}
	out := make([]sourceTokens, 0, len(bySource))
	for src, es := range bySource {
		sort.Slice(es, func(i, j int) bool { return es[i].ts.Before(es[j].ts) })
		var flat []string
		for _, e := range es {
			flat = append(flat, e.tokens...)
		}
		out = append(out, sourceTokens{
			Source: src,
			Tokens: flat,
			First:  es[0].ts,
			Last:   es[len(es)-1].ts,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// buildCommitSequences uses each commit Record as a single source: its
// Tokens (type, scope, dirs…) form one sequence. Each commit is its own
// "sequence container" so an n-gram inside the commit row counts.
func buildCommitSequences(recs []git.Record) []sourceTokens {
	out := make([]sourceTokens, 0, len(recs))
	for _, r := range recs {
		if len(r.Tokens) == 0 {
			continue
		}
		out = append(out, sourceTokens{
			Source: r.Source,
			Tokens: append([]string(nil), r.Tokens...),
			First:  r.SeenAt,
			Last:   r.SeenAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// seenAtIndex tracks per-(kind, signature) context sources and time bounds.
type seenAtIndex struct {
	// sourceTime[sourceID] = the time the source contributed (min seen for
	// transcript sessions, the commit time for git, etc.).
	sourceTime map[string]time.Time
}

func newSeenAtIndex() *seenAtIndex {
	return &seenAtIndex{sourceTime: make(map[string]time.Time)}
}

func (s *seenAtIndex) record(source string, t time.Time) {
	if existing, ok := s.sourceTime[source]; ok {
		if t.Before(existing) {
			s.sourceTime[source] = t
		}
		return
	}
	s.sourceTime[source] = t
}

func (s *seenAtIndex) recordSourcesFromTranscripts(recs []transcript.Record) {
	for _, r := range recs {
		s.record(r.Source, r.SeenAt)
	}
}

func (s *seenAtIndex) recordSourcesFromSpecs(recs []spec.Record) {
	for _, r := range recs {
		s.record(r.Source, r.SeenAt)
	}
}

func (s *seenAtIndex) recordSourcesFromGit(recs []git.Record) {
	for _, r := range recs {
		s.record(r.Source, r.SeenAt)
	}
}

// mineKind extracts n-grams for the given kind across the supplied
// per-source sequences, builds Candidates, and returns them.
//
// Contexts (unique source IDs) and FirstSeenAt/LastSeenAt are derived by
// walking the sequences in parallel with the n-gram extraction to record
// which sources contributed each signature.
func mineKind(kind pattern.Kind, sources []sourceTokens, minFreq int, idx *seenAtIndex) []pattern.Candidate {
	if len(sources) == 0 {
		return nil
	}

	// Per-signature accumulators.
	type acc struct {
		count       int
		contexts    map[string]struct{}
		firstSeenAt time.Time
		lastSeenAt  time.Time
	}
	accs := make(map[string]*acc)

	for _, size := range extractNGramSizes {
		// Walk each source's sequence inline (so context attribution stays
		// exact) and tally n-grams. Cannot reuse pattern.ExtractNGrams here
		// because it discards the per-source provenance — but we still call
		// it once over all sequences to drive the canonical signature/count
		// shape and final min-freq filter.
		for _, src := range sources {
			if len(src.Tokens) < size {
				continue
			}
			for i := 0; i+size <= len(src.Tokens); i++ {
				sig := joinNGram(src.Tokens[i : i+size])
				a, ok := accs[sig]
				if !ok {
					a = &acc{contexts: make(map[string]struct{})}
					accs[sig] = a
				}
				a.count++
				a.contexts[src.Source] = struct{}{}
				if a.firstSeenAt.IsZero() || src.First.Before(a.firstSeenAt) {
					a.firstSeenAt = src.First
				}
				if src.Last.After(a.lastSeenAt) {
					a.lastSeenAt = src.Last
				}
			}
		}
	}

	// Find the max count to feed into Score normalization (REQ-2).
	maxCount := 0
	for _, a := range accs {
		if a.count >= minFreq && a.count > maxCount {
			maxCount = a.count
		}
	}

	candidates := make([]pattern.Candidate, 0, len(accs))
	for sig, a := range accs {
		if a.count < minFreq {
			continue
		}
		contexts := make([]string, 0, len(a.contexts))
		for src := range a.contexts {
			contexts = append(contexts, src)
		}
		sort.Strings(contexts)
		first := a.firstSeenAt
		last := a.lastSeenAt
		if first.IsZero() {
			first = last
		}
		if first.After(last) {
			first, last = last, first
		}
		candidates = append(candidates, pattern.Candidate{
			Kind:        kind,
			Signature:   sig,
			Frequency:   a.count,
			Contexts:    contexts,
			Score:       pattern.Score(a.count, maxCount),
			FirstSeenAt: first.UTC(),
			LastSeenAt:  last.UTC(),
		})
	}
	// idx parameter currently unused beyond the per-source seenAt registry
	// that already feeds into a.firstSeenAt/a.lastSeenAt via sourceTokens
	// First/Last. The argument is kept on the signature so future kinds
	// (memory, error-fix) can plug their own source-time map in without a
	// signature break.
	_ = idx
	return candidates
}

// joinNGram delegates to pattern.Sep so the separator is single-sourced.
// Previously this file duplicated the literal; now it imports the canonical
// constant to prevent drift between extract and pattern packages.

func joinNGram(tokens []string) string {
	switch len(tokens) {
	case 0:
		return ""
	case 1:
		return tokens[0]
	}
	// Manual join — strings.Join would also work; preferring an explicit
	// builder keeps the hot loop allocation-free for the common 2/3/4 case.
	n := 0
	for _, t := range tokens {
		n += len(t)
	}
	n += len(pattern.Sep) * (len(tokens) - 1)
	buf := make([]byte, 0, n)
	for i, t := range tokens {
		if i > 0 {
			buf = append(buf, pattern.Sep...)
		}
		buf = append(buf, t...)
	}
	return string(buf)
}

// writeCandidatesFile creates (or truncates) the output file and writes one
// pattern.Marshal line per candidate. An empty slice yields a zero-byte file
// — REQ-5's "no sources → empty output" path. The output is fsynced before
// rename via a temp-then-rename pattern so a crash mid-write does not leave
// a half-written candidates.jsonl.
func writeCandidatesFile(path string, candidates []pattern.Candidate) error {
	tmp, createErr := os.CreateTemp(dirOf(path), ".candidates-*.tmp")
	if createErr != nil {
		return learnerr.Runtimef("extract: create temp output: %w", createErr)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	for i := range candidates {
		line, marshalErr := pattern.Marshal(&candidates[i])
		if marshalErr != nil {
			_ = tmp.Close()
			cleanup()
			return learnerr.Runtimef("extract: marshal candidate %d: %w", i, marshalErr)
		}
		if _, writeErr := tmp.Write(line); writeErr != nil {
			_ = tmp.Close()
			cleanup()
			return learnerr.Runtimef("extract: write candidate %d: %w", i, writeErr)
		}
	}
	if syncErr := tmp.Sync(); syncErr != nil {
		_ = tmp.Close()
		cleanup()
		return learnerr.Runtimef("extract: sync output: %w", syncErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		cleanup()
		return learnerr.Runtimef("extract: close output: %w", closeErr)
	}
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		cleanup()
		return learnerr.Runtimef("extract: rename %q → %q: %w", tmpPath, path, renameErr)
	}
	return nil
}

// dirOf returns the directory portion of path, defaulting to "." when there
// is no separator. Used to anchor os.CreateTemp on the same filesystem as
// the final output so the rename is atomic.
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == os.PathSeparator {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

// persistCandidates upserts each Candidate into the patterns + candidates
// tables for the audit trail. The store is opened by path; runs without a
// db file (empty DBPath) are tolerated for unit tests that don't care about
// persistence — but typical CLI invocations always supply a path.
func persistCandidates(ctx context.Context, dbPath string, candidates []pattern.Candidate, now time.Time) error {
	if dbPath == "" {
		return nil
	}
	st, openErr := store.Open(dbPath)
	if openErr != nil {
		// Audit-trail persistence is best-effort on a clean DB; surface the
		// failure as Runtime so the operator sees it but only when there's
		// no other way to recover.
		return learnerr.Runtimef("extract: open store: %w", openErr)
	}
	defer func() { _ = st.Close() }()

	db := st.DB()
	for i := range candidates {
		c := &candidates[i]
		contextsJSON, marshalErr := json.Marshal(c.Contexts)
		if marshalErr != nil {
			return learnerr.Runtimef("extract: marshal contexts: %w", marshalErr)
		}

		// Upsert patterns row. ON CONFLICT(signature) updates frequency, last_seen_at, score.
		const upsertPattern = `
INSERT INTO patterns (kind, signature, frequency, first_seen_at, last_seen_at, score)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(signature) DO UPDATE SET
    frequency = excluded.frequency,
    last_seen_at = excluded.last_seen_at,
    score = excluded.score
RETURNING id`
		var patternID int64
		scanErr := db.QueryRowContext(ctx, upsertPattern,
			string(c.Kind),
			c.Signature,
			c.Frequency,
			c.FirstSeenAt.Format(time.RFC3339Nano),
			c.LastSeenAt.Format(time.RFC3339Nano),
			c.Score,
		).Scan(&patternID)
		if scanErr != nil {
			return learnerr.Runtimef("extract: upsert pattern %q: %w", c.Signature, scanErr)
		}

		const insertCandidate = `
INSERT INTO candidates (pattern_id, contexts_json, score, created_at)
VALUES (?, ?, ?, ?)`
		if _, execErr := db.ExecContext(ctx, insertCandidate,
			patternID,
			string(contextsJSON),
			c.Score,
			now.Format(time.RFC3339Nano),
		); execErr != nil {
			return learnerr.Runtimef("extract: insert candidate row: %w", execErr)
		}
	}
	return nil
}
