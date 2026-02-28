package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validTasksMD = `# TASKS: Test Plan

## A. Document Intake Summary

Some intake summary.

## G. Task List

| # | File | Name | Epic | User-visible outcome | Risk | Size |
|---|------|------|------|---------------------|------|------|
| 0 | TASK0.md | Setup base project | Epic 1 — Thin E2E | Base project with CI | Low | S |
| 1 | TASK1.md | Add user auth | Epic 2 — Thin E2E | Users can log in | Medium | M |
| 2 | TASK2.md | Add dashboard | Epic 3 — Thin E2E | Dashboard displays data | Medium | M |
| 3 | TASK3.md | Add settings page | Epic 4 — Thin E2E | Users can change settings | Low | S |
| 4 | TASK4.md | Add notifications | Epic 5 — Thin E2E | Users receive notifications | High | L |

## H. Dependency Graph & Critical Path

TASK0 → TASK1 → TASK2
`

func TestCountTasksInSummary_ValidFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(validTasksMD), 0o600))

	count, err := CountTasksInSummary(dir)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestCountTasksInSummary_MissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := CountTasksInSummary(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read TASKS.md")
}

func TestCountTasksInSummary_NoSectionG(t *testing.T) {
	dir := t.TempDir()
	content := `# TASKS: Test Plan

## A. Document Intake Summary

Some intake summary.

## H. Dependency Graph
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(content), 0o600))

	count, err := CountTasksInSummary(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountTasksInSummary_EmptySectionG(t *testing.T) {
	dir := t.TempDir()
	content := `# TASKS: Test Plan

## G. Task List

No tasks here — just prose.

## H. Dependency Graph
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(content), 0o600))

	count, err := CountTasksInSummary(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestExtractTaskSpecs_ValidFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(validTasksMD), 0o600))

	specs, err := ExtractTaskSpecs(dir)
	require.NoError(t, err)
	require.Len(t, specs, 5)

	assert.Equal(t, 0, specs[0].Number)
	assert.Equal(t, "TASK0.md", specs[0].FileName)
	assert.Equal(t, "Setup base project", specs[0].Name)
	assert.Contains(t, specs[0].Spec, "TASK0.md")

	assert.Equal(t, 4, specs[4].Number)
	assert.Equal(t, "TASK4.md", specs[4].FileName)
	assert.Equal(t, "Add notifications", specs[4].Name)
}

func TestBatchSlicing(t *testing.T) {
	tests := []struct {
		name      string
		n         int
		batchSize int
		want      []int // lengths of each batch
	}{
		{"N=0", 0, 5, nil},
		{"N<B", 3, 5, []int{3}},
		{"N=B", 5, 5, []int{5}},
		{"N>B", 12, 5, []int{5, 5, 2}},
		{"B=1", 3, 1, []int{1, 1, 1}},
		{"B=0", 5, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := make([]int, tt.n)
			for i := range items {
				items[i] = i
			}
			batches := splitBatches(items, tt.batchSize)
			if tt.want == nil {
				assert.Nil(t, batches)
				return
			}
			require.Len(t, batches, len(tt.want))
			for i, wantLen := range tt.want {
				assert.Len(t, batches[i], wantLen, "batch %d", i)
			}
		})
	}
}
