# Golden fixtures (approved-fixtures pattern)

Golden-file testing locks down the **shape** of a response — status code, envelope,
field names, casing — not just the presence of a handful of asserted fields. Any accidental
drift (a new field leaked, casing changed, envelope removed) fails the test until a human
reviews the diff and either fixes the code or updates the golden.

This complements field-level asserts; it does not replace them. Use asserts for logic,
goldens for structure.

## Quick start

```go
import "github.com/jrmarcello/gopherplate/tests/testutil/golden"

func TestE2E_CreateUser_Golden(t *testing.T) {
    // ... issue request, capture w.Body.Bytes()

    golden.AssertJSONWithMask(t, "create_user_201", w.Body.Bytes(),
        golden.Mask{Paths: []string{"data.id", "data.created_at"}})
}
```

On first run (or after an intentional response change), regenerate the golden:

```bash
make golden-update
# or: go test ./tests/e2e/... -update -count=1
```

Golden files live at `<pkg>/testdata/golden/<name>.json`.

## What to mask

Any field whose value legitimately changes between runs but whose **presence** and **type**
you want to keep locked:

- `id` — UUID v7, new per request
- `created_at` / `updated_at` / `deleted_at` — ISO 8601 timestamps
- Any path that includes a random seed, correlation ID, or server-generated value

Mask paths use dotted notation. `user.id` descends into nested objects. Missing paths are
silently ignored (safe for optional fields).

## Workflow after an intentional API change

1. Change the handler / DTO / response format.
2. Run `make golden-update`.
3. `git diff tests/e2e/testdata/golden/` — inspect the diff.
4. If the diff reflects exactly what you intended, commit code + golden together.
5. If the diff is larger than expected, the code change had unintended scope — fix before
   committing.

The diff is the review artifact. Reviewers should read it carefully; that is the control.

## What NOT to do

- **Don't commit goldens with dynamic values unmasked.** They will be flaky on the next run.
  If you see `id` or `created_at` in a diff you didn't expect, add them to the mask.
- **Don't use goldens for asserting business rules.** Use plain asserts with meaningful
  names: `assert.Equal(t, http.StatusCreated, w.Code)`. Goldens lock structure, not logic.
- **Don't manually edit a golden file.** Always regenerate via `-update`.
- **Don't mix masked and unmasked runs for the same test.** One source of truth per test.

## Extending

The helper supports dotted paths. Future-roadmap: array indexing (`items[0].id`) and
wildcards (`items[*].id`). File an issue or extend `tests/testutil/golden/` if you need them
before then.

## Related

- [docs/harness.md](../harness.md) — golden fixtures are listed as a behavior sensor.
- [.specs/behavior-harness.md](../../.specs/behavior-harness.md) — full spec with rationale.
- Fowler, ["Harness Engineering for Coding Agents"](https://martinfowler.com/articles/harness-engineering.html)
  — the "approved fixtures" pattern mentioned in the Behavior Harness section.
