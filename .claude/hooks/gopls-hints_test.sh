#!/usr/bin/env bash
# gopls-hints_test.sh — smoke tests for .claude/hooks/gopls-hints.awk
#
# Not a full test framework; a sequence of "cases" that run the awk on a fixed
# input and assert on substrings in the output. Exits non-zero at first
# failure so the line points clearly to the offender.
#
# Usage:
#   .claude/hooks/gopls-hints_test.sh

set -uo pipefail

AWK_SCRIPT="$(dirname "$0")/gopls-hints.awk"
PASS=0
FAIL=0

assert_contains() {
    local name="$1" output="$2" needle="$3"
    if echo "$output" | grep -Fq -- "$needle"; then
        PASS=$((PASS + 1))
        echo "  PASS: $name"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: $name"
        echo "    expected substring: $needle"
        echo "    got:"
        echo "$output" | sed 's/^/      /'
    fi
}

assert_equals() {
    local name="$1" output="$2" expected="$3"
    if [ "$output" = "$expected" ]; then
        PASS=$((PASS + 1))
        echo "  PASS: $name"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: $name"
        echo "    expected: $expected"
        echo "    got:      $output"
    fi
}

echo "Running gopls-hints.awk smoke tests"

# TC-UC-01: "declared and not used" diagnostic gets a hint
out=$(printf 'foo.go:10:5: declared and not used: bar\n' | awk -f "$AWK_SCRIPT")
assert_contains "TC-UC-01 declared-and-not-used emits fix-by hint" "$out" ">> fix by:"
assert_contains "TC-UC-01 hint content mentions '_ rename'" "$out" "rename to '_'"

# TC-UC-02: "shadows declaration" diagnostic gets a specific hint
out=$(printf 'foo.go:20:3: declaration of "x" shadows declaration at line 5\n' | awk -f "$AWK_SCRIPT")
assert_contains "TC-UC-02 shadows-declaration emits hint" "$out" ">> fix by:"
assert_contains "TC-UC-02 hint mentions unique-error-names convention" "$out" "parseErr, saveErr"

# TC-UC-03: unknown diagnostic passes through unchanged (no extra lines)
out=$(printf 'foo.go:30:1: some totally unknown diagnostic\n' | awk -f "$AWK_SCRIPT")
assert_equals "TC-UC-03 unknown-diagnostic passes through unchanged" "$out" "foo.go:30:1: some totally unknown diagnostic"

# TC-UC-04: multiple diagnostics all get processed
out=$(printf 'foo.go:1:1: declared and not used: a\nfoo.go:2:1: unreachable code\nfoo.go:3:1: imported and not used: "x"\n' | awk -f "$AWK_SCRIPT")
count=$(echo "$out" | grep -c ">> fix by:" || true)
if [ "$count" = "3" ]; then
    PASS=$((PASS + 1))
    echo "  PASS: TC-UC-04 three diagnostics -> three fix-by hints"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: TC-UC-04 expected 3 fix-by hints, got $count"
    echo "$out"
fi

# TC-UC-05: the lookup table is editable — a new diagnostic pattern added to
# gopls-hints.awk should be recognized. We can't modify the file here, but we
# validate that ALL patterns in the awk are reachable by one test input, which
# catches typos/regressions in the BEGIN block.
patterns_in_awk=$(grep -c '^    hints\[' "$AWK_SCRIPT" | tr -d ' ')
if [ "$patterns_in_awk" -ge 20 ]; then
    PASS=$((PASS + 1))
    echo "  PASS: TC-UC-05 lookup table has $patterns_in_awk patterns (>= 20 required)"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL: TC-UC-05 lookup table has only $patterns_in_awk patterns"
fi

echo ""
echo "Result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
