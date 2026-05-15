# Pre-refactor baseline — `tools/learn/`

Captured by TASK-BASELINE of `.specs/refactor-tools-learn-flat-layout.md`.

## Invariants

- `make learn-build`: PASS
- `make learn-lint`: PASS (0 issues)
- `go test ./...`: PASS across 15 packages (210 tests total)
- Module path: `github.com/jrmarcello/gopherplate/tools/learn`

## Test count per package

| Package | Tests |
|---------|-------|
| `internal/audit` | 7 |
| `internal/cli` | 3 |
| `internal/cmd` (15 files) | 78 |
| `internal/config` | 12 |
| `internal/ingest/git` | 11 |
| `internal/ingest/memory` | 8 |
| `internal/ingest/spec` | 7 |
| `internal/ingest/transcript` | 8 |
| `internal/learnerr` | 16 |
| `internal/logging` | 2 |
| `internal/pattern` (ngram 10 + schema 13) | 23 |
| `internal/recall` | 6 |
| `internal/sanitize` | 8 |
| `internal/similar` (bm25 8 + levenshtein 4) | 12 |
| `internal/store` | 9 |
| **Total** | **210** |

## `learn --help` subcommands (canonical 16)

`apply-decision`, `audit-skills-prep`, `complete-task`, `extract`, `init`,
`nudge-tick`, `recall`, `record-decision`, `refine-apply`, `reindex`,
`rollback`, `similar`, `smoke`, `stats`, `track-use`, `validate-skill`.

Plus cobra auto-injected: `completion`, `help`.

## `learn stats` JSON keys (against existing DB)

```json
{
  "counts": {"events", "patterns", "candidates", "decisions", "skills", "memory"},
  "last_extract_at": "",
  "last_nudge_at": "",
  "nudge_counter": 0,
  "top_skills_by_use": [],
  "bottom_skills_by_use": [],
  "db_size_bytes": <int>
}
```

## Exported-identifier inventory (per Rename Ledger)

All identifiers listed in the spec's Design § Rename Ledger were verified
present in the source tree at the start of TASK-2. No "extras" surfaced
that require spec amendment. The full per-package export listing is
captured below for reference (raw `grep` output preserved for executor
auditability).

### audit
- `const FileName = "audit.jsonl"`
- `type Entry struct { Timestamp, DecisionID, Action, SourcePath, DeprecatedPath, MergedInto, CandidateSignature, Rationale }`
- `func Path / Append / Read / FindByDecisionID / LearningDirFromDB / FormatTimestamp`

### cli
- `func Run / RunCmd / NewRootCmd`

### cmd
- `func RegisterAll`
- Plus 16 subcommand files, each with an `init()` calling `register(...)`

### config
- `var DefaultSecretPatterns`
- `func DefaultConfig`
- `type Config / SecretPattern`
- `func Load`

### ingest/git
- `const MaxDirsPerCommit = 8`
- `type Record / Options`
- `func ParseLog`

### ingest/memory
- `const Kind = "memory-entry"`
- `type Record / Frontmatter`
- `func ParseFile / ParseDir`

### ingest/spec
- `type Record`
- `const KindStatusTransition / KindTask / KindTDDStep`
- `func Parse` (test is `package spec_test` — black-box)

### ingest/transcript
- `type Record`
- `func ParseFile / ParseDir`

### learnerr
- `type UsageError / RuntimeError`
- `func Usagef / Runtimef / ExitCode`

### logging
- `func New / NewJSONHandler`

### pattern
- `const Sep = " → "`
- `type Counted / Kind / Candidate`
- `var AllKinds = []Kind{KindToolSequence, KindFileSequence, KindCommitPattern, KindErrorFix}`
- `func ExtractNGrams / Marshal / UnmarshalStrict / Score`

### recall
- `const KindSkill / KindMemory / KindPattern`
- `type Match / Options`
- `func Recall`

### sanitize
- `type Pattern / Sanitizer`
- `func BuiltinPatterns / New / NewFromConfig`

### similar
- `type Index / Match / QueryOpts`
- `func Query / Distance / NormalizedDistance / EscapeMatch`

### store
- `type Store / Event / SkillIndexEntry / MemoryIndexEntry / DeprecationCandidate`
- `func Open`
- 14 methods on `*Store`: `Close, DB, InsertEvent, IncrementNudgeCounter,
  GetNudgeCounter, ResetNudgeCounter, MarkNudgeRun, ListDeprecationCandidates,
  UpsertSkillIndex, UpsertMemoryIndex, DeleteOrphanSkill, DeleteOrphanMemory,
  IncrementSkillUsage, IncrementMemoryUsage`

## Worktree state at refactor start

```
/Users/marcelojr/Development/Workspace/gopherplate d5cf1c3 [main]
```

(No orphan worktrees.)
