# CI-parity sensor (`make ci-local`)

Simulates a fresh clone + the full CI pipeline locally, in a detached `git worktree` at
`HEAD`. Catches drift that the per-edit hooks cannot see, specifically **artifacts that
compile locally because they exist on disk but are hidden from CI by `.gitignore`** — the
most common being `gen/proto/**` (buf-generated gRPC stubs) and `docs/swagger.{json,yaml}`.

## What it detects

| Symptom caught | Root cause |
| --- | --- |
| `no required module provides package .../gen/proto/...` on CI | `gen/` gitignored; CI never ran `buf generate`; local hooks passed because stubs existed on disk |
| Handler added a `@Router` annotation, swagger not regenerated | `docs/` changes locally but CI regenerates and compares |
| Generator contract drift (e.g. `buf.gen.yaml` or `buf.yaml` changed) | Local binary cached; CI runs whatever `buf` produces from current proto |
| `goose: command not found` in CI | Tool not in the CI runner image; local hook uses `$GOBIN` |

It does **not** replace the per-edit hooks. `lint-go-file.sh` (gopls on save) and
`stop-validate.sh` (Stop hook) keep their fast-feedback role — they run on every edit and
on Claude's Stop, and they trust the working copy. `ci-local` is deliberately slower and
scoped to a single moment: **pre-push**.

## How to run

```bash
make ci-local
```

Invokes [.claude/hooks/ci-local.sh](../../.claude/hooks/ci-local.sh), which:

1. Short-circuits in < 2s if the current `HEAD` sha is already in `.git/ci-parity-pass`.
2. Auto-installs any missing tool (`buf`, `protoc-gen-go`, `protoc-gen-go-grpc`, `swag`,
   `goose`, `golangci-lint`, `govulncheck`) into `$GOBIN` via `go install` — idempotent.
3. Creates a detached worktree: `git worktree add --detach /tmp/gopherplate-ci-parity.XXXX HEAD`.
4. Runs inside it: `buf generate` → `swag init` → `go build ./...` → `go vet ./...` →
   `golangci-lint run ./...` → `go test ./internal/... -short` → `govulncheck ./...`.
5. On success: writes `HEAD` sha to `.git/ci-parity-pass`.
6. On failure: prints the failing step, its captured `stderr`, and a one-line remediation
   hint.
7. Always removes the worktree via `trap cleanup EXIT INT TERM` — a `Ctrl+C` mid-run
   leaves no orphan entry in `git worktree list` and does not touch the working copy.

## How the cache works

`.git/ci-parity-pass` holds the sha of the last validated `HEAD`. Re-runs on the same
commit short-circuit instantly. The file lives inside `.git/`, which git itself ignores —
no `.gitignore` rule needed. To force a re-run: `rm .git/ci-parity-pass`.

## Hooked into git push

[lefthook.yml](../../lefthook.yml) `pre-push` calls `make ci-local`. If the sensor fails,
the push is aborted.

**Bypass in emergencies:**

```bash
git push --no-verify
```

This is the standard lefthook bypass — no project-specific override.

## Adding a new gitignored-generated artifact

If you introduce another generator (e.g., `mockgen`, `wire`), add it to the pipeline in
[.claude/hooks/ci-local.sh](../../.claude/hooks/ci-local.sh):

1. Register the binary in `ensure_tool` near the top.
2. Add a `run_step <name> <command>` in the pipeline block.
3. Add a remediation hint for the new step name in `hint_for`.
4. Extend [.claude/hooks/ci-local_test.sh](../../.claude/hooks/ci-local_test.sh) with a
   `tc_<step>` that stubs the step failing.

Keep the step name short and unique (`proto`, `swag`, `build`, …) — it surfaces in failure
messages and in the log directory (`.ci-parity/<step>.log` inside the worktree).

## Testing the sensor itself

```bash
bash .claude/hooks/ci-local_test.sh
```

Runs a hermetic harness with 10 TCs. Each test builds a throwaway git repo in
`$(mktemp -d)` and exercises `ci-local.sh` with `CI_PARITY_STEP_RUNNER` pointing at a
stub script, so no real `buf` or `go build` runs. Fast (< 5s total).

## Performance

- **First run on a new `HEAD`**: 10–30s, dominated by `go build ./...` + `golangci-lint`
  + `go test`. Go's build cache and module cache are global (not per-worktree), so runs
  after the first are hot.
- **Cached (same `HEAD`)**: < 2s.
- **After amending a commit** (new sha): full run, no cache hit — expected.

The cost belongs at pre-push. Do not call `make ci-local` from inside other hooks that
run on every edit.
