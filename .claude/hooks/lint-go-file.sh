#!/bin/bash
# PostToolUse[Edit|Write] — Go diagnostics via gopls + goimports
# Uses gopls-lsp toolchain: goimports for formatting/imports, gopls check for diagnostics
set -uo pipefail

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only check Go files
[[ "$FILE_PATH" != *.go ]] && exit 0
[[ ! -f "$FILE_PATH" ]] && exit 0

ISSUES=""

# 1. Formatting + imports (goimports subsumes gofmt + organizes imports)
if command -v goimports &>/dev/null; then
  DIFF=$(goimports -d "$FILE_PATH" 2>/dev/null)
  if [ -n "$DIFF" ]; then
    ISSUES="goimports: $FILE_PATH needs formatting/import fixes. Apply with: goimports -w \"$FILE_PATH\"\n$DIFF\n"
  fi
else
  DIFF=$(gofmt -d "$FILE_PATH" 2>/dev/null)
  if [ -n "$DIFF" ]; then
    ISSUES="gofmt: $FILE_PATH is not formatted. Apply with: gofmt -w \"$FILE_PATH\"\n$DIFF\n"
  fi
fi

# 2. gopls diagnostics (type errors, unused imports, missing deps — richer than go vet)
# Note: gopls check requires a FILE path, not a directory (confirmed with gopls v0.21.1).
if command -v gopls &>/dev/null; then
  DIAG=$(timeout 10 gopls check "$FILE_PATH" 2>/dev/null || true)
  if [ -n "$DIAG" ]; then
    # Enrich diagnostics with "fix by:" hints for common patterns.
    # Fallback-safe: if awk or the hints script is missing, raw gopls output is used.
    HINTS_SCRIPT="$(dirname "$0")/gopls-hints.awk"
    if command -v awk &>/dev/null && [ -f "$HINTS_SCRIPT" ]; then
      DIAG=$(echo "$DIAG" | awk -f "$HINTS_SCRIPT")
    fi
    ISSUES="${ISSUES}\ngopls diagnostics:\n$DIAG\n"
  fi
fi

if [ -n "$ISSUES" ]; then
  printf "%b" "$ISSUES" >&2
  exit 2
fi

exit 0
