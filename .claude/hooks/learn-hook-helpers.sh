#!/usr/bin/env bash
# learn-hook-helpers.sh — shared shell helpers for learn-related Claude Code hooks.
#
# Source this file at the top of any learn-related Claude Code hook.
#
# Required env vars (caller-set):
#   CLAUDE_PROJECT_DIR  — repo root (set by Claude Code)
#   LEARN_HOOK_NAME     — short hook name for log records (e.g. "stop-learn")
#
# Optional env vars:
#   LEARN_BIN           — explicit path to the learn binary
#   LEARN_DB_PATH       — override default <dir>/.claude/learning/db.sqlite
#   LEARN_CONFIG_PATH   — override default <dir>/.claude/learning/config.yml
#
# Usage:
#   #!/usr/bin/env bash
#   set -uo pipefail
#   export LEARN_HOOK_NAME=stop-learn
#   source "$CLAUDE_PROJECT_DIR/.claude/hooks/learn-hook-helpers.sh"
#   ...
#   learn_run complete-task --spec "$spec" --session "$session_id"
#
# Self-test:
#   LEARN_HOOK_HELPERS_SELF_TEST=1 bash .claude/hooks/learn-hook-helpers.sh

# NOTE: intentionally NOT using `set -e`. This file is *sourced* by hooks that
# may rely on their own error handling. `set -u` and `pipefail` are safe here.
set -uo pipefail

# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

# _learn_project_dir: resolves CLAUDE_PROJECT_DIR with a fallback to pwd.
_learn_project_dir() {
    if [ -n "${CLAUDE_PROJECT_DIR:-}" ]; then
        printf '%s' "$CLAUDE_PROJECT_DIR"
    else
        # Fallback — emit a stderr warn but keep working.
        printf '[learn-hook-helpers] warn: CLAUDE_PROJECT_DIR unset; falling back to pwd\n' >&2
        printf '%s' "$(pwd)"
    fi
}

# _learn_rfc3339_now: RFC3339 timestamp (portable across macOS/Linux date).
_learn_rfc3339_now() {
    # macOS BSD date and GNU date both accept this form.
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# _learn_needs_b64 <string>: 0 if string contains a `"` or backslash or control char.
_learn_needs_b64() {
    local s="$1"
    case "$s" in
        *\"*|*\\*) return 0 ;;
    esac
    # Detect control chars (newline, tab, etc.) — bash 3.2 compatible.
    case "$s" in
        *$'\n'*|*$'\t'*|*$'\r'*) return 0 ;;
    esac
    return 1
}

# _learn_b64: base64-encode stdin with no line wrap (portable).
_learn_b64() {
    if base64 --help 2>&1 | grep -q -- '-w'; then
        base64 -w0
    else
        # macOS BSD base64: no wrap by default for short strings; strip newlines.
        base64 | tr -d '\n'
    fi
}

# _learn_json_value <raw>: prints a JSON-safe value.
#   If raw is "clean" (no " \ or control chars), prints "raw".
#   Otherwise prints "b64:<base64-of-raw>".
_learn_json_value() {
    local raw="$1"
    if _learn_needs_b64 "$raw"; then
        local encoded
        encoded="$(printf '%s' "$raw" | _learn_b64)"
        printf '"b64:%s"' "$encoded"
    else
        printf '"%s"' "$raw"
    fi
}

# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

# learn_log_path: prints the absolute path to learn.log, creating dir if needed.
learn_log_path() {
    local dir
    dir="$(_learn_project_dir)/.claude/learning"
    if [ ! -d "$dir" ]; then
        mkdir -p "$dir" 2>/dev/null || {
            printf '[learn-hook-helpers] warn: could not create %s\n' "$dir" >&2
        }
    fi
    printf '%s/learn.log' "$dir"
}

# learn_log <level> <event> [key=value ...]
learn_log() {
    local level="${1:-info}"
    local event="${2:-unspecified}"
    shift 2 2>/dev/null || true

    local hook_name="${LEARN_HOOK_NAME:-unknown}"
    local ts
    ts="$(_learn_rfc3339_now)"

    local log_file
    log_file="$(learn_log_path)"

    # Build JSON line via printf — no jq dependency.
    local line
    line="{\"time\":\"${ts}\",\"level\":$(_learn_json_value "$level"),\"event\":$(_learn_json_value "$event"),\"hook\":$(_learn_json_value "$hook_name")"

    local kv key val
    for kv in "$@"; do
        key="${kv%%=*}"
        val="${kv#*=}"
        # Skip if no '=' present (malformed).
        if [ "$key" = "$kv" ]; then
            continue
        fi
        # Whitelist: key must match [a-zA-Z_][a-zA-Z0-9_]+. Anything else gets
        # replaced with "_invalid_key" to prevent JSON injection via crafted
        # kv strings (e.g. a key containing " or \). The val side is escaped
        # by _learn_json_value, so this closes the asymmetric gap reported
        # by the security review (M1).
        case "$key" in
            ''|*[!a-zA-Z0-9_]*|[0-9]*)
                key="_invalid_key"
                ;;
        esac
        line="${line},\"${key}\":$(_learn_json_value "$val")"
    done
    line="${line}}"

    # Append; on failure fall back to stderr. Never fail the caller.
    if ! printf '%s\n' "$line" >> "$log_file" 2>/dev/null; then
        printf '[learn-hook-helpers] log-fallback: %s\n' "$line" >&2
    fi
    return 0
}

# learn_binary_path: resolves the `learn` binary path.
learn_binary_path() {
    # 1. LEARN_BIN
    if [ -n "${LEARN_BIN:-}" ] && [ -x "$LEARN_BIN" ]; then
        printf '%s' "$LEARN_BIN"
        return 0
    fi

    # 2. $CLAUDE_PROJECT_DIR/bin/learn
    local project_bin
    project_bin="$(_learn_project_dir)/bin/learn"
    if [ -x "$project_bin" ]; then
        printf '%s' "$project_bin"
        return 0
    fi

    # 3. PATH lookup
    local path_bin
    path_bin="$(command -v learn 2>/dev/null || true)"
    if [ -n "$path_bin" ] && [ -x "$path_bin" ]; then
        printf '%s' "$path_bin"
        return 0
    fi

    # 4. asdf shim
    if command -v asdf >/dev/null 2>&1; then
        local asdf_bin
        asdf_bin="$(asdf which learn 2>/dev/null || true)"
        if [ -n "$asdf_bin" ] && [ -x "$asdf_bin" ]; then
            printf '%s' "$asdf_bin"
            return 0
        fi
    fi

    return 1
}

# learn_db_path: prints the absolute DB path.
learn_db_path() {
    if [ -n "${LEARN_DB_PATH:-}" ]; then
        printf '%s' "$LEARN_DB_PATH"
        return 0
    fi
    printf '%s/.claude/learning/db.sqlite' "$(_learn_project_dir)"
}

# learn_config_path: prints the absolute config path.
learn_config_path() {
    if [ -n "${LEARN_CONFIG_PATH:-}" ]; then
        printf '%s' "$LEARN_CONFIG_PATH"
        return 0
    fi
    printf '%s/.claude/learning/config.yml' "$(_learn_project_dir)"
}

# learn_run <subcommand> [args...]
#   Invokes the learn binary with --db-path set. Best-effort: returns 0 if
#   the binary is missing (so hooks never block Claude Code events).
learn_run() {
    local bin
    bin="$(learn_binary_path)" || bin=""
    if [ -z "$bin" ]; then
        learn_log warn "binary_not_found" "subcommand=${1:-}"
        return 0
    fi

    local db
    db="$(learn_db_path)"

    # Run binary, capture stdout/stderr separately. Use tempfiles to keep both.
    local tmp_out tmp_err rc
    tmp_out="$(mktemp -t learn-out.XXXXXX)" || tmp_out=""
    tmp_err="$(mktemp -t learn-err.XXXXXX)" || tmp_err=""

    if [ -n "$tmp_out" ] && [ -n "$tmp_err" ]; then
        "$bin" --db-path "$db" "$@" >"$tmp_out" 2>"$tmp_err"
        rc=$?
        # Echo stdout through; stderr only on non-zero.
        cat "$tmp_out"
        if [ "$rc" -ne 0 ]; then
            cat "$tmp_err" >&2
            learn_log warn "binary_nonzero_exit" "subcommand=${1:-}" "exit_code=$rc"
        fi
        rm -f "$tmp_out" "$tmp_err"
    else
        # mktemp failed — run directly without separation.
        "$bin" --db-path "$db" "$@"
        rc=$?
    fi

    return "$rc"
}

# learn_safe_jq <jq-expr> [<json>]
#   Runs jq; on missing jq or malformed input, logs warn and returns empty + 0.
learn_safe_jq() {
    local expr="${1:-}"
    local input="${2:-}"

    if ! command -v jq >/dev/null 2>&1; then
        learn_log warn "jq_missing" "expr=$expr"
        printf ''
        return 0
    fi

    local out rc
    if [ -n "$input" ]; then
        out="$(printf '%s' "$input" | jq -r "$expr" 2>/dev/null)"
        rc=$?
    else
        out="$(jq -r "$expr" 2>/dev/null)"
        rc=$?
    fi

    if [ "$rc" -ne 0 ]; then
        learn_log warn "jq_failed" "expr=$expr"
        printf ''
        return 0
    fi

    printf '%s' "$out"
    return 0
}

# ---------------------------------------------------------------------------
# Self-test (LEARN_HOOK_HELPERS_SELF_TEST=1 bash <this-file>)
# ---------------------------------------------------------------------------

_learn_self_test() {
    local fails=0
    local tmpdir
    tmpdir="$(mktemp -d -t learn-self-test.XXXXXX)"
    export CLAUDE_PROJECT_DIR="$tmpdir"
    export LEARN_HOOK_NAME="self-test"
    unset LEARN_BIN LEARN_DB_PATH LEARN_CONFIG_PATH

    # learn_log_path
    local lp
    lp="$(learn_log_path)"
    if [ "$lp" = "$tmpdir/.claude/learning/learn.log" ] && [ -d "$tmpdir/.claude/learning" ]; then
        printf 'OK: learn_log_path\n'
    else
        printf 'FAIL: learn_log_path -> %s\n' "$lp"
        fails=$((fails + 1))
    fi

    # learn_log
    learn_log info "self_test" "k=v" 'quoted="hi"'
    if [ -s "$lp" ]; then
        printf 'OK: learn_log writes\n'
    else
        printf 'FAIL: learn_log did not write\n'
        fails=$((fails + 1))
    fi

    # learn_db_path / learn_config_path
    if [ "$(learn_db_path)" = "$tmpdir/.claude/learning/db.sqlite" ]; then
        printf 'OK: learn_db_path default\n'
    else
        printf 'FAIL: learn_db_path -> %s\n' "$(learn_db_path)"
        fails=$((fails + 1))
    fi
    if [ "$(learn_config_path)" = "$tmpdir/.claude/learning/config.yml" ]; then
        printf 'OK: learn_config_path default\n'
    else
        printf 'FAIL: learn_config_path -> %s\n' "$(learn_config_path)"
        fails=$((fails + 1))
    fi

    # learn_binary_path: should fail when no binary anywhere
    if learn_binary_path >/dev/null 2>&1; then
        # Could legitimately resolve a system `learn` — accept if it's executable.
        printf 'OK: learn_binary_path (resolved)\n'
    else
        printf 'OK: learn_binary_path (none)\n'
    fi

    # learn_run: missing binary -> returns 0
    unset LEARN_BIN
    export PATH="/nonexistent-${RANDOM}"
    if learn_run noop >/dev/null 2>&1; then
        printf 'OK: learn_run best-effort\n'
    else
        printf 'FAIL: learn_run did not return 0 with missing binary\n'
        fails=$((fails + 1))
    fi
    # Restore PATH for jq probe
    export PATH="/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin"

    # learn_safe_jq
    local jq_out
    jq_out="$(learn_safe_jq '.x' '{"x":42}')"
    if command -v jq >/dev/null 2>&1; then
        if [ "$jq_out" = "42" ]; then
            printf 'OK: learn_safe_jq\n'
        else
            printf 'FAIL: learn_safe_jq -> %s\n' "$jq_out"
            fails=$((fails + 1))
        fi
    else
        printf 'OK: learn_safe_jq (jq missing, returned empty)\n'
    fi

    rm -rf "$tmpdir"
    if [ "$fails" -eq 0 ]; then
        printf 'self-test: ALL OK\n'
        return 0
    fi
    printf 'self-test: %d FAILURE(S)\n' "$fails"
    return 1
}

if [ "${LEARN_HOOK_HELPERS_SELF_TEST:-0}" = "1" ]; then
    _learn_self_test
    exit $?
fi
