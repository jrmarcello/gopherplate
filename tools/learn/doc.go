// Package learn is the learning-loop CLI for the gopherplate harness:
// pattern extraction over execution logs / transcripts / git, FTS5-backed
// retrieval, periodic nudge with TTL-based deprecation, and explicit
// decision tracking for skill/memory refinement.
//
// Layout: a single flat package at the root of `tools/learn/`. The module
// was refactored from a 14-internal-package Clean-Architecture layout on
// 2026-05-15 (see .specs/refactor-tools-learn-flat-layout.md for rationale).
// Identifiers are camelCase (unexported) except for three exceptions
// consumed by `package main` in cmd/learn/main.go: NewRootCmd, RunCmd,
// RegisterAll.
package learn
