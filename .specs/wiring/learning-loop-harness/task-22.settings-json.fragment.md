# Fragment: TASK-22 → .claude/settings.json

## Intent

Register `stop-learn.sh` as an additional Stop hook (parallel to
`stop-validate.sh`), so the learning-loop harness records a completion event
each time Claude Code emits a Stop event and a spec has moved to status DONE.

## Target

.claude/settings.json

## Additions

### Section: hooks.Stop

Append this hook entry to the existing `hooks.Stop[0].hooks` array (which
currently contains only `stop-validate.sh`):

```json
{
  "type": "command",
  "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/stop-learn.sh",
  "timeout": 30,
  "statusMessage": "Recording learning-loop event..."
}
```

## Notes

- Order matters loosely: `stop-validate.sh` runs first (blocks on Go build /
  fmt / vet / test failures); `stop-learn.sh` runs after. Both must exit 0 for
  the Stop event to proceed cleanly; `stop-learn.sh` always exits 0 by design
  (REQ-Hook-1 best-effort: never block the Stop event).
- TASK-31 (MERGE-SETTINGS) uses `jq` to perform the structural append
  idempotently — re-running TASK-31 must not duplicate the entry.
- The hook depends on `.claude/hooks/learn-hook-helpers.sh` and the
  `bin/learn` binary; if either is missing the hook logs a warning to
  `.claude/learning/learn.log` and still exits 0.
