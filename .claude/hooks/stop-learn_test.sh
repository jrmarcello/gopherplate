#!/usr/bin/env bash
# stop-learn_test.sh — vanilla bash test runner for stop-learn.sh
#
# Usage: bash .claude/hooks/stop-learn_test.sh
# Exit code = number of FAILed assertions (0 = all PASS).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOK="$SCRIPT_DIR/stop-learn.sh"
FIXTURE_DIR="$REPO_ROOT/tools/learn/testdata/hooks"

PASS_COUNT=0
FAIL_COUNT=0
LAST_NAME=""

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    printf 'PASS: %s\n' "$1"
}
fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    printf 'FAIL: %s: %s\n' "$1" "$2"
}

# Assertion helpers operate on saved state: $T_OUT, $T_ERR, $T_RC.
t_assert_exit0() {
    if [ "$T_RC" -eq 0 ]; then
        pass "$LAST_NAME: exit 0"
    else
        fail "$LAST_NAME" "expected exit 0, got $T_RC; stderr: $(cat "$T_ERR" 2>/dev/null)"
    fi
}
t_assert_stderr_contains() {
    if grep -qF -- "$1" "$T_ERR" 2>/dev/null; then
        pass "$LAST_NAME: stderr contains '$1'"
    else
        fail "$LAST_NAME" "stderr missing '$1'; got: $(cat "$T_ERR" 2>/dev/null)"
    fi
}
t_assert_stdout_contains() {
    if grep -qF -- "$1" "$T_OUT" 2>/dev/null; then
        pass "$LAST_NAME: stdout contains '$1'"
    else
        fail "$LAST_NAME" "stdout missing '$1'; got: $(cat "$T_OUT" 2>/dev/null)"
    fi
}
t_assert_no_output() {
    if [ ! -s "$T_OUT" ] && [ ! -s "$T_ERR" ]; then
        pass "$LAST_NAME: no output"
    else
        fail "$LAST_NAME" "unexpected output: stdout='$(cat "$T_OUT")', stderr='$(cat "$T_ERR")'"
    fi
}
t_assert_file_exists() {
    if [ -e "$1" ]; then
        pass "$LAST_NAME: file exists $1"
    else
        fail "$LAST_NAME" "file missing: $1"
    fi
}
t_assert_stderr_matches() {
    if grep -qE -- "$1" "$T_ERR" 2>/dev/null; then
        pass "$LAST_NAME: stderr matches /$1/"
    else
        fail "$LAST_NAME" "stderr does not match /$1/; got: $(cat "$T_ERR" 2>/dev/null)"
    fi
}

# ---------------------------------------------------------------------------
# Shared learn binary build (sentinel: /tmp/learn-test-bin/learn).
# ---------------------------------------------------------------------------
LEARN_BIN_DIR="/tmp/learn-test-bin"
LEARN_BIN="$LEARN_BIN_DIR/learn"
if [ ! -x "$LEARN_BIN" ]; then
    mkdir -p "$LEARN_BIN_DIR"
    (cd "$REPO_ROOT/tools/learn" && go build -o "$LEARN_BIN" ./cmd/learn) || {
        printf 'FATAL: could not build learn binary\n' >&2
        exit 1
    }
fi
export LEARN_BIN

# ---------------------------------------------------------------------------
# Per-test sandbox helper.
# ---------------------------------------------------------------------------
make_sandbox() {
    local td
    td="$(mktemp -d -t stop-learn-test.XXXXXX)"
    # Init as a git repo so the hook's git checks succeed.
    git -C "$td" init -q
    git -C "$td" -c user.email=x@x.test -c user.name=x commit --allow-empty -q -m init
    mkdir -p "$td/.specs" "$td/.claude/hooks"
    # The hook sources learn-hook-helpers.sh — symlink it in.
    ln -s "$SCRIPT_DIR/learn-hook-helpers.sh" "$td/.claude/hooks/learn-hook-helpers.sh"
    printf '%s' "$td"
}

# Init learning store in $1 (sandbox dir).
init_learn_store() {
    local sb="$1"
    "$LEARN_BIN" init --dir "$sb/.claude/learning" >/dev/null 2>&1
}

# Run the hook in sandbox $1 with stdin from $2 (file).
run_hook() {
    local sb="$1"
    local input_file="$2"
    T_OUT="$(mktemp)"
    T_ERR="$(mktemp)"
    (
        cd "$sb" || exit 99
        CLAUDE_PROJECT_DIR="$sb" \
            LEARN_BIN="${OVERRIDE_LEARN_BIN:-$LEARN_BIN}" \
            LEARN_CONFIG_PATH="${LEARN_CONFIG_PATH:-}" \
            bash "$HOOK" <"$input_file" >"$T_OUT" 2>"$T_ERR"
    )
    T_RC=$?
}

cleanup_artifacts=()
cleanup() {
    local p
    for p in "${cleanup_artifacts[@]}"; do
        rm -rf "$p"
    done
    [ -n "${T_OUT:-}" ] && rm -f "$T_OUT"
    [ -n "${T_ERR:-}" ] && rm -f "$T_ERR"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# TC-SH-01 — Happy: spec with `## Status: DONE` triggers complete-task.
# ---------------------------------------------------------------------------
test_happy_done_spec_recorded() {
    LAST_NAME="TC-SH-01 happy_done_spec_recorded"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    init_learn_store "$sb"

    printf '# Test\n\n## Status: DONE\n' > "$sb/.specs/test-stop-happy.md"
    (cd "$sb" && git add .specs/test-stop-happy.md)

    run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    t_assert_exit0

    # Verify an event row was written.
    local stats
    stats="$("$LEARN_BIN" --db-path "$sb/.claude/learning/db.sqlite" stats 2>/dev/null)"
    if printf '%s' "$stats" | grep -q '"events": *[1-9]'; then
        pass "$LAST_NAME: events count > 0"
    else
        fail "$LAST_NAME" "expected events >= 1; got stats: $stats"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-02 — Edge: no .specs/*.md changes => no-op, no events.
# ---------------------------------------------------------------------------
test_no_spec_changes_noop() {
    LAST_NAME="TC-SH-02 no_spec_changes_noop"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    init_learn_store "$sb"

    # No .specs/*.md files exist or are tracked.
    run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    t_assert_exit0

    local stats
    stats="$("$LEARN_BIN" --db-path "$sb/.claude/learning/db.sqlite" stats 2>/dev/null)"
    if printf '%s' "$stats" | grep -qE '"events": *0'; then
        pass "$LAST_NAME: events count == 0"
    else
        fail "$LAST_NAME" "expected events == 0; got stats: $stats"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-03 — Infra: missing binary => exit 0, warn in learn.log.
# ---------------------------------------------------------------------------
test_missing_binary_logs_warn() {
    LAST_NAME="TC-SH-03 missing_binary_logs_warn"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    init_learn_store "$sb"
    printf '# Test\n\n## Status: DONE\n' > "$sb/.specs/test-missing.md"
    (cd "$sb" && git add .specs/test-missing.md)

    OVERRIDE_LEARN_BIN="/nonexistent/learn"
    run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    unset OVERRIDE_LEARN_BIN
    t_assert_exit0
    if grep -q '"event":"binary_not_found"' "$sb/.claude/learning/learn.log" 2>/dev/null; then
        pass "$LAST_NAME: warn logged"
    else
        fail "$LAST_NAME" "binary_not_found warn missing from log: $(cat "$sb/.claude/learning/learn.log" 2>/dev/null)"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-03a — Mocked learn binary exiting 2 => hook still exits 0.
# ---------------------------------------------------------------------------
test_mock_binary_nonzero_exit() {
    LAST_NAME="TC-SH-03a mock_binary_nonzero_exit"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    init_learn_store "$sb"
    printf '# Test\n\n## Status: DONE\n' > "$sb/.specs/test-nonzero.md"
    (cd "$sb" && git add .specs/test-nonzero.md)

    local mock="$sb/mock-learn"
    cat > "$mock" <<'EOF'
#!/usr/bin/env bash
exit 2
EOF
    chmod +x "$mock"

    OVERRIDE_LEARN_BIN="$mock"
    run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    unset OVERRIDE_LEARN_BIN
    t_assert_exit0
    if grep -q '"event":"binary_failed"' "$sb/.claude/learning/learn.log" 2>/dev/null; then
        pass "$LAST_NAME: binary_failed warn logged"
    else
        fail "$LAST_NAME" "binary_failed warn missing from log"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-03b — Idempotent repeat: 2nd call increments counter properly.
# ---------------------------------------------------------------------------
test_idempotent_repeat() {
    LAST_NAME="TC-SH-03b idempotent_repeat"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    init_learn_store "$sb"

    printf '# Test\n\n## Status: DONE\n' > "$sb/.specs/test-idem.md"
    (cd "$sb" && git add .specs/test-idem.md)

    run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    t_assert_exit0
    run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    t_assert_exit0

    local stats
    stats="$("$LEARN_BIN" --db-path "$sb/.claude/learning/db.sqlite" stats 2>/dev/null)"
    # Events count should be 2 (two recordings of the same DONE spec).
    if printf '%s' "$stats" | grep -qE '"events": *2'; then
        pass "$LAST_NAME: events count == 2 after two calls"
    else
        fail "$LAST_NAME" "expected events == 2; got stats: $stats"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-04 — Nudge advisory exact-string surface (threshold=1).
# ---------------------------------------------------------------------------
test_nudge_advisory_exact_string() {
    LAST_NAME="TC-SH-04 nudge_advisory_exact_string"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    init_learn_store "$sb"
    # Override nudge_threshold to 1 so a single completion triggers nudge.
    cat > "$sb/.claude/learning/config.yml" <<'EOF'
nudge_threshold: 1
min_pattern_freq: 3
similarity_threshold: 0.6
ttl_days: 90
recall_min_score: 0.4
min_prompt_len_for_recall: 20
recall_top_k: 3
recall_max_tokens: 500
recall_timeout_seconds: 2
llm_model: ""
secret_patterns: []
EOF
    printf '# Test\n\n## Status: DONE\n' > "$sb/.specs/test-nudge.md"
    (cd "$sb" && git add .specs/test-nudge.md)

    # complete-task reads config from <dir>/config.yml by default.
    LEARN_CONFIG_PATH="$sb/.claude/learning/config.yml" \
        run_hook "$sb" "$FIXTURE_DIR/stop-input.json"
    t_assert_exit0
    # Exact-string check on stderr.
    if grep -qF 'Learning nudge due — run /learn-nudge when ready (1 specs since last nudge)' "$T_ERR"; then
        pass "$LAST_NAME: exact nudge advisory string emitted"
    else
        fail "$LAST_NAME" "expected nudge advisory not found; stderr: $(cat "$T_ERR")"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-11 — STATIC SOURCE CHECK (not a runtime behavioral test): the hook
# script source contains `set -uo pipefail`. A truly behavioral assertion
# would require running the hook with an unbound var in a controlled
# subshell; this grep is a documentation-style guard catching accidental
# removal of the pragma during refactors.
# ---------------------------------------------------------------------------
test_source_has_set_uo_pipefail() {
    LAST_NAME="TC-SH-11 source_has_set_uo_pipefail"
    if grep -qF 'set -uo pipefail' "$HOOK"; then
        pass "$LAST_NAME"
    else
        fail "$LAST_NAME" "stop-learn.sh missing 'set -uo pipefail'"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-12 — STATIC SOURCE CHECK (not a runtime behavioral test): hook
# invokes `learn` with `--db-path` explicitly (REQ-Hook-2 — works even
# when cwd is not project root).
# ---------------------------------------------------------------------------
test_source_uses_db_path() {
    LAST_NAME="TC-SH-12 source_uses_db_path"
    if grep -qF -- '--db-path' "$HOOK"; then
        pass "$LAST_NAME"
    else
        fail "$LAST_NAME" "stop-learn.sh missing --db-path"
    fi
}

# ---------------------------------------------------------------------------
# Runner
# ---------------------------------------------------------------------------
test_happy_done_spec_recorded
test_no_spec_changes_noop
test_missing_binary_logs_warn
test_mock_binary_nonzero_exit
test_idempotent_repeat
test_nudge_advisory_exact_string
test_source_has_set_uo_pipefail
test_source_uses_db_path

printf '\n--- stop-learn_test.sh: %d PASS, %d FAIL ---\n' "$PASS_COUNT" "$FAIL_COUNT"
exit "$FAIL_COUNT"
