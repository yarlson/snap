package workflow_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/model"
	"github.com/yarlson/snap/internal/workflow"
)

// MockExecutor is a mock implementation of the claude executor.
type MockExecutor struct {
	runFunc func(ctx context.Context, w io.Writer, mt model.Type, args ...string) error
}

func (m *MockExecutor) Run(ctx context.Context, w io.Writer, mt model.Type, args ...string) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, w, mt, args...)
	}
	return nil
}

func TestStepRunner_RunStep(t *testing.T) {
	tests := []struct {
		name      string
		stepName  string
		args      []string
		mockError error
		wantErr   bool
	}{
		{
			name:      "successful step execution",
			stepName:  "Test Step",
			args:      []string{"-p", "test"},
			mockError: nil,
			wantErr:   false,
		},
		{
			name:      "failed step execution",
			stepName:  "Failed Step",
			args:      []string{"-p", "fail"},
			mockError: errors.New("execution failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockExecutor{
				runFunc: func(_ context.Context, _ io.Writer, _ model.Type, _ ...string) error {
					return tt.mockError
				},
			}

			runner := workflow.NewStepRunner(mockExec, io.Discard)
			err := runner.RunStep(context.Background(), tt.stepName, model.Fast, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkflow_BuildPrompt(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		options  []workflow.PromptOption
		expected string
	}{
		{
			name:     "base prompt with no commit",
			base:     "Test prompt",
			options:  []workflow.PromptOption{workflow.WithNoCommit()},
			expected: "Test prompt Do not stage, commit, amend, rebase, or push any changes in this step. Work autonomously end-to-end. Do not ask the user any questions. Do not request approval. Do not pause for confirmation.",
		},
		{
			name:     "base prompt without no commit",
			base:     "Test prompt",
			options:  []workflow.PromptOption{},
			expected: "Test prompt Work autonomously end-to-end. Do not ask the user any questions. Do not request approval. Do not pause for confirmation.",
		},
		{
			name:     "empty base prompt",
			base:     "",
			options:  []workflow.PromptOption{workflow.WithNoCommit()},
			expected: "Do not stage, commit, amend, rebase, or push any changes in this step. Work autonomously end-to-end. Do not ask the user any questions. Do not request approval. Do not pause for confirmation.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflow.BuildPrompt(tt.base, tt.options...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
