# CLI flavors

`gopherplate new` scaffolds a service from the template. A **flavor** is a
named variant of that scaffold: same base skeleton, different harness
pre-wiring for the topology.

The name comes from Martin Fowler's ["Harness Engineering for Coding
Agents"](https://martinfowler.com/articles/harness-engineering.html) —
specifically the recommendation to ship *harness templates per service
topology*: a CRUD service and an event processor have different fitness
functions, different sensor profiles, and different review patterns.

## Overview

```bash
gopherplate new my-service                      # defaults to --flavor crud
gopherplate new my-service --flavor crud        # explicit
gopherplate new --help                          # lists currently-registered flavors
```

Each flavor adds **overlays** on top of the base scaffold. An overlay is a
typed action (create file, append, insert at marker, overwrite with sentinel,
or add a go.mod require) that runs after the base copy+rewrite completes.

### Currently registered flavors

| Flavor | Topology | Status |
| --- | --- | --- |
| `crud` (default) | HTTP+gRPC CRUD with Postgres | **Implemented** — zero overlays; inherits all base harness artifacts (`.semgrep/`, `tests/load/baselines/`, `tests/testutil/golden/`). |
| `event-processor` | Redis Streams consumer with retry/DLQ + lag metric | **Planned** — see [.specs/flavors-event-data.md](../../.specs/flavors-event-data.md). |
| `data-pipeline` | Batch worker with benchmark-based perf gate + idempotency key contract | **Planned** — same follow-up spec. |

## How overlays work

The overlay engine lives in
[`cmd/cli/flavors/overlay.go`](../../cmd/cli/flavors/overlay.go) and supports
five actions:

| Action | Purpose | Safety |
| --- | --- | --- |
| `create` | Write a new file | Fails if path exists (no silent clobber) |
| `append` | Add to end of existing file | Requires file to exist |
| `insert-marker` | Inject at a comment-line marker in the base (`# @flavor-makefile-targets`, `// @flavor-di-wiring`, etc.) | Fails if marker absent — forces explicit base preparation |
| `overwrite` | Replace file entirely | Requires template to contain sentinel `overlay: overwrite <reason>` in a comment; emits warning |
| `go-mod-require` | Add a module require via `golang.org/x/mod/modfile` | Resolves version conflicts by picking the higher one + warning |

All paths are validated against traversal (`../` rejected) before any write.
The engine is unit-tested in
[`cmd/cli/flavors/overlay_test.go`](../../cmd/cli/flavors/overlay_test.go) with
fixtures covering each action's happy path and error modes.

## How to add a new flavor

Adding a flavor is **three files** and one line in the registry.

### 1. Create the constructor

`cmd/cli/flavors/<yourflavor>.go`:

```go
package flavors

func YourFlavor() Flavor {
    return Flavor{
        ID:          "your-flavor",
        Description: "Short one-line description for --help",
        Overlays: []Overlay{
            {
                Action:   ActionCreate,
                Path:     ".semgrep/your-flavor.yml",
                Template: yourFlavorSemgrepRule, // string const or embed.FS content
            },
            {
                Action: ActionGoModRequire,
                Path:   "go.mod",
                Module: "github.com/example/dep v1.2.3",
            },
            // ... more overlays
        },
    }
}
```

Keep templates short; if they're big, put them in
`cmd/cli/flavors/<yourflavor>/templates/` and load via `//go:embed`.

### 2. Register the flavor

Add a line in [`cmd/cli/flavors/default.go`](../../cmd/cli/flavors/default.go):

```go
flavors := []Flavor{
    Crud(),
    YourFlavor(), // <- add here
}
```

### 3. Write tests

Add unit tests for any custom overlay logic in
`cmd/cli/flavors/<yourflavor>_test.go`. For end-to-end validation, extend
[`cmd/cli/commands/flavors_e2e_test.go`](../../cmd/cli/commands/flavors_e2e_test.go)
with a `TestE2E_NewFlavorYourFlavor_Builds` that scaffolds and verifies the
output compiles.

### Check-in checklist

- [ ] Constructor file compiles and passes `go vet`.
- [ ] Overlay templates are accurate — run the CLI manually at least once with
      `--flavor your-flavor` and confirm the scaffold builds.
- [ ] Unit tests cover each custom overlay action.
- [ ] E2E test in `flavors_e2e_test.go` exercises the full scaffold + build.
- [ ] `--help` output lists the new flavor (auto-generated via `Registry.List()`
      — nothing to edit).
- [ ] Update this document's "Currently registered flavors" table.
- [ ] Update [`docs/guides/template-cli.md`](template-cli.md) if the flavor
      introduces new top-level options.

## Validation that runs automatically

Every `gopherplate new` invocation, regardless of flavor:

1. Rejects invalid service names (must match `^[a-z][a-z0-9-]*$`).
2. Refuses to scaffold if the target directory already exists.
3. Runs `go build ./...` in the generated tree as a smoke test; prints a
   WARNING if build fails but leaves the scaffold in place for inspection.

## Related

- [docs/harness.md](../harness.md) — flavors are listed under "CLI
  (scaffolders)" as architecture-fitness guides.
- [.specs/cli-harness-flavors.md](../../.specs/cli-harness-flavors.md) — full
  spec with rationale and test plan.
- Fowler, ["Harness Engineering for Coding
  Agents"](https://martinfowler.com/articles/harness-engineering.html).
