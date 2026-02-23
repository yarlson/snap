package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/state"
)

func TestResolveStartup(t *testing.T) {
	t.Run("returns select action for nil state", func(t *testing.T) {
		target, err := resolveStartup(nil, t.TempDir(), 9)
		require.NoError(t, err)
		assert.Equal(t, actionSelect, target.action)
	})

	t.Run("returns select action for idle state", func(t *testing.T) {
		s := state.NewState("docs/tasks", "PRD.md", 9)
		target, err := resolveStartup(s, t.TempDir(), 9)
		require.NoError(t, err)
		assert.Equal(t, actionSelect, target.action)
	})

	t.Run("returns resume target for valid active state", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "# Task 1")
		createFile(t, dir, "TASK2.md", "# Task 2")

		s := state.NewState(dir, "PRD.md", 9)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 3

		target, err := resolveStartup(s, dir, 9)
		require.NoError(t, err)
		assert.Equal(t, actionResume, target.action)
		assert.Equal(t, "TASK1", target.taskID)
		assert.Equal(t, "TASK1.md", target.taskFile)
		assert.Equal(t, 3, target.step)
	})

	t.Run("errors when active task file missing from directory", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK2.md", "# Task 2") // TASK1.md is missing

		s := state.NewState(dir, "PRD.md", 9)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 1

		_, err := resolveStartup(s, dir, 9)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TASK1")
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "--fresh")
	})

	t.Run("errors when active task already completed", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "# Task 1")

		// Construct state manually (bypasses IsValid which also catches this).
		s := &state.State{
			TasksDir:         dir,
			CurrentTaskID:    "TASK1",
			CurrentTaskFile:  "TASK1.md",
			CurrentStep:      1,
			TotalSteps:       9,
			CompletedTaskIDs: []string{"TASK1"},
			PRDPath:          "PRD.md",
		}

		_, err := resolveStartup(s, dir, 9)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already")
		assert.Contains(t, err.Error(), "--fresh")
	})

	t.Run("errors when step is zero", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "# Task 1")

		s := state.NewState(dir, "PRD.md", 9)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 0

		_, err := resolveStartup(s, dir, 9)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid step")
		assert.Contains(t, err.Error(), "--fresh")
	})

	t.Run("errors when step exceeds total", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "# Task 1")

		s := state.NewState(dir, "PRD.md", 5)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 7 // > totalSteps(5) + 1

		_, err := resolveStartup(s, dir, 5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid step")
	})

	t.Run("valid when step equals total steps", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "# Task 1")

		s := state.NewState(dir, "PRD.md", 9)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 9

		target, err := resolveStartup(s, dir, 9)
		require.NoError(t, err)
		assert.Equal(t, actionResume, target.action)
		assert.Equal(t, 9, target.step)
	})

	t.Run("valid when step equals total steps plus one (cleanup pending)", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK1.md", "# Task 1")

		s := state.NewState(dir, "PRD.md", 9)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 10 // totalSteps + 1: all steps done, cleanup pending

		target, err := resolveStartup(s, dir, 9)
		require.NoError(t, err)
		assert.Equal(t, actionResume, target.action)
		assert.Equal(t, 10, target.step)
	})

	t.Run("errors when tasks directory cannot be read", func(t *testing.T) {
		s := state.NewState("/nonexistent", "PRD.md", 9)
		s.CurrentTaskID = "TASK1"
		s.CurrentTaskFile = "TASK1.md"
		s.CurrentStep = 1

		_, err := resolveStartup(s, "/nonexistent", 9)
		assert.Error(t, err)
	})

	t.Run("backfills CurrentTaskFile when empty on resume", func(t *testing.T) {
		dir := t.TempDir()
		createFile(t, dir, "TASK2.md", "# Task 2")

		// Simulate v1 migration: ID is set but file is empty.
		s := state.NewState(dir, "PRD.md", 9)
		s.CurrentTaskID = "TASK2"
		s.CurrentTaskFile = "" // empty, as after v1 migration
		s.CurrentStep = 3

		target, err := resolveStartup(s, dir, 9)
		require.NoError(t, err)
		assert.Equal(t, actionResume, target.action)
		assert.Equal(t, "TASK2", target.taskID)
		assert.Equal(t, "TASK2.md", target.taskFile)
		// State should also be updated in place.
		assert.Equal(t, "TASK2.md", s.CurrentTaskFile)
	})

	t.Run("does not call scanner when state is idle", func(t *testing.T) {
		// Pass a nonexistent directory. If the scanner were called,
		// it would fail. Idle state should not trigger scanning.
		s := state.NewState("/nonexistent", "PRD.md", 9)
		target, err := resolveStartup(s, "/nonexistent", 9)
		require.NoError(t, err)
		assert.Equal(t, actionSelect, target.action)
	})
}
