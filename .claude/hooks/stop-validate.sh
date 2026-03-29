#!/bin/bash
# Stop — Post-implementation validation gate
# Blocks Claude from finishing when Go code changes fail basic quality checks.
# Tiers:
#   1st attempt  → build + fmt + vet + swagger + lint + tests
#   2nd attempt  → build + fmt + vet only (faster retry)
#   3rd+ attempt → pass (avoid infinite loop)
set -uo pipefail

INPUT=$(cat)
STOP_HOOK_ACTIVE=$(echo "$INPUT" | jq -r '.stop_hook_active // false')
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // "unknown"')

# ── Loop breaker ────────────────────────────────────────────────────
COUNTER_FILE="/tmp/claude-validate-${SESSION_ID}"
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || echo "0")
COUNT=$((COUNT + 1))
echo "$COUNT" > "$COUNTER_FILE"

if [ "$COUNT" -ge 3 ]; then
  rm -f "$COUNTER_FILE"
  exit 0
fi

# ── Detect Go changes ──────────────────────────────────────────────
CHANGED_FILES=""
CHANGED_FILES+=$(git diff --name-only 2>/dev/null || true)
CHANGED_FILES+=$'\n'
CHANGED_FILES+=$(git diff --cached --name-only 2>/dev/null || true)
CHANGED_FILES+=$'\n'
CHANGED_FILES+=$(git ls-files --others --exclude-standard 2>/dev/null || true)

GO_CHANGES=$(echo "$CHANGED_FILES" | grep '\.go$' | sort -u || true)

# No Go changes → pass
if [ -z "$GO_CHANGES" ]; then
  rm -f "$COUNTER_FILE"
  exit 0
fi

ERRORS=""

# ── 1. Build (always) ──────────────────────────────────────────────
BUILD_OUT=$(go build ./... 2>&1) || ERRORS="BUILD FAILED:\n${BUILD_OUT}\n\n"

# ── 2. Formatting + imports (goimports > gofmt) ──────────────────
if command -v goimports &>/dev/null; then
  FMT_FILES=$(goimports -l . 2>/dev/null | head -20)
  FMT_CMD="goimports -w ."
else
  FMT_FILES=$(gofmt -l . 2>/dev/null | head -20)
  FMT_CMD="gofmt -w ."
fi
if [ -n "$FMT_FILES" ]; then
  ERRORS="${ERRORS}FILES NOT FORMATTED (run ${FMT_CMD}):\n${FMT_FILES}\n\n"
fi

# ── 3. Go vet (always) ─────────────────────────────────────────────
VET_OUT=$(go vet ./... 2>&1) || ERRORS="${ERRORS}GO VET ISSUES:\n${VET_OUT}\n\n"

# ── 4. Swagger freshness (if handler/router with annotations changed) ─
SWAGGER_FILES=$(echo "$GO_CHANGES" | grep -E 'handler/|router/' | while read -r f; do
  [ -f "$f" ] && grep -lE '@(Summary|Router|Param|Success|Failure|Tags)' "$f" 2>/dev/null
done || true)
if [ -n "$SWAGGER_FILES" ]; then
  DOCS_CHANGED=$(echo "$CHANGED_FILES" | grep '^docs/' || true)
  if [ -z "$DOCS_CHANGED" ]; then
    ERRORS="${ERRORS}SWAGGER STALE: handler/router files changed but docs/ not regenerated.\nRun: swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal\n\n"
  fi
fi

# ── 5. golangci-lint (first attempt only, skip on retry) ───────────
if [ "$STOP_HOOK_ACTIVE" != "true" ] && [ -z "$ERRORS" ]; then
  if command -v golangci-lint &>/dev/null; then
    LINT_OUT=$(golangci-lint run ./... 2>&1) || ERRORS="${ERRORS}LINT ERRORS:\n${LINT_OUT}\n\n"
  fi
fi

# ── 6. Unit tests (first attempt only, skip on retry) ──────────────
if [ "$STOP_HOOK_ACTIVE" != "true" ] && [ -z "$ERRORS" ]; then
  TEST_OUT=$(go test ./internal/... -count=1 -short -timeout 60s 2>&1) || \
    ERRORS="${ERRORS}TEST FAILURES:\n${TEST_OUT}\n\n"
fi

# ── Result ──────────────────────────────────────────────────────────
if [ -n "$ERRORS" ]; then
  printf "Post-implementation validation FAILED:\n\n%b" "$ERRORS" >&2
  exit 2
fi

# All passed
rm -f "$COUNTER_FILE"
exit 0
