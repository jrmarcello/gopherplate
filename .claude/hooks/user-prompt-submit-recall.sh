#!/usr/bin/env bash
# user-prompt-submit-recall.sh — Claude Code UserPromptSubmit hook.
#
# Fires on each user prompt submission. Invokes `learn recall` to retrieve
# top-k relevant lessons/playbook entries from prior runs and injects them
# into the conversation as a system-reminder (REQ-14a template). Tracks
# usage of retrieved paths for citation accounting (REQ-15/16/17).
#
# Contract (REQ-Hook-1, REQ-Hook-2, REQ-14):
#   - Never blocks the user prompt (always exit 0, even on failure).
#   - Best-effort retrieval with a 2s timeout around `learn recall`.
#   - --db-path passed explicitly via learn_db_path.
#   - Trusts `learn recall --format=system-reminder` output verbatim.
#
# Stdin: Claude Code UserPromptSubmit event JSON ({"prompt": "...", ...}).
# Stdout: system-reminder text (injected into context) or empty.
# Exit:   always 0.

set -uo pipefail

export LEARN_HOOK_NAME="user-prompt-submit-recall"

# Resolve project dir (Claude Code sets CLAUDE_PROJECT_DIR; fallback to pwd).
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"

# Source helpers. If missing, log to stderr and exit 0 (never block).
HELPERS="$PROJECT_DIR/.claude/hooks/learn-hook-helpers.sh"
if [ ! -f "$HELPERS" ]; then
    printf '[user-prompt-submit-recall] warn: helpers not found at %s\n' "$HELPERS" >&2
    exit 0
fi
# shellcheck disable=SC1090
source "$HELPERS"

# ---------------------------------------------------------------------------
# 1. Read event JSON from stdin and extract prompt.
# ---------------------------------------------------------------------------

event_json="$(cat 2>/dev/null || true)"

if [ -z "$event_json" ]; then
    learn_log debug "empty_stdin"
    exit 0
fi

prompt="$(printf '%s' "$event_json" | learn_safe_jq '.prompt // ""')"

if [ -z "$prompt" ]; then
    learn_log debug "empty_prompt"
    exit 0
fi

# ---------------------------------------------------------------------------
# 2. Resolve binary + timeout wrapper.
# ---------------------------------------------------------------------------

bin="$(learn_binary_path 2>/dev/null || true)"
if [ -z "$bin" ]; then
    learn_log warn "binary_not_found"
    exit 0
fi

db="$(learn_db_path)"

# Pick the available timeout command (gtimeout on macOS via coreutils, timeout on Linux).
timeout_cmd=""
if command -v gtimeout >/dev/null 2>&1; then
    timeout_cmd="gtimeout"
elif command -v timeout >/dev/null 2>&1; then
    timeout_cmd="timeout"
else
    learn_log warn "no_timeout_available"
fi

# ---------------------------------------------------------------------------
# 3. Invoke `learn recall` (best-effort, 2s timeout).
#    learn enforces MIN_PROMPT_LEN_FOR_RECALL internally — pass prompt as-is.
# ---------------------------------------------------------------------------

recall_out=""
recall_rc=0

if [ -n "$timeout_cmd" ]; then
    recall_out="$("$timeout_cmd" 2 "$bin" --db-path "$db" recall \
        --format=system-reminder \
        --prompt "$prompt" 2>/dev/null)" || recall_rc=$?
else
    recall_out="$("$bin" --db-path "$db" recall \
        --format=system-reminder \
        --prompt "$prompt" 2>/dev/null)" || recall_rc=$?
fi

if [ "$recall_rc" -ne 0 ]; then
    # 124 = timeout-killed; anything else = learn errored (e.g. no DB yet).
    learn_log warn "recall_nonzero" "exit_code=$recall_rc"
fi

# ---------------------------------------------------------------------------
# 4. Emit system-reminder verbatim if non-empty.
# ---------------------------------------------------------------------------

if [ -z "$recall_out" ]; then
    learn_log debug "no_recall_matches"
    exit 0
fi

# Print verbatim to stdout — Claude Code injects this as context.
printf '%s\n' "$recall_out"

# ---------------------------------------------------------------------------
# 5. Track usage of cited paths (best-effort).
#    Extract paths from lines matching: `^- <path> (`
# ---------------------------------------------------------------------------

paths_csv="$(printf '%s\n' "$recall_out" \
    | awk '/^- [^ ]+ \(/ { sub(/^- /, ""); sub(/ \(.*$/, ""); print }' \
    | paste -sd, - 2>/dev/null || true)"

if [ -n "$paths_csv" ]; then
    # Fire-and-forget; never propagate failure.
    learn_run track-use --paths "$paths_csv" >/dev/null 2>&1 || true
    learn_log info "recall_injected" "paths=$paths_csv"
else
    learn_log info "recall_injected" "paths=none"
fi

exit 0
