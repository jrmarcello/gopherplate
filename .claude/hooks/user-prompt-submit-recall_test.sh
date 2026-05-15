#!/usr/bin/env bash
# user-prompt-submit-recall_test.sh — vanilla bash tests for the recall hook.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOK="$SCRIPT_DIR/user-prompt-submit-recall.sh"
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
t_assert_stdout_contains() {
    if grep -qF -- "$1" "$T_OUT" 2>/dev/null; then pass "$LAST_NAME: stdout contains '$1'"
    else fail "$LAST_NAME" "stdout missing '$1'; got: $(cat "$T_OUT")"; fi
}
t_assert_no_stdout() {
    if [ ! -s "$T_OUT" ]; then pass "$LAST_NAME: no stdout"
    else fail "$LAST_NAME" "unexpected stdout: $(cat "$T_OUT")"; fi
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
    td="$(mktemp -d -t recall-test.XXXXXX)"
    mkdir -p "$td/.claude/hooks" "$td/.claude/learning"
    ln -s "$SCRIPT_DIR/learn-hook-helpers.sh" "$td/.claude/hooks/learn-hook-helpers.sh"
    "$LEARN_BIN" init --dir "$td/.claude/learning" >/dev/null 2>&1
    # Lower recall_min_score so seeded skill matches.
    cat > "$td/.claude/learning/config.yml" <<'EOF'
nudge_threshold: 5
min_pattern_freq: 3
similarity_threshold: 0.6
ttl_days: 90
recall_min_score: 0.0
min_prompt_len_for_recall: 20
recall_top_k: 3
recall_max_tokens: 500
recall_timeout_seconds: 2
llm_model: ""
secret_patterns: []
EOF
    printf '%s' "$td"
}

seed_skill() {
    local sb="$1" path="$2" body="$3"
    sqlite3 "$sb/.claude/learning/db.sqlite" \
        "INSERT INTO skill_index(path,title,body,tags,frontmatter_json,indexed_at) VALUES('$path','Test','$body','','{}','2025-01-01T00:00:00Z')"
}

run_hook() {
    local sb="$1" input_file="$2"
    T_OUT="$(mktemp)"
    T_ERR="$(mktemp)"
    (
        cd "$sb" || exit 99
        CLAUDE_PROJECT_DIR="$sb" \
            LEARN_BIN="${OVERRIDE_LEARN_BIN:-$LEARN_BIN}" \
            bash "$HOOK" <"$input_file" >"$T_OUT" 2>"$T_ERR"
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
# TC-SH-05 — Happy: matching skill emits REQ-14a template.
# ---------------------------------------------------------------------------
test_happy_match_emits_reminder() {
    LAST_NAME="TC-SH-05 happy_match_emits_reminder"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    seed_skill "$sb" ".claude/skills/test/SKILL.md" "refactor user use case caching and tests"

    run_hook "$sb" "$FIXTURE_DIR/user-prompt-input.json"
    t_assert_exit0
    t_assert_stdout_contains "<system-reminder>"
    t_assert_stdout_contains "Learning loop recall —"
    t_assert_stdout_contains ".claude/skills/test/SKILL.md"
    t_assert_stdout_contains "</system-reminder>"
}

# ---------------------------------------------------------------------------
# TC-SH-05a — track-use increments usage_count for the cited path.
# ---------------------------------------------------------------------------
test_track_use_increments_usage() {
    LAST_NAME="TC-SH-05a track_use_increments_usage"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    seed_skill "$sb" ".claude/skills/test/SKILL.md" "refactor user use case caching and tests"

    run_hook "$sb" "$FIXTURE_DIR/user-prompt-input.json"
    t_assert_exit0

    local count
    count="$(sqlite3 "$sb/.claude/learning/db.sqlite" \
        "SELECT usage_count FROM skill_usage WHERE path='.claude/skills/test/SKILL.md'" 2>/dev/null)"
    if [ "${count:-0}" -ge 1 ]; then
        pass "$LAST_NAME: usage_count >= 1 ($count)"
    else
        fail "$LAST_NAME" "expected usage_count >= 1; got '$count'"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-06 — Boundary: prompt below MIN_PROMPT_LEN_FOR_RECALL => no stdout.
# ---------------------------------------------------------------------------
test_short_prompt_silent() {
    LAST_NAME="TC-SH-06 short_prompt_silent"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    seed_skill "$sb" ".claude/skills/test/SKILL.md" "refactor user use case caching and tests"

    local fixture; fixture="$(mktemp)"
    printf '{"prompt":"short"}\n' > "$fixture"
    run_hook "$sb" "$fixture"
    rm -f "$fixture"
    t_assert_exit0
    t_assert_no_stdout
}

# ---------------------------------------------------------------------------
# TC-SH-06a — Boundary inclusive: prompt exactly 20 chars runs recall.
# ---------------------------------------------------------------------------
test_boundary_inclusive_prompt() {
    LAST_NAME="TC-SH-06a boundary_inclusive_prompt"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    # Use a body containing words from the 20-char prompt.
    seed_skill "$sb" ".claude/skills/test/SKILL.md" "refactor user use case caching tests"

    # Exactly 20 chars including spaces: "refactor user caching" is 21; pick 20.
    local prompt="refactor user tests!"  # 20 chars exactly
    local fixture; fixture="$(mktemp)"
    printf '{"prompt":"%s"}\n' "$prompt" > "$fixture"
    run_hook "$sb" "$fixture"
    rm -f "$fixture"
    t_assert_exit0
    t_assert_stdout_contains "<system-reminder>"
}

# ---------------------------------------------------------------------------
# TC-SH-07 — Empty index => no stdout, exit 0.
# ---------------------------------------------------------------------------
test_empty_index_silent() {
    LAST_NAME="TC-SH-07 empty_index_silent"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    run_hook "$sb" "$FIXTURE_DIR/user-prompt-input.json"
    t_assert_exit0
    t_assert_no_stdout
}

# ---------------------------------------------------------------------------
# TC-SH-07a — Mock binary that sleeps => hook kills via timeout, silent.
# ---------------------------------------------------------------------------
test_timeout_silent() {
    LAST_NAME="TC-SH-07a timeout_silent"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    local mock="$sb/mock-learn"
    cat > "$mock" <<'EOF'
#!/usr/bin/env bash
sleep 5
EOF
    chmod +x "$mock"
    OVERRIDE_LEARN_BIN="$mock"
    local start_ts; start_ts=$(date +%s)
    run_hook "$sb" "$FIXTURE_DIR/user-prompt-input.json"
    local elapsed=$(($(date +%s) - start_ts))
    unset OVERRIDE_LEARN_BIN
    t_assert_exit0
    t_assert_no_stdout
    if [ "$elapsed" -le 3 ]; then
        pass "$LAST_NAME: completed in $elapsed s (<= 3, timeout enforced)"
    else
        fail "$LAST_NAME" "took $elapsed s — timeout not enforced"
    fi
}

# ---------------------------------------------------------------------------
# TC-SH-07b — track-use fails but recall stdout still printed, hook exit 0.
# ---------------------------------------------------------------------------
test_track_use_failure_nonblocking() {
    LAST_NAME="TC-SH-07b track_use_failure_nonblocking"
    local sb; sb="$(make_sandbox)"; cleanup_artifacts+=("$sb")
    seed_skill "$sb" ".claude/skills/test/SKILL.md" "refactor user use case caching and tests"

    # Mock learn: succeed for `recall`, fail for `track-use`.
    local mock="$sb/mock-learn"
    cat > "$mock" <<EOF
#!/usr/bin/env bash
# Forward args; find the subcommand (first non-flag).
sub=""
for a in "\$@"; do
    case "\$a" in
        --*) continue ;;
        -*)  continue ;;
        recall|track-use|init|stats|complete-task|reindex)
            sub="\$a"; break ;;
    esac
done
# Args that take a value (skip the value, not the subcommand search above).
case "\$sub" in
    track-use) exit 1 ;;
    recall)    exec $LEARN_BIN "\$@" ;;
    *)         exec $LEARN_BIN "\$@" ;;
esac
EOF
    chmod +x "$mock"
    OVERRIDE_LEARN_BIN="$mock"
    run_hook "$sb" "$FIXTURE_DIR/user-prompt-input.json"
    unset OVERRIDE_LEARN_BIN
    t_assert_exit0
    t_assert_stdout_contains "<system-reminder>"
}

# ---------------------------------------------------------------------------
# TC-SH-11 / TC-SH-12 — STATIC SOURCE CHECKS (not runtime behavioral tests):
# grep-based guards that catch accidental removal of `set -uo pipefail` and
# explicit `--db-path` during refactors. Behavioral assertions live in the
# other TC-SH tests in this file.
# ---------------------------------------------------------------------------
test_source_has_set_uo_pipefail() {
    LAST_NAME="TC-SH-11 source_has_set_uo_pipefail"
    if grep -qF 'set -uo pipefail' "$HOOK"; then pass "$LAST_NAME"
    else fail "$LAST_NAME" "missing 'set -uo pipefail'"; fi
}
test_source_uses_db_path() {
    LAST_NAME="TC-SH-12 source_uses_db_path"
    if grep -qF -- '--db-path' "$HOOK"; then pass "$LAST_NAME"
    else fail "$LAST_NAME" "missing --db-path"; fi
}

test_happy_match_emits_reminder
test_track_use_increments_usage
test_short_prompt_silent
test_boundary_inclusive_prompt
test_empty_index_silent
test_timeout_silent
test_track_use_failure_nonblocking
test_source_has_set_uo_pipefail
test_source_uses_db_path

printf '\n--- user-prompt-submit-recall_test.sh: %d PASS, %d FAIL ---\n' "$PASS_COUNT" "$FAIL_COUNT"
exit "$FAIL_COUNT"
