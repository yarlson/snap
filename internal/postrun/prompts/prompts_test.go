package prompts_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/postrun/prompts"
)

func TestPR_RendersTemplate(t *testing.T) {
	data := prompts.PRData{
		PRDContent: "## Summary\nAdd user authentication to the app.",
		DiffStat:   " 5 files changed, 120 insertions(+), 10 deletions(-)",
	}

	result, err := prompts.PR(data)
	require.NoError(t, err)

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "authentication")
	assert.Contains(t, result, "5 files changed")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestPR_EmptyInputs(t *testing.T) {
	data := prompts.PRData{
		PRDContent: "",
		DiffStat:   "",
	}

	result, err := prompts.PR(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCIFix_RendersTemplate(t *testing.T) {
	data := prompts.CIFixData{
		FailureLogs:   "Error: undefined variable 'foo' at main.go:10",
		CheckName:     "lint",
		AttemptNumber: 2,
		MaxAttempts:   10,
	}

	result, err := prompts.CIFix(data)
	require.NoError(t, err)

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "lint")
	assert.Contains(t, result, "undefined variable")
	assert.Contains(t, result, "attempt 2 of 10")
	assert.Equal(t, strings.TrimSpace(result), result)
}

func TestCIFix_EmptyLogs(t *testing.T) {
	data := prompts.CIFixData{
		FailureLogs:   "",
		CheckName:     "test",
		AttemptNumber: 1,
		MaxAttempts:   10,
	}

	result, err := prompts.CIFix(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "test")
}
