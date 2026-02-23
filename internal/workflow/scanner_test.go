package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanTasks(t *testing.T) {
	t.Run("finds and sorts task files numerically", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK3.md", "task 3")
		createFile(t, dir, "TASK1.md", "task 1")
		createFile(t, dir, "TASK2.md", "task 2")

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Len(t, tasks, 3)
		assert.Equal(t, TaskInfo{ID: "TASK1", Number: 1, Filename: "TASK1.md"}, tasks[0])
		assert.Equal(t, TaskInfo{ID: "TASK2", Number: 2, Filename: "TASK2.md"}, tasks[1])
		assert.Equal(t, TaskInfo{ID: "TASK3", Number: 3, Filename: "TASK3.md"}, tasks[2])
	})

	t.Run("handles non-sequential numbering", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "task 1")
		createFile(t, dir, "TASK5.md", "task 5")
		createFile(t, dir, "TASK10.md", "task 10")

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Len(t, tasks, 3)
		assert.Equal(t, 1, tasks[0].Number)
		assert.Equal(t, 5, tasks[1].Number)
		assert.Equal(t, 10, tasks[2].Number)
	})

	t.Run("ignores non-task files", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "task 1")
		createFile(t, dir, "PRD.md", "prd")
		createFile(t, dir, "progress.md", "progress")
		createFile(t, dir, "TECHNOLOGY.md", "tech")
		createFile(t, dir, "TASKS.md", "tasks")
		createFile(t, dir, "README.md", "readme")
		createFile(t, dir, "TASK2.md", "task 2")

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, "TASK1", tasks[0].ID)
		assert.Equal(t, "TASK2", tasks[1].ID)
	})

	t.Run("returns empty for no task files", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "PRD.md", "prd")
		createFile(t, dir, "progress.md", "progress")

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Empty(t, tasks)
	})

	t.Run("returns empty for empty directory", func(t *testing.T) {
		dir := t.TempDir()

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Empty(t, tasks)
	})

	t.Run("errors on duplicate task numbers from leading zeros", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "first")
		createFile(t, dir, "TASK01.md", "duplicate")

		_, err := ScanTasks(dir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate task number 1")
	})

	t.Run("errors on nonexistent directory", func(t *testing.T) {
		_, err := ScanTasks("/nonexistent/path")
		assert.Error(t, err)
	})

	t.Run("ignores directories matching pattern", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "task 1")
		// Create a directory named like a task file.
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "TASK2.md"), 0o755))

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, "TASK1", tasks[0].ID)
	})

	t.Run("handles large task numbers", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK999.md", "task 999")
		createFile(t, dir, "TASK1.md", "task 1")

		tasks, err := ScanTasks(dir)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, 1, tasks[0].Number)
		assert.Equal(t, 999, tasks[1].Number)
	})
}

func TestSelectNextTask(t *testing.T) {
	t.Run("selects first task when none completed", func(t *testing.T) {
		tasks := []TaskInfo{
			{ID: "TASK1", Number: 1, Filename: "TASK1.md"},
			{ID: "TASK2", Number: 2, Filename: "TASK2.md"},
			{ID: "TASK3", Number: 3, Filename: "TASK3.md"},
		}

		next := SelectNextTask(tasks, nil)
		assert.NotNil(t, next)
		assert.Equal(t, "TASK1", next.ID)
	})

	t.Run("skips completed tasks", func(t *testing.T) {
		tasks := []TaskInfo{
			{ID: "TASK1", Number: 1, Filename: "TASK1.md"},
			{ID: "TASK2", Number: 2, Filename: "TASK2.md"},
			{ID: "TASK3", Number: 3, Filename: "TASK3.md"},
		}

		next := SelectNextTask(tasks, []string{"TASK1", "TASK2"})
		assert.NotNil(t, next)
		assert.Equal(t, "TASK3", next.ID)
	})

	t.Run("returns nil when all completed", func(t *testing.T) {
		tasks := []TaskInfo{
			{ID: "TASK1", Number: 1, Filename: "TASK1.md"},
			{ID: "TASK2", Number: 2, Filename: "TASK2.md"},
		}

		next := SelectNextTask(tasks, []string{"TASK1", "TASK2"})
		assert.Nil(t, next)
	})

	t.Run("returns nil for empty task list", func(t *testing.T) {
		next := SelectNextTask(nil, nil)
		assert.Nil(t, next)
	})

	t.Run("handles non-sequential with gaps", func(t *testing.T) {
		tasks := []TaskInfo{
			{ID: "TASK1", Number: 1, Filename: "TASK1.md"},
			{ID: "TASK3", Number: 3, Filename: "TASK3.md"},
			{ID: "TASK5", Number: 5, Filename: "TASK5.md"},
		}

		next := SelectNextTask(tasks, []string{"TASK1"})
		assert.NotNil(t, next)
		assert.Equal(t, "TASK3", next.ID)
	})

	t.Run("handles completed IDs not in discovered tasks", func(t *testing.T) {
		tasks := []TaskInfo{
			{ID: "TASK2", Number: 2, Filename: "TASK2.md"},
			{ID: "TASK3", Number: 3, Filename: "TASK3.md"},
		}

		// TASK1 completed but file no longer exists — that's fine.
		next := SelectNextTask(tasks, []string{"TASK1"})
		assert.NotNil(t, next)
		assert.Equal(t, "TASK2", next.ID)
	})
}

func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
