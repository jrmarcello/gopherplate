# Fragment: TASK-24 → .claude/settings.json

## Intent
Register reindex-learning.sh as an additional PostToolUse[Edit|Write] hook
(parallel to lint-go-file.sh and validate-migration.sh).

## Target
.claude/settings.json

## Additions

### Section: hooks.PostToolUse.Edit|Write

Append to the existing `hooks.PostToolUse[0].hooks` array (matcher "Edit|Write"):

```json
{
  "type": "command",
  "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/reindex-learning.sh",
  "timeout": 5,
  "statusMessage": "Updating learning index..."
}
```

## Notes
- Hook exits 0 even on failure (best-effort). Only blocks the save event if `set -e` is mistakenly added.
- Fires AFTER goimports/migration validators (those return non-zero on real errors and block before this one runs).
- TASK-31 (MERGE-SETTINGS) uses jq's `.hooks.PostToolUse[0].hooks += [...]`.
