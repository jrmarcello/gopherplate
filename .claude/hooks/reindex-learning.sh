#!/usr/bin/env bash
# reindex-learning.sh — PostToolUse[Edit|Write] hook.
#
# Incrementally reindexes a single learning-source file (skill SKILL.md or
# memory/*.md) after Claude Code edits or creates it. Best-effort: never blocks
# the save event. Implements REQ-Hook-1 (no blocking), REQ-Hook-2 (filtering),
# REQ-20 (single-file incremental reindex).
#
# Input: PostToolUse event JSON on stdin (tool_name + tool_input.file_path).
# Exits 0 unconditionally.

set -uo pipefail

export LEARN_HOOK_NAME=reindex-learning

# shellcheck disable=SC1091
source "${CLAUDE_PROJECT_DIR:-$(pwd)}/.claude/hooks/learn-hook-helpers.sh"

# Read event JSON from stdin (may be empty).
input="$(cat 2>/dev/null || true)"
if [ -z "$input" ]; then
    exit 0
fi

tool_name="$(printf '%s' "$input" | learn_safe_jq '.tool_name // ""')"
file_path="$(printf '%s' "$input" | learn_safe_jq '.tool_input.file_path // ""')"

# Filter on tool_name.
case "$tool_name" in
    Edit|Write) ;;
    *) exit 0 ;;
esac

if [ -z "$file_path" ]; then
    exit 0
fi

# Resolve relative paths against CLAUDE_PROJECT_DIR.
case "$file_path" in
    /*) ;;
    *)
        project_dir="${CLAUDE_PROJECT_DIR:-$(pwd)}"
        file_path="${project_dir%/}/${file_path}"
        ;;
esac

# Skip deprecated namespace entirely (saves a process — learn excludes these too).
case "$file_path" in
    */_deprecated/*) exit 0 ;;
esac

# Filter on path pattern. Only:
#   - .claude/skills/<name>/SKILL.md
#   - memory/<name>.md  (excluding MEMORY.md index itself)
matched=0
case "$file_path" in
    */.claude/skills/*/SKILL.md) matched=1 ;;
    */memory/MEMORY.md)          matched=0 ;;
    */memory/*.md)               matched=1 ;;
esac

if [ "$matched" -ne 1 ]; then
    exit 0
fi

# Best-effort incremental reindex. learn_run already logs nonzero exits.
if ! learn_run reindex --path "$file_path" >/dev/null 2>&1; then
    learn_log warn reindex_failed "path=$file_path"
fi

exit 0
