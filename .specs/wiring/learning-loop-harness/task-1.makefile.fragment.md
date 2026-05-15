# Fragment: TASK-1 → Makefile

## Intent

Add `learn-build` target to the root Makefile so the learning loop binary can
be compiled from the project root without `cd tools/learn`.

## Target

Makefile

## Imports

(none — Makefile additions only)

## Additions

### Section: learn targets

```makefile
## Learning loop

.PHONY: learn-build

learn-build: ## Compile tools/learn into bin/learn
	$(MAKE) -C tools/learn build LEARN_BIN=$(CURDIR)/bin/learn
```

## Notes

- Sibling targets (`learn-setup`, `learn-reindex`, `learn-smoke`, `learn-stats`,
  `learn-lint`, `learn-test`) are introduced by TASK-33A's fragment and
  consolidated by TASK-35 (MERGE-MAKEFILE).
- The anchor `learn targets` is registered for this spec only and inserts a
  new section before the existing `help:` target.
- `CURDIR` propagation ensures `LEARN_BIN` is absolute, surviving the
  `-C tools/learn` directory change.
