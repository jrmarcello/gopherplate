# Fragment: TASK-33A → Makefile

## Intent

Add operational `learn-*` targets so the learning loop is invokable from the
project root without `cd tools/learn`. Complements `learn-build` (introduced
by TASK-1's fragment).

## Target

Makefile

## Additions

### Section: learn targets

```makefile
.PHONY: learn-setup learn-reindex learn-smoke learn-stats learn-lint learn-test

learn-setup: learn-build ## Initialize the learning-loop store and config under .claude/learning/
	$(CURDIR)/bin/learn init --dir $(CURDIR)/.claude/learning

learn-reindex: learn-build ## Full reindex of skills + memory FTS5 index
	$(CURDIR)/bin/learn reindex \
		--skills-dir $(CURDIR)/.claude/skills \
		--memory-dir $(CURDIR)/memory \
		--db-path $(CURDIR)/.claude/learning/db.sqlite

learn-stats: learn-build ## Print learning-loop KB stats as JSON
	$(CURDIR)/bin/learn stats --db-path $(CURDIR)/.claude/learning/db.sqlite

learn-smoke: learn-build ## Run learning-loop end-to-end smoke (fixtures)
	$(MAKE) -C tools/learn smoke

learn-lint: ## golangci-lint over the tools/learn module
	$(MAKE) -C tools/learn lint

learn-test: ## go test over the tools/learn module
	$(MAKE) -C tools/learn test
```

## Notes

- All `learn-*` runtime targets depend on `learn-build` so they auto-recompile
  when the binary is missing (cheap because Go's build cache handles re-runs).
- `learn-lint` and `learn-test` do NOT depend on `learn-build` — they exercise
  the module-local Makefile inside `tools/learn/`, which has its own targets.
- `learn-smoke` delegates to `tools/learn/Makefile`'s `smoke` target, which is
  introduced by TASK-41 (E2E + smoke subcommand). Until TASK-41 ships, this
  target fails with "No rule to make target 'smoke'" — acceptable, the merge
  order in the spec is fragment → TASK-35 merge → TASK-41 adds the rule.
- `$(CURDIR)` propagation keeps paths absolute when `make -C` is used.
- Recipe lines MUST use TAB indentation (Makefile-mandatory) — the fenced
  block above is authored with literal TABs.

## Anchor

`learn targets` — registered in the spec's "Registered anchors" section.
TASK-35 (MERGE-MAKEFILE) reads both this fragment and
`task-1.makefile.fragment.md` and inserts the combined `learn-*` block into
the root `Makefile` before the existing `help:` target.
