#!/bin/bash
# ci-local.sh — CI-parity sensor
#
# Simulates a fresh clone by running the full CI pipeline inside a detached
# `git worktree` at HEAD. The working copy, index, and stash are never touched
# (REQ-3). Called by `make ci-local` and by the lefthook pre-push command.
#
# Pipeline: proto -> swag -> build -> vet -> lint -> test -> vulncheck
#
# Cache: on pass, writes HEAD's sha to .git/ci-parity-pass. Re-runs on the same
# sha short-circuit in <2s (REQ-4). The cache is inside `.git/`, which git itself
# ignores — no .gitignore rule required.
#
# Test hook: if CI_PARITY_STEP_RUNNER is set, each pipeline step is invoked via
# "$CI_PARITY_STEP_RUNNER <step-name> <cmd...>" instead of executing the command
# directly. The test harness uses this to stub heavy commands.
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
  echo "ci-parity: not inside a git repository" >&2
  exit 1
fi
cd "$REPO_ROOT"

HEAD_SHA="$(git rev-parse HEAD 2>/dev/null || true)"
if [ -z "$HEAD_SHA" ]; then
  echo "ci-parity: could not resolve HEAD (empty repo?)" >&2
  exit 1
fi

CACHE_FILE=".git/ci-parity-pass"

# ── Cache short-circuit (REQ-4) ────────────────────────────────────
if [ -f "$CACHE_FILE" ] && [ "$(cat "$CACHE_FILE" 2>/dev/null)" = "$HEAD_SHA" ]; then
  echo "ci-parity: cached pass for $HEAD_SHA (remove $CACHE_FILE to force a re-run)"
  exit 0
fi

# ── Resolve Go env from the active toolchain (asdf-aware) ──────────
if command -v asdf >/dev/null 2>&1; then
  GO_BIN="$(asdf which go 2>/dev/null || true)"
  if [ -n "${GO_BIN:-}" ]; then
    GOROOT_RESOLVED="$(dirname "$(dirname "$GO_BIN")")"
    export GOROOT="$GOROOT_RESOLVED"
    export GOPATH="$(dirname "$GOROOT_RESOLVED")/packages"
    export GOBIN="$(dirname "$GOROOT_RESOLVED")/bin"
    export PATH="$GOBIN:$PATH"
  fi
fi

# ── Tool auto-install (REQ-7) ──────────────────────────────────────
ensure_tool() {
  local bin="$1" pkg="$2"
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "ci-parity: installing $bin ($pkg)..." >&2
    if ! go install "$pkg" >&2; then
      echo "ci-parity: failed to install $bin from $pkg" >&2
      exit 1
    fi
  fi
}

if [ -z "${CI_PARITY_SKIP_TOOL_INSTALL:-}" ]; then
  ensure_tool buf                github.com/bufbuild/buf/cmd/buf@latest
  ensure_tool protoc-gen-go      google.golang.org/protobuf/cmd/protoc-gen-go@latest
  ensure_tool protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ensure_tool swag               github.com/swaggo/swag/cmd/swag@latest
  ensure_tool goose              github.com/pressly/goose/v3/cmd/goose@latest
  ensure_tool golangci-lint      github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
  ensure_tool govulncheck        golang.org/x/vuln/cmd/govulncheck@latest
fi

# ── Worktree simulation (REQ-1, REQ-2, REQ-3) ──────────────────────
WORKTREE="$(mktemp -d -t gopherplate-ci-parity.XXXXXX)"
FAILED_STEP=""
FAILED_LOG=""

cleanup() {
  local rc=$?
  if [ -n "${WORKTREE:-}" ] && [ -d "$WORKTREE" ]; then
    git worktree remove --force "$WORKTREE" >/dev/null 2>&1 || rm -rf "$WORKTREE"
    git worktree prune >/dev/null 2>&1 || true
  fi
  exit "$rc"
}
trap cleanup EXIT INT TERM

if ! git worktree add --detach "$WORKTREE" HEAD >/dev/null 2>&1; then
  echo "ci-parity: failed to create worktree at $WORKTREE" >&2
  exit 1
fi

LOGDIR="$WORKTREE/.ci-parity"
mkdir -p "$LOGDIR"

run_step() {
  local name="$1"; shift
  local log="$LOGDIR/$name.log"
  echo "ci-parity: [$name] $*"
  local rc=0
  if [ -n "${CI_PARITY_STEP_RUNNER:-}" ]; then
    ( cd "$WORKTREE" && "$CI_PARITY_STEP_RUNNER" "$name" "$@" ) >"$log" 2>&1 || rc=$?
  else
    ( cd "$WORKTREE" && "$@" ) >"$log" 2>&1 || rc=$?
  fi
  if [ "$rc" -ne 0 ]; then
    FAILED_STEP="$name"
    FAILED_LOG="$log"
    return 1
  fi
  return 0
}

# ── Remediation hints (REQ-6) ──────────────────────────────────────
hint_for() {
  case "$1" in
    proto)     echo "run 'make proto' and commit any generated changes in buf.gen.yaml / proto/" ;;
    swag)      echo "run 'swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal' and commit docs/" ;;
    build)     echo "fix the compile errors above; if they mention gen/proto or docs/, run 'make proto' or swag first" ;;
    vet)       echo "fix the go vet issues above" ;;
    lint)      echo "run 'golangci-lint run ./...' locally and address each report" ;;
    test)      echo "fix failing tests; reproduce locally with 'go test ./internal/... -count=1 -short'" ;;
    vulncheck) echo "inspect the vulnerability report; upgrade or pin affected modules" ;;
    *)         echo "see the captured log above for details" ;;
  esac
}

# ── Pipeline ───────────────────────────────────────────────────────
run_step proto     buf generate \
  && run_step swag      swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal \
  && run_step build     go build ./... \
  && run_step vet       go vet ./... \
  && run_step lint      golangci-lint run ./... \
  && run_step test      go test ./internal/... -short -count=1 -timeout 60s \
  && run_step vulncheck govulncheck ./...

if [ -n "$FAILED_STEP" ]; then
  {
    echo ""
    echo "ci-parity: FAILED at step '$FAILED_STEP'"
    echo "ci-parity: --- captured stderr ---"
    cat "$FAILED_LOG"
    echo "ci-parity: --- remediation ---"
    echo "ci-parity: $(hint_for "$FAILED_STEP")"
  } >&2
  exit 1
fi

# ── Success — cache HEAD sha (REQ-4) ───────────────────────────────
echo "$HEAD_SHA" > "$CACHE_FILE"
echo "ci-parity: PASS for $HEAD_SHA"
exit 0
