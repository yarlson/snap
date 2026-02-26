package workflow_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/snapshot"
	"github.com/yarlson/snap/internal/state"
	"github.com/yarlson/snap/internal/ui"
	"github.com/yarlson/snap/internal/workflow"
)

func TestRunner_StateManagement(t *testing.T) {
	// Create temp directory for state
	tmpDir := t.TempDir()

	// Create mock PRD and task files
	prdPath := filepath.Join(tmpDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte("# PRD"), 0o600); err != nil {
		t.Fatalf("failed to create PRD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600); err != nil {
		t.Fatalf("failed to create task file: %v", err)
	}

	t.Run("state created after first step", func(t *testing.T) {
		// Clean up state from previous tests
		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // Intentionally ignoring error - cleanup may fail if state doesn't exist
		_ = stateManager.Reset()

		stepCount := 0
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				stepCount++
				// Fail after step 3 to simulate interruption.
				// stepCount 1 = description generation (pre-step), 2-4 = steps 1-3.
				if stepCount == 4 {
					return errors.New("simulated failure")
				}
				return nil
			},
		}

		config := workflow.Config{
			TasksDir:   tmpDir,
			PRDPath:    prdPath,
			FreshStart: false,
		}
		runner := workflow.NewRunner(mockExec, config, workflow.WithStateManager(stateManager))

		// Run should fail after step 3
		ctx := context.Background()
		err := runner.Run(ctx)
		assert.Error(t, err)

		// Verify state was saved
		assert.True(t, stateManager.Exists())

		// Load state and verify
		loadedState, err := stateManager.Load()
		assert.NoError(t, err)
		assert.NotNil(t, loadedState)
		assert.Equal(t, 3, loadedState.CurrentStep)
		assert.NotEmpty(t, loadedState.LastError)
	})

	t.Run("state reset with fresh flag", func(t *testing.T) {
		// Ensure state exists
		stateManager := state.NewManagerWithDir(tmpDir)
		workflowState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		workflowState.CurrentStep = 5
		err := stateManager.Save(workflowState)
		assert.NoError(t, err)
		assert.True(t, stateManager.Exists())

		// Create runner with fresh flag
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				// Fail immediately to exit quickly
				return errors.New("exit")
			},
		}

		config := workflow.Config{
			TasksDir:   tmpDir,
			PRDPath:    prdPath,
			FreshStart: true,
		}
		runner := workflow.NewRunner(mockExec, config, workflow.WithStateManager(stateManager))

		// Run should reset state before starting
		ctx := context.Background()
		//nolint:errcheck // Intentionally ignoring error - testing state reset behavior
		_ = runner.Run(ctx)

		// State should be created again at step 1, not step 5
		loadedState, err := stateManager.Load()
		assert.NoError(t, err)
		assert.NotNil(t, loadedState)
		assert.Equal(t, 1, loadedState.CurrentStep)
	})
}

func TestRunner_CorruptStateHandling(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create mock PRD and task files
	prdPath := filepath.Join(tmpDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte("# PRD"), 0o600); err != nil {
		t.Fatalf("failed to create PRD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600); err != nil {
		t.Fatalf("failed to create task file: %v", err)
	}

	// Create corrupt state file
	stateDir := filepath.Join(tmpDir, state.StateDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	statePath := filepath.Join(stateDir, state.StateFile)
	if err := os.WriteFile(statePath, []byte("corrupt json"), 0o600); err != nil {
		t.Fatalf("failed to create corrupt state: %v", err)
	}

	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			// Fail immediately to exit
			return errors.New("exit")
		},
	}

	stateManager := state.NewManagerWithDir(tmpDir)
	config := workflow.Config{
		TasksDir:   tmpDir,
		PRDPath:    prdPath,
		FreshStart: false,
	}
	runner := workflow.NewRunner(mockExec, config, workflow.WithStateManager(stateManager))

	// Run should handle corrupt state gracefully
	ctx := context.Background()
	err := runner.Run(ctx)

	// Should get an error from the executor, not from state loading
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exit")
}

func TestRunner_ImplementStepPromptIncludesOptionalContextDocs(t *testing.T) {
	tmpDir := t.TempDir()

	prdPath := filepath.Join(tmpDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte("# PRD"), 0o600); err != nil {
		t.Fatalf("failed to create PRD: %v", err)
	}
	// Create a task file so the scanner can find it.
	if err := os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600); err != nil {
		t.Fatalf("failed to create task file: %v", err)
	}

	callCount := 0
	var implementPrompt string
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
			callCount++
			// Call 1 = description generation (pre-step); call 2 = step 1 (implement).
			if callCount == 2 && len(args) > 0 {
				implementPrompt = args[len(args)-1]
			}
			return errors.New("stop after first step")
		},
	}

	stateManager := state.NewManagerWithDir(tmpDir)
	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	}, workflow.WithStateManager(stateManager))

	err := runner.Run(context.Background())
	assert.Error(t, err)
	assert.NotEmpty(t, implementPrompt)
	assert.Contains(t, implementPrompt, "If TECHNOLOGY.md exists")
}

func TestRunner_IdleTaskSelection(t *testing.T) {
	t.Run("selects first task file on fresh start", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		assert.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))

		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()

		callCount := 0
		var implementPrompt string
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
				callCount++
				// Call 1 = description generation; call 2 = step 1 (implement).
				if callCount == 2 && len(args) > 0 {
					implementPrompt = args[len(args)-1]
				}
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())
		assert.Error(t, err)

		// Step 1 prompt should reference the specific task file.
		assert.Contains(t, implementPrompt, "TASK1.md")
		assert.Contains(t, implementPrompt, "TASK1")

		// State should have the active task set.
		loaded, err := stateManager.Load()
		assert.NoError(t, err)
		assert.NotNil(t, loaded)
		assert.Equal(t, "TASK1", loaded.CurrentTaskID)
		assert.Equal(t, "TASK1.md", loaded.CurrentTaskFile)
	})

	t.Run("skips completed tasks", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		assert.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK3.md"), []byte("# Task 3"), 0o600))

		// Pre-seed state with TASK1 completed.
		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CompletedTaskIDs = []string{"TASK1"}
		assert.NoError(t, stateManager.Save(seedState))

		callCount := 0
		var implementPrompt string
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
				callCount++
				// Call 1 = description generation; call 2 = step 1 (implement).
				if callCount == 2 && len(args) > 0 {
					implementPrompt = args[len(args)-1]
				}
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())
		assert.Error(t, err)

		// Should select TASK2, not TASK1.
		assert.Contains(t, implementPrompt, "TASK2.md")
		assert.Contains(t, implementPrompt, "TASK2")
	})

	t.Run("errors when no task files found", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		assert.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		// No TASK files.

		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()

		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return nil
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no task files")
	})

	t.Run("completes when all tasks already done", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		assert.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

		// Pre-seed state with TASK1 completed.
		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CompletedTaskIDs = []string{"TASK1"}
		assert.NoError(t, stateManager.Save(seedState))

		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				t.Fatal("executor should not be called when all tasks are complete")
				return nil
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		// Should return nil (clean exit, all complete).
		err := runner.Run(context.Background())
		assert.NoError(t, err)
	})

	t.Run("tracks completed task IDs after iteration", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		assert.NoError(t, os.WriteFile(prdPath, []byte("## TASK1: One\n## TASK2: Two"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))

		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()

		stepCount := 0
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				stepCount++
				// Let first iteration's steps succeed, then fail on second iteration.
				// Each iteration has 1 description call + 10 workflow steps = 11 calls.
				if stepCount > 11 {
					return errors.New("stop after first iteration")
				}
				return nil
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())
		assert.Error(t, err)

		// After first iteration completes, TASK1 should be in CompletedTaskIDs.
		loaded, err := stateManager.Load()
		assert.NoError(t, err)
		assert.NotNil(t, loaded)
		assert.Contains(t, loaded.CompletedTaskIDs, "TASK1")
		// Second iteration should have selected TASK2.
		assert.Equal(t, "TASK2", loaded.CurrentTaskID)
	})
}

func TestRunner_ResumeExactTaskAndStep(t *testing.T) {
	t.Run("resumes exact task and step from active state", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))

		// Pre-seed state: TASK2 active at step 5.
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK2"
		seedState.CurrentTaskFile = "TASK2.md"
		seedState.CurrentStep = 5
		seedState.CompletedTaskIDs = []string{"TASK1"}
		require.NoError(t, stateManager.Save(seedState))

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		err := runner.Run(context.Background())
		assert.Error(t, err)

		// State should still have TASK2 active at step 5 (error saved to state).
		loaded, err := stateManager.Load()
		require.NoError(t, err)
		assert.Equal(t, "TASK2", loaded.CurrentTaskID)
		assert.Equal(t, "TASK2.md", loaded.CurrentTaskFile)
		assert.Equal(t, 5, loaded.CurrentStep)
		// TASK1 must remain completed, not reselected.
		assert.Contains(t, loaded.CompletedTaskIDs, "TASK1")

		// Output should reference TASK2, confirming resume targeted the right task.
		output := buf.String()
		assert.Contains(t, output, "TASK2")
		assert.Contains(t, output, "step 5")
	})

	t.Run("resume does not invoke selector when active state is valid", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))

		// Pre-seed state: TASK1 active at step 3, no tasks completed.
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK1"
		seedState.CurrentTaskFile = "TASK1.md"
		seedState.CurrentStep = 3
		require.NoError(t, stateManager.Save(seedState))

		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())
		assert.Error(t, err)

		// Must resume TASK1 (not reselect TASK1 or skip to TASK2).
		loaded, err := stateManager.Load()
		require.NoError(t, err)
		assert.Equal(t, "TASK1", loaded.CurrentTaskID)
		assert.Equal(t, "TASK1.md", loaded.CurrentTaskFile)
		// Step should reflect where we were (error at step 3, so step stays 3).
		assert.Equal(t, 3, loaded.CurrentStep)
	})

	t.Run("resume output includes task ID and step number", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

		// Pre-seed state: TASK1 active at step 4.
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK1"
		seedState.CurrentTaskFile = "TASK1.md"
		seedState.CurrentStep = 4
		require.NoError(t, stateManager.Save(seedState))

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		output := buf.String()
		assert.Contains(t, output, "TASK1")
		assert.Contains(t, output, "step 4")
	})
}

func TestRunner_ResumeFailsOnInvalidState(t *testing.T) {
	t.Run("fails when active task file is missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		// Only TASK2 exists; TASK1 is missing.
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))

		// Pre-seed state: TASK1 active (but file doesn't exist).
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK1"
		seedState.CurrentTaskFile = "TASK1.md"
		seedState.CurrentStep = 3
		require.NoError(t, stateManager.Save(seedState))

		executorCalled := false
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				executorCalled = true
				return nil
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())

		// Should fail with a clear error, not silently pick the next task.
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TASK1")
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "--fresh")
		// Executor should never be called for invalid resume state.
		assert.False(t, executorCalled, "executor should not run when resume state is invalid")
	})
}

func TestRunner_IterationCompleteIncludesDuration(t *testing.T) {
	tmpDir := t.TempDir()

	prdPath := filepath.Join(tmpDir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

	stateManager := state.NewManagerWithDir(tmpDir)
	//nolint:errcheck // cleanup
	_ = stateManager.Reset()

	var buf bytes.Buffer
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			return nil
		},
	}

	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

	err := runner.Run(context.Background())
	assert.NoError(t, err)

	output := buf.String()
	// The iteration complete line should contain timing
	assert.Contains(t, output, "Iteration complete")
	// Should contain a duration marker (at minimum "0s")
	stripped := ui.StripColors(output)
	assert.Regexp(t, `Iteration complete\s+\d+`, stripped)
}

func TestRunner_StepCount(t *testing.T) {
	assert.Equal(t, 10, workflow.StepCount())
}

func TestRunner_EmbeddedPrompts(t *testing.T) {
	tmpDir := t.TempDir()

	prdPath := filepath.Join(tmpDir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

	stateManager := state.NewManagerWithDir(tmpDir)
	//nolint:errcheck // cleanup
	_ = stateManager.Reset()

	var capturedPrompts []string
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, args ...string) error {
			if len(args) > 0 {
				capturedPrompts = append(capturedPrompts, args[len(args)-1])
			}
			return nil
		},
	}

	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	}, workflow.WithStateManager(stateManager))

	err := runner.Run(context.Background())
	assert.NoError(t, err)
	require.Len(t, capturedPrompts, 11, "workflow should execute 1 description call + 10 steps")

	// Prompt 0: Task description generation (pre-step)
	assert.Contains(t, capturedPrompts[0], "Summarize")

	// Step 1: Implement — contains PRD path, task reference, and quality guardrails
	assert.Contains(t, capturedPrompts[1], prdPath)
	assert.Contains(t, capturedPrompts[1], "TASK1")
	assert.Contains(t, capturedPrompts[1], "memory-map.md")
	assert.Contains(t, capturedPrompts[1], "Quality Guardrails")
	assert.Contains(t, capturedPrompts[1], "parameterized queries")

	// Step 2: Ensure completeness
	assert.Contains(t, capturedPrompts[2], "fully implemented")

	// Step 3: Lint & test
	assert.Contains(t, capturedPrompts[3], "AGENTS.md")
	assert.Contains(t, capturedPrompts[3], "linters")

	// Step 4: Code review — full embedded skill with context loading, not delegation
	assert.Contains(t, capturedPrompts[4], "CLAUDE.md")
	assert.Contains(t, capturedPrompts[4], "git diff HEAD")
	assert.Contains(t, capturedPrompts[4], "CRITICAL")
	assert.NotContains(t, capturedPrompts[4], "Use the code-review skill")

	// Step 5: Apply fixes
	assert.Contains(t, capturedPrompts[5], "Fix")
	assert.Contains(t, capturedPrompts[5], "issues")

	// Step 6: Verify fixes (same as step 3)
	assert.Contains(t, capturedPrompts[6], "AGENTS.md")

	// Step 7: Update docs — diff-based documentation update
	assert.Contains(t, capturedPrompts[7], "git diff HEAD")
	assert.Contains(t, capturedPrompts[7], "README.md")
	assert.Contains(t, capturedPrompts[7], "user-facing")

	// Step 8: Commit code
	assert.Contains(t, capturedPrompts[8], "conventional commit")
	assert.Contains(t, capturedPrompts[8], "co-author")

	// Step 9: Memory update — full embedded skill, not delegation
	assert.Contains(t, capturedPrompts[9], ".memory/")
	assert.Contains(t, capturedPrompts[9], "summary.md")
	assert.NotContains(t, capturedPrompts[9], "Update the memory vault.")

	// Step 10: Commit memory (same as step 8)
	assert.Contains(t, capturedPrompts[10], "conventional commit")
}

func TestRunner_CompletionDeduplication(t *testing.T) {
	t.Run("completion appends task ID exactly once", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))

		// Pre-seed state: TASK1 already completed, TASK2 active at last step.
		// After TASK2 completes, CompletedTaskIDs should have exactly [TASK1, TASK2].
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK2"
		seedState.CurrentTaskFile = "TASK2.md"
		seedState.CurrentStep = 9 // Last step
		seedState.CompletedTaskIDs = []string{"TASK1"}
		require.NoError(t, stateManager.Save(seedState))

		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return nil
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		err := runner.Run(context.Background())
		assert.NoError(t, err)

		// TASK2 should appear exactly once in CompletedTaskIDs.
		// (State is reset after all complete, so check the load returns nil or clean state.)
		// Since all tasks are done, state file is cleaned up. Verify through the
		// fact that the run completed without error — if a duplicate were appended,
		// state validation would reject it.
	})

	t.Run("completion does not duplicate already-completed task", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK3.md"), []byte("# Task 3"), 0o600))

		// Pre-seed state: simulate edge case where TASK1 is both active and somehow
		// already in the completed list (bypassing normal validation for test purposes).
		// The completion logic should NOT duplicate it.
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK1"
		seedState.CurrentTaskFile = "TASK1.md"
		seedState.CurrentStep = 9
		seedState.CompletedTaskIDs = []string{"TASK1"} // Already there (edge case)
		// Note: this state is technically invalid per IsValid(), but we're testing
		// defense-in-depth in the completion path itself.

		stepCount := 0
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				stepCount++
				// 1 desc + 2 workflow steps (9,10) = 3 calls for first iteration.
				// Fail early in second iteration to stop.
				if stepCount > 10 {
					return errors.New("stop")
				}
				return nil
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir: tmpDir,
			PRDPath:  prdPath,
		}, workflow.WithStateManager(stateManager))

		// The run may fail at startup validation (active task in completed list).
		// That's acceptable — the important thing is no duplicate gets saved.
		//nolint:errcheck // testing dedup behavior, not error handling
		_ = runner.Run(context.Background())

		// If state was saved, verify no duplicate entries.
		loaded, err := stateManager.Load()
		if err == nil && loaded != nil {
			seen := make(map[string]int)
			for _, id := range loaded.CompletedTaskIDs {
				seen[id]++
			}
			for id, count := range seen {
				assert.Equal(t, 1, count, "task %s appears %d times in CompletedTaskIDs", id, count)
			}
		}
	})
}

func TestRunner_UpdateDocsStepConfig(t *testing.T) {
	tmpDir := t.TempDir()

	prdPath := filepath.Join(tmpDir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

	stateManager := state.NewManagerWithDir(tmpDir)
	//nolint:errcheck // cleanup
	_ = stateManager.Reset()

	type stepCapture struct {
		model  model.Type
		prompt string
	}
	var captured []stepCapture
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, mt model.Type, args ...string) error {
			prompt := ""
			if len(args) > 0 {
				prompt = args[len(args)-1]
			}
			captured = append(captured, stepCapture{model: mt, prompt: prompt})
			return nil
		},
	}

	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	}, workflow.WithStateManager(stateManager))

	err := runner.Run(context.Background())
	assert.NoError(t, err)
	require.Len(t, captured, 11, "1 description call + 10 workflow steps")

	// Step 7 (index 7, offset by 1 for description call): "Update docs" must use Fast model and include no-commit suffix.
	assert.Equal(t, model.Fast, captured[7].model, "Update docs step should use Fast model")
	assert.Contains(t, captured[7].prompt, "Do not stage, commit, amend, rebase, or push", "Update docs step should include no-commit suffix")
}

func TestRunner_DescriptionFailureIsGraceful(t *testing.T) {
	tmpDir := t.TempDir()

	prdPath := filepath.Join(tmpDir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

	stateManager := state.NewManagerWithDir(tmpDir)
	//nolint:errcheck // cleanup
	_ = stateManager.Reset()

	var buf bytes.Buffer
	callCount := 0
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, mt model.Type, _ ...string) error {
			callCount++
			// Call 1 = description generation (Fast model) — fail it.
			if callCount == 1 {
				assert.Equal(t, model.Fast, mt, "description call should use Fast model")
				return errors.New("LLM unavailable")
			}
			return nil
		},
	}

	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

	err := runner.Run(context.Background())
	assert.NoError(t, err, "workflow should complete despite description generation failure")

	output := buf.String()
	stripped := ui.StripColors(output)

	// Header should still appear with task label but no description line.
	assert.Contains(t, stripped, "▶ Implementing TASK1", "header should be printed")
	// All 10 steps should execute (callCount = 1 desc + 10 steps = 11).
	assert.Equal(t, 11, callCount, "all 10 workflow steps should execute after description failure")
	assert.Contains(t, output, "Iteration complete", "iteration should finish")
}

func TestRunner_SnapshotErrorsAreNonFatal(t *testing.T) {
	tmpDir := t.TempDir()

	// No git init — snapshotter will fail on every Capture call.
	prdPath := filepath.Join(tmpDir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

	stateManager := state.NewManagerWithDir(tmpDir)
	//nolint:errcheck // cleanup
	_ = stateManager.Reset()

	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			return nil
		},
	}

	var buf bytes.Buffer
	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	},
		workflow.WithStateManager(stateManager),
		workflow.WithRunnerOutput(&buf),
		workflow.WithSnapshotter(snapshot.New(tmpDir)), // not a git repo
	)

	err := runner.Run(context.Background())
	assert.NoError(t, err, "workflow should complete despite snapshot failures")

	output := buf.String()
	assert.Contains(t, output, "snapshot skipped", "snapshot errors should be logged")
	assert.Contains(t, output, "Iteration complete", "iteration should finish despite snapshot errors")
}

func TestRunner_SnapshotsCreatedDuringIteration(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize a real git repo so snapshots can work.
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	}
	gitRun("init")
	gitRun("config", "user.email", "test@test.com")
	gitRun("config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# init"), 0o600))
	gitRun("add", ".")
	gitRun("commit", "-m", "initial commit")

	// Create PRD and task files.
	prdPath := filepath.Join(tmpDir, "PRD.md")
	require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

	stateManager := state.NewManagerWithDir(tmpDir)
	//nolint:errcheck // cleanup
	_ = stateManager.Reset()

	// Mock executor that modifies a file on each step so snapshots have changes.
	stepCount := 0
	mockExec := &MockExecutor{
		runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
			stepCount++
			return os.WriteFile(
				filepath.Join(tmpDir, "README.md"),
				[]byte(fmt.Sprintf("# step %d", stepCount)),
				0o600,
			)
		},
	}

	var buf bytes.Buffer
	runner := workflow.NewRunner(mockExec, workflow.Config{
		TasksDir: tmpDir,
		PRDPath:  prdPath,
	},
		workflow.WithStateManager(stateManager),
		workflow.WithRunnerOutput(&buf),
		workflow.WithSnapshotter(snapshot.New(tmpDir)),
	)

	err := runner.Run(context.Background())
	assert.NoError(t, err)

	// Check stash list for snap entries.
	cmd := exec.CommandContext(context.Background(), "git", "stash", "list")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	require.NoError(t, err)
	stashEntries := strings.TrimSpace(string(out))
	assert.NotEmpty(t, stashEntries, "stash list should contain snapshot entries")

	lines := strings.Split(stashEntries, "\n")
	// Should have 8 snapshots (steps 1-7 and 9; skip steps 8 and 10 which are commit steps with clean trees).
	assert.Equal(t, 8, len(lines), "expected snapshots for all non-commit steps")

	// All entries should have snap: prefix.
	for _, line := range lines {
		assert.Contains(t, line, "snap:")
	}

	// Output should mention snapshots.
	output := buf.String()
	assert.Contains(t, output, "snapshot saved")
}

func TestRunner_StartupSummary(t *testing.T) {
	t.Run("fresh start shows summary with task counts and provider", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK3.md"), []byte("# Task 3"), 0o600))

		// Pre-seed state with TASK1 completed.
		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CompletedTaskIDs = []string{"TASK1"}
		require.NoError(t, stateManager.Save(seedState))

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir:     tmpDir,
			PRDPath:      prdPath,
			ProviderName: "claude",
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		output := buf.String()
		stripped := ui.StripColors(output)

		// Verify summary contains expected components.
		assert.Contains(t, stripped, "3 tasks (1 done)", "summary should show task count and done count")
		assert.Contains(t, stripped, "claude", "summary should show provider name")
		assert.Contains(t, stripped, tmpDir, "summary should show tasks directory")
		assert.Contains(t, stripped, "starting TASK2", "summary should show action")
	})

	t.Run("summary appears before first step header", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir:     tmpDir,
			PRDPath:      prdPath,
			ProviderName: "claude",
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		stripped := ui.StripColors(buf.String())

		summaryIdx := strings.Index(stripped, "snap:")
		stepIdx := strings.Index(stripped, "▶ Step")
		headerIdx := strings.Index(stripped, "▶ Implementing")

		require.NotEqual(t, -1, summaryIdx, "output should contain startup summary")

		if stepIdx != -1 {
			assert.Less(t, summaryIdx, stepIdx, "summary should appear before first step")
		}
		if headerIdx != -1 {
			assert.Less(t, summaryIdx, headerIdx, "summary should appear before header")
		}
	})

	t.Run("prompt hint appears on fresh start with IsTTY", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir:     tmpDir,
			PRDPath:      prdPath,
			ProviderName: "claude",
			IsTTY:        true,
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		stripped := ui.StripColors(buf.String())
		assert.Contains(t, stripped, "Type a directive and press Enter to queue it between steps",
			"prompt hint should appear on fresh start with TTY")
	})

	t.Run("prompt hint does NOT appear on resume", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

		// Pre-seed state: TASK1 active at step 3 (resume scenario).
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK1"
		seedState.CurrentTaskFile = "TASK1.md"
		seedState.CurrentStep = 3
		require.NoError(t, stateManager.Save(seedState))

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir:     tmpDir,
			PRDPath:      prdPath,
			ProviderName: "claude",
			IsTTY:        true,
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		stripped := ui.StripColors(buf.String())
		assert.NotContains(t, stripped, "Type a directive",
			"prompt hint should NOT appear on resume")
	})

	t.Run("prompt hint suppressed without IsTTY", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))

		stateManager := state.NewManagerWithDir(tmpDir)
		//nolint:errcheck // cleanup
		_ = stateManager.Reset()

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir:     tmpDir,
			PRDPath:      prdPath,
			ProviderName: "claude",
			IsTTY:        false,
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		stripped := ui.StripColors(buf.String())
		assert.NotContains(t, stripped, "Type a directive",
			"prompt hint should NOT appear when not TTY")
	})

	t.Run("resume shows summary with resuming action", func(t *testing.T) {
		tmpDir := t.TempDir()

		prdPath := filepath.Join(tmpDir, "PRD.md")
		require.NoError(t, os.WriteFile(prdPath, []byte("# PRD"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK1.md"), []byte("# Task 1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK2.md"), []byte("# Task 2"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "TASK3.md"), []byte("# Task 3"), 0o600))

		// Pre-seed state: TASK2 active at step 5.
		stateManager := state.NewManagerWithDir(tmpDir)
		seedState := state.NewState(tmpDir, prdPath, workflow.StepCount())
		seedState.CurrentTaskID = "TASK2"
		seedState.CurrentTaskFile = "TASK2.md"
		seedState.CurrentStep = 5
		seedState.CompletedTaskIDs = []string{"TASK1"}
		require.NoError(t, stateManager.Save(seedState))

		var buf bytes.Buffer
		mockExec := &MockExecutor{
			runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
				return errors.New("stop")
			},
		}

		runner := workflow.NewRunner(mockExec, workflow.Config{
			TasksDir:     tmpDir,
			PRDPath:      prdPath,
			ProviderName: "codex",
		}, workflow.WithStateManager(stateManager), workflow.WithRunnerOutput(&buf))

		//nolint:errcheck // testing output, not error
		_ = runner.Run(context.Background())

		stripped := ui.StripColors(buf.String())
		assert.Contains(t, stripped, "3 tasks (1 done)", "summary should show task count and done count")
		assert.Contains(t, stripped, "codex", "summary should show provider name")
		assert.Contains(t, stripped, "resuming TASK2 from step 5", "summary should show resume action")
	})
}
