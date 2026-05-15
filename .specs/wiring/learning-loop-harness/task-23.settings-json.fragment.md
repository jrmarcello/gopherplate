# Fragment: TASK-23 → .claude/settings.json

## Intent
Register user-prompt-submit-recall.sh on the UserPromptSubmit event.

## Target
.claude/settings.json

## Additions

### Section: hooks.UserPromptSubmit

Create the array if it doesn't exist; otherwise append. Entry:

```json
{
  "type": "command",
  "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/user-prompt-submit-recall.sh",
  "timeout": 5,
  "statusMessage": "Retrieving learning-loop context..."
}
```

## Notes
- Hook MUST exit 0 even when learn-binary fails or recall times out (best-effort retrieval).
- TASK-31 (MERGE-SETTINGS) uses jq to add the UserPromptSubmit array if missing.
- Top-level timeout 5s in settings.json gives 2s for `learn recall` + 3s overhead margin.
