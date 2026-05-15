#!/usr/bin/env bash
# stop-learn.sh — Claude Code Stop hook for the learning-loop harness.
#
# Fires on the Stop event. When a spec on disk has been moved to status DONE
# (detected via git diff against .specs/*.md), records the completion via
# `learn complete-task` and surfaces a nudge advisory on stderr if the binary
# signals LEARN_NUDGE_DUE=true.
#
# Contract (REQ-1, REQ-13, REQ-Hook-1, REQ-Hook-2):
#   - Best-effort: ALWAYS exits 0, even when the binary fails, git is absent,
#     or no spec changed. Never blocks the Stop event.
#   - Never invokes a skill — only prints an advisory line.
#   - Reads Stop event JSON from stdin; only `.session_id` is used.
#
# Output:
#   On nudge-due, prints to stderr exactly:
#     Learning nudge due — run /learn-nudge when ready (<N> specs since last nudge)
#
set -uo pipefail

export LEARN_HOOK_NAME=stop-learn

# Resolve project dir for sourcing the helpers; fall back to pwd if unset.
_project_dir="${CLAUDE_PROJECT_DIR:-$(pwd)}"
# shellcheck disable=SC1091
if [ -r "$_project_dir/.claude/hooks/learn-hook-helpers.sh" ]; then
    . "$_project_dir/.claude/hooks/learn-hook-helpers.sh"
else
    # No helpers available — log to stderr and bail out (best-effort).
    printf '[stop-learn] warn: learn-hook-helpers.sh not found under %s\n' \
        "$_project_dir/.claude/hooks" >&2
    exit 0
fi

# ── tempfile cleanup ────────────────────────────────────────────────
_tmp_stderr=""
_cleanup() {
    if [ -n "${_tmp_stderr:-}" ] && [ -e "$_tmp_stderr" ]; then
        rm -f "$_tmp_stderr"
    fi
}
trap _cleanup EXIT

# ── Read Stop event JSON ────────────────────────────────────────────
_input="$(cat || true)"
_session_id="$(learn_safe_jq '.session_id // "unknown"' "$_input")"
if [ -z "$_session_id" ]; then
    _session_id="unknown"
fi

# ── Detect candidate spec files (changed .specs/*.md) ────────────────
if ! command -v git >/dev/null 2>&1; then
    learn_log warn "git_missing"
    exit 0
fi

# Confirm we're inside a git work tree; if not, no-op.
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    learn_log warn "not_a_git_repo"
    exit 0
fi

_changed=""
_changed="$(git diff --name-only HEAD 2>/dev/null || true)"
_changed="${_changed}"$'\n'"$(git diff --cached --name-only 2>/dev/null || true)"
_changed="${_changed}"$'\n'"$(git ls-files --others --exclude-standard 2>/dev/null || true)"

# Filter to .specs/*.md (top-level under .specs/), deduplicate.
_spec_candidates="$(printf '%s\n' "$_changed" \
    | grep -E '^\.specs/[^/]+\.md$' \
    | sort -u \
    || true)"

if [ -z "$_spec_candidates" ]; then
    exit 0
fi

# ── Iterate, detect DONE, call complete-task ────────────────────────
_nudge_due=0
_nudge_counter=""
_commit_sha="$(git rev-parse HEAD 2>/dev/null || echo '')"

_tmp_stderr="$(mktemp -t learn-stop-stderr.XXXXXX)" || _tmp_stderr=""

# Resolve binary + db once. `learn_run` from the helpers captures stderr
# internally and only echoes it on non-zero exit, which would swallow the
# LEARN_NUDGE_DUE signal — call the binary directly here so we can inspect
# stderr regardless of exit code.
_learn_bin="$(learn_binary_path 2>/dev/null || true)"
_learn_db="$(learn_db_path)"

if [ -z "$_learn_bin" ]; then
    learn_log warn "binary_not_found" "subcommand=complete-task"
    exit 0
fi

while IFS= read -r _spec; do
    [ -z "$_spec" ] && continue
    [ ! -f "$_spec" ] && continue

    if ! grep -q '^## Status: DONE' "$_spec" 2>/dev/null; then
        continue
    fi

    # Reset stderr capture per-iteration.
    if [ -n "$_tmp_stderr" ]; then
        : > "$_tmp_stderr"

        "$_learn_bin" --db-path "$_learn_db" complete-task \
            --spec "$_spec" \
            --session "$_session_id" \
            --commit-sha "$_commit_sha" \
            >/dev/null 2>"$_tmp_stderr"
        _rc=$?

        if [ "$_rc" -ne 0 ]; then
            learn_log warn "binary_failed" "cmd=complete-task" "spec=$_spec" "exit_code=$_rc"
        fi

        # Forward learn's stderr to our own stderr so it appears in transcripts.
        if [ -s "$_tmp_stderr" ]; then
            cat "$_tmp_stderr" >&2
            if grep -q '^LEARN_NUDGE_DUE=true' "$_tmp_stderr" 2>/dev/null; then
                _nudge_due=1
                _nudge_counter="$(grep -o 'counter=[0-9][0-9]*' "$_tmp_stderr" \
                    | head -1 \
                    | sed 's/^counter=//' \
                    || true)"
            fi
        fi
    else
        # mktemp failed — call without capturing; we won't detect nudge.
        if ! "$_learn_bin" --db-path "$_learn_db" complete-task \
                --spec "$_spec" \
                --session "$_session_id" \
                --commit-sha "$_commit_sha" >/dev/null 2>&1; then
            learn_log warn "binary_failed" "cmd=complete-task" "spec=$_spec"
        fi
    fi
done <<EOF
$_spec_candidates
EOF

# ── Surface nudge advisory ──────────────────────────────────────────
if [ "$_nudge_due" -eq 1 ]; then
    _n="${_nudge_counter:-?}"
    printf 'Learning nudge due — run /learn-nudge when ready (%s specs since last nudge)\n' \
        "$_n" >&2
    learn_log info "nudge_signaled" "counter=$_n"
fi

exit 0
