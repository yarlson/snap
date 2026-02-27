package input

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

// withStubbedTerm swaps termMakeRaw/termRestore with test doubles and restores
// them when the test finishes. The restored flag is set to true when termRestore
// is called.
func withStubbedTerm(t *testing.T, restored *bool) {
	t.Helper()
	origMakeRaw := termMakeRaw
	origRestore := termRestore
	t.Cleanup(func() {
		termMakeRaw = origMakeRaw
		termRestore = origRestore
	})

	termMakeRaw = func(int) (*term.State, error) { return new(term.State), nil }
	termRestore = func(int, *term.State) error { *restored = true; return nil }
}

func TestWithRawMode_RestoresOnNormalReturn(t *testing.T) {
	var restored bool
	withStubbedTerm(t, &restored)

	err := WithRawMode(0, func() error { return nil })
	require.NoError(t, err)
	assert.True(t, restored, "terminal should be restored on normal return")
}

func TestWithRawMode_RestoresOnError(t *testing.T) {
	var restored bool
	withStubbedTerm(t, &restored)

	testErr := errors.New("test error")
	err := WithRawMode(0, func() error { return testErr })
	require.ErrorIs(t, err, testErr)
	assert.True(t, restored, "terminal should be restored on error return")
}

func TestWithRawMode_RestoresOnPanic(t *testing.T) {
	var restored bool
	withStubbedTerm(t, &restored)

	assert.Panics(t, func() {
		//nolint:errcheck // Testing panic path; error return is irrelevant.
		WithRawMode(0, func() error { panic("test panic") })
	})
	assert.True(t, restored, "terminal should be restored on panic")
}
