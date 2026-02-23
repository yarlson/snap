package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/workflow"
)

func TestStepContext_SetAndGet(t *testing.T) {
	ctx := workflow.NewStepContext()

	ctx.Set(3, 9, "Validate implementation")
	current, total, name := ctx.Get()

	assert.Equal(t, 3, current)
	assert.Equal(t, 9, total)
	assert.Equal(t, "Validate implementation", name)
}

func TestStepContext_DefaultValues(t *testing.T) {
	ctx := workflow.NewStepContext()

	current, total, name := ctx.Get()

	assert.Equal(t, 0, current)
	assert.Equal(t, 0, total)
	assert.Equal(t, "", name)
}

func TestStepContext_OverwritesPrevious(t *testing.T) {
	ctx := workflow.NewStepContext()

	ctx.Set(1, 9, "First step")
	ctx.Set(5, 10, "Fifth step")

	current, total, name := ctx.Get()
	assert.Equal(t, 5, current)
	assert.Equal(t, 10, total)
	assert.Equal(t, "Fifth step", name)
}
