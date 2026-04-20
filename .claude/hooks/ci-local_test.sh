#!/bin/bash
# ci-local_test.sh — test harness for ci-local.sh
#
# Strategy: each TC spins up a throwaway git repo in $(mktemp -d), then runs
# ci-local.sh with CI_PARITY_STEP_RUNNER pointing at a stub that returns
# pre-configured exit codes per pipeline step. This keeps tests fast and
# hermetic — no real `buf generate` / `go build` inside the harness.
#
# TC-H-09 (lefthook --no-verify bypass) and TC-H-11 (fresh-clone auto-install)
# cover lefthook-level and network behavior; they are exercised manually in
# TASK-8 runtime validation, not here.
#
# Run: bash .claude/hooks/ci-local_test.sh
#   exit 0 = all TCs passed, exit 1 = at least one TC failed.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CI_LOCAL="$SCRIPT_DIR/ci-local.sh"

if [ ! -x "$CI_LOCAL" ]; then
  echo "FAIL: $CI_LOCAL not executable" >&2
  exit 1
fi

PASS=0
FAIL=0
FAILURES=()

pass() { PASS=$((PASS+1)); printf "  \033[32mPASS\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL+1)); FAILURES+=("$1: $2"); printf "  \033[31mFAIL\033[0m %s — %s\n" "$1" "$2"; }

# ── Fixture builder ────────────────────────────────────────────────
# Creates a minimal git repo at $1 with a tracked HEAD commit and one
# optional gitignored "generated" directory to mimic gen/proto.
setup_fixture() {
  local dir="$1"
  (
    cd "$dir" || exit 1
    git init -q -b main .
    git config user.email t@t
    git config user.name t
    echo '*.generated' > .gitignore
    echo 'package main; func main(){}' > main.go
    git add .
    git commit -qm init
  )
}

# Build a stub step-runner that reads $STUB_<step> env vars for exit codes.
# Default: every step exits 0.
write_stub_runner() {
  local path="$1"
  cat > "$path" <<'STUB'
#!/bin/bash
# $1 = step name, $2... = original command (ignored)
step="$1"
var="STUB_${step}"
rc="${!var:-0}"
if [ -n "${STUB_STDERR:-}" ]; then echo "$STUB_STDERR" >&2; fi
exit "$rc"
STUB
  chmod +x "$path"
}

# Run ci-local.sh in an isolated repo with tool-install skipped and a stub runner.
run_ci_local() {
  local repo="$1"
  (
    cd "$repo"
    CI_PARITY_SKIP_TOOL_INSTALL=1 \
    CI_PARITY_STEP_RUNNER="$repo/.stub-runner" \
    "$CI_LOCAL" "$@"
  )
}

# ── TC-H-02: happy path — all steps stubbed to pass ────────────────
tc_h_02() {
  local name="TC-H-02"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  local out; out=$(run_ci_local "$repo" 2>&1)
  local rc=$?
  if [ "$rc" -eq 0 ] && [ -f "$repo/.git/ci-parity-pass" ]; then
    pass "$name"
  else
    fail "$name" "expected exit 0 + cache file; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-05: cache short-circuit on same HEAD ──────────────────────
tc_h_05() {
  local name="TC-H-05"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  run_ci_local "$repo" >/dev/null 2>&1
  # Second run: flip a stub to fail — if cache honored, should still pass.
  local out; out=$(cd "$repo" && STUB_build=1 CI_PARITY_SKIP_TOOL_INSTALL=1 \
    CI_PARITY_STEP_RUNNER="$repo/.stub-runner" "$CI_LOCAL" 2>&1)
  local rc=$?
  if [ "$rc" -eq 0 ] && echo "$out" | grep -q "cached pass"; then
    pass "$name"
  else
    fail "$name" "expected cached pass; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-06: cache invalidates on HEAD change ──────────────────────
tc_h_06() {
  local name="TC-H-06"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  run_ci_local "$repo" >/dev/null 2>&1
  # New commit ⇒ new HEAD sha ⇒ cache must not short-circuit.
  (cd "$repo" && echo '// v2' >> main.go && git add . && git commit -qm v2)
  local out; out=$(cd "$repo" && STUB_build=1 CI_PARITY_SKIP_TOOL_INSTALL=1 \
    CI_PARITY_STEP_RUNNER="$repo/.stub-runner" "$CI_LOCAL" 2>&1)
  local rc=$?
  if [ "$rc" -ne 0 ] && echo "$out" | grep -q "FAILED at step 'build'"; then
    pass "$name"
  else
    fail "$name" "expected fresh run to fail at build; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-01: missing gen/proto surfaces as build-step failure ──────
# Model: stub fails 'proto' to mimic "proto wasn't regenerated".
tc_h_01() {
  local name="TC-H-01"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  local out; out=$(cd "$repo" && STUB_proto=1 CI_PARITY_SKIP_TOOL_INSTALL=1 \
    CI_PARITY_STEP_RUNNER="$repo/.stub-runner" "$CI_LOCAL" 2>&1)
  local rc=$?
  if [ "$rc" -ne 0 ] \
     && echo "$out" | grep -q "FAILED at step 'proto'" \
     && echo "$out" | grep -qi "make proto"; then
    pass "$name"
  else
    fail "$name" "expected proto-named failure with remediation; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-07: build-step stderr is surfaced ─────────────────────────
tc_h_07() {
  local name="TC-H-07"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  local out; out=$(cd "$repo" && STUB_build=1 STUB_STDERR="syntax error at 42" \
    CI_PARITY_SKIP_TOOL_INSTALL=1 CI_PARITY_STEP_RUNNER="$repo/.stub-runner" \
    "$CI_LOCAL" 2>&1)
  local rc=$?
  if [ "$rc" -ne 0 ] \
     && echo "$out" | grep -q "FAILED at step 'build'" \
     && echo "$out" | grep -q "syntax error at 42"; then
    pass "$name"
  else
    fail "$name" "expected build step + captured stderr; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-08: proto breaking change surfaces via proto/build hint ───
tc_h_08() {
  local name="TC-H-08"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  local out; out=$(cd "$repo" && STUB_proto=1 STUB_STDERR="proto: missing symbol XYZ" \
    CI_PARITY_SKIP_TOOL_INSTALL=1 CI_PARITY_STEP_RUNNER="$repo/.stub-runner" \
    "$CI_LOCAL" 2>&1)
  local rc=$?
  if [ "$rc" -ne 0 ] \
     && echo "$out" | grep -q "FAILED at step 'proto'" \
     && echo "$out" | grep -q "missing symbol XYZ"; then
    pass "$name"
  else
    fail "$name" "expected proto step to fail with cited symbol; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-10: swag step failure ─────────────────────────────────────
tc_h_10() {
  local name="TC-H-10"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  local out; out=$(cd "$repo" && STUB_swag=1 CI_PARITY_SKIP_TOOL_INSTALL=1 \
    CI_PARITY_STEP_RUNNER="$repo/.stub-runner" "$CI_LOCAL" 2>&1)
  local rc=$?
  if [ "$rc" -ne 0 ] && echo "$out" | grep -q "FAILED at step 'swag'"; then
    pass "$name"
  else
    fail "$name" "expected swag-named failure; got rc=$rc, out=$out"
  fi
  rm -rf "$repo"
}

# ── TC-H-12: no orphan worktrees after success ─────────────────────
tc_h_12() {
  local name="TC-H-12"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  local before; before=$(cd "$repo" && git worktree list | wc -l)
  run_ci_local "$repo" >/dev/null 2>&1
  local after; after=$(cd "$repo" && git worktree list | wc -l)
  if [ "$before" -eq "$after" ]; then
    pass "$name"
  else
    fail "$name" "worktree count changed ($before -> $after)"
  fi
  rm -rf "$repo"
}

# ── TC-H-03: SIGINT mid-run leaves no orphan worktree ──────────────
# Stub sleeps long enough to be interrupted.
tc_h_03() {
  local name="TC-H-03"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  cat > "$repo/.stub-runner" <<'STUB'
#!/bin/bash
if [ "$1" = "proto" ]; then sleep 5; fi
exit 0
STUB
  chmod +x "$repo/.stub-runner"
  local before; before=$(cd "$repo" && git worktree list | wc -l)
  ( cd "$repo" && CI_PARITY_SKIP_TOOL_INSTALL=1 \
    CI_PARITY_STEP_RUNNER="$repo/.stub-runner" "$CI_LOCAL" >/dev/null 2>&1 ) &
  local pid=$!
  sleep 0.5
  kill -INT "$pid" 2>/dev/null
  wait "$pid" 2>/dev/null
  local after; after=$(cd "$repo" && git worktree list | wc -l)
  if [ "$before" -eq "$after" ]; then
    pass "$name"
  else
    fail "$name" "worktree left after SIGINT ($before -> $after)"
  fi
  rm -rf "$repo"
}

# ── TC-H-04: staging area untouched ────────────────────────────────
tc_h_04() {
  local name="TC-H-04"
  local repo; repo=$(mktemp -d)
  setup_fixture "$repo"
  write_stub_runner "$repo/.stub-runner"
  (cd "$repo" && echo 'staged' > staged.txt && git add staged.txt)
  local before; before=$(cd "$repo" && git diff --cached --name-only | sort)
  run_ci_local "$repo" >/dev/null 2>&1
  local after; after=$(cd "$repo" && git diff --cached --name-only | sort)
  if [ "$before" = "$after" ]; then
    pass "$name"
  else
    fail "$name" "staged file list changed (before=[$before] after=[$after])"
  fi
  rm -rf "$repo"
}

# ── Run all ─────────────────────────────────────────────────────────
echo "ci-local.sh test harness"
echo "========================"
tc_h_01
tc_h_02
tc_h_03
tc_h_04
tc_h_05
tc_h_06
tc_h_07
tc_h_08
tc_h_10
tc_h_12

echo ""
echo "Summary: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  printf "\nFailures:\n"
  printf "  - %s\n" "${FAILURES[@]}"
  exit 1
fi
exit 0
