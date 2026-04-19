#!/usr/bin/awk -f
# gopls-hints.awk — Rewrite common gopls diagnostics with actionable "fix by:"
# hints so LLM-driven agents know how to resolve them on first try.
#
# Usage (piped):
#   gopls check ... | awk -f .claude/hooks/gopls-hints.awk
#
# Behavior:
# - Every input line is echoed unchanged (fallback-safe).
# - If a line matches a known gopls diagnostic pattern, a "  >> fix by: ..."
#   line is printed immediately after it.
# - Pattern-to-hint pairs live in the BEGIN block — edit there to extend.
# - Matching is substring-based (via awk's index()) so patterns don't need
#   regex escaping; keep them short and distinctive.

BEGIN {
    # --- unused / unreferenced ---
    hints["declared and not used"]  = "remove the variable, or rename to '_' if you need to keep it for symmetry"
    hints["declared but not used"]  = "remove the variable, or rename to '_' if you need to keep it for symmetry"
    hints["imported and not used"]  = "remove the import, or add a blank identifier import ('_ \"path\"') if you need the side effect"

    # --- shadowing ---
    hints["shadows declaration"]    = "rename this variable so it doesn't shadow the outer binding (error handling uses unique names per project convention: parseErr, saveErr, fetchErr, etc.)"
    hints["redeclared in this block"] = "rename one of the identifiers or remove the duplicate declaration"

    # --- control flow ---
    hints["unreachable code"]       = "delete the unreachable lines"
    hints["missing return"]         = "add an explicit return statement (or a panic) at the end of this path"
    hints["expected 'IDENT'"]       = "the syntax error is usually a stray character or a missing type name — re-read the line"

    # --- nil / unsafe ---
    hints["possible nil pointer"]   = "add a nil check before dereferencing, or ensure the value is set by construction"
    hints["nil dereference"]        = "add a nil check before dereferencing, or ensure the value is set by construction"
    hints["invalid memory address"] = "add a nil check before dereferencing the pointer"

    # --- loop / closure ---
    hints["loop variable captured"] = "copy the loop variable to a local (v := v) before capturing in a closure or goroutine (Go 1.22+ no longer needs this, but staticcheck may still flag)"
    hints["range variable"]         = "copy the range variable to a local if it's used in a closure or goroutine"

    # --- formatting / printf ---
    hints["Printf format"]          = "adjust the format verbs to match the argument types, or add/remove arguments"
    hints["arg .* for "]            = "check the number and types of args passed to the format function"

    # --- style / correctness ---
    hints["assignment to"]          = "if the value is never read after assignment, remove the statement; if it's intentional (discard), use '_ = expr'"
    hints["composite literal uses unkeyed fields"] = "add field names to the composite literal for forward-compatibility"
    hints["could not import"]       = "run 'go mod tidy' — the import path is missing from go.mod or the package was moved"

    # --- type assertion ---
    hints["impossible type assertion"] = "the target type does not implement the source interface — check the method set"
    hints["type assertion"]         = "use the comma-ok form (x, ok := v.(T)) and handle the !ok branch"
}

{
    print $0
    for (p in hints) {
        if (index($0, p) > 0) {
            print "  >> fix by: " hints[p]
            break
        }
    }
}
