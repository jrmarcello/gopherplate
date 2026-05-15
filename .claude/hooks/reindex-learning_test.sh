#!/usr/bin/env bash
# reindex-learning_test.sh — vanilla bash tests for the reindex hook.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOK="$SCRIPT_DIR/reindex-learning.sh"
FIXTURE_DIR="$REPO_ROOT/tools/learn/testdata/hooks"

PASS_COUNT=0
FAIL_COUNT=0
LAST_NAME=""
pass() { PASS_COUNT=$((PASS_COUNT + 1)); printf 'PASS: %s\n' "$1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); printf 'FAIL: %s: %s\n' "$1" "$2"; }

t_assert_exit0() {
    if [ "$T_RC" -eq 0 ]; then pass "$LAST_NAME: exit 0"
    else fail "$LAST_NAME" "expected exit 0, got $T_RC; stderr: $(cat "$T_ERR" 2>/dev/null)"; fi
}

# Shared learn binary.
LEARN_BIN_DIR="/tmp/learn-test-bin"
LEARN_BIN="$LEARN_BIN_DIR/learn"
if [ ! -x "$LEARN_BIN" ]; then
    mkdir -p "$LEARN_BIN_DIR"
    (cd "$REPO_ROOT/tools/learn" && go build -o "$LEARN_BIN" ./cmd/learn) || exit 1
fi
export LEARN_BIN

make_sandbox() {
    local td
    td="$(mktemp -d -t reindex-test.XXXXXX)"
    mkdir -p "$td/.claude/hooks" "$td/.claude/learning"
    ln -s "$SCRIPT_DIR/learn-hook-helpers.sh" "$td/.claude/hooks/learn-hook-helpers.sh"
    "$LEARN_BIN" init --dir "$td/.claude/learning" >/dev/null 2>&1
    printf '%s' "$td"
}

# Run hook in sandbox $1 with stdin JSON in $2.
run_hook() {
    local sb="$1" input_json="$2"
    T_OUT="$(mktemp)"
    T_ERR="$(mktemp)"
    (
        cd "$sb" || exit 99
        CLAUDE_PROJECT_DIR="$sb" \
            LEARN_BIN="${OVERRIDE_LEARN_BIN:-$LEARN_BIN}" \
            bash "$HOOK" <<<"$input_json" >"$T_OUT" 2>"$T_ERR"
    )
    T_RC=$?
}

cleanup_artifacts=()
cleanup() {
    local p
    for p in "${cleanup_artifacts[@]}"; do rm -rf "$p"; done
    [ -n "${T_OUT:-}" ] && rm -f "$T_OUT"
    [ -n "${T_ERR:-}" ] && rm -f "$T_ERR"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# TC-SH-08 — Happy: SKILL.md edit triggers reindex --path.
# ---------------------------------------------------------------------------
test_skill_edit_triggers_reindex() {
    LAST_NAME="TC-SH-08 skill_edit_triggers_reindex"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")

    # Create a real SKILL.md so reindex can read it.
    mkdir -p "$sb/.claude/skills/foo"
    cat > "$sb/.claude/skills/foo/SKILL.md" <<'EOF'
---
name: foo
description: test skill
---
# Foo
Body content for foo skill.
EOF

    local payload
    payload='{"tool_name":"Edit","tool_input":{"file_path":".claude/skills/foo/SKILL.md"}}'
    run_hook "$sb" "$payload"
    t_assert_exit0

    # Verify reindex actually wrote to skill_index.
    local count
    count="$(sqlite3 "$sb/.claude/learning/db.sqlite" \
        "SELECT COUNT(*) FROM skill_index WHERE path LIKE '%foo/SKILL.md'" 2>/dev/null)"
    if [ "${count:-0}" -ge 1 ]; then
        pass "$LAST_NAME: skill_index row created ($count)"
    else
        fail "$LAST_NAME" "expected skill_index row; got count='$count'; log: $(cat "$sb/.claude/learning/learn.log" 2>/dev/null)"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-09 — Edge: non-matching path => no-op, no learn invocation.
# ---------------------------------------------------------------------------
test_nonmatching_path_noop() {
    LAST_NAME="TC-SH-09 nonmatching_path_noop"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")

    # Use a mock learn binary that writes a sentinel file if called.
    local mock="$sb/mock-learn"
    local sentinel="$sb/learn-was-called"
    cat > "$mock" <<EOF
#!/usr/bin/env bash
touch "$sentinel"
exit 0
EOF
    chmod +x "$mock"
    OVERRIDE_LEARN_BIN="$mock"

    local payload='{"tool_name":"Edit","tool_input":{"file_path":"cmd/api/main.go"}}'
    run_hook "$sb" "$payload"
    unset OVERRIDE_LEARN_BIN
    t_assert_exit0
    if [ ! -e "$sentinel" ]; then
        pass "$LAST_NAME: learn binary not invoked"
    else
        fail "$LAST_NAME" "learn binary should not have been invoked"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-10 — Infra: mock binary exits 2 => hook exits 0, warn in learn.log.
# ---------------------------------------------------------------------------
test_mock_binary_nonzero_exit() {
    LAST_NAME="TC-SH-10 mock_binary_nonzero_exit"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    mkdir -p "$sb/.claude/skills/foo"
    cat > "$sb/.claude/skills/foo/SKILL.md" <<'EOF'
---
name: foo
description: x
---
body
EOF

    local mock="$sb/mock-learn"
    cat > "$mock" <<'EOF'
#!/usr/bin/env bash
exit 2
EOF
    chmod +x "$mock"
    OVERRIDE_LEARN_BIN="$mock"

    local payload='{"tool_name":"Edit","tool_input":{"file_path":".claude/skills/foo/SKILL.md"}}'
    run_hook "$sb" "$payload"
    unset OVERRIDE_LEARN_BIN
    t_assert_exit0
    if grep -qE '"event":"(reindex_failed|binary_nonzero_exit)"' "$sb/.claude/learning/learn.log" 2>/dev/null; then
        pass "$LAST_NAME: warn logged"
    else
        fail "$LAST_NAME" "expected warn log; got: $(cat "$sb/.claude/learning/learn.log" 2>/dev/null)"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-11 / TC-SH-12 — STATIC SOURCE CHECKS (not runtime behavioral tests):
# grep-based guards that catch accidental removal of `set -uo pipefail`;
# `--db-path` is verified indirectly via the learn_run helper which always
# passes it. Behavioral assertions live in the other TC-SH tests above.
# ---------------------------------------------------------------------------
test_source_has_set_uo_pipefail() {
    LAST_NAME="TC-SH-11 source_has_set_uo_pipefail"
    if grep -qF 'set -uo pipefail' "$HOOK"; then pass "$LAST_NAME"
    else fail "$LAST_NAME" "missing 'set -uo pipefail'"; fi
}
test_uses_learn_run_or_db_path() {
    LAST_NAME="TC-SH-12 uses_learn_run_or_db_path"
    # reindex-learning.sh delegates to learn_run, which itself passes --db-path.
    if grep -qE 'learn_run|--db-path' "$HOOK"; then pass "$LAST_NAME"
    else fail "$LAST_NAME" "neither learn_run nor --db-path found"; fi
}

test_skill_edit_triggers_reindex
test_nonmatching_path_noop
test_mock_binary_nonzero_exit
test_source_has_set_uo_pipefail
test_uses_learn_run_or_db_path

printf '\n--- reindex-learning_test.sh: %d PASS, %d FAIL ---\n' "$PASS_COUNT" "$FAIL_COUNT"
exit "$FAIL_COUNT"
