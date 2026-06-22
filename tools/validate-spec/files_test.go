package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TC-06: collectFiles returns sorted union from out-of-order fixture
func TestCollectFiles_SortedUnion(t *testing.T) {
	t.Parallel()
	m := loadSpec(t, "files-union", "spec.md")
	files := collectFiles(m)
	assert.Equal(t, []string{
		"internal/domain/a/entity.go",
		"internal/domain/m/entity.go",
		"internal/domain/z/entity.go",
	}, files, "collectFiles must return sorted deduplicated union")
}

// TC-07: dedup + unique files kept
func TestCollectFiles_Dedup(t *testing.T) {
	t.Parallel()
	m := model{
		tasks: []task{
			{id: "TASK-1", files: []string{"a.go", "b.go"}},
			{id: "TASK-2", files: []string{"b.go", "c.go"}},
		},
	}
	files := collectFiles(m)
	assert.Equal(t, []string{"a.go", "b.go", "c.go"}, files)
}

// TC-22: execOnly task contributes no path to collectFiles
func TestCollectFiles_ExecOnlyExcluded(t *testing.T) {
	t.Parallel()
	m := model{
		tasks: []task{
			{id: "TASK-1", files: []string{"a.go"}},
			{id: "TASK-SMOKE", execOnly: true},
		},
	}
	files := collectFiles(m)
	assert.Equal(t, []string{"a.go"}, files, "execOnly task must contribute no paths")
}
