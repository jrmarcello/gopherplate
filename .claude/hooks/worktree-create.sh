#!/bin/bash
# WorktreeCreate — Create git worktree and run project setup
# Replaces default git worktree behavior. Must print worktree path to stdout.
set -euo pipefail

INPUT=$(cat)
NAME=$(echo "$INPUT" | jq -r '.name')

REPO_ROOT=$(git rev-parse --show-toplevel)
SAFE_NAME=$(echo "$NAME" | tr '/' '-')
WORKTREE_DIR="${REPO_ROOT}/.claude/worktrees/${SAFE_NAME}"
BRANCH="worktree-${NAME}"

# ── Determine base branch ────────────────────────────────────────
# Prefer develop (integration branch), fallback to remote HEAD
if git rev-parse --verify "origin/develop" &>/dev/null; then
  BASE="origin/develop"
else
  DEFAULT=$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@refs/remotes/origin/@@' || echo "main")
  BASE="origin/${DEFAULT}"
fi

echo "Creating worktree '${NAME}' from ${BASE}..." >&2

# ── Fetch latest ──────────────────────────────────────────────────
git fetch origin >&2

# ── Create git worktree ──────────────────────────────────────────
mkdir -p "$(dirname "$WORKTREE_DIR")"

if git rev-parse --verify "$BRANCH" &>/dev/null; then
  # Branch already exists (resumed worktree) — reuse it
  echo "Reusing existing branch ${BRANCH}" >&2
  git worktree add "$WORKTREE_DIR" "$BRANCH" >&2
else
  # Create new branch from base
  git worktree add "$WORKTREE_DIR" -b "$BRANCH" "$BASE" >&2
fi

cd "$WORKTREE_DIR"

# ── Project setup ────────────────────────────────────────────────

# 0. Ensure git identity (inherit from main repo → global → env vars → warn)
if [ -z "$(git config user.email 2>/dev/null)" ]; then
  MAIN_EMAIL=$(git -C "${REPO_ROOT}" config user.email 2>/dev/null || echo "${GIT_AUTHOR_EMAIL:-}")
  MAIN_NAME=$(git -C "${REPO_ROOT}" config user.name 2>/dev/null || echo "${GIT_AUTHOR_NAME:-}")
  if [ -n "$MAIN_EMAIL" ] && [ -n "$MAIN_NAME" ]; then
    git config user.email "$MAIN_EMAIL"
    git config user.name "$MAIN_NAME"
    echo "Git identity: ${MAIN_NAME} <${MAIN_EMAIL}>" >&2
  else
    echo "WARNING: Git identity not configured. Commits will fail." >&2
    echo "  Run: git config --global user.email 'you@example.com'" >&2
  fi
fi

# 1. Go dependencies
echo "Downloading Go dependencies..." >&2
go mod download >&2

# 2. Copy .env from main project (not tracked by git)
if [ -f "${REPO_ROOT}/.env" ]; then
  cp "${REPO_ROOT}/.env" "$WORKTREE_DIR/.env"
  echo "Copied .env from main project" >&2
elif [ -f "${REPO_ROOT}/.env.example" ]; then
  cp "${REPO_ROOT}/.env.example" "$WORKTREE_DIR/.env"
  echo "Copied .env.example as .env (review DB settings)" >&2
fi

# 3. Copy local settings (not tracked by git)
if [ -f "${REPO_ROOT}/.claude/settings.local.json" ]; then
  mkdir -p "$WORKTREE_DIR/.claude"
  cp "${REPO_ROOT}/.claude/settings.local.json" "$WORKTREE_DIR/.claude/settings.local.json"
  echo "Copied .claude/settings.local.json" >&2
fi

# 4. Install git hooks (lefthook)
if command -v lefthook &>/dev/null; then
  lefthook install >&2 || true
fi

# 5. Verify build compiles
echo "Verifying build..." >&2
if go build ./... >&2; then
  echo "Build OK" >&2
else
  echo "WARNING: Build failed — dependencies may need updating" >&2
fi

# 6. Check Docker infrastructure
if command -v docker &>/dev/null; then
  RUNNING=$(docker ps --format '{{.Names}}' 2>/dev/null || true)
  if ! echo "$RUNNING" | grep -q 'boilerplate-db'; then
    echo "WARNING: boilerplate-db not running. Run 'make docker-up' in the main project." >&2
  fi
  if ! echo "$RUNNING" | grep -q 'boilerplate-redis'; then
    echo "WARNING: boilerplate-redis not running. Run 'make docker-up' in the main project." >&2
  fi
fi

echo "" >&2
echo "Worktree ready: ${WORKTREE_DIR}" >&2
echo "Branch: ${BRANCH} (based on ${BASE})" >&2
echo "Docker services are shared with the main project." >&2

# Return the worktree path (stdout — Claude Code reads this)
echo "$WORKTREE_DIR"
