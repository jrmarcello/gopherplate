#!/usr/bin/env bash
# learn-hook-helpers_test.sh — vanilla bash test runner for learn-hook-helpers.sh
#
# Usage:
#   bash .claude/hooks/learn-hook-helpers_test.sh
#
# Exit code = number of FAILed assertions (0 on full pass).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HELPERS="$SCRIPT_DIR/learn-hook-helpers.sh"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    printf 'PASS: %s\n' "$1"
}

fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    printf 'FAIL: %s: %s\n' "$1" "$2"
}

# Fresh sandbox per-test: each test runs in a subshell with its own tmpdir.
make_sandbox() {
    local td
    td="$(mktemp -d -t learn-test.XXXXXX)"
    printf '%s' "$td"
}

# Has python3 or python for JSON validation?
have_python() {
    command -v python3 >/dev/null 2>&1 || command -v python >/dev/null 2>&1
}

python_cmd() {
    if command -v python3 >/dev/null 2>&1; then
        printf 'python3'
    else
        printf 'python'
    fi
}

# ---------------------------------------------------------------------------
# Test: learn_binary_path with LEARN_BIN
# ---------------------------------------------------------------------------
test_binary_path_explicit() {
    local name="learn_binary_path: LEARN_BIN explicit + executable"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        local mock="$td/mock-learn"
        printf '#!/bin/sh\necho mocked\n' > "$mock"
        chmod +x "$mock"
        export LEARN_BIN="$mock"
        # shellcheck disable=SC1090
        source "$HELPERS"
        local got
        got="$(learn_binary_path)"
        rm -rf "$td"
        if [ "$got" = "$mock" ]; then
            exit 0
        else
            printf 'got=%s expected=%s\n' "$got" "$mock" >&2
            exit 1
        fi
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "see stderr"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_binary_path with project bin/learn
# ---------------------------------------------------------------------------
test_binary_path_project_bin() {
    local name="learn_binary_path: \$CLAUDE_PROJECT_DIR/bin/learn"
    (
        local td orig_path
        td="$(make_sandbox)"
        orig_path="$PATH"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        unset LEARN_BIN
        mkdir -p "$td/bin"
        printf '#!/bin/sh\necho proj\n' > "$td/bin/learn"
        chmod +x "$td/bin/learn"
        # Block PATH so command -v can't find another 'learn'
        export PATH="/nonexistent-$RANDOM"
        # shellcheck disable=SC1090
        source "$HELPERS"
        local got
        got="$(learn_binary_path)"
        export PATH="$orig_path"
        rm -rf "$td"
        if [ "$got" = "$td/bin/learn" ]; then
            exit 0
        else
            printf 'got=%s\n' "$got" >&2
            exit 1
        fi
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "see stderr"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_binary_path fails when nothing available
# ---------------------------------------------------------------------------
test_binary_path_missing() {
    local name="learn_binary_path: returns 1 when not found"
    (
        local td orig_path
        td="$(make_sandbox)"
        orig_path="$PATH"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        unset LEARN_BIN
        export PATH="/nonexistent-$RANDOM"
        # shellcheck disable=SC1090
        source "$HELPERS"
        local rc=0
        if learn_binary_path >/dev/null 2>&1; then
            rc=1
        fi
        export PATH="$orig_path"
        rm -rf "$td"
        exit "$rc"
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "expected non-zero"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_db_path & learn_config_path defaulting
# ---------------------------------------------------------------------------
test_db_config_defaults() {
    local name="learn_db_path/learn_config_path defaults"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        unset LEARN_DB_PATH LEARN_CONFIG_PATH
        # shellcheck disable=SC1090
        source "$HELPERS"
        local db cfg
        db="$(learn_db_path)"
        cfg="$(learn_config_path)"
        rm -rf "$td"
        if [ "$db" = "$td/.claude/learning/db.sqlite" ] && \
           [ "$cfg" = "$td/.claude/learning/config.yml" ]; then
            exit 0
        else
            printf 'db=%s cfg=%s\n' "$db" "$cfg" >&2
            exit 1
        fi
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "wrong default"; fi
}

test_db_config_overrides() {
    local name="learn_db_path/learn_config_path overrides honored"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        export LEARN_DB_PATH="/tmp/custom.sqlite"
        export LEARN_CONFIG_PATH="/tmp/custom.yml"
        # shellcheck disable=SC1090
        source "$HELPERS"
        local db cfg
        db="$(learn_db_path)"
        cfg="$(learn_config_path)"
        rm -rf "$td"
        if [ "$db" = "/tmp/custom.sqlite" ] && [ "$cfg" = "/tmp/custom.yml" ]; then
            exit 0
        else
            exit 1
        fi
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "override not honored"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_log writes valid JSON per call
# ---------------------------------------------------------------------------
test_learn_log_valid_json() {
    local name="learn_log writes one valid JSON line per call"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test-hook"
        # shellcheck disable=SC1090
        source "$HELPERS"

        learn_log info "started" "spec=foo" "session=abc"
        learn_log warn "weird" "msg=has \"quotes\" inside"
        learn_log error "boom" "trace=line1
line2"

        local log_file
        log_file="$td/.claude/learning/learn.log"
        local lines
        lines="$(wc -l < "$log_file" | tr -d ' ')"

        if [ "$lines" != "3" ]; then
            printf 'expected 3 lines, got %s\n' "$lines" >&2
            rm -rf "$td"
            exit 1
        fi

        # Validate each line is parseable JSON.
        if have_python; then
            local pc rc
            pc="$(python_cmd)"
            # shellcheck disable=SC2002
            cat "$log_file" | "$pc" -c '
import sys, json
for i, line in enumerate(sys.stdin, 1):
    line = line.strip()
    if not line: continue
    try:
        obj = json.loads(line)
    except Exception as e:
        print(f"line {i} invalid: {e}", file=sys.stderr)
        sys.exit(1)
    for k in ("time","level","event","hook"):
        if k not in obj:
            print(f"line {i} missing {k}", file=sys.stderr)
            sys.exit(1)
sys.exit(0)
'
            rc=$?
            rm -rf "$td"
            exit "$rc"
        elif command -v jq >/dev/null 2>&1; then
            local rc
            jq -e . "$log_file" >/dev/null 2>&1
            rc=$?
            rm -rf "$td"
            exit "$rc"
        else
            # No parser — accept if file non-empty and lines start with {
            local first
            first="$(head -n1 "$log_file" | cut -c1)"
            rm -rf "$td"
            [ "$first" = "{" ] && exit 0 || exit 1
        fi
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "JSON validation failed"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_run falls back when binary missing
# ---------------------------------------------------------------------------
test_learn_run_missing_binary() {
    local name="learn_run: graceful fallback when binary missing"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        unset LEARN_BIN
        # Keep PATH so date/mkdir/grep work; bail if `learn` is actually on PATH
        # (very unlikely in CI/dev). If present, this test is skipped-as-pass.
        if command -v learn >/dev/null 2>&1; then
            rm -rf "$td"
            exit 0
        fi
        # Also make sure no project bin/learn shadows.
        rm -f "$td/bin/learn" 2>/dev/null
        # shellcheck disable=SC1090
        source "$HELPERS"

        local rc
        learn_run complete-task --foo bar >/dev/null 2>&1
        rc=$?
        if [ "$rc" -ne 0 ]; then
            rm -rf "$td"
            exit 1
        fi

        # Should have logged a binary_not_found warn.
        local log_file="$td/.claude/learning/learn.log"
        if grep -q "binary_not_found" "$log_file" 2>/dev/null; then
            rm -rf "$td"
            exit 0
        fi
        rm -rf "$td"
        exit 1
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "no graceful fallback"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_run invokes the binary with --db-path
# ---------------------------------------------------------------------------
test_learn_run_invokes_binary() {
    local name="learn_run: invokes binary with --db-path"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"

        local mock="$td/mock-learn"
        cat > "$mock" <<'MOCK'
#!/bin/sh
printf 'ARGS: %s\n' "$*"
exit 0
MOCK
        chmod +x "$mock"
        export LEARN_BIN="$mock"

        # shellcheck disable=SC1090
        source "$HELPERS"

        local out
        out="$(learn_run hello --x 1 2>&1)"
        rm -rf "$td"
        case "$out" in
            *"--db-path"*"hello"*"--x"*"1"*) exit 0 ;;
            *) printf 'got: %s\n' "$out" >&2; exit 1 ;;
        esac
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "wrong invocation"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_safe_jq with valid input
# ---------------------------------------------------------------------------
test_safe_jq_valid() {
    local name="learn_safe_jq: valid input"
    if ! command -v jq >/dev/null 2>&1; then
        # No jq: should return empty + 0
        (
            local td
            td="$(make_sandbox)"
            export CLAUDE_PROJECT_DIR="$td"
            export LEARN_HOOK_NAME="test"
            # shellcheck disable=SC1090
            source "$HELPERS"
            local out rc
            out="$(learn_safe_jq '.x' '{"x":1}')"
            rc=$?
            rm -rf "$td"
            if [ "$rc" -eq 0 ] && [ -z "$out" ]; then
                exit 0
            fi
            exit 1
        )
        if [ "$?" -eq 0 ]; then
            pass "$name (jq missing — empty fallback)"
        else
            fail "$name" "fallback failed"
        fi
        return
    fi

    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        # shellcheck disable=SC1090
        source "$HELPERS"
        local out
        out="$(learn_safe_jq '.x' '{"x":42}')"
        rm -rf "$td"
        if [ "$out" = "42" ]; then
            exit 0
        fi
        printf 'got=%s\n' "$out" >&2
        exit 1
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "wrong value"; fi
}

# ---------------------------------------------------------------------------
# Test: learn_safe_jq with malformed JSON
# ---------------------------------------------------------------------------
test_safe_jq_malformed() {
    local name="learn_safe_jq: malformed input -> empty + 0"
    (
        local td
        td="$(make_sandbox)"
        export CLAUDE_PROJECT_DIR="$td"
        export LEARN_HOOK_NAME="test"
        # shellcheck disable=SC1090
        source "$HELPERS"
        local out rc
        out="$(learn_safe_jq '.x' 'not-json')"
        rc=$?
        rm -rf "$td"
        if [ "$rc" -eq 0 ] && [ -z "$out" ]; then
            exit 0
        fi
        printf 'rc=%s out=%s\n' "$rc" "$out" >&2
        exit 1
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "did not fall back"; fi
}

# ---------------------------------------------------------------------------
# Test: project dir fallback to pwd when CLAUDE_PROJECT_DIR unset
# ---------------------------------------------------------------------------
test_project_dir_fallback() {
    local name="learn_*_path: falls back to pwd when CLAUDE_PROJECT_DIR unset"
    (
        local td
        td="$(make_sandbox)"
        cd "$td" || exit 1
        unset CLAUDE_PROJECT_DIR
        unset LEARN_DB_PATH LEARN_CONFIG_PATH
        export LEARN_HOOK_NAME="test"
        # shellcheck disable=SC1090
        source "$HELPERS" 2>/dev/null
        local db
        db="$(learn_db_path 2>/dev/null)"
        # `cd $td` on macOS may return /private/var/... — accept either prefix.
        local real_td
        real_td="$(cd "$td" && pwd)"
        rm -rf "$td"
        case "$db" in
            "$real_td/.claude/learning/db.sqlite") exit 0 ;;
            "$td/.claude/learning/db.sqlite") exit 0 ;;
            *) printf 'db=%s\n' "$db" >&2; exit 1 ;;
        esac
    )
    if [ "$?" -eq 0 ]; then pass "$name"; else fail "$name" "fallback path wrong"; fi
}

# ---------------------------------------------------------------------------
# Test: self-test entrypoint
# ---------------------------------------------------------------------------
test_self_test_entrypoint() {
    local name="self-test entrypoint runs and exits OK"
    if LEARN_HOOK_HELPERS_SELF_TEST=1 bash "$HELPERS" >/dev/null 2>&1; then
        pass "$name"
    else
        fail "$name" "self-test returned non-zero"
    fi
}

# ---------------------------------------------------------------------------
# Run
# ---------------------------------------------------------------------------

test_binary_path_explicit
test_binary_path_project_bin
test_binary_path_missing
test_db_config_defaults
test_db_config_overrides
test_learn_log_valid_json
test_learn_run_missing_binary
test_learn_run_invokes_binary
test_safe_jq_valid
test_safe_jq_malformed
test_project_dir_fallback
test_self_test_entrypoint

printf '\n----\nSummary: %d passed, %d failed\n' "$PASS_COUNT" "$FAIL_COUNT"
exit "$FAIL_COUNT"
